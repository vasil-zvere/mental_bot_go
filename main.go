package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("ошибка конфигурации: %v", err)
	}

	logger, err := SetupLogging(cfg.Logging.Dir, cfg.Logging.BaseName)
	if err != nil {
		log.Fatalf("не удалось настроить логирование: %v", err)
	}
	defer logger.Close()

	log.Println("MentalBot запускается...")

	content, err := LoadContentStore("content.json")
	if err != nil {
		log.Fatalf("ошибка загрузки content.json: %v", err)
	}

	history, err := NewHistoryStore(cfg.Database.Path)
	if err != nil {
		log.Fatalf("не удалось открыть историю: %v", err)
	}
	defer history.Close()

	engine := NewEngine(content)
	reports := NewReportService(history, content)

	abcStore, err := NewABCStore(history.db)
	if err != nil {
		log.Fatalf("не удалось инициализировать ABC-дневник: %v", err)
	}
	engine.SetABCStore(abcStore)
	reports.SetABCStore(abcStore)

	wellbeingStore, err := NewWellbeingStore(history.db)
	if err != nil {
		log.Fatalf("не удалось инициализировать хранилище самочувствия: %v", err)
	}
	engine.SetWellbeingStore(wellbeingStore)
	reports.SetWellbeingStore(wellbeingStore)

	eraser := NewDataEraser(history.db)
	engine.SetDataEraser(eraser)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Планировщик напоминаний ───────────────────────────────────────────────
	notifier := NewNotifier(history.db)
	notifier.InactiveDays = cfg.Notifier.InactiveDays

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if cfg.Telegram.Token != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewTelegramClient(cfg.Telegram.Token, engine, history, reports)
			notifier.RegisterSender("telegram", NewTelegramSender(client))
			log.Println("Telegram-клиент запущен")
			if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("telegram: %w", err)
			}
		}()
	}

	if cfg.VK.Token != "" && cfg.VK.GroupID != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewVKClient(cfg.VK.Token, cfg.VK.GroupID, cfg.VK.APIVersion, engine, history, reports)
			notifier.RegisterSender("vk", NewVKSender(client))
			log.Println("VK-клиент запущен")
			if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("vk: %w", err)
			}
		}()
	}

	// Запуск планировщика напоминаний в отдельной горутине
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifier.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Println("бот остановлен")
	case err := <-errCh:
		log.Printf("ошибка: %v", err)
		stop()
	}

	wg.Wait()
	log.Println("MentalBot завершил работу")
}

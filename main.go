package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	content, err := LoadContentStore("content.json")
	if err != nil {
		log.Fatalf("ошибка загрузки content.json: %v", err)
	}

	engine := NewEngine(content)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tgToken := os.Getenv("TG_BOT_TOKEN")
	vkToken := os.Getenv("VK_GROUP_TOKEN")
	vkGroupIDStr := os.Getenv("VK_GROUP_ID")
	vkAPIVersion := os.Getenv("VK_API_VERSION")

	if tgToken == "" && (vkToken == "" || vkGroupIDStr == "") {
		log.Fatal("укажи хотя бы одну конфигурацию: TG_BOT_TOKEN или VK_GROUP_TOKEN + VK_GROUP_ID")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if tgToken != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewTelegramClient(tgToken, engine)
			if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("telegram: %w", err)
			}
		}()
	}

	if vkToken != "" && vkGroupIDStr != "" {
		vkGroupID, err := strconv.ParseInt(vkGroupIDStr, 10, 64)
		if err != nil {
			log.Fatalf("VK_GROUP_ID должен быть числом: %v", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewVKClient(vkToken, vkGroupID, vkAPIVersion, engine)
			if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("vk: %w", err)
			}
		}()
	}

	select {
	case <-ctx.Done():
		log.Println("бот остановлен")
	case err := <-errCh:
		log.Printf("ошибка: %v", err)
		stop()
	}

	wg.Wait()
}

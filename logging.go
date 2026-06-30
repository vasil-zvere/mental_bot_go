package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogMaxSizeBytes — порог размера файла лога, после которого происходит ротация.
const LogMaxSizeBytes = 10 * 1024 * 1024 // 10 МБ

// LogMaxBackups — сколько архивных файлов лога хранить.
const LogMaxBackups = 5

// RotatingLogger — простой логгер с ротацией по размеру файла.
// Пишет одновременно в файл и в стандартный вывод.
type RotatingLogger struct {
	mu       sync.Mutex
	dir      string
	baseName string
	file     *os.File
	size     int64
}

// NewRotatingLogger создаёт логгер, пишущий в dir/baseName.log.
// При превышении LogMaxSizeBytes файл переименовывается с таймстампом,
// создаётся новый, а старые файлы сверх LogMaxBackups удаляются.
func NewRotatingLogger(dir, baseName string) (*RotatingLogger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию логов: %w", err)
	}

	rl := &RotatingLogger{dir: dir, baseName: baseName}
	if err := rl.openCurrent(); err != nil {
		return nil, err
	}
	return rl, nil
}

func (rl *RotatingLogger) logPath() string {
	return filepath.Join(rl.dir, rl.baseName+".log")
}

func (rl *RotatingLogger) openCurrent() error {
	path := rl.logPath()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл лога: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	rl.file = f
	rl.size = info.Size()
	return nil
}

// Write реализует io.Writer — используется как вывод для стандартного log.Logger.
func (rl *RotatingLogger) Write(p []byte) (int, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.size+int64(len(p)) > LogMaxSizeBytes {
		if err := rl.rotate(); err != nil {
			// Если ротация не удалась — продолжаем писать в текущий файл,
			// чтобы не потерять логи.
			fmt.Fprintf(os.Stderr, "ошибка ротации лога: %v\n", err)
		}
	}

	n, err := rl.file.Write(p)
	rl.size += int64(n)
	return n, err
}

// rotate переименовывает текущий файл лога и открывает новый,
// затем удаляет самые старые архивы сверх лимита LogMaxBackups.
func (rl *RotatingLogger) rotate() error {
	if rl.file != nil {
		rl.file.Close()
	}

	timestamp := time.Now().Format("20060102_150405")
	archivedPath := filepath.Join(rl.dir, fmt.Sprintf("%s_%s.log", rl.baseName, timestamp))

	if err := os.Rename(rl.logPath(), archivedPath); err != nil {
		// Если переименовать не получилось (например, файла нет) — просто открываем новый
		_ = err
	}

	if err := rl.openCurrent(); err != nil {
		return err
	}

	rl.cleanupOldBackups()
	return nil
}

// cleanupOldBackups удаляет старые архивные файлы лога сверх LogMaxBackups.
func (rl *RotatingLogger) cleanupOldBackups() {
	pattern := filepath.Join(rl.dir, rl.baseName+"_*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= LogMaxBackups {
		return
	}

	// Сортируем по имени — таймстамп в имени гарантирует хронологический порядок.
	for i := 0; i < len(matches)-LogMaxBackups; i++ {
		_ = os.Remove(matches[i])
	}
}

// Close закрывает текущий файл лога.
func (rl *RotatingLogger) Close() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.file != nil {
		return rl.file.Close()
	}
	return nil
}

// SetupLogging настраивает стандартный пакет log на запись одновременно
// в ротируемый файл и в стандартный вывод (stdout), и возвращает логгер для Close().
func SetupLogging(dir, baseName string) (*RotatingLogger, error) {
	rl, err := NewRotatingLogger(dir, baseName)
	if err != nil {
		return nil, err
	}

	multi := io.MultiWriter(os.Stdout, rl)
	log.SetOutput(multi)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return rl, nil
}

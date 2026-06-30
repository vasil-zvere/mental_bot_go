package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config — конфигурация бота, загружаемая из config.yaml.
// Значения переменных окружения, если они заданы, имеют приоритет
// над значениями из файла — это удобно для CI/CD и контейнеров.
type Config struct {
	Telegram struct {
		Token string `yaml:"token"`
	} `yaml:"telegram"`

	VK struct {
		Token      string `yaml:"token"`
		GroupID    int64  `yaml:"group_id"`
		APIVersion string `yaml:"api_version"`
	} `yaml:"vk"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Logging struct {
		Dir      string `yaml:"dir"`
		BaseName string `yaml:"base_name"`
	} `yaml:"logging"`

	Notifier struct {
		InactiveDays int `yaml:"inactive_days"`
	} `yaml:"notifier"`
}

// defaultConfig возвращает конфигурацию со значениями по умолчанию.
func defaultConfig() Config {
	var c Config
	c.Database.Path = "bot_history.db"
	c.Logging.Dir = "./logs"
	c.Logging.BaseName = "mentalbot"
	c.Notifier.InactiveDays = 3
	c.VK.APIVersion = "5.199"
	return c
}

// LoadConfig загружает конфигурацию из YAML-файла, если он существует,
// затем применяет переопределения из переменных окружения и валидирует результат.
func LoadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("ошибка разбора %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return cfg, fmt.Errorf("ошибка чтения %s: %w", path, err)
	}
	// Если файла нет — используем значения по умолчанию и переменные окружения,
	// это сохраняет совместимость с предыдущим способом запуска.

	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// applyEnvOverrides переопределяет поля конфигурации значениями
// переменных окружения, если они заданы.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("TG_BOT_TOKEN"); v != "" {
		c.Telegram.Token = v
	}
	if v := os.Getenv("VK_GROUP_TOKEN"); v != "" {
		c.VK.Token = v
	}
	if v := os.Getenv("VK_GROUP_ID"); v != "" {
		var id int64
		if _, err := fmt.Sscanf(v, "%d", &id); err == nil {
			c.VK.GroupID = id
		}
	}
	if v := os.Getenv("VK_API_VERSION"); v != "" {
		c.VK.APIVersion = v
	}
}

// Validate проверяет, что конфигурация содержит минимально достаточные данные для запуска.
func (c Config) Validate() error {
	hasTelegram := c.Telegram.Token != ""
	hasVK := c.VK.Token != "" && c.VK.GroupID != 0

	if !hasTelegram && !hasVK {
		return fmt.Errorf("укажи хотя бы одну платформу: telegram.token в config.yaml " +
			"(или TG_BOT_TOKEN) либо vk.token + vk.group_id (или VK_GROUP_TOKEN + VK_GROUP_ID)")
	}
	if c.VK.Token != "" && c.VK.GroupID == 0 {
		return fmt.Errorf("указан vk.token, но не указан vk.group_id")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database.path не может быть пустым")
	}
	if c.Notifier.InactiveDays <= 0 {
		return fmt.Errorf("notifier.inactive_days должен быть положительным числом")
	}
	return nil
}

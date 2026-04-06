package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NATSURL      string `yaml:"nats_url"`
	Port         string `yaml:"port"`
	CacheEnabled bool   `yaml:"cache_enabled"`
	LogLevel     string `yaml:"log_level"`
}

func Load() (*Config, error) {
	cfg := &Config{
		NATSURL:      "nats://127.0.0.1:4222",
		Port:         "8080",
		CacheEnabled: true,
		LogLevel:     "info",
	}

	data, err := os.ReadFile("config.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
		}
	}
	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.NATSURL = v
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("CACHE_ENABLED"); v == "false" {
		cfg.CacheEnabled = false
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	return cfg, nil
}

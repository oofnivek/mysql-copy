package config

import (
	"log/slog"
	"os"
)

type Config struct {
	Addr     string
	Env      string
	LogLevel slog.Level
}

func Load() *Config {
	cfg := &Config{
		Addr: getEnv("ADDR", ":8080"),
		Env:  getEnv("ENV", "development"),
	}

	if cfg.Env == "production" {
		cfg.LogLevel = slog.LevelInfo
	} else {
		cfg.LogLevel = slog.LevelDebug
	}

	return cfg
}

func (c *Config) IsDev() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

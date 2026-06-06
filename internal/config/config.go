package config

import (
	"log/slog"
	"os"
	"path/filepath"
)

type Config struct {
	Addr            string
	Env             string
	LogLevel        slog.Level
	ConnectionsFile string
	PresetsFile     string
}

func Load() *Config {
	cfg := &Config{
		Addr:            getEnv("ADDR", ":8080"),
		Env:             getEnv("ENV", "development"),
		ConnectionsFile: getEnv("CONNECTIONS_FILE", defaultDataFile("connections.json")),
		PresetsFile:     getEnv("PRESETS_FILE", defaultDataFile("presets.json")),
	}

	if cfg.Env == "production" {
		cfg.LogLevel = slog.LevelInfo
	} else {
		cfg.LogLevel = slog.LevelDebug
	}

	return cfg
}

func defaultDataFile(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".mysql-copy", name)
	}
	return filepath.Join(home, ".mysql-copy", name)
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

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
}

func Load() *Config {
	cfg := &Config{
		Addr:            getEnv("ADDR", ":8080"),
		Env:             getEnv("ENV", "development"),
		ConnectionsFile: getEnv("CONNECTIONS_FILE", defaultConnectionsFile()),
	}

	if cfg.Env == "production" {
		cfg.LogLevel = slog.LevelInfo
	} else {
		cfg.LogLevel = slog.LevelDebug
	}

	return cfg
}

func defaultConnectionsFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mysql-copy/connections.json"
	}
	return filepath.Join(home, ".mysql-copy", "connections.json")
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

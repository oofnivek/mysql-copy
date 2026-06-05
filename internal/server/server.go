package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/oofnivek/mysql-copy/internal/config"
	"github.com/oofnivek/mysql-copy/internal/handler"
)

func New(cfg *config.Config, logger *slog.Logger, webFS fs.FS) *http.Server {
	h := handler.New(cfg, logger, webFS)

	return &http.Server{
		Addr:         cfg.Addr,
		Handler:      h.Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

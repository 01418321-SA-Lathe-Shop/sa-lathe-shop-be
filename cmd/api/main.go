package main

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/config"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/repository/postgres"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConnections)
	if err != nil {
		logger.Error("database configuration invalid")
		os.Exit(1)
	}
	defer db.Close()
	healthService := health.NewService(db, cfg.DBPingTimeout)
	router := httpserver.Router(logger, cfg.HTTPTimeout, cfg.CORSOrigins, health.Live, healthService.Handler)
	if err := httpserver.Run(ctx, logger, cfg.HTTPAddr, cfg.ShutdownTimeout, router); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

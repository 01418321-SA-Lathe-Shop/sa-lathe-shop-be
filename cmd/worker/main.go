package main

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/config"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/notification"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/repository/postgres"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	db, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConnections)
	if err != nil {
		logger.Error("database configuration invalid")
		os.Exit(1)
	}
	defer db.Close()
	check, cancel := context.WithTimeout(ctx, 3*time.Second)
	err = db.Check(check)
	cancel()
	if err != nil {
		logger.Error("database not ready")
		os.Exit(1)
	}
	integrations, err := config.LoadIntegrations()
	if err != nil {
		logger.Error("invalid integration configuration", "error", err)
		os.Exit(1)
	}
	line := notification.NewLINE(integrations.LINEEnabled, integrations.LINEToken, integrations.LINEAdminUserID)
	logger.Info("notification worker started", "line_enabled", integrations.LINEEnabled)
	if err = notification.NewDispatcher(db, line, logger).Run(ctx); err != nil {
		logger.Error("notification worker failed")
		os.Exit(1)
	}
	logger.Info("worker stopped")
}

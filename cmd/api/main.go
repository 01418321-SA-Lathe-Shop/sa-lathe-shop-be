package main

import (
	"context"
	"errors"
	"flag"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/config"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/repository/postgres"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	bootstrap := flag.Bool("bootstrap-admin", false, "Create the first Admin from private process environment, then exit")
	flag.Parse()
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
	authService, err := auth.NewService(db, auth.NewArgonHasher(), cfg.SessionTTL)
	if err != nil {
		logger.Error("auth initialization failed")
		os.Exit(1)
	}
	if *bootstrap {
		setup, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		account, err := authService.BootstrapAdmin(setup, os.Getenv("BOOTSTRAP_ADMIN_USERNAME"), os.Getenv("BOOTSTRAP_ADMIN_NAME"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
		if err != nil {
			message := "Admin bootstrap failed; check migrations and configuration"
			if errors.Is(err, auth.ErrAdminExists) || errors.Is(err, auth.ErrInvalidInput) {
				message = err.Error()
			}
			logger.Error(message)
			os.Exit(1)
		}
		logger.Info("initial Admin created", "account_id", account.ID)
		return
	}
	healthService := health.NewService(db, cfg.DBPingTimeout)
	authHandler := auth.NewHandler(authService)
	router := httpserver.RouterWithRoutes(logger, cfg.HTTPTimeout, cfg.CORSOrigins, health.Live, healthService.Handler, authHandler.Routes)
	if err := httpserver.Run(ctx, logger, cfg.HTTPAddr, cfg.ShutdownTimeout, router); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

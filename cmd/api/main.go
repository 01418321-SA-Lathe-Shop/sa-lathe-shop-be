package main

import (
	"context"
	"errors"
	"flag"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/config"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/notification"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/repository/postgres"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
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
	hasher := auth.NewArgonHasher()
	authService, err := auth.NewService(db, hasher, cfg.SessionTTL)
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
	integrations, err := config.LoadIntegrations()
	if err != nil {
		logger.Error("invalid integration configuration", "error", err)
		os.Exit(1)
	}
	store := objectstore.New(objectstore.Config{Endpoint: integrations.S3Endpoint, PublicEndpoint: integrations.S3PublicEndpoint, Region: integrations.S3Region, Bucket: integrations.S3Bucket, AccessKey: integrations.S3AccessKey, SecretKey: integrations.S3SecretKey, PathStyle: integrations.S3PathStyle})
	requestService := req.NewService(db)
	fileService := file.NewService(db, store, requestService)
	customerService := auth.NewCustomerService(db, hasher)
	router := httpserver.RouterWithRoutes(logger, cfg.HTTPTimeout, cfg.CORSOrigins, health.Live, healthService.Handler, func(r chi.Router) {
		authHandler.Routes(r)
		authHandler.CustomerRoutes(r, customerService)
		req.NewHandler(requestService).Routes(r, authHandler.Authenticate)
		fileService.Routes(r, authHandler.Authenticate)
		notification.Routes(r, notification.NewInbox(db), authHandler.Authenticate)
		quotation.NewHandler(quotation.NewService(db)).Routes(r, authHandler.Authenticate)
		quotation.NewDecisionHandler(quotation.NewDecisionService(db)).Routes(r, authHandler.Authenticate)
	})
	if err := httpserver.Run(ctx, logger, cfg.HTTPAddr, cfg.ShutdownTimeout, router); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

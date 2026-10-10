package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func Run(ctx context.Context, logger *slog.Logger, addr string, shutdown time.Duration, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	errs := make(chan error, 1)
	go func() { logger.Info("api listening", "addr", addr); errs <- srv.ListenAndServe() }()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		stop, cancel := context.WithTimeout(context.Background(), shutdown)
		defer cancel()
		if err := srv.Shutdown(stop); err != nil {
			_ = srv.Close()
			return err
		}
		return nil
	}
}

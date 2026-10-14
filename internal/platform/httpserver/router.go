package httpserver

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"log/slog"
	"net/http"
	"time"
)

func Router(logger *slog.Logger, timeout time.Duration, origins []string, live http.HandlerFunc, ready ...http.HandlerFunc) http.Handler {
	var readyHandler http.HandlerFunc
	if len(ready) > 0 {
		readyHandler = ready[0]
	}
	return RouterWithRoutes(logger, timeout, origins, live, readyHandler, nil)
}

func RouterWithRoutes(logger *slog.Logger, timeout time.Duration, origins []string, live, ready http.HandlerFunc, register func(chi.Router)) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(RequestLogger(logger))
	r.Use(Recoverer(logger))
	r.Use(cors.Handler(cors.Options{AllowedOrigins: origins, AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"}, ExposedHeaders: []string{"X-Request-ID"}, MaxAge: 300}))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
			next.ServeHTTP(w, r)
		})
	})
	r.Use(Timeout(timeout))
	r.Get("/health", live)
	if ready != nil {
		r.Get("/health/ready", ready)
	}
	if register != nil {
		register(r)
	}
	r.NotFound(func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, 404, "not_found", "Endpoint not found") })
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, 405, "method_not_allowed", "Method not allowed")
	})
	return r
}

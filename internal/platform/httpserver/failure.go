package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/failure"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type loggerContextKey struct{}

// LogInternalError deliberately never logs err.Error(): driver/network messages
// can contain SQL values, credentials, signed URLs or private evidence metadata.
func LogInternalError(r *http.Request, err error) {
	if err == nil {
		return
	}
	logger, _ := r.Context().Value(loggerContextKey{}).(*slog.Logger)
	if logger == nil {
		logger = slog.Default()
	}
	kind := "internal"
	if errors.Is(err, context.DeadlineExceeded) {
		kind = "deadline_exceeded"
	} else if errors.Is(err, context.Canceled) {
		kind = "canceled"
	}
	operation, state := "", ""
	var labelled *failure.OperationError
	if errors.As(err, &labelled) {
		operation = labelled.Operation
	}
	var databaseError interface{ SQLState() string }
	if errors.As(err, &databaseError) {
		// Only accept a standard five-character SQLSTATE, not arbitrary error text.
		candidate := databaseError.SQLState()
		if len(candidate) == 5 {
			valid := true
			for _, c := range candidate {
				valid = valid && ((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z'))
			}
			if valid {
				state, kind = candidate, "database"
			}
		}
	}
	route := ""
	if routing := chi.RouteContext(r.Context()); routing != nil {
		route = routing.RoutePattern()
	}
	logger.Error("request operation failed", "request_id", middleware.GetReqID(r.Context()), "method", r.Method, "route", route, "operation", operation, "error_kind", kind, "sqlstate", state)
}

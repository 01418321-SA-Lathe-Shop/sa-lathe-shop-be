package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/failure"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type privateDatabaseError struct{}

func (privateDatabaseError) Error() string    { return "PASSWORD_SLIP_SIGNED_URL_PRIVATE" }
func (privateDatabaseError) SQLState() string { return "08006" }

func TestInternalFailureLogsOnlySafeContext(t *testing.T) {
	for name, cause := range map[string]error{"database": privateDatabaseError{}, "deadline_exceeded": context.DeadlineExceeded, "canceled": context.Canceled, "internal": errors.New("PASSWORD_SLIP_SIGNED_URL_PRIVATE")} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			router := chi.NewRouter()
			router.Use(middleware.RequestID, RequestLogger(logger))
			router.Get("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
				LogInternalError(r, failure.Wrap("fulfillment.actor", cause))
				WriteError(w, r, 500, "internal_error", "An unexpected error occurred")
			})
			request := httptest.NewRequest("GET", "/jobs/private-id?token=PASSWORD_SLIP_SIGNED_URL_PRIVATE", nil)
			request.Header.Set("Authorization", "Bearer PASSWORD_SLIP_SIGNED_URL_PRIVATE")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if strings.Contains(output.String(), "PASSWORD_SLIP_SIGNED_URL_PRIVATE") || strings.Contains(response.Body.String(), "fulfillment.actor") {
				t.Fatal("private error/header/query content leaked")
			}
			var record map[string]any
			if err := json.NewDecoder(&output).Decode(&record); err != nil {
				t.Fatal(err)
			}
			if record["request_id"] == "" || record["route"] != "/jobs/{id}" || record["operation"] != "fulfillment.actor" || record["error_kind"] != name {
				t.Fatal("safe diagnostic context missing", record)
			}
			if name == "database" && record["sqlstate"] != "08006" {
				t.Fatal("safe SQLSTATE missing")
			}
			if !errors.Is(failure.Wrap("operation", cause), cause) {
				t.Fatal("wrapped cause was lost")
			}
		})
	}
}

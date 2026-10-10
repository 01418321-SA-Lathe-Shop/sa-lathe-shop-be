package httpserver_test

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPResponses(t *testing.T) {
	r := httpserver.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second, []string{"http://localhost:3000"}, health.Live)
	for path, status := range map[string]int{"/health": 200, "/missing": 404} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status || w.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://untrusted.test")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("untrusted origin allowed")
	}
}

func TestPanicAndTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, h := range map[string]http.HandlerFunc{"panic": func(http.ResponseWriter, *http.Request) { panic("test") }, "timeout": func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }} {
		t.Run(name, func(t *testing.T) {
			r := httpserver.Router(logger, 10*time.Millisecond, []string{"http://localhost:3000"}, h)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
			want := 500
			if name == "timeout" {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("got %d want %d", w.Code, want)
			}
		})
	}
}

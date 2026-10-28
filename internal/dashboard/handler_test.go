package dashboard

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandlerGuardAndQuery(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(nil)).Routes(router, func(h http.Handler) http.Handler { return h })
	for _, path := range []string{"/dashboard/overview", "/dashboard/jobs"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 403 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Code, w.Header())
		}
	}
	for _, raw := range []string{"status=queued", "range=", "range=7d&range=30d", "token=private"} {
		q, _ := url.ParseQuery(raw)
		if queryAllowed(q, "range") {
			t.Fatal(raw)
		}
	}
}

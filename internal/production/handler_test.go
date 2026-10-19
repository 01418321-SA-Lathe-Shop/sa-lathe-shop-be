package production

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerGuard(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(nil), nil).Routes(router, func(h http.Handler) http.Handler { return h })
	job := uuid.NewString()
	for _, v := range []struct{ method, path string }{{"GET", "/production/jobs"}, {"GET", "/production/jobs/" + job}, {"POST", "/production/jobs/" + job + "/start"}, {"POST", "/production/jobs/" + job + "/usage"}, {"POST", "/production/jobs/" + job + "/time"}, {"POST", "/production/jobs/" + job + "/complete"}, {"PUT", "/production/jobs/" + job + "/labor-cost"}, {"POST", "/production/jobs/" + job + "/photos/uploads"}, {"GET", "/job-photos/" + uuid.NewString() + "/download"}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(v.method, v.path, nil))
		if w.Code != 403 {
			t.Fatalf("%s %s: %d", v.method, v.path, w.Code)
		}
	}
}

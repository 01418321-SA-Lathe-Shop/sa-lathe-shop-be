package payment

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictEvidenceJSON(t *testing.T) {
	repo := &fakeRepository{}
	r := chi.NewRouter()
	NewHandler(NewService(repo)).Routes(r, func(next http.Handler) http.Handler { return next })
	for _, body := range []string{`{"kind":"deposit","submitted_by":"spoof"}`, `{"kind":"deposit","amount":800}`, `{"kind":"po"} {}`, `{"outcome":"approved","reviewed_at":"2030-01-01"}`} {
		request := httptest.NewRequest("POST", "/quotations/"+uuid.NewString()+"/evidence", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != 400 {
			t.Fatal(response.Code, body)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid input reached repository")
	}
}

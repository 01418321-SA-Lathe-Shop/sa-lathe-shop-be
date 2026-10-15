package quotation

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecisionHandlerRejectsSpoofedAndExtraJSON(t *testing.T) {
	repo := &decisionFake{}
	router := chi.NewRouter()
	NewDecisionHandler(NewDecisionService(repo)).Routes(router, func(next http.Handler) http.Handler { return next })
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"outcome":"accept","expected_lock_version":2,"actor_id":"spoofed"}`, 400},
		{`{"outcome":"accept","expected_lock_version":2,"accepted_at":"2030-01-01"}`, 400},
		{`{"outcome":"accept","expected_lock_version":2} {}`, 400},
		{`{"outcome":"accept","expected_lock_version":2}`, 403},
	} {
		r := httptest.NewRequest("POST", "/quotations/"+testID+"/decision", strings.NewReader(test.body))
		r.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		if response.Code != test.status {
			t.Fatalf("wanted %d, got %d", test.status, response.Code)
		}
	}
	if repo.calls != 0 {
		t.Fatal("spoofed or anonymous decision reached repository")
	}
}

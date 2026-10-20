package delivery

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAuthenticationAndStrictJSON(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(NewService(nil), NewEvidenceService(nil, nil)).Routes(r, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unauthorized", 401) })
	})
	for _, path := range []string{"/fulfillment/jobs", "/customer-evidence/00000000-0000-0000-0000-000000000001/download"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
}
func TestServiceRoleAndInputGuards(t *testing.T) {
	s := NewService(nil)
	ctx := httptest.NewRequest("GET", "/", nil).Context()
	worker := auth.Account{Active: true, Role: "worker"}
	if _, e := s.Issue(ctx, auth.Account{Active: true, Role: "customer"}, "", Command{}); e == nil {
		t.Fatal("customer issue")
	}
	if _, e := s.Get(ctx, worker, "invalid"); e == nil {
		t.Fatal("invalid ID")
	}
	if _, e := s.Ship(ctx, worker, "", ShipInput{Method: "self", ReceiverName: strings.Repeat("x", 201)}); e == nil {
		t.Fatal("receiver bounds")
	}
	if _, e := s.Terms(ctx, worker, "", TermsInput{}); e == nil {
		t.Fatal("required terms")
	}
}

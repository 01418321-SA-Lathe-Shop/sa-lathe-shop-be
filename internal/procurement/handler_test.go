package procurement

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPGuardBeforeParsing(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(nil)).Routes(router, func(h http.Handler) http.Handler { return h })
	for _, tt := range []struct{ method, path string }{{"GET", "/suppliers"}, {"POST", "/suppliers"}, {"PUT", "/suppliers/" + uuid.NewString()}, {"GET", "/material-orders"}, {"POST", "/material-orders"}, {"GET", "/material-orders/" + uuid.NewString()}, {"POST", "/material-orders/" + uuid.NewString() + "/actions"}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(tt.method, tt.path, nil))
		if response.Code != 403 {
			t.Fatalf("%+v: %d", tt, response.Code)
		}
	}
}

type placementSpy struct {
	Repository
	input OrderInput
}

func (s *placementSpy) PlaceOrder(_ context.Context, _ string, in OrderInput) (Order, error) {
	s.input = in
	return Order{ID: uuid.NewString()}, nil
}
func TestFlexiblePlacementCanonicalization(t *testing.T) {
	spy := &placementSpy{}
	s := NewService(spy)
	a := auth.Account{ID: uuid.NewString(), Role: auth.RoleAdmin, Active: true}
	first, second := uuid.NewString(), uuid.NewString()
	in := OrderInput{IdempotencyKey: uuid.NewString(), SupplierID: uuid.NewString(), JobIDs: []string{second, first}, Lines: []LineInput{{MaterialID: uuid.NewString(), Quantity: decimal("9999"), UnitCost: decimal("1")}}}
	expected := time.Date(2026, 11, 1, 9, 0, 0, 123456789, time.FixedZone("Bangkok", 7*60*60))
	in.ExpectedAt = &expected
	if _, err := s.Place(context.Background(), a, in); err != nil {
		t.Fatal(err)
	}
	if len(spy.input.JobIDs) != 2 || spy.input.JobIDs[0] > spy.input.JobIDs[1] {
		t.Fatal("job references not sorted")
	}
	if in.JobIDs[0] != second {
		t.Fatal("mutated caller slice")
	}
	if spy.input.ExpectedAt.Location() != time.UTC || spy.input.ExpectedAt.Nanosecond() != 123456000 || in.ExpectedAt.Nanosecond() != 123456789 {
		t.Fatal("expected_at precision/caller mutation")
	}
	in.JobIDs = nil
	if _, err := s.Place(context.Background(), a, in); err != nil {
		t.Fatal("general stock order rejected", err)
	}
	in.JobIDs = []string{first, first}
	if _, err := s.Place(context.Background(), a, in); err == nil {
		t.Fatal("duplicate job reference accepted")
	}
}
func TestHTTPErrorEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	respond(w, r, nil, ErrConflict)
	if w.Code != 409 || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatal("inconsistent error", w.Code, w.Header())
	}
}

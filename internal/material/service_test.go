package material

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPermissionsAndInvalidPlans(t *testing.T) {
	s := NewService(nil)
	ctx := context.Background()
	worker := auth.Account{Role: auth.RoleWorker, Active: true}
	admin := auth.Account{Role: auth.RoleAdmin, Active: true}
	customer := auth.Account{Role: auth.RoleCustomer, Active: true}
	id := uuid.NewString()
	if _, err := s.CreateUnit(ctx, worker, UnitInput{Name: "kg"}); !errors.Is(err, req.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := s.Search(ctx, customer, "", 50, 0); !errors.Is(err, req.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := s.Save(ctx, admin, id, PlanInput{}); !errors.Is(err, req.ErrForbidden) {
		t.Fatal(err)
	}
	quantity := decimal(t, "1")
	for _, in := range []PlanInput{{IdempotencyKey: uuid.NewString()}, {IdempotencyKey: uuid.NewString(), Lines: []Requirement{{MaterialID: id}}}, {IdempotencyKey: uuid.NewString(), Lines: []Requirement{{MaterialID: id, Quantity: &quantity}, {MaterialID: id, Quantity: &quantity}}}} {
		if _, err := s.Save(ctx, worker, id, in); !errors.Is(err, req.ErrInvalidInput) {
			t.Fatal("bad plan accepted", err)
		}
	}
}
func TestStrictInventoryJSON(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(nil)).Routes(router, func(next http.Handler) http.Handler { return next })
	for _, body := range []string{`{"mode":"addition","quantity":2}`, `{"mode":"addition","created_by":"spoofed"}`, `{"quantity":"1"} {}`} {
		r := httptest.NewRequest("POST", "/materials/"+uuid.NewString()+"/stock", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
}

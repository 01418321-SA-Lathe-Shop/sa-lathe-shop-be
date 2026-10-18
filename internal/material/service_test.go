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

type planSpy struct {
	Repository
	Input       PlanInput
	ReserveOnly bool
}

func (s *planSpy) ApplyPlan(_ context.Context, _ string, id string, in PlanInput, reserve bool) (Plan, error) {
	s.Input = in
	s.ReserveOnly = reserve
	return Plan{JobID: id, Version: in.ExpectedVersion + 1}, nil
}
func TestServiceCanonicalPlanAndRecheck(t *testing.T) {
	spy := &planSpy{}
	service := NewService(spy)
	worker := auth.Account{ID: uuid.NewString(), Role: auth.RoleWorker, Active: true}
	quantity := decimal(t, "1.25")
	jobID := uuid.NewString()
	in := PlanInput{IdempotencyKey: uuid.NewString(), Lines: []Requirement{{MaterialID: "ffffffff-ffff-ffff-ffff-ffffffffffff", Quantity: &quantity}, {MaterialID: "00000000-0000-0000-0000-000000000001", Quantity: &quantity}}}
	if _, err := service.Save(context.Background(), worker, jobID, in); err != nil {
		t.Fatal(err)
	}
	if spy.ReserveOnly || spy.Input.Lines[0].MaterialID != "00000000-0000-0000-0000-000000000001" {
		t.Fatal("plan was not canonicalized")
	}
	if in.Lines[0].MaterialID != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatal("service mutated the caller's plan slice")
	}
	if _, err := service.Reserve(context.Background(), worker, jobID, ReserveInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if !spy.ReserveOnly || spy.Input.Lines != nil {
		t.Fatal("recheck trusted client requirements")
	}
}

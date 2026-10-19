package production

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func qty(s string) m.Decimal {
	v, e := m.ParseDecimal(s)
	if e != nil {
		panic(e)
	}
	return v
}
func TestOwnReservationFirst(t *testing.T) {
	stock := m.Material{Active: true, OnHand: qty("10"), Reserved: qty("8")}
	used, next, e := IssueEffect(stock, qty("3"), qty("5"))
	if e != nil || used.String() != "3.000000" || next.String() != "5.000000" {
		t.Fatal(used, next, e)
	}
	if _, _, e = IssueEffect(stock, qty("3"), qty("6")); !errors.Is(e, ErrConflict) {
		t.Fatal("stole another job's stock")
	}
	stock.Active = false
	if _, _, e = IssueEffect(stock, qty("3"), qty("4")); !errors.Is(e, ErrConflict) {
		t.Fatal("inactive extra material used")
	}
	if _, _, e = IssueEffect(stock, qty("3"), qty("2")); e != nil {
		t.Fatal("own inactive reservation cannot be consumed", e)
	}
}

type spy struct {
	Repository
	input Action
}

func (r *spy) ApplyProduction(_ context.Context, _ string, _ string, in Action) (State, error) {
	r.input = in
	cost := qty("50")
	amount, _ := q.ParseAmount("50")
	return State{Finance: &Finance{}, Usage: []Usage{{UnitCost: &cost, Total: &amount}}}, nil
}
func TestRolesAndRedaction(t *testing.T) {
	r := &spy{}
	s := NewService(r)
	ctx := context.Background()
	a := auth.Account{Active: true, Role: auth.RoleWorker}
	v, e := s.Complete(ctx, a, uuid.NewString(), Command{uuid.NewString(), 1})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(v)
	for _, field := range []string{"finance", "unit_cost", "total_amount"} {
		if strings.Contains(string(raw), field) {
			t.Fatal("worker financial leak")
		}
	}
	if _, e = s.Labor(ctx, a, "", LaborInput{}); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("worker set costs")
	}
	a.Role = auth.RoleAdmin
	if _, e = s.Complete(ctx, a, "", Command{}); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("Admin changed production")
	}
	a.Role = auth.RoleCustomer
	if _, e = s.Get(ctx, a, uuid.NewString()); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("customer production read")
	}
}
func TestValidationAndMoney(t *testing.T) {
	s := NewService(nil)
	ctx := context.Background()
	a := auth.Account{Active: true, Role: auth.RoleWorker}
	if _, e := s.Start(ctx, a, uuid.NewString(), StartInput{}); e == nil {
		t.Fatal("missing plan version")
	}
	if _, e := s.Time(ctx, a, uuid.NewString(), TimeInput{Minutes: 0}); e == nil {
		t.Fatal("zero minutes")
	}
	max, _ := q.ParseAmount("9999999999999999.99")
	one, _ := q.ParseAmount("0.01")
	if _, e := SumAmounts(max, one); e == nil {
		t.Fatal("money overflow")
	}
}

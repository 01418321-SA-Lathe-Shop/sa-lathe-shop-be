package procurement

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"testing"
)

func decimal(s string) *m.Decimal {
	v, err := m.ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return &v
}
func TestExactTotals(t *testing.T) {
	for _, tt := range []struct{ qty, cost, want string }{{"3", "0.335", "1.01"}, {"25", "0.335", "8.38"}, {"1", "0.004999", "0.00"}, {"1", "0.005", "0.01"}, {"0.000001", "0.000001", "0.00"}} {
		v, err := PriceLine(*decimal(tt.qty), *decimal(tt.cost))
		if err != nil || v.String() != tt.want {
			t.Fatalf("%+v: %s %v", tt, v.String(), err)
		}
	}
	_, total, err := Totals([]LineInput{{Quantity: decimal("3"), UnitCost: decimal("0.335")}, {Quantity: decimal("3"), UnitCost: decimal("0.335")}})
	if err != nil || total.String() != "2.02" {
		t.Fatalf("sum rounded lines: %s %v", total.String(), err)
	}
	if _, _, err = Totals([]LineInput{{Quantity: decimal("999999999999"), UnitCost: decimal("999999999999")}}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, _, err = Totals([]LineInput{{Quantity: decimal("0"), UnitCost: decimal("1")}}); err == nil {
		t.Fatal("zero quantity accepted")
	}
}
func TestOnlyAdmin(t *testing.T) {
	s := NewService(nil)
	ctx := context.Background()
	for _, role := range []string{auth.RoleWorker, auth.RoleCustomer, ""} {
		a := auth.Account{Role: role, Active: true}
		checks := []func() error{func() error { _, e := s.Place(ctx, a, OrderInput{}); return e }, func() error { _, e := s.Act(ctx, a, "", ActionInput{}); return e }, func() error { _, e := s.Get(ctx, a, ""); return e }, func() error { _, e := s.Orders(ctx, a, "", 1, 0); return e }, func() error { _, e := s.Suppliers(ctx, a, "", 1, 0); return e }, func() error { _, e := s.CreateSupplier(ctx, a, SupplierInput{}); return e }, func() error { _, e := s.UpdateSupplier(ctx, a, "", SupplierUpdate{}); return e }}
		for _, check := range checks {
			if !errors.Is(check(), req.ErrForbidden) {
				t.Fatal("non-admin allowed", role)
			}
		}
	}
}
func TestContactsAndCommands(t *testing.T) {
	for _, in := range []SupplierInput{{}, {Name: "X", Email: "bad"}, {Name: "X", Email: "Display <a@example.com>"}, {Name: "X", Phone: "abc1234"}} {
		if _, err := contact(in); err == nil {
			t.Fatal("invalid contact", in)
		}
	}
	if _, err := contact(SupplierInput{Name: "Supplier", Phone: "+66812345678", Email: "s@example.com"}); err != nil {
		t.Fatal(err)
	}
	s := NewService(nil)
	a := auth.Account{Role: auth.RoleAdmin, Active: true}
	for _, action := range []string{"reject", "cancel", "edit"} {
		if _, err := s.Act(context.Background(), a, uuid.NewString(), ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Action: action}); !errors.Is(err, req.ErrInvalidInput) {
			t.Fatal("invalid action accepted")
		}
	}
}

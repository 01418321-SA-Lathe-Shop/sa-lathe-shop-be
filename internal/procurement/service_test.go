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
	if _, _, err = Totals([]LineInput{{Quantity: decimal("999999999999"), UnitCost: decimal("9999")}, {Quantity: decimal("999999999999"), UnitCost: decimal("9999")}}); err == nil {
		t.Fatal("sum overflow accepted")
	}
	if _, _, err = Totals([]LineInput{{Quantity: decimal("0"), UnitCost: decimal("1")}}); err == nil {
		t.Fatal("zero quantity accepted")
	}
}
func TestRoleGates(t *testing.T) {
	s := NewService(nil)
	ctx := context.Background()
	ordering := func(a auth.Account) []func() error {
		return []func() error{func() error { _, e := s.Place(ctx, a, OrderInput{}); return e }, func() error { _, e := s.Act(ctx, a, "", ActionInput{}); return e }, func() error { _, e := s.Get(ctx, a, ""); return e }, func() error { _, e := s.Suppliers(ctx, a, "", 0, 0); return e }, func() error { _, e := s.Orders(ctx, a, "bad", 1, 0); return e }, func() error { _, e := s.RecommendSuppliers(ctx, a, "bad", ""); return e }}
	}
	suppliers := func(a auth.Account) []func() error {
		return []func() error{func() error { _, e := s.CreateSupplier(ctx, a, SupplierInput{}); return e }, func() error { _, e := s.UpdateSupplier(ctx, a, "", SupplierUpdate{}); return e }}
	}
	for _, a := range []auth.Account{{Role: auth.RoleCustomer, Active: true}, {Role: "", Active: true}, {Role: auth.RoleWorker}, {Role: auth.RoleAdmin}} {
		for _, check := range append(ordering(a), suppliers(a)...) {
			if !errors.Is(check(), req.ErrForbidden) {
				t.Fatal("forbidden account allowed", a.Role, a.Active)
			}
		}
	}
	// UC6: an active Worker (Technician) places and manages orders; supplier master data is Admin-only.
	for _, role := range []string{auth.RoleWorker, auth.RoleAdmin} {
		a := auth.Account{Role: role, Active: true}
		for _, check := range ordering(a) {
			if !errors.Is(check(), req.ErrInvalidInput) {
				t.Fatal("staff should pass the ordering role gate", role)
			}
		}
	}
	worker := auth.Account{Role: auth.RoleWorker, Active: true}
	for _, check := range suppliers(worker) {
		if !errors.Is(check(), req.ErrForbidden) {
			t.Fatal("worker must not manage suppliers")
		}
	}
	if _, err := s.CreateSupplier(ctx, auth.Account{Role: auth.RoleAdmin, Active: true}, SupplierInput{}); !errors.Is(err, req.ErrInvalidInput) {
		t.Fatal("admin should pass supplier gate", err)
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

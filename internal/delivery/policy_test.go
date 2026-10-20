package delivery

import (
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"testing"
)

func money(s string) q.Amount {
	v, e := q.ParseAmount(s)
	if e != nil {
		panic(e)
	}
	return v
}
func TestBalancesAndClosure(t *testing.T) {
	v := State{RequestStatus: "ongoing", JobStatus: "completed", Invoice: &Invoice{Amount: money("100.01"), Balance: money("100.01")}, Deliveries: []Shipment{{Status: "delivered"}}}
	if ShouldClose(v) {
		t.Fatal("unpaid closed")
	}
	left, e := Remaining(v.Invoice.Amount, money("60.01"))
	if e != nil || left.String() != "40.00" {
		t.Fatal(left, e)
	}
	a := money("100.02")
	if CanApply(v, Action{Kind: "bank", Amount: &a}) == nil {
		t.Fatal("overpaid")
	}
	v.Invoice.Balance = money("0")
	if !ShouldClose(v) {
		t.Fatal("paid accepted not closed")
	}
	v.Deliveries[0].Status = "returned"
	if ShouldClose(v) {
		t.Fatal("return closed")
	}
	v.Deliveries[0].Status = "shipping"
	if ShouldClose(v) {
		t.Fatal("payment implies acceptance")
	}
}
func TestLifecycle(t *testing.T) {
	days := 7
	v := State{RequestStatus: "ongoing", JobStatus: "completed", CreditDays: &days}
	if CanApply(v, Action{Kind: "invoice"}) != nil {
		t.Fatal("invoice rejected")
	}
	v.Invoice = &Invoice{}
	if CanApply(v, Action{Kind: "invoice"}) == nil {
		t.Fatal("second invoice")
	}
	v.Deliveries = []Shipment{{Status: "delivered"}}
	if CanApply(v, Action{Kind: "return"}) == nil || CanApply(v, Action{Kind: "ship"}) == nil {
		t.Fatal("accepted shipment changed")
	}
	v.Deliveries[0].Status = "returned"
	if CanApply(v, Action{Kind: "ship"}) != nil {
		t.Fatal("redelivery")
	}
	v.JobStatus = "in_progress"
	if CanApply(v, Action{Kind: "ship"}) == nil {
		t.Fatal("unfinished redelivery")
	}
}

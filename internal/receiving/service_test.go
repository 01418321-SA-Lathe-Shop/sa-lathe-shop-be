package receiving

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func qty(s string) *m.Decimal {
	v, e := m.ParseDecimal(s)
	if e != nil {
		panic(e)
	}
	return &v
}

type spy struct {
	Repository
	input ReceiptInput
}

func (r *spy) Receive(_ context.Context, _ string, _ string, in ReceiptInput) (State, error) {
	r.input = in
	return State{Finance: &Finance{PaymentStatus: "paid"}}, nil
}
func TestReceiptValidationAndRedaction(t *testing.T) {
	r := &spy{}
	s := NewService(r)
	a := auth.Account{ID: uuid.NewString(), Role: auth.RoleWorker, Active: true}
	order := uuid.NewString()
	in := ReceiptInput{Command: Command{uuid.NewString(), 1}, Lines: []ReceiptLine{{MaterialID: uuid.NewString(), Good: qty("1"), Damaged: qty("2")}}}
	v, e := s.Receive(context.Background(), a, order, in)
	if e != nil || v.Finance != nil {
		t.Fatal("worker finance leak", e)
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "finance") {
		t.Fatal("finance serialized")
	}
	in.Lines[0].Good = qty("0")
	in.Lines[0].Damaged = qty("0")
	if _, e = s.Receive(context.Background(), a, order, in); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("empty delivery accepted")
	}
	in.Lines[0].Good = qty("1")
	in.Lines = append(in.Lines, in.Lines[0])
	if _, e = s.Receive(context.Background(), a, order, in); e == nil {
		t.Fatal("duplicate line accepted")
	}
}
func TestFinancialPermissions(t *testing.T) {
	s := NewService(nil)
	ctx := context.Background()
	a := auth.Account{Role: auth.RoleWorker, Active: true}
	if _, e := s.Credit(ctx, a, "", CreditInput{}); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("worker credit access")
	}
	if _, e := s.Settle(ctx, a, "", SettlementInput{}); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("worker payment access")
	}
	a.Role = auth.RoleCustomer
	if _, e := s.Get(ctx, a, uuid.NewString()); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("customer receiving access")
	}
	a.Role = auth.RoleAdmin
	if _, e := s.Settle(ctx, a, uuid.NewString(), SettlementInput{Command: Command{uuid.NewString(), 1}, Method: "bank"}); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("bank proof missing")
	}
}

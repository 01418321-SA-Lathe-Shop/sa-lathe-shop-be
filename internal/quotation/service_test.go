package quotation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"strings"
	"testing"
)

const testID = "57039d8b-c58e-4ee1-8943-539dc6203626"

type fakeRepo struct {
	calls int
	q     Quotation
	err   error
}

func (f *fakeRepo) CreateDraft(_ context.Context, _ string, d Draft) (Quotation, error) {
	f.calls++
	f.q = Quotation{ID: d.ID, RequestID: d.RequestID, Status: "draft", MaterialCost: d.MaterialCost, LaborCost: d.LaborCost, FinalPrice: d.FinalPrice}
	return f.q, f.err
}
func (f *fakeRepo) UpdateDraft(context.Context, string, Draft) (Quotation, error) {
	f.calls++
	return f.q, f.err
}
func (f *fakeRepo) SendQuotation(context.Context, string, string, int64) (Quotation, error) {
	f.calls++
	return f.q, f.err
}
func (f *fakeRepo) RejectRequest(context.Context, string, string, string) error {
	f.calls++
	return f.err
}
func (f *fakeRepo) GetQuotation(context.Context, string, string) (Quotation, error) {
	f.calls++
	return f.q, f.err
}
func (f *fakeRepo) ListQuotations(context.Context, string, string, int, int) ([]Quotation, error) {
	f.calls++
	return []Quotation{f.q}, f.err
}
func (f *fakeRepo) GetRequestRejection(context.Context, string, string) (Rejection, error) {
	f.calls++
	return Rejection{}, f.err
}
func worker() auth.Account { return auth.Account{ID: testID, Active: true, Role: auth.RoleWorker} }
func input(t *testing.T, price string) DraftInput {
	m, l, p := amount(t, "800"), amount(t, "200"), amount(t, price)
	return DraftInput{MaterialCost: &m, LaborCost: &l, FinalPrice: &p}
}
func TestDraftPriceAndPermissions(t *testing.T) {
	f := &fakeRepo{}
	s := NewService(f)
	if _, err := s.Create(context.Background(), worker(), testID, input(t, "1599.99")); !errors.Is(err, ErrInvalidInput) || f.calls != 0 {
		t.Fatal("below minimum accepted")
	}
	q, err := s.Create(context.Background(), worker(), testID, input(t, "1600"))
	if err != nil || q.Costs == nil || q.Costs.MinimumPrice.String() != "1600.00" {
		t.Fatal(err)
	}
	for _, role := range []string{auth.RoleAdmin, auth.RoleCustomer, ""} {
		a := worker()
		a.Role = role
		calls := f.calls
		_, err = s.Create(context.Background(), a, testID, input(t, "1600"))
		if !errors.Is(err, ErrForbidden) || f.calls != calls {
			t.Fatal("non-worker wrote quotation")
		}
	}
}
func TestCustomerCostRedaction(t *testing.T) {
	f := &fakeRepo{q: Quotation{Status: "sent", MaterialCost: amount(t, "800"), LaborCost: amount(t, "200"), FinalPrice: amount(t, "1600"), Costs: &Estimate{}}}
	a := worker()
	a.Role = auth.RoleCustomer
	q, err := NewService(f).Get(context.Background(), a, testID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(q)
	for _, private := range []string{"material_cost", "labor_cost", "internal_costs", "minimum_price", "total_cost"} {
		if strings.Contains(string(body), private) {
			t.Fatalf("leaked %s", private)
		}
	}
	f.q.Status = "draft"
	if _, err = NewService(f).Get(context.Background(), a, testID); !errors.Is(err, ErrNotFound) {
		t.Fatal("customer saw draft")
	}
}
func TestInvalidEditSendReject(t *testing.T) {
	f := &fakeRepo{}
	s := NewService(f)
	ctx := context.Background()
	if _, err := s.Edit(ctx, worker(), testID, EditInput{DraftInput: input(t, "1600")}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err := s.Send(ctx, worker(), testID, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if err := s.Reject(ctx, worker(), testID, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if f.calls != 0 {
		t.Fatal("invalid commands reached repository")
	}
}
func TestDraftDocumentFields(t *testing.T) {
	f := &fakeRepo{}
	s := NewService(f)
	in := input(t, "1600")
	bad := "31/12/2026"
	in.ValidUntil = &bad
	if _, err := s.Create(context.Background(), worker(), testID, in); !errors.Is(err, ErrInvalidInput) || f.calls != 0 {
		t.Fatal("bad valid_until accepted")
	}
	good := "2026-12-31"
	in.ValidUntil, in.Notes = &good, "  ราคานี้ไม่รวมค่าขนส่ง  "
	if _, err := s.Create(context.Background(), worker(), testID, in); err != nil {
		t.Fatal(err)
	}
	in.Notes = strings.Repeat("ก", 2001)
	if _, err := s.Create(context.Background(), worker(), testID, in); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("long notes accepted")
	}
}

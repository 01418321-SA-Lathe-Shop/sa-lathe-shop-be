package quotation

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"testing"
	"time"
)

type decisionFake struct {
	calls int
	in    DecisionInput
	value Decision
	err   error
}

func (f *decisionFake) DecideQuotation(_ context.Context, _, _ string, in DecisionInput) (Decision, error) {
	f.calls++
	f.in = in
	return f.value, f.err
}
func (f *decisionFake) GetDecision(context.Context, string, string) (Decision, error) {
	f.calls++
	return f.value, f.err
}
func (f *decisionFake) ListDecisions(context.Context, string, string, int, int) ([]Decision, error) {
	f.calls++
	return []Decision{f.value}, f.err
}
func TestAcceptanceTerms(t *testing.T) {
	accepted := time.Date(2026, 10, 16, 10, 30, 0, 0, time.FixedZone("Bangkok", 7*3600))
	for _, test := range []struct {
		kind, price, deposit, balance string
		po, required                  bool
	}{
		{"individual", "1600.01", "800.01", "800.00", false, true},
		{"individual", "0.01", "0.01", "0.00", false, true},
		{"company", "9999.99", "0.00", "9999.99", true, false},
		{"company", "10000.00", "0.00", "10000.00", true, false},
		{"company", "10000.01", "5000.01", "5000.00", true, true},
		{"company", "9999999999999999.99", "5000000000000000.00", "4999999999999999.99", true, true},
	} {
		v, err := AcceptanceTerms(test.kind, amount(t, test.price), accepted)
		if err != nil || v.DepositAmount.String() != test.deposit || v.BalanceAfterDeposit.String() != test.balance || v.PORequired != test.po || v.DepositRequired != test.required || v.DocumentsDueAt.Sub(v.AcceptedAt) != 336*time.Hour || v.AcceptedAt.Location() != time.UTC {
			t.Fatalf("invalid terms for %s/%s: %v", test.kind, test.price, err)
		}
	}
	if _, err := AcceptanceTerms("supplier", Amount{}, accepted); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("unknown customer type")
	}
	if _, err := AcceptanceTerms("individual", Amount{}, time.Time{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("zero acceptance date")
	}
}
func TestDecisionPermissionAndValidation(t *testing.T) {
	repo := &decisionFake{}
	service := NewDecisionService(repo)
	a := auth.Account{ID: testID, Active: true, Role: auth.RoleCustomer}
	for _, in := range []DecisionInput{{Outcome: "approve", ExpectedLockVersion: 1}, {Outcome: "accept", ExpectedLockVersion: 0}, {Outcome: "accept", Reason: "extra", ExpectedLockVersion: 1}, {Outcome: "reject", Reason: "bad\x00text", ExpectedLockVersion: 1}} {
		if _, err := service.Decide(context.Background(), a, testID, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatal(err)
		}
	}
	for _, role := range []string{auth.RoleAdmin, auth.RoleWorker, ""} {
		actor := a
		actor.Role = role
		if _, err := service.Decide(context.Background(), actor, testID, DecisionInput{Outcome: "accept", ExpectedLockVersion: 1}); !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid decisions reached repository")
	}
	_, err := service.Decide(context.Background(), a, testID, DecisionInput{Outcome: "reject", Reason: "  price  ", ExpectedLockVersion: 2})
	if err != nil || repo.in.Reason != "price" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := repo.calls
	if _, err = service.Decide(ctx, a, testID, DecisionInput{Outcome: "accept", ExpectedLockVersion: 2}); !errors.Is(err, context.Canceled) || repo.calls != calls {
		t.Fatal("cancelled decision reached repository")
	}
}

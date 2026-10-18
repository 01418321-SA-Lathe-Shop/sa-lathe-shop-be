package payment

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestSubmissionAmounts(t *testing.T) {
	price, _ := q.ParseAmount("1600.01")
	terms, _ := q.AcceptanceTerms("individual", price, time.Now())
	now := time.Now()
	in := Submission{IdempotencyKey: uuid.NewString(), AttachmentID: uuid.NewString(), Kind: "deposit", PaidAt: &now}
	for _, test := range []struct {
		amount string
		ok     bool
	}{{"800.00", false}, {"800.01", true}, {"1600.01", true}, {"1600.02", false}, {"0.00", false}} {
		amount, _ := q.ParseAmount(test.amount)
		in.Amount = &amount
		if (ValidateSubmission(terms, in) == nil) != test.ok {
			t.Fatal(test)
		}
	}
	in.Amount = nil
	if ValidateSubmission(terms, in) == nil {
		t.Fatal("missing amount accepted")
	}
	company, _ := q.AcceptanceTerms("company", price, time.Now())
	in.Kind = "po"
	in.PaidAt = nil
	if ValidateSubmission(company, in) != nil {
		t.Fatal("company PO rejected")
	}
	if ValidateSubmission(terms, in) == nil {
		t.Fatal("individual PO accepted")
	}
	zero, _ := q.ParseAmount("0.00")
	zeroTerms, _ := q.AcceptanceTerms("individual", zero, time.Now())
	in.Kind = "deposit"
	in.Amount = &zero
	in.PaidAt = &now
	if ValidateSubmission(zeroTerms, in) != nil {
		t.Fatal("existing zero-price contract cannot be reviewed")
	}
}

type fakeRepository struct{ calls int }

func (f *fakeRepository) SubmitEvidence(context.Context, string, string, Submission) (Evidence, error) {
	f.calls++
	return Evidence{}, nil
}
func (f *fakeRepository) ReviewEvidence(context.Context, string, string, Review) (Evidence, error) {
	f.calls++
	return Evidence{}, nil
}
func (f *fakeRepository) GetFulfillment(context.Context, string, string) (Fulfillment, error) {
	f.calls++
	return Fulfillment{}, nil
}
func TestRolesAndReviewValidation(t *testing.T) {
	f := &fakeRepository{}
	s := NewService(f)
	id := uuid.NewString()
	ctx := context.Background()
	for _, role := range []string{auth.RoleAdmin, auth.RoleWorker} {
		a := auth.Account{ID: id, Role: role, Active: true}
		if _, err := s.Review(ctx, a, id, Review{Outcome: "approved", ExpectedVersion: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Submit(ctx, a, id, Submission{}); !errors.Is(err, req.ErrForbidden) {
			t.Fatal("staff submitted", err)
		}
	}
	customer := auth.Account{ID: id, Role: auth.RoleCustomer, Active: true}
	if _, err := s.Review(ctx, customer, id, Review{Outcome: "approved", ExpectedVersion: 1}); !errors.Is(err, req.ErrForbidden) {
		t.Fatal(err)
	}
	for _, in := range []Review{{Outcome: "rejected", ExpectedVersion: 1}, {Outcome: "approved", ExpectedVersion: 2}, {Outcome: "approved", ExpectedVersion: 1, Reason: "secret"}} {
		if ValidateReview(in) == nil {
			t.Fatal("bad review", in)
		}
	}
	if ValidateReview(Review{Outcome: "rejected", ExpectedVersion: 1, Reason: "wrong document"}) != nil {
		t.Fatal("reason rejected")
	}
}

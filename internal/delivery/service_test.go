package delivery

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"testing"
	"time"
)

type mockRepository struct {
	calls int
	last  Action
	err   error
}

func TestPersistedTextRejectsNULAndInvalidUTF8BeforeRepository(t *testing.T) {
	for _, value := range []string{"name\x00hidden", string([]byte{0xff})} {
		repository := &mockRepository{}
		service := NewService(repository)
		ctx := context.Background()
		job := uuid.NewString()
		command := Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1}
		_, shipErr := service.Ship(ctx, auth.Account{Active: true, Role: auth.RoleWorker}, job, ShipInput{Command: command, Method: "self", ReceiverName: value})
		_, inspectionErr := service.Inspect(ctx, auth.Account{Active: true, Role: auth.RoleCustomer}, job, InspectInput{Command: command, Outcome: "return", Reason: value})
		_, reviewErr := service.Review(ctx, auth.Account{Active: true, Role: auth.RoleAdmin}, job, ReviewInput{Command: command, PaymentID: uuid.NewString(), Outcome: "rejected", Reason: value})
		for _, err := range []error{shipErr, inspectionErr, reviewErr} {
			if !errors.Is(err, req.ErrInvalidInput) {
				t.Fatal("incompatible PostgreSQL text was not rejected as invalid input")
			}
		}
		if repository.calls != 0 {
			t.Fatal("invalid text reached repository")
		}
	}
}

func (m *mockRepository) FulfillmentState(context.Context, string, string) (State, error) {
	m.calls++
	return State{Payments: []Payment{{ID: uuid.NewString()}}}, m.err
}
func (m *mockRepository) FulfillmentJobs(context.Context, string, int, int) ([]State, error) {
	return nil, m.err
}
func (m *mockRepository) ApplyFulfillment(_ context.Context, _, _ string, in Action) (State, error) {
	m.calls++
	m.last = in
	return State{}, m.err
}
func TestServiceValidationAndRoleGuards(t *testing.T) {
	ctx := context.Background()
	r := &mockRepository{}
	s := NewService(r)
	job := uuid.NewString()
	worker := auth.Account{ID: uuid.NewString(), Active: true, Role: "worker"}
	admin := auth.Account{ID: uuid.NewString(), Active: true, Role: "admin"}
	customer := auth.Account{ID: uuid.NewString(), Active: true, Role: "customer"}
	command := Command{IdempotencyKey: uuid.NewString()}
	days := 7
	if _, e := s.Terms(ctx, worker, job, TermsInput{Command: command, CreditDays: &days}); e == nil {
		t.Fatal("worker terms")
	}
	if _, e := s.Inspect(ctx, admin, job, InspectInput{Command: command, Outcome: "accept"}); e == nil {
		t.Fatal("staff proxy")
	}
	if _, e := s.Inspect(ctx, customer, job, InspectInput{Command: command, Outcome: "return"}); e == nil {
		t.Fatal("reason required")
	}
	future := time.Now().Add(time.Hour)
	a := money("1")
	if _, e := s.Pay(ctx, customer, job, PaymentInput{Command: command, Amount: &a, PaidAt: &future, EvidenceID: uuid.NewString()}, false); e == nil {
		t.Fatal("future payment")
	}
	if r.calls != 0 {
		t.Fatal("invalid command reached DB")
	}
	_, e := s.Terms(ctx, admin, job, TermsInput{Command: command, CreditDays: &days})
	if e != nil || r.last.Kind != "terms" {
		t.Fatal("valid terms", e)
	}
	v, e := s.Get(ctx, worker, job)
	if e != nil || len(v.Payments) != 0 {
		t.Fatal("worker redaction")
	}
	r.err = errors.New("repository failure")
	if _, e = s.Issue(ctx, worker, job, command); !errors.Is(e, r.err) {
		t.Fatal("repository error lost")
	}
}

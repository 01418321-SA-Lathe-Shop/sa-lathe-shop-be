package delivery

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/validation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"strings"
	"time"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{r} }
func validID(id string) bool {
	v, e := uuid.Parse(id)
	return e == nil && v != uuid.Nil && v.String() == id
}
func permitted(a auth.Account, role string) error {
	if !a.Active {
		return req.ErrForbidden
	}
	if role == "any" && (a.Role == "admin" || a.Role == "worker" || a.Role == "customer") {
		return nil
	}
	if role == "staff" && (a.Role == "admin" || a.Role == "worker") {
		return nil
	}
	if a.Role != role {
		return req.ErrForbidden
	}
	return nil
}
func (s *Service) Get(ctx context.Context, a auth.Account, id string) (State, error) {
	if e := permitted(a, "any"); e != nil {
		return State{}, e
	}
	if !validID(id) {
		return State{}, req.ErrInvalidInput
	}
	v, e := s.repo.FulfillmentState(ctx, a.ID, id)
	return v, e
}
func (s *Service) List(ctx context.Context, a auth.Account, limit, offset int) ([]State, error) {
	if e := permitted(a, "any"); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, req.ErrInvalidInput
	}
	v, e := s.repo.FulfillmentJobs(ctx, a.ID, limit, offset)
	return v, e
}
func (s *Service) apply(ctx context.Context, a auth.Account, id, role string, in Action) (State, error) {
	if e := permitted(a, role); e != nil {
		return State{}, e
	}
	if !validID(id) || !validID(in.IdempotencyKey) || in.ExpectedVersion < 0 {
		return State{}, req.ErrInvalidInput
	}
	v, e := s.repo.ApplyFulfillment(ctx, a.ID, id, in)
	return v, e
}
func (s *Service) Terms(ctx context.Context, a auth.Account, id string, in TermsInput) (State, error) {
	if in.CreditDays == nil || *in.CreditDays < 0 || *in.CreditDays > 36500 {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, "staff", Action{Command: in.Command, Kind: "terms", CreditDays: in.CreditDays})
}
func (s *Service) Issue(ctx context.Context, a auth.Account, id string, in Command) (State, error) {
	return s.apply(ctx, a, id, "staff", Action{Command: in, Kind: "invoice"})
}
func (s *Service) Ship(ctx context.Context, a auth.Account, id string, in ShipInput) (State, error) {
	in.ReceiverName = strings.TrimSpace(in.ReceiverName)
	if (in.Method != "self" && in.Method != "third_party" && in.Method != "pickup") || !validation.Text(in.ReceiverName, 1, 200) {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, "staff", Action{Command: in.Command, Kind: "ship", Method: in.Method, ReceiverName: in.ReceiverName})
}
func (s *Service) Inspect(ctx context.Context, a auth.Account, id string, in InspectInput) (State, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if (in.Outcome != "accept" && in.Outcome != "return") || (in.Outcome == "return" && in.Reason == "") || (in.Outcome == "accept" && in.Reason != "") || !validation.Text(in.Reason, 0, 2000) {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, "customer", Action{Command: in.Command, Kind: in.Outcome, Reason: in.Reason})
}
func (s *Service) Pay(ctx context.Context, a auth.Account, id string, in PaymentInput, cash bool) (State, error) {
	if in.Amount == nil || in.Amount.String() == "0.00" || in.PaidAt == nil || in.PaidAt.After(time.Now().UTC()) || (!cash && !validID(in.EvidenceID)) || (in.EvidenceID != "" && !validID(in.EvidenceID)) {
		return State{}, req.ErrInvalidInput
	}
	kind, role := "bank", "customer"
	if cash {
		kind, role = "cash", "staff"
	}
	t := in.PaidAt.UTC()
	return s.apply(ctx, a, id, role, Action{Command: in.Command, Kind: kind, Amount: in.Amount, PaidAt: &t, EvidenceID: in.EvidenceID})
}
func (s *Service) Review(ctx context.Context, a auth.Account, id string, in ReviewInput) (State, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.PaymentID) || (in.Outcome != "approved" && in.Outcome != "rejected") || (in.Outcome == "rejected" && in.Reason == "") || (in.Outcome == "approved" && in.Reason != "") || !validation.Text(in.Reason, 0, 2000) {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, "staff", Action{Command: in.Command, Kind: "review", PaymentID: in.PaymentID, Outcome: in.Outcome, Reason: in.Reason})
}

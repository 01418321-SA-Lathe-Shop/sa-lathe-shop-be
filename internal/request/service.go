package request

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/validation"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// Defensive API bounds, not business order-quantity limits.
	MaxDescriptionCharacters  = 10000
	MaxMaterialSpecCharacters = 2000
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func validPrincipal(p Principal) bool {
	return p.Active && p.Role == "customer" && validID(p.AccountID)
}

func validID(id string) bool {
	_, err := uuid.Parse(id)
	return len(id) == 36 && err == nil
}

func (s *Service) Submit(ctx context.Context, p Principal, input Input) (Request, error) {
	if !validPrincipal(p) {
		return Request{}, ErrForbidden
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	input.Description = strings.TrimSpace(input.Description)
	input.MaterialSpec = strings.TrimSpace(input.MaterialSpec)
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "net"
	}
	if source != "net" && source != "phone" && source != "line" {
		return Request{}, ErrInvalidInput
	}
	input.Source = source
	if !validID(input.CustomerID) || !validation.Text(input.Description, 1, MaxDescriptionCharacters) ||
		!validation.Text(input.MaterialSpec, 0, MaxMaterialSpecCharacters) || input.Quantity <= 0 {
		return Request{}, ErrInvalidInput
	}
	now := s.now().UTC()
	if input.Deadline != nil {
		if !input.Deadline.After(now) || input.Deadline.Year() > 9999 {
			return Request{}, ErrInvalidInput
		}
		deadline := input.Deadline.UTC()
		input.Deadline = &deadline
	}
	// A customer's requested deadline is not a guaranteed production delivery date.
	// Quotation/Deposit/PO deadlines belong to later use cases.
	value := Request{ID: uuid.NewString(), CreatedBy: p.AccountID, Input: input,
		Source: source, Status: "pending", CreatedAt: now}
	return s.repo.Create(ctx, value)
}

func (s *Service) Get(ctx context.Context, p Principal, id string) (Request, error) {
	if !validReader(p) {
		return Request{}, ErrForbidden
	}
	if !validID(id) {
		return Request{}, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	return s.repo.GetOwned(ctx, p.AccountID, id)
}

func validReader(p Principal) bool {
	return p.Active && validID(p.AccountID) && (p.Role == "customer" || p.Role == "worker" || p.Role == "admin")
}
func (s *Service) List(ctx context.Context, p Principal, limit, offset int) ([]Request, error) {
	if !validReader(p) {
		return nil, ErrForbidden
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalidInput
	}
	return s.repo.ListRequests(ctx, p.AccountID, limit, offset)
}

// RollbackPending deletes a newly created pending request when initial attachment upload fails (UC1 Except-Step 5 / Q1.E3).
func (s *Service) RollbackPending(ctx context.Context, p Principal, id string) error {
	if !validPrincipal(p) {
		return ErrForbidden
	}
	if !validID(id) {
		return ErrInvalidInput
	}
	opt, ok := s.repo.(OptionalRepository)
	if !ok {
		return ErrForbidden
	}
	return opt.DeletePending(ctx, p.AccountID, id)
}

// RequestInfo records a Worker request for additional customer details (UC2 Alt-Step 5).
func (s *Service) RequestInfo(ctx context.Context, p Principal, id, note string) (Request, error) {
	if !p.Active || !validID(p.AccountID) || p.Role != "worker" {
		return Request{}, ErrForbidden
	}
	note = strings.TrimSpace(note)
	if !validID(id) || !validation.Text(note, 1, 2000) {
		return Request{}, ErrInvalidInput
	}
	opt, ok := s.repo.(OptionalRepository)
	if !ok {
		return Request{}, ErrForbidden
	}
	return opt.RequestInfo(ctx, p.AccountID, id, note)
}

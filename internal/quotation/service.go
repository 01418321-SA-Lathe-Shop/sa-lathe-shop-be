package quotation

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/google/uuid"
	"strings"
	"unicode/utf8"
)

func validID(id string) bool { _, err := uuid.Parse(id); return len(id) == 36 && err == nil }
func inputDraft(in DraftInput) (Draft, error) {
	if in.MaterialCost == nil || in.LaborCost == nil || in.FinalPrice == nil {
		return Draft{}, ErrInvalidInput
	}
	estimate, err := EstimatePrice(*in.MaterialCost, *in.LaborCost)
	if err != nil {
		return Draft{}, err
	}
	if in.FinalPrice.satang < estimate.MinimumPrice.satang {
		return Draft{}, ErrInvalidInput
	}
	return Draft{MaterialCost: *in.MaterialCost, LaborCost: *in.LaborCost, FinalPrice: *in.FinalPrice}, nil
}
func staffView(q Quotation, role string) (Quotation, error) {
	q.Costs = nil
	if role == auth.RoleWorker || role == auth.RoleAdmin {
		estimate, err := EstimatePrice(q.MaterialCost, q.LaborCost)
		if err != nil {
			return Quotation{}, err
		}
		q.Costs = &estimate
	}
	return q, nil
}
func (s *Service) Estimate(ctx context.Context, a auth.Account, in DraftInput) (Estimate, error) {
	if !isWorker(a) {
		return Estimate{}, ErrForbidden
	}
	if err := ctx.Err(); err != nil {
		return Estimate{}, err
	}
	if in.MaterialCost == nil || in.LaborCost == nil || in.FinalPrice != nil {
		return Estimate{}, ErrInvalidInput
	}
	return EstimatePrice(*in.MaterialCost, *in.LaborCost)
}
func (s *Service) Create(ctx context.Context, a auth.Account, requestID string, in DraftInput) (Quotation, error) {
	if !isWorker(a) {
		return Quotation{}, ErrForbidden
	}
	if !validID(requestID) {
		return Quotation{}, ErrInvalidInput
	}
	d, err := inputDraft(in)
	if err != nil {
		return Quotation{}, err
	}
	d.ID = uuid.NewString()
	d.RequestID = requestID
	q, err := s.repo.CreateDraft(ctx, a.ID, d)
	if err != nil {
		return Quotation{}, err
	}
	return staffView(q, a.Role)
}
func (s *Service) Edit(ctx context.Context, a auth.Account, id string, in EditInput) (Quotation, error) {
	if !isWorker(a) {
		return Quotation{}, ErrForbidden
	}
	if !validID(id) || in.ExpectedLockVersion < 1 {
		return Quotation{}, ErrInvalidInput
	}
	d, err := inputDraft(in.DraftInput)
	if err != nil {
		return Quotation{}, err
	}
	d.ID = id
	d.ExpectedLockVersion = in.ExpectedLockVersion
	q, err := s.repo.UpdateDraft(ctx, a.ID, d)
	if err != nil {
		return Quotation{}, err
	}
	return staffView(q, a.Role)
}
func (s *Service) Send(ctx context.Context, a auth.Account, id string, expected int64) (Quotation, error) {
	if !isWorker(a) {
		return Quotation{}, ErrForbidden
	}
	if !validID(id) || expected < 1 {
		return Quotation{}, ErrInvalidInput
	}
	q, err := s.repo.SendQuotation(ctx, a.ID, id, expected)
	if err != nil {
		return Quotation{}, err
	}
	return staffView(q, a.Role)
}
func (s *Service) Reject(ctx context.Context, a auth.Account, requestID, reason string) error {
	if !isWorker(a) {
		return ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if !validID(requestID) || !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) || reason == "" || utf8.RuneCountInString(reason) > 2000 {
		return ErrInvalidInput
	}
	return s.repo.RejectRequest(ctx, a.ID, requestID, reason)
}
func reader(a auth.Account) bool {
	return a.Active && validID(a.ID) && (a.Role == auth.RoleWorker || a.Role == auth.RoleAdmin || a.Role == auth.RoleCustomer)
}
func (s *Service) Get(ctx context.Context, a auth.Account, id string) (Quotation, error) {
	if !reader(a) {
		return Quotation{}, ErrForbidden
	}
	if !validID(id) {
		return Quotation{}, ErrInvalidInput
	}
	q, err := s.repo.GetQuotation(ctx, a.ID, id)
	if err != nil {
		return Quotation{}, err
	}
	if a.Role == auth.RoleCustomer && q.Status == "draft" {
		return Quotation{}, ErrNotFound
	}
	return staffView(q, a.Role)
}
func (s *Service) List(ctx context.Context, a auth.Account, requestID string, limit, offset int) ([]Quotation, error) {
	if !reader(a) {
		return nil, ErrForbidden
	}
	if !validID(requestID) || limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalidInput
	}
	items, err := s.repo.ListQuotations(ctx, a.ID, requestID, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]Quotation, 0, len(items))
	for _, q := range items {
		if a.Role == auth.RoleCustomer && q.Status == "draft" {
			continue
		}
		view, err := staffView(q, a.Role)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}
func (s *Service) Rejection(ctx context.Context, a auth.Account, requestID string) (Rejection, error) {
	if !reader(a) {
		return Rejection{}, ErrForbidden
	}
	if !validID(requestID) {
		return Rejection{}, ErrInvalidInput
	}
	return s.repo.GetRequestRejection(ctx, a.ID, requestID)
}

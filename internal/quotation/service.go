package quotation

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode/utf8"
)

func validID(id string) bool { _, err := uuid.Parse(id); return len(id) == 36 && err == nil }
func inputDraft(in DraftInput) (Draft, error) {
	var mat, lab, fin Amount
	// M-03 (UC2 Q2.8): Allow saving an initial un-priced draft when all cost fields are omitted or zero.
	allOmitted := in.MaterialCost == nil && in.LaborCost == nil && in.FinalPrice == nil
	if !allOmitted {
		if in.MaterialCost == nil || in.LaborCost == nil || in.FinalPrice == nil {
			return Draft{}, ErrInvalidInput
		}
		mat, lab, fin = *in.MaterialCost, *in.LaborCost, *in.FinalPrice
		allZero := mat.satang == 0 && lab.satang == 0 && fin.satang == 0
		if !allZero {
			// H-03 (UC2 Step 7.1): Both material_cost and labor_cost must be strictly positive (> 0).
			if mat.satang <= 0 || lab.satang <= 0 {
				return Draft{}, ErrInvalidInput
			}
			estimate, err := EstimatePrice(mat, lab)
			if err != nil {
				return Draft{}, err
			}
			if fin.satang < estimate.MinimumPrice.satang {
				return Draft{}, ErrInvalidInput
			}
		}
	}
	notes := strings.TrimSpace(in.Notes)
	if !utf8.ValidString(notes) || strings.ContainsRune(notes, 0) || utf8.RuneCountInString(notes) > 2000 {
		return Draft{}, ErrInvalidInput
	}
	if in.ValidUntil != nil {
		if _, err := time.Parse(time.DateOnly, *in.ValidUntil); err != nil {
			return Draft{}, ErrInvalidInput
		}
	}
	return Draft{MaterialCost: mat, LaborCost: lab, FinalPrice: fin, ValidUntil: in.ValidUntil, Notes: notes}, nil
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
	// H-03 (UC2 Step 7.1): material_cost > 0 and labor_cost > 0 required.
	if in.MaterialCost == nil || in.LaborCost == nil || in.FinalPrice != nil || in.MaterialCost.satang <= 0 || in.LaborCost.satang <= 0 {
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

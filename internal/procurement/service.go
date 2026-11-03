package procurement

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{r} }
func staff(a auth.Account) error {
	if !a.Active || (a.Role != auth.RoleAdmin && a.Role != auth.RoleWorker) {
		return req.ErrForbidden
	}
	return nil
}
func validID(s string) bool {
	v, err := uuid.Parse(s)
	return err == nil && len(s) == 36 && v != uuid.Nil
}
func text(s string, n int, required bool) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && (!required || s != "") && !strings.ContainsAny(s, "\x00\r\n")
}
func contact(in SupplierInput) (SupplierInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)
	if !text(in.Name, 200, true) || !text(in.Email, 254, false) || !text(in.Phone, 32, false) {
		return in, req.ErrInvalidInput
	}
	if in.Email != "" {
		v, err := mail.ParseAddress(in.Email)
		if err != nil || v.Address != in.Email {
			return in, req.ErrInvalidInput
		}
	}
	if in.Phone != "" {
		digits := strings.TrimPrefix(in.Phone, "+")
		if len(digits) < 7 || len(digits) > 15 {
			return in, req.ErrInvalidInput
		}
		for _, c := range digits {
			if c < '0' || c > '9' {
				return in, req.ErrInvalidInput
			}
		}
	}
	return in, nil
}
func (s *Service) CreateSupplier(ctx context.Context, a auth.Account, in SupplierInput) (Supplier, error) {
	if err := staff(a); err != nil {
		return Supplier{}, err
	}
	v, err := contact(in)
	if err != nil {
		return Supplier{}, err
	}
	return s.repo.CreateSupplier(ctx, a.ID, v)
}
func (s *Service) UpdateSupplier(ctx context.Context, a auth.Account, id string, in SupplierUpdate) (Supplier, error) {
	if err := staff(a); err != nil {
		return Supplier{}, err
	}
	if !validID(id) || in.ExpectedVersion < 1 || in.Active == nil {
		return Supplier{}, req.ErrInvalidInput
	}
	v, err := contact(in.SupplierInput)
	if err != nil {
		return Supplier{}, err
	}
	in.SupplierInput = v
	return s.repo.UpdateSupplier(ctx, a.ID, strings.ToLower(id), in)
}
func page(term string, limit, offset int) error {
	if !text(term, 200, false) || limit < 1 || limit > 100 || offset < 0 {
		return req.ErrInvalidInput
	}
	return nil
}
func (s *Service) Suppliers(ctx context.Context, a auth.Account, term string, limit, offset int) ([]Supplier, error) {
	if err := staff(a); err != nil {
		return nil, err
	}
	term = strings.TrimSpace(term)
	if err := page(term, limit, offset); err != nil {
		return nil, err
	}
	return s.repo.ListSuppliers(ctx, a.ID, term, limit, offset)
}
func (s *Service) RecommendSuppliers(ctx context.Context, a auth.Account, materialID, excludeSupplierID string) ([]SupplierRecommendation, error) {
	if err := staff(a); err != nil {
		return nil, err
	}
	materialID = strings.TrimSpace(materialID)
	excludeSupplierID = strings.TrimSpace(excludeSupplierID)
	if materialID != "" && !validID(materialID) {
		return nil, req.ErrInvalidInput
	}
	if excludeSupplierID != "" && !validID(excludeSupplierID) {
		return nil, req.ErrInvalidInput
	}
	return s.repo.RecommendSuppliers(ctx, a.ID, strings.ToLower(materialID), strings.ToLower(excludeSupplierID))
}
func (s *Service) Shortages(ctx context.Context, a auth.Account) ([]ShortageItem, error) {
	if err := staff(a); err != nil {
		return nil, err
	}
	return s.repo.ListShortages(ctx, a.ID)
}
func (s *Service) Place(ctx context.Context, a auth.Account, in OrderInput) (Order, error) {
	if err := staff(a); err != nil {
		return Order{}, err
	}
	if !validID(in.IdempotencyKey) || !validID(in.SupplierID) || len(in.Lines) < 1 || len(in.Lines) > 100 || len(in.JobIDs) > 100 {
		return Order{}, req.ErrInvalidInput
	}
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	in.SupplierID = strings.ToLower(in.SupplierID)
	in.JobIDs = append([]string{}, in.JobIDs...)
	seen := map[string]bool{}
	for i, id := range in.JobIDs {
		id = strings.ToLower(id)
		if !validID(id) || seen[id] {
			return Order{}, req.ErrInvalidInput
		}
		seen[id] = true
		in.JobIDs[i] = id
	}
	sort.Strings(in.JobIDs)
	in.Lines = append([]LineInput{}, in.Lines...)
	seen = map[string]bool{}
	for i, line := range in.Lines {
		id := strings.ToLower(line.MaterialID)
		if !validID(id) || seen[id] {
			return Order{}, req.ErrInvalidInput
		}
		seen[id] = true
		in.Lines[i].MaterialID = id
	}
	sort.Slice(in.Lines, func(i, j int) bool { return in.Lines[i].MaterialID < in.Lines[j].MaterialID })
	if _, _, err := Totals(in.Lines); err != nil {
		return Order{}, err
	}
	if in.ExpectedAt != nil {
		v := in.ExpectedAt.UTC().Truncate(time.Microsecond)
		if v.Year() < 2000 || v.Year() > 9999 {
			return Order{}, req.ErrInvalidInput
		}
		in.ExpectedAt = &v
	}
	return s.repo.PlaceOrder(ctx, a.ID, in)
}
func (s *Service) Act(ctx context.Context, a auth.Account, id string, in ActionInput) (Order, error) {
	if err := staff(a); err != nil {
		return Order{}, err
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(id) || !validID(in.IdempotencyKey) || in.ExpectedVersion < 1 || !text(in.Reason, 2000, in.Action != "accept") {
		return Order{}, req.ErrInvalidInput
	}
	if in.Action != "accept" && in.Action != "reject" && in.Action != "cancel" {
		return Order{}, req.ErrInvalidInput
	}
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	return s.repo.ActOnOrder(ctx, a.ID, strings.ToLower(id), in)
}
func (s *Service) Get(ctx context.Context, a auth.Account, id string) (Order, error) {
	if err := staff(a); err != nil {
		return Order{}, err
	}
	if !validID(id) {
		return Order{}, req.ErrInvalidInput
	}
	return s.repo.ReadOrder(ctx, a.ID, strings.ToLower(id))
}
func (s *Service) Orders(ctx context.Context, a auth.Account, jobID string, limit, offset int) ([]Order, error) {
	if err := staff(a); err != nil {
		return nil, err
	}
	if err := page("", limit, offset); err != nil {
		return nil, err
	}
	if jobID != "" && !validID(jobID) {
		return nil, req.ErrInvalidInput
	}
	return s.repo.ListOrders(ctx, a.ID, strings.ToLower(jobID), limit, offset)
}

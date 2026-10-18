package receiving

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{r} }
func permitted(a auth.Account, admin bool) error {
	if !a.Active || (a.Role != auth.RoleAdmin && (admin || a.Role != auth.RoleWorker)) {
		return req.ErrForbidden
	}
	return nil
}
func id(s string) bool { v, e := uuid.Parse(s); return len(s) == 36 && e == nil && v != uuid.Nil }
func command(order string, c Command) error {
	if !id(order) || !id(c.IdempotencyKey) || c.ExpectedVersion < 1 {
		return req.ErrInvalidInput
	}
	return nil
}
func note(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= 2000 && !strings.ContainsRune(s, 0)
}
func visible(a auth.Account, v State, e error) (State, error) {
	if a.Role != auth.RoleAdmin {
		v.Finance = nil
	}
	return v, e
}
func (s *Service) List(ctx context.Context, a auth.Account, limit, offset int) ([]Summary, error) {
	if e := permitted(a, false); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, req.ErrInvalidInput
	}
	return s.repo.ReceivingOrders(ctx, a.ID, limit, offset)
}
func (s *Service) Get(ctx context.Context, a auth.Account, order string) (State, error) {
	if e := permitted(a, false); e != nil {
		return State{}, e
	}
	if !id(order) {
		return State{}, req.ErrInvalidInput
	}
	v, e := s.repo.ReceivingState(ctx, a.ID, strings.ToLower(order))
	return visible(a, v, e)
}
func (s *Service) Receive(ctx context.Context, a auth.Account, order string, in ReceiptInput) (State, error) {
	if e := permitted(a, false); e != nil {
		return State{}, e
	}
	if e := command(order, in.Command); e != nil {
		return State{}, e
	}
	in.Note = strings.TrimSpace(in.Note)
	if !note(in.Note) || len(in.Lines) < 1 || len(in.Lines) > 100 {
		return State{}, req.ErrInvalidInput
	}
	in.Lines = append([]ReceiptLine{}, in.Lines...)
	seen := map[string]bool{}
	for i, line := range in.Lines {
		line.MaterialID = strings.ToLower(line.MaterialID)
		line.ReplacementClaimID = strings.ToLower(line.ReplacementClaimID)
		if !id(line.MaterialID) || (line.ReplacementClaimID != "" && !id(line.ReplacementClaimID)) || line.Good == nil || line.Damaged == nil || (line.Good.IsZero() && line.Damaged.IsZero()) {
			return State{}, req.ErrInvalidInput
		}
		if _, e := line.Good.Add(*line.Damaged); e != nil {
			return State{}, req.ErrInvalidInput
		}
		key := line.MaterialID + line.ReplacementClaimID
		if seen[key] {
			return State{}, req.ErrInvalidInput
		}
		seen[key] = true
		in.Lines[i] = line
	}
	sort.Slice(in.Lines, func(i, j int) bool {
		return in.Lines[i].MaterialID+in.Lines[i].ReplacementClaimID < in.Lines[j].MaterialID+in.Lines[j].ReplacementClaimID
	})
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	v, e := s.repo.Receive(ctx, a.ID, strings.ToLower(order), in)
	return visible(a, v, e)
}
func (s *Service) Claim(ctx context.Context, a auth.Account, order, claimID string, in ClaimInput) (State, error) {
	if e := permitted(a, false); e != nil {
		return State{}, e
	}
	in.Note = strings.TrimSpace(in.Note)
	if e := command(order, in.Command); e != nil {
		return State{}, e
	}
	if !id(claimID) || !note(in.Note) || in.Note == "" {
		return State{}, req.ErrInvalidInput
	}
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	v, e := s.repo.ClaimReplacement(ctx, a.ID, strings.ToLower(order), strings.ToLower(claimID), in)
	return visible(a, v, e)
}
func (s *Service) Credit(ctx context.Context, a auth.Account, order string, in CreditInput) (State, error) {
	if e := permitted(a, true); e != nil {
		return State{}, e
	}
	if e := command(order, in.Command); e != nil {
		return State{}, e
	}
	if in.Days == nil || *in.Days < 0 || *in.Days > 36500 {
		return State{}, req.ErrInvalidInput
	}
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	return s.repo.SetCredit(ctx, a.ID, strings.ToLower(order), in)
}
func (s *Service) Settle(ctx context.Context, a auth.Account, order string, in SettlementInput) (State, error) {
	if e := permitted(a, true); e != nil {
		return State{}, e
	}
	if e := command(order, in.Command); e != nil {
		return State{}, e
	}
	if (in.Method != "bank" && in.Method != "cash") || in.PaidAt.IsZero() || (in.Method == "bank" && in.EvidenceID == "") || (in.EvidenceID != "" && !id(in.EvidenceID)) {
		return State{}, req.ErrInvalidInput
	}
	in.PaidAt = in.PaidAt.UTC().Truncate(time.Microsecond)
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	in.EvidenceID = strings.ToLower(in.EvidenceID)
	return s.repo.SettleSupplier(ctx, a.ID, strings.ToLower(order), in)
}

var ErrConflict = p.ErrConflict

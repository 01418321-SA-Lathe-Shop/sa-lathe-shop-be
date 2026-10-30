package production

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"sort"
	"strings"
	"unicode/utf8"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{r} }
func permitted(a auth.Account, role string) error {
	if !a.Active || (a.Role != role && (role != "staff" || (a.Role != auth.RoleAdmin && a.Role != auth.RoleWorker))) {
		return req.ErrForbidden
	}
	return nil
}
func validID(s string) bool { v, e := uuid.Parse(s); return e == nil && len(s) == 36 && v != uuid.Nil }
func validNote(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= 2000 && !strings.ContainsRune(s, 0)
}
func visible(a auth.Account, v State, e error) (State, error) {
	if a.Role != auth.RoleAdmin {
		v.Finance = nil
		for i := range v.Usage {
			v.Usage[i].UnitCost = nil
			v.Usage[i].Total = nil
		}
	}
	return v, e
}
func (s *Service) apply(ctx context.Context, a auth.Account, id string, in Action) (State, error) {
	role := auth.RoleWorker
	if in.Kind == "labor" {
		role = auth.RoleAdmin
	}
	if e := permitted(a, role); e != nil {
		return State{}, e
	}
	if !validID(id) || !validID(in.IdempotencyKey) || in.ExpectedVersion < 0 || !validNote(in.Note) {
		return State{}, req.ErrInvalidInput
	}
	in.IdempotencyKey = strings.ToLower(in.IdempotencyKey)
	v, e := s.repo.ApplyProduction(ctx, a.ID, strings.ToLower(id), in)
	return visible(a, v, e)
}
func (s *Service) Start(ctx context.Context, a auth.Account, id string, in StartInput) (State, error) {
	if e := permitted(a, auth.RoleWorker); e != nil {
		return State{}, e
	}
	if in.ExpectedPlanVersion == nil || *in.ExpectedPlanVersion < 1 {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, Action{Command: in.Command, Kind: "start", PlanVersion: *in.ExpectedPlanVersion})
}
func (s *Service) Use(ctx context.Context, a auth.Account, id string, in UsageInput) (State, error) {
	if e := permitted(a, auth.RoleWorker); e != nil {
		return State{}, e
	}
	if len(in.Lines) < 1 || len(in.Lines) > 100 {
		return State{}, req.ErrInvalidInput
	}
	in.Lines = append([]UsageLine{}, in.Lines...)
	seen := map[string]bool{}
	for i, line := range in.Lines {
		line.MaterialID = strings.ToLower(line.MaterialID)
		line.UnitID = strings.ToLower(line.UnitID)
		if !validID(line.MaterialID) || line.Quantity == nil || line.Quantity.IsZero() || seen[line.MaterialID] || (line.UnitID != "" && !validID(line.UnitID)) {
			return State{}, req.ErrInvalidInput
		}
		seen[line.MaterialID] = true
		in.Lines[i] = line
	}
	sort.Slice(in.Lines, func(i, j int) bool { return in.Lines[i].MaterialID < in.Lines[j].MaterialID })
	return s.apply(ctx, a, id, Action{Command: in.Command, Kind: "use", Lines: in.Lines, Note: strings.TrimSpace(in.Note)})
}
func (s *Service) Time(ctx context.Context, a auth.Account, id string, in TimeInput) (State, error) {
	if e := permitted(a, auth.RoleWorker); e != nil {
		return State{}, e
	}
	if in.Minutes < 1 || in.Minutes > 1000000 {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, Action{Command: in.Command, Kind: "time", Minutes: in.Minutes, Note: strings.TrimSpace(in.Note)})
}
func (s *Service) Complete(ctx context.Context, a auth.Account, id string, in Command) (State, error) {
	return s.apply(ctx, a, id, Action{Command: in, Kind: "complete"})
}
func (s *Service) Labor(ctx context.Context, a auth.Account, id string, in LaborInput) (State, error) {
	if e := permitted(a, auth.RoleAdmin); e != nil {
		return State{}, e
	}
	if in.Amount == nil {
		return State{}, req.ErrInvalidInput
	}
	return s.apply(ctx, a, id, Action{Command: in.Command, Kind: "labor", Amount: in.Amount})
}
func (s *Service) Get(ctx context.Context, a auth.Account, id string) (State, error) {
	if e := permitted(a, "staff"); e != nil {
		return State{}, e
	}
	if !validID(id) {
		return State{}, req.ErrInvalidInput
	}
	v, e := s.repo.ProductionState(ctx, a.ID, strings.ToLower(id))
	return visible(a, v, e)
}
func (s *Service) List(ctx context.Context, a auth.Account, limit, offset int) ([]Summary, error) {
	if e := permitted(a, "staff"); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, req.ErrInvalidInput
	}
	return s.repo.ProductionJobs(ctx, a.ID, limit, offset)
}

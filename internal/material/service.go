package material

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"sort"
	"strings"
	"unicode/utf8"
)

func validID(s string) bool { _, err := uuid.Parse(s); return err == nil && len(s) == 36 }
func validText(s string, n int) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= n && !strings.ContainsAny(s, "\x00\r\n")
}
func allowed(a auth.Account, role string) bool {
	return a.Active && (a.Role == role || (role == "staff" && (a.Role == auth.RoleAdmin || a.Role == auth.RoleWorker)))
}
func ValidateStock(in StockInput) error {
	if !validID(in.IdempotencyKey) || in.ExpectedVersion < 1 || in.Quantity == nil || !validText(in.Reason, 2000) || (in.UnitID != "" && !validID(in.UnitID)) {
		return req.ErrInvalidInput
	}
	switch in.Mode {
	case "addition":
		if in.Quantity.IsZero() || in.UnitCost == nil {
			return req.ErrInvalidInput
		}
	case "correction":
		if in.UnitCost != nil {
			return req.ErrInvalidInput
		}
	default:
		return req.ErrInvalidInput
	}
	return nil
}
func StockEffect(m Material, in StockInput) (Decimal, Decimal, error) {
	if err := ValidateStock(in); err != nil {
		return Decimal{}, Decimal{}, err
	}
	qty := *in.Quantity
	cost := m.AverageCost
	if in.Mode == "addition" {
		var err error
		cost, err = WeightedAverage(m.OnHand, m.AverageCost, qty, *in.UnitCost)
		if err != nil {
			return Decimal{}, Decimal{}, err
		}
		qty, err = m.OnHand.Add(qty)
		if err != nil {
			return Decimal{}, Decimal{}, err
		}
	}
	if _, err := qty.Subtract(m.Reserved); err != nil {
		return Decimal{}, Decimal{}, ErrConflict
	}
	return qty, cost, nil
}

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{r} }
func (s *Service) CreateUnit(ctx context.Context, a auth.Account, in UnitInput) (Unit, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Unit{}, req.ErrForbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validText(in.Name, 50) {
		return Unit{}, req.ErrInvalidInput
	}
	return s.repo.CreateUnit(ctx, a.ID, in)
}
func (s *Service) UpdateUnit(ctx context.Context, a auth.Account, unitID string, in UnitInput) (Unit, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Unit{}, req.ErrForbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validID(unitID) || !validText(in.Name, 50) {
		return Unit{}, req.ErrInvalidInput
	}
	return s.repo.UpdateUnit(ctx, a.ID, unitID, in)
}
func (s *Service) Units(ctx context.Context, a auth.Account) ([]Unit, error) {
	if !allowed(a, "staff") {
		return nil, req.ErrForbidden
	}
	return s.repo.ListUnits(ctx, a.ID)
}
func (s *Service) Create(ctx context.Context, a auth.Account, in MaterialInput) (Material, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Material{}, req.ErrForbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validText(in.Name, 200) || !validID(in.UnitID) {
		return Material{}, req.ErrInvalidInput
	}
	return s.repo.CreateMaterial(ctx, a.ID, in)
}
func (s *Service) Update(ctx context.Context, a auth.Account, id string, in MaterialUpdate) (Material, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Material{}, req.ErrForbidden
	}
	if !validID(id) || in.ExpectedVersion < 1 || (in.Name == nil && in.Active == nil) {
		return Material{}, req.ErrInvalidInput
	}
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if !validText(v, 200) {
			return Material{}, req.ErrInvalidInput
		}
		in.Name = &v
	}
	return s.repo.UpdateMaterial(ctx, a.ID, id, in)
}
func (s *Service) Search(ctx context.Context, a auth.Account, term string, limit, offset int) ([]Material, error) {
	if !allowed(a, "staff") {
		return nil, req.ErrForbidden
	}
	term = strings.TrimSpace(term)
	if !utf8.ValidString(term) || strings.ContainsRune(term, 0) || utf8.RuneCountInString(term) > 200 || limit < 1 || limit > 100 || offset < 0 {
		return nil, req.ErrInvalidInput
	}
	return s.repo.SearchMaterials(ctx, a.ID, term, limit, offset)
}
func (s *Service) Stock(ctx context.Context, a auth.Account, id string, in StockInput) (Movement, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Movement{}, req.ErrForbidden
	}
	in.Reason = strings.TrimSpace(in.Reason)
	in.UnitID = strings.ToLower(in.UnitID)
	if !validID(id) {
		return Movement{}, req.ErrInvalidInput
	}
	if err := ValidateStock(in); err != nil {
		return Movement{}, err
	}
	return s.repo.ChangeStock(ctx, a.ID, id, in)
}
func (s *Service) Movements(ctx context.Context, a auth.Account, id string, limit, offset int) ([]Movement, error) {
	if !allowed(a, "staff") {
		return nil, req.ErrForbidden
	}
	if !validID(id) || limit < 1 || limit > 100 || offset < 0 {
		return nil, req.ErrInvalidInput
	}
	return s.repo.ListMovements(ctx, a.ID, id, limit, offset)
}
func (s *Service) Save(ctx context.Context, a auth.Account, id string, in PlanInput) (Plan, error) {
	if !allowed(a, auth.RoleWorker) {
		return Plan{}, req.ErrForbidden
	}
	if !validID(id) || !validID(in.IdempotencyKey) || in.ExpectedVersion < 0 || in.Lines == nil || len(in.Lines) > 500 {
		return Plan{}, req.ErrInvalidInput
	}
	lines := make([]Requirement, len(in.Lines))
	copy(lines, in.Lines)
	in.Lines = lines
	seen := map[string]bool{}
	for i, line := range in.Lines {
		key := strings.ToLower(line.MaterialID)
		if !validID(key) || line.Quantity == nil || line.Quantity.IsZero() || seen[key] || (line.UnitID != "" && !validID(line.UnitID)) {
			return Plan{}, req.ErrInvalidInput
		}
		seen[key] = true
		in.Lines[i].MaterialID = key
		in.Lines[i].UnitID = strings.ToLower(line.UnitID)
	}
	sort.Slice(in.Lines, func(i, j int) bool { return in.Lines[i].MaterialID < in.Lines[j].MaterialID })
	return s.repo.ApplyPlan(ctx, a.ID, id, in, false)
}
func (s *Service) Reserve(ctx context.Context, a auth.Account, id string, in ReserveInput) (Plan, error) {
	if !allowed(a, auth.RoleWorker) {
		return Plan{}, req.ErrForbidden
	}
	if !validID(id) || !validID(in.IdempotencyKey) || in.ExpectedVersion < 0 {
		return Plan{}, req.ErrInvalidInput
	}
	return s.repo.ApplyPlan(ctx, a.ID, id, PlanInput{IdempotencyKey: in.IdempotencyKey, ExpectedVersion: in.ExpectedVersion}, true)
}
func (s *Service) Conversions(ctx context.Context, a auth.Account, id string) ([]Conversion, error) {
	if !allowed(a, "staff") {
		return nil, req.ErrForbidden
	}
	if !validID(id) {
		return nil, req.ErrInvalidInput
	}
	return s.repo.ListConversions(ctx, a.ID, id)
}
func (s *Service) CreateConversion(ctx context.Context, a auth.Account, id string, in ConversionInput) (Conversion, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Conversion{}, req.ErrForbidden
	}
	if !validID(id) || !validID(in.UnitID) || in.Factor == nil || in.Factor.IsZero() {
		return Conversion{}, req.ErrInvalidInput
	}
	return s.repo.CreateConversion(ctx, a.ID, id, in)
}
func (s *Service) UpdateConversion(ctx context.Context, a auth.Account, id, conversionID string, in ConversionUpdate) (Conversion, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Conversion{}, req.ErrForbidden
	}
	if !validID(id) || !validID(conversionID) || in.Factor == nil || in.Factor.IsZero() {
		return Conversion{}, req.ErrInvalidInput
	}
	return s.repo.UpdateConversion(ctx, a.ID, id, conversionID, in)
}
func (s *Service) DeactivateConversion(ctx context.Context, a auth.Account, id, conversionID string) (Conversion, error) {
	if !allowed(a, auth.RoleAdmin) {
		return Conversion{}, req.ErrForbidden
	}
	if !validID(id) || !validID(conversionID) {
		return Conversion{}, req.ErrInvalidInput
	}
	return s.repo.DeactivateConversion(ctx, a.ID, id, conversionID)
}
func (s *Service) Get(ctx context.Context, a auth.Account, id string) (Plan, error) {
	if !allowed(a, "staff") {
		return Plan{}, req.ErrForbidden
	}
	if !validID(id) {
		return Plan{}, req.ErrInvalidInput
	}
	return s.repo.GetPlan(ctx, a.ID, id)
}

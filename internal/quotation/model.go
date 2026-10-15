package quotation

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"time"
)

var (
	ErrForbidden = errors.New("quotation access denied")
	ErrNotFound  = errors.New("quotation or request not found")
	ErrConflict  = errors.New("quotation state or edit version changed")
)

type DraftInput struct {
	MaterialCost *Amount `json:"material_cost"`
	LaborCost    *Amount `json:"labor_cost"`
	FinalPrice   *Amount `json:"final_price"`
}
type EditInput struct {
	DraftInput
	ExpectedLockVersion int64 `json:"expected_lock_version"`
}
type Quotation struct {
	ID           string     `json:"id"`
	Number       int64      `json:"quotation_no"`
	RequestID    string     `json:"request_id"`
	Version      int64      `json:"version"`
	PreviousID   *string    `json:"previous_quotation_id,omitempty"`
	Status       string     `json:"status"`
	LockVersion  int64      `json:"lock_version"`
	FinalPrice   Amount     `json:"final_price"`
	MaterialCost Amount     `json:"-"`
	LaborCost    Amount     `json:"-"`
	Costs        *Estimate  `json:"internal_costs,omitempty"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	EditedAt     time.Time  `json:"edited_at"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
}
type Draft struct {
	ID, RequestID                       string
	MaterialCost, LaborCost, FinalPrice Amount
	ExpectedLockVersion                 int64
}
type Repository interface {
	CreateDraft(context.Context, string, Draft) (Quotation, error)
	UpdateDraft(context.Context, string, Draft) (Quotation, error)
	SendQuotation(context.Context, string, string, int64) (Quotation, error)
	RejectRequest(context.Context, string, string, string) error
	GetQuotation(context.Context, string, string) (Quotation, error)
	ListQuotations(context.Context, string, string, int, int) ([]Quotation, error)
}
type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func isWorker(a auth.Account) bool        { return a.Active && a.Role == auth.RoleWorker && validID(a.ID) }

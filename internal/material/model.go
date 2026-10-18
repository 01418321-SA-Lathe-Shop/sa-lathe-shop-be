package material

import (
	"context"
	"errors"
	"time"
)

var ErrConflict = errors.New("inventory or material plan state conflict")

type Unit struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type UnitInput struct {
	Name string `json:"name"`
}
type Material struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	UnitID      string  `json:"unit_id"`
	UnitName    string  `json:"unit_name"`
	Active      bool    `json:"is_active"`
	Version     int64   `json:"version"`
	OnHand      Decimal `json:"qty_on_hand"`
	Reserved    Decimal `json:"qty_reserved"`
	Available   Decimal `json:"qty_available"`
	AverageCost Decimal `json:"avg_cost"`
}
type MaterialInput struct {
	Name   string `json:"name"`
	UnitID string `json:"unit_id"`
}
type MaterialUpdate struct {
	Name            *string `json:"name,omitempty"`
	Active          *bool   `json:"is_active,omitempty"`
	ExpectedVersion int64   `json:"expected_version"`
}
type StockInput struct {
	IdempotencyKey  string   `json:"idempotency_key"`
	Mode            string   `json:"mode"`
	Quantity        *Decimal `json:"quantity"`
	UnitCost        *Decimal `json:"unit_cost,omitempty"`
	Reason          string   `json:"reason"`
	ExpectedVersion int64    `json:"expected_version"`
}
type Movement struct {
	ID         string `json:"id"`
	MaterialID string `json:"material_id"`
	ActorID    string `json:"actor_id"`
	StockInput
	BeforeQuantity Decimal   `json:"before_quantity"`
	AfterQuantity  Decimal   `json:"after_quantity"`
	BeforeCost     Decimal   `json:"before_cost"`
	AfterCost      Decimal   `json:"after_cost"`
	Version        int64     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
}
type Requirement struct {
	MaterialID string   `json:"material_id"`
	Quantity   *Decimal `json:"quantity"`
}
type PlanInput struct {
	IdempotencyKey  string        `json:"idempotency_key"`
	ExpectedVersion int64         `json:"expected_version"`
	Lines           []Requirement `json:"lines"`
}
type ReserveInput struct {
	IdempotencyKey  string `json:"idempotency_key"`
	ExpectedVersion int64  `json:"expected_version"`
}
type PlanLine struct {
	MaterialID string  `json:"material_id"`
	Required   Decimal `json:"required"`
	Reserved   Decimal `json:"reserved"`
	Shortage   Decimal `json:"shortage"`
}
type Plan struct {
	JobID   string     `json:"job_id"`
	Version int64      `json:"version"`
	Ready   bool       `json:"ready"`
	Lines   []PlanLine `json:"lines"`
}
type Repository interface {
	CreateUnit(context.Context, string, UnitInput) (Unit, error)
	ListUnits(context.Context, string) ([]Unit, error)
	CreateMaterial(context.Context, string, MaterialInput) (Material, error)
	UpdateMaterial(context.Context, string, string, MaterialUpdate) (Material, error)
	SearchMaterials(context.Context, string, string, int, int) ([]Material, error)
	ChangeStock(context.Context, string, string, StockInput) (Movement, error)
	ListMovements(context.Context, string, string, int, int) ([]Movement, error)
	ApplyPlan(context.Context, string, string, PlanInput, bool) (Plan, error)
	GetPlan(context.Context, string, string) (Plan, error)
}

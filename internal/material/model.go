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

// Entered records what a person typed when it was not the base unit.
type Entered struct {
	EnteredQuantity *Decimal `json:"entered_quantity,omitempty"`
	EnteredUnit     *string  `json:"entered_unit_name,omitempty"`
	FactorUsed      *Decimal `json:"factor_used,omitempty"`
}

// Quantity is in UnitID when set, else the base unit. UnitCost is always per base unit.
type StockInput struct {
	IdempotencyKey  string   `json:"idempotency_key"`
	Mode            string   `json:"mode"`
	Quantity        *Decimal `json:"quantity"`
	UnitID          string   `json:"unit_id,omitempty"`
	UnitCost        *Decimal `json:"unit_cost,omitempty"`
	Reason          string   `json:"reason"`
	ExpectedVersion int64    `json:"expected_version"`
}
type Movement struct {
	ID         string `json:"id"`
	MaterialID string `json:"material_id"`
	ActorID    string `json:"actor_id"`
	StockInput
	Entered
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
	UnitID     string   `json:"unit_id,omitempty"`
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
	Consumed   Decimal `json:"consumed"`
	Shortage   Decimal `json:"shortage"`
	Entered
}
type Plan struct {
	JobID   string     `json:"job_id"`
	Version int64      `json:"version"`
	Ready   bool       `json:"ready"`
	Lines   []PlanLine `json:"lines"`
}

// Conversion means 1 unit = Factor base units of one material.
type Conversion struct {
	ID            string     `json:"id"`
	MaterialID    string     `json:"material_id"`
	UnitID        string     `json:"unit_id"`
	UnitName      string     `json:"unit_name"`
	Factor        Decimal    `json:"factor"`
	Active        bool       `json:"is_active"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	DeactivatedBy *string    `json:"deactivated_by,omitempty"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
}
type ConversionInput struct {
	UnitID string   `json:"unit_id"`
	Factor *Decimal `json:"factor"`
}
type ConversionUpdate struct {
	Factor *Decimal `json:"factor"`
}
type Repository interface {
	CreateUnit(context.Context, string, UnitInput) (Unit, error)
	UpdateUnit(context.Context, string, string, UnitInput) (Unit, error)
	ListUnits(context.Context, string) ([]Unit, error)
	CreateMaterial(context.Context, string, MaterialInput) (Material, error)
	UpdateMaterial(context.Context, string, string, MaterialUpdate) (Material, error)
	SearchMaterials(context.Context, string, string, int, int) ([]Material, error)
	ChangeStock(context.Context, string, string, StockInput) (Movement, error)
	ListMovements(context.Context, string, string, int, int) ([]Movement, error)
	ApplyPlan(context.Context, string, string, PlanInput, bool) (Plan, error)
	GetPlan(context.Context, string, string) (Plan, error)
	ListConversions(context.Context, string, string) ([]Conversion, error)
	CreateConversion(context.Context, string, string, ConversionInput) (Conversion, error)
	UpdateConversion(context.Context, string, string, string, ConversionUpdate) (Conversion, error)
	DeactivateConversion(context.Context, string, string, string) (Conversion, error)
}

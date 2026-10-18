package procurement

import (
	"context"
	"errors"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"time"
)

var ErrConflict = errors.New("procurement state or idempotency conflict")

type SupplierInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}
type Supplier struct {
	ID string `json:"id"`
	SupplierInput
	Active  bool  `json:"is_active"`
	Version int64 `json:"version"`
}

// Updates replace contact fields, never remove supplier history.
type SupplierUpdate struct {
	SupplierInput
	Active          *bool `json:"is_active"`
	ExpectedVersion int64 `json:"expected_version"`
}
type LineInput struct {
	MaterialID string     `json:"material_id"`
	Quantity   *m.Decimal `json:"quantity"`
	UnitCost   *m.Decimal `json:"unit_cost"`
}
type OrderInput struct {
	IdempotencyKey string      `json:"idempotency_key"`
	SupplierID     string      `json:"supplier_id"`
	JobIDs         []string    `json:"job_ids"`
	ExpectedAt     *time.Time  `json:"expected_at,omitempty"`
	Lines          []LineInput `json:"lines"`
}
type Line struct {
	MaterialID   string    `json:"material_id"`
	MaterialName string    `json:"material_name"`
	UnitID       string    `json:"unit_id"`
	UnitName     string    `json:"unit_name"`
	Quantity     m.Decimal `json:"quantity"`
	UnitCost     m.Decimal `json:"unit_cost"`
	Total        q.Amount  `json:"total_amount"`
}
type Event struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}
type Order struct {
	ID               string     `json:"id"`
	OrderNo          string     `json:"order_no"`
	Supplier         Supplier   `json:"supplier"`
	JobIDs           []string   `json:"job_ids"`
	Lines            []Line     `json:"lines"`
	Total            q.Amount   `json:"total_amount"`
	Status           string     `json:"status"`
	SupplierResponse string     `json:"supplier_response"`
	Version          int64      `json:"version"`
	ExpectedAt       *time.Time `json:"expected_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CreatedBy        string     `json:"created_by"`
	Events           []Event    `json:"events"`
}
type ActionInput struct {
	IdempotencyKey  string `json:"idempotency_key"`
	ExpectedVersion int64  `json:"expected_version"`
	Action          string `json:"action"` // accept, reject, cancel
	Reason          string `json:"reason"`
}
type Repository interface {
	CreateSupplier(context.Context, string, SupplierInput) (Supplier, error)
	UpdateSupplier(context.Context, string, string, SupplierUpdate) (Supplier, error)
	ListSuppliers(context.Context, string, string, int, int) ([]Supplier, error)
	PlaceOrder(context.Context, string, OrderInput) (Order, error)
	ActOnOrder(context.Context, string, string, ActionInput) (Order, error)
	ReadOrder(context.Context, string, string) (Order, error)
	ListOrders(context.Context, string, string, int, int) ([]Order, error)
}

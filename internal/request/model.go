// Package request implements UC1 manufacturing request business rules.
package request

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid request input")
	ErrForbidden    = errors.New("customer access required")
	ErrNotFound     = errors.New("request not found")
	ErrConflict     = errors.New("request state changed")
)

// Principal must come from authenticated server context, never a JSON body.
type Principal struct {
	AccountID string
	Role      string
	Active    bool
}

type Input struct {
	CustomerID   string     `json:"customer_id"`
	Description  string     `json:"description"`
	Quantity     int64      `json:"qty"`
	MaterialSpec string     `json:"material_spec"`
	Deadline     *time.Time `json:"deadline,omitempty"`
	Source       string     `json:"source,omitempty"`
}

type Request struct {
	Number    int64  `json:"no_request"`
	ID        string `json:"id"`
	CreatedBy string `json:"created_by"`
	Input
	Source          string     `json:"source"`
	Status          string     `json:"status"`
	InfoRequestNote string     `json:"info_request_note,omitempty"`
	InfoRequestedAt *time.Time `json:"info_requested_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Repository implementations must verify authenticated account membership in
// the selected customer/company, never trust a client-supplied ID. Create persists
// the request and its notification outbox event atomically. GetOwned returns
// ErrNotFound for both missing and inaccessible requests to avoid ID disclosure.
// Company members share their company's requests; staff may read the shop queue.
type Repository interface {
	Create(context.Context, Request) (Request, error)
	GetOwned(context.Context, string, string) (Request, error)
	ListRequests(context.Context, string, int, int) ([]Request, error)
}

type OptionalRepository interface {
	DeletePending(ctx context.Context, accountID, id string) error
	RequestInfo(ctx context.Context, accountID, id, note string) (Request, error)
}

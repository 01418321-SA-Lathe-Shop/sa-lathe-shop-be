// Package payment implements UC4 manual evidence verification, not a payment gateway.
package payment

import (
	"context"
	"errors"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"time"
)

var ErrConflict = errors.New("evidence or fulfillment state conflict")

type Submission struct {
	IdempotencyKey string     `json:"idempotency_key"`
	AttachmentID   string     `json:"attachment_id"`
	Kind           string     `json:"kind"`
	Amount         *q.Amount  `json:"amount,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
}
type Review struct {
	Outcome         string `json:"outcome"`
	Reason          string `json:"reason,omitempty"`
	ExpectedVersion int64  `json:"expected_version"`
}
type Evidence struct {
	ID          string `json:"id"`
	QuotationID string `json:"quotation_id"`
	SubmittedBy string `json:"submitted_by"`
	Submission
	Status      string     `json:"status"`
	Version     int64      `json:"version"`
	SubmittedAt time.Time  `json:"submitted_at"`
	ReviewedBy  *string    `json:"reviewed_by"`
	ReviewedAt  *time.Time `json:"reviewed_at"`
	Reason      string     `json:"reason,omitempty"`
}
type Job struct {
	ID            string     `json:"id"`
	QuotationID   string     `json:"quotation_id"`
	RequestID     string     `json:"request_id"`
	Status        string     `json:"status"`
	DepositAmount q.Amount   `json:"deposit_amount"`
	Balance       q.Amount   `json:"balance"`
	DueAt         *time.Time `json:"due_at"`
	CreatedAt     time.Time  `json:"created_at"`
}
type Fulfillment struct {
	QuotationID   string     `json:"quotation_id"`
	RequestStatus string     `json:"request_status"`
	Terms         q.Terms    `json:"terms"`
	Evidence      []Evidence `json:"evidence"`
	Job           *Job       `json:"job"`
	CanceledAt    *time.Time `json:"canceled_at"`
}
type Repository interface {
	SubmitEvidence(context.Context, string, string, Submission) (Evidence, error)
	ReviewEvidence(context.Context, string, string, Review) (Evidence, error)
	GetFulfillment(context.Context, string, string) (Fulfillment, error)
}
type ExpiryRepository interface {
	ExpireUnfulfilled(context.Context) (int, error)
}

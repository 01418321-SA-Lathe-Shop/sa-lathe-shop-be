// Package delivery owns UC9 customer fulfillment and receivables, not supplier payments.
package delivery

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"time"
)

var ErrConflict = errors.New("fulfillment state or command conflict")

type Command struct {
	IdempotencyKey  string `json:"idempotency_key"`
	ExpectedVersion int64  `json:"expected_version"`
}
type TermsInput struct {
	Command
	CreditDays *int `json:"credit_days"`
}
type ShipInput struct {
	Command
	Method       string `json:"method"`
	ReceiverName string `json:"receiver_name"`
}
type InspectInput struct {
	Command
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
}
type PaymentInput struct {
	Command
	Amount     *q.Amount  `json:"amount"`
	PaidAt     *time.Time `json:"paid_at"`
	EvidenceID string     `json:"evidence_id"`
}
type ReviewInput struct {
	Command
	PaymentID string `json:"payment_id"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason"`
}

// Action is constructed by Service, never decoded directly from HTTP.
type Action struct {
	Command
	Kind         string     `json:"kind"`
	CreditDays   *int       `json:"credit_days,omitempty"`
	Method       string     `json:"method,omitempty"`
	ReceiverName string     `json:"receiver_name,omitempty"`
	Outcome      string     `json:"outcome,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Amount       *q.Amount  `json:"amount,omitempty"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
	EvidenceID   string     `json:"evidence_id,omitempty"`
	PaymentID    string     `json:"payment_id,omitempty"`
}
type Invoice struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	Amount     q.Amount  `json:"amount"`
	Paid       q.Amount  `json:"paid"`
	Balance    q.Amount  `json:"balance"`
	Status     string    `json:"status"`
	CreditDays int       `json:"credit_days"`
	IssuedAt   time.Time `json:"issued_at"`
	DueAt      time.Time `json:"due_at"`
}
type Shipment struct {
	ID           string     `json:"id"`
	Cycle        int64      `json:"cycle"`
	Method       string     `json:"method"`
	ReceiverName string     `json:"receiver_name"`
	Status       string     `json:"status"`
	ShippedAt    time.Time  `json:"shipped_at"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	DecidedBy    *string    `json:"decided_by,omitempty"`
	Reason       string     `json:"reason"`
}
type Payment struct {
	ID          string    `json:"id"`
	SubmittedBy string    `json:"submitted_by"`
	Method      string    `json:"method"`
	Amount      q.Amount  `json:"amount"`
	PaidAt      time.Time `json:"paid_at"`
	Status      string    `json:"status"`
	EvidenceID  *string   `json:"evidence_id,omitempty"`
	Reason      string    `json:"reason"`
	ReviewedBy  *string   `json:"reviewed_by,omitempty"`
}
type Event struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Version   int64     `json:"version"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type State struct {
	JobID          string        `json:"job_id"`
	RequestID      string        `json:"request_id"`
	JobStatus      string        `json:"job_status"`
	RequestStatus  string        `json:"request_status"`
	Version        int64         `json:"version"`
	Cycle          int64         `json:"cycle"`
	CreditDays     *int          `json:"credit_days,omitempty"`
	OpeningBalance q.Amount      `json:"opening_balance"`
	Deposit        q.Amount      `json:"deposit"`
	Invoice        *Invoice      `json:"invoice,omitempty"`
	Deliveries     []Shipment    `json:"deliveries"`
	Payments       []Payment     `json:"payments,omitempty"`
	Events         []Event       `json:"events"`
	ReworkHistory  []ReworkCycle `json:"rework_history"`
	Photos         []JobPhoto    `json:"photos"`
}

// JobPhoto is a ready UC8 production photo the job's customer may view.
type JobPhoto struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
	State       string `json:"state"`
}

type ReworkCycle struct {
	Cycle       int64     `json:"cycle"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	ReopenedAt  time.Time `json:"reopened_at"`
	DeliveryID  string    `json:"delivery_id"`
}
type Repository interface {
	FulfillmentState(context.Context, string, string) (State, error)
	FulfillmentJobs(context.Context, string, int, int) ([]State, error)
	ApplyFulfillment(context.Context, string, string, Action) (State, error)
}
type Evidence struct {
	ID         string `json:"id"`
	JobID      string `json:"job_id"`
	UploadedBy string `json:"uploaded_by"`
	file.Input
	StagingKey string    `json:"-"`
	ObjectKey  string    `json:"-"`
	State      string    `json:"state"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type EvidenceRepository interface {
	CreateCustomerEvidence(context.Context, Evidence) error
	BeginCustomerEvidence(context.Context, string, string) (Evidence, error)
	FinishCustomerEvidence(context.Context, string, string, bool) error
	ReadCustomerEvidence(context.Context, string, string) (Evidence, error)
}

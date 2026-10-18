package receiving

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"time"
)

type Command struct {
	IdempotencyKey  string `json:"idempotency_key"`
	ExpectedVersion int64  `json:"expected_version"`
}
type ReceiptLine struct {
	MaterialID         string     `json:"material_id"`
	Good               *m.Decimal `json:"good"`
	Damaged            *m.Decimal `json:"damaged"`
	ReplacementClaimID string     `json:"replacement_claim_id,omitempty"`
}
type ReceiptInput struct {
	Command
	Note  string        `json:"note"`
	Lines []ReceiptLine `json:"lines"`
}
type ClaimInput struct {
	Command
	Note string `json:"note"`
}
type CreditInput struct {
	Command
	Days *int `json:"credit_days"`
}
type SettlementInput struct {
	Command
	Method     string    `json:"method"`
	PaidAt     time.Time `json:"paid_at"`
	EvidenceID string    `json:"evidence_id,omitempty"`
}
type Summary struct {
	OrderID        string     `json:"order_id"`
	OrderNo        string     `json:"order_no"`
	SupplierName   string     `json:"supplier_name"`
	Status         string     `json:"status"`
	Version        int64      `json:"version"`
	FullReceivedAt *time.Time `json:"full_received_at,omitempty"`
}
type Progress struct {
	MaterialID  string    `json:"material_id"`
	Name        string    `json:"material_name"`
	UnitName    string    `json:"unit_name"`
	Ordered     m.Decimal `json:"ordered"`
	Good        m.Decimal `json:"good"`
	Damaged     m.Decimal `json:"damaged"`
	Outstanding m.Decimal `json:"outstanding"`
}
type Receipt struct {
	ID        string        `json:"id"`
	ActorID   string        `json:"actor_id"`
	Note      string        `json:"note"`
	CreatedAt time.Time     `json:"created_at"`
	Lines     []ReceiptLine `json:"lines"`
}
type Claim struct {
	ID         string     `json:"id"`
	ReceiptID  string     `json:"receipt_id"`
	MaterialID string     `json:"material_id"`
	Quantity   m.Decimal  `json:"quantity"`
	Replaced   m.Decimal  `json:"replaced"`
	Status     string     `json:"status"`
	ClaimedBy  *string    `json:"claimed_by,omitempty"`
	ClaimedAt  *time.Time `json:"claimed_at,omitempty"`
	Note       string     `json:"note"`
}
type Settlement struct {
	ActorID    string    `json:"actor_id"`
	Amount     q.Amount  `json:"amount"`
	Method     string    `json:"method"`
	PaidAt     time.Time `json:"paid_at"`
	EvidenceID *string   `json:"evidence_id,omitempty"`
}
type Finance struct {
	Total         q.Amount    `json:"total_amount"`
	CreditDays    *int        `json:"credit_days"`
	DueAt         *time.Time  `json:"payment_due_at,omitempty"`
	PaymentStatus string      `json:"payment_status"`
	Settlement    *Settlement `json:"settlement,omitempty"`
}

// Workers get this operational view, never a procurement.Order or finance JSON.
type State struct {
	Summary
	Lines    []Progress `json:"lines"`
	Receipts []Receipt  `json:"receipts"`
	Claims   []Claim    `json:"claims"`
	Finance  *Finance   `json:"finance,omitempty"`
}
type Evidence struct {
	ID         string `json:"id"`
	OrderID    string `json:"order_id"`
	UploadedBy string `json:"uploaded_by"`
	file.Input
	StagingKey string    `json:"-"`
	ObjectKey  string    `json:"-"`
	State      string    `json:"state"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Repository interface {
	ReceivingOrders(context.Context, string, int, int) ([]Summary, error)
	ReceivingState(context.Context, string, string) (State, error)
	Receive(context.Context, string, string, ReceiptInput) (State, error)
	ClaimReplacement(context.Context, string, string, string, ClaimInput) (State, error)
	SetCredit(context.Context, string, string, CreditInput) (State, error)
	SettleSupplier(context.Context, string, string, SettlementInput) (State, error)
}
type EvidenceRepository interface {
	CreateOrderEvidence(context.Context, Evidence) error
	BeginOrderEvidence(context.Context, string, string) (Evidence, error)
	FinishOrderEvidence(context.Context, string, string, bool) error
	ReadOrderEvidence(context.Context, string, string) (Evidence, error)
}

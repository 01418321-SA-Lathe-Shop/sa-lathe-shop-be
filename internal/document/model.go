package document

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid document input")
	ErrForbidden    = errors.New("document access denied")
	ErrNotFound     = errors.New("document not found")
	ErrConflict     = errors.New("document state or edit version changed")
)

// Empty strings mean "not provided"; printed documents show a fill-in line instead.
type ShopProfile struct {
	Name            string `json:"name"`
	Address         string `json:"address"`
	TaxID           string `json:"tax_id"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	BankName        string `json:"bank_name"`
	BankAccountName string `json:"bank_account_name"`
	BankAccountNo   string `json:"bank_account_no"`
	// 0 until the first save.
	LockVersion int64      `json:"lock_version"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}
type ShopInput struct {
	ShopProfile
	// 0 creates the profile; otherwise it must match the stored lock_version.
	ExpectedLockVersion int64 `json:"expected_lock_version"`
}
type Billing struct {
	CustomerID  string `json:"customer_id"`
	Name        string `json:"name"`
	Type        string `json:"customer_type"`
	Address     string `json:"address"`
	TaxID       string `json:"tax_id"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	LockVersion int64  `json:"lock_version"`
}
type BillingInput struct {
	Address             string `json:"address"`
	TaxID               string `json:"tax_id"`
	Phone               string `json:"phone"`
	Email               string `json:"email"`
	ExpectedLockVersion int64  `json:"expected_lock_version"`
}

// The PO amount is never stored: it is always the accepted quotation's final price.
type CustomerPO struct {
	QuotationID string    `json:"quotation_id"`
	PONo        string    `json:"po_no"`
	IssuedOn    string    `json:"issued_on"`
	Remarks     string    `json:"remarks"`
	LockVersion int64     `json:"lock_version"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type POInput struct {
	PONo     string `json:"po_no"`
	IssuedOn string `json:"issued_on"`
	Remarks  string `json:"remarks"`
	// 0 creates the PO; otherwise it must match the stored lock_version.
	ExpectedLockVersion int64 `json:"expected_lock_version"`
}

type Repository interface {
	GetShopProfile(context.Context) (ShopProfile, error)
	SaveShopProfile(context.Context, string, ShopInput) (ShopProfile, error)
	GetBilling(context.Context, string, string) (Billing, error)
	SaveBilling(context.Context, string, string, BillingInput) (Billing, error)
	GetCustomerPO(context.Context, string, string) (CustomerPO, error)
	SaveCustomerPO(context.Context, string, string, POInput) (CustomerPO, error)
}

func validID(id string) bool { _, err := uuid.Parse(id); return len(id) == 36 && err == nil }

// text trims s and rejects invalid UTF-8 and control characters (newlines only when multiline).
func text(s *string, max int, multiline bool) error {
	*s = strings.TrimSpace(*s)
	if !utf8.ValidString(*s) || utf8.RuneCountInString(*s) > max {
		return ErrInvalidInput
	}
	for _, r := range *s {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t')) {
			return ErrInvalidInput
		}
	}
	return nil
}
func email(s *string) error {
	if err := text(s, 254, false); err != nil || *s == "" {
		return err
	}
	if a, err := mail.ParseAddress(*s); err != nil || a.Address != *s {
		return ErrInvalidInput
	}
	return nil
}
func date(s *string) error {
	*s = strings.TrimSpace(*s)
	if _, err := time.Parse(time.DateOnly, *s); err != nil {
		return ErrInvalidInput
	}
	return nil
}
func check(errs ...error) error { return errors.Join(errs...) }

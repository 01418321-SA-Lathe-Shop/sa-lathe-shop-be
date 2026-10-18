package payment

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode/utf8"
)

func validID(id string) bool { _, err := uuid.Parse(id); return err == nil && len(id) == 36 }
func ValidateSubmission(terms q.Terms, in Submission) error {
	if !validID(in.IdempotencyKey) || !validID(in.AttachmentID) {
		return req.ErrInvalidInput
	}
	switch in.Kind {
	case "deposit":
		if !terms.DepositRequired || in.Amount == nil || in.PaidAt == nil || in.PaidAt.IsZero() || in.PaidAt.Year() < 1 || in.PaidAt.Year() > 9999 {
			return req.ErrInvalidInput
		}
		if _, err := in.Amount.Subtract(terms.DepositAmount); err != nil {
			return req.ErrInvalidInput
		}
		if _, err := terms.FinalPrice.Subtract(*in.Amount); err != nil {
			return req.ErrInvalidInput
		}
	case "po":
		if !terms.PORequired || in.Amount != nil || in.PaidAt != nil {
			return req.ErrInvalidInput
		}
	default:
		return req.ErrInvalidInput
	}
	return nil
}
func ValidateReview(in Review) error {
	if in.ExpectedVersion != 1 || (in.Outcome != "approved" && in.Outcome != "rejected") || !utf8.ValidString(in.Reason) || strings.ContainsRune(in.Reason, 0) || utf8.RuneCountInString(in.Reason) > 2000 || (in.Outcome == "rejected" && in.Reason == "") || (in.Outcome == "approved" && in.Reason != "") {
		return req.ErrInvalidInput
	}
	return nil
}

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) Submit(ctx context.Context, a auth.Account, id string, in Submission) (Evidence, error) {
	if !a.Active || a.Role != auth.RoleCustomer {
		return Evidence{}, req.ErrForbidden
	}
	if !validID(id) || !validID(in.IdempotencyKey) || !validID(in.AttachmentID) || (in.Kind != "deposit" && in.Kind != "po") {
		return Evidence{}, req.ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	if in.PaidAt != nil {
		value := in.PaidAt.UTC().Truncate(time.Microsecond)
		in.PaidAt = &value
	}
	in.AttachmentID = strings.ToLower(in.AttachmentID)
	// The repository loads immutable terms under the request lock and calls the
	// same domain validation before persisting. No SQL/DB driver enters services.
	return s.repo.SubmitEvidence(ctx, a.ID, id, in)
}
func (s *Service) Review(ctx context.Context, a auth.Account, id string, in Review) (Evidence, error) {
	if !a.Active || (a.Role != auth.RoleAdmin && a.Role != auth.RoleWorker) {
		return Evidence{}, req.ErrForbidden
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(id) {
		return Evidence{}, req.ErrInvalidInput
	}
	if err := ValidateReview(in); err != nil {
		return Evidence{}, err
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	return s.repo.ReviewEvidence(ctx, a.ID, id, in)
}
func (s *Service) Get(ctx context.Context, a auth.Account, id string) (Fulfillment, error) {
	if !a.Active || (a.Role != auth.RoleAdmin && a.Role != auth.RoleWorker && a.Role != auth.RoleCustomer) {
		return Fulfillment{}, req.ErrForbidden
	}
	if !validID(id) {
		return Fulfillment{}, req.ErrInvalidInput
	}
	return s.repo.GetFulfillment(ctx, a.ID, id)
}

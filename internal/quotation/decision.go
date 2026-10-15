package quotation

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"strings"
	"time"
	"unicode/utf8"
)

type DecisionInput struct {
	Outcome             string `json:"outcome"`
	Reason              string `json:"reason,omitempty"`
	ExpectedLockVersion int64  `json:"expected_lock_version"`
}
type Decision struct {
	QuotationID         string    `json:"quotation_id"`
	RequestID           string    `json:"request_id"`
	ActorID             string    `json:"actor_id"`
	Outcome             string    `json:"outcome"`
	Reason              string    `json:"reason,omitempty"`
	ExpectedLockVersion int64     `json:"expected_lock_version"`
	DecidedAt           time.Time `json:"decided_at"`
	RejectionCount      int       `json:"rejection_count"`
	RequestStatus       string    `json:"request_status"`
	Terms               *Terms    `json:"terms,omitempty"`
}
type DecisionRepository interface {
	DecideQuotation(context.Context, string, string, DecisionInput) (Decision, error)
	GetDecision(context.Context, string, string) (Decision, error)
	ListDecisions(context.Context, string, string, int, int) ([]Decision, error)
}
type DecisionService struct{ repo DecisionRepository }

// DecisionEffect is the domain policy; the repository supplies the locked audit
// count and persists this result atomically with the quotation and request.
func DecisionEffect(outcome string, previous int) (string, string, int, error) {
	if previous < 0 || previous >= 6 {
		return "", "", 0, ErrConflict
	}
	if outcome == "accept" {
		return "approved", "ongoing", previous, nil
	}
	if outcome != "reject" {
		return "", "", 0, ErrInvalidInput
	}
	count := previous + 1
	requestStatus := "ongoing"
	if count == 6 {
		requestStatus = "canceled"
	}
	return "rejected", requestStatus, count, nil
}
func NewDecisionService(repo DecisionRepository) *DecisionService {
	return &DecisionService{repo: repo}
}
func (s *DecisionService) Decide(ctx context.Context, a auth.Account, id string, in DecisionInput) (Decision, error) {
	if !a.Active || a.Role != auth.RoleCustomer || !validID(a.ID) {
		return Decision{}, ErrForbidden
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(id) || in.ExpectedLockVersion < 1 || (in.Outcome != "accept" && in.Outcome != "reject") || !utf8.ValidString(in.Reason) || strings.ContainsRune(in.Reason, 0) || utf8.RuneCountInString(in.Reason) > 2000 || (in.Outcome == "accept" && in.Reason != "") {
		return Decision{}, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}
	// Current active membership is checked under repository transaction locks.
	// Company ownership is not required, per explicit owner confirmation.
	return s.repo.DecideQuotation(ctx, a.ID, id, in)
}
func (s *DecisionService) Get(ctx context.Context, a auth.Account, id string) (Decision, error) {
	if !reader(a) {
		return Decision{}, ErrForbidden
	}
	if !validID(id) {
		return Decision{}, ErrInvalidInput
	}
	return s.repo.GetDecision(ctx, a.ID, id)
}
func (s *DecisionService) List(ctx context.Context, a auth.Account, requestID string, limit, offset int) ([]Decision, error) {
	if !reader(a) {
		return nil, ErrForbidden
	}
	if !validID(requestID) || limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalidInput
	}
	return s.repo.ListDecisions(ctx, a.ID, requestID, limit, offset)
}

package production

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"time"
)

var ErrConflict = errors.New("production state or command conflict")

type Command struct {
	IdempotencyKey  string `json:"idempotency_key"`
	ExpectedVersion int64  `json:"expected_version"`
}
type StartInput struct {
	Command
	ExpectedPlanVersion *int64 `json:"expected_plan_version"`
}
type UsageLine struct {
	MaterialID string     `json:"material_id"`
	Quantity   *m.Decimal `json:"quantity"`
}
type UsageInput struct {
	Command
	Lines []UsageLine `json:"lines"`
	Note  string      `json:"note"`
}
type TimeInput struct {
	Command
	Minutes int64  `json:"minutes"`
	Note    string `json:"note"`
}
type LaborInput struct {
	Command
	Amount *q.Amount `json:"amount"`
}

// Action is constructed by the service, never decoded from an HTTP request.
type Action struct {
	Command
	Kind        string      `json:"kind"`
	PlanVersion int64       `json:"plan_version,omitempty"`
	Lines       []UsageLine `json:"lines,omitempty"`
	Minutes     int64       `json:"minutes,omitempty"`
	Amount      *q.Amount   `json:"amount,omitempty"`
	Note        string      `json:"note,omitempty"`
}
type Summary struct {
	JobID       string     `json:"job_id"`
	RequestID   string     `json:"request_id"`
	Status      string     `json:"status"`
	Version     int64      `json:"version"`
	PlanVersion int64      `json:"material_plan_version"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
type MaterialState struct {
	MaterialID          string    `json:"material_id"`
	Name                string    `json:"name"`
	UnitName            string    `json:"unit_name"`
	Required            m.Decimal `json:"required"`
	Reserved            m.Decimal `json:"reserved"`
	ReservationConsumed m.Decimal `json:"reservation_consumed"`
	Used                m.Decimal `json:"used"`
	Released            m.Decimal `json:"released"`
}
type Usage struct {
	ID              string     `json:"id"`
	MaterialID      string     `json:"material_id"`
	MaterialName    string     `json:"material_name"`
	UnitName        string     `json:"unit_name"`
	Quantity        m.Decimal  `json:"quantity"`
	ReservationUsed m.Decimal  `json:"reservation_used"`
	ActorID         string     `json:"actor_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UnitCost        *m.Decimal `json:"unit_cost,omitempty"`
	Total           *q.Amount  `json:"total_amount,omitempty"`
}
type TimeEntry struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Minutes   int64     `json:"minutes"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}
type Finance struct {
	MaterialCost q.Amount  `json:"material_cost"`
	LaborCost    *q.Amount `json:"labor_cost"`
	TotalCost    *q.Amount `json:"total_cost"`
}
type Photo struct {
	ID         string `json:"id"`
	JobID      string `json:"job_id"`
	UploadedBy string `json:"uploaded_by"`
	file.Input
	StagingKey string    `json:"-"`
	ObjectKey  string    `json:"-"`
	State      string    `json:"state"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type State struct {
	Summary
	Materials    []MaterialState `json:"materials"`
	Usage        []Usage         `json:"usage"`
	TimeEntries  []TimeEntry     `json:"time_entries"`
	LaborMinutes int64           `json:"labor_minutes"`
	Events       []Event         `json:"events"`
	Photos       []Photo         `json:"photos"`
	Finance      *Finance        `json:"finance,omitempty"`
}
type Repository interface {
	ProductionJobs(context.Context, string, int, int) ([]Summary, error)
	ProductionState(context.Context, string, string) (State, error)
	ApplyProduction(context.Context, string, string, Action) (State, error)
}
type PhotoRepository interface {
	CreateJobPhoto(context.Context, Photo) error
	BeginJobPhoto(context.Context, string, string) (Photo, error)
	FinishJobPhoto(context.Context, string, string, bool) error
	ReadJobPhoto(context.Context, string, string) (Photo, error)
}

// Package dashboard provides read-only, role-scoped production reporting.
package dashboard

import (
	"context"
	"time"
)

type Counts struct {
	Queued     int64 `json:"queued"`
	InProgress int64 `json:"in_progress"`
	Completed  int64 `json:"completed"`
	Cancelled  int64 `json:"cancelled"`
	Total      int64 `json:"total"`
}
type Queue struct {
	PendingRequests int64 `json:"pending_requests"`
	QueuedJobs      int64 `json:"queued_jobs"`
	ReceivingOrders int64 `json:"receiving_orders"`
}
type Period struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
type Day struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
type CompletionSummary struct {
	Total          int64   `json:"total"`
	BestDay        *string `json:"best_day"`
	BestDayCount   int64   `json:"best_day_count"`
	CurrentStreak  *int64  `json:"current_streak"`
	AveragePerWeek *string `json:"average_per_week"`
}
type Heatmap struct {
	StartDate          string            `json:"start_date"`
	EndDate            string            `json:"end_date"`
	Days               []Day             `json:"days"`
	HistoryComplete    bool              `json:"history_complete"`
	MissingHistoryJobs int64             `json:"missing_history_jobs"`
	Summary            CompletionSummary `json:"summary"`
}
type Overview struct {
	AsOf    time.Time `json:"as_of"`
	Range   string    `json:"range"`
	Period  Period    `json:"period"`
	Counts  Counts    `json:"counts"`
	Queue   Queue     `json:"queue"`
	Heatmap Heatmap   `json:"heatmap"`
}
type Job struct {
	JobID            string     `json:"job_id"`
	RequestID        string     `json:"request_id"`
	RequestNumber    int64      `json:"request_number"`
	Description      string     `json:"description"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	FirstCompletedAt *time.Time `json:"first_completed_at"`
}
type Jobs struct {
	AsOf   time.Time `json:"as_of"`
	Items  []Job     `json:"items"`
	Total  int64     `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}
type Window struct {
	Range       string
	From, AsOf  time.Time
	HeatmapFrom time.Time
	Today       time.Time
}
type Filter struct {
	Window
	Status, FirstCompletedOn string
	Limit, Offset            int
}
type Repository interface {
	DashboardOverview(context.Context, string, Window) (Overview, error)
	DashboardJobs(context.Context, string, Filter) (Jobs, error)
}

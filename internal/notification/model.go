package notification

import (
	"context"
	"time"
)

type Event struct {
	ID, RequestID, LeaseToken, Channel string
	RequestNumber                      int64
	Attempts                           int
}
type Item struct {
	ID        string     `json:"id"`
	RequestID string     `json:"request_id"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}
type Repository interface {
	ClaimEvent(context.Context) (Event, error)
	DeliverInbox(context.Context, Event) error
	CompleteEvent(context.Context, Event) error
	RetryEvent(context.Context, Event, bool) error
	ListNotifications(context.Context, string, int, int) ([]Item, error)
	ReadNotification(context.Context, string, string) error
}
type Sender interface {
	Send(context.Context, Event) error
}

package notification

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var ErrEmpty = errors.New("no pending event")
var ErrDisabled = errors.New("LINE delivery is disabled")

type Dispatcher struct {
	repo   Repository
	line   Sender
	logger *slog.Logger
}

func NewDispatcher(repo Repository, line Sender, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{repo: repo, line: line, logger: logger}
}
func (d *Dispatcher) Once(ctx context.Context) error {
	e, err := d.repo.ClaimEvent(ctx)
	if err != nil {
		return err
	}
	if e.Channel == "in_system" {
		err = d.repo.DeliverInbox(ctx, e)
	} else {
		err = d.line.Send(ctx, e)
		if err == nil {
			err = d.repo.CompleteEvent(ctx, e)
		}
	}
	if err != nil {
		// Do not log sender error bodies, tokens, recipient IDs or signed links.
		disabled := errors.Is(err, ErrDisabled)
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		retryErr := d.repo.RetryEvent(cleanup, e, disabled)
		if !disabled {
			d.logger.Warn("notification delivery deferred", "event_id", e.ID, "channel", e.Channel)
		}
		if retryErr != nil {
			return retryErr
		}
	}
	return nil
}
func (d *Dispatcher) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		err := d.Once(ctx)
		if err != nil && !errors.Is(err, ErrEmpty) {
			d.logger.Warn("notification worker database operation failed")
		}
		if errors.Is(err, ErrEmpty) || err != nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
		}
	}
}

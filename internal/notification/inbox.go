package notification

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
)

type Inbox struct{ repo Repository }

func NewInbox(repo Repository) *Inbox { return &Inbox{repo: repo} }
func (i *Inbox) List(ctx context.Context, a auth.Account, limit, offset int) ([]Item, error) {
	if !a.Active || a.ID == "" {
		return nil, req.ErrForbidden
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, req.ErrInvalidInput
	}
	return i.repo.ListNotifications(ctx, a.ID, limit, offset)
}
func (i *Inbox) Read(ctx context.Context, a auth.Account, id string) error {
	if !a.Active || a.ID == "" {
		return req.ErrForbidden
	}
	if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
		return req.ErrInvalidInput
	}
	return i.repo.ReadNotification(ctx, a.ID, id)
}

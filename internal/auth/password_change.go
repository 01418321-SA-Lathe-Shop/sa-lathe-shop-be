package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrCurrentPassword is a 400, never a 401: the caller's session stays valid.
var ErrCurrentPassword = errors.New("current password is incorrect")

// ChangePassword replaces the actor's own password after verifying the current
// one, then revokes every other session of the account. The calling session
// identified by token stays active.
func (s *Service) ChangePassword(ctx context.Context, actor Account, token, current, next string) error {
	if !actor.Active || len(current) < 1 || len(current) > 128 {
		return ErrCurrentPassword
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	if next == current {
		return fmt.Errorf("%w: new_password must differ from current_password", ErrInvalidInput)
	}
	if strings.Contains(strings.ToLower(next), actor.Username) {
		return fmt.Errorf("%w: new_password must not contain the username", ErrInvalidInput)
	}
	account, err := s.repo.FindAccount(ctx, actor.Username)
	if errors.Is(err, ErrNotFound) || err == nil && (account.ID != actor.ID || !account.Active) {
		return ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	match, err := s.hasher.Verify(ctx, account.PasswordHash, current)
	if err != nil {
		return err
	}
	if !match {
		return ErrCurrentPassword
	}
	hash, err := s.hasher.Hash(ctx, next)
	if err != nil {
		return err
	}
	// Repository rechecks the hash under a row lock so a concurrent change wins once.
	return s.repo.ChangePassword(ctx, account.ID, account.PasswordHash, hash, digest(token))
}

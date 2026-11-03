package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// UC12: only an active Super Admin manages other Admins.
func requireSuperAdmin(actor Account) error {
	if err := requireAdmin(actor); err != nil || !actor.SuperAdmin {
		return ErrForbidden
	}
	return nil
}
func (s *Service) ListAdmins(ctx context.Context, actor Account) ([]Admin, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}
	return s.repo.ListAdmins(ctx)
}

// CreateAdmin always creates a regular Admin; Super Admins come only from bootstrap or migration.
func (s *Service) CreateAdmin(ctx context.Context, actor Account, input AdminInput) (Admin, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return Admin{}, err
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return Admin{}, err
	}
	name := strings.TrimSpace(input.Name)
	if err = validateName(name); err != nil {
		return Admin{}, err
	}
	if err = validatePassword(input.Password); err != nil {
		return Admin{}, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return Admin{}, err
	}
	phone, err := normalizePhone(input.Phone)
	if err != nil {
		return Admin{}, err
	}
	hash, err := s.hasher.Hash(ctx, input.Password)
	if err != nil {
		return Admin{}, err
	}
	return s.repo.CreateAdmin(ctx, NewStaff{Account: Account{ID: uuid.NewString(), PersonID: uuid.NewString(), Username: username, Name: name, PasswordHash: hash, Role: RoleAdmin, Active: true}, Email: email, Phone: phone})
}

// SetAdminActive refuses Super Admins, including the actor, so a Super Admin always remains.
func (s *Service) SetAdminActive(ctx context.Context, actor Account, id string, active *bool) (Admin, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return Admin{}, err
	}
	if err := validateID(id); err != nil {
		return Admin{}, err
	}
	if active == nil {
		return Admin{}, ErrInvalidInput
	}
	return s.repo.SetAdminActive(ctx, id, *active)
}

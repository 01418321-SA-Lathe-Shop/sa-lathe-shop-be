package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func (f *fakeRepo) ListAdmins(context.Context) ([]Admin, error) {
	result := []Admin{}
	for _, a := range f.accounts {
		if a.Role == RoleAdmin {
			result = append(result, Admin{Account: a})
		}
	}
	return result, nil
}
func (f *fakeRepo) CreateAdmin(_ context.Context, input NewStaff) (Admin, error) {
	if _, ok := f.accounts[input.Account.Username]; ok {
		return Admin{}, ErrConflict
	}
	f.accounts[input.Account.Username] = input.Account
	return Admin{Account: input.Account, Email: &input.Email, Phone: &input.Phone}, nil
}
func (f *fakeRepo) SetAdminActive(_ context.Context, id string, active bool) (Admin, error) {
	for username, a := range f.accounts {
		if a.ID == id && a.Role == RoleAdmin {
			if a.SuperAdmin {
				return Admin{}, ErrForbidden
			}
			a.Active = active
			f.accounts[username] = a
			return Admin{Account: a}, nil
		}
	}
	return Admin{}, ErrNotFound
}

func TestOnlySuperAdminManagesAdmins(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	s, err := NewService(repo, fastHasher{}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	super := Account{ID: "57039d8b-c58e-4ee1-8943-539dc6203626", Role: RoleAdmin, Active: true, SuperAdmin: true}
	input := AdminInput{Username: "second.admin", Password: "a-long-test-password", Name: " Second ", Email: "second@example.com", Phone: "0812345678"}
	for _, actor := range []Account{{Role: RoleAdmin, Active: true}, {Role: RoleWorker, Active: true, SuperAdmin: true}, {Role: RoleAdmin, SuperAdmin: true}} {
		if _, err := s.CreateAdmin(ctx, actor, input); !errors.Is(err, ErrForbidden) {
			t.Fatalf("non-super actor %+v created an admin: %v", actor, err)
		}
		if _, err := s.ListAdmins(ctx, actor); !errors.Is(err, ErrForbidden) {
			t.Fatalf("non-super actor %+v listed admins: %v", actor, err)
		}
	}
	created, err := s.CreateAdmin(ctx, super, input)
	if err != nil || created.Role != RoleAdmin || created.SuperAdmin || created.Name != "Second" || created.PasswordHash == "" {
		t.Fatalf("create admin = %+v, %v", created, err)
	}
	if _, err := s.CreateAdmin(ctx, super, AdminInput{Username: "third.admin", Password: "a-long-test-password", Name: "Third"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("admin without contacts accepted: %v", err)
	}
	off := false
	if v, err := s.SetAdminActive(ctx, super, created.ID, &off); err != nil || v.Active {
		t.Fatalf("deactivate = %+v, %v", v, err)
	}
	if _, err := s.SetAdminActive(ctx, super, created.ID, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing is_active accepted: %v", err)
	}
	repo.accounts["root"] = super
	if _, err := s.SetAdminActive(ctx, super, super.ID, &off); !errors.Is(err, ErrForbidden) {
		t.Fatalf("super admin deactivated: %v", err)
	}
}

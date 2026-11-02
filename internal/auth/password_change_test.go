package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestChangePassword(t *testing.T) {
	s, _, admin := setupService(t)
	ctx := context.Background()
	current, err := s.Login(ctx, "admin.one", "Test-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Login(ctx, "admin.one", "Test-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		current, next string
		want          error
	}{
		{"Wrong-password-1234", "New-password-5678", ErrCurrentPassword},
		{"", "New-password-5678", ErrCurrentPassword},
		{"Test-password-1234", "short", ErrInvalidInput},
		{"Test-password-1234", "Test-password-1234", ErrInvalidInput},
		{"Test-password-1234", "my-ADMIN.ONE-secret", ErrInvalidInput},
	} {
		if err := s.ChangePassword(ctx, admin, current.AccessToken, tc.current, tc.next); !errors.Is(err, tc.want) {
			t.Fatalf("%q -> %q: got %v want %v", tc.current, tc.next, err, tc.want)
		}
	}
	if err = s.ChangePassword(ctx, admin, current.AccessToken, "Test-password-1234", "New-password-5678"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, current.AccessToken); err != nil {
		t.Fatal("calling session must stay active")
	}
	if _, err = s.Authenticate(ctx, other.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("other sessions must be revoked")
	}
	if _, err = s.Login(ctx, "admin.one", "Test-password-1234"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("old password still works")
	}
	if _, err = s.Login(ctx, "admin.one", "New-password-5678"); err != nil {
		t.Fatal(err)
	}
}

func TestChangePasswordHTTP(t *testing.T) {
	s, _, _ := setupService(t)
	r := chi.NewRouter()
	NewHandler(s).Routes(r)
	login, err := s.Login(context.Background(), "admin.one", "Test-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		token, body string
		status      int
	}{
		{"", `{"current_password":"Test-password-1234","new_password":"New-password-5678"}`, 401},
		{login.AccessToken, `{"current_password":"Wrong-password-1234","new_password":"New-password-5678"}`, 400},
		{login.AccessToken, `{"current_password":"Test-password-1234","new_password":"New-password-5678","account_id":"x"}`, 400},
		{login.AccessToken, `{"current_password":"Test-password-1234","new_password":"New-password-5678"}`, 204},
	} {
		req := httptest.NewRequest("POST", "/auth/password", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "password-") {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body.String())
		}
		if tc.status == 400 && strings.Contains(tc.body, "Wrong") && !strings.Contains(w.Body.String(), "invalid_current_password") {
			t.Fatal("wrong current password must have its own code")
		}
	}
}

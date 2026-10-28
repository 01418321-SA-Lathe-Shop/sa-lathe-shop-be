package auth

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type localTestRepo struct {
	calls   int
	session Session
}

func (r *localTestRepo) CreateLocalTestSession(_ context.Context, s Session, username string) (Account, error) {
	r.calls++
	r.session = s
	if username != "pim.admin" {
		return Account{}, ErrUnauthenticated
	}
	return Account{Username: username, Role: RoleAdmin, Active: true, PasswordHash: "must-not-leak"}, nil
}

func TestLocalTestDisabled(t *testing.T) {
	repo := &localTestRepo{}
	s := NewLocalTestService(repo, false, time.Hour)
	router := chi.NewRouter()
	s.Routes(router)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/auth/local-test/login", strings.NewReader(`{"username":"pim.admin"}`)))
	if w.Code != 404 {
		t.Fatal("disabled route exists")
	}
	if _, err := s.Login(context.Background(), "pim.admin"); err != ErrUnauthenticated || repo.calls != 0 {
		t.Fatal("disabled service called repository")
	}
}

func TestLocalTestLogin(t *testing.T) {
	repo := &localTestRepo{}
	s := NewLocalTestService(repo, true, time.Hour)
	result, err := s.Login(context.Background(), "pim.admin")
	if err != nil || !validToken(result.AccessToken) || result.Account.PasswordHash != "" || result.TokenType != "Bearer" {
		t.Fatal("invalid session result")
	}
	if string(repo.session.TokenHash) != string(digest(result.AccessToken)) || repo.session.ID == "" || repo.session.ExpiresAt.Sub(repo.session.CreatedAt) != time.Hour {
		t.Fatal("session contract differs from normal auth")
	}
	router := chi.NewRouter()
	s.Routes(router)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"username":"pim.admin"}`, 200},
		{`{"username":"unowned"}`, 401},
		{`{"username":"pim.admin","role":"admin"}`, 400},
		{`{}`, 401},
		{`{`, 400},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth/local-test/login", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "must-not-leak") {
			t.Fatalf("unexpected status %d expected %d", w.Code, tc.status)
		}
	}
	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", "/auth/local-test/login", strings.NewReader(`{"username":"pim.admin"}`)))
		if i == 5 && (w.Code != 429 || w.Header().Get("Retry-After") != "60") {
			t.Fatal("missing rate limit")
		}
	}
}

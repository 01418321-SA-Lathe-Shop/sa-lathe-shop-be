package auth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
)

type Service struct {
	repo      Repository
	hasher    PasswordHasher
	ttl       time.Duration
	dummyHash string
	now       func() time.Time
}

func NewService(repo Repository, hasher PasswordHasher, ttl time.Duration) (*Service, error) {
	if ttl <= 0 {
		return nil, ErrInvalidInput
	}
	token, _, err := newToken()
	if err != nil {
		return nil, err
	}
	dummy, err := hasher.Hash(context.Background(), token)
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, hasher: hasher, ttl: ttl, dummyHash: dummy, now: time.Now}, nil
}
func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	username, err := normalizeUsername(username)
	if err != nil || len(password) < 1 || len(password) > 128 {
		return LoginResult{}, ErrUnauthenticated
	}
	account, err := s.repo.FindAccount(ctx, username)
	missing := errors.Is(err, ErrNotFound)
	if err != nil && !missing {
		return LoginResult{}, err
	}
	hash := account.PasswordHash
	if missing || !account.Active {
		hash = s.dummyHash
	}
	match, err := s.hasher.Verify(ctx, hash, password)
	if err != nil {
		return LoginResult{}, err
	}
	if !match || missing || !account.Active {
		return LoginResult{}, ErrUnauthenticated
	}
	token, hashBytes, err := newToken()
	if err != nil {
		return LoginResult{}, err
	}
	now := s.now().UTC()
	expires := now.Add(s.ttl)
	session := Session{ID: uuid.NewString(), TokenHash: hashBytes, Account: account, CreatedAt: now, ExpiresAt: expires}
	// Repository rechecks active state/hash under a row lock before creating a session.
	if err = s.repo.CreateSession(ctx, session, account.PasswordHash); err != nil {
		return LoginResult{}, err
	}
	account.PasswordHash = ""
	return LoginResult{AccessToken: token, TokenType: "Bearer", ExpiresAt: expires, Account: account}, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (Account, error) {
	result, err := s.Resume(ctx, token)
	return result.Account, err
}

// Resume validates an existing session token without extending it (used to
// restore a browser's cookie session into memory after reload).
func (s *Service) Resume(ctx context.Context, token string) (LoginResult, error) {
	if !validToken(token) {
		return LoginResult{}, ErrUnauthenticated
	}
	session, err := s.repo.FindSession(ctx, digest(token))
	if errors.Is(err, ErrNotFound) {
		return LoginResult{}, ErrUnauthenticated
	}
	if err != nil {
		return LoginResult{}, err
	}
	if !session.Account.Active || session.RevokedAt != nil || !s.now().Before(session.ExpiresAt) {
		return LoginResult{}, ErrUnauthenticated
	}
	if session.Account.Role != RoleAdmin && session.Account.Role != RoleWorker && session.Account.Role != RoleCustomer {
		return LoginResult{}, ErrUnauthenticated
	}
	session.Account.PasswordHash = ""
	return LoginResult{AccessToken: token, TokenType: "Bearer", ExpiresAt: session.ExpiresAt, Account: session.Account}, nil
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if !validToken(token) {
		return ErrUnauthenticated
	}
	return s.repo.RevokeSession(ctx, digest(token))
}
func (s *Service) BootstrapAdmin(ctx context.Context, username, name, password string) (Account, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return Account{}, err
	}
	if err = validateName(name); err != nil {
		return Account{}, err
	}
	if err = validatePassword(password); err != nil {
		return Account{}, err
	}
	hash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return Account{}, err
	}
	return s.repo.BootstrapAdmin(ctx, NewStaff{Account: Account{ID: uuid.NewString(), PersonID: uuid.NewString(), Username: username, Name: name, PasswordHash: hash, Role: RoleAdmin, Active: true}})
}

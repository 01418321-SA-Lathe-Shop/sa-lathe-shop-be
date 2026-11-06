package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// LocalTestRepository must atomically verify seed ownership and active identity
// before storing a normal session. Username matching alone is never sufficient.
type LocalTestRepository interface {
	CreateLocalTestSession(context.Context, Session, string) (Account, error)
}

type LocalTestService struct {
	repo    LocalTestRepository
	enabled bool
	ttl     time.Duration
}

func NewLocalTestService(repo LocalTestRepository, enabled bool, ttl time.Duration) *LocalTestService {
	return &LocalTestService{repo: repo, enabled: enabled, ttl: ttl}
}

func (s *LocalTestService) Login(ctx context.Context, username string) (LoginResult, error) {
	if !s.enabled || s.ttl <= 0 {
		return LoginResult{}, ErrUnauthenticated
	}
	username, err := normalizeUsername(username)
	if err != nil {
		return LoginResult{}, ErrUnauthenticated
	}
	token, hash, err := newToken()
	if err != nil {
		return LoginResult{}, err
	}
	now := time.Now().UTC()
	session := Session{ID: uuid.NewString(), TokenHash: hash, CreatedAt: now, ExpiresAt: now.Add(s.ttl)}
	account, err := s.repo.CreateLocalTestSession(ctx, session, username)
	if err != nil {
		return LoginResult{}, err
	}
	account.PasswordHash = ""
	return LoginResult{AccessToken: token, TokenType: "Bearer", ExpiresAt: session.ExpiresAt, Account: account}, nil
}

// Disabled means no HTTP route at all, not a hidden UI button. Normal password
// login, session verification, role gates and logout remain unchanged.
func (s *LocalTestService) Routes(r chi.Router) {
	if !s.enabled {
		return
	}
	r.With(NewLoginLimiter().Middleware).Post("/auth/local-test/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var input struct {
			Username string `json:"username"`
		}
		if err := decode(w, r, &input); err != nil {
			writeAuthError(w, r, err)
			return
		}
		result, err := s.Login(r.Context(), input.Username)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		setSessionCookie(w, result.AccessToken, result.ExpiresAt)
		httpserver.JSON(w, http.StatusOK, result)
	})
}

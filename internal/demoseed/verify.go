package demoseed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

var ErrVerification = errors.New("real API verification failed; check target, credentials, roles and membership (response bodies withheld)")

func verifyOrigin(base string) bool {
	u, e := url.Parse(base)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == ""
}

// VerifyHTTP is a real HTTP client, not a mock. It does not create business
// records. Tokens exist only in memory; every successfully decoded login gets
// a best-effort logout, including errors. Revoked session rows are retained.
func (s *Service) VerifyHTTP(ctx context.Context, base string, get func(string) string) error {
	if !verifyOrigin(base) {
		return ErrGuard
	}
	expected := map[string]auth.Account{}
	e := s.repo.Within(ctx, s.cfg, func(store Store) error {
		status, e := store.Inspect(ctx)
		if e != nil {
			return e
		}
		if status.State != "ready" {
			return ErrConflict
		}
		for _, spec := range Accounts() {
			a, e := store.FindAccount(ctx, spec.Username)
			if e != nil {
				return e
			}
			a.PasswordHash = ""
			expected[spec.Username] = a
		}
		return nil
	})
	if e != nil {
		return e
	}
	// Release DB locks before login, which writes a normal auth_session row.
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	profiles := map[string][]auth.CustomerProfile{}
	for _, spec := range Accounts() {
		p, e := verifyAccount(ctx, client, base, spec, expected[spec.Username], get(spec.PasswordEnv))
		if e != nil {
			return e
		}
		profiles[spec.Username] = p
	}
	nida, ploy, somchai := profiles["nida.company"], profiles["ploy.company"], profiles["somchai"]
	if len(nida) != 1 || len(ploy) != 1 || len(somchai) != 1 || nida[0].ID != ploy[0].ID || nida[0].ID == somchai[0].ID || !nida[0].Owner || ploy[0].Owner || !somchai[0].Owner || nida[0].Type != "company" || somchai[0].Type != "individual" {
		return ErrVerification
	}
	return nil
}

func verifyAccount(ctx context.Context, client *http.Client, base string, spec AccountSpec, expected auth.Account, password string) ([]auth.CustomerProfile, error) {
	var login auth.LoginResult
	if e := verifyCall(ctx, client, base, "POST", "/auth/login", "", map[string]string{"username": spec.Username, "password": password}, 200, &login); e != nil {
		return nil, e
	}
	if login.AccessToken == "" {
		return nil, ErrVerification
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = verifyCall(cleanup, client, base, "POST", "/auth/logout", login.AccessToken, nil, 204, nil)
	}()
	if login.TokenType != "Bearer" || login.Account.ID != expected.ID || login.Account.Username != expected.Username || login.Account.Role != spec.Role {
		return nil, ErrVerification
	}
	var me auth.Account
	if e := verifyCall(ctx, client, base, "GET", "/auth/me", login.AccessToken, nil, 200, &me); e != nil {
		return nil, e
	}
	if me.ID != expected.ID || me.Role != spec.Role || !me.Active {
		return nil, ErrVerification
	}
	for _, path := range []string{"/admin/workers", "/materials", "/suppliers", "/production/jobs", "/notifications"} {
		want := 200
		if path == "/admin/workers" || path == "/suppliers" {
			if spec.Role != "admin" {
				want = 403
			}
		}
		if (path == "/materials" || path == "/production/jobs") && spec.Role == "customer" {
			want = 403
		}
		if e := verifyCall(ctx, client, base, "GET", path, login.AccessToken, nil, want, nil); e != nil {
			return nil, e
		}
	}
	want := 200
	if spec.Role != "customer" {
		want = 403
	}
	var profiles struct {
		Items []auth.CustomerProfile `json:"items"`
	}
	if e := verifyCall(ctx, client, base, "GET", "/customers", login.AccessToken, nil, want, &profiles); e != nil {
		return nil, e
	}
	if e := verifyCall(ctx, client, base, "POST", "/auth/logout", login.AccessToken, nil, 204, nil); e != nil {
		return nil, e
	}
	if e := verifyCall(ctx, client, base, "GET", "/auth/me", login.AccessToken, nil, 401, nil); e != nil {
		return nil, e
	}
	return profiles.Items, nil
}

func verifyCall(ctx context.Context, client *http.Client, base, method, path, token string, body any, want int, out any) error {
	var data []byte
	var e error
	if body != nil {
		data, e = json.Marshal(body)
		if e != nil {
			return ErrVerification
		}
	}
	r, e := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(data))
	if e != nil {
		return ErrVerification
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	response, e := client.Do(r)
	if e != nil {
		return ErrVerification
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		return ErrVerification
	}
	if out != nil && want == 200 {
		if e = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(out); e != nil {
			return ErrVerification
		}
	}
	return nil
}

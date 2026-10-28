// Package demoseed orchestrates explicit local/test fixtures, never API startup.
package demoseed

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrGuard      = errors.New("seed safety guard refused the target or migration state")
	ErrConflict   = errors.New("seed ownership or identity conflict; no records adopted or repaired")
	ErrDependency = errors.New("cleanup refused: dependent or modified records must be retained")
)

type Config struct {
	DatabaseURL string `json:"-"`
	Database    string
	Environment string
}

func LoadConfig(get func(string) string) (Config, error) {
	c := Config{DatabaseURL: get("SEED_DATABASE_URL"), Environment: get("SEED_ENV")}
	if get("SEED_ENABLED") != "true" || (c.Environment != "local" && c.Environment != "test") {
		return Config{}, ErrGuard
	}
	u, e := url.Parse(c.DatabaseURL)
	if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.Fragment != "" {
		return Config{}, ErrGuard
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return Config{}, ErrGuard
	}
	c.Database = strings.TrimPrefix(u.Path, "/")
	if (c.Database != "sa_lathe_shop_demo" && c.Database != "sa_lathe_shop_seed_test") || get("SEED_CONFIRM_DATABASE") != c.Database {
		return Config{}, ErrGuard
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return Config{}, ErrGuard
	}
	// Disallow libpq/pgx host, service, search_path and database overrides.
	for key, values := range q {
		if (key != "sslmode" && key != "connect_timeout") || len(values) != 1 {
			return Config{}, ErrGuard
		}
	}
	return c, nil
}

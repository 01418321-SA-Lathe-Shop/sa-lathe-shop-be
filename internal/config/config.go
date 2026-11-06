package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr                                    string
	HTTPTimeout, ShutdownTimeout, DBPingTimeout time.Duration
	CORSOrigins                                 []string
	DatabaseURL                                 string
	DBMaxConnections                            int32
	SessionTTL                                  time.Duration
	LocalTestAccountEnable                      bool
	// SQLConsoleEnable exposes /admin/sql (Super Admin); SQLConsoleDatabaseURL optionally uses a separate least-privilege role.
	SQLConsoleEnable      bool
	SQLConsoleDatabaseURL string
}

func Load() (Config, error) { return load(os.Getenv) }

func load(get func(string) string) (Config, error) {
	c := Config{HTTPAddr: value(get, "HTTP_ADDR", ":8080"), DBMaxConnections: 10}
	switch get("LOCAL_TEST_ACCOUNT_ENABLE") {
	case "", "false":
	case "true":
		c.LocalTestAccountEnable = true
	default:
		return Config{}, fmt.Errorf("LOCAL_TEST_ACCOUNT_ENABLE must be true or false")
	}
	switch get("SQL_CONSOLE_ENABLE") {
	case "", "false":
	case "true":
		c.SQLConsoleEnable = true
		c.SQLConsoleDatabaseURL = get("SQL_CONSOLE_DATABASE_URL")
	default:
		return Config{}, fmt.Errorf("SQL_CONSOLE_ENABLE must be true or false")
	}
	var err error
	c.SessionTTL, err = time.ParseDuration(value(get, "AUTH_SESSION_TTL", "12h"))
	if err != nil || c.SessionTTL <= 0 || c.SessionTTL > 7*24*time.Hour {
		return Config{}, fmt.Errorf("AUTH_SESSION_TTL must be positive and at most 168h")
	}
	for key, dst := range map[string]*time.Duration{"HTTP_TIMEOUT": &c.HTTPTimeout, "SHUTDOWN_TIMEOUT": &c.ShutdownTimeout, "DB_PING_TIMEOUT": &c.DBPingTimeout} {
		fallback := "10s"
		if key == "HTTP_TIMEOUT" {
			fallback = "30s"
		}
		if key == "DB_PING_TIMEOUT" {
			fallback = "3s"
		}
		*dst, err = time.ParseDuration(value(get, key, fallback))
		if err != nil || *dst <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration", key)
		}
	}
	if _, _, err = net.SplitHostPort(c.HTTPAddr); err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port")
	}
	for _, origin := range strings.Split(value(get, "CORS_ORIGINS", "http://localhost:3000"), ",") {
		origin = strings.TrimSpace(origin)
		u, e := url.Parse(origin)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, fmt.Errorf("CORS_ORIGINS must contain explicit HTTP origins")
		}
		c.CORSOrigins = append(c.CORSOrigins, origin)
	}
	max, err := strconv.ParseInt(value(get, "DB_MAX_CONNECTIONS", "10"), 10, 32)
	if err != nil || max <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_CONNECTIONS must be positive")
	}
	c.DBMaxConnections = int32(max)
	if raw := get("DATABASE_URL"); raw != "" {
		u, e := url.Parse(raw)
		if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
			return Config{}, fmt.Errorf("invalid DATABASE_URL")
		}
		c.DatabaseURL = raw
	} else {
		u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(value(get, "DB_HOST", "localhost"), value(get, "DB_PORT", "55432")), Path: "/" + value(get, "DB_NAME", "sa_lathe_shop"), User: url.UserPassword(value(get, "DB_USER", "sa_lathe_shop"), get("DB_PASSWORD"))}
		q := url.Values{"sslmode": {value(get, "DB_SSLMODE", "disable")}}
		u.RawQuery = q.Encode()
		c.DatabaseURL = u.String()
	}
	if c.LocalTestAccountEnable {
		u, e := url.Parse(c.DatabaseURL)
		if e != nil || (u.Path != "/sa_lathe_shop_demo" && u.Path != "/sa_lathe_shop_seed_test") {
			return Config{}, fmt.Errorf("LOCAL_TEST_ACCOUNT_ENABLE requires a dedicated demo/test database")
		}
	}
	return c, nil
}

func value(get func(string) string, key, fallback string) string {
	if v := get(key); v != "" {
		return v
	}
	return fallback
}

package config

import "testing"

func TestInvalidConfiguration(t *testing.T) {
	for key, value := range map[string]string{"AUTH_SESSION_TTL": "169h", "HTTP_TIMEOUT": "0s", "HTTP_ADDR": "invalid", "DB_MAX_CONNECTIONS": "-1", "CORS_ORIGINS": "*", "DATABASE_URL": "https://invalid"} {
		t.Run(key, func(t *testing.T) {
			_, err := load(func(k string) string {
				if k == key {
					return value
				}
				return ""
			})
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestLocalTestAccountFlag(t *testing.T) {
	for _, tc := range []struct {
		flag, db         string
		enabled, invalid bool
	}{
		{"", "sa_lathe_shop", false, false},
		{"false", "sa_lathe_shop", false, false},
		{"true", "sa_lathe_shop_demo", true, false},
		{"true", "sa_lathe_shop_seed_test", true, false},
		{"true", "sa_lathe_shop", false, true},
		{"TRUE", "sa_lathe_shop_demo", false, true},
		{"1", "sa_lathe_shop_demo", false, true},
	} {
		c, err := load(func(k string) string {
			if k == "LOCAL_TEST_ACCOUNT_ENABLE" {
				return tc.flag
			}
			if k == "DB_NAME" {
				return tc.db
			}
			return ""
		})
		if (err != nil) != tc.invalid || c.LocalTestAccountEnable != tc.enabled {
			t.Fatalf("unexpected flag guard for %q/%s", tc.flag, tc.db)
		}
	}
}

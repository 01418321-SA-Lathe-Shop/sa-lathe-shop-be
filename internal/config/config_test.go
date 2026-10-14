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

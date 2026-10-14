package config

import "testing"

func TestIntegrations(t *testing.T) {
	get := func(string) string { return "" }
	c, err := loadIntegrations(get)
	if err != nil || c.LINEEnabled || c.S3Bucket == "" {
		t.Fatal("invalid defaults")
	}
	for _, env := range []map[string]string{{"LINE_ENABLED": "true"}, {"LINE_ENABLED": "true", "LINE_CHANNEL_ACCESS_TOKEN": "test", "LINE_ADMIN_USER_ID": "Cgroup"}, {"S3_ENDPOINT": "http://user:password@example.com"}, {"S3_ACCESS_KEY": "only-one"}, {"S3_PATH_STYLE": "maybe"}} {
		if _, err := loadIntegrations(func(k string) string { return env[k] }); err == nil {
			t.Fatal("accepted invalid integration config")
		}
	}
}

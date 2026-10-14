package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
)

type Integrations struct {
	S3Endpoint, S3PublicEndpoint, S3Region, S3Bucket, S3AccessKey, S3SecretKey string
	S3PathStyle                                                                bool
	LINEEnabled                                                                bool
	LINEToken, LINEAdminUserID                                                 string
}

func LoadIntegrations() (Integrations, error) { return loadIntegrations(os.Getenv) }
func loadIntegrations(get func(string) string) (Integrations, error) {
	c := Integrations{S3Endpoint: value(get, "S3_ENDPOINT", "http://localhost:19000"), S3PublicEndpoint: get("S3_PUBLIC_ENDPOINT"), S3Region: value(get, "S3_REGION", "us-east-1"), S3Bucket: value(get, "S3_BUCKET", "sa-lathe-shop-private"), S3AccessKey: get("S3_ACCESS_KEY"), S3SecretKey: get("S3_SECRET_KEY"), LINEToken: get("LINE_CHANNEL_ACCESS_TOKEN"), LINEAdminUserID: get("LINE_ADMIN_USER_ID")}
	var err error
	c.S3PathStyle, err = strconv.ParseBool(value(get, "S3_PATH_STYLE", "true"))
	if err != nil {
		return c, fmt.Errorf("S3_PATH_STYLE must be boolean")
	}
	c.LINEEnabled, err = strconv.ParseBool(value(get, "LINE_ENABLED", "false"))
	if err != nil {
		return c, fmt.Errorf("LINE_ENABLED must be boolean")
	}
	for _, endpoint := range []string{c.S3Endpoint, c.S3PublicEndpoint} {
		if endpoint == "" {
			continue
		}
		u, e := url.Parse(endpoint)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return c, fmt.Errorf("S3 endpoint must be an HTTP origin")
		}
	}
	if (c.S3AccessKey == "") != (c.S3SecretKey == "") {
		return c, fmt.Errorf("configure both S3 credentials")
	}
	if c.LINEEnabled && (c.LINEToken == "" || !regexp.MustCompile(`^U[0-9a-f]{32}$`).MatchString(c.LINEAdminUserID)) {
		return c, fmt.Errorf("LINE_ENABLED requires a token and one Admin user ID (not a group ID)")
	}
	return c, nil
}

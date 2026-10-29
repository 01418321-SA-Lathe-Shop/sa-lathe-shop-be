package demoworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

type client struct {
	cfg    Config
	http   *http.Client
	tokens map[string]string
}

func newClient(c Config) *client {
	return &client{cfg: c, http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, tokens: map[string]string{}}
}
func (c *client) call(ctx context.Context, actor, method, path string, body Object) (Object, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, ErrGuard
	}
	var data []byte
	var e error
	if body != nil {
		data, e = json.Marshal(body)
		if e != nil {
			return nil, e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, c.cfg.APIOrigin+path, bytes.NewReader(data))
	if e != nil {
		return nil, ErrGuard
	}
	if token := c.tokens[actor]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := c.http.Do(req)
	if e != nil {
		return nil, fmt.Errorf("demo API transport failed (details withheld)")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("demo API returned HTTP %d", res.StatusCode)
	}
	if res.StatusCode == 204 {
		return Object{}, nil
	}
	data, e = io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return nil, ErrGuard
	}
	var v Object
	if e = json.Unmarshal(data, &v); e != nil {
		return nil, ErrGuard
	}
	return v, nil
}
func (c *client) login(ctx context.Context, accounts []auth.Account) error {
	for _, a := range accounts {
		v, e := c.call(ctx, "", "POST", "/auth/local-test/login", Object{"username": a.Username})
		if e != nil {
			return e
		}
		token, _ := v["access_token"].(string)
		c.tokens[a.Username] = token
		got := obj(v["account"])
		if token == "" || str(got, "id") != a.ID || str(got, "role") != a.Role {
			return ErrGuard
		}
		me, e := c.call(ctx, a.Username, "GET", "/auth/me", nil)
		if e != nil {
			return e
		}
		if str(me, "id") != a.ID || str(me, "username") != a.Username {
			return ErrGuard
		}
	}
	return nil
}
func (c *client) logout() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for actor := range c.tokens {
		_, _ = c.call(ctx, actor, "POST", "/auth/logout", nil)
		delete(c.tokens, actor)
	}
}
func (c *client) upload(ctx context.Context, actor, path, completePrefix, name, text string) (Object, error) {
	data := []byte(text)
	intent, e := c.call(ctx, actor, "POST", path, Object{"file_name": name, "content_type": "text/plain", "byte_size": len(data)})
	if e != nil {
		return nil, e
	}
	u, e := url.Parse(str(intent, "url"))
	if e != nil || u.Scheme+"://"+u.Host != c.cfg.StorageOrigin || u.User != nil {
		return nil, ErrGuard
	}
	req, e := http.NewRequestWithContext(ctx, "PUT", u.String(), bytes.NewReader(data))
	if e != nil {
		return nil, ErrGuard
	}
	for k, v := range obj(intent["headers"]) {
		s, ok := v.(string)
		if !ok {
			return nil, ErrGuard
		}
		req.Header.Set(k, s)
	}
	// Never send an API Bearer token to the storage origin.
	if req.Header.Get("Authorization") != "" {
		return nil, ErrGuard
	}
	req.Header.Set("Content-Type", "text/plain")
	res, e := c.http.Do(req)
	if e != nil {
		return nil, fmt.Errorf("private demo upload failed (details withheld)")
	}
	res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("private demo upload returned HTTP %d", res.StatusCode)
	}
	v, e := c.call(ctx, actor, "POST", completePrefix+str(intent, "attachment_id")+"/complete", nil)
	if e == nil && str(v, "state") != "ready" {
		return nil, ErrGuard
	}
	return v, e
}
func obj(v any) Object                { m, _ := v.(map[string]any); return m }
func str(v Object, key string) string { s, _ := v[key].(string); return s }
func number(v Object, key string) int64 {
	switch n := v[key].(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}
func items(v Object) []Object {
	var result []Object
	values, _ := v["items"].([]any)
	for _, item := range values {
		result = append(result, obj(item))
	}
	return result
}

// Package demoworkflow creates fictional local testing data through real APIs.
// SQL, business state updates and credentials never live in the fixture runner.
package demoworkflow

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
)

var ErrGuard = errors.New("workflow seed target or ownership guard refused")
var ErrUncertain = errors.New("workflow step outcome uncertain; reconcile recorded operation before retry, never reset")

type Object = map[string]any
type Config struct {
	Seed                     demoseed.Config
	APIOrigin, StorageOrigin string
}

func LoadConfig(get func(string) string) (Config, error) {
	c := Config{APIOrigin: get("SEED_WORKFLOW_API_ORIGIN"), StorageOrigin: get("SEED_WORKFLOW_S3_ORIGIN")}
	var e error
	if get("SEED_WORKFLOWS_ENABLE") != "true" {
		return c, ErrGuard
	}
	c.Seed, e = demoseed.LoadConfig(get)
	if e != nil {
		return c, ErrGuard
	}
	if !localOrigin(c.APIOrigin) || !localOrigin(c.StorageOrigin) {
		return c, ErrGuard
	}
	return c, nil
}
func localOrigin(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && u.Host != ""
}

type Operation struct {
	Key, Actor, Method, Path string
	Body                     Object
	SafeRetry                bool
}
type Step struct {
	Operation
	Result   Object
	Existing bool
}
type Store interface {
	Prepare(context.Context, Operation) (Step, error)
	Complete(context.Context, string, Object) error
	Finish(context.Context) error
	Results(context.Context) (map[string]Object, error)
}
type Repository interface {
	Within(context.Context, Config, bool, func(Store, []auth.Account, bool, time.Time) error) error
}

package demoworkflow

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/google/uuid"
)

//go:embed assets/*.pdf
var seedPDFs embed.FS

type Report struct {
	Reused    bool              `json:"reused"`
	Steps     int               `json:"steps"`
	Resources map[string]string `json:"resources"`
}
type Runner struct {
	store    Store
	api      *client
	started  time.Time
	progress func(string)
}

// OperationError contains a fixed fixture key and a sanitized client error,
// never SQL diagnostics, credentials, signed URLs or HTTP response bodies.
type OperationError struct {
	Key   string
	Cause error
}

func (e *OperationError) Error() string { return fmt.Sprintf("step %s: %v", e.Key, e.Cause) }
func (e *OperationError) Unwrap() error { return e.Cause }

func Apply(ctx context.Context, repo Repository, cfg Config, verifyOnly bool, progress func(string)) (Report, error) {
	var report Report
	if !localOrigin(cfg.APIOrigin) || !localOrigin(cfg.StorageOrigin) {
		return report, ErrGuard
	}
	e := repo.Within(ctx, cfg, !verifyOnly, func(store Store, accounts []auth.Account, complete bool, started time.Time) error {
		r := &Runner{store: store, api: newClient(cfg), started: started, progress: progress}
		// A complete apply is read-only: never reset tester progress or issue logins.
		if complete && !verifyOnly {
			report.Reused = true
			return r.report(ctx, &report)
		}
		defer r.api.logout()
		if e := r.api.login(ctx, accounts); e != nil {
			return e
		}
		if verifyOnly && !complete {
			return ErrGuard
		}
		if !verifyOnly {
			if e := r.build(ctx); e != nil {
				return e
			}
		}
		if e := r.verify(ctx, !verifyOnly); e != nil {
			return e
		}
		if !verifyOnly {
			if e := store.Finish(ctx); e != nil {
				return e
			}
		}
		return r.report(ctx, &report)
	})
	return report, e
}
func (r *Runner) report(ctx context.Context, out *Report) error {
	results, e := r.store.Results(ctx)
	if e != nil {
		return e
	}
	out.Steps = len(results)
	out.Resources = map[string]string{}
	for key, v := range results {
		if id := str(v, "id"); id != "" {
			out.Resources[key] = id
		}
	}
	return nil
}
func (r *Runner) step(ctx context.Context, key, actor, method, path string, body Object, safe bool, upload func() (Object, error)) (Object, error) {
	step, e := r.store.Prepare(ctx, Operation{Key: key, Actor: actor, Method: method, Path: path, Body: body, SafeRetry: safe})
	if e != nil {
		return nil, e
	}
	if step.Result != nil {
		return step.Result, nil
	}
	if step.Existing && !step.SafeRetry {
		return nil, fmt.Errorf("%s: %w", key, ErrUncertain)
	}
	var v Object
	if upload != nil {
		v, e = upload()
	} else {
		v, e = r.api.call(ctx, step.Actor, step.Method, step.Path, step.Body)
	}
	if e != nil {
		return nil, &OperationError{Key: key, Cause: e}
	}
	if e = r.store.Complete(ctx, key, v); e != nil {
		return nil, fmt.Errorf("step %s checkpoint failed: %w", key, ErrUncertain)
	}
	if r.progress != nil {
		r.progress(key)
	}
	return v, nil
}
func (r *Runner) post(ctx context.Context, key, actor, path string, body Object) (Object, error) {
	return r.step(ctx, key, actor, "POST", path, body, false, nil)
}
func (r *Runner) command(ctx context.Context, key, actor, method, path string, state Object, extra Object) (Object, error) {
	body := Object{"idempotency_key": uuid.NewString(), "expected_version": number(state, "version")}
	for k, v := range extra {
		body[k] = v
	}
	return r.step(ctx, key, actor, method, path, body, true, nil)
}
func (r *Runner) document(ctx context.Context, key, actor, path, prefix, _ string) (Object, error) {
	name := key + ".pdf"
	content, err := seedPDFs.ReadFile("assets/" + name)
	if err != nil || len(content) < 8 || string(content[:5]) != "%PDF-" {
		return nil, ErrGuard
	}
	body := Object{"file_name": name, "content_type": "application/pdf", "byte_size": len(content), "sha256": fmt.Sprintf("%x", sha256.Sum256(content))}
	return r.step(ctx, key, actor, "UPLOAD", path, body, false, func() (Object, error) {
		return r.api.upload(ctx, actor, path, prefix, name, content)
	})
}
func (r *Runner) get(ctx context.Context, actor, path string) (Object, error) {
	return r.api.call(ctx, actor, http.MethodGet, path, nil)
}

package demoworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memoryStore struct {
	steps        map[string]Step
	failComplete bool
}

func (s *memoryStore) Prepare(_ context.Context, o Operation) (Step, error) {
	if p, ok := s.steps[o.Key]; ok {
		p.Existing = true
		return p, nil
	}
	p := Step{Operation: o}
	s.steps[o.Key] = p
	return p, nil
}
func (s *memoryStore) Complete(_ context.Context, k string, v Object) error {
	if s.failComplete {
		return errors.New("injected checkpoint failure")
	}
	p := s.steps[k]
	p.Result = v
	s.steps[k] = p
	return nil
}
func (s *memoryStore) Finish(context.Context) error { return nil }
func (s *memoryStore) Results(context.Context) (map[string]Object, error) {
	v := map[string]Object{}
	for k, p := range s.steps {
		if p.Result != nil {
			v[k] = p.Result
		}
	}
	return v, nil
}
func TestStepRerunAndUncertainOutcomes(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{steps: map[string]Step{}}
	r := &Runner{store: store}
	calls := 0
	create := func() (Object, error) { calls++; return Object{"id": "created-resource"}, nil }
	for i := 0; i < 2; i++ {
		v, e := r.step(ctx, "request", "somchai", "POST", "/requests", Object{}, false, create)
		if e != nil || str(v, "id") != "created-resource" {
			t.Fatal("completed step could not be reused")
		}
	}
	if calls != 1 {
		t.Fatal("completed rerun wrote again")
	}
	store.failComplete = true
	_, e := r.step(ctx, "ambiguous", "somchai", "POST", "/requests", Object{}, false, create)
	if !errors.Is(e, ErrUncertain) {
		t.Fatal("checkpoint failure not uncertain")
	}
	store.failComplete = false
	_, e = r.step(ctx, "ambiguous", "somchai", "POST", "/requests", Object{}, false, create)
	if !errors.Is(e, ErrUncertain) || calls != 2 {
		t.Fatal("unsafe operation replayed after unknown outcome")
	}
}
func TestIdempotentRetryUsesPersistedPayload(t *testing.T) {
	var bodies []Object
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v Object
		_ = json.NewDecoder(r.Body).Decode(&v)
		bodies = append(bodies, v)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"same-operation"}`))
	}))
	defer server.Close()
	store := &memoryStore{steps: map[string]Step{}, failComplete: true}
	r := &Runner{store: store, api: newClient(Config{APIOrigin: server.URL})}
	_, _ = r.step(context.Background(), "stock", "pim.admin", "POST", "/stock", Object{"idempotency_key": "original", "expected_version": 1}, true, nil)
	store.failComplete = false
	_, e := r.step(context.Background(), "stock", "pim.admin", "POST", "/stock", Object{"idempotency_key": "new-must-not-use", "expected_version": 99}, true, nil)
	if e != nil || len(bodies) != 2 || str(bodies[1], "idempotency_key") != "original" || number(bodies[1], "expected_version") != 1 {
		t.Fatal("safe retry changed original payload")
	}
}
func TestConfigGuards(t *testing.T) {
	base := map[string]string{"SEED_WORKFLOWS_ENABLE": "true", "SEED_ENABLED": "true", "SEED_ENV": "test", "SEED_CONFIRM_DATABASE": "sa_lathe_shop_seed_test", "SEED_DATABASE_URL": "postgres://fixture@localhost/sa_lathe_shop_seed_test", "SEED_WORKFLOW_API_ORIGIN": "http://localhost:8080", "SEED_WORKFLOW_S3_ORIGIN": "http://localhost:19000"}
	if _, e := LoadConfig(func(k string) string { return base[k] }); e != nil {
		t.Fatal(e)
	}
	for key, value := range map[string]string{"SEED_WORKFLOWS_ENABLE": "false", "SEED_ENV": "production", "SEED_CONFIRM_DATABASE": "shop", "SEED_WORKFLOW_API_ORIGIN": "https://example.com", "SEED_WORKFLOW_S3_ORIGIN": "http://localhost:19000/private"} {
		_, e := LoadConfig(func(k string) string {
			if k == key {
				return value
			}
			return base[k]
		})
		if e == nil {
			t.Fatalf("guard accepted %s", key)
		}
	}
}
func TestUploadRefusesForeignOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://example.com/private","attachment_id":"x","headers":{}}`))
	}))
	defer server.Close()
	c := newClient(Config{APIOrigin: server.URL, StorageOrigin: "http://localhost:19000"})
	c.tokens["somchai"] = "test-only-api-token"
	_, e := c.upload(context.Background(), "somchai", "/uploads", "/complete/", "TEST.txt", "DEMO / TEST ONLY")
	if !errors.Is(e, ErrGuard) {
		t.Fatal("foreign signed upload destination accepted")
	}
}

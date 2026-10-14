package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStaffAuthIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set TEST_DATABASE_URL for an isolated migrated test DB")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/sa_lathe_shop_phase3_test" {
		t.Fatal("integration tests require a dedicated sa_lathe_shop_phase3_test database")
	}
	adminPassword := os.Getenv("TEST_ADMIN_PASSWORD")
	workerPassword := os.Getenv("TEST_WORKER_PASSWORD")
	if len(adminPassword) < 12 || len(workerPassword) < 12 {
		t.Fatal("supply private test passwords through process environment")
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn, 10)
	if err != nil {
		t.Fatal("cannot open test DB")
	}
	defer db.Close()
	service, err := auth.NewService(db, auth.NewArgonHasher(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var adminCount int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM account WHERE role='admin'`).Scan(&adminCount); err != nil {
		t.Fatal(err)
	}
	if adminCount != 0 {
		t.Fatal("test DB must be fresh; existing accounts will not be overwritten")
	}
	t.Run("concurrent_first_admin", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, e := service.BootstrapAdmin(ctx, "phase3_admin", "Admin", adminPassword)
				results <- e
			}()
		}
		wg.Wait()
		close(results)
		success, exists := 0, 0
		for e := range results {
			if e == nil {
				success++
			} else if errors.Is(e, auth.ErrAdminExists) {
				exists++
			} else {
				t.Fatal(e)
			}
		}
		if success != 1 || exists != 1 {
			t.Fatal("bootstrap was not atomic")
		}
	})
	admin, err := db.FindAccount(ctx, "phase3_admin")
	if err != nil {
		t.Fatal(err)
	}
	handler := auth.NewHandler(service)
	ready := health.NewService(db, time.Second)
	router := httpserver.RouterWithRoutes(slog.New(slog.NewTextHandler(io.Discard, nil)), 10*time.Second, []string{"http://localhost:3000"}, health.Live, ready.Handler, handler.Routes)
	server := httptest.NewServer(router)
	defer server.Close()
	call := func(method, path string, body any, token string, want int, out any) {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r, err := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s %s: got %d want %d", method, path, resp.StatusCode, want)
		}
		if bytes.Contains(raw, []byte("password_hash")) {
			t.Fatal("hash exposed by HTTP")
		}
		if out != nil {
			if err = json.Unmarshal(raw, out); err != nil {
				t.Fatal(err)
			}
		}
	}
	login := func(username, password string) auth.LoginResult {
		t.Helper()
		var result auth.LoginResult
		call("POST", "/auth/login", map[string]string{"username": username, "password": password}, "", 200, &result)
		return result
	}
	adminLogin := login("PHASE3_ADMIN", adminPassword)
	call("GET", "/auth/me", nil, "", 401, nil)
	call("POST", "/auth/login", map[string]string{"username": "phase3_admin", "password": "invalid-test-password"}, "", 401, nil)
	var worker auth.Worker
	call("POST", "/admin/workers", auth.WorkerInput{Username: "phase3_worker", Name: "Worker", EmployeeNo: "P3-001", Position: "Lathe", Password: workerPassword}, adminLogin.AccessToken, 201, &worker)
	workerLogin := login("phase3_worker", workerPassword)
	call("GET", "/auth/me", nil, workerLogin.AccessToken, 200, nil)
	call("GET", "/admin/workers", nil, workerLogin.AccessToken, 403, nil)
	call("PATCH", "/admin/workers/"+worker.ID, map[string]any{"is_active": false}, workerLogin.AccessToken, 403, nil)
	call("GET", "/admin/workers", nil, adminLogin.AccessToken, 200, nil)
	call("PATCH", "/admin/workers/"+worker.ID, map[string]any{"role": "admin"}, adminLogin.AccessToken, 400, nil)
	call("PATCH", "/admin/workers/"+admin.ID, map[string]any{"is_active": false}, adminLogin.AccessToken, 404, nil)
	t.Run("transaction_rollback", func(t *testing.T) {
		var before, after int
		_ = db.pool.QueryRow(ctx, `SELECT count(*) FROM person`).Scan(&before)
		_, e := service.CreateWorker(ctx, admin, auth.WorkerInput{Username: "duplicate_employee", Name: "Not persisted", EmployeeNo: "P3-001", Password: workerPassword})
		if !errors.Is(e, auth.ErrConflict) {
			t.Fatal("duplicate employee accepted")
		}
		_ = db.pool.QueryRow(ctx, `SELECT count(*) FROM person`).Scan(&after)
		if before != after {
			t.Fatal("orphan person after rollback")
		}
	})
	snapshot, err := db.GetWorker(ctx, worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	call("PATCH", "/admin/workers/"+worker.ID, map[string]any{"name": "Updated Worker", "position": "Machinist", "is_active": false}, adminLogin.AccessToken, 200, nil)
	call("GET", "/auth/me", nil, workerLogin.AccessToken, 401, nil)
	call("POST", "/auth/login", map[string]string{"username": "phase3_worker", "password": workerPassword}, "", 401, nil)
	t.Run("stale_login_after_deactivation", func(t *testing.T) {
		_, hash, _ := authToken()
		e := db.CreateSession(ctx, auth.Session{ID: uuid.NewString(), TokenHash: hash, Account: snapshot.Account, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}, snapshot.PasswordHash)
		if !errors.Is(e, auth.ErrUnauthenticated) {
			t.Fatal("session created after deactivation")
		}
	})
	call("PATCH", "/admin/workers/"+worker.ID, map[string]any{"is_active": true}, adminLogin.AccessToken, 200, nil)
	call("GET", "/auth/me", nil, workerLogin.AccessToken, 401, nil)
	resetLogin := login("phase3_worker", workerPassword)
	resetPassword := workerPassword + "-reset"
	call("PATCH", "/admin/workers/"+worker.ID, map[string]any{"password": resetPassword}, adminLogin.AccessToken, 200, nil)
	call("GET", "/auth/me", nil, resetLogin.AccessToken, 401, nil)
	t.Run("stale_login_after_password_reset", func(t *testing.T) {
		_, hash, _ := authToken()
		e := db.CreateSession(ctx, auth.Session{ID: uuid.NewString(), TokenHash: hash, Account: snapshot.Account, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}, snapshot.PasswordHash)
		if !errors.Is(e, auth.ErrUnauthenticated) {
			t.Fatal("session created with stale hash")
		}
	})
	call("POST", "/auth/login", map[string]string{"username": "phase3_worker", "password": workerPassword}, "", 401, nil)
	expireLogin := login("phase3_worker", resetPassword)
	tokenHash := sha256.Sum256([]byte(expireLogin.AccessToken))
	if _, err = db.pool.Exec(ctx, `UPDATE auth_session SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE token_hash=$1`, tokenHash[:]); err != nil {
		t.Fatal(err)
	}
	call("GET", "/auth/me", nil, expireLogin.AccessToken, 401, nil)
	logoutLogin := login("phase3_worker", resetPassword)
	call("POST", "/auth/logout", nil, logoutLogin.AccessToken, 204, nil)
	call("GET", "/auth/me", nil, logoutLogin.AccessToken, 401, nil)
	t.Run("stale_login_after_username_change", func(t *testing.T) {
		before, e := db.GetWorker(ctx, worker.ID)
		if e != nil {
			t.Fatal(e)
		}
		username := "phase3_worker_renamed"
		if _, e = service.UpdateWorker(ctx, admin, worker.ID, auth.WorkerUpdate{Username: &username}); e != nil {
			t.Fatal(e)
		}
		_, hash, _ := authToken()
		e = db.CreateSession(ctx, auth.Session{ID: uuid.NewString(), TokenHash: hash, Account: before.Account, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}, before.PasswordHash)
		if !errors.Is(e, auth.ErrUnauthenticated) {
			t.Fatal("session created with stale username")
		}
	})
	var storedHash []byte
	rawHash := sha256.Sum256([]byte(adminLogin.AccessToken))
	if err = db.pool.QueryRow(ctx, `SELECT token_hash FROM auth_session WHERE token_hash=$1`, rawHash[:]).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if len(storedHash) != 32 || strings.Contains(admin.PasswordHash, adminPassword) {
		t.Fatal("raw credentials persisted")
	}
}

// Random test credential without exposing auth package private helpers.
func authToken() (string, []byte, error) {
	token := uuid.NewString() + uuid.NewString()
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

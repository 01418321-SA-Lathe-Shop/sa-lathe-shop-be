package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// In-memory repository double; service tests import no database driver.
type fakeRepo struct {
	accounts         map[string]Account
	workers          map[string]Worker
	sessions         map[string]Session
	createSessionErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{accounts: map[string]Account{}, workers: map[string]Worker{}, sessions: map[string]Session{}}
}
func (f *fakeRepo) FindAccount(_ context.Context, u string) (Account, error) {
	a, ok := f.accounts[u]
	if !ok {
		return Account{}, ErrNotFound
	}
	return a, nil
}
func (f *fakeRepo) CreateSession(_ context.Context, s Session, expected string) error {
	if f.createSessionErr != nil {
		return f.createSessionErr
	}
	a, ok := f.accounts[s.Account.Username]
	if !ok || !a.Active || a.PasswordHash != expected {
		return ErrUnauthenticated
	}
	f.sessions[string(s.TokenHash)] = s
	return nil
}
func (f *fakeRepo) FindSession(_ context.Context, hash []byte) (Session, error) {
	s, ok := f.sessions[string(hash)]
	if !ok {
		return Session{}, ErrNotFound
	}
	for _, a := range f.accounts {
		if a.ID == s.Account.ID {
			s.Account = a
		}
	}
	return s, nil
}
func (f *fakeRepo) RevokeSession(_ context.Context, hash []byte) error {
	s, ok := f.sessions[string(hash)]
	if ok {
		now := time.Now()
		s.RevokedAt = &now
		f.sessions[string(hash)] = s
	}
	return nil
}
func (f *fakeRepo) BootstrapAdmin(_ context.Context, input NewStaff) (Account, error) {
	for _, a := range f.accounts {
		if a.Role == RoleAdmin {
			return Account{}, ErrAdminExists
		}
	}
	f.accounts[input.Account.Username] = input.Account
	return input.Account, nil
}
func (f *fakeRepo) CreateWorker(_ context.Context, input NewStaff) (Worker, error) {
	if _, ok := f.accounts[input.Account.Username]; ok {
		return Worker{}, ErrConflict
	}
	for _, w := range f.workers {
		if w.EmployeeNo == input.EmployeeNo {
			return Worker{}, ErrConflict
		}
	}
	w := Worker{Account: input.Account, EmployeeNo: input.EmployeeNo, Position: input.Position, Email: &input.Email, Phone: &input.Phone}
	f.accounts[w.Username] = w.Account
	f.workers[w.ID] = w
	return w, nil
}
func (f *fakeRepo) ListWorkers(_ context.Context, limit, offset int) ([]Worker, error) {
	ws := make([]Worker, 0)
	for _, w := range f.workers {
		ws = append(ws, w)
	}
	if offset >= len(ws) {
		return []Worker{}, nil
	}
	ws = ws[offset:]
	if len(ws) > limit {
		ws = ws[:limit]
	}
	return ws, nil
}
func (f *fakeRepo) GetWorker(_ context.Context, id string) (Worker, error) {
	w, ok := f.workers[id]
	if !ok {
		return Worker{}, ErrNotFound
	}
	return w, nil
}
func (f *fakeRepo) UpdateWorker(ctx context.Context, id string, c WorkerChanges) (Worker, error) {
	w, err := f.GetWorker(ctx, id)
	if err != nil {
		return Worker{}, err
	}
	old := w.Username
	if c.Email != nil {
		w.Email = c.Email
	}
	if c.Phone != nil {
		w.Phone = c.Phone
	}
	if c.Username != nil {
		w.Username = *c.Username
	}
	if c.Name != nil {
		w.Name = *c.Name
	}
	if c.EmployeeNo != nil {
		w.EmployeeNo = *c.EmployeeNo
	}
	if c.Position != nil {
		w.Position = *c.Position
	}
	if c.Active != nil {
		w.Active = *c.Active
	}
	if c.PasswordHash != nil {
		w.PasswordHash = *c.PasswordHash
	}
	delete(f.accounts, old)
	f.accounts[w.Username] = w.Account
	f.workers[id] = w
	if !w.Active || c.Username != nil || c.PasswordHash != nil {
		for k, s := range f.sessions {
			if s.Account.ID == id {
				now := time.Now()
				s.RevokedAt = &now
				f.sessions[k] = s
			}
		}
	}
	return w, nil
}

type fastHasher struct{}

func (fastHasher) Hash(_ context.Context, p string) (string, error)    { return "test:" + p, nil }
func (fastHasher) Verify(_ context.Context, h, p string) (bool, error) { return h == "test:"+p, nil }
func setupService(t *testing.T) (*Service, *fakeRepo, Account) {
	t.Helper()
	repo := newFakeRepo()
	s, err := NewService(repo, fastHasher{}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.BootstrapAdmin(context.Background(), "admin.one", "Shop Admin", "Test-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	return s, repo, a
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	s, repo, admin := setupService(t)
	ctx := context.Background()
	for _, u := range []string{"admin.one", "missing.user"} {
		_, err := s.Login(ctx, u, "Wrong-password-1234")
		if !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("expected same generic error: %v", err)
		}
	}
	admin.Active = false
	repo.accounts[admin.Username] = admin
	if _, err := s.Login(ctx, admin.Username, "Test-password-1234"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("session created for rejected credentials")
	}
}

func TestSessionChecksAndLogout(t *testing.T) {
	for _, kind := range []string{"expired", "revoked", "inactive", "invalid_role", "logout"} {
		t.Run(kind, func(t *testing.T) {
			s, repo, admin := setupService(t)
			ctx := context.Background()
			login, err := s.Login(ctx, "ADMIN.ONE", "Test-password-1234")
			if err != nil {
				t.Fatal(err)
			}
			if login.Account.PasswordHash != "" {
				t.Fatal("hash leaked from login")
			}
			if _, err = s.Authenticate(ctx, login.AccessToken); err != nil {
				t.Fatal(err)
			}
			key := string(digest(login.AccessToken))
			sess := repo.sessions[key]
			switch kind {
			case "expired":
				sess.ExpiresAt = s.now()
			case "revoked":
				now := s.now()
				sess.RevokedAt = &now
			case "inactive":
				admin.Active = false
				repo.accounts[admin.Username] = admin
			case "invalid_role":
				admin.Role = "supplier"
				repo.accounts[admin.Username] = admin
			case "logout":
				if err = s.Logout(ctx, login.AccessToken); err != nil {
					t.Fatal(err)
				}
				sess = repo.sessions[key]
			}
			repo.sessions[key] = sess
			if _, err = s.Authenticate(ctx, login.AccessToken); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("accepted %s session: %v", kind, err)
			}
		})
	}
}
func TestLoginHonorsConcurrentAccountChange(t *testing.T) {
	s, repo, _ := setupService(t)
	repo.createSessionErr = ErrUnauthenticated
	if _, err := s.Login(context.Background(), "admin.one", "Test-password-1234"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("stale login created session")
	}
}

func TestWorkerPermissionsAndValidation(t *testing.T) {
	s, _, admin := setupService(t)
	ctx := context.Background()
	input := WorkerInput{Username: "Worker.One", Name: "Technician", EmployeeNo: "emp-001", Position: "Lathe", Password: "Worker-password-1234", Email: "worker@example.com", Phone: "+66812345678"}
	actor := Account{Role: RoleWorker, Active: true}
	if _, err := s.CreateWorker(ctx, actor, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("worker created staff")
	}
	if _, err := s.ListWorkers(ctx, actor, 10, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("worker listed staff")
	}
	input.Password = "short"
	if _, err := s.CreateWorker(ctx, admin, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("weak password accepted")
	}
	input.Password = "Worker-password-1234"
	w, err := s.CreateWorker(ctx, admin, input)
	if err != nil {
		t.Fatal(err)
	}
	if w.Role != RoleWorker || w.Username != "worker.one" || w.EmployeeNo != "EMP-001" {
		t.Fatal("normalization failed")
	}
	if _, err := s.CreateWorker(ctx, admin, input); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate accepted")
	}
	if _, err := s.UpdateWorker(ctx, actor, w.ID, WorkerUpdate{}); !errors.Is(err, ErrForbidden) {
		t.Fatal("worker edited staff")
	}
	if _, err := s.UpdateWorker(ctx, admin, w.ID, WorkerUpdate{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("empty patch accepted")
	}
	if _, err := s.GetWorker(ctx, admin, "not-uuid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("bad id accepted")
	}
}

func TestWorkerDeactivationAndPasswordReset(t *testing.T) {
	s, _, admin := setupService(t)
	ctx := context.Background()
	w, err := s.CreateWorker(ctx, admin, WorkerInput{Username: "worker.one", Name: "Worker", EmployeeNo: "EMP-1", Password: "Worker-password-1234", Email: "worker@example.com", Phone: "+66812345678"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Login(ctx, w.Username, "Worker-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	active := false
	if _, err = s.UpdateWorker(ctx, admin, w.ID, WorkerUpdate{Active: &active}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, first.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("deactivated token accepted")
	}
	active = true
	if _, err = s.UpdateWorker(ctx, admin, w.ID, WorkerUpdate{Active: &active}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, first.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("old token restored")
	}
	second, err := s.Login(ctx, w.Username, "Worker-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	password := "New-password-1234"
	if _, err = s.UpdateWorker(ctx, admin, w.ID, WorkerUpdate{Password: &password}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, second.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("password-reset token accepted")
	}
	if _, err = s.Login(ctx, w.Username, "Worker-password-1234"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("old password accepted")
	}
}

func TestBootstrapIsOneTime(t *testing.T) {
	s, _, _ := setupService(t)
	if _, err := s.BootstrapAdmin(context.Background(), "second.admin", "Second", "Test-password-1234"); !errors.Is(err, ErrAdminExists) {
		t.Fatal("second bootstrap accepted")
	}
}

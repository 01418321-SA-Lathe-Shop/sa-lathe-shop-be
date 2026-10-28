package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
)

// Called from the guarded, fresh PostgreSQL integration suite after its cleanup
// checks. These logins create real (revoked, retained) session audit records.
func verifyLocalTestSessions(t *testing.T, db *Database) {
	t.Helper()
	ctx := context.Background()
	s := auth.NewLocalTestService(db, true, time.Hour)
	regular, e := auth.NewService(db, auth.NewArgonHasher(), time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	for _, spec := range demoseed.Accounts() {
		login, e := s.Login(ctx, spec.Username)
		if e != nil {
			t.Fatal(e)
		}
		a, e := regular.Authenticate(ctx, login.AccessToken)
		if e != nil || a.Username != spec.Username || a.Role != spec.Role || login.Account.PasswordHash != "" {
			t.Fatal("local login did not produce a normal scoped session")
		}
		if e = regular.Logout(ctx, login.AccessToken); e != nil {
			t.Fatal(e)
		}
		if _, e = regular.Authenticate(ctx, login.AccessToken); !errors.Is(e, auth.ErrUnauthenticated) {
			t.Fatal("local session not revocable")
		}
	}
	for _, username := range []string{"outside.company", "legacy.staff", "unknown"} {
		if _, e = s.Login(ctx, username); !errors.Is(e, auth.ErrUnauthenticated) {
			t.Fatal("unowned account accepted")
		}
	}
	a, e := db.FindAccount(ctx, "anan.staff")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []string{`UPDATE account SET is_active=false WHERE id=$1`, `UPDATE account SET role='admin' WHERE id=$1`} {
		tx, e := db.pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		// Use a committed change so the separate login transaction observes it.
		if _, e = tx.Exec(ctx, change, a.ID); e != nil {
			tx.Rollback(ctx)
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		_, loginErr := s.Login(ctx, a.Username)
		_, restoreErr := db.pool.Exec(ctx, `UPDATE account SET is_active=true,role='worker' WHERE id=$1`, a.ID)
		if restoreErr != nil {
			t.Fatal(restoreErr)
		}
		if !errors.Is(loginErr, auth.ErrUnauthenticated) {
			t.Fatal("inactive or role-drifted account accepted")
		}
	}
	_, e = db.pool.Exec(ctx, `UPDATE demo_seed_step SET manifest_digest=repeat('0',64)`)
	if e != nil {
		t.Fatal(e)
	}
	_, loginErr := s.Login(ctx, "pim.admin")
	_, e = db.pool.Exec(ctx, `UPDATE demo_seed_step SET manifest_digest=$1`, demoseed.ManifestDigest())
	if e != nil {
		t.Fatal(e)
	}
	if !errors.Is(loginErr, auth.ErrUnauthenticated) {
		t.Fatal("incomplete/invalid seed accepted")
	}
}

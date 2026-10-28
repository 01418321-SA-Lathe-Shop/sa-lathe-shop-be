package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
)

func TestDemoSeedIntegration(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()
	password := os.Getenv("SEED_TEST_PASSWORD")
	cfg := demoseed.Config{DatabaseURL: os.Getenv("SEED_TEST_DATABASE_URL"), Database: "sa_lathe_shop_seed_test", Environment: "test"}
	hasher := auth.NewArgonHasher()
	seed := demoseed.New(NewDemoSeedRepository(db), hasher, cfg)
	credentials := func(string) string { return password }
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	count := func(table string) int {
		t.Helper()
		var n int
		check(db.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n))
		return n
	}
	if count("account") != 0 {
		t.Fatal("test DB must be fresh")
	}
	t.Run("late_validation_failure_is_atomic", func(t *testing.T) {
		_, e := seed.Apply(ctx, func(key string) string {
			if key == "SEED_SOMCHAI_PASSWORD" {
				return ""
			}
			return password
		})
		if !errors.Is(e, auth.ErrInvalidInput) {
			t.Fatal("invalid password not rejected")
		}
		if count("person") != 0 || count("demo_seed_dataset") != 0 {
			t.Fatal("partial identity or provenance leaked")
		}
	})
	t.Run("capture_failure_is_atomic", func(t *testing.T) {
		_, e := db.pool.Exec(ctx, `CREATE FUNCTION seed_test_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected provenance failure'; END $$; CREATE TRIGGER seed_test_fail BEFORE INSERT ON demo_seed_step FOR EACH ROW EXECUTE FUNCTION seed_test_fail()`)
		check(e)
		_, e = seed.Apply(ctx, credentials)
		if e == nil {
			t.Fatal("injected capture failure ignored")
		}
		_, e = db.pool.Exec(ctx, `DROP TRIGGER seed_test_fail ON demo_seed_step;DROP FUNCTION seed_test_fail()`)
		check(e)
		if count("account") != 0 || count("demo_seed_record") != 0 {
			t.Fatal("capture failure leaked identities")
		}
	})
	t.Run("unowned_matching_username_never_adopted", func(t *testing.T) {
		customers := auth.NewCustomerService(db, hasher)
		a, e := customers.Register(ctx, auth.Registration{Username: "pim.admin", Name: "Unrelated identity", Password: password, CustomerType: "individual"})
		check(e)
		before, e := db.FindAccount(ctx, a.Username)
		check(e)
		_, e = seed.Apply(ctx, credentials)
		if !errors.Is(e, demoseed.ErrConflict) {
			t.Fatal("unowned account not rejected")
		}
		after, e := db.FindAccount(ctx, a.Username)
		check(e)
		if after.ID != before.ID || after.PasswordHash != before.PasswordHash || count("account") != 1 {
			t.Fatal("unrelated identity changed")
		}
		profiles, e := customers.Profiles(ctx, a)
		check(e)
		// Only exact test-created rows in this guarded disposable DB are removed.
		_, e = db.pool.Exec(ctx, `DELETE FROM customer_member WHERE account_id=$1`, a.ID)
		check(e)
		_, e = db.pool.Exec(ctx, `DELETE FROM customer WHERE id=$1`, profiles[0].ID)
		check(e)
		_, e = db.pool.Exec(ctx, `DELETE FROM account WHERE id=$1`, a.ID)
		check(e)
		_, e = db.pool.Exec(ctx, `DELETE FROM person WHERE id=$1`, a.PersonID)
		check(e)
	})
	t.Run("apply_and_exact_reruns", func(t *testing.T) {
		status, e := seed.Apply(ctx, credentials)
		check(e)
		if status.State != "ready" || status.Accounts != 6 || status.Records != 12 || status.Reused {
			t.Fatal("unexpected seed result")
		}
		if count("auth_session") != 0 || count("customer") != 2 || count("worker") != 2 || count("customer_member") != 3 {
			t.Fatal("account-only shape violated")
		}
		before, e := db.FindAccount(ctx, "pim.admin")
		check(e)
		for _, spec := range demoseed.Accounts() {
			a, e := db.FindAccount(ctx, spec.Username)
			check(e)
			ok, e := hasher.Verify(ctx, a.PasswordHash, password)
			check(e)
			if !ok || a.Role != spec.Role || a.Name != spec.Name {
				t.Fatal("seed identity or password invalid")
			}
		}
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, e := seed.Apply(ctx, func(string) string { t.Error("rerun requested a password"); return "" })
				if e == nil && !v.Reused {
					e = demoseed.ErrConflict
				}
				results <- e
			}()
		}
		wg.Wait()
		close(results)
		for e := range results {
			check(e)
		}
		after, e := db.FindAccount(ctx, "pim.admin")
		check(e)
		if after.PasswordHash != before.PasswordHash || after.ID != before.ID || count("account") != 6 {
			t.Fatal("rerun reset or duplicated identity")
		}
	})
	t.Run("identity_drift_and_database_guard", func(t *testing.T) {
		a, e := db.FindAccount(ctx, "anan.staff")
		check(e)
		_, e = db.pool.Exec(ctx, `UPDATE person SET name='Changed by tester' WHERE id=$1`, a.PersonID)
		check(e)
		_, e = seed.Apply(ctx, credentials)
		if !errors.Is(e, demoseed.ErrConflict) {
			t.Fatal("drift was silently repaired")
		}
		_, e = db.pool.Exec(ctx, `UPDATE person SET name=$2 WHERE id=$1`, a.PersonID, a.Name)
		check(e)
		bad := cfg
		bad.Database = "sa_lathe_shop_demo"
		_, e = demoseed.New(NewDemoSeedRepository(db), hasher, bad).Status(ctx)
		if !errors.Is(e, demoseed.ErrGuard) {
			t.Fatal("connected database mismatch accepted")
		}
	})
	t.Run("migration_and_cleanup_safety", func(t *testing.T) {
		down, e := os.ReadFile("../../../migrations/000013_demo_seed.down.sql")
		check(e)
		tx, e := db.pool.Begin(ctx)
		check(e)
		_, e = tx.Exec(ctx, string(down))
		if e == nil {
			t.Fatal("populated provenance downgrade was allowed")
		}
		check(tx.Rollback(ctx))
		_, e = db.pool.Exec(ctx, `CREATE FUNCTION seed_test_delete_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected cleanup failure'; END $$; CREATE TRIGGER seed_test_delete_fail BEFORE DELETE ON account FOR EACH ROW EXECUTE FUNCTION seed_test_delete_fail()`)
		check(e)
		_, e = seed.Cleanup(ctx, true)
		if !errors.Is(e, demoseed.ErrDependency) || count("account") != 6 || count("customer_member") != 3 || count("demo_seed_record") != 12 {
			t.Fatal("cleanup failure did not preserve all rows")
		}
		_, e = db.pool.Exec(ctx, `DROP TRIGGER seed_test_delete_fail ON account;DROP FUNCTION seed_test_delete_fail()`)
		check(e)
		status, e := seed.Cleanup(ctx, false)
		check(e)
		if status.State != "cleanup_eligible" || count("account") != 6 {
			t.Fatal("dry run persisted deletion")
		}
		status, e = seed.Cleanup(ctx, true)
		check(e)
		if status.State != "uninitialized" || count("person") != 0 || count("demo_seed_dataset") != 0 {
			t.Fatal("unused account cleanup incomplete")
		}
		_, e = seed.Apply(ctx, credentials)
		check(e)
	})
	t.Run("real_http_roles_and_customer_isolation", func(t *testing.T) { verifyDemoAccountHTTP(t, db, cfg, password) })
	t.Run("used_dataset_refuses_cleanup_and_can_retire", func(t *testing.T) {
		before, e := db.FindAccount(ctx, "pim.admin")
		check(e)
		staff, e := auth.NewService(db, hasher, time.Hour)
		check(e)
		login, e := staff.Login(ctx, "pim.admin", password)
		check(e)
		_, e = seed.Cleanup(ctx, true)
		if !errors.Is(e, demoseed.ErrDependency) || count("account") != 7 || count("job_request") != 3 {
			t.Fatal("used data not preserved")
		}
		status, e := seed.Retire(ctx)
		check(e)
		if status.State != "retired" {
			t.Fatal("retirement not recorded")
		}
		_, e = staff.Authenticate(ctx, login.AccessToken)
		if !errors.Is(e, auth.ErrUnauthenticated) {
			t.Fatal("retired session remains active")
		}
		after, e := db.FindAccount(ctx, "pim.admin")
		check(e)
		if after.Active || after.PasswordHash != before.PasswordHash || count("job_request") != 3 {
			t.Fatal("retirement changed credentials or business rows")
		}
		other, e := db.FindAccount(ctx, "outside.company")
		check(e)
		if !other.Active {
			t.Fatal("unowned account retired")
		}
		_, e = seed.Apply(ctx, credentials)
		if !errors.Is(e, demoseed.ErrConflict) {
			t.Fatal("retired identities silently reactivated")
		}
		status, e = seed.Status(ctx)
		check(e)
		if status.State != "retired" {
			t.Fatal("retired provenance invalid")
		}
	})
}

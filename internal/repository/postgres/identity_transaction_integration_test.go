package postgres

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

func seedTestDB(t *testing.T) *Database {
	t.Helper()
	dsn := os.Getenv("SEED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("run scripts/verify-demo-seed.ps1 for disposable PostgreSQL")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/sa_lathe_shop_seed_test" || os.Getenv("SEED_TEST_CONFIRM") != "disposable" {
		t.Fatal("dedicated disposable seed test database required")
	}
	db, e := Open(context.Background(), dsn, 5)
	if e != nil {
		t.Fatal("cannot open seed test DB")
	}
	t.Cleanup(db.Close)
	return db
}

func TestIdentityTransactionIntegration(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()
	tx, e := db.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	bound := &Database{pool: db.pool, identityTx: tx}
	hasher := auth.NewArgonHasher()
	staff, e := auth.NewService(bound, hasher, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	password := os.Getenv("SEED_TEST_PASSWORD")
	admin, e := staff.BootstrapAdmin(ctx, "transaction.admin", "Transaction Admin", password)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = bound.FindAccount(ctx, admin.Username); e != nil {
		t.Fatal("uncommitted identity not visible")
	}
	_, e = staff.CreateWorker(ctx, admin, auth.WorkerInput{Username: "transaction.worker", Name: "Transaction Worker", Password: password, EmployeeNo: "TX-1", Email: "worker@example.com", Phone: "0812345678"})
	if e != nil {
		t.Fatal(e)
	}
	customers := auth.NewCustomerService(bound, hasher)
	owner, e := customers.Register(ctx, auth.Registration{Username: "transaction.owner", Name: "Owner", Password: password, CustomerType: "company", CompanyName: "Transaction Company"})
	if e != nil {
		t.Fatal(e)
	}
	profiles, e := customers.Profiles(ctx, owner)
	if e != nil || len(profiles) != 1 {
		t.Fatal("uncommitted membership not visible")
	}
	invitation, e := customers.Invite(ctx, owner, profiles[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = customers.Register(ctx, auth.Registration{Username: "transaction.member", Name: "Member", Password: password, CustomerType: "company", Invitation: invitation})
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"person", "account", "worker", "admin", "customer", "customer_member", "customer_invitation", "auth_session"} {
		var count int
		// Fixed test-owned table names, never user input.
		if e = db.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil || count != 0 {
			t.Fatalf("outer rollback leaked %s", table)
		}
	}
}

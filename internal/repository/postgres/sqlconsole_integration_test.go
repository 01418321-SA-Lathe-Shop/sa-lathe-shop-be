package postgres

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/sqlconsole"
)

// Dedicated empty database only: this test runs DDL and DML through the console.
func TestSQLConsoleIntegration(t *testing.T) {
	dsn := os.Getenv("SQLCONSOLE_DATABASE_URL")
	if dsn == "" {
		t.Skip("set dedicated SQLCONSOLE_DATABASE_URL")
	}
	if u, e := url.Parse(dsn); e != nil || u.Path != "/sqlconsole_test" {
		t.Fatal("dedicated database /sqlconsole_test required")
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	run := func(sql string) sqlconsole.Result {
		t.Helper()
		r, e := db.ExecuteSQL(ctx, sql, 5*time.Second, 1000)
		if e != nil {
			t.Fatalf("%s: %v", sql, e)
		}
		return r
	}
	run(`DROP TABLE IF EXISTS c_order, c_customer`)
	run(`CREATE TABLE c_customer (id serial PRIMARY KEY, name text NOT NULL)`)
	run(`CREATE TABLE c_order (id serial PRIMARY KEY, customer_id int REFERENCES c_customer(id), amount numeric(10,2) NOT NULL DEFAULT 0)`)
	run(`ALTER TABLE c_order ADD COLUMN note text`)
	run(`ALTER TABLE c_order ALTER COLUMN note SET DEFAULT 'n/a'`)
	run(`CREATE INDEX c_order_customer_idx ON c_order(customer_id)`)
	if r := run(`INSERT INTO c_customer(name) VALUES ('A'),('B'),('C')`); r.RowsAffected != 3 {
		t.Fatalf("insert affected %d", r.RowsAffected)
	}
	run(`INSERT INTO c_order(customer_id, amount) VALUES (1,10.50),(1,4.50),(2,7)`)
	r := run(`SELECT c.name, count(o.id) AS n, sum(o.amount) AS total FROM c_customer c LEFT JOIN c_order o ON o.customer_id=c.id WHERE c.name <> 'C' GROUP BY c.name HAVING count(o.id) > 0 ORDER BY total DESC`)
	if len(r.Rows) != 2 || *r.Rows[0][0] != "A" || *r.Rows[0][2] != "15.00" || r.Columns[2].Type != "numeric" {
		t.Fatalf("join/aggregate: %+v", r)
	}
	if r = run(`SELECT o.id FROM c_customer c RIGHT JOIN c_order o ON o.customer_id=c.id`); len(r.Rows) != 3 {
		t.Fatal("right join")
	}
	if r = run(`WITH x AS (SELECT * FROM c_order) SELECT (SELECT count(*) FROM x) c, NULL::text n`); *r.Rows[0][0] != "3" || r.Rows[0][1] != nil {
		t.Fatalf("cte/null: %+v", r)
	}
	if r = run(`UPDATE c_order SET amount = amount + 1 WHERE customer_id = 1`); r.RowsAffected != 2 || r.Command != "UPDATE 2" {
		t.Fatalf("update: %+v", r)
	}
	// FK violation fails with the database message and rolls back the whole statement.
	var dbe *sqlconsole.DBError
	if _, e := db.ExecuteSQL(ctx, `DELETE FROM c_customer WHERE id=1`, 5*time.Second, 10); !errors.As(e, &dbe) || dbe.SQLState != "23503" {
		t.Fatalf("fk: %v", e)
	}
	if _, e := db.ExecuteSQL(ctx, `selec 1`, 5*time.Second, 10); !errors.As(e, &dbe) || dbe.SQLState != "42601" || dbe.Position != 1 {
		t.Fatalf("syntax: %v", e)
	}
	if _, e := db.ExecuteSQL(ctx, `SELECT 1; SELECT 2`, 5*time.Second, 10); e == nil {
		t.Fatal("multiple statements must fail")
	}
	if r = run(`SELECT generate_series(1,50)`); len(r.Rows) != 50 {
		t.Fatal("series")
	}
	if r, _ = db.ExecuteSQL(ctx, `SELECT generate_series(1,50)`, 5*time.Second, 10); !r.Truncated || len(r.Rows) != 10 {
		t.Fatalf("cap: %+v", r)
	}
	if _, e := db.ExecuteSQL(ctx, `SELECT pg_sleep(3)`, 500*time.Millisecond, 10); !errors.As(e, &dbe) || dbe.SQLState != "57014" {
		t.Fatalf("timeout: %v", e)
	}
	// A failed statement leaves no partial change.
	if _, e := db.ExecuteSQL(ctx, `WITH d AS (DELETE FROM c_order RETURNING 1) SELECT 1/0 FROM d`, 5*time.Second, 10); e == nil {
		t.Fatal("expected division error")
	}
	if r = run(`SELECT count(*) FROM c_order`); *r.Rows[0][0] != "3" {
		t.Fatal("rolled back delete leaked")
	}
	tables, err := db.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tb := range tables {
		if tb.Name != "c_order" {
			continue
		}
		found = true
		fk, idx := false, false
		for _, k := range tb.Constraints {
			fk = fk || (k.Type == "foreign_key" && k.References == "c_customer")
		}
		for _, x := range tb.Indexes {
			idx = idx || x.Name == "c_order_customer_idx"
		}
		if len(tb.Columns) != 4 || !fk || !idx || tb.Columns[3].Default != "'n/a'::text" {
			t.Fatalf("schema: %+v", tb)
		}
	}
	if !found {
		t.Fatal("c_order missing from schema")
	}
	if r = run(`DROP TABLE c_order, c_customer`); r.Command != "DROP TABLE" {
		t.Fatal(r.Command)
	}
	if tables, _ = db.Schema(ctx); len(tables) != 0 {
		t.Fatalf("schema not refreshed after DDL: %d", len(tables))
	}
}

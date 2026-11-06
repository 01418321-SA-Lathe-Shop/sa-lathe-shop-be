package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/sqlconsole"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ sqlconsole.Repository = (*Database)(nil)

// ExecuteSQL runs exactly one statement (the extended protocol rejects more) in
// a transaction that commits only on success. Results use PostgreSQL text format.
func (d *Database) ExecuteSQL(ctx context.Context, sql string, timeout time.Duration, maxRows int) (sqlconsole.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return sqlconsole.Result{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", timeout.Milliseconds())); err != nil {
		return sqlconsole.Result{}, err
	}
	start := time.Now()
	rows, err := tx.Query(ctx, sql, pgx.QueryResultFormats{pgx.TextFormatCode})
	if err != nil {
		return sqlconsole.Result{}, dbError(err)
	}
	res := sqlconsole.Result{Columns: []sqlconsole.Column{}, Rows: [][]*string{}}
	types := tx.Conn().TypeMap()
	for _, f := range rows.FieldDescriptions() {
		name := fmt.Sprintf("oid:%d", f.DataTypeOID)
		if t, ok := types.TypeForOID(f.DataTypeOID); ok {
			name = t.Name
		}
		res.Columns = append(res.Columns, sqlconsole.Column{Name: f.Name, Type: name})
	}
	for rows.Next() {
		if len(res.Rows) == maxRows {
			res.Truncated = true
			break
		}
		raw := rows.RawValues()
		row := make([]*string, len(raw))
		for i, v := range raw {
			if v != nil {
				s := string(v)
				row[i] = &s
			}
		}
		res.Rows = append(res.Rows, row)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return sqlconsole.Result{}, dbError(err)
	}
	tag := rows.CommandTag()
	if err = tx.Commit(ctx); err != nil {
		return sqlconsole.Result{}, dbError(err)
	}
	res.Command, res.RowsAffected, res.ElapsedMs = tag.String(), tag.RowsAffected(), time.Since(start).Milliseconds()
	if res.Truncated {
		res.RowsAffected = int64(len(res.Rows))
	}
	return res, nil
}

func dbError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return &sqlconsole.DBError{Message: pg.Message, SQLState: pg.Code, Position: int(pg.Position), Detail: pg.Detail, Hint: pg.Hint}
	}
	return err
}

func (d *Database) Schema(ctx context.Context) ([]sqlconsole.SchemaTable, error) {
	var tables []sqlconsole.SchemaTable
	index := map[string]int{}
	cols, err := d.pool.Query(ctx, `SELECT c.relname, CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'table' ELSE 'view' END,
 a.attname, format_type(a.atttypid,a.atttypmod), a.attnotnull, COALESCE(pg_get_expr(d.adbin,d.adrelid),'')
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
WHERE n.nspname='public' AND c.relkind IN ('r','p','v') ORDER BY c.relname, a.attnum`)
	if err != nil {
		return nil, err
	}
	for cols.Next() {
		var table, kind string
		var c sqlconsole.SchemaColumn
		if err = cols.Scan(&table, &kind, &c.Name, &c.Type, &c.NotNull, &c.Default); err != nil {
			cols.Close()
			return nil, err
		}
		i, ok := index[table]
		if !ok {
			i = len(tables)
			index[table] = i
			tables = append(tables, sqlconsole.SchemaTable{Name: table, Kind: kind, Columns: []sqlconsole.SchemaColumn{}, Constraints: []sqlconsole.SchemaConstraint{}, Indexes: []sqlconsole.SchemaIndex{}})
		}
		tables[i].Columns = append(tables[i].Columns, c)
	}
	cols.Close()
	if err = cols.Err(); err != nil {
		return nil, err
	}
	cons, err := d.pool.Query(ctx, `SELECT c.relname, k.conname, k.contype::text, pg_get_constraintdef(k.oid), COALESCE(rc.relname,'')
FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
LEFT JOIN pg_class rc ON rc.oid=k.confrelid
WHERE n.nspname='public' AND k.contype IN ('p','u','f','c') ORDER BY c.relname, k.conname`)
	if err != nil {
		return nil, err
	}
	kinds := map[string]string{"p": "primary_key", "u": "unique", "f": "foreign_key", "c": "check"}
	for cons.Next() {
		var table, kind string
		var k sqlconsole.SchemaConstraint
		if err = cons.Scan(&table, &k.Name, &kind, &k.Definition, &k.References); err != nil {
			cons.Close()
			return nil, err
		}
		k.Type = kinds[kind]
		if i, ok := index[table]; ok {
			tables[i].Constraints = append(tables[i].Constraints, k)
		}
	}
	cons.Close()
	if err = cons.Err(); err != nil {
		return nil, err
	}
	idx, err := d.pool.Query(ctx, `SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname='public' ORDER BY tablename, indexname`)
	if err != nil {
		return nil, err
	}
	for idx.Next() {
		var table string
		var x sqlconsole.SchemaIndex
		if err = idx.Scan(&table, &x.Name, &x.Definition); err != nil {
			idx.Close()
			return nil, err
		}
		if i, ok := index[table]; ok {
			tables[i].Indexes = append(tables[i].Indexes, x)
		}
	}
	idx.Close()
	if err = idx.Err(); err != nil {
		return nil, err
	}
	if tables == nil {
		tables = []sqlconsole.SchemaTable{}
	}
	return tables, nil
}

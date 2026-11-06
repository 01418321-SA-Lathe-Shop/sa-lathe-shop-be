package sqlconsole

import (
	"context"
	"time"
)

const (
	MaxRows      = 1000
	MaxSQLBytes  = 64 * 1024
	StatementTTL = 10 * time.Second
)

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Rows hold PostgreSQL text output; nil is SQL NULL.
type Result struct {
	Command      string      `json:"command"`
	RowsAffected int64       `json:"rows_affected"`
	Columns      []Column    `json:"columns"`
	Rows         [][]*string `json:"rows"`
	Truncated    bool        `json:"truncated"`
	ElapsedMs    int64       `json:"elapsed_ms"`
}

// DBError carries a database-reported failure back to the user verbatim.
type DBError struct {
	Message  string
	SQLState string
	Position int
	Detail   string
	Hint     string
}

func (e *DBError) Error() string { return e.Message }

type SchemaColumn struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	NotNull bool   `json:"not_null"`
	Default string `json:"default,omitempty"`
}
type SchemaConstraint struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // primary_key, unique, foreign_key, check
	Definition string `json:"definition"`
	References string `json:"references,omitempty"`
}
type SchemaIndex struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}
type SchemaTable struct {
	Name        string             `json:"name"`
	Kind        string             `json:"kind"` // table, view
	Columns     []SchemaColumn     `json:"columns"`
	Constraints []SchemaConstraint `json:"constraints"`
	Indexes     []SchemaIndex      `json:"indexes"`
}

type Repository interface {
	ExecuteSQL(ctx context.Context, sql string, timeout time.Duration, maxRows int) (Result, error)
	Schema(ctx context.Context) ([]SchemaTable, error)
}

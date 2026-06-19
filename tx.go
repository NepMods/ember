package ember

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

var savepointNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validateSavepointName(name string) error {
	if !savepointNameRe.MatchString(name) {
		return fmt.Errorf("ember: invalid savepoint name %q", name)
	}
	return nil
}

// Tx wraps a database transaction.
type Tx struct {
	tx *sql.Tx
	db *DB
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("ember: commit: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction.
func (t *Tx) Rollback() error {
	if err := t.tx.Rollback(); err != nil && err != sql.ErrTxDone {
		return fmt.Errorf("ember: rollback: %w", err)
	}
	return nil
}

// Table returns a new Builder within this transaction.
func (t *Tx) Table(name string) *Builder {
	return newBuilder(t.db, t, name)
}

// Raw creates a raw SQL query within this transaction.
func (t *Tx) Raw(query string, bindings ...interface{}) *RawQuery {
	return &RawQuery{tx: t, sql: query, bindings: bindings}
}

// Savepoint creates a savepoint in the transaction.
func (t *Tx) Savepoint(ctx context.Context, name string) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	_, err := t.tx.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %s", name))
	return err
}

// RollbackTo rolls back to a named savepoint.
func (t *Tx) RollbackTo(ctx context.Context, name string) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	_, err := t.tx.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", name))
	return err
}

// ReleaseSavepoint releases a named savepoint.
func (t *Tx) ReleaseSavepoint(ctx context.Context, name string) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	_, err := t.tx.ExecContext(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", name))
	return err
}

// ExecContext executes a write query within the transaction.
func (t *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, newQueryError(query, args, err)
	}
	return res, nil
}

// QueryContext executes a query within the transaction.
func (t *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, newQueryError(query, args, err)
	}
	return rows, nil
}

// QueryRowContext returns a Row whose Scan errors are wrapped in QueryError.
func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *queryRow {
	return &queryRow{row: t.tx.QueryRowContext(ctx, query, args...), query: query, args: args}
}

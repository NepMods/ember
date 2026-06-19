package ember

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
)

// RawQuery represents a raw SQL query.
type RawQuery struct {
	db       *DB
	tx       *Tx
	sql      string
	bindings []interface{}
}

// Scan executes the raw query and scans results into dest.
func (r *RawQuery) Scan(ctx context.Context, dest interface{}) error {
	if dest == nil {
		return fmt.Errorf("ember/raw: dest must not be nil")
	}
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() == reflect.Ptr && !destVal.IsNil() && destVal.Elem().Kind() == reflect.Struct {
		return r.First(ctx, dest)
	}
	rows, err := r.queryRows(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanRows(rows, dest)
}

// First executes the raw query and scans the first row into dest.
func (r *RawQuery) First(ctx context.Context, dest interface{}) error {
	if dest == nil {
		return fmt.Errorf("ember/raw: dest must not be nil")
	}
	rows, err := r.queryRows(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrNotFound
	}
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.IsNil() {
		return fmt.Errorf("ember/raw: First() dest must be a non-nil pointer")
	}
	elem := destVal.Elem()
	if elem.Kind() == reflect.Struct {
		schema, err := ParseSchema(dest)
		if err != nil {
			return err
		}
		ptrs, timeFields := scanPointersByCol(cols, elem, schema, r.dialect().Name() != "postgres")
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		scanTimeStrings(elem, ptrs, timeFields, schema)
		applyCastsOnRead(elem, schema)
		return rows.Err()
	}
	if len(cols) > 1 {
		return fmt.Errorf("ember/raw: First() with scalar dest on multi-column result")
	}
	if err := rows.Scan(dest); err != nil {
		return err
	}
	return rows.Err()
}

// Get is an alias for Scan.
func (r *RawQuery) Get(ctx context.Context, dest interface{}) error {
	return r.Scan(ctx, dest)
}

// Exec executes the raw query and returns the result.
func (r *RawQuery) Exec(ctx context.Context) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, r.sql, r.bindings...)
	}
	return r.db.ExecContext(ctx, r.sql, r.bindings...)
}

// ExecAffected executes the raw query and returns rows affected.
func (r *RawQuery) ExecAffected(ctx context.Context) (int64, error) {
	res, err := r.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Pluck executes the raw query and scans a single column into dest.
func (r *RawQuery) Pluck(ctx context.Context, dest interface{}) error {
	rows, err := r.queryRows(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanColumn(rows, dest)
}

// Value executes the raw query and scans a single value into dest.
func (r *RawQuery) Value(ctx context.Context, dest interface{}) error {
	row := r.queryRowRaw(ctx)
	return row.Scan(dest)
}

// ToSQL returns the raw SQL and bindings.
func (r *RawQuery) ToSQL() (string, []interface{}) {
	return r.sql, r.bindings
}

func (r *RawQuery) dialect() Dialect {
	if r.tx != nil {
		return r.tx.db.Dialect()
	}
	if r.db != nil {
		return r.db.Dialect()
	}
	panic("ember: RawQuery has no DB or Tx set")
}

func (r *RawQuery) queryRows(ctx context.Context) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, r.sql, r.bindings...)
	}
	return r.db.QueryContext(ctx, r.sql, r.bindings...)
}

func (r *RawQuery) queryRowRaw(ctx context.Context) *queryRow {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, r.sql, r.bindings...)
	}
	return r.db.QueryRowContext(ctx, r.sql, r.bindings...)
}

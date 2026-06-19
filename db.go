package ember

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

type queryRow struct {
	row   *sql.Row
	query string
	args  []interface{}
}

func (r *queryRow) Scan(dest ...interface{}) error {
	err := r.row.Scan(dest...)
	if err != nil {
		return newQueryError(r.query, r.args, err)
	}
	return err
}

// DB wraps a sql.DB connection pool with dialect and event support.
type DB struct {
	config     Config
	dialect    Dialect
	master     *sql.DB
	replicas   []*sql.DB // immutable after Open(); do not modify
	replicaIdx uint64

	scopes *ScopeRegistry

	eventsMu sync.RWMutex
	events   *EventDispatcher
}

type ctxKey int

const (
	ctxKeySticky ctxKey = iota
)

// Open initializes a database connection from the given config.
func Open(cfg Config) (*DB, error) {
	if cfg.Driver == "" {
		return nil, fmt.Errorf("ember: driver cannot be empty")
	}
	if cfg.Master == "" {
		return nil, fmt.Errorf("ember: master DSN cannot be empty")
	}

	d, err := GetDialect(cfg.Driver)
	if err != nil {
		return nil, err
	}
	db := &DB{
		config:  cfg,
		dialect: d,
		scopes:  NewScopeRegistry(),
	}

	masterDB, err := sql.Open(cfg.Driver, cfg.Master)
	if err != nil {
		return nil, fmt.Errorf("ember: failed to open master: %w", err)
	}
	applyPool(masterDB, cfg.Pool)
	if err := masterDB.PingContext(context.Background()); err != nil {
		masterDB.Close()
		return nil, fmt.Errorf("ember: failed to ping master: %w", err)
	}
	db.master = masterDB

	var opened []*sql.DB
	defer func() {
		for _, o := range opened {
			o.Close()
		}
	}()
	for i, dsn := range cfg.Replicas {
		rep, err := sql.Open(cfg.Driver, dsn)
		if err != nil {
			masterDB.Close()
			return nil, fmt.Errorf("ember: failed to open replica %d: %w", i, err)
		}
		opened = append(opened, rep)
		applyPool(rep, cfg.Pool)
		if err := rep.PingContext(context.Background()); err != nil {
			masterDB.Close()
			return nil, fmt.Errorf("ember: failed to ping replica %d: %w", i, err)
		}
		db.replicas = append(db.replicas, rep)
	}
	opened = nil

	if len(db.replicas) == 0 {
		db.replicas = append(db.replicas, masterDB)
	}

	return db, nil
}

func applyPool(d *sql.DB, p PoolConfig) {
	if p.MaxOpenConns > 0 {
		d.SetMaxOpenConns(p.MaxOpenConns)
	}
	if p.MaxIdleConns > 0 {
		d.SetMaxIdleConns(p.MaxIdleConns)
	}
	if p.ConnMaxLifetime > 0 {
		d.SetConnMaxLifetime(p.ConnMaxLifetime)
	}
	if p.ConnMaxIdleTime > 0 {
		d.SetConnMaxIdleTime(p.ConnMaxIdleTime)
	}
}

// Close closes all database connections (master and replicas).
func (db *DB) Close() error {
	var errs []string
	if db.master != nil {
		if err := db.master.Close(); err != nil {
			errs = append(errs, "master: "+err.Error())
		}
	}
	for i, rep := range db.replicas {
		if rep == db.master {
			continue
		}
		if err := rep.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("replica[%d]: %v", i, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ember: close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Ping verifies connectivity to master and all replicas.
func (db *DB) Ping(ctx context.Context) error {
	if err := db.master.PingContext(ctx); err != nil {
		return fmt.Errorf("ember: master ping failed: %w", err)
	}
	for i, rep := range db.replicas {
		if rep == db.master {
			continue
		}
		if err := rep.PingContext(ctx); err != nil {
			return fmt.Errorf("ember: replica[%d] ping failed: %w", i, err)
		}
	}
	return nil
}

// Dialect returns the database dialect.
func (db *DB) Dialect() Dialect {
	return db.dialect
}

// Master returns the master *sql.DB.
func (db *DB) Master() *sql.DB {
	return db.master
}

func (db *DB) readDB(ctx context.Context) *sql.DB {
	if sticky, _ := ctx.Value(ctxKeySticky).(bool); sticky {
		return db.master
	}
	idx := atomic.AddUint64(&db.replicaIdx, 1)
	return db.replicas[idx%uint64(len(db.replicas))]
}

func (db *DB) writeDB() *sql.DB {
	return db.master
}

// WithStickyMaster forces subsequent reads to use the master connection.
func WithStickyMaster(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeySticky, true)
}

func isWriteCTE(s string) bool {
	upper := strings.ToUpper(s)
	// Build a set of byte positions that are inside string literals
	inString := make(map[int]bool)
	var in bool
	for i := 0; i < len(upper); i++ {
		if upper[i] == '\'' {
			if i+1 < len(upper) && upper[i+1] == '\'' {
				i++
				continue
			}
			in = !in
			continue
		}
		if in {
			inString[i] = true
		}
	}
	for _, kw := range []string{"INSERT", "UPDATE", "DELETE"} {
		for i := 0; i <= len(upper)-len(kw); i++ {
			if upper[i:i+len(kw)] != kw {
				continue
			}
			if inString[i] {
				continue
			}
			prevOK := i == 0 || !isIdentRune(rune(upper[i-1]))
			nextOK := i+len(kw) >= len(upper) || !isIdentRune(rune(upper[i+len(kw)]))
			if prevOK && nextOK {
				return true
			}
		}
	}
	return false
}

func isIdentRune(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func hasWord(s, kw string) bool {
	for i := 0; i <= len(s)-len(kw); i++ {
		if s[i:i+len(kw)] != kw {
			continue
		}
		prevOK := i == 0 || !isIdentRune(rune(s[i-1]))
		nextOK := i+len(kw) >= len(s) || !isIdentRune(rune(s[i+len(kw)]))
		if prevOK && nextOK {
			return true
		}
	}
	return false
}

// ExecContext executes a write query on the master.
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	res, err := db.writeDB().ExecContext(ctx, query, args...)
	if err != nil {
		return nil, newQueryError(query, args, err)
	}
	return res, nil
}

// QueryContext routes SELECT queries to replicas, writes to master.
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	upper := strings.TrimSpace(strings.ToUpper(query))
	isSelect := strings.HasPrefix(upper, "SELECT")
	isWith := strings.HasPrefix(upper, "WITH") && !isWriteCTE(upper)
	hasForUpdate := hasWord(upper, "FOR UPDATE")
	var conn *sql.DB
	if (isSelect || isWith) && !hasForUpdate {
		conn = db.readDB(ctx)
	} else {
		conn = db.writeDB()
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, newQueryError(query, args, err)
	}
	return rows, nil
}

// QueryRowContext returns a Row whose Scan errors are wrapped in QueryError.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *queryRow {
	upper := strings.TrimSpace(strings.ToUpper(query))
	isSelect := strings.HasPrefix(upper, "SELECT")
	isWith := strings.HasPrefix(upper, "WITH") && !isWriteCTE(upper)
	hasForUpdate := hasWord(upper, "FOR UPDATE")
	var sqlRow *sql.Row
	if (isSelect || isWith) && !hasForUpdate {
		sqlRow = db.readDB(ctx).QueryRowContext(ctx, query, args...)
	} else {
		sqlRow = db.writeDB().QueryRowContext(ctx, query, args...)
	}
	return &queryRow{row: sqlRow, query: query, args: args}
}

// Begin starts a new transaction on the master connection.
func (db *DB) Begin(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	sqlTx, err := db.master.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("ember: begin transaction: %w", err)
	}
	return &Tx{tx: sqlTx, db: db}, nil
}

// Transaction runs fn inside a transaction, auto-committing on success.
func (db *DB) Transaction(ctx context.Context, fn func(tx *Tx) error) error {
	return db.TransactionWithOptions(ctx, nil, fn)
}

// TransactionWithOptions is like Transaction with explicit TxOptions.
func (db *DB) TransactionWithOptions(ctx context.Context, opts *sql.TxOptions, fn func(tx *Tx) error) error {
	tx, err := db.Begin(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Table returns a new Builder for the given table.
func (db *DB) Table(name string) *Builder {
	return newBuilder(db, nil, name)
}

// Raw creates a raw SQL query.
func (db *DB) Raw(sql string, bindings ...interface{}) *RawQuery {
	return &RawQuery{db: db, sql: sql, bindings: bindings}
}

// ScopeRegistry returns the global scope registry.
func (db *DB) ScopeRegistry() *ScopeRegistry {
	return db.scopes
}

// SetEventDispatcher sets the event dispatcher for lifecycle events.
func (db *DB) SetEventDispatcher(ed *EventDispatcher) {
	db.eventsMu.Lock()
	db.events = ed
	db.eventsMu.Unlock()
}

// EventDispatcher returns the current event dispatcher.
func (db *DB) EventDispatcher() *EventDispatcher {
	db.eventsMu.RLock()
	defer db.eventsMu.RUnlock()
	return db.events
}

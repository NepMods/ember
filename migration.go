package ember

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Migration defines a database migration with up and down methods.
type Migration interface {
	Version() string
	Up(schema *Schema) error
	Down(schema *Schema) error
}

// Schema wraps a DB connection for schema migrations.
type Schema struct {
	db      *DB
	dialect Dialect
}

// NewSchema creates a new Schema wrapper for the given DB.
func NewSchema(db *DB) *Schema {
	return &Schema{db: db, dialect: db.dialect}
}

func newSchema(db *DB) *Schema {
	return NewSchema(db)
}

// Schema.Create
func (s *Schema) Create(table string, fn func(*Blueprint)) error {
	return s.CreateContext(context.Background(), table, fn)
}

// Schema.CreateContext
func (s *Schema) CreateContext(ctx context.Context, table string, fn func(*Blueprint)) error {
	bp := newBlueprint(table)
	fn(bp)
	createSQL := bp.ToCreateSQL(s.dialect)
	if _, err := s.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("ember/migrate: create table %s: %w", table, err)
	}
	for _, idxSQL := range bp.ToIndexSQL(s.dialect) {
		if _, err := s.db.ExecContext(ctx, idxSQL); err != nil {
			return fmt.Errorf("ember/migrate: create index on %s: %w", table, err)
		}
	}
	return nil
}

// Schema.Table
func (s *Schema) Table(table string, fn func(*Blueprint)) error {
	return s.TableContext(context.Background(), table, fn)
}

// Schema.TableContext
func (s *Schema) TableContext(ctx context.Context, table string, fn func(*Blueprint)) error {
	bp := newBlueprint(table)
	fn(bp)
	for _, alterSQL := range bp.ToAlterSQL(s.dialect) {
		if _, err := s.db.ExecContext(ctx, alterSQL); err != nil {
			return fmt.Errorf("ember/migrate: alter table %s: %w", table, err)
		}
	}
	return nil
}

// Schema.Drop
func (s *Schema) Drop(table string) error {
	_, err := s.db.ExecContext(context.Background(), "DROP TABLE "+s.dialect.QuoteIdentifier(table))
	return err
}

// Schema.DropIfExists
func (s *Schema) DropIfExists(table string) error {
	bp := newBlueprint(table)
	_, err := s.db.ExecContext(context.Background(), bp.ToDropSQL(s.dialect))
	return err
}

// Schema.Raw
func (s *Schema) Raw(sql string) error {
	return s.RawContext(context.Background(), sql)
}

// Schema.RawContext
func (s *Schema) RawContext(ctx context.Context, sql string) error {
	_, err := s.db.ExecContext(ctx, sql)
	return err
}

// Schema.HasTable
func (s *Schema) HasTable(table string) (bool, error) {
	return s.HasTableContext(context.Background(), table)
}

// Schema.HasTableContext
func (s *Schema) HasTableContext(ctx context.Context, table string) (bool, error) {
	var query string
	var args []interface{}
	switch s.dialect.Name() {
	case "postgres":
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1"
		args = []interface{}{table}
	case "mysql":
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
		args = []interface{}{table}
	default:
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		args = []interface{}{table}
	}
	var count int
	ctx = WithStickyMaster(ctx)
	row := s.db.QueryRowContext(ctx, query, args...)
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// Schema.HasColumn
func (s *Schema) HasColumn(table, column string) (bool, error) {
	return s.HasColumnContext(context.Background(), table, column)
}

// Schema.HasColumnContext
func (s *Schema) HasColumnContext(ctx context.Context, table, column string) (bool, error) {
	var query string
	var args []interface{}
	switch s.dialect.Name() {
	case "postgres":
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2"
		args = []interface{}{table, column}
	case "mysql":
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?"
		args = []interface{}{table, column}
	default:
		query = "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?"
		args = []interface{}{table, column}
	}
	var count int
	ctx = WithStickyMaster(ctx)
	row := s.db.QueryRowContext(ctx, query, args...)
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// Migrator runs and rolls back database migrations.
type Migrator struct {
	db         *DB
	migrations []Migration
	table      string
}

// NewMigrator creates a new Migrator with the given DB and migrations.
func NewMigrator(db *DB, migrations ...Migration) *Migrator {
	return &Migrator{
		db:         db,
		migrations: migrations,
		table:      "migrations",
	}
}

// Migrator.SetTable
func (m *Migrator) SetTable(name string) *Migrator {
	m.table = name
	return m
}

// Migrator.Add
func (m *Migrator) Add(migrations ...Migration) *Migrator {
	m.migrations = append(m.migrations, migrations...)
	return m
}

func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	s := newSchema(m.db)
	exists, err := s.HasTableContext(ctx, m.table)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	bp := newBlueprint(m.table)
	bp.ID()
	bp.String("version", 255).Unique()
	bp.Timestamp("executed_at").Nullable()
	createSQL := bp.ToCreateSQL(m.db.dialect)
	_, err = m.db.ExecContext(ctx, createSQL)
	return err
}

// Migrator.Ran
func (m *Migrator) Ran(ctx context.Context) (map[string]bool, error) {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return nil, err
	}
	ctx = WithStickyMaster(ctx)
	rows, err := m.db.QueryContext(ctx,
		fmt.Sprintf("SELECT version FROM %s", m.db.dialect.QuoteIdentifier(m.table)),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ran := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		ran[v] = true
	}
	return ran, rows.Err()
}

// Migrator.Pending
func (m *Migrator) Pending(ctx context.Context) ([]Migration, error) {
	ran, err := m.Ran(ctx)
	if err != nil {
		return nil, err
	}
	sorted := m.sorted()
	var pending []Migration
	for _, mg := range sorted {
		if !ran[mg.Version()] {
			pending = append(pending, mg)
		}
	}
	return pending, nil
}

// Migrator.Run
func (m *Migrator) Run(ctx context.Context) error {
	pending, err := m.Pending(ctx)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	schema := newSchema(m.db)
	for _, mg := range pending {
		if err := mg.Up(schema); err != nil {
			return fmt.Errorf("ember/migrate: running %s: %w", mg.Version(), err)
		}
		now := time.Now()
		if err := m.markRan(ctx, mg.Version(), now); err != nil {
			return fmt.Errorf("ember/migrate: recording %s: %w", mg.Version(), err)
		}
	}
	return nil
}

// Migrator.Rollback
func (m *Migrator) Rollback(ctx context.Context, steps int) error {
	ran, err := m.Ran(ctx)
	if err != nil {
		return err
	}
	if len(ran) == 0 {
		return nil
	}

	sorted := m.sorted()
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}

	schema := newSchema(m.db)
	rolled := 0
	for _, mg := range sorted {
		if !ran[mg.Version()] {
			continue
		}
		if steps > 0 && rolled >= steps {
			break
		}
		if err := mg.Down(schema); err != nil {
			return fmt.Errorf("ember/migrate: rolling back %s: %w", mg.Version(), err)
		}
		if err := m.markRolledBack(ctx, mg.Version()); err != nil {
			return fmt.Errorf("ember/migrate: unrecording %s: %w", mg.Version(), err)
		}
		rolled++
	}
	return nil
}

// Migrator.Fresh
func (m *Migrator) Fresh(ctx context.Context) error {
	if err := m.dropAllTables(ctx); err != nil {
		return err
	}
	schema := newSchema(m.db)
	if err := schema.DropIfExists(m.table); err != nil {
		return fmt.Errorf("ember/migrate: dropping migrations table: %w", err)
	}
	return m.Run(ctx)
}

// MigrationStatus describes a single migration run.
type MigrationStatus struct {
	Version string
	Ran     bool
}

// Migrator.Status
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	ran, err := m.Ran(ctx)
	if err != nil {
		return nil, err
	}
	var statuses []MigrationStatus
	for _, mg := range m.sorted() {
		statuses = append(statuses, MigrationStatus{
			Version: mg.Version(),
			Ran:     ran[mg.Version()],
		})
	}
	return statuses, nil
}

func (m *Migrator) sorted() []Migration {
	cp := make([]Migration, len(m.migrations))
	copy(cp, m.migrations)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Version() < cp[j].Version()
	})
	return cp
}

func (m *Migrator) markRan(ctx context.Context, version string, t time.Time) error {
	tbl := m.db.dialect.QuoteIdentifier(m.table)
	p1 := m.db.dialect.Placeholder(1)
	p2 := m.db.dialect.Placeholder(2)
	query := fmt.Sprintf("INSERT INTO %s (version, executed_at) VALUES (%s, %s)", tbl, p1, p2)
	_, err := m.db.ExecContext(ctx, query, version, t)
	return err
}

func (m *Migrator) markRolledBack(ctx context.Context, version string) error {
	tbl := m.db.dialect.QuoteIdentifier(m.table)
	p1 := m.db.dialect.Placeholder(1)
	query := fmt.Sprintf("DELETE FROM %s WHERE version = %s", tbl, p1)
	_, err := m.db.ExecContext(ctx, query, version)
	return err
}

func (m *Migrator) dropAllTables(ctx context.Context) error {
	var tables []string
	var err error

	switch m.db.dialect.Name() {
	case "postgres":
		rows, e := m.db.QueryContext(ctx,
			"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'")
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			if err = rows.Scan(&t); err != nil {
				return err
			}
			tables = append(tables, t)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		if len(tables) > 0 {
			quoted := make([]string, len(tables))
			for i, t := range tables {
				quoted[i] = m.db.dialect.QuoteIdentifier(t)
			}
			_, err = m.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+strings.Join(quoted, ", ")+" CASCADE")
		}
	case "mysql":
		rows, e := m.db.QueryContext(ctx,
			"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()")
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			if err = rows.Scan(&t); err != nil {
				return err
			}
			tables = append(tables, t)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		if len(tables) > 0 {
			conn, err := m.db.Master().Conn(ctx)
			if err != nil {
				return fmt.Errorf("ember/migrate: get connection for drop tables: %w", err)
			}
			defer conn.Close()
			if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
				return fmt.Errorf("ember/migrate: disable FK checks: %w", err)
			}
			defer conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")
			for _, t := range tables {
				if _, e := conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+
					m.db.dialect.QuoteIdentifier(t)); e != nil {
					return e
				}
			}
		}
	default:
		rows, e := m.db.QueryContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			if err = rows.Scan(&t); err != nil {
				return err
			}
			tables = append(tables, t)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		for _, t := range tables {
			if _, e := m.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+
				m.db.dialect.QuoteIdentifier(t)); e != nil {
				return e
			}
		}
	}
	return err
}

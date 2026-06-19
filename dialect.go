package ember

import (
	"fmt"
	"strings"
)

// Dialect abstracts SQL differences across database drivers.
type Dialect interface {
	Name() string
	QuoteIdentifier(name string) string
	Placeholder(index int) string
	SupportsReturning() bool
	UpsertClause(conflictCols []string, updateCols []string) (string, error)
}

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct{}

// PostgresDialect.Name
func (d *PostgresDialect) Name() string { return "postgres" }

// PostgresDialect.QuoteIdentifier
func (d *PostgresDialect) QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// PostgresDialect.Placeholder
func (d *PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// PostgresDialect.SupportsReturning
func (d *PostgresDialect) SupportsReturning() bool { return true }

// PostgresDialect.UpsertClause
func (d *PostgresDialect) UpsertClause(conflictCols []string, updateCols []string) (string, error) {
	if len(conflictCols) == 0 {
		return "", fmt.Errorf("postgres upsert requires at least one conflict column")
	}
	quoted := make([]string, len(conflictCols))
	for i, c := range conflictCols {
		quoted[i] = d.QuoteIdentifier(c)
	}
	if len(updateCols) == 0 {
		return fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING",
			strings.Join(quoted, ", ")), nil
	}
	setClauses := make([]string, len(updateCols))
	for i, c := range updateCols {
		q := d.QuoteIdentifier(c)
		setClauses[i] = fmt.Sprintf("%s = EXCLUDED.%s", q, q)
	}
	return fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s",
		strings.Join(quoted, ", "),
		strings.Join(setClauses, ", ")), nil
}

// MySQLDialect implements Dialect for MySQL.
type MySQLDialect struct{}

// MySQLDialect.Name
func (d *MySQLDialect) Name() string { return "mysql" }

// MySQLDialect.QuoteIdentifier
func (d *MySQLDialect) QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// MySQLDialect.Placeholder
func (d *MySQLDialect) Placeholder(_ int) string { return "?" }

// MySQLDialect.SupportsReturning
func (d *MySQLDialect) SupportsReturning() bool { return false }

// MySQLDialect.UpsertClause
func (d *MySQLDialect) UpsertClause(conflictCols []string, updateCols []string) (string, error) {
	if len(conflictCols) > 0 {
		return "", fmt.Errorf("mysql upsert does not support conflict columns")
	}
	if len(updateCols) == 0 {
		return "", fmt.Errorf("mysql upsert requires at least one update column")
	}
	setClauses := make([]string, len(updateCols))
	for i, c := range updateCols {
		q := d.QuoteIdentifier(c)
		setClauses[i] = fmt.Sprintf("%s = new.%s", q, q)
	}
	return " ON DUPLICATE KEY UPDATE " + strings.Join(setClauses, ", "), nil
}

// SQLiteDialect implements Dialect for SQLite.
type SQLiteDialect struct{}

// SQLiteDialect.Name
func (d *SQLiteDialect) Name() string { return "sqlite3" }

// SQLiteDialect.QuoteIdentifier
func (d *SQLiteDialect) QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// SQLiteDialect.Placeholder
func (d *SQLiteDialect) Placeholder(_ int) string { return "?" }

// SQLiteDialect.SupportsReturning
func (d *SQLiteDialect) SupportsReturning() bool { return true }

// SQLiteDialect.UpsertClause
func (d *SQLiteDialect) UpsertClause(conflictCols []string, updateCols []string) (string, error) {
	if len(conflictCols) == 0 {
		if len(updateCols) > 0 {
			return "", fmt.Errorf("ember: sqlite upsert requires at least one conflict column when updating")
		}
		return " ON CONFLICT DO NOTHING", nil
	}
	quoted := make([]string, len(conflictCols))
	for i, c := range conflictCols {
		quoted[i] = d.QuoteIdentifier(c)
	}
	if len(updateCols) == 0 {
		return fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(quoted, ", ")), nil
	}
	setClauses := make([]string, len(updateCols))
	for i, c := range updateCols {
		q := d.QuoteIdentifier(c)
		setClauses[i] = fmt.Sprintf("%s = excluded.%s", q, q)
	}
	return fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s",
		strings.Join(quoted, ", "),
		strings.Join(setClauses, ", ")), nil
}

// GetDialect returns the Dialect for a given driver name.
func GetDialect(driver string) (Dialect, error) {
	switch driver {
	case "postgres", "pgx":
		return &PostgresDialect{}, nil
	case "mysql":
		return &MySQLDialect{}, nil
	case "sqlite3", "sqlite":
		return &SQLiteDialect{}, nil
	default:
		return nil, fmt.Errorf("ember: unknown driver %q", driver)
	}
}

package ember

import (
	"fmt"
	"strings"
)

// ColumnDef defines a column in a Blueprint.
type ColumnDef struct {
	name         string
	colType      string
	length       int
	precision    int
	scale        int
	nullable     bool
	defaultValue *string
	unsigned     bool
	autoIncr     bool
	unique       bool
	primary      bool
	after        string
	comment      string
}

// Nullable marks the column as nullable.
func (c *ColumnDef) Nullable() *ColumnDef {
	c.nullable = true
	return c
}

// Default sets the column's default value.
func (c *ColumnDef) Default(v interface{}) *ColumnDef {
	switch val := v.(type) {
	case string:
		quoted := "'" + strings.ReplaceAll(val, "'", "''") + "'"
		c.defaultValue = &quoted
	case bool:
		if val {
			s := "TRUE"
			c.defaultValue = &s
		} else {
			s := "FALSE"
			c.defaultValue = &s
		}
	case nil:
		s := "NULL"
		c.defaultValue = &s
	default:
		s := fmt.Sprintf("%v", val)
		c.defaultValue = &s
	}
	return c
}

// Unsigned marks the column as unsigned (MySQL only).
func (c *ColumnDef) Unsigned() *ColumnDef {
	c.unsigned = true
	return c
}

// Unique adds a UNIQUE constraint to the column.
func (c *ColumnDef) Unique() *ColumnDef {
	c.unique = true
	return c
}

// Comment sets the column comment (MySQL only).
func (c *ColumnDef) Comment(s string) *ColumnDef {
	c.comment = s
	return c
}

// After positions the column after another column (MySQL only).
func (c *ColumnDef) After(col string) *ColumnDef {
	c.after = col
	return c
}

// AutoIncrement marks the column as auto-incrementing.
func (c *ColumnDef) AutoIncrement() *ColumnDef {
	c.autoIncr = true
	return c
}

// IndexDef defines a database index.
type IndexDef struct {
	name    string
	columns []string
	unique  bool
	primary bool
}

// ForeignKeyDef defines a foreign key constraint.
type ForeignKeyDef struct {
	column    string
	refTable  string
	refColumn string
	onDelete  string
	onUpdate  string
	name      string
}

// OnDelete sets the ON DELETE action.
func (f *ForeignKeyDef) OnDelete(action string) *ForeignKeyDef {
	f.onDelete = strings.ToUpper(action)
	return f
}

// OnUpdate sets the ON UPDATE action.
func (f *ForeignKeyDef) OnUpdate(action string) *ForeignKeyDef {
	f.onUpdate = strings.ToUpper(action)
	return f
}

// CascadeOnDelete sets ON DELETE CASCADE.
func (f *ForeignKeyDef) CascadeOnDelete() *ForeignKeyDef {
	return f.OnDelete("CASCADE")
}

// NullOnDelete sets ON DELETE SET NULL.
func (f *ForeignKeyDef) NullOnDelete() *ForeignKeyDef {
	return f.OnDelete("SET NULL")
}

// Blueprint defines a database table schema for creation or alteration.
type Blueprint struct {
	table       string
	columns     []*ColumnDef
	indexes     []*IndexDef
	foreignKeys []*ForeignKeyDef
	drops       []string
	dropsIndex  []string

	hasTimestamps bool
	hasSoftDelete bool
}

func newBlueprint(table string) *Blueprint {
	return &Blueprint{table: table}
}

// NewBlueprintForTest creates a new Blueprint for testing.
func NewBlueprintForTest(table string) *Blueprint {
	return newBlueprint(table)
}

func (b *Blueprint) col(name, typ string) *ColumnDef {
	c := &ColumnDef{name: name, colType: typ}
	b.columns = append(b.columns, c)
	return c
}

// ID adds an auto-incrementing bigint primary key column.
func (b *Blueprint) ID() *ColumnDef {
	c := &ColumnDef{name: "id", colType: "bigint", autoIncr: true, primary: true, unsigned: true}
	b.columns = append(b.columns, c)
	return c
}

// UUID adds a UUID primary key column.
func (b *Blueprint) UUID(name ...string) *ColumnDef {
	n := "id"
	if len(name) > 0 {
		n = name[0]
	}
	c := b.col(n, "varchar")
	c.length = 36
	c.primary = true
	return c
}

// String adds a VARCHAR column.
func (b *Blueprint) String(name string, length ...int) *ColumnDef {
	c := b.col(name, "varchar")
	c.length = 255
	if len(length) > 0 {
		c.length = length[0]
	}
	return c
}

// Text adds a TEXT column.
func (b *Blueprint) Text(name string) *ColumnDef {
	return b.col(name, "text")
}

// MediumText adds a MEDIUMTEXT column.
func (b *Blueprint) MediumText(name string) *ColumnDef {
	return b.col(name, "mediumtext")
}

// LongText adds a LONGTEXT column.
func (b *Blueprint) LongText(name string) *ColumnDef {
	return b.col(name, "longtext")
}

// Integer adds an INT column.
func (b *Blueprint) Integer(name string) *ColumnDef {
	return b.col(name, "int")
}

// TinyInteger adds a TINYINT column.
func (b *Blueprint) TinyInteger(name string) *ColumnDef {
	return b.col(name, "tinyint")
}

// SmallInteger adds a SMALLINT column.
func (b *Blueprint) SmallInteger(name string) *ColumnDef {
	return b.col(name, "smallint")
}

// BigInteger adds a BIGINT column.
func (b *Blueprint) BigInteger(name string) *ColumnDef {
	return b.col(name, "bigint")
}

// Float adds a FLOAT column.
func (b *Blueprint) Float(name string) *ColumnDef {
	return b.col(name, "float")
}

// Double adds a DOUBLE column.
func (b *Blueprint) Double(name string) *ColumnDef {
	return b.col(name, "double")
}

// Decimal adds a DECIMAL column with given precision and scale.
func (b *Blueprint) Decimal(name string, precision, scale int) *ColumnDef {
	c := b.col(name, "decimal")
	c.precision = precision
	c.scale = scale
	return c
}

// Boolean adds a BOOLEAN column.
func (b *Blueprint) Boolean(name string) *ColumnDef {
	return b.col(name, "boolean")
}

// Date adds a DATE column.
func (b *Blueprint) Date(name string) *ColumnDef {
	return b.col(name, "date")
}

// DateTime adds a DATETIME column.
func (b *Blueprint) DateTime(name string) *ColumnDef {
	return b.col(name, "datetime")
}

// Timestamp adds a TIMESTAMP column.
func (b *Blueprint) Timestamp(name string) *ColumnDef {
	return b.col(name, "timestamp")
}

// TimestampTz adds a TIMESTAMPTZ column (PostgreSQL).
func (b *Blueprint) TimestampTz(name string) *ColumnDef {
	return b.col(name, "timestamptz")
}

// Time adds a TIME column.
func (b *Blueprint) Time(name string) *ColumnDef {
	return b.col(name, "time")
}

// Year adds a YEAR column.
func (b *Blueprint) Year(name string) *ColumnDef {
	return b.col(name, "year")
}

// JSON adds a JSON column.
func (b *Blueprint) JSON(name string) *ColumnDef {
	return b.col(name, "json")
}

// JSONB adds a JSONB column (PostgreSQL).
func (b *Blueprint) JSONB(name string) *ColumnDef {
	return b.col(name, "jsonb")
}

// Binary adds a BLOB column.
func (b *Blueprint) Binary(name string) *ColumnDef {
	return b.col(name, "blob")
}

// Enum adds an ENUM column with the given values.
func (b *Blueprint) Enum(name string, values []string) *ColumnDef {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
	}
	c := b.col(name, "enum")
	c.colType = "enum(" + strings.Join(quoted, ",") + ")"
	return c
}

// UnsignedBigInteger adds an unsigned BIGINT column.
func (b *Blueprint) UnsignedBigInteger(name string) *ColumnDef {
	c := b.col(name, "bigint")
	c.unsigned = true
	return c
}

// UnsignedInteger adds an unsigned INT column.
func (b *Blueprint) UnsignedInteger(name string) *ColumnDef {
	c := b.col(name, "int")
	c.unsigned = true
	return c
}

// Timestamps adds created_at and updated_at timestamp columns.
func (b *Blueprint) Timestamps() {
	b.Timestamp("created_at").Nullable()
	b.Timestamp("updated_at").Nullable()
	b.hasTimestamps = true
}

// SoftDeletes adds a nullable deleted_at timestamp column.
func (b *Blueprint) SoftDeletes() {
	b.Timestamp("deleted_at").Nullable()
	b.hasSoftDelete = true
}

// NullableMorphs adds nullable polymorphic relation columns.
func (b *Blueprint) NullableMorphs(name string) {
	b.String(name + "_type").Nullable()
	b.UnsignedBigInteger(name + "_id").Nullable()
}

// Index adds an index on the given columns.
func (b *Blueprint) Index(columns ...string) *IndexDef {
	idx := &IndexDef{
		name:    "idx_" + b.table + "_" + strings.Join(columns, "_"),
		columns: columns,
	}
	b.indexes = append(b.indexes, idx)
	return idx
}

// UniqueIndex adds a unique index on the given columns.
func (b *Blueprint) UniqueIndex(columns ...string) *IndexDef {
	idx := &IndexDef{
		name:    "uniq_" + b.table + "_" + strings.Join(columns, "_"),
		columns: columns,
		unique:  true,
	}
	b.indexes = append(b.indexes, idx)
	return idx
}

// Primary adds a primary key constraint on the given columns.
func (b *Blueprint) Primary(columns ...string) *IndexDef {
	idx := &IndexDef{columns: columns, primary: true}
	b.indexes = append(b.indexes, idx)
	return idx
}

// DropIndex marks an index for dropping on alter.
func (b *Blueprint) DropIndex(name string) {
	b.dropsIndex = append(b.dropsIndex, name)
}

// Foreign adds a foreign key constraint on the given column.
func (b *Blueprint) Foreign(column string) *ForeignKeyDef {
	fk := &ForeignKeyDef{
		column: column,
		name:   "fk_" + b.table + "_" + column,
	}
	b.foreignKeys = append(b.foreignKeys, fk)
	return fk
}

// References sets the referenced column.
func (f *ForeignKeyDef) References(column string) *ForeignKeyDef {
	f.refColumn = column
	return f
}

// On sets the referenced table.
func (f *ForeignKeyDef) On(table string) *ForeignKeyDef {
	f.refTable = table
	return f
}

// DropColumn marks columns for dropping on alter.
func (b *Blueprint) DropColumn(names ...string) {
	b.drops = append(b.drops, names...)
}

// ToCreateSQL generates a CREATE TABLE statement.
func (b *Blueprint) ToCreateSQL(d Dialect) string {
	var parts []string

	for _, c := range b.columns {
		parts = append(parts, columnToSQL(c, d))
	}

	pkSeen := make(map[string]bool)
	var pks []string
	for _, c := range b.columns {
		if c.primary && !c.autoIncr {
			n := d.QuoteIdentifier(c.name)
			if !pkSeen[n] {
				pkSeen[n] = true
				pks = append(pks, n)
			}
		}
	}
	for _, idx := range b.indexes {
		if idx.primary {
			for _, c := range idx.columns {
				n := d.QuoteIdentifier(c)
				if !pkSeen[n] {
					pkSeen[n] = true
					pks = append(pks, n)
				}
			}
		}
	}
	if len(pks) > 0 {
		parts = append(parts, "PRIMARY KEY ("+strings.Join(pks, ", ")+")")
	}

	for _, c := range b.columns {
		if c.unique && !c.primary {
			parts = append(parts, fmt.Sprintf("UNIQUE (%s)", d.QuoteIdentifier(c.name)))
		}
	}

	for _, fk := range b.foreignKeys {
		parts = append(parts, foreignKeyToSQL(fk, d))
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)",
		d.QuoteIdentifier(b.table),
		strings.Join(parts, ",\n  "),
	)
	return sql
}

// ToIndexSQL generates index creation statements.
func (b *Blueprint) ToIndexSQL(d Dialect) []string {
	var sqls []string
	for _, idx := range b.indexes {
		if idx.primary {
			continue
		}
		quotedCols := make([]string, len(idx.columns))
		for i, c := range idx.columns {
			quotedCols[i] = d.QuoteIdentifier(c)
		}
		idxType := "INDEX"
		if idx.unique {
			idxType = "UNIQUE INDEX"
		}
		sqls = append(sqls, fmt.Sprintf("CREATE %s %s ON %s (%s)",
			idxType,
			idx.name,
			d.QuoteIdentifier(b.table),
			strings.Join(quotedCols, ", "),
		))
	}
	return sqls
}

// ToDropSQL generates a DROP TABLE statement.
func (b *Blueprint) ToDropSQL(d Dialect) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s", d.QuoteIdentifier(b.table))
}

// ToAlterSQL generates ALTER TABLE statements.
func (b *Blueprint) ToAlterSQL(d Dialect) []string {
	var sqls []string

	for _, c := range b.columns {
		colDef := columnToSQL(c, d)
		after := ""
		if c.after != "" && d.Name() == "mysql" {
			after = " AFTER " + d.QuoteIdentifier(c.after)
		}
		sqls = append(sqls, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s%s",
			d.QuoteIdentifier(b.table), colDef, after))
	}

	for _, col := range b.drops {
		sqls = append(sqls, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s",
			d.QuoteIdentifier(b.table), d.QuoteIdentifier(col)))
	}

	for _, sql := range b.ToIndexSQL(d) {
		sqls = append(sqls, sql)
	}

	for _, idxName := range b.dropsIndex {
		if d.Name() == "mysql" {
			sqls = append(sqls, fmt.Sprintf("ALTER TABLE %s DROP INDEX %s",
				d.QuoteIdentifier(b.table), idxName))
		} else {
			sqls = append(sqls, fmt.Sprintf("DROP INDEX IF EXISTS %s", idxName))
		}
	}

	return sqls
}

func columnToSQL(c *ColumnDef, d Dialect) string {
	var sb strings.Builder
	sb.WriteString(d.QuoteIdentifier(c.name))
	sb.WriteString(" ")

	typStr := resolveType(c, d)
	sb.WriteString(typStr)

	if strings.HasPrefix(c.colType, "enum") && d.Name() == "postgres" {
		// Extract enum values from colType: enum('val1','val2')
		inner := c.colType[5 : len(c.colType)-1] // strip 'enum(' and ')'
		parts := splitEnumValues(inner)
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = "'" + strings.ReplaceAll(p, "'", "''") + "'"
		}
		sb.WriteString(" CHECK (" + d.QuoteIdentifier(c.name) + " IN (" + strings.Join(quoted, ", ") + "))")
	}

	if c.autoIncr {
		switch d.Name() {
		case "postgres":
		case "mysql":
			sb.WriteString(" AUTO_INCREMENT")
		case "sqlite3":
		}
	}

	if !c.nullable {
		sb.WriteString(" NOT NULL")
	}

	if c.defaultValue != nil {
		sb.WriteString(" DEFAULT ")
		sb.WriteString(*c.defaultValue)
	}

	if c.primary && c.autoIncr {
		sb.WriteString(" PRIMARY KEY")
	}

	if c.comment != "" && d.Name() == "mysql" {
		escaped := strings.ReplaceAll(c.comment, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, "'", "''")
		sb.WriteString(fmt.Sprintf(" COMMENT '%s'", escaped))
	}

	return sb.String()
}

func resolveType(c *ColumnDef, d Dialect) string {
	switch d.Name() {
	case "postgres":
		return resolvePostgresType(c)
	case "sqlite3":
		return resolveSQLiteType(c)
	default:
		return resolveMySQLType(c)
	}
}

func resolveMySQLType(c *ColumnDef) string {
	switch {
	case c.colType == "varchar":
		l := c.length
		if l == 0 {
			l = 255
		}
		return fmt.Sprintf("VARCHAR(%d)", l)
	case c.colType == "int" && c.unsigned:
		return "INT UNSIGNED"
	case c.colType == "bigint" && c.unsigned:
		return "BIGINT UNSIGNED"
	case c.colType == "decimal":
		return fmt.Sprintf("DECIMAL(%d,%d)", c.precision, c.scale)
	case strings.HasPrefix(c.colType, "enum"):
		return c.colType
	case c.colType == "boolean":
		return "TINYINT(1)"
	case c.colType == "blob":
		return "BLOB"
	case c.colType == "timestamptz":
		return "TIMESTAMP"
	default:
		return strings.ToUpper(c.colType)
	}
}

func resolvePostgresType(c *ColumnDef) string {
	switch {
	case c.colType == "varchar":
		l := c.length
		if l == 0 {
			l = 255
		}
		return fmt.Sprintf("VARCHAR(%d)", l)
	case c.colType == "bigint" && c.autoIncr:
		return "BIGSERIAL"
	case c.colType == "int" && c.autoIncr:
		return "SERIAL"
	case c.colType == "smallint" && c.autoIncr:
		return "SMALLSERIAL"
	case c.colType == "decimal":
		return fmt.Sprintf("DECIMAL(%d,%d)", c.precision, c.scale)
	case strings.HasPrefix(c.colType, "enum"):
		return "VARCHAR(255)"
	case c.colType == "boolean":
		return "BOOLEAN"
	case c.colType == "blob":
		return "BYTEA"
	case c.colType == "datetime":
		return "TIMESTAMP"
	case c.colType == "timestamptz":
		return "TIMESTAMPTZ"
	case c.colType == "tinyint":
		return "SMALLINT"
	case c.colType == "mediumtext", c.colType == "longtext":
		return "TEXT"
	case c.colType == "json":
		return "JSON"
	default:
		return strings.ToUpper(c.colType)
	}
}

func resolveSQLiteType(c *ColumnDef) string {
	switch {
	case c.colType == "varchar", c.colType == "text",
		c.colType == "mediumtext", c.colType == "longtext",
		c.colType == "json", c.colType == "jsonb",
		strings.HasPrefix(c.colType, "enum"):
		return "TEXT"
	case c.colType == "int", c.colType == "tinyint",
		c.colType == "smallint", c.colType == "bigint",
		c.colType == "boolean", c.colType == "year":
		return "INTEGER"
	case c.colType == "float", c.colType == "double", c.colType == "decimal":
		return "REAL"
	case c.colType == "blob":
		return "BLOB"
	case c.colType == "datetime", c.colType == "timestamp",
		c.colType == "timestamptz", c.colType == "date", c.colType == "time":
		return "TEXT"
	default:
		return strings.ToUpper(c.colType)
	}
}

func foreignKeyToSQL(fk *ForeignKeyDef, d Dialect) string {
	if fk.refTable == "" {
		panic("ember: foreign key refTable must not be empty")
	}
	if fk.refColumn == "" {
		panic("ember: foreign key refColumn must not be empty")
	}
	return fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)%s%s",
		d.QuoteIdentifier(fk.name),
		d.QuoteIdentifier(fk.column),
		d.QuoteIdentifier(fk.refTable),
		d.QuoteIdentifier(fk.refColumn),
		fkOnDelete(fk),
		fkOnUpdate(fk),
	)
}

func fkOnDelete(fk *ForeignKeyDef) string {
	if fk.onDelete != "" {
		return " ON DELETE " + fk.onDelete
	}
	return ""
}

func fkOnUpdate(fk *ForeignKeyDef) string {
	if fk.onUpdate != "" {
		return " ON UPDATE " + fk.onUpdate
	}
	return ""
}

func splitEnumValues(s string) []string {
	var result []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' {
			if inQuote && i+1 < len(s) && s[i+1] == '\'' {
				cur.WriteByte('\'')
				i++
				continue
			}
			inQuote = !inQuote
			continue
		}
		if ch == ',' && !inQuote {
			result = append(result, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(ch)
	}
	if cur.Len() > 0 {
		result = append(result, cur.String())
	}
	return result
}

package ember

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

var validOperators = map[string]bool{
	"=": true, "!=": true, "<>": true, "<": true, ">": true,
	"<=": true, ">=": true, "LIKE": true, "NOT LIKE": true,
	"ILIKE": true, "~": true, "!~": true, "REGEXP": true,
}

var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05",
	time.RFC3339,
	"2006-01-02",
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("ember: cannot parse time string %q", s)
}

type whereClause struct {
	raw     string
	rawArgs []interface{}

	column   string
	operator string
	value    interface{}

	boolean string
	isGroup bool
	group   []whereClause

	isNull   bool
	notNull  bool
	isIn     bool
	notIn    bool
	inValues []interface{}

	isBetween bool
	between   [2]interface{}
}

type joinClause struct {
	joinType  string
	table     string
	first     string
	operator  string
	second    string
	rawOn     string
	rawOnArgs []interface{}
}

type orderByClause struct {
	raw string
}

// Builder builds and executes SQL queries.
type Builder struct {
	db      *DB
	tx      *Tx
	dialect Dialect

	table      string
	tableAlias string

	selects  []string
	distinct bool

	wheres   []whereClause
	joins    []joinClause
	orderBys []orderByClause
	groupBys []string
	havings  []whereClause

	limit  *int
	offset *int

	insertCols   []string
	insertValues [][]interface{}

	updateSets   []string
	updateValues []interface{}

	conflictCols     []string
	updateOnConflict []string

	lockMode string

	macros map[string]BuilderMacro

	err error
}

func newBuilder(db *DB, tx *Tx, table string) *Builder {
	return &Builder{
		db:      db,
		tx:      tx,
		dialect: db.dialect,
		table:   table,
		selects: []string{"*"},
	}
}

func (b *Builder) clone() *Builder {
	c := *b
	c.selects = copySlice(b.selects)
	c.wheres = copySlice(b.wheres)
	c.joins = copySlice(b.joins)
	c.orderBys = copySlice(b.orderBys)
	c.groupBys = copySlice(b.groupBys)
	c.havings = copySlice(b.havings)
	c.insertCols = copySlice(b.insertCols)
	c.insertValues = copyNestedSlice(b.insertValues)
	c.updateSets = copySlice(b.updateSets)
	c.updateValues = copySlice(b.updateValues)
	c.conflictCols = copySlice(b.conflictCols)
	c.updateOnConflict = copySlice(b.updateOnConflict)
	if b.macros != nil {
		c.macros = make(map[string]BuilderMacro, len(b.macros))
		for k, v := range b.macros {
			c.macros[k] = v
		}
	}
	return &c
}

// Select
func (b *Builder) Select(columns ...string) *Builder {
	b.selects = columns
	return b
}

// SelectRaw
func (b *Builder) SelectRaw(expr string) *Builder {
	if b.err != nil {
		return b
	}
	b.selects = append(b.selects, expr)
	return b
}

// AddSelect
func (b *Builder) AddSelect(columns ...string) *Builder {
	if b.err != nil {
		return b
	}
	b.selects = append(b.selects, columns...)
	return b
}

// Distinct
func (b *Builder) Distinct() *Builder {
	b.distinct = true
	return b
}

// From
func (b *Builder) From(table string) *Builder {
	b.table = table
	return b
}

// As
func (b *Builder) As(alias string) *Builder {
	b.tableAlias = alias
	return b
}

// Where
func (b *Builder) Where(column, operator string, value interface{}) *Builder {
	if !validOperators[operator] {
		b.err = fmt.Errorf("ember: invalid operator %q", operator)
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		column: column, operator: operator, value: value, boolean: "AND",
	})
	return b
}

// OrWhere
func (b *Builder) OrWhere(column, operator string, value interface{}) *Builder {
	if !validOperators[operator] {
		b.err = fmt.Errorf("ember: invalid operator %q", operator)
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		column: column, operator: operator, value: value, boolean: "OR",
	})
	return b
}

// WhereRaw
func (b *Builder) WhereRaw(expr string, args ...interface{}) *Builder {
	if b.err != nil {
		return b
	}
	b.wheres = append(b.wheres, whereClause{raw: expr, rawArgs: args, boolean: "AND"})
	return b
}

// OrWhereRaw
func (b *Builder) OrWhereRaw(expr string, args ...interface{}) *Builder {
	if b.err != nil {
		return b
	}
	b.wheres = append(b.wheres, whereClause{raw: expr, rawArgs: args, boolean: "OR"})
	return b
}

// WhereGroup
func (b *Builder) WhereGroup(fn func(*Builder)) *Builder {
	nested := &Builder{dialect: b.dialect, db: b.db, tx: b.tx}
	if b.macros != nil {
		nested.macros = make(map[string]BuilderMacro, len(b.macros))
		for k, v := range b.macros {
			nested.macros[k] = v
		}
	}
	fn(nested)
	if nested.err != nil {
		b.err = nested.err
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		boolean: "AND",
		isGroup: true,
		group:   nested.wheres,
	})
	return b
}

// OrWhereGroup
func (b *Builder) OrWhereGroup(fn func(*Builder)) *Builder {
	nested := &Builder{dialect: b.dialect, db: b.db, tx: b.tx}
	if b.macros != nil {
		nested.macros = make(map[string]BuilderMacro, len(b.macros))
		for k, v := range b.macros {
			nested.macros[k] = v
		}
	}
	fn(nested)
	if nested.err != nil {
		b.err = nested.err
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		boolean: "OR",
		isGroup: true,
		group:   nested.wheres,
	})
	return b
}

// WhereIn
func (b *Builder) WhereIn(column string, values ...interface{}) *Builder {
	b.wheres = append(b.wheres, whereClause{
		column: column, boolean: "AND", isIn: true, inValues: values,
	})
	return b
}

// WhereNotIn
func (b *Builder) WhereNotIn(column string, values ...interface{}) *Builder {
	b.wheres = append(b.wheres, whereClause{
		column: column, boolean: "AND", isIn: true, notIn: true, inValues: values,
	})
	return b
}

// WhereNull
func (b *Builder) WhereNull(column string) *Builder {
	b.wheres = append(b.wheres, whereClause{column: column, boolean: "AND", isNull: true})
	return b
}

// WhereNotNull
func (b *Builder) WhereNotNull(column string) *Builder {
	b.wheres = append(b.wheres, whereClause{column: column, boolean: "AND", isNull: true, notNull: true})
	return b
}

// WhereBetween
func (b *Builder) WhereBetween(column string, low, high interface{}) *Builder {
	b.wheres = append(b.wheres, whereClause{
		column: column, boolean: "AND", isBetween: true, between: [2]interface{}{low, high},
	})
	return b
}

func quoteQualified(d Dialect, name string) string {
	parts := strings.Split(name, ".")
	for i, p := range parts {
		parts[i] = d.QuoteIdentifier(p)
	}
	return strings.Join(parts, ".")
}

// WhereColumn
func (b *Builder) WhereColumn(col1, operator, col2 string) *Builder {
	if !validOperators[operator] {
		b.err = fmt.Errorf("ember: invalid operator %q", operator)
		return b
	}
	expr := fmt.Sprintf("%s %s %s",
		quoteQualified(b.dialect, col1),
		operator,
		quoteQualified(b.dialect, col2),
	)
	b.wheres = append(b.wheres, whereClause{raw: expr, boolean: "AND"})
	return b
}

// Join
func (b *Builder) Join(table, first, operator, second string) *Builder {
	return b.addJoin("INNER", table, first, operator, second, "", nil)
}

// LeftJoin
func (b *Builder) LeftJoin(table, first, operator, second string) *Builder {
	return b.addJoin("LEFT", table, first, operator, second, "", nil)
}

// RightJoin
func (b *Builder) RightJoin(table, first, operator, second string) *Builder {
	return b.addJoin("RIGHT", table, first, operator, second, "", nil)
}

// CrossJoin
func (b *Builder) CrossJoin(table string) *Builder {
	b.joins = append(b.joins, joinClause{joinType: "CROSS", table: table})
	return b
}

// JoinRaw
func (b *Builder) JoinRaw(joinType, table, rawOn string, args ...interface{}) *Builder {
	return b.addJoin(joinType, table, "", "", "", rawOn, args)
}

func (b *Builder) addJoin(joinType, table, first, operator, second, rawOn string, rawOnArgs []interface{}) *Builder {
	b.joins = append(b.joins, joinClause{
		joinType:  joinType,
		table:     table,
		first:     first,
		operator:  operator,
		second:    second,
		rawOn:     rawOn,
		rawOnArgs: rawOnArgs,
	})
	return b
}

// GroupBy
func (b *Builder) GroupBy(columns ...string) *Builder {
	b.groupBys = append(b.groupBys, columns...)
	return b
}

// GroupByRaw
func (b *Builder) GroupByRaw(expr string) *Builder {
	b.groupBys = append(b.groupBys, expr)
	return b
}

// Having
func (b *Builder) Having(column, operator string, value interface{}) *Builder {
	if !validOperators[operator] {
		b.err = fmt.Errorf("ember: invalid operator %q", operator)
		return b
	}
	b.havings = append(b.havings, whereClause{
		column: column, operator: operator, value: value, boolean: "AND",
	})
	return b
}

// HavingRaw
func (b *Builder) HavingRaw(expr string, args ...interface{}) *Builder {
	b.havings = append(b.havings, whereClause{raw: expr, rawArgs: args, boolean: "AND"})
	return b
}

// OrderBy
func (b *Builder) OrderBy(column, direction string) *Builder {
	dir := strings.ToUpper(direction)
	if dir != "ASC" && dir != "DESC" {
		b.err = fmt.Errorf("ember: invalid order direction %q", direction)
		return b
	}
	b.orderBys = append(b.orderBys, orderByClause{
		raw: quoteQualified(b.dialect, column) + " " + dir,
	})
	return b
}

// OrderByRaw
func (b *Builder) OrderByRaw(expr string) *Builder {
	b.orderBys = append(b.orderBys, orderByClause{raw: expr})
	return b
}

// Latest
func (b *Builder) Latest(column ...string) *Builder {
	col := "created_at"
	if len(column) > 0 {
		col = column[0]
	}
	return b.OrderBy(col, "DESC")
}

// Oldest
func (b *Builder) Oldest(column ...string) *Builder {
	col := "created_at"
	if len(column) > 0 {
		col = column[0]
	}
	return b.OrderBy(col, "ASC")
}

// Limit
func (b *Builder) Limit(n int) *Builder {
	if n < 0 {
		n = 0
	}
	b.limit = &n
	return b
}

// Offset
func (b *Builder) Offset(n int) *Builder {
	if n < 0 {
		n = 0
	}
	b.offset = &n
	return b
}

// Take
func (b *Builder) Take(n int) *Builder {
	return b.Limit(n)
}

// Skip
func (b *Builder) Skip(n int) *Builder {
	return b.Offset(n)
}

// ForPage
func (b *Builder) ForPage(page, perPage int) *Builder {
	if b.err != nil {
		return b
	}
	if page < 1 {
		page = 1
	}
	c := b.clone()
	c.Limit(perPage).Offset((page - 1) * perPage)
	return c
}

// LockForUpdate
func (b *Builder) LockForUpdate() *Builder {
	b.lockMode = "FOR UPDATE"
	return b
}

// SharedLock
func (b *Builder) SharedLock() *Builder {
	b.lockMode = "FOR SHARE"
	return b
}

// ToSQL
func (b *Builder) ToSQL() (string, []interface{}) {
	if b.err != nil {
		return "", nil
	}
	var s strings.Builder
	var args []interface{}
	placeholderIdx := 1

	s.WriteString("SELECT ")
	if b.distinct {
		s.WriteString("DISTINCT ")
	}
	if len(b.selects) == 0 {
		s.WriteString("*")
	} else {
		s.WriteString(strings.Join(b.selects, ", "))
	}

	s.WriteString(" FROM ")
	s.WriteString(b.dialect.QuoteIdentifier(b.table))
	if b.tableAlias != "" {
		s.WriteString(" AS ")
		s.WriteString(b.dialect.QuoteIdentifier(b.tableAlias))
	}

	for _, j := range b.joins {
		if j.joinType == "CROSS" {
			s.WriteString(" CROSS JOIN ")
			s.WriteString(b.dialect.QuoteIdentifier(j.table))
			continue
		}
		s.WriteString(fmt.Sprintf(" %s JOIN %s", j.joinType, b.dialect.QuoteIdentifier(j.table)))
		if j.rawOn != "" {
			s.WriteString(" ON ")
			s.WriteString(b.rewritePlaceholders(j.rawOn, &placeholderIdx))
			args = append(args, j.rawOnArgs...)
		} else {
			s.WriteString(fmt.Sprintf(" ON %s %s %s",
				quoteQualified(b.dialect, j.first),
				j.operator,
				quoteQualified(b.dialect, j.second),
			))
		}
	}

	if len(b.wheres) > 0 {
		s.WriteString(" WHERE ")
		wherePart, whereArgs := b.compileWheres(b.wheres, &placeholderIdx)
		s.WriteString(wherePart)
		args = append(args, whereArgs...)
	}

	if len(b.groupBys) > 0 {
		s.WriteString(" GROUP BY ")
		quoted := make([]string, len(b.groupBys))
		for i, g := range b.groupBys {
			if isRawExpr(g) {
				quoted[i] = g
			} else {
				quoted[i] = quoteQualified(b.dialect, g)
			}
		}
		s.WriteString(strings.Join(quoted, ", "))
	}

	if len(b.havings) > 0 {
		s.WriteString(" HAVING ")
		havingPart, havingArgs := b.compileWheres(b.havings, &placeholderIdx)
		s.WriteString(havingPart)
		args = append(args, havingArgs...)
	}

	if len(b.orderBys) > 0 {
		parts := make([]string, len(b.orderBys))
		for i, o := range b.orderBys {
			parts[i] = o.raw
		}
		s.WriteString(" ORDER BY " + strings.Join(parts, ", "))
	}

	if b.limit != nil {
		s.WriteString(fmt.Sprintf(" LIMIT %d", *b.limit))
	}

	if b.offset != nil {
		s.WriteString(fmt.Sprintf(" OFFSET %d", *b.offset))
	}

	if b.lockMode != "" {
		s.WriteString(" " + b.lockMode)
	}

	return s.String(), args
}

func (b *Builder) compileWheres(wheres []whereClause, idx *int) (string, []interface{}) {
	var parts []string
	var args []interface{}

	for i, w := range wheres {
		prefix := ""
		if i > 0 {
			prefix = w.boolean + " "
		}

		switch {
		case w.raw != "":
			parts = append(parts, prefix+b.rewritePlaceholders(w.raw, idx))
			args = append(args, w.rawArgs...)

		case w.isGroup:
			inner, innerArgs := b.compileWheres(w.group, idx)
			parts = append(parts, prefix+"("+inner+")")
			args = append(args, innerArgs...)

		case w.isNull:
			if w.notNull {
				parts = append(parts, prefix+quoteQualified(b.dialect, w.column)+" IS NOT NULL")
			} else {
				parts = append(parts, prefix+quoteQualified(b.dialect, w.column)+" IS NULL")
			}

		case w.isIn:
			if len(w.inValues) == 0 {
				if w.notIn {
					parts = append(parts, prefix+"1=1")
				} else {
					parts = append(parts, prefix+"1=0")
				}
				break
			}
			placeholders := make([]string, len(w.inValues))
			for j := range w.inValues {
				placeholders[j] = b.dialect.Placeholder(*idx)
				*idx++
			}
			inOp := "IN"
			if w.notIn {
				inOp = "NOT IN"
			}
			parts = append(parts, fmt.Sprintf("%s%s %s (%s)",
				prefix,
				quoteQualified(b.dialect, w.column),
				inOp,
				strings.Join(placeholders, ", "),
			))
			args = append(args, w.inValues...)

		case w.isBetween:
			p1 := b.dialect.Placeholder(*idx)
			*idx++
			p2 := b.dialect.Placeholder(*idx)
			*idx++
			parts = append(parts, fmt.Sprintf("%s%s BETWEEN %s AND %s",
				prefix,
				quoteQualified(b.dialect, w.column),
				p1, p2,
			))
			args = append(args, w.between[0], w.between[1])

		default:
			ph := b.dialect.Placeholder(*idx)
			*idx++
			parts = append(parts, fmt.Sprintf("%s%s %s %s",
				prefix,
				quoteQualified(b.dialect, w.column),
				w.operator,
				ph,
			))
			args = append(args, w.value)
		}
	}

	return strings.Join(parts, " "), args
}

func (b *Builder) rewritePlaceholders(expr string, idx *int) string {
	if b.dialect.Name() != "postgres" {
		return expr
	}
	var out strings.Builder
	inString := false
	runes := []rune(expr)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				out.WriteRune(ch)
				out.WriteRune(ch)
				i++
				continue
			}
			inString = !inString
		}
		if ch == '?' && !inString {
			out.WriteString(b.dialect.Placeholder(*idx))
			*idx++
		} else {
			out.WriteRune(ch)
		}
	}
	return out.String()
}

func isRawExpr(s string) bool {
	if s == "" {
		return false
	}
	hasParen := strings.Contains(s, "(") && strings.Contains(s, ")")
	hasSpace := strings.Contains(s, " ")
	return hasParen || hasSpace
}

// Count
func (b *Builder) Count(ctx context.Context) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.aggregateInt64(ctx, "COUNT(*)")
}

// Sum
func (b *Builder) Sum(ctx context.Context, column string) (float64, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.aggregateFloat64(ctx, fmt.Sprintf("SUM(%s)", b.dialect.QuoteIdentifier(column)))
}

// Avg
func (b *Builder) Avg(ctx context.Context, column string) (float64, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.aggregateFloat64(ctx, fmt.Sprintf("AVG(%s)", b.dialect.QuoteIdentifier(column)))
}

// Min
func (b *Builder) Min(ctx context.Context, column string) (float64, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.aggregateFloat64(ctx, fmt.Sprintf("MIN(%s)", b.dialect.QuoteIdentifier(column)))
}

// Max
func (b *Builder) Max(ctx context.Context, column string) (float64, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.aggregateFloat64(ctx, fmt.Sprintf("MAX(%s)", b.dialect.QuoteIdentifier(column)))
}

func (b *Builder) aggregateInt64(ctx context.Context, expr string) (int64, error) {
	clone := b.cloneForAggregate(expr)
	query, args := clone.ToSQL()
	var result sql.NullInt64
	if err := b.queryRow(ctx, query, args).Scan(&result); err != nil {
		return 0, err
	}
	return result.Int64, nil
}

func (b *Builder) aggregateFloat64(ctx context.Context, expr string) (float64, error) {
	clone := b.cloneForAggregate(expr)
	query, args := clone.ToSQL()
	var result sql.NullFloat64
	if err := b.queryRow(ctx, query, args).Scan(&result); err != nil {
		return 0, err
	}
	return result.Float64, nil
}

func (b *Builder) cloneForAggregate(expr string) *Builder {
	c := *b
	c.selects = []string{expr}
	c.orderBys = nil
	c.limit = nil
	c.offset = nil
	c.lockMode = ""
	if c.macros != nil {
		c.macros = make(map[string]BuilderMacro)
		for k, v := range b.macros {
			c.macros[k] = v
		}
	}
	return &c
}

// Exists
func (b *Builder) Exists(ctx context.Context) (bool, error) {
	count, err := b.Count(ctx)
	return count > 0, err
}

// DoesntExist
func (b *Builder) DoesntExist(ctx context.Context) (bool, error) {
	count, err := b.Count(ctx)
	return count == 0, err
}

// Get
func (b *Builder) Get(ctx context.Context, dest interface{}) error {
	if b.err != nil {
		return b.err
	}
	query, args := b.ToSQL()
	rows, err := b.query(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanRows(rows, dest)
}

// First
func (b *Builder) First(ctx context.Context, dest interface{}) error {
	if b.err != nil {
		return b.err
	}
	clone := b.clone()
	one := 1
	clone.limit = &one
	query, args := clone.ToSQL()
	rows, err := b.query(ctx, query, args)
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
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("ember: First() dest must be a pointer to a struct")
	}
	elem := destVal.Elem()
	schema, err := ParseSchema(dest)
	if err != nil {
		return err
	}
	ptrs, timeFields := scanPointersByCol(cols, elem, schema, b.dialect.Name() != "postgres")
	if err := rows.Scan(ptrs...); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	scanTimeStrings(elem, ptrs, timeFields, schema)
	applyCastsOnRead(elem, schema)
	return rows.Err()
}

// Find
func (b *Builder) Find(ctx context.Context, id interface{}, dest interface{}) error {
	if b.err != nil {
		return b.err
	}
	clone := b.clone()
	pkCol := "id"
	if schema, err := ParseSchema(dest); err == nil && schema.PrimaryKey != nil {
		pkCol = schema.PrimaryKey.ColumnName
	}
	return clone.Where(pkCol, "=", id).First(ctx, dest)
}

// Pluck
func (b *Builder) Pluck(ctx context.Context, column string, dest interface{}) error {
	if b.err != nil {
		return b.err
	}
	clone := b.clone()
	clone.selects = []string{b.dialect.QuoteIdentifier(column)}
	query, args := clone.ToSQL()
	rows, err := b.query(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanColumn(rows, dest)
}

// Value
func (b *Builder) Value(ctx context.Context, column string, dest interface{}) error {
	if b.err != nil {
		return b.err
	}
	clone := b.clone()
	clone.selects = []string{b.dialect.QuoteIdentifier(column)}
	one := 1
	clone.limit = &one
	query, args := clone.ToSQL()
	row := b.queryRow(ctx, query, args)
	if err := row.Scan(dest); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Chunk
func (b *Builder) Chunk(ctx context.Context, size int, fn func(rows []map[string]interface{}) bool) error {
	if b.err != nil {
		return b.err
	}
	page := 0
	for {
		var batch []map[string]interface{}
		clone := b.clone()
		clone.limit = &size
		offset := page * size
		clone.offset = &offset

		if b.err != nil {
			return b.err
		}

		query, args := clone.ToSQL()
		rows, err := b.query(ctx, query, args)
		if err != nil {
			return err
		}

		if err := scanRowsToMaps(rows, &batch); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		if len(batch) == 0 {
			break
		}
		if !fn(batch) {
			break
		}
		ctx = WithStickyMaster(ctx)
		if len(batch) < size {
			break
		}
		page++
	}
	return nil
}

// Insert
func (b *Builder) Insert(ctx context.Context, data map[string]interface{}) (int64, error) {
	if data == nil {
		return 0, fmt.Errorf("ember: Insert() data must not be nil")
	}
	return b.InsertBatch(ctx, []map[string]interface{}{data})
}

// InsertBatch
func (b *Builder) InsertBatch(ctx context.Context, rows []map[string]interface{}) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	cols := make([]string, 0, len(rows[0]))
	for col := range rows[0] {
		cols = append(cols, col)
	}
	for i, row := range rows[1:] {
		if len(row) != len(cols) {
			return 0, fmt.Errorf("ember: InsertBatch: row %d has %d columns, expected %d", i+1, len(row), len(cols))
		}
		for col := range row {
			found := false
			for _, c := range cols {
				if c == col {
					found = true
					break
				}
			}
			if !found {
				return 0, fmt.Errorf("ember: InsertBatch: row %d has unexpected column %q", i+1, col)
			}
		}
		for _, c := range cols {
			if _, ok := row[c]; !ok {
				return 0, fmt.Errorf("ember: InsertBatch: row %d is missing column %q", i+1, c)
			}
		}
	}
	sort.Strings(cols)

	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = b.dialect.QuoteIdentifier(c)
	}

	var allArgs []interface{}
	var valuePlaceholders []string
	placeholderIdx := 1

	for _, row := range rows {
		rowPlaceholders := make([]string, len(cols))
		for i, col := range cols {
			rowPlaceholders[i] = b.dialect.Placeholder(placeholderIdx)
			placeholderIdx++
			allArgs = append(allArgs, row[col])
		}
		valuePlaceholders = append(valuePlaceholders,
			"("+strings.Join(rowPlaceholders, ", ")+")")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		b.dialect.QuoteIdentifier(b.table),
		strings.Join(quotedCols, ", "),
		strings.Join(valuePlaceholders, ", "),
	)

	result, err := b.exec(ctx, query, allArgs)
	if err != nil {
		return 0, err
	}
	if b.dialect.Name() == "postgres" {
		return 0, nil
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// InsertGetId
func (b *Builder) InsertGetId(ctx context.Context, data map[string]interface{}, idColumn string) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	if idColumn == "" {
		idColumn = "id"
	}

	cols := make([]string, 0, len(data))
	for col := range data {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	quotedCols := make([]string, len(cols))
	var args []interface{}
	var placeholders []string
	idx := 1
	for i, col := range cols {
		quotedCols[i] = b.dialect.QuoteIdentifier(col)
		placeholders = append(placeholders, b.dialect.Placeholder(idx))
		args = append(args, data[col])
		idx++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		b.dialect.QuoteIdentifier(b.table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)

	if b.dialect.SupportsReturning() {
		query += " RETURNING " + b.dialect.QuoteIdentifier(idColumn)
		var id int64
		row := b.queryRowWrite(ctx, query, args)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}

	result, err := b.exec(ctx, query, args)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Upsert
func (b *Builder) Upsert(ctx context.Context, data map[string]interface{}, conflictCols, updateCols []string) error {
	if b.err != nil {
		return b.err
	}
	cols := make([]string, 0, len(data))
	for col := range data {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	quotedCols := make([]string, len(cols))
	var args []interface{}
	var placeholders []string
	idx := 1
	for i, col := range cols {
		quotedCols[i] = b.dialect.QuoteIdentifier(col)
		placeholders = append(placeholders, b.dialect.Placeholder(idx))
		args = append(args, data[col])
		idx++
	}

	upsertSuffix, err := b.dialect.UpsertClause(conflictCols, updateCols)
	if err != nil {
		return err
	}

	alias := ""
	if b.dialect.Name() == "mysql" {
		alias = " AS new"
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)%s%s",
		b.dialect.QuoteIdentifier(b.table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
		alias,
		upsertSuffix,
	)

	_, err = b.exec(ctx, query, args)
	return err
}

// Update
func (b *Builder) Update(ctx context.Context, data map[string]interface{}) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(data) == 0 {
		return 0, nil
	}

	cols := make([]string, 0, len(data))
	for col := range data {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	var setClauses []string
	var args []interface{}
	idx := 1

	for _, col := range cols {
		setClauses = append(setClauses,
			fmt.Sprintf("%s = %s", b.dialect.QuoteIdentifier(col), b.dialect.Placeholder(idx)),
		)
		args = append(args, data[col])
		idx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s",
		b.dialect.QuoteIdentifier(b.table),
		strings.Join(setClauses, ", "),
	)

	if len(b.wheres) > 0 {
		wherePart, whereArgs := b.compileWheres(b.wheres, &idx)
		query += " WHERE " + wherePart
		args = append(args, whereArgs...)
	}

	result, err := b.exec(ctx, query, args)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// Increment
func (b *Builder) Increment(ctx context.Context, column string, amount ...int) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	n := 1
	if len(amount) > 0 {
		n = amount[0]
	}
	col := b.dialect.QuoteIdentifier(column)
	idx := 1
	query := fmt.Sprintf("UPDATE %s SET %s = %s + %s",
		b.dialect.QuoteIdentifier(b.table), col, col, b.dialect.Placeholder(idx))
	idx++
	args := []interface{}{n}
	if len(b.wheres) > 0 {
		wherePart, whereArgs := b.compileWheres(b.wheres, &idx)
		query += " WHERE " + wherePart
		args = append(args, whereArgs...)
	}
	result, err := b.exec(ctx, query, args)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// Decrement
func (b *Builder) Decrement(ctx context.Context, column string, amount ...int) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	n := 1
	if len(amount) > 0 {
		n = amount[0]
	}
	col := b.dialect.QuoteIdentifier(column)
	idx := 1
	query := fmt.Sprintf("UPDATE %s SET %s = %s - %s",
		b.dialect.QuoteIdentifier(b.table), col, col, b.dialect.Placeholder(idx))
	idx++
	args := []interface{}{n}
	if len(b.wheres) > 0 {
		wherePart, whereArgs := b.compileWheres(b.wheres, &idx)
		query += " WHERE " + wherePart
		args = append(args, whereArgs...)
	}
	result, err := b.exec(ctx, query, args)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// Delete
func (b *Builder) Delete(ctx context.Context) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(b.wheres) == 0 {
		return 0, fmt.Errorf("ember: Delete() requires at least one WHERE clause; use Truncate() to delete all rows")
	}
	idx := 1
	query := fmt.Sprintf("DELETE FROM %s", b.dialect.QuoteIdentifier(b.table))
	var args []interface{}
	if len(b.wheres) > 0 {
		wherePart, whereArgs := b.compileWheres(b.wheres, &idx)
		query += " WHERE " + wherePart
		args = append(args, whereArgs...)
	}
	result, err := b.exec(ctx, query, args)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// Truncate
func (b *Builder) Truncate(ctx context.Context) error {
	if b.err != nil {
		return b.err
	}
	if b.tx != nil {
		_, err := b.tx.ExecContext(ctx, "TRUNCATE TABLE "+b.dialect.QuoteIdentifier(b.table))
		return err
	}
	switch b.dialect.Name() {
	case "mysql":
		conn, err := b.db.Master().Conn(ctx)
		if err != nil {
			return fmt.Errorf("ember: get connection for truncate: %w", err)
		}
		defer conn.Close()
		restoreFK := true
		if _, err = conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
			restoreFK = false
			return fmt.Errorf("ember: truncate disable FK checks: %w", err)
		}
		defer func() {
			if restoreFK {
				conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")
			}
		}()
		query := fmt.Sprintf("TRUNCATE TABLE %s", b.dialect.QuoteIdentifier(b.table))
		_, err = conn.ExecContext(ctx, query)
		return err
	case "sqlite3":
		query := fmt.Sprintf("DELETE FROM %s", b.dialect.QuoteIdentifier(b.table))
		_, err := b.exec(ctx, query, nil)
		return err
	default:
		query := fmt.Sprintf("TRUNCATE TABLE %s", b.dialect.QuoteIdentifier(b.table))
		_, err := b.exec(ctx, query, nil)
		return err
	}
}

func (b *Builder) exec(ctx context.Context, query string, args []interface{}) (sql.Result, error) {
	if b.tx != nil {
		return b.tx.ExecContext(ctx, query, args...)
	}
	return b.db.ExecContext(ctx, query, args...)
}

func (b *Builder) query(ctx context.Context, query string, args []interface{}) (*sql.Rows, error) {
	if b.tx != nil {
		return b.tx.QueryContext(ctx, query, args...)
	}
	return b.db.QueryContext(ctx, query, args...)
}

func (b *Builder) queryRow(ctx context.Context, query string, args []interface{}) *queryRow {
	if b.tx != nil {
		return b.tx.QueryRowContext(ctx, query, args...)
	}
	return b.db.QueryRowContext(ctx, query, args...)
}

func (b *Builder) queryRowWrite(ctx context.Context, query string, args []interface{}) *queryRow {
	if b.tx != nil {
		return b.tx.QueryRowContext(ctx, query, args...)
	}
	row := b.db.master.QueryRowContext(ctx, query, args...)
	return &queryRow{row: row, query: query, args: args}
}

func scanRows(rows *sql.Rows, dest interface{}) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("ember: scanRows dest must be a pointer to a slice")
	}

	sliceVal := destVal.Elem()
	elemType := sliceVal.Type().Elem()
	isMap := elemType == reflect.TypeOf(map[string]interface{}{})
	elemTypeIsPtr := !isMap && elemType.Kind() == reflect.Ptr
	structType := elemType
	if elemTypeIsPtr {
		structType = structType.Elem()
	}

	for rows.Next() {
		scanValues := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range scanValues {
			valuePtrs[i] = &scanValues[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		if isMap {
			m := make(map[string]interface{}, len(cols))
			for i, col := range cols {
				m[col] = scanValues[i]
			}
			sliceVal.Set(reflect.Append(sliceVal, reflect.ValueOf(m)))
		} else {
			elem := reflect.New(structType).Elem()
			if err := mapColumnsToStruct(cols, scanValues, elem); err != nil {
				return err
			}
			if structType.Kind() == reflect.Struct {
				if schema, err := ParseSchema(elem.Addr().Interface()); err == nil {
					applyCastsOnRead(elem, schema)
				}
			}
			if elemTypeIsPtr {
				sliceVal.Set(reflect.Append(sliceVal, elem.Addr()))
			} else {
				sliceVal.Set(reflect.Append(sliceVal, elem))
			}
		}
	}
	return rows.Err()
}

func scanRowsToMaps(rows *sql.Rows, batch *[]map[string]interface{}) error {
	if rows == nil {
		return fmt.Errorf("ember: rows is nil")
	}
	if batch == nil {
		return ErrInvalidModel
	}
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		scanValues := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range scanValues {
			valuePtrs[i] = &scanValues[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}
		m := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			m[col] = scanValues[i]
		}
		*batch = append(*batch, m)
	}
	return rows.Err()
}

func scanColumn(rows *sql.Rows, dest interface{}) error {
	sliceVal := reflect.ValueOf(dest)
	if sliceVal.Kind() != reflect.Ptr || sliceVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("ember: Pluck() dest must be a pointer to a slice")
	}
	sl := sliceVal.Elem()
	elemType := sl.Type().Elem()

	for rows.Next() {
		elem := reflect.New(elemType).Elem()
		if err := rows.Scan(elem.Addr().Interface()); err != nil {
			return err
		}
		sl.Set(reflect.Append(sl, elem))
	}
	return rows.Err()
}

func mapColumnsToStruct(cols []string, vals []interface{}, structVal reflect.Value) error {
	t := structVal.Type()
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("ember: mapColumnsToStruct requires a struct, got %s", t.Kind())
	}
	colIdx := make(map[string]int, len(cols))
	for i, c := range cols {
		colIdx[c] = i
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		colName := toSnakeCase(field.Name)
		tag := field.Tag.Get("ember")
		if tag != "" {
			if col := tagValue(tag, "column"); col != "" {
				colName = col
			}
		}
		if idx, ok := colIdx[colName]; ok {
			fv := structVal.Field(i)
			if fv.CanSet() {
				v := reflect.ValueOf(vals[idx])
				if v.IsValid() {
					switch {
					case fv.Kind() == reflect.Ptr && v.Type().AssignableTo(fv.Type().Elem()):
						ptr := reflect.New(fv.Type().Elem())
						ptr.Elem().Set(v)
						fv.Set(ptr)
					case v.Type().AssignableTo(fv.Type()):
						fv.Set(v)
					case v.Type().ConvertibleTo(fv.Type()):
						fv.Set(v.Convert(fv.Type()))
					case fv.Type() == reflect.TypeOf(time.Time{}):
						if s, ok := vals[idx].(string); ok && s != "" {
							if t, err := parseTime(s); err == nil {
								fv.Set(reflect.ValueOf(t))
							}
						}
					case fv.Type() == reflect.TypeOf(&time.Time{}):
						if s, ok := vals[idx].(string); ok && s != "" {
							if t, err := parseTime(s); err == nil {
								fv.Set(reflect.ValueOf(&t))
							}
						}
					case fv.Kind() == reflect.String:
						fv.SetString(fmt.Sprint(v.Interface()))
					case fv.Kind() >= reflect.Int && fv.Kind() <= reflect.Int64:
						if s, ok := vals[idx].(string); ok {
							if i, err := strconv.ParseInt(s, 10, 64); err == nil {
								fv.SetInt(i)
							}
						}
					case fv.Kind() >= reflect.Float32 && fv.Kind() <= reflect.Float64:
						if s, ok := vals[idx].(string); ok {
							if f, err := strconv.ParseFloat(s, 64); err == nil {
								fv.SetFloat(f)
							}
						}
					case fv.Kind() == reflect.Bool:
						if s, ok := vals[idx].(string); ok {
							b := s == "1" || s == "true" || s == "t"
							fv.SetBool(b)
						}
					default:
						if s, ok := vals[idx].(string); ok && s != "" {
							target := reflect.New(fv.Type())
							if err := json.Unmarshal([]byte(s), target.Interface()); err == nil {
								fv.Set(target.Elem())
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func copySlice[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

func copyNestedSlice(src [][]interface{}) [][]interface{} {
	if src == nil {
		return nil
	}
	dst := make([][]interface{}, len(src))
	for i, s := range src {
		dst[i] = copySlice(s)
	}
	return dst
}

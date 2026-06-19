package ember

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// BeforeCreator is implemented by models to hook before creation.
type BeforeCreator interface {
	BeforeCreate(ctx context.Context) error
}

// AfterCreator is implemented by models to hook after creation.
type AfterCreator interface {
	AfterCreate(ctx context.Context) error
}

// BeforeSaver is implemented by models to hook before save.
type BeforeSaver interface {
	BeforeSave(ctx context.Context) error
}

// AfterSaver is implemented by models to hook after save.
type AfterSaver interface {
	AfterSave(ctx context.Context) error
}

// BeforeDeleter is implemented by models to hook before deletion.
type BeforeDeleter interface {
	BeforeDelete(ctx context.Context) error
}

// AfterDeleter is implemented by models to hook after deletion.
type AfterDeleter interface {
	AfterDelete(ctx context.Context) error
}

// BeforeUpdater is implemented by models to hook before update.
type BeforeUpdater interface {
	BeforeUpdate(ctx context.Context) error
}

// AfterUpdater is implemented by models to hook after update.
type AfterUpdater interface {
	AfterUpdate(ctx context.Context) error
}

// Tabler is implemented by models to provide a custom table name.
type Tabler interface {
	TableName() string
}

type relationLoad struct {
	name   string
	order  []string
	nested []relationLoad
}

// ModelDB provides a typed API for a single model type.
type ModelDB struct {
	db            *DB
	tx            *Tx
	relationLoads []relationLoad
}

// DB.Model
func (db *DB) Model() *ModelDB {
	return &ModelDB{db: db}
}

// Tx.Model
func (t *Tx) Model() *ModelDB {
	return &ModelDB{db: t.db, tx: t}
}

func (m *ModelDB) fireEvent(ctx context.Context, event ModelEvent, model interface{}) error {
	if m.db != nil {
		ed := m.db.EventDispatcher()
		if ed != nil {
			return ed.Fire(ctx, event, model)
		}
	}
	return nil
}

// ModelDB.Create
func (m *ModelDB) Create(ctx context.Context, model interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	return m.create(ctx, model, schema)
}

func (m *ModelDB) create(ctx context.Context, model interface{}, schema *ModelSchema) error {
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	elem := val.Elem()

	if err := m.fireEvent(ctx, EventSaving, model); err != nil {
		return err
	}
	if err := m.fireEvent(ctx, EventCreating, model); err != nil {
		return err
	}

	if h, ok := model.(BeforeSaver); ok {
		if err := h.BeforeSave(ctx); err != nil {
			return err
		}
	}
	if h, ok := model.(BeforeCreator); ok {
		if err := h.BeforeCreate(ctx); err != nil {
			return err
		}
	}

	data := make(map[string]interface{})
	pkWasZero := false
	for _, f := range schema.Fields {
		if f.IsPrimary && f.AutoIncr {
			fv := elem.Field(f.Index)
			if isZero(fv) {
				pkWasZero = true
				continue
			}
		}
		data[f.ColumnName] = elem.Field(f.Index).Interface()
	}

	now := time.Now()
	if schema.HasCreatedAt {
		data["created_at"] = now
	}
	if schema.HasUpdatedAt {
		data["updated_at"] = now
	}

	applyCastsOnWrite(data, schema)

	b := m.builder(schema.TableName)
	pkCol := "id"
	if schema.PrimaryKey != nil {
		pkCol = schema.PrimaryKey.ColumnName
	}
	id, err := b.InsertGetId(ctx, data, pkCol)
	if err != nil {
		return err
	}

	if schema.HasCreatedAt {
		setFieldByColumn(elem, schema, "created_at", now)
	}
	if schema.HasUpdatedAt {
		setFieldByColumn(elem, schema, "updated_at", now)
	}
	if schema.PrimaryKey != nil && schema.PrimaryKey.AutoIncr && pkWasZero {
		pkField := elem.Field(schema.PrimaryKey.Index)
		if pkField.CanSet() {
			idVal := reflect.ValueOf(id)
			if idVal.Type().ConvertibleTo(pkField.Type()) {
				pkField.Set(idVal.Convert(pkField.Type()))
			} else if pkField.Kind() == reflect.Ptr && idVal.Type().ConvertibleTo(pkField.Type().Elem()) {
				ptr := reflect.New(pkField.Type().Elem())
				ptr.Elem().Set(idVal.Convert(pkField.Type().Elem()))
				pkField.Set(ptr)
			} else {
				return fmt.Errorf("ember: cannot set generated primary key of type %s into field of type %s", idVal.Type(), pkField.Type())
			}
		}
	}

	if h, ok := model.(AfterCreator); ok {
		if err := h.AfterCreate(ctx); err != nil {
			return err
		}
	}
	if h, ok := model.(AfterSaver); ok {
		if err := h.AfterSave(ctx); err != nil {
			return err
		}
	}

	if err := m.fireEvent(ctx, EventCreated, model); err != nil {
		return err
	}
	if err := m.fireEvent(ctx, EventSaved, model); err != nil {
		return err
	}

	return nil
}

// ModelDB.Save
func (m *ModelDB) Save(ctx context.Context, model interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	elem := val.Elem()

	if schema.PrimaryKey == nil || isZero(elem.Field(schema.PrimaryKey.Index)) {
		return m.create(ctx, model, schema)
	}
	return m.updateModel(ctx, model, schema, elem, nil)
}

// ModelDB.Update
func (m *ModelDB) Update(ctx context.Context, model interface{}, changedCols ...string) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	elem := val.Elem()

	if schema.PrimaryKey == nil || isZero(elem.Field(schema.PrimaryKey.Index)) {
		return ErrMissingPrimaryKey
	}
	return m.updateModel(ctx, model, schema, elem, changedCols)
}

func (m *ModelDB) updateModel(ctx context.Context, model interface{}, schema *ModelSchema, elem reflect.Value, cols []string) error {
	if err := m.fireEvent(ctx, EventSaving, model); err != nil {
		return err
	}
	if err := m.fireEvent(ctx, EventUpdating, model); err != nil {
		return err
	}

	if h, ok := model.(BeforeSaver); ok {
		if err := h.BeforeSave(ctx); err != nil {
			return err
		}
	}
	if h, ok := model.(BeforeUpdater); ok {
		if err := h.BeforeUpdate(ctx); err != nil {
			return err
		}
	}

	now := time.Now()

	data := make(map[string]interface{})
	colSet := make(map[string]bool, len(cols))
	for _, c := range cols {
		colSet[c] = true
	}

	for _, f := range schema.Fields {
		if f.IsPrimary {
			continue
		}
		if len(cols) > 0 && !colSet[f.ColumnName] {
			continue
		}
		if f.ColumnName == "updated_at" && schema.HasUpdatedAt {
			data[f.ColumnName] = now
			continue
		}
		data[f.ColumnName] = elem.Field(f.Index).Interface()
	}

	applyCastsOnWrite(data, schema)

	pkVal := elem.Field(schema.PrimaryKey.Index).Interface()
	b := m.builder(schema.TableName).Where(schema.PrimaryKey.ColumnName, "=", pkVal)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}
	_, err := b.Update(ctx, data)
	if err != nil {
		return err
	}

	if schema.HasUpdatedAt {
		setFieldByColumn(elem, schema, "updated_at", now)
	}

	if h, ok := model.(AfterUpdater); ok {
		if err := h.AfterUpdate(ctx); err != nil {
			return err
		}
	}
	if h, ok := model.(AfterSaver); ok {
		if err := h.AfterSave(ctx); err != nil {
			return err
		}
	}

	if err := m.fireEvent(ctx, EventUpdated, model); err != nil {
		return err
	}
	if err := m.fireEvent(ctx, EventSaved, model); err != nil {
		return err
	}
	return nil
}

// ModelDB.Find
func (m *ModelDB) Find(ctx context.Context, model interface{}, id interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	pkCol := "id"
	if schema.PrimaryKey != nil {
		pkCol = schema.PrimaryKey.ColumnName
	}

	b := m.builder(schema.TableName).Where(pkCol, "=", id)

	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}

	if schema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}
	if err := m.scanFirst(ctx, b, model, schema); err != nil {
		return err
	}

	if len(m.relationLoads) > 0 {
		val := reflect.ValueOf(model)
		if err := m.EagerLoadSingle(ctx, val, schema, m.relationLoads); err != nil {
			return err
		}
	}
	return nil
}

// ModelDB.First
func (m *ModelDB) First(ctx context.Context, model interface{}, fn ...func(*Builder)) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	b := m.builder(schema.TableName)

	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}

	if schema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}
	for _, f := range fn {
		f(b)
	}
	if err := m.scanFirst(ctx, b, model, schema); err != nil {
		return err
	}

	if len(m.relationLoads) > 0 {
		val := reflect.ValueOf(model)
		if err := m.EagerLoadSingle(ctx, val, schema, m.relationLoads); err != nil {
			return err
		}
	}
	return nil
}

// ModelDB.All
func (m *ModelDB) All(ctx context.Context, dest interface{}, fn ...func(*Builder)) error {
	schema, err := parseSchemaFromSlice(dest)
	if err != nil {
		return err
	}
	b := m.builder(schema.TableName)

	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}

	if schema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}
	for _, f := range fn {
		f(b)
	}
	query, args := b.ToSQL()
	rows, err := m.queryRows(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close()
	if err := scanRowsIntoModels(rows, dest, schema, m.db.Dialect().Name() != "postgres"); err != nil {
		return err
	}

	if len(m.relationLoads) > 0 {
		sliceVal := reflect.ValueOf(dest).Elem()
		if err := m.EagerLoadSlice(ctx, sliceVal, schema, m.relationLoads); err != nil {
			return err
		}
	}
	return nil
}

// Where executes the query and returns all matching records.
func (m *ModelDB) Where(ctx context.Context, dest interface{}, fn func(*Builder)) error {
	return m.All(ctx, dest, fn)
}

// ModelDB.Delete
func (m *ModelDB) Delete(ctx context.Context, model interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	elem := val.Elem()

	if schema.PrimaryKey == nil || isZero(elem.Field(schema.PrimaryKey.Index)) {
		return ErrMissingPrimaryKey
	}

	if err := m.fireEvent(ctx, EventDeleting, model); err != nil {
		return err
	}

	if h, ok := model.(BeforeDeleter); ok {
		if err := h.BeforeDelete(ctx); err != nil {
			return err
		}
	}

	pkVal := elem.Field(schema.PrimaryKey.Index).Interface()
	b := m.builder(schema.TableName).Where(schema.PrimaryKey.ColumnName, "=", pkVal)

	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}

	if schema.HasSoftDelete {
		now := time.Now()
		if _, err := b.Update(ctx, map[string]interface{}{"deleted_at": now}); err != nil {
			return err
		}
		setFieldByColumn(elem, schema, "deleted_at", &now)
	} else {
		if _, err := b.Delete(ctx); err != nil {
			return err
		}
	}

	if h, ok := model.(AfterDeleter); ok {
		if err := h.AfterDelete(ctx); err != nil {
			return err
		}
	}

	if err := m.fireEvent(ctx, EventDeleted, model); err != nil {
		return err
	}
	return nil
}

// ModelDB.ForceDelete
func (m *ModelDB) ForceDelete(ctx context.Context, model interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	elem := val.Elem()
	if schema.PrimaryKey == nil || isZero(elem.Field(schema.PrimaryKey.Index)) {
		return ErrMissingPrimaryKey
	}
	if err := m.fireEvent(ctx, EventDeleting, model); err != nil {
		return err
	}

	if h, ok := model.(BeforeDeleter); ok {
		if err := h.BeforeDelete(ctx); err != nil {
			return err
		}
	}

	pkVal := elem.Field(schema.PrimaryKey.Index).Interface()
	b := m.builder(schema.TableName).Where(schema.PrimaryKey.ColumnName, "=", pkVal)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}
	_, err = b.Delete(ctx)
	if err != nil {
		return err
	}

	if h, ok := model.(AfterDeleter); ok {
		if err := h.AfterDelete(ctx); err != nil {
			return err
		}
	}

	if err := m.fireEvent(ctx, EventDeleted, model); err != nil {
		return err
	}

	return nil
}

// ModelDB.Restore
func (m *ModelDB) Restore(ctx context.Context, model interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	if !schema.HasSoftDelete {
		return fmt.Errorf("ember: model does not use soft deletes")
	}
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	elem := val.Elem()
	if schema.PrimaryKey == nil || isZero(elem.Field(schema.PrimaryKey.Index)) {
		return ErrMissingPrimaryKey
	}

	if err := m.fireEvent(ctx, EventSaving, model); err != nil {
		return err
	}
	if err := m.fireEvent(ctx, EventRestoring, model); err != nil {
		return err
	}

	if h, ok := model.(BeforeSaver); ok {
		if err := h.BeforeSave(ctx); err != nil {
			return err
		}
	}

	pkVal := elem.Field(schema.PrimaryKey.Index).Interface()
	b := m.builder(schema.TableName).Where(schema.PrimaryKey.ColumnName, "=", pkVal)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}
	_, err = b.Update(ctx, map[string]interface{}{"deleted_at": nil})
	if err != nil {
		return err
	}
	setFieldByColumn(elem, schema, "deleted_at", nil)

	if h, ok := model.(AfterSaver); ok {
		if err := h.AfterSave(ctx); err != nil {
			return err
		}
	}

	if err := m.fireEvent(ctx, EventRestored, model); err != nil {
		return err
	}
	return m.fireEvent(ctx, EventSaved, model)
}

// FillFromMap populates a model from a map using field names or db column tags.
func FillFromMap(model interface{}, data map[string]interface{}) error {
	schema, err := ParseSchema(model)
	if err != nil {
		return err
	}
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return ErrInvalidModel
	}
	elem := val.Elem()

	for col, v := range data {
		fs, ok := schema.FieldByCol[col]
		if !ok {
			continue
		}
		fv := elem.Field(fs.Index)
		if !fv.CanSet() {
			continue
		}
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			fv.Set(reflect.Zero(fv.Type()))
			continue
		}
		if rv.Type().AssignableTo(fv.Type()) {
			fv.Set(rv)
		} else if rv.Type().ConvertibleTo(fv.Type()) {
			fv.Set(rv.Convert(fv.Type()))
		} else if s, ok := v.(string); ok && fv.Kind() != reflect.String {
			target := reflect.New(fv.Type())
			if err := json.Unmarshal([]byte(s), target.Interface()); err == nil {
				fv.Set(target.Elem())
			}
		}
	}
	return nil
}

func scanPointersByCol(cols []string, elem reflect.Value, schema *ModelSchema, timeAsStrings bool) ([]interface{}, map[int]int) {
	ptrs := make([]interface{}, len(cols))
	timeFields := make(map[int]int)
	for i, col := range cols {
		if fs, ok := schema.FieldByCol[col]; ok {
			fv := elem.Field(fs.Index)
			if timeAsStrings && (fs.CastType == CastDate || fs.CastType == CastDatetime) &&
				fv.Type() == reflect.TypeOf(time.Time{}) {
				var s string
				ptrs[i] = &s
				timeFields[i] = fs.Index
			} else if timeAsStrings && (fs.CastType == CastDate || fs.CastType == CastDatetime) &&
				fv.Type() == reflect.TypeOf(&time.Time{}) {
				var s *string
				ptrs[i] = &s
				timeFields[i] = fs.Index
			} else if fs.CastType == CastJSON &&
				fv.Kind() != reflect.String && fv.Kind() != reflect.Slice {
				var s string
				ptrs[i] = &s
				timeFields[i] = fs.Index
			} else {
				ptrs[i] = fv.Addr().Interface()
			}
		} else {
			var discard interface{}
			ptrs[i] = &discard
		}
	}
	return ptrs, timeFields
}

func scanTimeStrings(elem reflect.Value, ptrs []interface{}, timeFields map[int]int, schema *ModelSchema) {
	structIdxToSchema := make(map[int]*FieldSchema, len(schema.Fields))
	for _, fs := range schema.Fields {
		structIdxToSchema[fs.Index] = fs
	}

	for colIdx, fieldIdx := range timeFields {
		fv := elem.Field(fieldIdx)
		if fs, ok := structIdxToSchema[fieldIdx]; ok && fs.CastType == CastJSON {
			sPtr, ok := ptrs[colIdx].(*string)
			if !ok || sPtr == nil || *sPtr == "" {
				continue
			}
			target := reflect.New(fv.Type())
			if err := json.Unmarshal([]byte(*sPtr), target.Interface()); err == nil {
				fv.Set(target.Elem())
			}
			continue
		}
		if fv.Kind() == reflect.Ptr {
			sPtr, ok := ptrs[colIdx].(**string)
			if !ok || sPtr == nil || *sPtr == nil || **sPtr == "" {
				continue
			}
			if t, err := parseTime(**sPtr); err == nil {
				fv.Set(reflect.ValueOf(&t))
			}
		} else {
			sPtr, ok := ptrs[colIdx].(*string)
			if !ok || sPtr == nil || *sPtr == "" {
				continue
			}
			if t, err := parseTime(*sPtr); err == nil {
				fv.Set(reflect.ValueOf(t))
			}
		}
	}
}

func (m *ModelDB) builder(table string) *Builder {
	return newBuilder(m.db, m.tx, table)
}

func (m *ModelDB) scanFirst(ctx context.Context, b *Builder, model interface{}, schema *ModelSchema) error {
	if b.err != nil {
		return b.err
	}
	one := 1
	b2 := m.builder(b.table)
	b2.selects = append([]string{}, b.selects...)
	b2.wheres = append([]whereClause{}, b.wheres...)
	b2.joins = append([]joinClause{}, b.joins...)
	b2.orderBys = append([]orderByClause{}, b.orderBys...)
	b2.groupBys = append([]string{}, b.groupBys...)
	b2.havings = append([]whereClause{}, b.havings...)
	b2.distinct = b.distinct
	b2.tableAlias = b.tableAlias
	b2.lockMode = b.lockMode
	if b.macros != nil {
		b2.macros = make(map[string]BuilderMacro, len(b.macros))
		for k, v := range b.macros {
			b2.macros[k] = v
		}
	}
	b2.limit = &one
	if b.offset != nil {
		o := *b.offset
		b2.offset = &o
	}
	query, args := b2.ToSQL()
	rows, err := m.queryRows(ctx, query, args)
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
	val := reflect.ValueOf(model)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return ErrInvalidModel
	}
	elem := val.Elem()
	ptrs, timeFields := scanPointersByCol(cols, elem, schema, b.dialect.Name() != "postgres")
	if err := rows.Scan(ptrs...); err != nil {
		return err
	}
	scanTimeStrings(elem, ptrs, timeFields, schema)
	applyCastsOnRead(elem, schema)
	return rows.Err()
}

func (m *ModelDB) queryRows(ctx context.Context, query string, args []interface{}) (*sql.Rows, error) {
	if m.tx != nil {
		return m.tx.QueryContext(ctx, query, args...)
	}
	return m.db.QueryContext(ctx, query, args...)
}

func (m *ModelDB) queryRow(ctx context.Context, query string, args []interface{}) *queryRow {
	if m.tx != nil {
		return m.tx.QueryRowContext(ctx, query, args...)
	}
	return m.db.QueryRowContext(ctx, query, args...)
}

func scanRowsIntoModels(rows *sql.Rows, dest interface{}, schema *ModelSchema, timeAsStrings bool) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("ember: dest must be a pointer to a slice")
	}
	sliceVal := destVal.Elem()
	elemType := sliceVal.Type().Elem()
	elemTypeIsPtr := elemType.Kind() == reflect.Ptr
	if elemTypeIsPtr {
		elemType = elemType.Elem()
	}

	for rows.Next() {
		elem := reflect.New(elemType).Elem()
		ptrs, timeFields := scanPointersByCol(cols, elem, schema, timeAsStrings)
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		scanTimeStrings(elem, ptrs, timeFields, schema)
		applyCastsOnRead(elem, schema)
		if elemTypeIsPtr {
			sliceVal.Set(reflect.Append(sliceVal, elem.Addr()))
		} else {
			sliceVal.Set(reflect.Append(sliceVal, elem))
		}
	}
	return rows.Err()
}

func parseSchemaFromSlice(dest interface{}) (*ModelSchema, error) {
	t := reflect.TypeOf(dest)
	if t == nil || t.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("ember: dest must be a pointer to a slice of structs")
	}
	t = t.Elem()
	if t.Kind() != reflect.Slice {
		return nil, fmt.Errorf("ember: dest must be a pointer to a slice of structs")
	}
	t = t.Elem()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	zero := reflect.New(t).Interface()
	return ParseSchema(zero)
}

func setFieldByColumn(elem reflect.Value, schema *ModelSchema, col string, value interface{}) {
	fs, ok := schema.FieldByCol[col]
	if !ok {
		return
	}
	fv := elem.Field(fs.Index)
	if !fv.CanSet() {
		return
	}
	if value == nil {
		fv.Set(reflect.Zero(fv.Type()))
		return
	}
	rv := reflect.ValueOf(value)
	if rv.Type().AssignableTo(fv.Type()) {
		fv.Set(rv)
	} else if rv.Type().ConvertibleTo(fv.Type()) {
		fv.Set(rv.Convert(fv.Type()))
	} else if fv.Kind() == reflect.Ptr && rv.Type().AssignableTo(fv.Type().Elem()) {
		ptr := reflect.New(fv.Type().Elem())
		ptr.Elem().Set(rv)
		fv.Set(ptr)
	} else if fv.Kind() == reflect.Ptr && rv.Type().ConvertibleTo(fv.Type().Elem()) {
		ptr := reflect.New(fv.Type().Elem())
		ptr.Elem().Set(rv.Convert(fv.Type().Elem()))
		fv.Set(ptr)
	}
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			return t.IsZero()
		}
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	default:
		return false
	}
}

// ModelDB.With
func (m *ModelDB) With(relations ...string) *ModelDB {
	clone := *m
	clone.relationLoads = parseRelationLoads(relations)
	return &clone
}

// HasMany defines a one-to-many relationship.
type HasMany struct {
	ForeignKey string
	LocalKey   string
}

// HasOne defines a one-to-one relationship.
type HasOne struct {
	ForeignKey string
	LocalKey   string
}

// BelongsTo defines an inverse one-to-one/one-to-many relationship.
type BelongsTo struct {
	ForeignKey string
	OwnerKey   string
}

// BelongsToMany defines a many-to-many relationship.
type BelongsToMany struct {
	PivotTable string
	ForeignKey string
	RelatedKey string
}

// Scope is a function that modifies a Builder query.
type Scope func(*Builder) *Builder

// ApplyScopes applies the given scopes to a builder in order.
func ApplyScopes(b *Builder, scopes ...Scope) *Builder {
	for _, s := range scopes {
		b = s(b)
	}
	return b
}

// ActiveScope is a Scope that filters for active records.
func ActiveScope(b *Builder) *Builder {
	return b.Where("status", "=", "active")
}

// SoftDeleteScope is a Scope that excludes soft-deleted records.
func SoftDeleteScope(b *Builder) *Builder {
	return b.WhereNull("deleted_at")
}

// WithTrashedScope returns a Scope that includes soft-deleted records.
func WithTrashedScope() Scope {
	return func(b *Builder) *Builder {
		var filtered []whereClause
		for _, w := range b.wheres {
			if w.isNull && w.column == "deleted_at" {
				continue
			}
			filtered = append(filtered, w)
		}
		c := b.clone()
		c.wheres = filtered
		return c
	}
}

// OnlyTrashedScope returns a Scope that only includes soft-deleted records.
func OnlyTrashedScope() Scope {
	return func(b *Builder) *Builder {
		var filtered []whereClause
		for _, w := range b.wheres {
			if w.isNull && w.column == "deleted_at" {
				continue
			}
			filtered = append(filtered, w)
		}
		c := b.clone()
		c.wheres = filtered
		return c.WhereNotNull("deleted_at")
	}
}

func parseRelationLoads(relations []string) []relationLoad {
	var result []relationLoad
	seen := make(map[string]int)
	for _, rel := range relations {
		parts := strings.SplitN(rel, ".", 2)
		cleanName, order := parseOrderFromRelation(parts[0])
		if idx, ok := seen[cleanName]; ok {
			if len(parts) == 2 {
				result[idx].nested = append(result[idx].nested, parseRelationLoads([]string{parts[1]})...)
			}
			if len(order) > 0 {
				result[idx].order = append(result[idx].order, order...)
			}
			continue
		}
		rl := relationLoad{name: cleanName, order: order}
		if len(parts) == 2 {
			rl.nested = parseRelationLoads([]string{parts[1]})
		}
		seen[cleanName] = len(result)
		result = append(result, rl)
	}
	return result
}

func parseOrderFromRelation(name string) (string, []string) {
	const prefix = "OrderBy("
	idx := strings.Index(name, prefix)
	if idx < 0 {
		return name, nil
	}
	rest := name[idx+len(prefix):]
	closeIdx := strings.Index(rest, ")")
	if closeIdx < 0 {
		return name, nil
	}
	orderRaw := rest[:closeIdx]

	cleanName := name[:idx]

	var orders []string
	for _, part := range strings.Split(orderRaw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		orders = append(orders, part)
	}
	return cleanName, orders
}

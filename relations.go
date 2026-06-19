package ember

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

func safeKey(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "<nil>"
		}
		v = rv.Elem().Interface()
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// EagerLoadSlice eagerly loads relations for a slice of parent models.
func (m *ModelDB) EagerLoadSlice(ctx context.Context, parents reflect.Value, parentSchema *ModelSchema, rls []relationLoad) error {
	for _, rl := range rls {
		rel, ok := parentSchema.Relations[toGoName(rl.name)]
		if !ok {
			rel = findRelationBySnake(parentSchema, rl.name)
			if rel == nil {
				return fmt.Errorf("ember: relation %q not found on type %s", rl.name, parentSchema.GoType.Name())
			}
		}
		if err := m.eagerLoadOne(ctx, parents, parentSchema, rel, rl); err != nil {
			return err
		}
	}
	return nil
}

// EagerLoadSingle eagerly loads relations for a single parent model.
func (m *ModelDB) EagerLoadSingle(ctx context.Context, parent reflect.Value, parentSchema *ModelSchema, rls []relationLoad) error {
	t := parent.Type()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	slice := reflect.MakeSlice(reflect.SliceOf(t), 0, 1)
	if parent.Kind() == reflect.Ptr {
		slice = reflect.Append(slice, parent.Elem())
	} else {
		slice = reflect.Append(slice, parent)
	}
	return m.EagerLoadSlice(ctx, slice, parentSchema, rls)
}

func (m *ModelDB) eagerLoadOne(ctx context.Context, parents reflect.Value, parentSchema *ModelSchema, rel *RelationDef, rl relationLoad) error {
	switch rel.Type {
	case HasManyRelation:
		return m.eagerLoadHasMany(ctx, parents, parentSchema, rel, rl.nested, rl.order)
	case HasOneRelation:
		return m.eagerLoadHasOne(ctx, parents, parentSchema, rel, rl.nested, rl.order)
	case BelongsToRelation:
		return m.eagerLoadBelongsTo(ctx, parents, parentSchema, rel, rl.nested)
	case BelongsToManyRelation:
		return m.eagerLoadBelongsToMany(ctx, parents, parentSchema, rel, rl.nested, rl.order)
	default:
		return nil
	}
}

func (m *ModelDB) eagerLoadHasMany(ctx context.Context, parents reflect.Value, parentSchema *ModelSchema, rel *RelationDef, nested []relationLoad, order []string) error {
	if parents.Len() == 0 {
		return nil
	}

	localKey := rel.LocalKey
	if localKey == "" {
		if parentSchema.PrimaryKey != nil {
			localKey = parentSchema.PrimaryKey.ColumnName
		} else {
			localKey = "id"
		}
	}

	foreignKey := rel.ForeignKey
	if foreignKey == "" {
		foreignKey = toSnakeCase(parentSchema.GoType.Name()) + "_id"
	}

	localKeys := collectFieldValuesByCol(parents, parentSchema, localKey)
	if len(localKeys) == 0 {
		return nil
	}

	childSchema, err := ParseSchema(reflect.New(rel.RelatedType).Interface())
	if err != nil {
		return err
	}

	b := m.builder(childSchema.TableName)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(childSchema.GoType)
		b = ApplyScopes(b, scopes...)
	}
	if childSchema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}
	b = b.WhereIn(foreignKey, localKeys...)
	for _, o := range order {
		parts := strings.SplitN(o, " ", 2)
		if len(parts) == 2 {
			b = b.OrderBy(parts[0], parts[1])
		}
	}

	query, args := b.ToSQL()
	rows, err := m.queryRows(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close()

	sliceType := reflect.SliceOf(rel.RelatedType)
	allChildren := reflect.New(sliceType).Interface()
	if err := scanRowsIntoModels(rows, allChildren, childSchema, m.db.Dialect().Name() != "postgres"); err != nil {
		return err
	}
	childrenVal := reflect.ValueOf(allChildren).Elem()
	return m.eagerLoadHasManyPopulate(ctx, parents, parentSchema, rel, b, localKey, foreignKey, childrenVal, childSchema, nested)
}

func (m *ModelDB) eagerLoadHasManyPopulate(ctx context.Context, parents reflect.Value, parentSchema *ModelSchema, rel *RelationDef, b *Builder, localKey, foreignKey string, childrenVal reflect.Value, childSchema *ModelSchema, nested []relationLoad) error {
	grouped := make(map[string][]int)
	for i := 0; i < childrenVal.Len(); i++ {
		child := childrenVal.Index(i)
		fkField, ok := childSchema.FieldByCol[foreignKey]
		if !ok {
			continue
		}
		key := child.Field(fkField.Index).Interface()
		k := safeKey(key)
		grouped[k] = append(grouped[k], i)
	}

	for i := 0; i < parents.Len(); i++ {
		parent := parents.Index(i)
		if parent.Kind() == reflect.Ptr {
			parent = parent.Elem()
		}
		lkField, ok := parentSchema.FieldByCol[localKey]
		if !ok {
			return fmt.Errorf("ember: local key column %q not found in %s", localKey, b.table)
		}
		key := parent.Field(lkField.Index).Interface()
		indices := grouped[safeKey(key)]

		relField := parent.FieldByName(rel.GoName)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}

		childSlice := reflect.MakeSlice(relField.Type(), len(indices), len(indices))
		for j, idx := range indices {
			child := childrenVal.Index(idx)
			targetType := childSlice.Type().Elem()
			if child.Type() != targetType {
				if child.Type().ConvertibleTo(targetType) {
					childSlice.Index(j).Set(child.Convert(targetType))
				} else if child.CanAddr() && child.Addr().Type().ConvertibleTo(targetType) {
					childSlice.Index(j).Set(child.Addr().Convert(targetType))
				}
			} else {
				childSlice.Index(j).Set(child)
			}
		}
		relField.Set(childSlice)
	}

	if len(nested) > 0 && childrenVal.Len() > 0 {
		childSchema, err := ParseSchema(childrenVal.Index(0).Addr().Interface())
		if err != nil {
			return err
		}
		if err := m.EagerLoadSlice(ctx, childrenVal, childSchema, nested); err != nil {
			return err
		}
	}

	return nil
}

func (m *ModelDB) eagerLoadHasOne(ctx context.Context, parents reflect.Value, parentSchema *ModelSchema, rel *RelationDef, nested []relationLoad, order []string) error {
	if parents.Len() == 0 {
		return nil
	}

	localKey := rel.LocalKey
	if localKey == "" {
		if parentSchema.PrimaryKey != nil {
			localKey = parentSchema.PrimaryKey.ColumnName
		} else {
			localKey = "id"
		}
	}

	foreignKey := rel.ForeignKey
	if foreignKey == "" {
		foreignKey = toSnakeCase(parentSchema.GoType.Name()) + "_id"
	}

	localKeys := collectFieldValuesByCol(parents, parentSchema, localKey)
	if len(localKeys) == 0 {
		return nil
	}

	childSchema, err := ParseSchema(reflect.New(rel.RelatedType).Interface())
	if err != nil {
		return err
	}

	b := m.builder(childSchema.TableName)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(childSchema.GoType)
		b = ApplyScopes(b, scopes...)
	}
	if childSchema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}
	b = b.WhereIn(foreignKey, localKeys...)
	for _, o := range order {
		parts := strings.SplitN(o, " ", 2)
		if len(parts) == 2 {
			b = b.OrderBy(parts[0], parts[1])
		}
	}

	query, args := b.ToSQL()
	rows, err := m.queryRows(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close()

	sliceType := reflect.SliceOf(rel.RelatedType)
	allChildren := reflect.New(sliceType).Interface()
	if err := scanRowsIntoModels(rows, allChildren, childSchema, m.db.Dialect().Name() != "postgres"); err != nil {
		return err
	}
	childrenVal := reflect.ValueOf(allChildren).Elem()

	grouped := make(map[string]int)
	for i := 0; i < childrenVal.Len(); i++ {
		child := childrenVal.Index(i)
		fkField, ok := childSchema.FieldByCol[foreignKey]
		if !ok {
			continue
		}
		key := child.Field(fkField.Index).Interface()
		k := safeKey(key)
		if _, exists := grouped[k]; !exists {
			grouped[k] = i
		}
	}

	for i := 0; i < parents.Len(); i++ {
		parent := parents.Index(i)
		if parent.Kind() == reflect.Ptr {
			parent = parent.Elem()
		}
		lkField, ok := parentSchema.FieldByCol[localKey]
		if !ok {
			continue
		}
		key := parent.Field(lkField.Index).Interface()
		idx, found := grouped[safeKey(key)]
		if !found {
			continue
		}

		relField := parent.FieldByName(rel.GoName)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}

		childPtr := reflect.New(rel.RelatedType)
		childPtr.Elem().Set(childrenVal.Index(idx))
		if relField.Kind() == reflect.Ptr {
			relField.Set(childPtr)
		} else if childPtr.Elem().Type() == relField.Type() {
			relField.Set(childPtr.Elem())
		} else if childPtr.Elem().Type().ConvertibleTo(relField.Type()) {
			relField.Set(childPtr.Elem().Convert(relField.Type()))
		}
	}

	if len(nested) > 0 && childrenVal.Len() > 0 {
		childSchema, err := ParseSchema(childrenVal.Index(0).Addr().Interface())
		if err != nil {
			return err
		}
		if err := m.EagerLoadSlice(ctx, childrenVal, childSchema, nested); err != nil {
			return err
		}
	}

	return nil
}

func (m *ModelDB) eagerLoadBelongsTo(ctx context.Context, children reflect.Value, childSchema *ModelSchema, rel *RelationDef, nested []relationLoad) error {
	if children.Len() == 0 {
		return nil
	}

	foreignKey := rel.ForeignKey
	if foreignKey == "" {
		foreignKey = toSnakeCase(rel.RelatedType.Name()) + "_id"
	}

	ownerKey := rel.OwnerKey
	if ownerKey == "" {
		parentSchema, err := ParseSchema(reflect.New(rel.RelatedType).Interface())
		if err != nil {
			return err
		}
		if parentSchema.PrimaryKey != nil {
			ownerKey = parentSchema.PrimaryKey.ColumnName
		} else {
			ownerKey = "id"
		}
	}

	fkValues := collectFieldValuesByCol(children, childSchema, foreignKey)
	if len(fkValues) == 0 {
		return nil
	}
	fkValues = deduplicate(fkValues)

	parentSchema, err := ParseSchema(reflect.New(rel.RelatedType).Interface())
	if err != nil {
		return err
	}

	b := m.builder(parentSchema.TableName)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(parentSchema.GoType)
		b = ApplyScopes(b, scopes...)
	}
	if parentSchema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}
	b = b.WhereIn(ownerKey, fkValues...)

	query, args := b.ToSQL()
	rows, err := m.queryRows(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close()

	sliceType := reflect.SliceOf(rel.RelatedType)
	allParents := reflect.New(sliceType).Interface()
	if err := scanRowsIntoModels(rows, allParents, parentSchema, m.db.Dialect().Name() != "postgres"); err != nil {
		return err
	}
	parentsVal := reflect.ValueOf(allParents).Elem()

	parentIndex := make(map[string]int)
	for i := 0; i < parentsVal.Len(); i++ {
		p := parentsVal.Index(i)
		if p.Kind() == reflect.Ptr {
			p = p.Elem()
		}
		okField, ok := parentSchema.FieldByCol[ownerKey]
		if !ok {
			continue
		}
		key := p.Field(okField.Index).Interface()
		parentIndex[safeKey(key)] = i
	}

	for i := 0; i < children.Len(); i++ {
		child := children.Index(i)
		if child.Kind() == reflect.Ptr {
			child = child.Elem()
		}
		fkField, ok := childSchema.FieldByCol[foreignKey]
		if !ok {
			continue
		}
		key := child.Field(fkField.Index).Interface()
		idx, found := parentIndex[safeKey(key)]
		if !found {
			continue
		}

		relField := child.FieldByName(rel.GoName)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}

		parentPtr := reflect.New(rel.RelatedType)
		parentPtr.Elem().Set(parentsVal.Index(idx))
		if relField.Kind() == reflect.Ptr {
			relField.Set(parentPtr)
		} else if parentPtr.Elem().Type() == relField.Type() {
			relField.Set(parentPtr.Elem())
		} else if parentPtr.Elem().Type().ConvertibleTo(relField.Type()) {
			relField.Set(parentPtr.Elem().Convert(relField.Type()))
		}
	}

	if len(nested) > 0 && parentsVal.Len() > 0 {
		parentSchema, err := ParseSchema(parentsVal.Index(0).Addr().Interface())
		if err != nil {
			return err
		}
		if err := m.EagerLoadSlice(ctx, parentsVal, parentSchema, nested); err != nil {
			return err
		}
	}

	return nil
}

func (m *ModelDB) eagerLoadBelongsToMany(ctx context.Context, parents reflect.Value, parentSchema *ModelSchema, rel *RelationDef, nested []relationLoad, order []string) error {
	if parents.Len() == 0 {
		return nil
	}

	localKey := rel.LocalKey
	if localKey == "" {
		if parentSchema.PrimaryKey != nil {
			localKey = parentSchema.PrimaryKey.ColumnName
		} else {
			localKey = "id"
		}
	}

	pivotTable := rel.PivotTable
	if pivotTable == "" {
		pivotTable = toSnakeCase(parentSchema.GoType.Name()) + "_" + toSnakeCase(rel.RelatedType.Name())
	}

	pivotFK := rel.PivotFK
	if pivotFK == "" {
		pivotFK = toSnakeCase(parentSchema.GoType.Name()) + "_id"
	}

	pivotRK := rel.PivotRK
	if pivotRK == "" {
		pivotRK = toSnakeCase(rel.RelatedType.Name()) + "_id"
	}

	relatedPK := rel.OwnerKey
	if relatedPK == "" {
		relatedPK = "id"
	}

	localKeys := collectFieldValuesByCol(parents, parentSchema, localKey)
	if len(localKeys) == 0 {
		return nil
	}

	pivotB := m.builder(pivotTable).Select(pivotFK, pivotRK).WhereIn(pivotFK, localKeys...)
	pivotQuery, pivotArgs := pivotB.ToSQL()
	pivotRows, err := m.queryRows(ctx, pivotQuery, pivotArgs)
	if err != nil {
		return err
	}
	defer pivotRows.Close()

	type pivotRow struct {
		FK interface{}
		RK interface{}
	}
	var pivotData []pivotRow
	pivotCols, err := pivotRows.Columns()
	if err != nil {
		return err
	}
	for pivotRows.Next() {
		vals := make([]interface{}, len(pivotCols))
		ptrs := make([]interface{}, len(pivotCols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := pivotRows.Scan(ptrs...); err != nil {
			return err
		}
		pr := pivotRow{}
		for i, col := range pivotCols {
			if col == pivotFK {
				pr.FK = vals[i]
			}
			if col == pivotRK {
				pr.RK = vals[i]
			}
		}
		pivotData = append(pivotData, pr)
	}
	if err := pivotRows.Err(); err != nil {
		return err
	}

	if len(pivotData) == 0 {
		return nil
	}

	relatedIDs := make([]interface{}, 0, len(pivotData))
	seen := make(map[string]bool)
	for _, pr := range pivotData {
		k := safeKey(pr.RK)
		if !seen[k] {
			seen[k] = true
			relatedIDs = append(relatedIDs, pr.RK)
		}
	}

	relatedSchema, err := ParseSchema(reflect.New(rel.RelatedType).Interface())
	if err != nil {
		return err
	}

	relatedB := m.builder(relatedSchema.TableName)
	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(relatedSchema.GoType)
		relatedB = ApplyScopes(relatedB, scopes...)
	}
	if relatedSchema.HasSoftDelete {
		relatedB = relatedB.WhereNull("deleted_at")
	}
	relatedB = relatedB.WhereIn(relatedPK, relatedIDs...)
	for _, o := range order {
		parts := strings.SplitN(o, " ", 2)
		if len(parts) == 2 {
			relatedB = relatedB.OrderBy(parts[0], parts[1])
		}
	}
	relatedQuery, relatedArgs := relatedB.ToSQL()
	relatedRows, err := m.queryRows(ctx, relatedQuery, relatedArgs)
	if err != nil {
		return err
	}
	defer relatedRows.Close()

	sliceType := reflect.SliceOf(rel.RelatedType)
	allRelated := reflect.New(sliceType).Interface()
	if err := scanRowsIntoModels(relatedRows, allRelated, relatedSchema, m.db.Dialect().Name() != "postgres"); err != nil {
		return err
	}
	relatedVal := reflect.ValueOf(allRelated).Elem()

	relatedIndex := make(map[string]int)
	for i := 0; i < relatedVal.Len(); i++ {
		r := relatedVal.Index(i)
		if r.Kind() == reflect.Ptr {
			r = r.Elem()
		}
		pkField, ok := relatedSchema.FieldByCol[relatedPK]
		if !ok {
			continue
		}
		key := r.Field(pkField.Index).Interface()
		relatedIndex[safeKey(key)] = i
	}

	parentToRelated := make(map[string][]int)
	for _, pr := range pivotData {
		if ri, ok := relatedIndex[safeKey(pr.RK)]; ok {
			k := safeKey(pr.FK)
			parentToRelated[k] = append(parentToRelated[k], ri)
		}
	}

	for i := 0; i < parents.Len(); i++ {
		parent := parents.Index(i)
		if parent.Kind() == reflect.Ptr {
			parent = parent.Elem()
		}
		lkField, ok := parentSchema.FieldByCol[localKey]
		if !ok {
			continue
		}
		key := parent.Field(lkField.Index).Interface()

		relField := parent.FieldByName(rel.GoName)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}

		indices := parentToRelated[safeKey(key)]
		childSlice := reflect.MakeSlice(relField.Type(), len(indices), len(indices))
		for j, idx := range indices {
			child := relatedVal.Index(idx)
			if child.Type() != childSlice.Type().Elem() {
				if child.Type().ConvertibleTo(childSlice.Type().Elem()) {
					childSlice.Index(j).Set(child.Convert(childSlice.Type().Elem()))
				}
			} else {
				childSlice.Index(j).Set(child)
			}
		}
		relField.Set(childSlice)
	}

	if len(nested) > 0 && relatedVal.Len() > 0 {
		relatedSchema, err := ParseSchema(relatedVal.Index(0).Addr().Interface())
		if err != nil {
			return err
		}
		if err := m.EagerLoadSlice(ctx, relatedVal, relatedSchema, nested); err != nil {
			return err
		}
	}

	return nil
}

// LoadHasMany loads a HasMany relationship for a parent model.
func (m *ModelDB) LoadHasMany(ctx context.Context, parent interface{}, rel HasMany, dest interface{}) error {
	parentSchema, err := ParseSchema(parent)
	if err != nil {
		return err
	}
	childSchema, err := parseSchemaFromSlice(dest)
	if err != nil {
		return err
	}

	localKey := rel.LocalKey
	if localKey == "" && parentSchema.PrimaryKey != nil {
		localKey = parentSchema.PrimaryKey.ColumnName
	}
	foreignKey := rel.ForeignKey
	if foreignKey == "" {
		foreignKey = toSnakeCase(parentSchema.GoType.Name()) + "_id"
	}

	pVal := reflect.ValueOf(parent)
	if pVal.Kind() == reflect.Ptr {
		pVal = pVal.Elem()
	}
	lkField, ok := parentSchema.FieldByCol[localKey]
	if !ok {
		return fmt.Errorf("ember: LoadHasMany: field %q not found in %s", localKey, parentSchema.GoType.Name())
	}
	lkVal := pVal.Field(lkField.Index).Interface()

	return m.All(ctx, dest, func(b *Builder) {
		b.Where(foreignKey, "=", lkVal)
		if childSchema.HasSoftDelete {
			b.WhereNull("deleted_at")
		}
	})
}

// LoadHasOne loads a HasOne relationship for a parent model.
func (m *ModelDB) LoadHasOne(ctx context.Context, parent interface{}, rel HasOne, dest interface{}) error {
	parentSchema, err := ParseSchema(parent)
	if err != nil {
		return err
	}

	localKey := rel.LocalKey
	if localKey == "" && parentSchema.PrimaryKey != nil {
		localKey = parentSchema.PrimaryKey.ColumnName
	}
	foreignKey := rel.ForeignKey
	if foreignKey == "" {
		foreignKey = toSnakeCase(parentSchema.GoType.Name()) + "_id"
	}

	pVal := reflect.ValueOf(parent)
	if pVal.Kind() == reflect.Ptr {
		pVal = pVal.Elem()
	}
	lkField, ok := parentSchema.FieldByCol[localKey]
	if !ok {
		return fmt.Errorf("ember: LoadHasOne: field %q not found in %s", localKey, parentSchema.GoType.Name())
	}
	lkVal := pVal.Field(lkField.Index).Interface()

	childSchema, err := ParseSchema(dest)
	if err != nil {
		return err
	}

	cloneB := m.builder(childSchema.TableName).Where(foreignKey, "=", lkVal)
	if childSchema.HasSoftDelete {
		cloneB = cloneB.WhereNull("deleted_at")
	}
	return m.scanFirst(ctx, cloneB, dest, childSchema)
}

// LoadBelongsTo loads a BelongsTo relationship for a child model.
func (m *ModelDB) LoadBelongsTo(ctx context.Context, child interface{}, rel BelongsTo, dest interface{}) error {
	childSchema, err := ParseSchema(child)
	if err != nil {
		return err
	}

	foreignKey := rel.ForeignKey
	ownerKey := rel.OwnerKey
	if ownerKey == "" {
		destSchema, err := ParseSchema(dest)
		if err != nil {
			return err
		}
		if destSchema.PrimaryKey != nil {
			ownerKey = destSchema.PrimaryKey.ColumnName
		} else {
			ownerKey = "id"
		}
	}

	cVal := reflect.ValueOf(child)
	if cVal.Kind() == reflect.Ptr {
		cVal = cVal.Elem()
	}
	fkField, ok := childSchema.FieldByCol[foreignKey]
	if !ok {
		return fmt.Errorf("ember: LoadBelongsTo: field %q not found in %s", foreignKey, childSchema.GoType.Name())
	}
	fkVal := cVal.Field(fkField.Index).Interface()

	destSchema, err := ParseSchema(dest)
	if err != nil {
		return err
	}

	cloneB := m.builder(destSchema.TableName).Where(ownerKey, "=", fkVal)
	if destSchema.HasSoftDelete {
		cloneB = cloneB.WhereNull("deleted_at")
	}
	return m.scanFirst(ctx, cloneB, dest, destSchema)
}

// LoadBelongsToMany loads a BelongsToMany relationship for a parent model.
func (m *ModelDB) LoadBelongsToMany(ctx context.Context, parent interface{}, rel BelongsToMany, dest interface{}) error {
	parentSchema, err := ParseSchema(parent)
	if err != nil {
		return err
	}
	relatedSchema, err := parseSchemaFromSlice(dest)
	if err != nil {
		return err
	}

	foreignKey := rel.ForeignKey
	if foreignKey == "" {
		foreignKey = toSnakeCase(parentSchema.GoType.Name()) + "_id"
	}
	relatedKey := rel.RelatedKey
	if relatedKey == "" {
		relatedKey = toSnakeCase(relatedSchema.GoType.Name()) + "_id"
	}
	pivotTable := rel.PivotTable
	if pivotTable == "" {
		pivotTable = toSnakeCase(parentSchema.GoType.Name()) + "_" + toSnakeCase(relatedSchema.GoType.Name())
	}
	localKey := "id"
	if parentSchema.PrimaryKey != nil {
		localKey = parentSchema.PrimaryKey.ColumnName
	}

	pVal := reflect.ValueOf(parent)
	if pVal.Kind() == reflect.Ptr {
		pVal = pVal.Elem()
	}
	lkField, ok := parentSchema.FieldByCol[localKey]
	if !ok {
		return fmt.Errorf("ember: LoadBelongsToMany: field %q not found in %s", localKey, parentSchema.GoType.Name())
	}
	lkVal := pVal.Field(lkField.Index).Interface()

	var pivotIDs []interface{}
	if err := m.builder(pivotTable).
		Select(relatedKey).
		Where(foreignKey, "=", lkVal).
		Pluck(ctx, relatedKey, &pivotIDs); err != nil {
		return err
	}

	if len(pivotIDs) == 0 {
		return nil
	}

	return m.All(ctx, dest, func(b *Builder) {
		b.WhereIn(relatedKey, pivotIDs...)
		if relatedSchema.HasSoftDelete {
			b.WhereNull("deleted_at")
		}
	})
}

func collectFieldValuesByCol(slice reflect.Value, schema *ModelSchema, col string) []interface{} {
	var vals []interface{}
	fs, ok := schema.FieldByCol[col]
	if !ok {
		return vals
	}
	seen := make(map[string]bool)
	for i := 0; i < slice.Len(); i++ {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		val := elem.Field(fs.Index).Interface()
		k := safeKey(val)
		if !seen[k] {
			seen[k] = true
			vals = append(vals, val)
		}
	}
	return vals
}

func deduplicate(vals []interface{}) []interface{} {
	seen := make(map[string]bool)
	result := make([]interface{}, 0, len(vals))
	for _, v := range vals {
		k := safeKey(v)
		if !seen[k] {
			seen[k] = true
			result = append(result, v)
		}
	}
	return result
}

func findRelationBySnake(schema *ModelSchema, snakeName string) *RelationDef {
	snakeName = strings.ReplaceAll(snakeName, "_", "")
	for _, rel := range schema.Relations {
		if strings.EqualFold(strings.ReplaceAll(toSnakeCase(rel.GoName), "_", ""), snakeName) {
			return rel
		}
	}
	return nil
}

func toGoName(snake string) string {
	parts := strings.Split(snake, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

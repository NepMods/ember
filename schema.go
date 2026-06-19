package ember

import (
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
)

// FieldSchema describes a single model field mapped to a database column.
type FieldSchema struct {
	GoName     string
	ColumnName string
	IsPrimary  bool
	AutoIncr   bool
	Nullable   bool
	Unique     bool
	Default    string
	GoType     reflect.Type
	Index      int
	CastType   CastType
}

// ModelSchema holds the parsed schema for a model struct.
type ModelSchema struct {
	TableName     string
	GoType        reflect.Type
	Fields        []*FieldSchema
	FieldByCol    map[string]*FieldSchema
	FieldByGo     map[string]*FieldSchema
	PrimaryKey    *FieldSchema
	HasCreatedAt  bool
	HasUpdatedAt  bool
	HasSoftDelete bool
	Relations     map[string]*RelationDef
}

var (
	schemaCache   = make(map[reflect.Type]*ModelSchema)
	schemaCacheMu sync.RWMutex
)

// ParseSchema parses a model struct into a ModelSchema, using a cache.
func ParseSchema(model interface{}) (*ModelSchema, error) {
	t := reflect.TypeOf(model)
	if t == nil {
		return nil, ErrInvalidModel
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, ErrInvalidModel
	}

	schemaCacheMu.RLock()
	if s, ok := schemaCache[t]; ok {
		schemaCacheMu.RUnlock()
		return s, nil
	}
	schemaCacheMu.RUnlock()

	s := &ModelSchema{
		GoType:     t,
		FieldByCol: make(map[string]*FieldSchema),
		FieldByGo:  make(map[string]*FieldSchema),
		Relations:  make(map[string]*RelationDef),
	}

	ptrType := reflect.PtrTo(t)
	impl := reflect.TypeOf((*interface{ TableName() string })(nil)).Elem()
	if ptrType.Implements(impl) {
		s.TableName = reflect.New(t).Interface().(interface{ TableName() string }).TableName()
	} else {
		s.TableName = toTableName(t.Name())
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		if sf.Anonymous {
			continue
		}

		if strings.HasPrefix(sf.Tag.Get("ember"), "relation:") {
			rel := parseRelationTag(sf)
			if rel != nil {
				s.Relations[rel.GoName] = rel
			}
			continue
		}

		fs := &FieldSchema{
			GoName: sf.Name,
			GoType: sf.Type,
			Index:  i,
		}

		fs.ColumnName = toSnakeCase(sf.Name)

		tag := sf.Tag.Get("ember")
		if tag == "-" {
			continue
		}
		parseFieldTag(tag, fs)

		switch fs.ColumnName {
		case "created_at":
			s.HasCreatedAt = true
			if fs.GoType == reflect.TypeOf(time.Time{}) || (fs.GoType.Kind() == reflect.Ptr && fs.GoType.Elem() == reflect.TypeOf(time.Time{})) {
				if fs.CastType == CastNone {
					fs.CastType = CastDatetime
				}
			}
		case "updated_at":
			s.HasUpdatedAt = true
			if fs.GoType == reflect.TypeOf(time.Time{}) || (fs.GoType.Kind() == reflect.Ptr && fs.GoType.Elem() == reflect.TypeOf(time.Time{})) {
				if fs.CastType == CastNone {
					fs.CastType = CastDatetime
				}
			}
		case "deleted_at":
			s.HasSoftDelete = true
			if fs.CastType == CastNone {
				fs.CastType = CastDatetime
			}
		}

		s.Fields = append(s.Fields, fs)
		s.FieldByCol[fs.ColumnName] = fs
		s.FieldByGo[fs.GoName] = fs

		if fs.IsPrimary {
			s.PrimaryKey = fs
		}
	}

	if s.PrimaryKey == nil {
		if f, ok := s.FieldByCol["id"]; ok {
			f.IsPrimary = true
			f.AutoIncr = true
			s.PrimaryKey = f
		}
	}

	schemaCacheMu.Lock()
	schemaCache[t] = s
	schemaCacheMu.Unlock()

	return s, nil
}

func parseFieldTag(tag string, fs *FieldSchema) {
	if tag == "" {
		return
	}
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := ""
		if len(kv) == 2 {
			val = strings.TrimSpace(kv[1])
		}
		switch key {
		case "column":
			if val != "" {
				fs.ColumnName = val
			}
		case "primarykey":
			fs.IsPrimary = true
		case "autoincr", "autoincrement":
			fs.AutoIncr = true
		case "nullable":
			fs.Nullable = true
		case "unique":
			fs.Unique = true
		case "default":
			fs.Default = val
		case "cast":
			switch val {
			case "string":
				fs.CastType = CastString
			case "int":
				fs.CastType = CastInt
			case "float":
				fs.CastType = CastFloat
			case "bool":
				fs.CastType = CastBool
			case "json":
				fs.CastType = CastJSON
			case "date":
				fs.CastType = CastDate
			case "datetime":
				fs.CastType = CastDatetime
			}
		}
	}
}

// RelationType represents the type of a model relationship.
type RelationType int

const (
	// HasOneRelation indicates a one-to-one relationship.
	HasOneRelation RelationType = iota
	// HasManyRelation indicates a one-to-many relationship.
	HasManyRelation
	// BelongsToRelation indicates an inverse one-to-one/one-to-many.
	BelongsToRelation
	// BelongsToManyRelation indicates a many-to-many relationship.
	BelongsToManyRelation
)

// RelationDef describes a relationship between models.
type RelationDef struct {
	GoName      string
	Type        RelationType
	RelatedType reflect.Type
	IsSlice     bool
	ForeignKey  string
	LocalKey    string
	OwnerKey    string
	PivotTable  string
	PivotFK     string
	PivotRK     string
}

// CastType represents a type cast applied to model fields.
type CastType int

const (
	// CastNone applies no casting.
	CastNone CastType = iota
	// CastString casts field values to strings.
	CastString
	// CastInt casts field values to integers.
	CastInt
	// CastFloat casts field values to floats.
	CastFloat
	// CastBool casts field values to booleans.
	CastBool
	// CastJSON casts field values to/from JSON.
	CastJSON
	// CastDate casts field values to dates.
	CastDate
	// CastDatetime casts field values to datetimes.
	CastDatetime
)

func parseRelationTag(sf reflect.StructField) *RelationDef {
	rel := &RelationDef{GoName: sf.Name}
	tag := sf.Tag.Get("ember")
	if tag == "" || tag == "-" {
		return nil
	}
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := ""
		if len(kv) == 2 {
			val = strings.TrimSpace(kv[1])
		}
		switch key {
		case "relation":
			switch strings.ToLower(val) {
			case "hasone":
				rel.Type = HasOneRelation
				rel.IsSlice = false
			case "hasmany":
				rel.Type = HasManyRelation
				rel.IsSlice = true
			case "belongsto":
				rel.Type = BelongsToRelation
				rel.IsSlice = false
			case "belongstomany":
				rel.Type = BelongsToManyRelation
				rel.IsSlice = true
			}
		case "foreignkey":
			rel.ForeignKey = val
		case "localkey":
			rel.LocalKey = val
		case "ownerkey":
			rel.OwnerKey = val
		case "pivot":
			rel.PivotTable = val
		case "pivotfk":
			rel.PivotFK = val
		case "pivotrk":
			rel.PivotRK = val
		}
	}
	if sf.Type.Kind() == reflect.Ptr {
		rel.RelatedType = sf.Type.Elem()
	} else if sf.Type.Kind() == reflect.Slice {
		rel.RelatedType = sf.Type.Elem()
		if rel.RelatedType.Kind() == reflect.Ptr {
			rel.RelatedType = rel.RelatedType.Elem()
		}
	} else {
		rel.RelatedType = sf.Type
	}
	return rel
}

func toSnakeCase(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && !unicode.IsUpper(runes[i-1]) {
				result = append(result, '_')
			} else if i > 0 && unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func toTableName(typeName string) string {
	snake := toSnakeCase(typeName)
	return pluralize(snake)
}

var irregularPlurals = map[string]string{
	"person": "people",
	"child":  "children",
	"mouse":  "mice",
	"ox":     "oxen",
	"goose":  "geese",
	"tooth":  "teeth",
	"foot":   "feet",
	"man":    "men",
	"woman":  "women",
	"series": "series",
	"sheep":  "sheep",
	"deer":   "deer",
	"fish":   "fish",
}

func pluralize(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	if plural, ok := irregularPlurals[lower]; ok {
		if s[0] >= 'A' && s[0] <= 'Z' {
			return strings.ToUpper(plural[:1]) + plural[1:]
		}
		return plural
	}
	switch {
	case strings.HasSuffix(s, "sis"):
		return s[:len(s)-3] + "ses"
	case strings.HasSuffix(s, "s") ||
		strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") ||
		strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh"):
		return s + "es"
	case strings.HasSuffix(s, "y") && len(s) > 1 && !isVowel(rune(s[len(s)-2])):
		return s[:len(s)-1] + "ies"
	default:
		return s + "s"
	}
}

func isVowel(r rune) bool {
	return strings.ContainsRune("aeiou", r)
}

func tagValue(tag, key string) string {
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if strings.TrimSpace(kv[0]) == key && len(kv) == 2 {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

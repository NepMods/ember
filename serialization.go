package ember

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// SerializationConfig controls serialization behaviour for models.
type SerializationConfig struct {
	DateFormat        string
	DateTimeFormat    string
	IncludeTimestamps bool
	IncludeSoftDelete bool
}

// DefaultSerializationConfig is the default serialization configuration.
var DefaultSerializationConfig = SerializationConfig{
	DateFormat:        "2006-01-02",
	DateTimeFormat:    time.RFC3339,
	IncludeTimestamps: true,
	IncludeSoftDelete: true,
}

// ModelToMap converts a model to a map using DefaultSerializationConfig.
func ModelToMap(model interface{}) (map[string]interface{}, error) {
	return ModelToMapWithConfig(model, &DefaultSerializationConfig)
}

// ModelToMapWithConfig converts a model to a map with a given config.
func ModelToMapWithConfig(model interface{}, cfg *SerializationConfig) (map[string]interface{}, error) {
	if cfg == nil {
		cfg = &DefaultSerializationConfig
	}
	if model == nil {
		return nil, fmt.Errorf("ember: model is nil")
	}
	schema, err := ParseSchema(model)
	if err != nil {
		return nil, err
	}

	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("ember: model pointer is nil")
		}
		val = val.Elem()
	}

	result := make(map[string]interface{}, len(schema.Fields))
	for _, f := range schema.Fields {
		if !cfg.IncludeTimestamps && (f.ColumnName == "created_at" || f.ColumnName == "updated_at") {
			continue
		}
		if !cfg.IncludeSoftDelete && f.ColumnName == "deleted_at" {
			continue
		}

		fv := val.Field(f.Index)
		v := fv.Interface()

		switch f.CastType {
		case CastJSON:
			if data, err := json.Marshal(v); err == nil {
				v = string(data)
			}
		case CastDate:
			if t, ok := v.(time.Time); ok && !t.IsZero() {
				v = t.Format(cfg.DateFormat)
			} else if pt, ok := v.(*time.Time); ok && pt != nil && !pt.IsZero() {
				v = pt.Format(cfg.DateFormat)
			}
		case CastDatetime:
			if t, ok := v.(time.Time); ok && !t.IsZero() {
				v = t.Format(cfg.DateTimeFormat)
			} else if pt, ok := v.(*time.Time); ok && pt != nil && !pt.IsZero() {
				v = pt.Format(cfg.DateTimeFormat)
			}
		}

		result[f.ColumnName] = v
	}
	return result, nil
}

// ToJSON serializes a model to JSON using DefaultSerializationConfig.
func ToJSON(model interface{}) ([]byte, error) {
	m, err := ModelToMap(model)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// ToJSONWithConfig serializes a model to JSON with a given config.
func ToJSONWithConfig(model interface{}, cfg *SerializationConfig) ([]byte, error) {
	m, err := ModelToMapWithConfig(model, cfg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// CollectionToMap converts a model slice to a map slice using DefaultSerializationConfig.
func CollectionToMap(models interface{}) ([]map[string]interface{}, error) {
	return CollectionToMapWithConfig(models, &DefaultSerializationConfig)
}

// CollectionToMapWithConfig converts a model slice to maps with a given config.
func CollectionToMapWithConfig(models interface{}, cfg *SerializationConfig) ([]map[string]interface{}, error) {
	val := reflect.ValueOf(models)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("ember: expected a slice, got %T", models)
	}

	result := make([]map[string]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		if elem.Kind() == reflect.Ptr {
			if elem.IsNil() {
				result[i] = nil
				continue
			}
			elem = elem.Elem()
		}
		var modelPtr interface{}
		if elem.CanAddr() {
			modelPtr = elem.Addr().Interface()
		} else {
			cp := reflect.New(elem.Type())
			cp.Elem().Set(elem)
			modelPtr = cp.Interface()
		}
		m, err := ModelToMapWithConfig(modelPtr, cfg)
		if err != nil {
			return nil, err
		}
		result[i] = m
	}
	return result, nil
}

// CollectionToJSON serializes a model slice to JSON using DefaultSerializationConfig.
func CollectionToJSON(models interface{}) ([]byte, error) {
	maps, err := CollectionToMap(models)
	if err != nil {
		return nil, err
	}
	return json.Marshal(maps)
}

// SelectColumns generates a column list for SELECT queries.
func SelectColumns(schema *ModelSchema, dialect Dialect) string {
	cols := make([]string, len(schema.Fields))
	for i, f := range schema.Fields {
		cols[i] = dialect.QuoteIdentifier(schema.TableName) + "." + dialect.QuoteIdentifier(f.ColumnName)
	}
	return strings.Join(cols, ", ")
}

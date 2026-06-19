package ember

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

func applyCastsOnRead(elem reflect.Value, schema *ModelSchema) {
	for _, fs := range schema.Fields {
		if fs.CastType == CastNone {
			continue
		}
		fv := elem.Field(fs.Index)
		if !fv.IsValid() || !fv.CanSet() {
			continue
		}

		switch fs.CastType {
		case CastJSON:
			castJSONOnRead(fv)
		case CastDate:
			castDateOnRead(fv)
		case CastDatetime:
			castDatetimeOnRead(fv)
		case CastInt:
			castIntOnRead(fv)
		case CastFloat:
			castFloatOnRead(fv)
		case CastBool:
			castBoolOnRead(fv)
		case CastString:
			castStringOnRead(fv)
		}
	}
}

func applyCastsOnWrite(data map[string]interface{}, schema *ModelSchema) {
	for _, fs := range schema.Fields {
		val, ok := data[fs.ColumnName]
		if !ok {
			continue
		}
		if val == nil {
			continue
		}

		switch fs.CastType {
		case CastJSON:
			data[fs.ColumnName] = castJSONOnWrite(val)
		case CastDate:
			data[fs.ColumnName] = castDateOnWrite(val)
		case CastDatetime:
			data[fs.ColumnName] = castDatetimeOnWrite(val)
		}
	}
}

func castJSONOnRead(fv reflect.Value) {
	var data []byte
	switch fv.Interface().(type) {
	case []byte:
		data = fv.Bytes()
	case string:
		data = []byte(fv.String())
	default:
		return
	}
	if len(data) == 0 {
		return
	}
	target := reflect.New(fv.Type())
	if err := json.Unmarshal(data, target.Interface()); err == nil {
		fv.Set(target.Elem())
	}
}

func castDateOnRead(fv reflect.Value) {
	if fv.Type() == reflect.TypeOf(time.Time{}) {
		return
	}
	if fv.Type() == reflect.TypeOf(&time.Time{}) {
		return
	}
	s := fv.String()
	if s == "" {
		return
	}
	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		fv.Set(reflect.ValueOf(t))
	}
}

func castDatetimeOnRead(fv reflect.Value) {
	if fv.Type() == reflect.TypeOf(time.Time{}) {
		return
	}
	if fv.Type() == reflect.TypeOf(&time.Time{}) {
		return
	}
	s := fv.String()
	if s == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		fv.Set(reflect.ValueOf(t))
	}
}

func castIntOnRead(fv reflect.Value) {
	if !fv.CanSet() {
		return
	}
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return
	case reflect.Float32, reflect.Float64:
		i := int64(fv.Float())
		if !fv.OverflowInt(i) {
			fv.SetFloat(float64(i))
		}
	case reflect.String:
		s := fv.String()
		if s == "" {
			return
		}
		var i int64
		if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
			fv.SetString(fmt.Sprintf("%d", i))
		}
	}
}

func castFloatOnRead(fv reflect.Value) {
	if !fv.CanSet() {
		return
	}
	switch fv.Kind() {
	case reflect.Float32, reflect.Float64:
		return
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return
	case reflect.String:
		var f float64
		if _, err := fmt.Sscanf(fv.String(), "%f", &f); err == nil {
			fv.SetString(fmt.Sprintf("%f", f))
		}
	}
}

func castBoolOnRead(fv reflect.Value) {
	if !fv.CanSet() {
		return
	}
	if fv.Kind() == reflect.Bool {
		return
	}
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fv.SetBool(fv.Int() != 0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fv.SetBool(fv.Uint() != 0)
	case reflect.Float32, reflect.Float64:
		fv.SetBool(fv.Float() != 0)
	case reflect.String:
		s := fv.String()
		if s == "" {
			return
		}
		v, err := strconv.ParseBool(s)
		if err == nil {
			fv.SetBool(v)
		}
	}
}

func castStringOnRead(fv reflect.Value) {
	if !fv.CanSet() {
		return
	}
	if fv.Kind() == reflect.String {
		return
	}
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fv.SetString(strconv.FormatInt(fv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fv.SetString(strconv.FormatUint(fv.Uint(), 10))
	case reflect.Float32:
		fv.SetString(strconv.FormatFloat(fv.Float(), 'f', -1, 32))
	case reflect.Float64:
		fv.SetString(strconv.FormatFloat(fv.Float(), 'f', -1, 64))
	case reflect.Bool:
		fv.SetString(strconv.FormatBool(fv.Bool()))
	}
}

func castJSONOnWrite(val interface{}) interface{} {
	if s, ok := val.(string); ok {
		if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
			return s
		}
	}
	data, err := json.Marshal(val)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func castDateOnWrite(val interface{}) interface{} {
	switch v := val.(type) {
	case time.Time:
		return v.Format("2006-01-02")
	case *time.Time:
		if v != nil {
			return v.Format("2006-01-02")
		}
		return nil
	}
	return val
}

func castDatetimeOnWrite(val interface{}) interface{} {
	switch v := val.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case *time.Time:
		if v != nil {
			return v.Format(time.RFC3339)
		}
		return nil
	}
	return val
}

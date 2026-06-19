package ember

import "fmt"

var (
	// ErrNotFound is returned when no record matches the query.
	ErrNotFound = fmt.Errorf("ember: record not found")
	// ErrInvalidModel is returned when the model is not a pointer to a struct.
	ErrInvalidModel = fmt.Errorf("ember: model must be a pointer to a struct")
	// ErrMissingPrimaryKey is returned when the model has no primary key value.
	ErrMissingPrimaryKey = fmt.Errorf("ember: model has no primary key value")
	// ErrDuplicateConnection is returned when a connection name is already registered.
	ErrDuplicateConnection = fmt.Errorf("ember: a connection with this name already exists")
)

// QueryError wraps an SQL error with the query and bindings.
type QueryError struct {
	SQL      string
	Bindings []interface{}
	Cause    error
}

// QueryError.Error
func (e *QueryError) Error() string {
	return fmt.Sprintf("ember: query error [%s]: %v", e.SQL, e.Cause)
}

// QueryError.Unwrap
func (e *QueryError) Unwrap() error {
	return e.Cause
}

func newQueryError(sql string, bindings []interface{}, cause error) error {
	return &QueryError{SQL: sql, Bindings: bindings, Cause: cause}
}

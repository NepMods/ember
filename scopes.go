package ember

import (
	"reflect"
	"sync"
)

// ScopeRegistry manages global scopes for model types.
type ScopeRegistry struct {
	mu   sync.RWMutex
	data map[reflect.Type][]Scope
}

// NewScopeRegistry creates a new ScopeRegistry.
func NewScopeRegistry() *ScopeRegistry {
	return &ScopeRegistry{
		data: make(map[reflect.Type][]Scope),
	}
}

// Add registers global scopes for a model type.
func (r *ScopeRegistry) Add(model interface{}, scopes ...Scope) {
	if model == nil {
		return
	}
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	r.mu.Lock()
	r.data[t] = append(r.data[t], scopes...)
	r.mu.Unlock()
}

// Get returns a copy of the registered scopes for the given type.
func (r *ScopeRegistry) Get(modelType reflect.Type) []Scope {
	r.mu.RLock()
	defer r.mu.RUnlock()
	scopes := r.data[modelType]
	result := make([]Scope, len(scopes))
	copy(result, scopes)
	return result
}

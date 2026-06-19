package ember

import (
	"fmt"
	"sync"
)

// BuilderMacro is a callable macro attached to a Builder.
type BuilderMacro func(*Builder, ...interface{}) interface{}

var (
	globalBuilderMacros   = make(map[string]BuilderMacro)
	globalBuilderMacrosMu sync.RWMutex
)

// AddBuilderMacro registers a global builder macro.
func AddBuilderMacro(name string, fn BuilderMacro) {
	globalBuilderMacrosMu.Lock()
	defer globalBuilderMacrosMu.Unlock()
	globalBuilderMacros[name] = fn
}

// Macro attaches a macro to this builder instance.
func (b *Builder) Macro(name string, fn BuilderMacro) *Builder {
	if b.macros == nil {
		b.macros = make(map[string]BuilderMacro)
	}
	b.macros[name] = fn
	return b
}

// Call invokes a macro by name.
func (b *Builder) Call(name string, args ...interface{}) (interface{}, error) {
	if b.err != nil {
		return nil, b.err
	}
	if fn, ok := b.macros[name]; ok {
		return fn(b, args...), nil
	}
	globalBuilderMacrosMu.RLock()
	fn, ok := globalBuilderMacros[name]
	globalBuilderMacrosMu.RUnlock()
	if ok {
		return fn(b, args...), nil
	}
	return nil, fmt.Errorf("ember: macro %q not found", name)
}

package ember

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ConnectionManager manages multiple named database connections.
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*DB
	defaultName string
}

// NewConnectionManager creates a new ConnectionManager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*DB),
		defaultName: "default",
	}
}

// Add opens a connection from config and registers it.
func (m *ConnectionManager) Add(name string, cfg Config) error {
	db, err := Open(cfg)
	if err != nil {
		return err
	}
	return m.AddDB(name, db)
}

// AddDB registers an existing DB connection.
func (m *ConnectionManager) AddDB(name string, db *DB) error {
	if db == nil {
		return fmt.Errorf("ember: cannot add nil *DB as %q", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateConnection, name)
	}
	m.connections[name] = db
	return nil
}

// SetDefault sets the default connection name.
func (m *ConnectionManager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.connections[name]; !exists {
		return fmt.Errorf("ember: connection %q is not registered", name)
	}
	m.defaultName = name
	return nil
}

// Connection returns the named connection, panicking if not found.
func (m *ConnectionManager) Connection(name string) *DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	db, ok := m.connections[name]
	if !ok {
		panic(fmt.Sprintf("ember: connection %q is not registered", name))
	}
	return db
}

// ConnectionSafe returns the named connection without panicking.
func (m *ConnectionManager) ConnectionSafe(name string) (*DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	db, ok := m.connections[name]
	if !ok {
		return nil, fmt.Errorf("ember: connection %q is not registered", name)
	}
	return db, nil
}

// DB returns the default connection.
func (m *ConnectionManager) DB() *DB {
	return m.Connection(m.defaultName)
}

// Remove unregisters and closes the named connection.
func (m *ConnectionManager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	db, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("ember: connection %q is not registered", name)
	}
	delete(m.connections, name)
	return db.Close()
}

// CloseAll closes all registered connections.
func (m *ConnectionManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []string
	for name, db := range m.connections {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("[%s]: %v", name, err))
		}
	}
	m.connections = make(map[string]*DB)
	if len(errs) > 0 {
		return fmt.Errorf("ember: CloseAll errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Names returns all registered connection names.
func (m *ConnectionManager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.connections))
	for n := range m.connections {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

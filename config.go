package ember

import "time"

// Config holds database connection configuration.
type Config struct {
	Driver   string
	Master   string
	Replicas []string
	Pool     PoolConfig
}

// PoolConfig holds connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

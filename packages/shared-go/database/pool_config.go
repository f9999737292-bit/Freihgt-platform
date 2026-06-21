package database

import (
	"os"
	"strconv"
	"time"
)

// PoolConfig holds database/sql and pgxpool connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// LoadPoolConfig reads pool settings from environment variables.
func LoadPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: time.Duration(envInt("DB_CONN_MAX_LIFETIME_SECONDS", 300)) * time.Second,
		ConnMaxIdleTime: time.Duration(envInt("DB_CONN_MAX_IDLE_TIME_SECONDS", 60)) * time.Second,
	}
}

// ApplyToSQL configures a database/sql connection pool.
func (c PoolConfig) ApplyToSQL(db interface {
	SetMaxOpenConns(int)
	SetMaxIdleConns(int)
	SetConnMaxLifetime(time.Duration)
	SetConnMaxIdleTime(time.Duration)
}) {
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)
	db.SetConnMaxIdleTime(c.ConnMaxIdleTime)
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

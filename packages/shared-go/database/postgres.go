package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectPgxPool opens a PostgreSQL pool with shared pool configuration.
func ConnectPgxPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg := LoadPoolConfig()
	ApplyToPgxConfig(cfg, poolCfg)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// ApplyToPgxConfig maps pool settings to pgxpool configuration.
func ApplyToPgxConfig(cfg *pgxpool.Config, poolCfg PoolConfig) {
	maxOpen := poolCfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := poolCfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 1
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	cfg.MaxConns = int32(maxOpen)
	cfg.MinConns = int32(maxIdle)
	cfg.MaxConnLifetime = poolCfg.ConnMaxLifetime
	cfg.MaxConnIdleTime = poolCfg.ConnMaxIdleTime
}

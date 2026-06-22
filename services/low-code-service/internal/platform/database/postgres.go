package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	shareddb "github.com/freight-platform/shared-go/database"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := shareddb.ConnectPgxPool(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) Close() {
	if p != nil && p.Pool != nil {
		p.Pool.Close()
	}
}

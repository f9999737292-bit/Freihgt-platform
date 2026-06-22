package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadinessChecker struct {
	Pool *pgxpool.Pool
}

type ReadinessResult struct {
	DatabaseOK bool
	SchemaOK   bool
	TableOK    bool
	Ready      bool
	Error      string
}

func NewReadinessChecker(pool *pgxpool.Pool) *ReadinessChecker {
	return &ReadinessChecker{Pool: pool}
}

func (r *ReadinessChecker) Ping(ctx context.Context) error {
	result := r.Check(ctx)
	if !result.Ready {
		if result.Error != "" {
			return fmt.Errorf("%s", result.Error)
		}
		return fmt.Errorf("low-code-service not ready")
	}
	return nil
}

func (r *ReadinessChecker) Check(ctx context.Context) ReadinessResult {
	if r == nil || r.Pool == nil {
		return ReadinessResult{Error: "database not configured"}
	}

	if err := r.Pool.Ping(ctx); err != nil {
		return ReadinessResult{Error: err.Error()}
	}

	var schemaExists bool
	if err := r.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata WHERE schema_name = 'lowcode'
		)`).Scan(&schemaExists); err != nil {
		return ReadinessResult{DatabaseOK: true, Error: fmt.Sprintf("schema check failed: %v", err)}
	}
	if !schemaExists {
		return ReadinessResult{DatabaseOK: true, Error: "schema lowcode not found"}
	}

	var tableExists bool
	if err := r.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'lowcode' AND table_name = 'form_templates'
		)`).Scan(&tableExists); err != nil {
		return ReadinessResult{DatabaseOK: true, SchemaOK: true, Error: fmt.Sprintf("table check failed: %v", err)}
	}
	if !tableExists {
		return ReadinessResult{DatabaseOK: true, SchemaOK: true, Error: "table lowcode.form_templates not found"}
	}

	return ReadinessResult{
		DatabaseOK: true,
		SchemaOK:   true,
		TableOK:    true,
		Ready:      true,
	}
}

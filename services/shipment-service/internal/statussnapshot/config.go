package statussnapshot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/statussnapshot"
)

var (
	ErrInvalidConfig    = errors.New("invalid exporter config")
	ErrNotImplemented   = errors.New("NOT_IMPLEMENTED_EXPORT_QUERY")
	ErrUnknownFormat    = errors.New("unknown format")
	ErrConflictingScope = errors.New("conflicting scope flags")
)

const (
	DefaultBatchSize = 1000
	MinBatchSize     = 1
	MaxBatchSize     = 10000
	DefaultFormat    = "ndjson"
)

type Config struct {
	ScopeAll   bool
	TenantID   string
	BatchSize  int
	Format     string
	OutputPath string
}

func LoadConfig(scopeAll bool, tenantID string, batchSize int, format, outputPath string) (Config, error) {
	cfg := Config{
		ScopeAll:   scopeAll,
		TenantID:   strings.TrimSpace(tenantID),
		BatchSize:  batchSize,
		Format:     strings.TrimSpace(format),
		OutputPath: strings.TrimSpace(outputPath),
	}
	if cfg.Format == "" {
		cfg.Format = DefaultFormat
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = "-"
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.ScopeAll && c.TenantID != "" {
		return fmt.Errorf("%w: cannot combine scope all with tenant", ErrConflictingScope)
	}
	if !c.ScopeAll && c.TenantID == "" {
		return fmt.Errorf("%w: exactly one of --scope all or --tenant is required", ErrInvalidConfig)
	}
	if c.TenantID != "" {
		if _, err := uuid.Parse(c.TenantID); err != nil {
			return fmt.Errorf("%w: invalid tenant uuid", ErrInvalidConfig)
		}
	}
	if c.BatchSize < MinBatchSize || c.BatchSize > MaxBatchSize {
		return fmt.Errorf("%w: batch size out of range", ErrInvalidConfig)
	}
	if c.Format != DefaultFormat {
		return ErrUnknownFormat
	}
	return nil
}

func (c Config) Scope() statussnapshot.Scope {
	if c.ScopeAll {
		return statussnapshot.ScopeAll
	}
	return statussnapshot.ScopeTenant
}

func (c Config) ParsedTenantID() *uuid.UUID {
	if c.TenantID == "" {
		return nil
	}
	id, _ := uuid.Parse(c.TenantID)
	return &id
}

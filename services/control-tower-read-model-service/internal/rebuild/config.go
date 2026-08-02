package rebuild

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidConfig       = errors.New("invalid importer config")
	ErrNotImplemented      = errors.New("NOT_IMPLEMENTED")
	ErrConflictingMode     = errors.New("conflicting CLI mode")
	ErrActivationForbidden = errors.New("activation is not implemented in v0.1 core infrastructure")
)

const (
	DefaultBatchSize = 500
	MinBatchSize     = 1
	MaxBatchSize     = 10000
)

type Config struct {
	Stdin      bool
	DryRun     bool
	Activate   bool
	Status     bool
	Cleanup    bool
	Rollback   bool
	SnapshotID string
	BatchSize  int
}

func LoadConfig(stdin, dryRun, activate, status, cleanup, rollback bool, snapshotID string, batchSize int) (Config, error) {
	cfg := Config{
		Stdin: stdin, DryRun: dryRun, Activate: activate, Status: status,
		Cleanup: cleanup, Rollback: rollback, SnapshotID: strings.TrimSpace(snapshotID), BatchSize: batchSize,
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	modeCount := 0
	for _, on := range []bool{c.DryRun, c.Activate, c.Status, c.Cleanup, c.Rollback} {
		if on {
			modeCount++
		}
	}
	if modeCount > 1 {
		return ErrConflictingMode
	}
	if c.Activate || c.Cleanup || c.Rollback {
		if c.SnapshotID == "" {
			return fmt.Errorf("%w: snapshot-id required", ErrInvalidConfig)
		}
	}
	if c.Cleanup || c.Rollback {
		return ErrNotImplemented
	}
	if c.Status {
		if c.SnapshotID == "" {
			return fmt.Errorf("%w: snapshot-id required for status", ErrInvalidConfig)
		}
		return nil
	}
	if !c.Stdin {
		return fmt.Errorf("%w: --stdin required for import", ErrInvalidConfig)
	}
	if c.DryRun && c.Activate {
		return ErrConflictingMode
	}
	if c.BatchSize < MinBatchSize || c.BatchSize > MaxBatchSize {
		return fmt.Errorf("%w: batch size out of range", ErrInvalidConfig)
	}
	return nil
}

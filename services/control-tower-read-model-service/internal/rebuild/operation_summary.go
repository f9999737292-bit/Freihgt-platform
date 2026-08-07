package rebuild

import (
	"log/slog"
	"sync"
	"time"
)

// Operation summaries for activation/rollback CLI operations.
// These are structured logs, not scrapeable Prometheus metrics.
type operationSummary struct {
	mu sync.Mutex
}

var defaultOperationSummary = &operationSummary{}

func ObserveActivation(scope, result, errorCode string, duration time.Duration) {
	defaultOperationSummary.record("activation", scope, result, errorCode, duration)
}

func ObserveRollback(scope, result, errorCode string, duration time.Duration) {
	defaultOperationSummary.record("rollback", scope, result, errorCode, duration)
}

func ObserveLockWait(scope string, duration time.Duration) {
	slog.Info("projection rebuild lock wait",
		slog.String("operation", "lock_wait"),
		slog.String("scope", scope),
		slog.Duration("lock_wait_duration", duration),
	)
}

func (m *operationSummary) record(operation, scope, result, errorCode string, duration time.Duration) {
	args := []any{
		slog.String("operation", operation),
		slog.String("scope", scope),
		slog.String("result", result),
		slog.Duration("duration", duration),
	}
	if errorCode != "" {
		args = append(args, slog.String("safe_error_code", errorCode))
	}
	slog.Info("projection rebuild operation", args...)
}

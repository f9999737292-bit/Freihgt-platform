package chainapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
)

// RecoveryConfig is the runtime cadence for abandoned chain claims.
// NLO has no prior background worker, so the service owns a bounded sweep:
// one pass at startup, then every Interval, at most Limit rows per pass.
// The sweep only calls I2B SweepExpiredClaims. It does not recompute or take over.
type RecoveryConfig struct {
	Interval time.Duration
	Limit    int
	Log      *slog.Logger
}

func (c RecoveryConfig) normalized() RecoveryConfig {
	if c.Interval < time.Second {
		c.Interval = 5 * time.Second
	}
	if c.Limit < 1 {
		c.Limit = 32
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	return c
}

// RunRecovery sweeps expired claims until ctx is cancelled.
func RunRecovery(ctx context.Context, orch *chainorch.Orchestrator, cfg RecoveryConfig) {
	cfg = cfg.normalized()
	if orch == nil {
		return
	}
	sweep := func() {
		recovered, err := orch.SweepExpiredClaims(ctx, cfg.Limit)
		if err != nil && ctx.Err() == nil {
			cfg.Log.Error("chain claim recovery sweep failed", slog.String("error_class", "dependency"))
			return
		}
		if recovered > 0 {
			cfg.Log.Info("chain claim recovery sweep", slog.Int("recovered", recovered))
		}
	}
	sweep()
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}

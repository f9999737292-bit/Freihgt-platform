package service

import (
	"os"
	"strings"
	"sync/atomic"
)

var guardedActionsGlobalEnabled atomic.Bool

func init() {
	guardedActionsGlobalEnabled.Store(parseEnvBool(os.Getenv("GLOBAL_GUARDED_ACTIONS_ENABLED")))
}

// SetGuardedActionsGlobalEnabled configures the global guarded-action kill switch (test wiring).
func SetGuardedActionsGlobalEnabled(enabled bool) {
	guardedActionsGlobalEnabled.Store(enabled)
}

// GuardedActionsGlobalEnabled reports whether guarded external actions may execute.
func GuardedActionsGlobalEnabled() bool {
	return guardedActionsGlobalEnabled.Load()
}

func parseEnvBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

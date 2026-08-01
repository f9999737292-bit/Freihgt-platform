package legacyaggregate

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const defaultTimeout = 800 * time.Millisecond

func LoadTimeoutFromEnv() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("CONTROL_TOWER_LEGACY_STATUS_TIMEOUT"))
	if raw == "" {
		return defaultTimeout, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid CONTROL_TOWER_LEGACY_STATUS_TIMEOUT: %q", raw)
	}
	return parsed, nil
}

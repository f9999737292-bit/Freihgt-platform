package controltowerreadmodel

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envMode                   = "CONTROL_TOWER_READ_MODEL_MODE"
	envBaseURL                = "CONTROL_TOWER_READ_MODEL_BASE_URL"
	envTimeout                = "CONTROL_TOWER_READ_MODEL_TIMEOUT"
	envRequireConsumerRunning = "CONTROL_TOWER_READ_MODEL_REQUIRE_CONSUMER_RUNNING"
)

type Config struct {
	Mode                   Mode
	BaseURL                string
	Timeout                time.Duration
	RequireConsumerRunning bool
	MaxResponseBytes       int64
}

func LoadConfigFromEnv() (Config, error) {
	mode, err := ParseMode(os.Getenv(envMode))
	if err != nil {
		return Config{}, err
	}

	timeout := 800 * time.Millisecond
	if raw := strings.TrimSpace(os.Getenv(envTimeout)); raw != "" {
		parsed, parseErr := time.ParseDuration(raw)
		if parseErr != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envTimeout, parseErr)
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("%s must be > 0", envTimeout)
		}
		timeout = parsed
	}

	requireConsumer := true
	if raw := strings.TrimSpace(os.Getenv(envRequireConsumerRunning)); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envRequireConsumerRunning, parseErr)
		}
		requireConsumer = parsed
	}

	cfg := Config{
		Mode:                   mode,
		BaseURL:                strings.TrimRight(strings.TrimSpace(os.Getenv(envBaseURL)), "/"),
		Timeout:                timeout,
		RequireConsumerRunning: requireConsumer,
		MaxResponseBytes:       256 * 1024,
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Mode == ModeDisabled {
		return nil
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("%s is required when %s=%s", envBaseURL, envMode, c.Mode)
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", envBaseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s must use http or https scheme", envBaseURL)
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("%s must include host", envBaseURL)
	}
	if u.User != nil {
		return fmt.Errorf("%s must not include credentials", envBaseURL)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%s must not include query or fragment", envBaseURL)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("%s must be > 0", envTimeout)
	}
	return nil
}

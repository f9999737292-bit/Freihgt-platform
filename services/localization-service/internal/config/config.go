package config

import "github.com/freight-platform/shared-go/config"

// Config holds localization-service configuration.
type Config = config.Base

// Load reads service configuration from the environment.
func Load() (Config, error) {
	return config.Load("localization-service", 8083)
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServiceName                      string
	Environment                      string
	HTTPPort                         int
	LogLevel                         string
	DatabaseURL                      string
	InternalServiceToken             string
	JWTSecret                        string
	RelationshipTokenPepper          string
	RelationshipCommandsEnabled      bool
	ShipmentRelationshipServiceToken string
	RfxRelationshipServiceToken      string
}

func Load() (Config, error) {
	portRaw := os.Getenv("COMPANY_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8082"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid COMPANY_SERVICE_PORT: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	return Config{
		ServiceName:                      "company-service",
		Environment:                      getEnv("ENVIRONMENT", "development"),
		HTTPPort:                         port,
		LogLevel:                         getEnv("LOG_LEVEL", "info"),
		DatabaseURL:                      databaseURL,
		InternalServiceToken:             os.Getenv("INTERNAL_SERVICE_TOKEN"),
		JWTSecret:                        os.Getenv("JWT_SECRET"),
		RelationshipTokenPepper:          os.Getenv("RELATIONSHIP_TOKEN_PEPPER"),
		RelationshipCommandsEnabled:      relationshipCommandsEnabled(),
		ShipmentRelationshipServiceToken: os.Getenv("SHIPMENT_RELATIONSHIP_SERVICE_TOKEN"),
		RfxRelationshipServiceToken:      os.Getenv("RFX_RELATIONSHIP_SERVICE_TOKEN"),
	}, nil
}

func relationshipCommandsEnabled() bool {
	value := strings.TrimSpace(os.Getenv("COMPANY_RELATIONSHIP_COMMANDS_ENABLED"))
	return strings.EqualFold(value, "true") || value == "1"
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltower/legacyaggregate"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

type ServiceURLs struct {
	Identity        string
	Company         string
	TransportOrder  string
	RFX             string
	Shipment        string
	Document        string
	BillingRegister string
	Payment         string
	LowCode         string
	Tracking        string
}

type ControlTowerConfig struct {
	AtRiskMinutes           int
	CriticalDelayMinutes    int
	StaleWarningMinutes     int
	StaleCriticalMinutes    int
	MaxDownstreamFetchLimit int
	LegacyStatusTimeout     time.Duration
	ReadModel               controltowerreadmodel.Config
}

type Config struct {
	ServiceName         string
	Environment         string
	HTTPPort            int
	LogLevel            string
	Services            ServiceURLs
	ControlTower        ControlTowerConfig
	AuthEnabled         bool
	JWTSecret           string
	DevTenantID         string
	CORSAllowedOrigins  []string
	ProxyTimeoutSeconds int
	ReadyCheckTimeoutMS int
	OpenAPIDir          string
	RateLimitEnabled    bool
	RateLimitRPS        float64
	RateLimitBurst      int
	MaxRequestBodyBytes int64
	TrackingInternalToken string
}

func Load() (Config, error) {
	portRaw := os.Getenv("API_GATEWAY_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8080"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid API_GATEWAY_PORT: %w", err)
	}

	authEnabled := false
	if raw := strings.TrimSpace(os.Getenv("AUTH_ENABLED")); raw != "" {
		authEnabled, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AUTH_ENABLED: %w", err)
		}
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev_secret_change_me"
	}

	proxyTimeout := 30
	if raw := strings.TrimSpace(os.Getenv("PROXY_TIMEOUT_SECONDS")); raw != "" {
		proxyTimeout, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PROXY_TIMEOUT_SECONDS: %w", err)
		}
	}

	readyTimeoutMS := 2000
	if raw := strings.TrimSpace(os.Getenv("READY_CHECK_TIMEOUT_MS")); raw != "" {
		readyTimeoutMS, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid READY_CHECK_TIMEOUT_MS: %w", err)
		}
	}

	rateLimitEnabled := true
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_ENABLED")); raw != "" {
		rateLimitEnabled, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_ENABLED: %w", err)
		}
	}

	rateLimitRPS := 50.0
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_RPS")); raw != "" {
		rateLimitRPS, err = strconv.ParseFloat(raw, 64)
		if err != nil || rateLimitRPS <= 0 {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_RPS: %w", err)
		}
	}

	rateLimitBurst := 100
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_BURST")); raw != "" {
		rateLimitBurst, err = strconv.Atoi(raw)
		if err != nil || rateLimitBurst <= 0 {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
		}
	}

	maxBodyBytes := int64(10 * 1024 * 1024)
	if raw := strings.TrimSpace(os.Getenv("MAX_REQUEST_BODY_BYTES")); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("invalid MAX_REQUEST_BODY_BYTES: %w", parseErr)
		}
		maxBodyBytes = parsed
	}

	readModelCfg, err := controltowerreadmodel.LoadConfigFromEnv()
	if err != nil {
		return Config{}, err
	}

	legacyStatusTimeout, err := legacyaggregate.LoadTimeoutFromEnv()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName: "api-gateway",
		Environment: getEnv("ENVIRONMENT", "development"),
		HTTPPort:    port,
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		OpenAPIDir:  resolveOpenAPIDir(getEnv("OPENAPI_DIR", "")),
		ControlTower: ControlTowerConfig{
			AtRiskMinutes:           intEnv("CONTROL_TOWER_AT_RISK_MINUTES", 120),
			CriticalDelayMinutes:    intEnv("CONTROL_TOWER_CRITICAL_DELAY_MINUTES", 240),
			StaleWarningMinutes:     intEnv("CONTROL_TOWER_STALE_WARNING_MINUTES", 120),
			StaleCriticalMinutes:    intEnv("CONTROL_TOWER_STALE_CRITICAL_MINUTES", 360),
			MaxDownstreamFetchLimit: intEnv("CONTROL_TOWER_MAX_DOWNSTREAM_FETCH_LIMIT", 100),
			LegacyStatusTimeout:     legacyStatusTimeout,
			ReadModel:               readModelCfg,
		},
		Services: ServiceURLs{
			Identity:        getEnv("IDENTITY_SERVICE_URL", "http://localhost:8081"),
			Company:         getEnv("COMPANY_SERVICE_URL", "http://localhost:8082"),
			TransportOrder:  getEnv("TRANSPORT_ORDER_SERVICE_URL", "http://localhost:8083"),
			RFX:             getEnv("RFX_SERVICE_URL", "http://localhost:8084"),
			Shipment:        getEnv("SHIPMENT_SERVICE_URL", "http://localhost:8085"),
			Document:        getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8086"),
			BillingRegister: getEnv("BILLING_REGISTER_SERVICE_URL", "http://localhost:8087"),
			Payment:         getEnv("PAYMENT_SERVICE_URL", "http://localhost:8090"),
			LowCode:         getEnv("LOW_CODE_SERVICE_URL", "http://localhost:8088"),
			Tracking:        getEnv("TRACKING_SERVICE_URL", "http://localhost:8089"),
		},
		AuthEnabled:         authEnabled,
		JWTSecret:           jwtSecret,
		DevTenantID:         getEnv("DEV_TENANT_ID", ""),
		CORSAllowedOrigins:  parseOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001,http://localhost:5173")),
		ProxyTimeoutSeconds: proxyTimeout,
		ReadyCheckTimeoutMS: readyTimeoutMS,
		RateLimitEnabled:    rateLimitEnabled,
		RateLimitRPS:        rateLimitRPS,
		RateLimitBurst:      rateLimitBurst,
		MaxRequestBodyBytes: maxBodyBytes,
		TrackingInternalToken: getEnv("TRACKING_INTERNAL_SERVICE_TOKEN", getEnv("INTERNAL_SERVICE_TOKEN", "dev_internal_tracking_token")),
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func resolveOpenAPIDir(fromEnv string) string {
	if fromEnv != "" {
		return fromEnv
	}

	candidates := []string{
		"packages/openapi",
		"../../packages/openapi",
		"../../../packages/openapi",
		"/app/packages/openapi",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "packages/openapi"
}

func parseOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

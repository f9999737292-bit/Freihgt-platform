package storage

import (
	"net"
	"net/url"
	"os"
	"strings"
)

// Config is the object-storage contract. Secret fields are never formatted into errors.
type Config struct {
	Provider  string
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PathStyle bool
	TLS       bool
	Prefix    string
	LocalRoot string
}

// LoadConfig reads EDO_OBJECT_STORAGE_*. An empty provider selects the local adapter.
// Provider s3 fails closed when any required setting is missing and never falls back to local.
func LoadConfig(localRoot string) (Config, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("EDO_OBJECT_STORAGE_PROVIDER")))
	if provider == "" {
		provider = "local"
	}
	cfg := Config{
		Provider:  provider,
		Endpoint:  strings.TrimSpace(os.Getenv("EDO_OBJECT_STORAGE_ENDPOINT")),
		Region:    strings.TrimSpace(os.Getenv("EDO_OBJECT_STORAGE_REGION")),
		Bucket:    strings.TrimSpace(os.Getenv("EDO_OBJECT_STORAGE_BUCKET")),
		AccessKey: os.Getenv("EDO_OBJECT_STORAGE_ACCESS_KEY"),
		SecretKey: os.Getenv("EDO_OBJECT_STORAGE_SECRET_KEY"),
		PathStyle: envBool("EDO_OBJECT_STORAGE_PATH_STYLE", true),
		TLS:       envBool("EDO_OBJECT_STORAGE_TLS", true),
		Prefix:    strings.Trim(strings.TrimSpace(os.Getenv("EDO_OBJECT_STORAGE_PREFIX")), "/"),
		LocalRoot: localRoot,
	}
	if cfg.Provider == "local" {
		return cfg, nil
	}
	if cfg.Provider != "s3" {
		return Config{}, ErrInvalidStorageConfig
	}
	if cfg.Endpoint == "" || cfg.Region == "" || cfg.Bucket == "" || strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return Config{}, ErrInvalidStorageConfig
	}
	if err := validateEndpoint(cfg.Endpoint, cfg.TLS); err != nil {
		return Config{}, err
	}
	if err := validateBucket(cfg.Bucket); err != nil {
		return Config{}, err
	}
	if cfg.Prefix != "" {
		if err := ValidateObjectKey(cfg.Prefix); err != nil {
			return Config{}, ErrInvalidStorageConfig
		}
	}
	return cfg, nil
}

// Open returns the configured adapter. s3 never falls back to local storage.
func Open(cfg Config) (ObjectStore, error) {
	var store ObjectStore
	var err error
	switch cfg.Provider {
	case "local":
		store, err = NewLocalObjectStore(cfg.LocalRoot)
	case "s3":
		store, err = NewS3ObjectStore(cfg)
	default:
		return nil, ErrInvalidStorageConfig
	}
	if err != nil {
		return nil, err
	}
	ensureMetrics()
	return store, nil
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func validateEndpoint(raw string, tlsRequired bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ErrInvalidStorageConfig
	}
	host := parsed.Hostname()
	if host == "" || isMetadataHost(host) {
		return ErrInvalidStorageConfig
	}
	if tlsRequired {
		if parsed.Scheme != "https" || isBlockedIP(host, true) {
			return ErrInvalidStorageConfig
		}
		return nil
	}
	if parsed.Scheme != "http" || !isLoopback(host) {
		return ErrInvalidStorageConfig
	}
	return nil
}

func validateBucket(bucket string) error {
	if len(bucket) < 3 || len(bucket) > 63 || strings.Contains(bucket, "/") || strings.Contains(bucket, "..") {
		return ErrInvalidStorageConfig
	}
	return nil
}

func isMetadataHost(host string) bool {
	switch strings.ToLower(host) {
	case "169.254.169.254", "metadata.google.internal", "metadata.internal":
		return true
	default:
		return false
	}
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isBlockedIP(host string, rejectPrivate bool) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	return rejectPrivate && (ip.IsPrivate() || ip.IsInterfaceLocalMulticast())
}

package metrics

import (
	"regexp"
	"strings"
)

var prometheusComponentSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// PrometheusNamespace converts a service identifier (for example billing-register-service)
// into a valid Prometheus namespace component.
func PrometheusNamespace(serviceName string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(serviceName), "-", "_")
	normalized = prometheusComponentSanitizer.ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		return "service"
	}
	return normalized
}

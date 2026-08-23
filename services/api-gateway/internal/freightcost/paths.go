package freightcost

import (
	"strings"
)

const (
	httpMethodGet = "GET"
)

func mapPublicToInternalPath(publicPath string) (string, bool) {
	path := strings.TrimSuffix(publicPath, "/")
	if !strings.HasPrefix(path, "/api/v1/freight-costs") {
		return "", false
	}
	suffix := strings.TrimPrefix(path, "/api/v1/freight-costs")
	if suffix == "" {
		return "/internal/v1/freight-costs/", true
	}
	return "/internal/v1/freight-costs" + suffix, true
}

func isAllowlistedPublicPath(method, path string) bool {
	if method != httpMethodGet {
		return false
	}
	path = strings.TrimSuffix(path, "/")
	if path == "/api/v1/freight-costs" {
		return true
	}
	if path == "/api/v1/freight-costs/summary" {
		return true
	}
	if path == "/api/v1/freight-costs/accessorials/summary" {
		return true
	}
	if path == "/api/v1/freight-costs/carriers/performance" {
		return true
	}
	if path == "/api/v1/freight-costs/lanes/performance" {
		return true
	}
	if path == "/api/v1/freight-costs/analytics/overview" {
		return true
	}
	if path == "/api/v1/freight-costs/analytics/lanes" {
		return true
	}
	if path == "/api/v1/freight-costs/analytics/carriers" {
		return true
	}
	if path == "/api/v1/freight-costs/analytics/accessorials" {
		return true
	}
	if path == "/api/v1/freight-costs/opportunities" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/freight-costs/transport-orders/") {
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/freight-costs/transport-orders/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			return true
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "variance-detail" {
			return true
		}
	}
	return false
}

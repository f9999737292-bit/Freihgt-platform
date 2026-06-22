package handlers

import (
	"net/http"
	"strings"
)

const tenantHeader = "X-Tenant-ID"

func tenantIDFromRequest(r *http.Request) string {
	if tenantID := strings.TrimSpace(r.Header.Get(tenantHeader)); tenantID != "" {
		return tenantID
	}
	return strings.TrimSpace(r.URL.Query().Get("tenant_id"))
}

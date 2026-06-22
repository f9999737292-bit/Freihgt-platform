package handlers

import (
	"net/http"
	"strings"

	sharedlowcode "github.com/freight-platform/shared-go/lowcode"
)

const tenantHeader = sharedlowcode.HeaderTenantID

func tenantIDFromRequest(r *http.Request) string {
	if tenantID := sharedlowcode.TenantIDFromHeader(r.Header); tenantID != "" {
		return tenantID
	}
	return strings.TrimSpace(r.URL.Query().Get("tenant_id"))
}

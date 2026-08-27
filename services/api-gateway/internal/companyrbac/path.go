package companyrbac

import (
	"strings"

	"github.com/google/uuid"
)

// ParseTargetCompanyID extracts a company UUID from /api/v1/companies/{id} paths.
func ParseTargetCompanyID(path string) (uuid.UUID, bool) {
	const prefix = "/api/v1/companies/"
	if !strings.HasPrefix(path, prefix) {
		return uuid.Nil, false
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return uuid.Nil, false
	}
	segment := rest
	if idx := strings.Index(rest, "/"); idx >= 0 {
		segment = rest[:idx]
	}
	id, err := uuid.Parse(strings.TrimSpace(segment))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

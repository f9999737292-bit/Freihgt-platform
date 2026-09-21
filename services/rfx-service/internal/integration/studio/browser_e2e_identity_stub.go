//go:build integration

package studio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type browserIdentityStub struct {
	server            *httptest.Server
	rolesByUser       map[string][]string
	membershipsByUser map[string][]map[string]any
}

func startBrowserIdentityStub(t *testing.T, rolesByUser map[string][]string) *browserIdentityStub {
	t.Helper()
	return startBrowserIdentityStubWithMemberships(t, rolesByUser, nil)
}

func startBrowserIdentityStubWithMemberships(
	t *testing.T,
	rolesByUser map[string][]string,
	membershipsByUser map[string][]map[string]any,
) *browserIdentityStub {
	t.Helper()
	if rolesByUser == nil {
		rolesByUser = map[string][]string{}
	}
	if membershipsByUser == nil {
		membershipsByUser = map[string][]map[string]any{}
	}
	stub := &browserIdentityStub{rolesByUser: rolesByUser, membershipsByUser: membershipsByUser}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/me"):
			userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
			roles := stub.rolesByUser[userID]
			if roles == nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": roles})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/users/") && strings.HasSuffix(r.URL.Path, "/companies"):
			// Production gateway proxies GET /api/v1/users/{id}/companies to identity-service.
			userID := extractIdentityStubUserID(r.URL.Path)
			items := stub.membershipsByUser[userID]
			if items == nil {
				items = []map[string]any{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": len(items), "limit": 200, "offset": 0})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func extractIdentityStubUserID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "users" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func (s *browserIdentityStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func browserIdentityRolesForBuyer(userID string) map[string][]string {
	return map[string][]string{
		userID: {"PROCUREMENT_MANAGER"},
	}
}

func browserIdentityRolesForCarrier(userID string) map[string][]string {
	return map[string][]string{
		userID: {"CARRIER_DISPATCHER"},
	}
}

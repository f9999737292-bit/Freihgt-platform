//go:build integration

package carrierresponse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type browserIdentityStub struct {
	server           *httptest.Server
	rolesByUser      map[string][]string
	membershipsByUser map[string][]map[string]any
}

func startBrowserIdentityStub(t *testing.T, rolesByUser map[string][]string, membershipsByUser map[string][]map[string]any) *browserIdentityStub {
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
		case strings.Contains(r.URL.Path, "/users/") && strings.HasSuffix(r.URL.Path, "/companies"):
			userID := extractBrowserStubUserID(r.URL.Path)
			items := stub.membershipsByUser[userID]
			if items == nil {
				items = []map[string]any{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func extractBrowserStubUserID(path string) string {
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

func browserIdentityMembershipForCarrier(userID, companyID, legalName string) map[string][]map[string]any {
	return map[string][]map[string]any{
		userID: {
			{
				"membership_id":      companyID + "-membership",
				"company_id":         companyID,
				"legal_name":         legalName,
				"company_type":       "CARRIER",
				"membership_status":  "ACTIVE",
				"roles":              []map[string]string{{"role_id": "role-carrier", "code": "CARRIER_DISPATCHER", "name": "Carrier Dispatcher"}},
			},
		},
	}
}

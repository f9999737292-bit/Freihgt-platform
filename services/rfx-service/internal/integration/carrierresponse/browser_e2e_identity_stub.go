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
	server      *httptest.Server
	rolesByUser map[string][]string
}

func startBrowserIdentityStub(t *testing.T, rolesByUser map[string][]string) *browserIdentityStub {
	t.Helper()
	if rolesByUser == nil {
		rolesByUser = map[string][]string{}
	}
	stub := &browserIdentityStub{rolesByUser: rolesByUser}
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
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
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

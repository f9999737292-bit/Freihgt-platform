//go:build integration

package contractratepublic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	crtestkit "github.com/freight-platform/contract-rate-service/testkit"
	gwconfig "github.com/freight-platform/api-gateway/internal/config"
)

const internalToken = crtestkit.DefaultInternalToken

type identityMembership struct {
	CompanyID   uuid.UUID
	CompanyType string
	Roles       []string
}

type harness struct {
	pool      *pgxpool.Pool
	tenantID  uuid.UUID
	userID    uuid.UUID
	buyerID   uuid.UUID
	carrierID uuid.UUID
	originID  uuid.UUID
	destID    uuid.UUID
	gateway   http.Handler
	jwtSecret string
}

type harnessOptions struct {
	Pool              *pgxpool.Pool
	TenantID          uuid.UUID
	UserID            uuid.UUID
	BuyerID           uuid.UUID
	CarrierID         uuid.UUID
	OriginID          uuid.UUID
	DestID            uuid.UUID
	Memberships       []identityMembership
	MembershipsByUser map[uuid.UUID][]identityMembership
}

func newHarness(t *testing.T, opts harnessOptions) *harness {
	t.Helper()
	pool := opts.Pool
	if pool == nil {
		pool = crtestkit.SetupPool(t)
	}
	ctx := t.Context()

	if opts.TenantID == uuid.Nil {
		opts.TenantID = uuid.New()
	}
	if opts.UserID == uuid.Nil {
		opts.UserID = uuid.New()
	}
	if opts.BuyerID == uuid.Nil {
		opts.BuyerID = uuid.New()
	}
	if opts.CarrierID == uuid.Nil {
		opts.CarrierID = uuid.New()
	}
	if opts.OriginID == uuid.Nil {
		opts.OriginID = uuid.New()
	}
	if opts.DestID == uuid.Nil {
		opts.DestID = uuid.New()
	}

	crtestkit.SeedTenantAndCompanies(t, ctx, pool, opts.TenantID, opts.BuyerID, opts.CarrierID)
	crtestkit.SeedLocations(t, ctx, pool, opts.TenantID, opts.BuyerID, opts.OriginID, opts.DestID)

	membershipsByUser := opts.MembershipsByUser
	if membershipsByUser == nil {
		membershipsByUser = map[uuid.UUID][]identityMembership{}
	}
	if len(opts.Memberships) > 0 {
		membershipsByUser[opts.UserID] = opts.Memberships
	}
	if len(membershipsByUser) == 0 {
		crtestkit.SeedCompanyRole(t, ctx, pool, opts.TenantID, opts.UserID, opts.BuyerID, "PROCUREMENT_MANAGER")
		membershipsByUser[opts.UserID] = []identityMembership{{
			CompanyID: opts.BuyerID, CompanyType: "SHIPPER", Roles: []string{"PROCUREMENT_MANAGER"},
		}}
	}
	for userID, memberships := range membershipsByUser {
		for _, m := range memberships {
			for _, role := range m.Roles {
				crtestkit.SeedCompanyRole(t, ctx, pool, opts.TenantID, userID, m.CompanyID, role)
			}
		}
	}

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/companies"):
			userID := extractPathUserID(r.URL.Path)
			memberships := membershipsByUser[userID]
			items := make([]map[string]any, 0, len(memberships))
			for _, m := range memberships {
				roles := make([]map[string]any, 0, len(m.Roles))
				for _, role := range m.Roles {
					roles = append(roles, map[string]any{"code": role})
				}
				items = append(items, map[string]any{
					"company_id":   m.CompanyID.String(),
					"company_type": m.CompanyType,
					"roles":        roles,
				})
			}
			writeJSON(w, map[string]any{"items": items})
		case strings.HasSuffix(r.URL.Path, "/roles"):
			writeJSON(w, map[string]any{"items": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(identityServer.Close)

	crServer := crtestkit.StartServer(t, crtestkit.ServerOptions{
		Pool:                 pool,
		InternalServiceToken: internalToken,
	})

	cfg := gwconfig.Config{
		AuthEnabled:          true,
		JWTSecret:            "integration-jwt-secret",
		ProxyTimeoutSeconds:  15,
		InternalServiceToken: internalToken,
		Services: gwconfig.ServiceURLs{
			Identity:     identityServer.URL,
			ContractRate: crServer.URL,
		},
	}
	router := newTestGateway(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	return &harness{
		pool: pool, tenantID: opts.TenantID, userID: opts.UserID,
		buyerID: opts.BuyerID, carrierID: opts.CarrierID,
		originID: opts.OriginID, destID: opts.DestID,
		gateway: router, jwtSecret: cfg.JWTSecret,
	}
}

func (h *harness) tokenFor(userID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": h.tenantID.String(),
		"email":     "user@example.test",
		"sub":       userID.String(),
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(h.jwtSecret))
	return signed
}

type apiResponse struct {
	Status int
	Body   []byte
}

func (h *harness) request(userID uuid.UUID, companyID uuid.UUID, method, path string, body any, extraHeaders map[string]string) apiResponse {
	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			reader = bytes.NewReader(v)
		case string:
			reader = strings.NewReader(v)
		default:
			raw, _ := json.Marshal(v)
			reader = bytes.NewReader(raw)
		}
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+h.tokenFor(userID))
	req.Header.Set("X-Company-ID", companyID.String())
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.gateway.ServeHTTP(rec, req)
	return apiResponse{Status: rec.Code, Body: rec.Body.Bytes()}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func parseJSONField(t *testing.T, body []byte, field string) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse json: %v body=%s", err, string(body))
	}
	raw, ok := payload[field]
	if !ok || raw == nil {
		t.Fatalf("field %q missing in %s", field, string(body))
	}
	return fmt.Sprint(raw)
}

func mustStatus(t *testing.T, label string, resp apiResponse, want int) {
	t.Helper()
	if resp.Status != want {
		t.Fatalf("%s expected %d got %d body=%s", label, want, resp.Status, string(resp.Body))
	}
}

func extractPathUserID(path string) uuid.UUID {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "users" && i+1 < len(parts) {
			id, _ := uuid.Parse(parts[i+1])
			return id
		}
	}
	return uuid.Nil
}

//go:build integration

package freightcostpublic

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	gwconfig "github.com/freight-platform/api-gateway/internal/config"
)

const internalToken = "freight-cost-integration-token"

type identityMembership struct {
	CompanyID   uuid.UUID
	CompanyType string
	Roles       []string
}

type downstreamCapture struct {
	mu           sync.Mutex
	TenantID     string
	UserID       string
	CompanyID    string
	ActorKind    string
	RequestID    string
	ServiceToken string
}

type harness struct {
	tenantID        uuid.UUID
	userID          uuid.UUID
	buyerID         uuid.UUID
	carrierID       uuid.UUID
	carrierUserID   uuid.UUID
	resourceTenant  uuid.UUID
	orderID         uuid.UUID
	gateway         http.Handler
	jwtSecret       string
	downstream      *downstreamCapture
	freightCostDown *httptest.Server
}

type harnessOptions struct {
	TenantID           uuid.UUID
	UserID             uuid.UUID
	BuyerID            uuid.UUID
	CarrierID          uuid.UUID
	CarrierUserID      uuid.UUID
	ResourceTenantID   uuid.UUID
	BuyerMemberships   []identityMembership
	CarrierMemberships []identityMembership
	FreightCostDown    *httptest.Server
}

func newHarness(t *testing.T, opts harnessOptions) *harness {
	t.Helper()

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
	if opts.CarrierUserID == uuid.Nil {
		opts.CarrierUserID = uuid.New()
	}
	if opts.ResourceTenantID == uuid.Nil {
		opts.ResourceTenantID = opts.TenantID
	}
	if len(opts.BuyerMemberships) == 0 {
		opts.BuyerMemberships = []identityMembership{{
			CompanyID: opts.BuyerID, CompanyType: "SHIPPER", Roles: []string{"PROCUREMENT_MANAGER"},
		}}
	}
	if len(opts.CarrierMemberships) == 0 {
		opts.CarrierMemberships = []identityMembership{{
			CompanyID: opts.CarrierID, CompanyType: "CARRIER", Roles: []string{"CARRIER_ADMIN"},
		}}
	}

	capture := &downstreamCapture{}
	orderID := uuid.New()

	freightCostServer := opts.FreightCostDown
	if freightCostServer == nil {
		freightCostServer = httptest.NewServer(mockFreightCostHandler(t, capture, opts.ResourceTenantID, orderID))
		t.Cleanup(freightCostServer.Close)
	}

	membershipsByUser := map[uuid.UUID][]identityMembership{
		opts.UserID:        opts.BuyerMemberships,
		opts.CarrierUserID: opts.CarrierMemberships,
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

	cfg := gwconfig.Config{
		AuthEnabled:          true,
		JWTSecret:            "integration-jwt-secret",
		ProxyTimeoutSeconds:  15,
		InternalServiceToken: internalToken,
		Services: gwconfig.ServiceURLs{
			Identity:    identityServer.URL,
			FreightCost: freightCostServer.URL,
		},
	}
	router := newTestGateway(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	return &harness{
		tenantID:        opts.TenantID,
		userID:          opts.UserID,
		buyerID:         opts.BuyerID,
		carrierID:       opts.CarrierID,
		carrierUserID:   opts.CarrierUserID,
		resourceTenant:  opts.ResourceTenantID,
		orderID:         orderID,
		gateway:         router,
		jwtSecret:       cfg.JWTSecret,
		downstream:      capture,
		freightCostDown: freightCostServer,
	}
}

func mockFreightCostHandler(t *testing.T, capture *downstreamCapture, resourceTenant, orderID uuid.UUID) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.mu.Lock()
		capture.TenantID = r.Header.Get("X-Tenant-ID")
		capture.UserID = r.Header.Get("X-User-ID")
		capture.CompanyID = r.Header.Get("X-Company-ID")
		capture.ActorKind = r.Header.Get("X-Actor-Kind")
		capture.RequestID = r.Header.Get("X-Request-ID")
		capture.ServiceToken = r.Header.Get("X-Internal-Service-Token")
		capture.mu.Unlock()

		if r.Header.Get("X-Internal-Service-Token") != internalToken {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "internal service authentication failed"},
			})
			return
		}

		tenantHeader := r.Header.Get("X-Tenant-ID")
		tenantID, err := uuid.Parse(tenantHeader)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if tenantID != resourceTenant {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "freight cost resource not found"},
			})
			return
		}

		actorKind := strings.ToUpper(strings.TrimSpace(r.Header.Get("X-Actor-Kind")))
		path := strings.TrimSuffix(r.URL.Path, "/")

		switch {
		case path == "/internal/v1/freight-costs":
			writeJSON(w, listResponse(actorKind, orderID, r.URL.Query().Get("limit")))
		case path == "/internal/v1/freight-costs/summary":
			writeJSON(w, summaryResponse(actorKind))
		case strings.HasSuffix(path, "/variance-detail"):
			if actorKind == "CARRIER" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			writeJSON(w, map[string]any{
				"transport_order_id":      orderID.String(),
				"variance_drivers":          []any{},
				"reconciliation_findings": []any{},
			})
		case strings.HasSuffix(path, "/accessorials/summary"):
			if actorKind == "CARRIER" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			writeJSON(w, map[string]any{
				"items":           []any{},
				"currency_code":   "RUB",
				"data_capability": "NOT_AVAILABLE",
			})
		case strings.HasSuffix(path, "/lanes/performance"):
			writeJSON(w, map[string]any{
				"items":           []any{},
				"currency_code":   "RUB",
				"data_capability": "NOT_AVAILABLE",
			})
		case strings.HasSuffix(path, "/carriers/performance"):
			writeJSON(w, map[string]any{"items": []any{}, "currency_code": "RUB"})
		case strings.Contains(path, "/transport-orders/"):
			writeJSON(w, detailResponse(actorKind, orderID))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func listResponse(actorKind string, orderID uuid.UUID, limitRaw string) map[string]any {
	item := map[string]any{
		"transport_order_id":            orderID.String(),
		"buyer_company_id":              uuid.NewString(),
		"carrier_company_id":            uuid.NewString(),
		"currency_code":                 "RUB",
		"data_stage":                    "ACCRUAL_AVAILABLE",
		"financial_finality":            "CURRENT_ACTUAL",
		"sources_available":             []string{"PLANNED"},
		"planned_amount":                "1000.00",
		"current_actual_amount":         "950.00",
		"forecast_source_status":        "UNKNOWN",
		"billing_reconciliation_status": "MATCH",
		"cost_updated_at":               time.Now().UTC().Format(time.RFC3339),
	}
	if actorKind != "CARRIER" {
		item["accrued_amount"] = "900.00"
		item["forecast_exposure"] = "50.00"
		item["current_variance_amount"] = "50.00"
	}
	limit := 50
	if limitRaw != "" {
		if parsed, err := strconv.Atoi(limitRaw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return map[string]any{
		"items":  []any{item},
		"total":  1,
		"limit":  limit,
		"offset": 0,
	}
}

func summaryResponse(actorKind string) map[string]any {
	kpis := map[string]any{
		"planned_total":        "1000.00",
		"current_actual_total": "950.00",
		"final_actual_total":   "950.00",
	}
	if actorKind != "CARRIER" {
		kpis["accrued_total"] = "900.00"
		kpis["forecast_exposure_total"] = "50.00"
		kpis["current_variance_total"] = "50.00"
		kpis["reconciliation_mismatch_count"] = 0
	}
	return map[string]any{
		"currency_code": "RUB",
		"period": map[string]any{
			"from": "", "to": "", "date_dimension": "TRANSPORT_ORDER_CREATED_AT",
		},
		"kpis":           kpis,
		"mixed_currency": false,
	}
}

func detailResponse(actorKind string, orderID uuid.UUID) map[string]any {
	summary := listResponse(actorKind, orderID, "")["items"].([]any)[0].(map[string]any)
	return map[string]any{
		"summary":                 summary,
		"order_reference":         "",
		"carrier_name":            "",
		"planned_source":          nil,
		"variance_drivers":        []any{},
		"reconciliation_findings": []any{},
	}
}

func (h *harness) tokenFor(userID uuid.UUID, tenantID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
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

func (h *harness) request(userID, tenantID, companyID uuid.UUID, method, path string, extraHeaders map[string]string) apiResponse {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+h.tokenFor(userID, tenantID))
	req.Header.Set("X-Company-ID", companyID.String())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.gateway.ServeHTTP(rec, req)
	return apiResponse{Status: rec.Code, Body: rec.Body.Bytes()}
}

func (h *harness) lastDownstream() downstreamCapture {
	h.downstream.mu.Lock()
	defer h.downstream.mu.Unlock()
	return *h.downstream
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

func jsonHasKey(body []byte, key string) bool {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	_, ok := payload[key]
	return ok
}

func nestedJSONHasKey(body []byte, container, key string) bool {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	raw, ok := payload[container]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case map[string]any:
		_, ok := v[key]
		return ok
	case []any:
		if len(v) == 0 {
			return false
		}
		if item, ok := v[0].(map[string]any); ok {
			_, ok := item[key]
			return ok
		}
	}
	return false
}

func requestWithoutAuth(h *harness, companyID uuid.UUID, path string) apiResponse {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Company-ID", companyID.String())
	rec := httptest.NewRecorder()
	h.gateway.ServeHTTP(rec, req)
	return apiResponse{Status: rec.Code, Body: rec.Body.Bytes()}
}

func mustNotContainSQLLeak(t *testing.T, body []byte) {
	t.Helper()
	lower := strings.ToLower(string(body))
	for _, needle := range []string{"select ", "postgres://", "password=", "stack trace", internalToken} {
		if strings.Contains(lower, needle) {
			t.Fatalf("response leaked internal detail: %s", string(body))
		}
	}
}

func closedFreightCostServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	return server
}

//go:build integration

package systemwave2

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/rfxrbac"
	"github.com/freight-platform/api-gateway/internal/transportorderrbac"
)

// TestW2_OrderToRfxBootstrap proves buyer can create priced transport order and RFx freight request
// through gateway guards with verified company/actor context (Wave 2 blocked path).
func TestW2_OrderToRfxBootstrap(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyID := uuid.New().String()
	orderID := uuid.New().String()
	secret := "bootstrap-secret"

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"SHIPPER_ADMIN"}})
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"company_id": companyID, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_ADMIN"}}},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/roles"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identityServer.Close()

	var toCompany, toActor string
	transportOrderBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/transport-orders" {
			t.Fatalf("unexpected transport-order path: %s %s", r.Method, r.URL.Path)
		}
		toCompany = r.Header.Get(companycontext.HeaderCompanyID)
		toActor = r.Header.Get(companycontext.HeaderActorKind)
		if toCompany == "" || toActor == "" {
			t.Fatalf("transport-order missing actor context: company=%q actor=%q", toCompany, toActor)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"`+orderID+`","order_number":"TO-W2-BOOT"}`)
	}))
	defer transportOrderBackend.Close()

	rfxBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/freight-requests/from-transport-order" {
			t.Fatalf("unexpected rfx path: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), orderID) {
			t.Fatalf("rfx body missing transport order id: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"`+uuid.New().String()+`","transport_order_id":"`+orderID+`"}`)
	}))
	defer rfxBackend.Close()

	cfg := config.Config{
		AuthEnabled:         true,
		JWTSecret:           secret,
		ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{
			Identity:       identityServer.URL,
			TransportOrder: transportOrderBackend.URL,
			RFX:            rfxBackend.URL,
		},
	}

	proxyTo := func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, transportOrderBackend.URL+r.URL.Path[len("/api"):], r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
	proxyRfx := func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, rfxBackend.URL+r.URL.Path[len("/api"):], r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}

	r := chi.NewRouter()
	r.Use(gwmiddleware.Auth(cfg.AuthEnabled, cfg.JWTSecret))
	transportOrderGuard := transportorderrbac.NewGuard(cfg, http.HandlerFunc(proxyTo))
	rfxGuard := rfxrbac.NewGuard(cfg, http.HandlerFunc(proxyRfx))
	r.Post("/api/v1/transport-orders", transportOrderGuard.WithPolicy(transportorderrbac.PolicyCreate))
	r.Post("/api/v1/freight-requests/from-transport-order", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))

	gateway := httptest.NewServer(r)
	defer gateway.Close()

	token := signBootstrapToken(t, secret, userID, tenantID)
	client := gateway.Client()

	toBody := `{"order_number":"TO-W2-BOOT","pricing_context":{"carrier_company_id":"` + uuid.New().String() + `","pricing_source":"CONTRACT"}}`
	toReq, _ := http.NewRequest(http.MethodPost, gateway.URL+"/api/v1/transport-orders", strings.NewReader(toBody))
	toReq.Header.Set("Authorization", "Bearer "+token)
	toReq.Header.Set("Content-Type", "application/json")
	toReq.Header.Set("X-Company-ID", companyID)
	toReq.Header.Set("Idempotency-Key", "w2-bootstrap-idem")
	toResp, err := client.Do(toReq)
	if err != nil {
		t.Fatalf("transport order request: %v", err)
	}
	defer toResp.Body.Close()
	if toResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(toResp.Body)
		t.Fatalf("transport order status=%d body=%s", toResp.StatusCode, string(b))
	}
	if toCompany != companyID || toActor != companycontext.ActorBuyer {
		t.Fatalf("transport-order actor context company=%q actor=%q", toCompany, toActor)
	}

	frBody := `{"transport_order_id":"` + orderID + `","freight_request_number":"FR-W2-BOOT","request_type":"MINI_TENDER"}`
	frReq, _ := http.NewRequest(http.MethodPost, gateway.URL+"/api/v1/freight-requests/from-transport-order", strings.NewReader(frBody))
	frReq.Header.Set("Authorization", "Bearer "+token)
	frReq.Header.Set("Content-Type", "application/json")
	frReq.Header.Set("X-Company-ID", companyID)
	frResp, err := client.Do(frReq)
	if err != nil {
		t.Fatalf("freight request request: %v", err)
	}
	defer frResp.Body.Close()
	if frResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(frResp.Body)
		t.Fatalf("freight request status=%d body=%s", frResp.StatusCode, string(b))
	}
}

func signBootstrapToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     "buyer@wave2.test",
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

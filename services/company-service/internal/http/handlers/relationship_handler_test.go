package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/config"
	"github.com/freight-platform/company-service/internal/domain"
	httpserver "github.com/freight-platform/company-service/internal/http"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/security"
	"github.com/freight-platform/company-service/internal/service"
)

const (
	testSecret        = "relationship-test-secret"
	testPepper        = "relationship-test-pepper"
	shipmentTestToken = "shipment-relationship-test-token"
	rfxTestToken      = "rfx-relationship-test-token"
)

type recordingAuthority struct {
	byUser     map[uuid.UUID]domain.ActorAuthority
	seen       uuid.UUID
	userActive bool
}

func (a *recordingAuthority) LoadRelationshipActor(_ context.Context, _, userID, _ uuid.UUID) (domain.ActorAuthority, error) {
	a.seen = userID
	return a.byUser[userID], nil
}

func (a *recordingAuthority) ActiveUserInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return a.userActive, nil
}

func (a *recordingAuthority) ActiveCompanyInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func TestRelationshipHTTPSecurity(t *testing.T) {
	logBuf := &bytes.Buffer{}
	userID := uuid.New()
	tenantID := uuid.New()
	otherUser := uuid.New()
	companyID := uuid.New()
	carrierID := uuid.New()
	sourceID := uuid.New()
	authority := &recordingAuthority{byUser: map[uuid.UUID]domain.ActorAuthority{
		userID: {Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperAdmin, domain.RoleCarrierAdmin}},
	}}
	store := &recordingStore{carrier: true}
	svc := service.NewRelationshipService(store, authority, service.RelationshipConfig{
		CommandsEnabled: true, TokenPepper: testPepper, ShipmentServiceToken: shipmentTestToken, RfxServiceToken: rfxTestToken,
	}, nil, allowSource{sourceID: sourceID, tenantID: tenantID, userID: userID, companyID: companyID, carrierID: carrierID, fingerprint: "v1"})
	router := httpserver.NewRouter(slog.New(slog.NewJSONHandler(logBuf, nil)), nil, config.Config{
		JWTSecret: testSecret, RelationshipTokenPepper: testPepper, RelationshipCommandsEnabled: true, InternalServiceToken: "shared-token",
	}, nil, nil, nil, nil, svc)

	token := sign(t, userID, tenantID)
	body := createBody(userID, tenantID, companyID, sourceID, carrierID, domain.SourceTypeShipment, "same-key", "v1")
	first := doCreate(t, router, shipmentTestToken, token, companyID, body, otherUser)
	if first.Code != http.StatusCreated || authority.seen != userID {
		t.Fatalf("create status=%d seen=%s body=%s", first.Code, authority.seen, first.Body.String())
	}
	var created struct {
		AcceptanceToken string `json:"acceptance_token"`
		RequestID       string `json:"relationship_request_id"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil || created.AcceptanceToken == "" {
		t.Fatalf("token missing: %v %s", err, first.Body.String())
	}
	replay := doCreate(t, router, shipmentTestToken, token, companyID, body, uuid.Nil)
	if replay.Code != http.StatusOK || strings.Contains(replay.Body.String(), created.AcceptanceToken) {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	if store.lastCaller != domain.CallerShipmentService {
		t.Fatalf("stored caller %s", store.lastCaller)
	}
	if strings.Contains(logBuf.String(), created.AcceptanceToken) || strings.Contains(logBuf.String(), testPepper) || strings.Contains(logBuf.String(), shipmentTestToken) {
		t.Fatal("token, pepper, or service credential leaked into logs")
	}

	userOnly := doJSON(t, router, http.MethodPost, "/v1/carrier-relationship-requests", token, companyID, body, otherUser)
	if userOnly.Code != http.StatusUnauthorized || !strings.Contains(userOnly.Body.String(), "AUTHENTICATION_REQUIRED") || strings.Contains(userOnly.Body.String(), "acceptance_token") {
		t.Fatalf("user jwt without service credential: %d %s", userOnly.Code, userOnly.Body.String())
	}
	spoofName := httptest.NewRequest(http.MethodPost, "/v1/carrier-relationship-requests", bytes.NewReader(body))
	spoofName.Header.Set("Authorization", "Bearer "+token)
	spoofName.Header.Set("X-Company-ID", companyID.String())
	spoofName.Header.Set("X-Internal-Service-Token", "shared-token")
	spoofName.Header.Set("X-Internal-Service-Name", domain.CallerShipmentService)
	persisted := store.creates
	spoofed := doRaw(t, router, spoofName)
	if spoofed.Code != http.StatusUnauthorized || store.creates != persisted {
		t.Fatalf("shared token and service name were accepted: %d creates=%d", spoofed.Code, store.creates)
	}
	rfxBody := createBody(userID, tenantID, companyID, sourceID, carrierID, domain.SourceTypeRfxAward, "rfx-key", "v1")
	cross := doCreate(t, router, shipmentTestToken, token, companyID, rfxBody, uuid.Nil)
	if cross.Code != http.StatusForbidden || store.creates != persisted {
		t.Fatalf("shipment credential created an rfx request: %d creates=%d", cross.Code, store.creates)
	}
	reverseBody := createBody(userID, tenantID, companyID, sourceID, carrierID, domain.SourceTypeShipment, "rfx-on-shipment", "v1")
	reverse := doCreate(t, router, rfxTestToken, token, companyID, reverseBody, uuid.Nil)
	if reverse.Code != http.StatusForbidden || store.creates != persisted {
		t.Fatalf("rfx credential created a shipment request: %d creates=%d", reverse.Code, store.creates)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier-operating-relationship-acceptances", bytes.NewReader([]byte(`{"idempotency_key":"k"}`)))
	req.Header.Set("X-Company-ID", companyID.String())
	req.Header.Set("X-Internal-Service-Token", "shared-token")
	req.Header.Set("X-Internal-Service-Name", "shipment-service")
	req.Header.Set("X-User-ID", otherUser.String())
	spoofInternal := doRaw(t, router, req)
	if spoofInternal.Code != http.StatusUnauthorized {
		t.Fatalf("internal token was treated as a credential: %d", spoofInternal.Code)
	}

	denied := &recordingAuthority{byUser: map[uuid.UUID]domain.ActorAuthority{userID: {Active: true, ActiveMembership: true}}}
	deniedStore := &recordingStore{carrier: true}
	deniedSvc := service.NewRelationshipService(deniedStore, denied, service.RelationshipConfig{CommandsEnabled: true, TokenPepper: testPepper}, nil, nil)
	deniedRouter := httpserver.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, config.Config{
		JWTSecret: testSecret, RelationshipCommandsEnabled: true, RelationshipTokenPepper: testPepper,
	}, nil, nil, nil, nil, deniedSvc)
	acceptBody := []byte(`{"idempotency_key":"accept-key","acceptance_token":"` + created.AcceptanceToken + `"}`)
	ordinary := doJSON(t, deniedRouter, http.MethodPost, "/v1/carrier-operating-relationship-acceptances", token, companyID, acceptBody, uuid.Nil)
	if ordinary.Code != http.StatusForbidden || deniedStore.accepts != 0 || strings.Contains(ordinary.Body.String(), created.AcceptanceToken) || ordinary.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("ordinary accept: %d calls=%d body=%s", ordinary.Code, deniedStore.accepts, ordinary.Body.String())
	}

	notFoundBodies := []string{}
	for _, payload := range []string{
		`{"idempotency_key":"missing-token"}`,
		`{"idempotency_key":"bad-token","acceptance_token":"not-a-token"}`,
		`{"idempotency_key":"mismatch","acceptance_token":"` + created.AcceptanceToken + `","relationship_request_id":"` + uuid.NewString() + `"}`,
	} {
		rec := doJSON(t, router, http.MethodPost, "/v1/carrier-operating-relationship-acceptances", token, companyID, []byte(payload), uuid.Nil)
		if rec.Code != http.StatusNotFound || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("expected 404, got %d %s", rec.Code, rec.Body.String())
		}
		notFoundBodies = append(notFoundBodies, rec.Body.String())
	}
	if notFoundBodies[0] != notFoundBodies[1] || notFoundBodies[1] != notFoundBodies[2] {
		t.Fatalf("not-found bodies differ: %#v", notFoundBodies)
	}
	foreign := doJSON(t, router, http.MethodGet, "/v1/carrier-relationship-requests/"+uuid.NewString(), token, companyID, nil, uuid.Nil)
	if foreign.Code != http.StatusNotFound || foreign.Body.String() != notFoundBodies[0] {
		t.Fatalf("foreign get body=%s want=%s", foreign.Body.String(), notFoundBodies[0])
	}
}

func sign(t *testing.T, userID, tenantID uuid.UUID) string {
	t.Helper()
	token, err := security.SignHS256(testSecret, userID, tenantID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func createBody(userID, tenantID, companyID, sourceID, carrierID uuid.UUID, sourceType, key, fingerprint string) []byte {
	raw, _ := json.Marshal(map[string]string{
		"source_type": sourceType, "source_id": sourceID.String(), "source_tenant_id": tenantID.String(),
		"source_version_or_fingerprint": fingerprint, "shipper_carrier_company_id": carrierID.String(),
		"requesting_user_id": userID.String(), "requesting_company_id": companyID.String(), "idempotency_key": key,
	})
	return raw
}

func doCreate(t *testing.T, router http.Handler, serviceToken, userToken string, companyID uuid.UUID, body []byte, spoofUser uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier-relationship-requests", bytes.NewReader(body))
	req.Header.Set(service.RelationshipServiceTokenHeader, serviceToken)
	if userToken != "" {
		req.Header.Set("Authorization", "Bearer "+userToken)
	}
	if companyID != uuid.Nil {
		req.Header.Set("X-Company-ID", companyID.String())
	}
	if spoofUser != uuid.Nil {
		req.Header.Set("X-User-ID", spoofUser.String())
		req.Header.Set("X-Tenant-ID", uuid.NewString())
		req.Header.Set("X-User-Roles", domain.RolePlatformAdmin)
		req.Header.Set("X-Platform-Admin", "true")
		req.Header.Set("X-Internal-Service-Name", domain.CallerRfxService)
	}
	return doRaw(t, router, req)
}

func doJSON(t *testing.T, router http.Handler, method, path, token string, companyID uuid.UUID, body []byte, spoofUser uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if companyID != uuid.Nil {
		req.Header.Set("X-Company-ID", companyID.String())
	}
	if spoofUser != uuid.Nil {
		req.Header.Set("X-User-ID", spoofUser.String())
		req.Header.Set("X-Tenant-ID", uuid.NewString())
		req.Header.Set("X-User-Roles", domain.RolePlatformAdmin)
		req.Header.Set("X-Platform-Admin", "true")
	}
	return doRaw(t, router, req)
}

func doRaw(t *testing.T, router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type allowSource struct {
	sourceID, tenantID, userID, companyID, carrierID uuid.UUID
	fingerprint                                      string
}

func (a allowSource) ConfirmCreate(_ context.Context, subject domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	if subject.SourceID != a.sourceID || subject.SourceTenantID != a.tenantID || subject.RequestingUserID != a.userID || subject.RequestingCompanyID != a.companyID || subject.ShipperCarrierCompanyID != a.carrierID || subject.Fingerprint != a.fingerprint || subject.CallerService != domain.CallerShipmentService {
		return domain.SourceCreateNotFound, nil
	}
	return domain.SourceCreateConfirmed, nil
}

type recordingStore struct {
	carrier    bool
	accepts    int
	creates    int
	lists      int
	lastCaller string
	rows       map[string]domain.PersistCreateResult
	hashes     map[string][]byte
	pending    map[string]struct{}
}

func (s *recordingStore) CreateRelationshipRequest(_ context.Context, in domain.PersistCreate) (domain.PersistCreateResult, error) {
	s.creates++
	s.lastCaller = in.CallerService
	if s.rows == nil {
		s.rows = map[string]domain.PersistCreateResult{}
		s.hashes = map[string][]byte{}
		s.pending = map[string]struct{}{}
	}
	if existing, ok := s.rows[in.IdempotencyKey]; ok {
		if string(s.hashes[in.IdempotencyKey]) != string(in.RequestHash) {
			return domain.PersistCreateResult{}, apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "idempotency conflict")
		}
		return domain.PersistCreateResult{RequestID: existing.RequestID, Status: existing.Status, Created: false}, nil
	}
	created := domain.PersistCreateResult{RequestID: uuid.New(), Status: domain.RequestStatusPending, Created: true}
	s.rows[in.IdempotencyKey] = created
	s.hashes[in.IdempotencyKey] = append([]byte(nil), in.RequestHash...)
	return created, nil
}

func (s *recordingStore) AcceptRelationshipRequest(context.Context, domain.PersistAccept, domain.SourceRevalidator) (domain.PersistAcceptResult, error) {
	s.accepts++
	return domain.PersistAcceptResult{}, apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}

func (s *recordingStore) ListSourceRelationshipRequests(context.Context, string, uuid.UUID, uuid.UUID, uuid.UUID) ([]domain.SourceRequestHead, error) {
	s.lists++
	return []domain.SourceRequestHead{}, nil
}

func (s *recordingStore) CancelRelationshipRequest(context.Context, domain.PersistCancel) (domain.PersistCancelResult, error) {
	return domain.PersistCancelResult{}, apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}

func TestStatusRejectsDirectBypassBeforeList(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	companyID := uuid.New()
	sourceID := uuid.New()
	authority := &recordingAuthority{userActive: true, byUser: map[uuid.UUID]domain.ActorAuthority{
		userID: {Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleShipperAdmin}},
	}}
	store := &recordingStore{}
	svc := service.NewRelationshipService(store, authority, service.RelationshipConfig{
		CommandsEnabled: true, TokenPepper: testPepper, ShipmentServiceToken: shipmentTestToken, RfxServiceToken: rfxTestToken,
	}, nil, allowSource{})
	router := httpserver.NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), nil, config.Config{
		JWTSecret: testSecret, RelationshipCommandsEnabled: true,
	}, nil, nil, nil, nil, svc)
	post := func(token string, body string, roles bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/carrier-relationship-request-statuses", strings.NewReader(body))
		if token != "" {
			req.Header.Set(service.RelationshipServiceTokenHeader, token)
		}
		req.Header.Set("X-User-ID", uuid.NewString())
		req.Header.Set("X-Tenant-ID", uuid.NewString())
		req.Header.Set("X-Company-ID", uuid.NewString())
		if roles {
			req.Header.Set("X-User-Roles", "SHIPPER_ADMIN,PLATFORM_ADMIN")
			req.Header.Set("X-Platform-Admin", "true")
		}
		return doRaw(t, router, req)
	}
	missing := post(shipmentTestToken, `{"source_type":"SHIPMENT","source_id":"`+sourceID.String()+`","source_tenant_id":"`+tenantID.String()+`","requesting_user_id":"`+userID.String()+`"}`, true)
	if missing.Code != http.StatusBadRequest || store.lists != 0 {
		t.Fatalf("missing company %d lists=%d %s", missing.Code, store.lists, missing.Body.String())
	}
	authority.userActive = false
	inactive := post(shipmentTestToken, statusBody(sourceID, tenantID, userID, companyID, domain.SourceTypeShipment), true)
	if inactive.Code != http.StatusForbidden || store.lists != 0 || strings.Contains(inactive.Body.String(), sourceID.String()) {
		t.Fatalf("inactive %d lists=%d %s", inactive.Code, store.lists, inactive.Body.String())
	}
	authority.userActive = true
	wrong := post(rfxTestToken, statusBody(sourceID, tenantID, userID, companyID, domain.SourceTypeShipment), true)
	if wrong.Code != http.StatusForbidden || store.lists != 0 {
		t.Fatalf("rfx credential %d lists=%d %s", wrong.Code, store.lists, wrong.Body.String())
	}
	none := post("", statusBody(sourceID, tenantID, userID, companyID, domain.SourceTypeShipment), true)
	if none.Code != http.StatusUnauthorized || store.lists != 0 {
		t.Fatalf("missing token %d lists=%d %s", none.Code, store.lists, none.Body.String())
	}
	allowed := post(shipmentTestToken, statusBody(sourceID, tenantID, userID, companyID, domain.SourceTypeShipment), false)
	if allowed.Code != http.StatusOK || store.lists != 1 || strings.Contains(allowed.Body.String(), "acceptance_token") || allowed.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authorized %d lists=%d %s", allowed.Code, store.lists, allowed.Body.String())
	}
}

func statusBody(sourceID, tenantID, userID, companyID uuid.UUID, sourceType string) string {
	return `{"source_type":"` + sourceType + `","source_id":"` + sourceID.String() + `","source_tenant_id":"` + tenantID.String() + `","requesting_user_id":"` + userID.String() + `","requesting_company_id":"` + companyID.String() + `"}`
}

func (s *recordingStore) GetVisibleRelationshipRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.VisibleRequest, error) {
	return domain.VisibleRequest{}, apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}

type fixedAcceptStore struct {
	recordingStore
	result domain.PersistAcceptResult
}

func (s *fixedAcceptStore) AcceptRelationshipRequest(context.Context, domain.PersistAccept, domain.SourceRevalidator) (domain.PersistAcceptResult, error) {
	s.accepts++
	return s.result, nil
}

func TestAcceptSuccessSetsNoStoreAndOmitsToken(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	companyID := uuid.New()
	relationshipID := uuid.New()
	authority := &recordingAuthority{byUser: map[uuid.UUID]domain.ActorAuthority{
		userID: {Active: true, ActiveMembership: true, CompanyRoles: []string{domain.RoleCarrierAdmin}},
	}}
	store := &fixedAcceptStore{result: domain.PersistAcceptResult{
		RelationshipID: relationshipID, RelationshipVersion: 1, RequestStatus: domain.RequestStatusAccepted, Created: true,
	}}
	svc := service.NewRelationshipService(store, authority, service.RelationshipConfig{
		CommandsEnabled: true, TokenPepper: testPepper,
	}, nil, nil)
	router := httpserver.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, config.Config{
		JWTSecret: testSecret, RelationshipCommandsEnabled: true, RelationshipTokenPepper: testPepper,
	}, nil, nil, nil, nil, svc)
	token := sign(t, userID, tenantID)
	rec := doJSON(t, router, http.MethodPost, "/v1/carrier-operating-relationship-acceptances", token, companyID, []byte(`{"idempotency_key":"accept-key"}`), uuid.New())
	if rec.Code != http.StatusCreated || rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), "acceptance_token") {
		t.Fatalf("accept status=%d cache=%q body=%s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	if store.accepts != 1 || authority.seen != userID {
		t.Fatalf("accept calls=%d seen=%s", store.accepts, authority.seen)
	}
}

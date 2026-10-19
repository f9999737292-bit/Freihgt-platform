//go:build integration

package sourceauthority

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	rfxhttp "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/security"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

func TestRfxRelationshipCreatePostgres(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	var pgVersion, dbName string
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_database()`).Scan(&pgVersion, &dbName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbName, "rfx_src_") {
		t.Fatalf("refusing non-isolated database %s", dbName)
	}
	t.Logf("POSTGRES_VERSION=%s", pgVersion)
	t.Logf("TEST_DATABASE_ISOLATED=%s", dbName)

	fix := seedSource(t, pool)
	const (
		jwtSecret           = "rfx-relationship-jwt-secret"
		revalidationToken   = "dedicated-revalidation-token"
		rfxToken            = "dedicated-rfx-relationship-token"
		shipmentToken       = "dedicated-shipment-relationship-token"
		sharedInternalToken = "shared-internal-token"
	)
	repo := repository.NewRfxSourceAuthorityRepository(pool)
	confirmServer := httptest.NewServer(rfxhttp.WithRfxSourceAuthority(http.NotFoundHandler(), revalidationToken, service.NewRfxSourceAuthority(repo)))
	t.Cleanup(confirmServer.Close)
	companyURL := startRfxCompanyRelationshipProcess(t, pool, confirmServer.URL, revalidationToken, rfxToken, shipmentToken, sharedInternalToken)
	initiator := service.NewRfxRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient(companyURL, rfxToken, nil))
	router := rfxhttp.WithRfxRelationshipCreate(http.NotFoundHandler(), jwtSecret, initiator)

	before := sideCounts(t, pool)
	inactiveCarrier := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES ($1,$2,'CARRIER',$3,'INACTIVE')`, inactiveCarrier, fix.TenantA, inactiveCarrier.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_awards SET carrier_company_id = $2 WHERE id = $1`, fix.VersionAwardID, inactiveCarrier); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_events SET deleted_at = now() WHERE id = $1`, fix.DeletedEventID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_events SET status = 'PUBLISHED' WHERE id = $1`, fix.ArchivedEventID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_award_transport_orders SET buyer_company_id = $2 WHERE id = $1`, fix.BuyerMismatchLinkID, fix.CarrierID); err != nil {
		t.Fatal(err)
	}

	assertRfxCreateStatus(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, uuid.New(), "missing-award", http.StatusNotFound)
	assertRfxCreateStatus(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAwardLink, uuid.New(), "missing-link", http.StatusNotFound)
	foreign := postRfxCreate(t, router, jwtSecret, fix.TenantBProcurement, fix.TenantB, domain.RfxSourceTypeAward, fix.AwardID, "foreign")
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), fix.TenantA.String()) || strings.Contains(foreign.Body.String(), "acceptance_token") {
		t.Fatalf("foreign tenant %d %s", foreign.Code, foreign.Body.String())
	}
	assertRfxCreateStatus(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.DeletedAwardID, "deleted-event", http.StatusNotFound)
	assertRfxCreateStatus(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.ArchivedAwardID, "not-awarded", http.StatusNotFound)
	assertRfxCreateStatus(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAwardLink, fix.BuyerMismatchLinkID, "wrong-buyer", http.StatusNotFound)
	assertRfxCreateStatus(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.VersionAwardID, "wrong-carrier", http.StatusNotFound)
	for _, userID := range []uuid.UUID{fix.BlockedUser, fix.DeletedUser, fix.CarrierAdmin, fix.PlatformNoMembership, fix.InactiveMembership, fix.CompanyScopedPlatformAdmin} {
		assertRfxCreateStatus(t, router, jwtSecret, userID, fix.TenantA, domain.RfxSourceTypeAward, fix.AwardID, "actor-"+userID.String()[:8], http.StatusForbidden)
	}
	if sideCounts(t, pool) != before {
		t.Fatalf("refusal wrote rows: before=%s after=%s", before, sideCounts(t, pool))
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_awards SET carrier_company_id = $2 WHERE id = $1`, fix.VersionAwardID, fix.CarrierID); err != nil {
		t.Fatal(err)
	}
	if transportOrdersAfter(t, pool) != 0 {
		t.Fatal("refusals must not read or create transport orders")
	}

	first, second := concurrentRfxCreate(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.AwardID, "award-key", "award-key")
	if !sameCreatePair([]int{first.Code, second.Code}, http.StatusCreated, http.StatusOK) {
		t.Fatalf("concurrent award %d %s / %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
	tokenBodies := 0
	var rawToken string
	for _, rec := range []*httptest.ResponseRecorder{first, second} {
		if strings.Contains(rec.Body.String(), `"acceptance_token"`) {
			tokenBodies++
			rawToken = tokenBetween(t, rec.Body.String())
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("token response cache %q", rec.Header().Get("Cache-Control"))
			}
		}
	}
	if tokenBodies != 1 || rawToken == "" {
		t.Fatalf("token disclosures=%d", tokenBodies)
	}
	assertRfxTokenNotStored(t, pool, rawToken)
	afterCreate := sideCounts(t, pool)
	replay := postRfxCreate(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.AwardID, "award-key")
	if replay.Code != http.StatusOK || strings.Contains(replay.Body.String(), "acceptance_token") || sideCounts(t, pool) != afterCreate {
		t.Fatalf("replay %d %s counts=%s", replay.Code, replay.Body.String(), sideCounts(t, pool))
	}
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_awards SET version = version + 1 WHERE id = $1`, fix.AwardID); err != nil {
		t.Fatal(err)
	}
	stale := postRfxCreate(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.AwardID, "award-key")
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "RELATIONSHIP_IDEMPOTENCY_CONFLICT") || strings.Contains(stale.Body.String(), "acceptance_token") {
		t.Fatalf("changed version same key %d %s", stale.Code, stale.Body.String())
	}
	otherKey := postRfxCreate(t, router, jwtSecret, fix.Procurement, fix.TenantA, domain.RfxSourceTypeAward, fix.AwardID, "award-other-key")
	if otherKey.Code != http.StatusConflict || !strings.Contains(otherKey.Body.String(), "RELATIONSHIP_REQUEST_PENDING_CONFLICT") || strings.Contains(otherKey.Body.String(), rawToken) {
		t.Fatalf("changed version different key %d %s", otherKey.Code, otherKey.Body.String())
	}
	if countRfxRequests(t, pool, fix.AwardID) != 1 {
		t.Fatalf("award requests=%d", countRfxRequests(t, pool, fix.AwardID))
	}

	linkCreated := postRfxCreate(t, router, jwtSecret, fix.ShipperLogist, fix.TenantA, domain.RfxSourceTypeAwardLink, fix.LinkID, "link-key")
	if linkCreated.Code != http.StatusCreated || linkCreated.Header().Get("Cache-Control") != "no-store" || !strings.Contains(linkCreated.Body.String(), `"acceptance_token"`) {
		t.Fatalf("link create %d cache=%s body=%s", linkCreated.Code, linkCreated.Header().Get("Cache-Control"), linkCreated.Body.String())
	}
	linkToken := tokenBetween(t, linkCreated.Body.String())
	assertRfxTokenNotStored(t, pool, linkToken)
	if _, err := pool.Exec(ctx, `UPDATE rfx.rfx_award_transport_orders SET transport_order_id = $2 WHERE id = $1`, fix.LinkID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	changedOrder := postRfxCreate(t, router, jwtSecret, fix.ShipperLogist, fix.TenantA, domain.RfxSourceTypeAwardLink, fix.LinkID, "link-key")
	if changedOrder.Code != http.StatusConflict || strings.Contains(changedOrder.Body.String(), linkToken) {
		t.Fatalf("changed transport order %d %s", changedOrder.Code, changedOrder.Body.String())
	}
	if countRfxRequests(t, pool, fix.LinkID) != 1 || transportOrdersAfter(t, pool) != 0 {
		t.Fatalf("link requests=%d transport_orders=%d", countRfxRequests(t, pool, fix.LinkID), transportOrdersAfter(t, pool))
	}

	lostAward := insertAward(t, pool, fix.TenantA, fix.OwnerID, fix.CarrierID, "AWARDED")
	lostClient := service.NewCompanyRelationshipCreateClient(companyURL, rfxToken, &http.Client{Transport: loseRfxAck{base: http.DefaultTransport}, Timeout: 2 * time.Second})
	lostInitiator := service.NewRfxRelationshipCreateInitiator(repo, lostClient)
	if _, err := lostInitiator.Create(ctx, service.RfxRelationshipActor{UserID: fix.ForwarderManager, TenantID: fix.TenantA}, domain.RfxSourceTypeAward, lostAward, "lost-key"); err == nil {
		t.Fatal("lost ack returned success")
	}
	if countRfxRequests(t, pool, lostAward) != 1 {
		t.Fatalf("lost ack request count=%d", countRfxRequests(t, pool, lostAward))
	}
	lostReplay := postRfxCreate(t, router, jwtSecret, fix.ForwarderManager, fix.TenantA, domain.RfxSourceTypeAward, lostAward, "lost-key")
	if lostReplay.Code != http.StatusOK || strings.Contains(lostReplay.Body.String(), "acceptance_token") {
		t.Fatalf("lost ack same key %d %s", lostReplay.Code, lostReplay.Body.String())
	}
	lostOther := postRfxCreate(t, router, jwtSecret, fix.ForwarderManager, fix.TenantA, domain.RfxSourceTypeAward, lostAward, "lost-other-key")
	if lostOther.Code != http.StatusConflict || countRfxRequests(t, pool, lostAward) != 1 {
		t.Fatalf("lost ack different key %d count=%d %s", lostOther.Code, countRfxRequests(t, pool, lostAward), lostOther.Body.String())
	}

	platform := postRfxCreate(t, router, jwtSecret, fix.PlatformWithMembership, fix.TenantA, domain.RfxSourceTypeAward, fix.VersionAwardID, "platform-key")
	if platform.Code != http.StatusCreated || !strings.Contains(platform.Body.String(), `"acceptance_token"`) {
		t.Fatalf("platform admin %d %s", platform.Code, platform.Body.String())
	}
	shipper := postRfxCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, domain.RfxSourceTypeAwardLink, fix.OrderLinkID, "shipper-key")
	if shipper.Code != http.StatusCreated {
		t.Fatalf("shipper admin link %d %s", shipper.Code, shipper.Body.String())
	}

	wrong := service.NewRfxRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient(companyURL, "forged-credential", nil))
	if _, err := wrong.Create(ctx, service.RfxRelationshipActor{UserID: fix.Procurement, TenantID: fix.TenantA}, domain.RfxSourceTypeAward, fix.AwardID, "forged"); codeOfRfx(err) != apperrors.CodeRelationshipSourceUnavailable {
		t.Fatalf("forged credential %v", err)
	}
	closed := service.NewRfxRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient("", "", nil))
	if _, err := closed.Create(ctx, service.RfxRelationshipActor{UserID: fix.Procurement, TenantID: fix.TenantA}, domain.RfxSourceTypeAward, fix.AwardID, "closed"); codeOfRfx(err) != apperrors.CodeRelationshipSourceUnavailable {
		t.Fatalf("empty configuration %v", err)
	}
	shared := service.NewRfxRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient(companyURL, sharedInternalToken, nil))
	if _, err := shared.Create(ctx, service.RfxRelationshipActor{UserID: fix.Procurement, TenantID: fix.TenantA}, domain.RfxSourceTypeAward, fix.AwardID, "shared"); codeOfRfx(err) != apperrors.CodeRelationshipSourceUnavailable {
		t.Fatalf("shared internal token %v", err)
	}
	trap := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("shipment source called company-service")
	}))
	defer trap.Close()
	if _, err := service.NewCompanyRelationshipCreateClient(trap.URL, rfxToken, nil).Create(ctx, service.CompanyRelationshipCreateRequest{SourceType: "SHIPMENT", SourceID: uuid.New(), IdempotencyKey: "shipment"}); codeOfRfx(err) != apperrors.CodeRelationshipRequestForbidden {
		t.Fatalf("shipment source %v", err)
	}
	var caller string
	if err := pool.QueryRow(ctx, `SELECT caller_service FROM company.carrier_relationship_request_idempotency WHERE idempotency_key = 'award-key'`).Scan(&caller); err != nil || caller != "rfx-service" {
		t.Fatalf("caller_service=%s err=%v", caller, err)
	}
	lostID := rfxRequestID(t, lostReplay.Body.String())
	foreignStatus := postRfxStatus(t, router, jwtSecret, fix.TenantBProcurement, fix.TenantB, domain.RfxSourceTypeAward, lostAward)
	if foreignStatus.Code != http.StatusNotFound || strings.Contains(foreignStatus.Body.String(), lostID.String()) || strings.Contains(foreignStatus.Body.String(), "acceptance_token") {
		t.Fatalf("foreign status %d %s", foreignStatus.Code, foreignStatus.Body.String())
	}
	ownStatus := postRfxStatus(t, router, jwtSecret, fix.ForwarderManager, fix.TenantA, domain.RfxSourceTypeAward, lostAward)
	if ownStatus.Code != http.StatusOK || ownStatus.Header().Get("Cache-Control") != "no-store" || !strings.Contains(ownStatus.Body.String(), lostID.String()) || strings.Contains(ownStatus.Body.String(), "acceptance_token") {
		t.Fatalf("own status %d cache=%s %s", ownStatus.Code, ownStatus.Header().Get("Cache-Control"), ownStatus.Body.String())
	}
	cancelled := postRfxCancel(t, router, jwtSecret, fix.ForwarderManager, fix.TenantA, domain.RfxSourceTypeAward, lostAward, lostID, "recover-cancel")
	if cancelled.Code != http.StatusOK || cancelled.Header().Get("Cache-Control") != "no-store" || strings.Contains(cancelled.Body.String(), "acceptance_token") || !strings.Contains(cancelled.Body.String(), `"request_status":"CANCELLED"`) {
		t.Fatalf("cancel %d %s", cancelled.Code, cancelled.Body.String())
	}
	recovered := postRfxCreate(t, router, jwtSecret, fix.ForwarderManager, fix.TenantA, domain.RfxSourceTypeAward, lostAward, "recover-new")
	if recovered.Code != http.StatusCreated || recovered.Header().Get("Cache-Control") != "no-store" || !strings.Contains(recovered.Body.String(), `"acceptance_token"`) {
		t.Fatalf("create after cancel %d cache=%s %s", recovered.Code, recovered.Header().Get("Cache-Control"), recovered.Body.String())
	}
	recoveredReplay := postRfxCreate(t, router, jwtSecret, fix.ForwarderManager, fix.TenantA, domain.RfxSourceTypeAward, lostAward, "recover-new")
	if recoveredReplay.Code != http.StatusOK || strings.Contains(recoveredReplay.Body.String(), "acceptance_token") || recoveredReplay.Header().Get("Cache-Control") == "no-store" {
		t.Fatalf("replay after recovery %d cache=%s %s", recoveredReplay.Code, recoveredReplay.Header().Get("Cache-Control"), recoveredReplay.Body.String())
	}

	var relationships int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM company.carrier_relationships`).Scan(&relationships); err != nil || relationships != 0 {
		t.Fatalf("relationships=%d err=%v", relationships, err)
	}
	t.Log("POSTGRES_INTEGRATION=PASS")
}

func startRfxCompanyRelationshipProcess(t *testing.T, pool *pgxpool.Pool, rfxURL, revalidationToken, rfxToken, shipmentToken, sharedToken string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "company-service.exe")
	build := exec.Command("go", "build", "-o", bin, "./cmd/server")
	build.Dir = filepath.Join("..", "..", "..", "..", "company-service")
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build company-service: %v\n%s", err, output)
	}
	port := freePort(t)
	databaseURL := pool.Config().ConnString()
	if databaseURL == "" {
		t.Fatal("pool has no connection string")
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+databaseURL,
		"COMPANY_SERVICE_PORT="+strconv.Itoa(port),
		"COMPANY_RELATIONSHIP_COMMANDS_ENABLED=true",
		"RELATIONSHIP_TOKEN_PEPPER=postgres-pepper",
		"SHIPMENT_RELATIONSHIP_SERVICE_TOKEN="+shipmentToken,
		"RFX_RELATIONSHIP_SERVICE_TOKEN="+rfxToken,
		"SHIPMENT_SOURCE_AUTHORITY_BASE_URL=",
		"RFX_SOURCE_AUTHORITY_BASE_URL="+rfxURL,
		"COMPANY_RELATIONSHIP_REVALIDATION_TOKEN="+revalidationToken,
		"INTERNAL_SERVICE_TOKEN="+sharedToken,
		"JWT_SECRET=company-jwt",
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("company-service did not become ready")
	return ""
}

func postRfxCreate(t *testing.T, handler http.Handler, secret string, userID, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID, key string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := security.SignHS256(secret, userID, tenantID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	prefix := "/v1/rfx-awards/"
	if sourceType == domain.RfxSourceTypeAwardLink {
		prefix = "/v1/rfx-award-transport-orders/"
	}
	req := httptest.NewRequest(http.MethodPost, prefix+sourceID.String()+"/carrier-relationship-requests", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Internal-Service-Name", "shipment-service")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func postRfxStatus(t *testing.T, handler http.Handler, secret string, userID, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return postRfx(t, handler, secret, userID, tenantID, http.MethodGet, sourceType, sourceID, uuid.Nil, "")
}

func postRfxCancel(t *testing.T, handler http.Handler, secret string, userID, tenantID uuid.UUID, sourceType string, sourceID, requestID uuid.UUID, key string) *httptest.ResponseRecorder {
	t.Helper()
	return postRfx(t, handler, secret, userID, tenantID, http.MethodPost, sourceType, sourceID, requestID, key)
}

func postRfx(t *testing.T, handler http.Handler, secret string, userID, tenantID uuid.UUID, method, sourceType string, sourceID, requestID uuid.UUID, key string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := security.SignHS256(secret, userID, tenantID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	prefix := "/v1/rfx-awards/"
	if sourceType == domain.RfxSourceTypeAwardLink {
		prefix = "/v1/rfx-award-transport-orders/"
	}
	path := prefix + sourceID.String() + "/carrier-relationship-requests"
	if requestID != uuid.Nil {
		path += "/" + requestID.String() + "/cancel"
	}
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func rfxRequestID(t *testing.T, body string) uuid.UUID {
	t.Helper()
	var decoded struct {
		RequestID string `json:"relationship_request_id"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatal(err)
	}
	id, err := uuid.Parse(decoded.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertRfxCreateStatus(t *testing.T, handler http.Handler, secret string, userID, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID, key string, status int) {
	t.Helper()
	rec := postRfxCreate(t, handler, secret, userID, tenantID, sourceType, sourceID, key)
	if rec.Code != status || strings.Contains(rec.Body.String(), "acceptance_token") {
		t.Fatalf("%s status=%d body=%s", key, rec.Code, rec.Body.String())
	}
}

func concurrentRfxCreate(t *testing.T, handler http.Handler, secret string, userID, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID, leftKey, rightKey string) (*httptest.ResponseRecorder, *httptest.ResponseRecorder) {
	t.Helper()
	var wg sync.WaitGroup
	recs := [2]*httptest.ResponseRecorder{}
	keys := [2]string{leftKey, rightKey}
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			recs[i] = postRfxCreate(t, handler, secret, userID, tenantID, sourceType, sourceID, keys[i])
		}(i)
	}
	wg.Wait()
	return recs[0], recs[1]
}

func sameCreatePair(codes []int, left, right int) bool {
	return (codes[0] == left && codes[1] == right) || (codes[0] == right && codes[1] == left)
}

func tokenBetween(t *testing.T, body string) string {
	t.Helper()
	const marker = `"acceptance_token":"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("token missing in %s", body)
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		t.Fatalf("token truncated in %s", body)
	}
	return rest[:end]
}

func assertRfxTokenNotStored(t *testing.T, pool *pgxpool.Pool, rawToken string) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT acceptance_token_hash, source_version_or_fingerprint FROM company.carrier_relationship_requests`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var hash []byte
		var fingerprint string
		if err := rows.Scan(&hash, &fingerprint); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(hash, []byte(rawToken)) || strings.Contains(fingerprint, rawToken) || strings.Contains(string(hash), rawToken) {
			t.Fatal("raw acceptance token was stored")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var leaks int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM company.carrier_relationship_outbox WHERE strpos(payload::text, $1) > 0`, rawToken).Scan(&leaks); err != nil || leaks != 0 {
		t.Fatalf("outbox token leaks=%d err=%v", leaks, err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM company.carrier_relationship_audit WHERE strpos(coalesce(correlation_id, ''), $1) > 0 OR strpos(coalesce(idempotency_key, ''), $1) > 0`, rawToken).Scan(&leaks); err != nil || leaks != 0 {
		t.Fatalf("audit token leaks=%d err=%v", leaks, err)
	}
}

func countRfxRequests(t *testing.T, pool *pgxpool.Pool, sourceID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM company.carrier_relationship_requests WHERE source_id = $1`, sourceID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func codeOfRfx(err error) apperrors.Code {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

type loseRfxAck struct {
	base http.RoundTripper
}

func (l loseRfxAck) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := l.base.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New("lost ack")
}

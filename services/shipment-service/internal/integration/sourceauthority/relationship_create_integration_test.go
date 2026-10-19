//go:build integration

package sourceauthority

import (
	"bytes"
	"context"
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

	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/security"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestShipmentRelationshipCreatePostgres(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	var pgVersion, dbName string
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_database()`).Scan(&pgVersion, &dbName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbName, "src_auth_") {
		t.Fatalf("refusing non-isolated database %s", dbName)
	}
	t.Logf("POSTGRES_VERSION=%s", pgVersion)
	t.Logf("TEST_DATABASE_ISOLATED=%s", dbName)

	fix := seedSource(t, pool)
	const (
		jwtSecret           = "relationship-jwt-secret"
		revalidationToken   = "dedicated-revalidation-token"
		shipmentToken       = "dedicated-shipment-relationship-token"
		sharedInternalToken = "shared-internal-token"
	)
	repo := repository.NewShipmentSourceAuthorityRepository(pool)
	confirmServer := httptest.NewServer(shipmenthttp.WithShipmentSourceAuthority(http.NotFoundHandler(), revalidationToken, service.NewShipmentSourceAuthority(repo)))
	t.Cleanup(confirmServer.Close)
	companyURL := startCompanyRelationshipProcess(t, pool, confirmServer.URL, revalidationToken, shipmentToken, sharedInternalToken)
	initiator := service.NewRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient(companyURL, shipmentToken, nil))
	router := shipmenthttp.WithShipmentRelationshipCreate(http.NotFoundHandler(), jwtSecret, initiator)

	before := relationshipCounts(t, pool)
	inactiveUser := uuid.New()
	insertUser(t, pool, inactiveUser, fix.TenantA, "ACTIVE", false)
	if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status) VALUES ($1,$2,$3,'INACTIVE')`, fix.TenantA, fix.ShipperID, inactiveUser); err != nil {
		t.Fatal(err)
	}
	grantRole(t, pool, fix.TenantA, inactiveUser, &fix.ShipperID, "SHIPPER_ADMIN")

	assertCreateStatus(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, uuid.New(), "missing", http.StatusNotFound)
	foreign := postCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantB, fix.ShipmentID, "foreign")
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), fix.TenantA.String()) {
		t.Fatalf("foreign tenant %d %s", foreign.Code, foreign.Body.String())
	}
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET deleted_at = clock_timestamp() WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	assertCreateStatus(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, fix.ShipmentID, "deleted", http.StatusNotFound)
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET deleted_at = NULL WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET carrier_company_id = NULL WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	assertCreateStatus(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, fix.ShipmentID, "no-carrier", http.StatusNotFound)
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET carrier_company_id = $2 WHERE id = $1`, fix.ShipmentID, fix.CarrierID); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []uuid.UUID{fix.BlockedUser, fix.DeletedUser, fix.CarrierAdmin, fix.MemberNoRole, fix.PlatformNoMembership, inactiveUser} {
		assertCreateStatus(t, router, jwtSecret, userID, fix.TenantA, fix.ShipmentID, "actor-"+userID.String()[:8], http.StatusForbidden)
	}
	if relationshipCounts(t, pool) != before {
		t.Fatalf("refusal wrote rows: before=%s after=%s", before, relationshipCounts(t, pool))
	}

	first, second := concurrentCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, fix.ShipmentID, "same-key", "same-key")
	codes := []int{first.Code, second.Code}
	if !sameCreatePair(codes, http.StatusCreated, http.StatusOK) {
		t.Fatalf("concurrent same key %d %s / %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
	tokenBodies := 0
	var rawToken string
	for _, rec := range []*httptest.ResponseRecorder{first, second} {
		if strings.Contains(rec.Body.String(), `"acceptance_token"`) {
			tokenBodies++
			rawToken = tokenBetween(t, rec.Body.String())
		}
	}
	if tokenBodies != 1 || rawToken == "" {
		t.Fatalf("token disclosures=%d", tokenBodies)
	}
	assertTokenNotStored(t, pool, rawToken)
	replay := postCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, fix.ShipmentID, "same-key")
	if replay.Code != http.StatusOK || strings.Contains(replay.Body.String(), "acceptance_token") {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET version = version + 1 WHERE id = $1`, fix.ShipmentID); err != nil {
		t.Fatal(err)
	}
	stale := postCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, fix.ShipmentID, "same-key")
	if stale.Code != http.StatusConflict || strings.Contains(stale.Body.String(), "acceptance_token") {
		t.Fatalf("changed shipment same key %d %s", stale.Code, stale.Body.String())
	}
	otherKey := postCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, fix.ShipmentID, "other-key")
	if otherKey.Code != http.StatusConflict || strings.Contains(otherKey.Body.String(), rawToken) {
		t.Fatalf("different key %d %s", otherKey.Code, otherKey.Body.String())
	}

	lostShipment := cloneShipment(t, pool, fix.ShipmentID)
	lostClient := service.NewCompanyRelationshipCreateClient(companyURL, shipmentToken, &http.Client{Transport: loseAck{base: http.DefaultTransport}, Timeout: 2 * time.Second})
	lostInitiator := service.NewRelationshipCreateInitiator(repo, lostClient)
	if _, err := lostInitiator.Create(ctx, service.RelationshipActor{UserID: fix.ShipperAdmin, TenantID: fix.TenantA}, lostShipment, "lost-key"); err == nil {
		t.Fatal("lost ack returned success")
	}
	if countRequests(t, pool, lostShipment) != 1 {
		t.Fatalf("lost ack request count=%d", countRequests(t, pool, lostShipment))
	}
	lostReplay := postCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, lostShipment, "lost-key")
	if lostReplay.Code != http.StatusOK || strings.Contains(lostReplay.Body.String(), "acceptance_token") {
		t.Fatalf("lost ack same key %d %s", lostReplay.Code, lostReplay.Body.String())
	}
	lostOther := postCreate(t, router, jwtSecret, fix.ShipperAdmin, fix.TenantA, lostShipment, "lost-other-key")
	if lostOther.Code != http.StatusConflict {
		t.Fatalf("lost ack different key %d %s", lostOther.Code, lostOther.Body.String())
	}

	wrong := service.NewRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient(companyURL, "forged-credential", nil))
	if _, err := wrong.Create(ctx, service.RelationshipActor{UserID: fix.ShipperAdmin, TenantID: fix.TenantA}, fix.ShipmentID, "forged"); codeOf(err) != apperrors.CodeRelationshipSourceUnavailable {
		t.Fatalf("forged credential %v", err)
	}
	closed := service.NewRelationshipCreateInitiator(repo, service.NewCompanyRelationshipCreateClient("", "", nil))
	if _, err := closed.Create(ctx, service.RelationshipActor{UserID: fix.ShipperAdmin, TenantID: fix.TenantA}, fix.ShipmentID, "closed"); codeOf(err) != apperrors.CodeRelationshipSourceUnavailable {
		t.Fatalf("empty configuration %v", err)
	}
	trap := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("rfx create called company-service")
	}))
	defer trap.Close()
	if _, err := service.NewCompanyRelationshipCreateClient(trap.URL, shipmentToken, nil).Create(ctx, service.CompanyRelationshipCreateRequest{SourceType: "RFX_AWARD", SourceID: uuid.New(), IdempotencyKey: "rfx"}); codeOf(err) != apperrors.CodeRelationshipRequestForbidden {
		t.Fatalf("rfx source %v", err)
	}
	var caller string
	if err := pool.QueryRow(ctx, `SELECT caller_service FROM company.carrier_relationship_request_idempotency WHERE idempotency_key = 'same-key'`).Scan(&caller); err != nil || caller != "shipment-service" {
		t.Fatalf("caller_service=%s err=%v", caller, err)
	}
	var relationships int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM company.carrier_relationships`).Scan(&relationships); err != nil || relationships != 0 {
		t.Fatalf("relationships=%d err=%v", relationships, err)
	}
	t.Log("POSTGRES_INTEGRATION=PASS")
}

func startCompanyRelationshipProcess(t *testing.T, pool *pgxpool.Pool, shipmentURL, revalidationToken, shipmentToken, sharedToken string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "company-service.exe")
	build := exec.Command("go", "build", "-o", bin, "./cmd/server")
	build.Dir = companyServiceDir(t)
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
		"RFX_RELATIONSHIP_SERVICE_TOKEN=rfx-relationship-token-distinct",
		"SHIPMENT_SOURCE_AUTHORITY_BASE_URL="+shipmentURL,
		"RFX_SOURCE_AUTHORITY_BASE_URL=",
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

func companyServiceDir(t *testing.T) string {
	t.Helper()
	candidate := filepath.Join("..", "..", "..", "..", "company-service")
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate
	}
	t.Fatalf("company-service directory not found from %s", candidate)
	return ""
}

func postCreate(t *testing.T, handler http.Handler, secret string, userID, tenantID, shipmentID uuid.UUID, key string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := security.SignHS256(secret, userID, tenantID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+shipmentID.String()+"/carrier-relationship-requests", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Internal-Service-Name", "rfx-service")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertCreateStatus(t *testing.T, handler http.Handler, secret string, userID, tenantID, shipmentID uuid.UUID, key string, status int) {
	t.Helper()
	rec := postCreate(t, handler, secret, userID, tenantID, shipmentID, key)
	if rec.Code != status || strings.Contains(rec.Body.String(), "acceptance_token") {
		t.Fatalf("%s status=%d body=%s", key, rec.Code, rec.Body.String())
	}
}

func concurrentCreate(t *testing.T, handler http.Handler, secret string, userID, tenantID, shipmentID uuid.UUID, leftKey, rightKey string) (*httptest.ResponseRecorder, *httptest.ResponseRecorder) {
	t.Helper()
	var wg sync.WaitGroup
	recs := [2]*httptest.ResponseRecorder{}
	keys := [2]string{leftKey, rightKey}
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			recs[i] = postCreate(t, handler, secret, userID, tenantID, shipmentID, keys[i])
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

func assertTokenNotStored(t *testing.T, pool *pgxpool.Pool, rawToken string) {
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

func relationshipCounts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var requests, events, outbox, relationships int
	err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM company.carrier_relationship_requests),
			(SELECT count(*) FROM company.carrier_relationship_request_events),
			(SELECT count(*) FROM company.carrier_relationship_outbox),
			(SELECT count(*) FROM company.carrier_relationships)
	`).Scan(&requests, &events, &outbox, &relationships)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join([]string{
		"requests=" + strconv.Itoa(requests),
		"events=" + strconv.Itoa(events),
		"outbox=" + strconv.Itoa(outbox),
		"relationships=" + strconv.Itoa(relationships),
	}, " ")
}

func countRequests(t *testing.T, pool *pgxpool.Pool, shipmentID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM company.carrier_relationship_requests WHERE source_id = $1`, shipmentID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func cloneShipment(t *testing.T, pool *pgxpool.Pool, shipmentID uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := uuid.New()
	shipment := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode
		)
		SELECT $2, o.tenant_id, $3, o.status, o.shipper_company_id, o.consignee_company_id, o.origin_location_id, o.destination_location_id, o.transport_mode
		FROM transport.transport_orders o
		JOIN transport.shipments s ON s.transport_order_id = o.id
		WHERE s.id = $1
	`, shipmentID, orderID, "TO-"+orderID.String()[:8])
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, origin_location_id, destination_location_id, transport_mode, status, version
		)
		SELECT $2, tenant_id, $3, $4, shipper_company_id, consignee_company_id,
			carrier_company_id, origin_location_id, destination_location_id, transport_mode, status, version
		FROM transport.shipments WHERE id = $1
	`, shipmentID, shipment, "SHP-"+shipment.String()[:8], orderID)
	if err != nil {
		t.Fatal(err)
	}
	return shipment
}

func codeOf(err error) apperrors.Code {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

type loseAck struct {
	base http.RoundTripper
}

func (l loseAck) RoundTrip(req *http.Request) (*http.Response, error) {
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

//go:build integration

package relationship

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
)

func TestCompanyRelationshipFoundationPostgres(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	var pgVersion, dbName string
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_database()`).Scan(&pgVersion, &dbName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbName, "rel_found_") {
		t.Fatalf("refusing non-isolated database %s", dbName)
	}
	t.Logf("POSTGRES_VERSION=%s", pgVersion)
	t.Logf("TEST_DATABASE_ISOLATED=%s", dbName)
	marker := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, 'historical')`, marker, "hist-"+marker.String()[:8]); err != nil {
		t.Fatal(err)
	}
	before := coreSnapshot(t, pool)
	if err := applyRevokeDown(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := applyCancelDown(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := cycleFoundationMigration(ctx, pool); err != nil {
		t.Fatal(err)
	}
	after := coreSnapshot(t, pool)
	if before != after {
		t.Fatalf("historical tables changed during 000100 rollback: before=%s after=%s", before, after)
	}
	if err := cycleCancelMigration(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if again := coreSnapshot(t, pool); again != after {
		t.Fatalf("historical tables changed during 000101 rollback: before=%s after=%s", after, again)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM core.tenants WHERE id = $1`, marker).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("historical tenant after up/down/up remaining=%d err=%v", remaining, err)
	}
	t.Log("MIGRATION_UP_DOWN_UP=PASS")

	fix := seedFixture(t, pool)
	repo := repository.NewRelationshipRepository(pool)
	cat := newCatalog()
	match := service.NewRelationshipService(repo, repo, postgresRelationshipConfig(), matchSource{}, cat)
	closed := service.NewRelationshipService(repo, repo, postgresRelationshipConfig(), nil, nil)

	created := createOK(t, match, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "create-key", "v1")
	var storedCaller string
	if err := pool.QueryRow(ctx, `SELECT caller_service FROM company.carrier_relationship_request_idempotency WHERE relationship_request_id = $1`, created.RequestID).Scan(&storedCaller); err != nil || storedCaller != domain.CallerShipmentService {
		t.Fatalf("caller_service=%s err=%v", storedCaller, err)
	}
	replay, err := match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, created.SourceID, "create-key", "v1"))
	if err != nil || replay.Created || replay.AcceptanceToken != "" || replay.RequestID != created.RequestID {
		t.Fatalf("replay: %v %+v", err, replay)
	}
	cat.setFingerprint(created.SourceID, "v2")
	_, err = match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, created.SourceID, "create-key", "v2"))
	assertCode(t, err, apperrors.CodeRelationshipIdempotencyConflict)
	_, err = match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, created.SourceID, "other-key", "v2"))
	assertCode(t, err, apperrors.CodeRelationshipRequestPendingConflict)

	_, err = match.Create(ctx, createInput(fix.BlockedUser, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "blocked", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = match.Create(ctx, createInput(fix.DeletedUser, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "deleted", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = match.Create(ctx, createInput(fix.PlatformNoMembership, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "platform", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = match.Create(ctx, createInput(fix.CompanyScopedPlatformAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "scoped-platform", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	createOK(t, match, cat, fix.PlatformWithMembership, fix.TenantA, fix.ShipperA, fix.CarrierA, "global-platform", "v1")
	_, err = match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierB, uuid.New(), "cross", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = match.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.OrdinaryMember, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
		AcceptanceToken: created.Token, IdempotencyKey: "ordinary",
	})
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)

	_, err = closed.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
		AcceptanceToken: created.Token, IdempotencyKey: "closed",
	})
	assertCode(t, err, apperrors.CodeRelationshipSourceUnavailable)
	var relationships int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM company.carrier_relationships`).Scan(&relationships); err != nil || relationships != 0 {
		t.Fatalf("fail-closed accept wrote relationships=%d err=%v", relationships, err)
	}

	accepted, err := match.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
		AcceptanceToken: created.Token, IdempotencyKey: "accept-key",
	})
	if err != nil || !accepted.Created || accepted.RequestStatus != domain.RequestStatusAccepted {
		t.Fatalf("accept: %v %+v", err, accepted)
	}
	replayed, err := match.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
		IdempotencyKey: "accept-key",
	})
	if err != nil || replayed.Created || replayed.RelationshipID != accepted.RelationshipID {
		t.Fatalf("accept replay: %v %+v", err, replayed)
	}
	_, err = match.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB2,
		AcceptanceToken: created.Token, IdempotencyKey: "accept-other",
	})
	assertCode(t, err, apperrors.CodeRelationshipRequestAlreadyAccepted)
	if strings.Contains(err.Error(), fix.OperatingB.String()) {
		t.Fatal("already-accepted error revealed an operating company")
	}
	foreignView, err := match.Get(ctx, domain.Principal{UserID: fix.ShipperAdmin, TenantID: fix.TenantA}, fix.ShipperA, uuid.New())
	if foreignView.RequestID != uuid.Nil {
		t.Fatal("foreign get returned a row")
	}
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	ownView, err := match.Get(ctx, domain.Principal{UserID: fix.ShipperAdmin, TenantID: fix.TenantA}, fix.ShipperA, created.RequestID)
	if err != nil || ownView.Status != domain.RequestStatusAccepted {
		t.Fatalf("owner get: %v %+v", err, ownView)
	}
	_, err = match.Get(ctx, domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, fix.OperatingB2, created.RequestID)
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	operatingView, err := match.Get(ctx, domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, fix.OperatingB, created.RequestID)
	if err != nil || operatingView.RequestID != created.RequestID {
		t.Fatalf("accepting company get: %v %+v", err, operatingView)
	}

	assertSourceAuthority(t, pool, match, closed, cat, fix)
	assertTokenStoredOnlyAsHMAC(t, pool, created.RequestID, created.Token)
	assertDirectTenantGuards(t, pool, fix)
	assertDatabaseConstraints(t, pool, fix)
	assertRollbackIsAtomic(t, pool, fix)
	assertConcurrentCreate(t, match, cat, fix)
	assertCancelRecovery(t, pool, match, cat, fix)
	runPublicAcceptScenarios(t, pool)
	if err := applyRevokeUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
	runRevokeScenarios(t, pool)
	assertRevokeMigrationCycle(t)
	t.Log("POSTGRES_INTEGRATION=PASS")
}

type matchSource struct{}

func (matchSource) Revalidate(context.Context, domain.SourceSnapshot) (domain.SourceDecision, error) {
	return domain.SourceMatch, nil
}

type createdRequest struct {
	RequestID uuid.UUID
	SourceID  uuid.UUID
	Token     string
}

const (
	shipmentRelationshipToken = "shipment-relationship-test-token"
	rfxRelationshipToken      = "rfx-relationship-test-token"
)

func postgresRelationshipConfig() service.RelationshipConfig {
	return service.RelationshipConfig{
		CommandsEnabled: true, TokenPepper: "postgres-pepper",
		ShipmentServiceToken: shipmentRelationshipToken, RfxServiceToken: rfxRelationshipToken,
	}
}

type catalogSource struct {
	mu   sync.Mutex
	rows map[uuid.UUID]domain.SourceCreateSubject
	last domain.SourceCreateDecision
}

func newCatalog() *catalogSource {
	return &catalogSource{rows: map[uuid.UUID]domain.SourceCreateSubject{}}
}

func (c *catalogSource) allow(subject domain.SourceCreateSubject) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows[subject.SourceID] = subject
}

func (c *catalogSource) setFingerprint(id uuid.UUID, fingerprint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	row := c.rows[id]
	row.Fingerprint = fingerprint
	c.rows[id] = row
}

func (c *catalogSource) ConfirmCreate(_ context.Context, subject domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	row, ok := c.rows[subject.SourceID]
	decision := domain.SourceCreateConfirmed
	switch {
	case !ok:
		decision = domain.SourceCreateNotFound
	case row.SourceTenantID != subject.SourceTenantID:
		decision = domain.SourceCreateTenantMismatch
	case row.Fingerprint != subject.Fingerprint:
		decision = domain.SourceCreateStale
	case row.SourceType != subject.SourceType || row.RequestingUserID != subject.RequestingUserID || row.RequestingCompanyID != subject.RequestingCompanyID || row.ShipperCarrierCompanyID != subject.ShipperCarrierCompanyID:
		decision = domain.SourceCreateActorMismatch
	}
	c.last = decision
	return decision, nil
}

func createOK(t *testing.T, svc *service.RelationshipService, cat *catalogSource, user, tenant, company, carrier uuid.UUID, key, fingerprint string) createdRequest {
	t.Helper()
	sourceID := uuid.New()
	cat.allow(domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: sourceID, SourceTenantID: tenant, Fingerprint: fingerprint,
		ShipperCarrierCompanyID: carrier, RequestingCompanyID: company, RequestingUserID: user,
	})
	out, err := svc.Create(context.Background(), createInput(user, tenant, company, carrier, sourceID, key, fingerprint))
	if err != nil || !out.Created || out.AcceptanceToken == "" {
		t.Fatalf("create: %v %+v", err, out)
	}
	return createdRequest{RequestID: out.RequestID, SourceID: sourceID, Token: out.AcceptanceToken}
}

func createInput(user, tenant, company, carrier, source uuid.UUID, key, fingerprint string) service.CreateRelationshipInput {
	return service.CreateRelationshipInput{
		ServiceToken: shipmentRelationshipToken, Principal: domain.Principal{UserID: user, TenantID: tenant}, SelectedCompanyID: company,
		SourceType: domain.SourceTypeShipment, SourceID: source, Fingerprint: fingerprint,
		ShipperCarrierCompanyID: carrier, IdempotencyKey: key,
	}
}

type writeCounts struct {
	requests, events, idempotency, audit, outbox int
}

func relationshipWriteCounts(t *testing.T, pool *pgxpool.Pool) writeCounts {
	t.Helper()
	var counts writeCounts
	err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM company.carrier_relationship_requests),
			(SELECT count(*) FROM company.carrier_relationship_request_events),
			(SELECT count(*) FROM company.carrier_relationship_request_idempotency),
			(SELECT count(*) FROM company.carrier_relationship_audit),
			(SELECT count(*) FROM company.carrier_relationship_outbox)
	`).Scan(&counts.requests, &counts.events, &counts.idempotency, &counts.audit, &counts.outbox)
	if err != nil {
		t.Fatal(err)
	}
	return counts
}

func assertSourceAuthority(t *testing.T, pool *pgxpool.Pool, match, closed *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	ctx := context.Background()
	rfxSource := uuid.New()
	cat.allow(domain.SourceCreateSubject{
		SourceType: domain.SourceTypeRfxAward, SourceID: rfxSource, SourceTenantID: fix.TenantA, Fingerprint: "v1",
		ShipperCarrierCompanyID: fix.CarrierA, RequestingCompanyID: fix.ShipperA, RequestingUserID: fix.ShipperAdmin,
	})
	rfxInput := createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, rfxSource, "rfx-create", "v1")
	rfxInput.SourceType = domain.SourceTypeRfxAward
	rfxInput.ServiceToken = rfxRelationshipToken
	rfxCreated, err := match.Create(ctx, rfxInput)
	if err != nil || !rfxCreated.Created || rfxCreated.AcceptanceToken == "" {
		t.Fatalf("rfx identity create: %v %+v", err, rfxCreated)
	}
	var rfxCaller string
	if err := pool.QueryRow(ctx, `SELECT caller_service FROM company.carrier_relationship_request_idempotency WHERE relationship_request_id = $1`, rfxCreated.RequestID).Scan(&rfxCaller); err != nil || rfxCaller != domain.CallerRfxService {
		t.Fatalf("rfx caller_service=%s err=%v", rfxCaller, err)
	}

	before := relationshipWriteCounts(t, pool)
	missing := createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "missing-credential", "v1")
	missing.ServiceToken = ""
	_, err = match.Create(ctx, missing)
	assertCode(t, err, apperrors.CodeAuthenticationRequired)
	wrong := missing
	wrong.ServiceToken = "shared-internal-token"
	_, err = match.Create(ctx, wrong)
	assertCode(t, err, apperrors.CodeAuthenticationRequired)

	shipmentForRfx := createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, rfxSource, "shipment-for-rfx", "v1")
	shipmentForRfx.SourceType = domain.SourceTypeRfxAward
	_, err = match.Create(ctx, shipmentForRfx)
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	rfxForShipment := createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "rfx-for-shipment", "v1")
	rfxForShipment.ServiceToken = rfxRelationshipToken
	_, err = match.Create(ctx, rfxForShipment)
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)

	_, err = match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "missing-source", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	if cat.last != domain.SourceCreateNotFound {
		t.Fatalf("missing source decision %s", cat.last)
	}
	foreignSource := uuid.New()
	cat.allow(domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: foreignSource, SourceTenantID: fix.TenantB, Fingerprint: "v1",
		ShipperCarrierCompanyID: fix.CarrierA, RequestingCompanyID: fix.ShipperA, RequestingUserID: fix.ShipperAdmin,
	})
	_, err = match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, foreignSource, "foreign-source", "v1"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	if cat.last != domain.SourceCreateTenantMismatch {
		t.Fatalf("cross-tenant source decision %s", cat.last)
	}
	staleSource := uuid.New()
	cat.allow(domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: staleSource, SourceTenantID: fix.TenantA, Fingerprint: "current",
		ShipperCarrierCompanyID: fix.CarrierA, RequestingCompanyID: fix.ShipperA, RequestingUserID: fix.ShipperAdmin,
	})
	_, err = match.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, staleSource, "stale-source", "old"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	if cat.last != domain.SourceCreateStale {
		t.Fatalf("stale source decision %s", cat.last)
	}
	_, err = closed.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, staleSource, "fail-closed-create", "current"))
	assertCode(t, err, apperrors.CodeRelationshipSourceUnavailable)
	assertConcurrentUnconfirmed(t, match, fix)
	if relationshipWriteCounts(t, pool) != before {
		t.Fatal("source denial wrote request, event, idempotency, audit, or outbox rows")
	}
	t.Log("SOURCE_AUTHORITY=PASS")
}

func assertConcurrentUnconfirmed(t *testing.T, svc *service.RelationshipService, fix fixture) {
	t.Helper()
	in := createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, uuid.New(), "unconfirmed-concurrent", "v1")
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Create(context.Background(), in)
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		assertCode(t, errs[i], apperrors.CodeRelationshipRequestForbidden)
	}
}

func assertTokenStoredOnlyAsHMAC(t *testing.T, pool *pgxpool.Pool, requestID uuid.UUID, encoded string) {
	t.Helper()
	raw, err := domain.DecodeAcceptanceToken(encoded)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.FingerprintAcceptanceToken("postgres-pepper", raw)
	var stored []byte
	if err := pool.QueryRow(context.Background(), `SELECT acceptance_token_hash FROM company.carrier_relationship_requests WHERE relationship_request_id = $1`, requestID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(stored, want) || hmac.Equal(stored, raw) {
		t.Fatal("stored token hash is not the HMAC fingerprint")
	}
	var auditText, outboxText string
	if err := pool.QueryRow(context.Background(), `
		SELECT COALESCE(string_agg(COALESCE(idempotency_key, '') || COALESCE(reason, '') || COALESCE(correlation_id, ''), ' '), '')
		FROM company.carrier_relationship_audit
	`).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COALESCE(string_agg(payload::text, ' '), '') FROM company.carrier_relationship_outbox`).Scan(&outboxText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditText, encoded) || strings.Contains(outboxText, encoded) || strings.Contains(auditText, "postgres-pepper") || strings.Contains(outboxText, "postgres-pepper") {
		t.Fatal("audit or outbox contains the raw token or pepper")
	}
}

func assertDirectTenantGuards(t *testing.T, pool *pgxpool.Pool, fix fixture) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `UPDATE company.carrier_relationship_requests SET source_version_or_fingerprint = 'changed'`)
	if err == nil {
		t.Fatal("immutable request accepted an update")
	}
	hash := domain.FingerprintAcceptanceToken("postgres-pepper", bytes32(t))
	_, err = pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'x', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, uuid.New(), uuid.New(), fix.TenantA, fix.CarrierB, fix.ShipperA, fix.ShipperAdmin, hash)
	if err == nil {
		t.Fatal("cross-tenant carrier insert was accepted")
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'x', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, uuid.New(), uuid.New(), fix.TenantA, fix.CarrierA, fix.ShipperA, fix.BlockedUser, domain.FingerprintAcceptanceToken("postgres-pepper", bytes32(t)))
	if err == nil {
		t.Fatal("blocked user insert was accepted")
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'x', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, uuid.New(), uuid.New(), fix.TenantA, fix.CarrierA, fix.ShipperA, fix.DeletedUser, domain.FingerprintAcceptanceToken("postgres-pepper", bytes32(t)))
	if err == nil {
		t.Fatal("deleted user insert was accepted")
	}
}

func assertConcurrentCreate(t *testing.T, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	sourceID := uuid.New()
	cat.allow(domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: sourceID, SourceTenantID: fix.TenantA, Fingerprint: "v9",
		ShipperCarrierCompanyID: fix.CarrierA, RequestingCompanyID: fix.ShipperA, RequestingUserID: fix.ShipperAdmin,
	})
	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	outs := make([]service.CreateRelationshipOutput, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			outs[i], errs[i] = svc.Create(context.Background(), createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, sourceID, "concurrent-key", "v9"))
		}(i)
	}
	close(start)
	wg.Wait()
	created := 0
	var requestID uuid.UUID
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent create %d: %v", i, errs[i])
		}
		if outs[i].Created {
			created++
		}
		if requestID == uuid.Nil {
			requestID = outs[i].RequestID
		} else if outs[i].RequestID != requestID {
			t.Fatal("concurrent create returned different request ids")
		}
		if !outs[i].Created && outs[i].AcceptanceToken != "" {
			t.Fatal("concurrent replay returned a token")
		}
	}
	if created != 1 {
		t.Fatalf("concurrent creates=%d", created)
	}
}

func bytes32(t *testing.T) []byte {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	return raw
}

func uniqueTokenHash(t *testing.T) []byte {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return domain.FingerprintAcceptanceToken("postgres-pepper", raw)
}

type fixture struct {
	TenantA, TenantB                                         uuid.UUID
	ShipperA, CarrierA, CarrierB, OperatingB, OperatingB2    uuid.UUID
	ShipperAdmin, BlockedUser, DeletedUser                   uuid.UUID
	PlatformNoMembership, PlatformWithMembership             uuid.UUID
	CompanyScopedPlatformAdmin, CarrierAdmin, OrdinaryMember uuid.UUID
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantA: uuid.New(), TenantB: uuid.New(), ShipperA: uuid.New(), CarrierA: uuid.New(), CarrierB: uuid.New(),
		OperatingB: uuid.New(), OperatingB2: uuid.New(), ShipperAdmin: uuid.New(), BlockedUser: uuid.New(), DeletedUser: uuid.New(),
		PlatformNoMembership: uuid.New(), PlatformWithMembership: uuid.New(), CompanyScopedPlatformAdmin: uuid.New(),
		CarrierAdmin: uuid.New(), OrdinaryMember: uuid.New(),
	}
	for _, row := range []struct {
		id   uuid.UUID
		code string
	}{{fix.TenantA, "a"}, {fix.TenantB, "b"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`, row.id, row.code+"-"+row.id.String()[:8], row.code); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		kind       string
	}{
		{fix.ShipperA, fix.TenantA, "SHIPPER"}, {fix.CarrierA, fix.TenantA, "CARRIER"}, {fix.CarrierB, fix.TenantB, "CARRIER"},
		{fix.OperatingB, fix.TenantB, "CARRIER"}, {fix.OperatingB2, fix.TenantB, "CARRIER"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES ($1, $2, $3, $4, 'ACTIVE')`, row.id, row.tenant, row.kind, row.id.String()); err != nil {
			t.Fatal(err)
		}
	}
	insertUser(t, pool, fix.ShipperAdmin, fix.TenantA, "ACTIVE", nil)
	insertUser(t, pool, fix.BlockedUser, fix.TenantA, "BLOCKED", nil)
	now := time.Now().UTC()
	insertUser(t, pool, fix.DeletedUser, fix.TenantA, "ACTIVE", &now)
	insertUser(t, pool, fix.PlatformNoMembership, fix.TenantA, "ACTIVE", nil)
	insertUser(t, pool, fix.PlatformWithMembership, fix.TenantA, "ACTIVE", nil)
	insertUser(t, pool, fix.CompanyScopedPlatformAdmin, fix.TenantA, "ACTIVE", nil)
	insertUser(t, pool, fix.CarrierAdmin, fix.TenantB, "ACTIVE", nil)
	insertUser(t, pool, fix.OrdinaryMember, fix.TenantB, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantA, fix.ShipperA, fix.ShipperAdmin)
	insertMembership(t, pool, fix.TenantA, fix.ShipperA, fix.BlockedUser)
	insertMembership(t, pool, fix.TenantA, fix.ShipperA, fix.PlatformWithMembership)
	insertMembership(t, pool, fix.TenantA, fix.ShipperA, fix.CompanyScopedPlatformAdmin)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB, fix.CarrierAdmin)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB2, fix.CarrierAdmin)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB, fix.OrdinaryMember)
	grantRole(t, pool, fix.TenantA, fix.ShipperAdmin, &fix.ShipperA, "SHIPPER_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.PlatformNoMembership, nil, "PLATFORM_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.PlatformWithMembership, nil, "PLATFORM_ADMIN")
	grantRole(t, pool, fix.TenantA, fix.CompanyScopedPlatformAdmin, &fix.ShipperA, "PLATFORM_ADMIN")
	grantRole(t, pool, fix.TenantB, fix.CarrierAdmin, &fix.OperatingB, "CARRIER_ADMIN")
	grantRole(t, pool, fix.TenantB, fix.CarrierAdmin, &fix.OperatingB2, "CARRIER_ADMIN")
	return fix
}

func insertUser(t *testing.T, pool *pgxpool.Pool, id, tenant uuid.UUID, status string, deleted *time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO core.users (id, tenant_id, email, full_name, status, deleted_at)
		VALUES ($1, $2, $3, 'User', $4, $5)
	`, id, tenant, id.String()+"@example.test", status, deleted)
	if err != nil {
		t.Fatal(err)
	}
}

func insertMembership(t *testing.T, pool *pgxpool.Pool, tenant, company, user uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1, $2, $3, 'ACTIVE')
	`, tenant, company, user)
	if err != nil {
		t.Fatal(err)
	}
}

func grantRole(t *testing.T, pool *pgxpool.Pool, tenant, user uuid.UUID, company *uuid.UUID, code string) {
	t.Helper()
	var roleID uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = $1 AND deleted_at IS NULL`, code).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)
	`, tenant, user, company, roleID)
	if err != nil {
		t.Fatal(err)
	}
}

func assertCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.AppError
	if err == nil || !asError(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
	if len(appErr.Details) != 0 {
		t.Fatalf("error details leaked: %v", appErr.Details)
	}
}

func asError(err error, target **apperrors.AppError) bool {
	app, ok := err.(*apperrors.AppError)
	if ok {
		*target = app
	}
	return ok
}

func coreSnapshot(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var tenants, companies, users, roles, memberships int
	err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM core.tenants),
			(SELECT count(*) FROM core.companies),
			(SELECT count(*) FROM core.users),
			(SELECT count(*) FROM core.roles),
			(SELECT count(*) FROM core.company_memberships)
	`).Scan(&tenants, &companies, &users, &roles, &memberships)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("tenants=%d companies=%d users=%d roles=%d memberships=%d", tenants, companies, users, roles, memberships)
}

func expectCompanySchema(ctx context.Context, pool *pgxpool.Pool, present bool) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'company')`).Scan(&exists); err != nil {
		return err
	}
	if exists != present {
		return fmt.Errorf("company schema present=%v want=%v", exists, present)
	}
	return nil
}

func assertDatabaseConstraints(t *testing.T, pool *pgxpool.Pool, fix fixture) {
	t.Helper()
	ctx := context.Background()
	names := map[string]bool{}
	rows, err := pool.Query(ctx, `
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_namespace n ON n.oid = c.connamespace
		WHERE n.nspname = 'company'
		UNION
		SELECT indexname FROM pg_indexes WHERE schemaname = 'company'
	`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names[name] = true
	}
	rows.Close()
	for _, name := range []string{
		"chk_crr_token_hash_len", "chk_crr_source_type", "chk_crr_event_type", "uq_cr_business_key",
		"uq_crr_one_pending_source_carrier", "uq_crr_acceptance_token_hash", "uq_crr_event_accepted",
		"carrier_relationship_requests_source_tenant_id_fkey",
	} {
		if !names[name] {
			t.Fatalf("missing company constraint or index %s", name)
		}
	}
	mustFail(t, pool, `UPDATE company.carrier_relationship_request_events SET request_status = 'CANCELLED'`)
	mustFail(t, pool, `DELETE FROM company.carrier_relationship_request_events`)
	mustFail(t, pool, `UPDATE company.carrier_relationship_versions SET status = 'REVOKED'`)
	mustFail(t, pool, `DELETE FROM company.carrier_relationships`)
	shortHash := []byte{1, 2, 3}
	mustFail(t, pool, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'x', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, uuid.New(), uuid.New(), fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, shortHash)
	mustFail(t, pool, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'NOT_A_SOURCE', $2, $3, 'x', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, uuid.New(), uuid.New(), fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, domain.FingerprintAcceptanceToken("postgres-pepper", bytes32(t)))
	requestID := uuid.New()
	sourceID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'pending-a', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, requestID, sourceID, fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, uniqueTokenHash(t)); err != nil {
		t.Fatal(err)
	}
	eventID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_events (
			event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
		) VALUES ($1, $2, 1, 'CREATED', 'PENDING', $3)
	`, eventID, requestID, fix.ShipperAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_heads (
			relationship_request_id, source_type, source_id, shipper_carrier_company_id, current_event_id, current_version, request_status
		) VALUES ($1, 'SHIPMENT', $2, $3, $4, 1, 'PENDING')
	`, requestID, sourceID, fix.CarrierA, eventID); err != nil {
		t.Fatal(err)
	}
	secondRequest := uuid.New()
	secondEvent := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'pending-b', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, secondRequest, sourceID, fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, uniqueTokenHash(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_events (
			event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
		) VALUES ($1, $2, 1, 'CREATED', 'PENDING', $3)
	`, secondEvent, secondRequest, fix.ShipperAdmin); err != nil {
		t.Fatal(err)
	}
	mustFail(t, pool, `
		INSERT INTO company.carrier_relationship_request_heads (
			relationship_request_id, source_type, source_id, shipper_carrier_company_id, current_event_id, current_version, request_status
		) VALUES ($1, 'SHIPMENT', $2, $3, $4, 1, 'PENDING')
	`, secondRequest, sourceID, fix.CarrierA, secondEvent)
	mustFail(t, pool, `
		INSERT INTO company.carrier_relationships (
			relationship_id, shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id, operating_carrier_company_id
		) VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), fix.TenantA, fix.CarrierA, fix.TenantB, fix.OperatingB2)
}

func assertRollbackIsAtomic(t *testing.T, pool *pgxpool.Pool, fix fixture) {
	t.Helper()
	ctx := context.Background()
	before := companySnapshot(t, pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requestID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'rollback', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, requestID, uuid.New(), fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, uniqueTokenHash(t)); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_outbox (outbox_id, event_type, aggregate_id, payload)
		VALUES ($1, 'not-an-event', $2, '{}'::jsonb)
	`, uuid.New(), requestID); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("invalid outbox event was accepted")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if after := companySnapshot(t, pool); after != before {
		t.Fatalf("failed outbox insert left partial rows: before=%s after=%s", before, after)
	}
}

func companySnapshot(t *testing.T, pool *pgxpool.Pool) string {
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
	return fmt.Sprintf("requests=%d events=%d outbox=%d relationships=%d", requests, events, outbox, relationships)
}

func mustFail(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	tag, err := pool.Exec(context.Background(), query, args...)
	if err == nil {
		t.Fatalf("expected database rejection, affected=%d query=%s", tag.RowsAffected(), query)
	}
}

func setupPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required for the PostgreSQL gate")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres: %v", err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Logf("isolated database=%s", dbName)
	t.Log("MIGRATION_UP=PASS")
	return pool
}

func cycleFoundationMigration(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	up, err := os.ReadFile(filepath.Join(dir, "000100_company_carrier_relationship_foundation_v0_1c.up.sql"))
	if err != nil {
		return err
	}
	down, err := os.ReadFile(filepath.Join(dir, "000100_company_carrier_relationship_foundation_v0_1c.down.sql"))
	if err != nil {
		return err
	}
	if err := expectCompanySchema(ctx, pool, true); err != nil {
		return err
	}
	for _, present := range []bool{false, true, false, true} {
		script := down
		if present {
			script = up
		}
		if _, err := pool.Exec(ctx, string(script)); err != nil {
			return err
		}
		if err := expectCompanySchema(ctx, pool, present); err != nil {
			return err
		}
	}
	return nil
}

func createTempDatabase(ctx context.Context, adminURL string) (string, string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, err
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName := "rel_found_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = adminDB
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, err
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, err
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(testCfg.ConnConfig.User), url.QueryEscape(testCfg.ConnConfig.Password),
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, testCfg.ConnConfig.Database)
	cleanup := func(cctx context.Context) {
		p, _ := pgxpool.NewWithConfig(cctx, adminCfg)
		if p != nil {
			defer p.Close()
			_, _ = p.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		}
	}
	return dbName, testURL, cleanup, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

func migrationsDir() (string, error) {
	candidate := filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations")
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate, nil
	}
	return "", os.ErrNotExist
}

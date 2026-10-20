//go:build integration

package relationship

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
)

func TestPublicAcceptPostgres(t *testing.T) {
	pool := setupPostgres(t)
	runPublicAcceptScenarios(t, pool)
}

func runPublicAcceptScenarios(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	fix := seedFixture(t, pool)
	repo := repository.NewRelationshipRepository(pool)
	cat := newCatalog()
	source := &countingSource{decision: domain.SourceMatch}
	svc := service.NewRelationshipService(repo, repo, postgresRelationshipConfig(), source, cat)
	ctx := context.Background()

	isolatedCarrier := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1, $2, 'CARRIER', $3, 'ACTIVE')
	`, isolatedCarrier, fix.TenantA, isolatedCarrier.String()); err != nil {
		t.Fatal(err)
	}
	assertPublicAcceptAccess(t, pool, svc, cat, fix)
	assertSiblingRace(t, pool, svc, cat, source, fix)
	endpoint, sibling := operatingCompanies(t, pool, fix, fix.CarrierA)
	source.decision = domain.SourceUnavailable
	source.calls = 0
	assertEndpointMismatchSkipsRevalidate(t, pool, svc, cat, source, fix, endpoint, sibling)
	source.decision = domain.SourceMatch
	source.calls = 0
	assertExpiredSiblingThenValidSibling(t, pool, svc, cat, fix, sibling)
	assertRevokedReactivation(t, pool, svc, cat, fix, endpoint, sibling)

	race := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, isolatedCarrier, "public-accept-cancel-race", "race")
	assertCancelAcceptRace(t, pool, svc, fix, race)
	if headStatus(t, pool, race.RequestID) == domain.RequestStatusPending {
		t.Fatal("accept/cancel race left the request pending")
	}

	replayed := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, isolatedCarrier, "public-accept-replay", "replay")
	first, err := svc.Accept(ctx, acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, replayed.Token, "lost-ack"))
	if err != nil || !first.Created {
		t.Fatalf("lost-ack first accept: %v %+v", err, first)
	}
	again, err := svc.Accept(ctx, acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, "", "lost-ack"))
	if err != nil || again.Created || again.RelationshipID != first.RelationshipID || again.RelationshipVersion != first.RelationshipVersion {
		t.Fatalf("lost-ack replay: %v %+v", err, again)
	}
	if err != nil && strings.Contains(err.Error(), fix.OperatingB2.String()) {
		t.Fatal("replay error revealed a company")
	}
}

func assertPublicAcceptAccess(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	ctx := context.Background()
	created := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "access-create", "access")
	inactiveUser := uuid.New()
	insertUser(t, pool, inactiveUser, fix.TenantB, "ACTIVE", nil)
	if _, err := pool.Exec(ctx, `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1, $2, $3, 'INACTIVE')
	`, fix.TenantB, fix.OperatingB, inactiveUser); err != nil {
		t.Fatal(err)
	}
	grantRole(t, pool, fix.TenantB, inactiveUser, &fix.OperatingB, "CARRIER_ADMIN")
	_, err := svc.Accept(ctx, acceptInput(inactiveUser, fix.TenantB, fix.OperatingB, created.Token, "inactive-membership"))
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)
	if headStatus(t, pool, created.RequestID) != domain.RequestStatusPending {
		t.Fatal("inactive membership consumed the request")
	}

	escalated := uuid.New()
	insertUser(t, pool, escalated, fix.TenantB, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB2, escalated)
	grantRole(t, pool, fix.TenantB, escalated, &fix.OperatingB, "CARRIER_ADMIN")
	_, err = svc.Accept(ctx, acceptInput(escalated, fix.TenantB, fix.OperatingB2, created.Token, "role-escalation"))
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)

	_, err = svc.Accept(ctx, acceptInput(fix.ShipperAdmin, fix.TenantA, fix.OperatingB, created.Token, "tenant-isolation"))
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)
	otherTenant := fix.TenantA
	spoof := acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, created.Token, "tenant-spoof")
	spoof.BodyTenantID = &otherTenant
	_, err = svc.Accept(ctx, spoof)
	assertCode(t, err, apperrors.CodeTenantContextConflict)
	otherCompany := fix.OperatingB2
	companySpoof := acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, created.Token, "company-spoof")
	companySpoof.BodyCompanyID = &otherCompany
	_, err = svc.Accept(ctx, companySpoof)
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)
	if headStatus(t, pool, created.RequestID) != domain.RequestStatusPending {
		t.Fatal("refused accept consumed the request")
	}
}

func assertSiblingRace(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, source *countingSource, fix fixture) {
	t.Helper()
	left := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "race-left", "race-left")
	right := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "race-right", "race-right")
	before := source.calls
	var wg sync.WaitGroup
	type result struct {
		out service.AcceptRelationshipOutput
		err error
		id  uuid.UUID
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, item := range []struct {
		request createdRequest
		company uuid.UUID
		key     string
	}{
		{left, fix.OperatingB, "sibling-left"},
		{right, fix.OperatingB2, "sibling-right"},
	} {
		wg.Add(1)
		go func(request createdRequest, company uuid.UUID, key string) {
			defer wg.Done()
			<-start
			out, err := svc.Accept(context.Background(), acceptInput(fix.CarrierAdmin, fix.TenantB, company, request.Token, key))
			results <- result{out: out, err: err, id: request.RequestID}
		}(item.request, item.company, item.key)
	}
	close(start)
	wg.Wait()
	close(results)
	var won, lost result
	seen := 0
	for item := range results {
		seen++
		if item.err == nil && item.out.Created {
			won = item
			continue
		}
		lost = item
	}
	if seen != 2 || won.id == uuid.Nil || lost.id == uuid.Nil || won.id == lost.id {
		t.Fatalf("sibling race won=%s lost=%s seen=%d", won.id, lost.id, seen)
	}
	assertCode(t, lost.err, apperrors.CodeRelationshipAcceptanceForbidden)
	if strings.Contains(lost.err.Error(), fix.OperatingB.String()) || strings.Contains(lost.err.Error(), fix.OperatingB2.String()) {
		t.Fatal("sibling 403 revealed an operating company")
	}
	if headStatus(t, pool, lost.id) != domain.RequestStatusPending || headStatus(t, pool, won.id) != domain.RequestStatusAccepted {
		t.Fatalf("race statuses winner=%s loser=%s", headStatus(t, pool, won.id), headStatus(t, pool, lost.id))
	}
	var roots int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM company.carrier_relationships
		WHERE shipper_tenant_id = $1 AND shipper_carrier_company_id = $2 AND operating_tenant_id = $3
	`, fix.TenantA, fix.CarrierA, fix.TenantB).Scan(&roots); err != nil || roots != 1 {
		t.Fatalf("roots=%d err=%v", roots, err)
	}
	if source.calls-before != 1 {
		t.Fatalf("revalidate calls during locked race=%d", source.calls-before)
	}
}

func assertEndpointMismatchSkipsRevalidate(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, source *countingSource, fix fixture, endpoint, sibling uuid.UUID) {
	t.Helper()
	created := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "mismatch-token", "mismatch")
	before := source.calls
	_, err := svc.Accept(context.Background(), acceptInput(fix.CarrierAdmin, fix.TenantB, sibling, created.Token, "mismatch-sibling"))
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)
	if source.calls != before {
		t.Fatalf("sibling mismatch called Revalidate calls=%d before=%d", source.calls, before)
	}
	if headStatus(t, pool, created.RequestID) != domain.RequestStatusPending {
		t.Fatal("sibling mismatch consumed the request")
	}
	_, err = svc.Accept(context.Background(), acceptInput(fix.CarrierAdmin, fix.TenantB, endpoint, created.Token, "mismatch-owner"))
	assertCode(t, err, apperrors.CodeRelationshipSourceUnavailable)
	if source.calls != before+1 {
		t.Fatalf("matching unavailable source calls=%d before=%d", source.calls, before)
	}
	if headStatus(t, pool, created.RequestID) != domain.RequestStatusPending {
		t.Fatal("unavailable source consumed the request")
	}
}

func assertExpiredSiblingThenValidSibling(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture, sibling uuid.UUID) {
	t.Helper()
	expired := insertExpiredAcceptRequest(t, pool, fix)
	_, err := svc.Accept(context.Background(), acceptInput(fix.CarrierAdmin, fix.TenantB, sibling, expired.Token, "expired-sibling"))
	assertCode(t, err, apperrors.CodeRelationshipRequestExpired)
	if headStatus(t, pool, expired.RequestID) != domain.RequestStatusExpired || terminalEvents(t, pool, expired.RequestID) != 1 {
		t.Fatalf("expired head=%s events=%d", headStatus(t, pool, expired.RequestID), terminalEvents(t, pool, expired.RequestID))
	}
	var expiredEvents int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM company.carrier_relationship_request_events
		WHERE relationship_request_id = $1 AND event_type = 'EXPIRED'
	`, expired.RequestID).Scan(&expiredEvents); err != nil || expiredEvents != 1 {
		t.Fatalf("expired events=%d err=%v", expiredEvents, err)
	}
	valid := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "valid-sibling", "valid-sibling")
	_, err = svc.Accept(context.Background(), acceptInput(fix.CarrierAdmin, fix.TenantB, sibling, valid.Token, "valid-sibling-accept"))
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)
	if headStatus(t, pool, valid.RequestID) != domain.RequestStatusPending {
		t.Fatal("valid sibling token was consumed")
	}
}

func assertRevokedReactivation(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture, endpoint, sibling uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var relationshipID uuid.UUID
	var version int
	if err := pool.QueryRow(ctx, `
		SELECT relationship_id FROM company.carrier_relationships
		WHERE shipper_tenant_id = $1 AND shipper_carrier_company_id = $2 AND operating_tenant_id = $3
	`, fix.TenantA, fix.CarrierA, fix.TenantB).Scan(&relationshipID); err != nil {
		t.Fatal(err)
	}
	revokedVersion := markRelationshipRevoked(t, pool, relationshipID, fix.CarrierAdmin)
	if err := pool.QueryRow(ctx, `SELECT current_version FROM company.carrier_relationship_heads WHERE relationship_id = $1 AND status = 'REVOKED'`, relationshipID).Scan(&version); err != nil || version != revokedVersion {
		t.Fatalf("revoked head version=%d err=%v", version, err)
	}
	siblingToken := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "revoked-sibling", "revoked-sibling")
	_, err := svc.Accept(ctx, acceptInput(fix.CarrierAdmin, fix.TenantB, sibling, siblingToken.Token, "revoked-sibling-accept"))
	assertCode(t, err, apperrors.CodeRelationshipAcceptanceForbidden)
	if headStatus(t, pool, siblingToken.RequestID) != domain.RequestStatusPending {
		t.Fatal("sibling reactivation consumed the request")
	}
	var headStatusValue string
	var headVersion int
	if err := pool.QueryRow(ctx, `SELECT status, current_version FROM company.carrier_relationship_heads WHERE relationship_id = $1`, relationshipID).Scan(&headStatusValue, &headVersion); err != nil {
		t.Fatal(err)
	}
	if headStatusValue != domain.RelationshipStatusRevoked || headVersion != revokedVersion {
		t.Fatalf("sibling changed revoked head status=%s version=%d", headStatusValue, headVersion)
	}
	var activeAfterSibling int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM company.carrier_relationship_versions
		WHERE relationship_id = $1 AND status = 'ACTIVE' AND relationship_version > $2
	`, relationshipID, revokedVersion).Scan(&activeAfterSibling); err != nil || activeAfterSibling != 0 {
		t.Fatalf("sibling inserted active versions=%d err=%v", activeAfterSibling, err)
	}
	matching := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "revoked-match", "revoked-match")
	out, err := svc.Accept(ctx, acceptInput(fix.CarrierAdmin, fix.TenantB, endpoint, matching.Token, "revoked-match-accept"))
	if err != nil || !out.Created || out.RelationshipVersion != revokedVersion+1 {
		t.Fatalf("reactivation: %v %+v want version %d", err, out, revokedVersion+1)
	}
	var revokedRemains int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM company.carrier_relationship_versions
		WHERE relationship_id = $1 AND relationship_version = $2 AND status = 'REVOKED'
	`, relationshipID, revokedVersion).Scan(&revokedRemains); err != nil || revokedRemains != 1 {
		t.Fatalf("revoked version remains=%d err=%v", revokedRemains, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status, current_version FROM company.carrier_relationship_heads WHERE relationship_id = $1`, relationshipID).Scan(&headStatusValue, &headVersion); err != nil {
		t.Fatal(err)
	}
	if headStatusValue != domain.RelationshipStatusActive || headVersion != revokedVersion+1 {
		t.Fatalf("reactivated head status=%s version=%d", headStatusValue, headVersion)
	}
}

func acceptInput(user, tenant, company uuid.UUID, token, key string) service.AcceptRelationshipInput {
	return service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: user, TenantID: tenant}, SelectedCompanyID: company,
		AcceptanceToken: token, IdempotencyKey: key,
	}
}

func markRelationshipRevoked(t *testing.T, pool *pgxpool.Pool, relationshipID, actor uuid.UUID) int {
	t.Helper()
	ctx := context.Background()
	var version int
	var versionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT current_version, current_version_id
		FROM company.carrier_relationship_heads WHERE relationship_id = $1
	`, relationshipID).Scan(&version, &versionID); err != nil {
		t.Fatal(err)
	}
	var requestID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT source_relationship_request_id FROM company.carrier_relationship_versions WHERE relationship_version_id = $1
	`, versionID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	next := version + 1
	nextID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by
		) VALUES ($1, $2, $3, 'REVOKED', $4, $5, $6)
	`, nextID, relationshipID, next, versionID, requestID, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE company.carrier_relationship_heads
		SET current_version = $2, current_version_id = $3, status = 'REVOKED'
		WHERE relationship_id = $1
	`, relationshipID, next, nextID); err != nil {
		t.Fatal(err)
	}
	return next
}

func insertExpiredAcceptRequest(t *testing.T, pool *pgxpool.Pool, fix fixture) createdRequest {
	t.Helper()
	ctx := context.Background()
	raw, err := domain.NewAcceptanceToken()
	if err != nil {
		t.Fatal(err)
	}
	requestID := uuid.New()
	eventID := uuid.New()
	sourceID := uuid.New()
	hash := domain.FingerprintAcceptanceToken("postgres-pepper", raw)
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'expired', $4, $5, $6, $7, ts.t, ts.t + interval '72 hours', ts.t, $6
		FROM (SELECT clock_timestamp() - interval '73 hours' AS t) ts
	`, requestID, sourceID, fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_events (
			event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
		) VALUES ($1, $2, 1, 'CREATED', 'PENDING', $3)
	`, eventID, requestID, fix.ShipperAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_heads (
			relationship_request_id, source_type, source_id, shipper_carrier_company_id,
			current_event_id, current_version, request_status
		) VALUES ($1, 'SHIPMENT', $2, $3, $4, 1, 'PENDING')
	`, requestID, sourceID, fix.CarrierA, eventID); err != nil {
		t.Fatal(err)
	}
	return createdRequest{RequestID: requestID, SourceID: sourceID, Token: domain.EncodeAcceptanceToken(raw)}
}

func operatingCompanies(t *testing.T, pool *pgxpool.Pool, fix fixture, carrier uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var stored uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		SELECT operating_carrier_company_id FROM company.carrier_relationships
		WHERE shipper_tenant_id = $1 AND shipper_carrier_company_id = $2 AND operating_tenant_id = $3
	`, fix.TenantA, carrier, fix.TenantB).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == fix.OperatingB {
		return fix.OperatingB, fix.OperatingB2
	}
	if stored == fix.OperatingB2 {
		return fix.OperatingB2, fix.OperatingB
	}
	t.Fatalf("stored operating company %s is outside the sibling pair", stored)
	return uuid.Nil, uuid.Nil
}

type countingSource struct {
	mu       sync.Mutex
	calls    int
	decision domain.SourceDecision
}

func (c *countingSource) Revalidate(context.Context, domain.SourceSnapshot) (domain.SourceDecision, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.decision, nil
}

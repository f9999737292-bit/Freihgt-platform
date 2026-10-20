//go:build integration

package relationship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
)

func applyRevokeDown(ctx context.Context, pool *pgxpool.Pool) error {
	return execMigration(ctx, pool, "000102_company_carrier_relationship_revoke_v0_2.down.sql")
}

func applyRevokeUp(ctx context.Context, pool *pgxpool.Pool) error {
	return execMigration(ctx, pool, "000102_company_carrier_relationship_revoke_v0_2.up.sql")
}

func execMigration(ctx context.Context, pool *pgxpool.Pool, name string) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(raw))
	return err
}

func runRevokeScenarios(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	defer func() { repository.RevokeBeforeCommitHook = nil }()
	ctx := context.Background()
	fix := seedFixture(t, pool)
	repo := repository.NewRelationshipRepository(pool)
	cat := newCatalog()
	svc := service.NewRelationshipService(repo, repo, postgresRelationshipConfig(), matchSource{}, cat)
	disabled := service.NewRelationshipService(repo, repo, service.RelationshipConfig{TokenPepper: "postgres-pepper"}, matchSource{}, cat)

	leftCarrier := insertCarrier(t, pool, fix.TenantA)
	rightCarrier := insertCarrier(t, pool, fix.TenantA)
	leftID, leftRequest := acceptCarrier(t, svc, cat, fix, leftCarrier, "revoke-left")
	rightID, _ := acceptCarrier(t, svc, cat, fix, rightCarrier, "revoke-right")
	beforeLeft := versionRow(t, pool, leftID, 1)
	left, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, leftID, "same-key", "shipper ended"))
	if err != nil || left.Status != domain.RelationshipStatusRevoked || left.RelationshipVersion != 2 || left.RelationshipID != leftID {
		t.Fatalf("shipper revoke: %v %+v", err, left)
	}
	if versionRow(t, pool, leftID, 1) != beforeLeft {
		t.Fatal("revoke rewrote the active version")
	}
	if headStatus(t, pool, leftRequest.RequestID) != domain.RequestStatusAccepted {
		t.Fatal("revoke changed the accepted request")
	}
	assertRevokedHead(t, pool, leftID, 2)
	assertRevokeWrites(t, pool, leftID, 1, 1, 1)
	assertRevokePayload(t, pool, leftID)
	right, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, rightID, "same-key", "other relationship"))
	if err != nil || right.RelationshipVersion != 2 || codeOf(err) == apperrors.CodeRelationshipIdempotencyConflict {
		t.Fatalf("same key on another relationship: %v %+v", err, right)
	}

	replayExpected := 1
	replayed, err := svc.Revoke(ctx, withExpected(revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, leftID, "same-key", "changed reason"), &replayExpected))
	if err != nil || replayed.RelationshipVersion != 2 {
		t.Fatalf("replay: %v %+v", err, replayed)
	}
	assertRevokeWrites(t, pool, leftID, 1, 1, 1)
	observed, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, leftID, "new-key", "again"))
	if err != nil || observed.RelationshipVersion != 2 {
		t.Fatalf("new key: %v %+v", err, observed)
	}
	assertRevokeWrites(t, pool, leftID, 1, 1, 2)

	carrierCarrier := insertCarrier(t, pool, fix.TenantA)
	carrierRel, _ := acceptCarrier(t, svc, cat, fix, carrierCarrier, "revoke-carrier")
	carrierOut, err := svc.Revoke(ctx, revokeInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, carrierRel, "carrier-key", ""))
	if err != nil || carrierOut.RelationshipVersion != 2 {
		t.Fatalf("carrier revoke: %v %+v", err, carrierOut)
	}
	var storedReason *string
	if err := pool.QueryRow(ctx, `
		SELECT revoke_reason FROM company.carrier_relationship_versions
		WHERE relationship_id = $1 AND relationship_version = 2
	`, carrierRel).Scan(&storedReason); err != nil || storedReason != nil {
		t.Fatalf("empty reason stored as %v err=%v", storedReason, err)
	}

	conflictCarrier := insertCarrier(t, pool, fix.TenantA)
	conflictRel, _ := acceptCarrier(t, svc, cat, fix, conflictCarrier, "revoke-conflict")
	wrong := 99
	_, err = svc.Revoke(ctx, withExpected(revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, conflictRel, "conflict-key", "no"), &wrong))
	assertCode(t, err, apperrors.CodeRelationshipVersionConflict)
	if revokeKeyCount(t, pool, conflictRel) != 0 || versionCount(t, pool, conflictRel) != 1 {
		t.Fatal("version conflict wrote a revoke")
	}
	if _, err := svc.Revoke(ctx, withExpected(revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, conflictRel, "conflict-key", "yes"), intPtr(1))); err != nil {
		t.Fatal(err)
	}

	_, err = disabled.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, leftID, "disabled", "no"))
	assertCode(t, err, apperrors.CodeRelationshipCommandsDisabled)
	assertRevokeWrites(t, pool, leftID, 1, 1, 2)

	assertRevokeDenials(t, pool, svc, fix, leftID)
	assertSameTenantOperatingPredicate(t, pool, svc, cat, fix)
	assertRevokeRollbackAndRetry(t, pool, svc, cat, fix)
	assertRevokeConcurrency(t, pool, svc, cat, fix)
	assertRevokeAcceptConcurrency(t, pool, svc, cat, fix)
	assertHistoricalRowsUnchanged(t, pool, svc, cat, fix)
	assertRevokeReactivation(t, pool, svc, cat, fix)
	assertRevokeConstraints(t, pool, fix, leftID, leftRequest.RequestID)
	if _, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, leftRequest.SourceID, leftRequest.RequestID, "cancel-after-revoke", "no")); codeOf(err) != apperrors.CodeRelationshipRequestAlreadyAccepted {
		t.Fatalf("cancel after revoke: %v", err)
	}
	assertRevokedHead(t, pool, leftID, 2)
}

func assertRevokeDenials(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, fix fixture, relationshipID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	before := revokeKeyCount(t, pool, relationshipID)
	foreignTenant := uuid.New()
	foreignCompany := uuid.New()
	foreignUser := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, 'foreign')`, foreignTenant, "f-"+foreignTenant.String()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES ($1, $2, 'SHIPPER', $3, 'ACTIVE')`, foreignCompany, foreignTenant, foreignCompany.String()); err != nil {
		t.Fatal(err)
	}
	insertUser(t, pool, foreignUser, foreignTenant, "ACTIVE", nil)
	insertMembership(t, pool, foreignTenant, foreignCompany, foreignUser)
	grantRole(t, pool, foreignTenant, foreignUser, &foreignCompany, "SHIPPER_ADMIN")
	_, err := svc.Revoke(ctx, revokeInput(foreignUser, foreignTenant, foreignCompany, relationshipID, "foreign", "no"))
	assertCode(t, err, apperrors.CodeRelationshipNotFound)
	if stringsContainCompany(err, fix.OperatingB) || stringsContainCompany(err, fix.ShipperA) {
		t.Fatal("foreign tenant error revealed an endpoint")
	}
	_, err = svc.Revoke(ctx, revokeInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB2, relationshipID, "sibling", "no"))
	assertCode(t, err, apperrors.CodeRelationshipNotFound)
	inactive := uuid.New()
	insertUser(t, pool, inactive, fix.TenantB, "ACTIVE", nil)
	if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status) VALUES ($1, $2, $3, 'INACTIVE')`, fix.TenantB, fix.OperatingB, inactive); err != nil {
		t.Fatal(err)
	}
	grantRole(t, pool, fix.TenantB, inactive, &fix.OperatingB, "CARRIER_ADMIN")
	_, err = svc.Revoke(ctx, revokeInput(inactive, fix.TenantB, fix.OperatingB, relationshipID, "inactive", "no"))
	assertCode(t, err, apperrors.CodeRelationshipNotFound)
	_, err = svc.Revoke(ctx, revokeInput(fix.PlatformNoMembership, fix.TenantA, fix.ShipperA, relationshipID, "platform-none", "no"))
	assertCode(t, err, apperrors.CodeRelationshipNotFound)
	_, err = svc.Revoke(ctx, revokeInput(fix.CompanyScopedPlatformAdmin, fix.TenantA, fix.ShipperA, relationshipID, "scoped-platform", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRevokeForbidden)
	_, err = svc.Revoke(ctx, revokeInput(fix.OrdinaryMember, fix.TenantB, fix.OperatingB, relationshipID, "ordinary", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRevokeForbidden)
	inspector := uuid.New()
	insertUser(t, pool, inspector, fix.TenantB, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB, inspector)
	grantRole(t, pool, fix.TenantB, inspector, &fix.OperatingB, "GOV_INSPECTOR")
	_, err = svc.Revoke(ctx, revokeInput(inspector, fix.TenantB, fix.OperatingB, relationshipID, "inspector", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRevokeForbidden)
	dispatcher := uuid.New()
	insertUser(t, pool, dispatcher, fix.TenantB, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB, dispatcher)
	grantRole(t, pool, fix.TenantB, dispatcher, &fix.OperatingB, "CARRIER_DISPATCHER")
	_, err = svc.Revoke(ctx, revokeInput(dispatcher, fix.TenantB, fix.OperatingB, relationshipID, "dispatcher", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRevokeForbidden)
	platform := uuid.New()
	insertUser(t, pool, platform, fix.TenantB, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantB, fix.OperatingB, platform)
	grantRole(t, pool, fix.TenantB, platform, nil, "PLATFORM_ADMIN")
	if _, err := svc.Revoke(ctx, revokeInput(platform, fix.TenantB, fix.OperatingB, relationshipID, "platform-operating", "admin")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Revoke(ctx, revokeInput(fix.PlatformWithMembership, fix.TenantA, fix.ShipperA, relationshipID, "platform-shipper", "admin")); err != nil {
		t.Fatal(err)
	}
	if revokeKeyCount(t, pool, relationshipID) != before+2 {
		t.Fatalf("denials stored idempotency keys=%d before=%d", revokeKeyCount(t, pool, relationshipID), before)
	}
}

func assertSameTenantOperatingPredicate(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	operating := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES ($1, $2, 'CARRIER', $3, 'ACTIVE')`, operating, fix.TenantA, operating.String()); err != nil {
		t.Fatal(err)
	}
	operator := uuid.New()
	shipperOnly := uuid.New()
	insertUser(t, pool, operator, fix.TenantA, "ACTIVE", nil)
	insertUser(t, pool, shipperOnly, fix.TenantA, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantA, operating, operator)
	insertMembership(t, pool, fix.TenantA, operating, shipperOnly)
	grantRole(t, pool, fix.TenantA, operator, &operating, "CARRIER_ADMIN")
	grantRole(t, pool, fix.TenantA, shipperOnly, &operating, "SHIPPER_ADMIN")
	carrier := insertCarrier(t, pool, fix.TenantA)
	created := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, carrier, "same-tenant-create", "same-tenant")
	accepted, err := svc.Accept(context.Background(), acceptInput(operator, fix.TenantA, operating, created.Token, "same-tenant-accept"))
	if err != nil || !accepted.Created {
		t.Fatalf("same-tenant accept: %v %+v", err, accepted)
	}
	_, err = svc.Revoke(context.Background(), revokeInput(shipperOnly, fix.TenantA, operating, accepted.RelationshipID, "same-tenant-shipper-role", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRevokeForbidden)
	if _, err := svc.Revoke(context.Background(), revokeInput(operator, fix.TenantA, operating, accepted.RelationshipID, "same-tenant-carrier", "yes")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Revoke(context.Background(), revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, accepted.RelationshipID, "same-tenant-requesting", "observe")); err != nil {
		t.Fatal(err)
	}
}

func assertRevokeRollbackAndRetry(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	carrier := insertCarrier(t, pool, fix.TenantA)
	relationshipID, _ := acceptCarrier(t, svc, cat, fix, carrier, "revoke-rollback")
	repository.RevokeBeforeCommitHook = func(context.Context) error { return errors.New("rollback") }
	_, err := svc.Revoke(context.Background(), revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "rollback-key", "lost"))
	repository.RevokeBeforeCommitHook = nil
	assertCode(t, err, apperrors.CodeRelationshipStoreUnavailable)
	if versionCount(t, pool, relationshipID) != 1 || revokeKeyCount(t, pool, relationshipID) != 0 || scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_audit WHERE relationship_id = $1 AND action = 'relationship_revoked'`, relationshipID) != 0 {
		t.Fatal("rolled back revoke left a success")
	}
	out, err := svc.Revoke(context.Background(), revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "rollback-key", "retry"))
	if err != nil || out.RelationshipVersion != 2 || revokeKeyCount(t, pool, relationshipID) != 1 {
		t.Fatalf("retry after rollback: %v %+v keys=%d", err, out, revokeKeyCount(t, pool, relationshipID))
	}
	again, err := svc.Revoke(context.Background(), revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "rollback-key", "lost ack"))
	if err != nil || again.RelationshipVersion != 2 || scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_audit WHERE relationship_id = $1 AND action = 'relationship_revoked'`, relationshipID) != 1 {
		t.Fatalf("lost ack: %v %+v", err, again)
	}
}

func assertRevokeConcurrency(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	carrier := insertCarrier(t, pool, fix.TenantA)
	relationshipID, _ := acceptCarrier(t, svc, cat, fix, carrier, "revoke-race")
	errCh := make(chan error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, call := range []service.RevokeRelationshipInput{
		revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "race-shipper", "race"),
		revokeInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, relationshipID, "race-carrier", "race"),
	} {
		wg.Add(1)
		go func(in service.RevokeRelationshipInput) {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_, err := svc.Revoke(ctx, in)
			errCh <- err
		}(call)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("revoke race: %v", err)
		}
	}
	assertRevokedHead(t, pool, relationshipID, 2)
	assertRevokeWrites(t, pool, relationshipID, 1, 1, 2)
}

func assertRevokeAcceptConcurrency(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	carrier := insertCarrier(t, pool, fix.TenantA)
	relationshipID, original := acceptCarrier(t, svc, cat, fix, carrier, "revoke-accept-race")
	pending := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, carrier, "revoke-accept-pending", "pending")
	errCh := make(chan error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "with-accept", "race"))
		errCh <- err
	}()
	go func() {
		defer wg.Done()
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := svc.Accept(ctx, acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, pending.Token, "with-revoke"))
		errCh <- err
	}()
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("revoke/accept race: %v", err)
		}
	}
	var headVersion, maxVersion, revokedVersions, roots int
	var headID uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		SELECT h.current_version, h.current_version_id,
		       (SELECT max(relationship_version) FROM company.carrier_relationship_versions WHERE relationship_id = h.relationship_id),
		       (SELECT count(*) FROM company.carrier_relationship_versions WHERE relationship_id = h.relationship_id AND status = 'REVOKED'),
		       (SELECT count(*) FROM company.carrier_relationships WHERE shipper_tenant_id = $2 AND shipper_carrier_company_id = $3 AND operating_tenant_id = $4)
		FROM company.carrier_relationship_heads h
		WHERE h.relationship_id = $1
	`, relationshipID, fix.TenantA, carrier, fix.TenantB).Scan(&headVersion, &headID, &maxVersion, &revokedVersions, &roots); err != nil {
		t.Fatal(err)
	}
	if headVersion != maxVersion || revokedVersions != 1 || roots != 1 {
		t.Fatalf("race head=%d max=%d revoked=%d roots=%d", headVersion, maxVersion, revokedVersions, roots)
	}
	if versionRowStatus(t, pool, relationshipID, 1) != domain.RelationshipStatusActive {
		t.Fatal("race rewrote version 1")
	}
	if headStatus(t, pool, original.RequestID) != domain.RequestStatusAccepted || headStatus(t, pool, pending.RequestID) != domain.RequestStatusAccepted {
		t.Fatal("race changed a request away from accepted")
	}
	if scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_outbox WHERE aggregate_id = $1 AND event_type = 'carrier_relationship.revoked'`, relationshipID) != 1 {
		t.Fatal("race wrote more than one revoke outbox fact")
	}
}

func assertHistoricalRowsUnchanged(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	ctx := context.Background()
	carrier := insertCarrier(t, pool, fix.TenantA)
	relationshipID, _ := acceptCarrier(t, svc, cat, fix, carrier, "revoke-history")
	executionID := uuid.New()
	revisionID := uuid.New()
	driverID := uuid.New()
	vehicleID := uuid.New()
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_executions (id, operating_tenant_id, carrier_company_id, vehicle_id, driver_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, executionID, fix.TenantB, fix.OperatingB, vehicleID, driverID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			status, version, contract_sha256, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 1, $5, 1, 'fingerprint', 'DEPOT_START', 'ACTIVE', 1, $6, $7, $7)
	`, revisionID, executionID, fix.TenantB, uuid.New(), uuid.New(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE transport.transport_executions SET current_revision_id = $2 WHERE id = $1`, executionID, revisionID); err != nil {
		t.Fatal(err)
	}
	shipmentID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO control_tower.shipment_status_projection (
			tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
			last_event_type, last_occurred_at, last_consumed_at
		) VALUES ($1, $2, 1, 'IN_TRANSIT', $3, $4, 'shipment.status', $5, $5)
	`, fix.TenantA, shipmentID, uuid.New(), uuid.New(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO billing.payment_obligations (
			tenant_id, obligation_number, payer_company_id, payee_company_id, source_type, source_id,
			currency_code, original_amount, outstanding_amount
		) VALUES ($1, $2, $3, $4, 'BILLING_REGISTER', $5, 'RUB', 10, 10)
	`, fix.TenantA, "ob-"+uuid.NewString()[:8], fix.ShipperA, fix.CarrierA, uuid.New()); err != nil {
		t.Fatal(err)
	}
	tables := []string{
		"transport.transport_executions",
		"transport.transport_execution_revisions",
		"transport.shipments",
		"control_tower.shipment_status_projection",
		"billing.payment_obligations",
		"documents.pod_delivery_action_upload_intent",
	}
	before := map[string]string{}
	for _, table := range tables {
		before[table] = tableDigest(t, pool, table)
	}
	if _, err := svc.Revoke(ctx, revokeInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, relationshipID, "history", "preserve")); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if tableDigest(t, pool, table) != before[table] {
			t.Fatalf("%s changed during revoke", table)
		}
	}
	var storedDriver, storedVehicle uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT driver_id, vehicle_id FROM transport.transport_executions WHERE id = $1`, executionID).Scan(&storedDriver, &storedVehicle); err != nil || storedDriver != driverID || storedVehicle != vehicleID {
		t.Fatalf("execution assignment changed driver=%s vehicle=%s err=%v", storedDriver, storedVehicle, err)
	}
}

func assertRevokeReactivation(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	ctx := context.Background()
	carrier := insertCarrier(t, pool, fix.TenantA)
	relationshipID, _ := acceptCarrier(t, svc, cat, fix, carrier, "revoke-reactivate")
	revoked, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "reactivate-key", "pause"))
	if err != nil || revoked.RelationshipVersion != 2 {
		t.Fatalf("revoke before reactivation: %v %+v", err, revoked)
	}
	created := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, carrier, "reactivate-create", "reactivate")
	accepted, err := svc.Accept(ctx, acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, created.Token, "reactivate-accept"))
	if err != nil || !accepted.Created || accepted.RelationshipVersion != 3 || accepted.RelationshipID != relationshipID {
		t.Fatalf("reactivation: %v %+v", err, accepted)
	}
	if versionRowStatus(t, pool, relationshipID, 2) != domain.RelationshipStatusRevoked || versionRowStatus(t, pool, relationshipID, 3) != domain.RelationshipStatusActive {
		t.Fatal("reactivation rewrote a version")
	}
	var roots int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM company.carrier_relationships WHERE shipper_carrier_company_id = $1`, carrier).Scan(&roots); err != nil || roots != 1 {
		t.Fatalf("reactivation roots=%d err=%v", roots, err)
	}
	replay, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "reactivate-key", "stale"))
	if err != nil || replay.RelationshipVersion != 2 || versionRowStatus(t, pool, relationshipID, 3) != domain.RelationshipStatusActive {
		t.Fatalf("stale revoke replay: %v %+v", err, replay)
	}
}

func assertRevokeConstraints(t *testing.T, pool *pgxpool.Pool, fix fixture, relationshipID, requestID uuid.UUID) {
	t.Helper()
	mustFail(t, pool, `UPDATE company.carrier_relationship_versions SET status = 'ACTIVE' WHERE relationship_id = $1 AND relationship_version = 2`, relationshipID)
	mustFail(t, pool, `DELETE FROM company.carrier_relationship_versions WHERE relationship_id = $1 AND relationship_version = 2`, relationshipID)
	mustFail(t, pool, `UPDATE company.carrier_relationship_heads SET current_version = 1 WHERE relationship_id = $1`, relationshipID)
	mustFail(t, pool, `UPDATE company.carrier_relationship_heads SET status = 'CANCELLED' WHERE relationship_id = $1`, relationshipID)
	mustFail(t, pool, `UPDATE company.carrier_relationship_request_heads SET request_status = 'REVOKED' WHERE relationship_request_id = $1`, requestID)
	hash := domain.RevokeBodyHash(relationshipID)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO company.carrier_relationship_revoke_idempotency (
			relationship_id, actor_tenant_id, selected_company_id, idempotency_key, body_hash, relationship_version
		) VALUES ($1, $2, $3, 'foreign-tenant', $4, 2)
	`, relationshipID, uuid.New(), fix.ShipperA, hash); err == nil {
		t.Fatal("foreign tenant idempotency row was stored")
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO company.carrier_relationship_revoke_idempotency (
			relationship_id, actor_tenant_id, selected_company_id, idempotency_key, body_hash, relationship_version
		) VALUES ($1, $2, $3, 'same-key', $4, 2)
	`, relationshipID, fix.TenantA, fix.ShipperA, hash); err == nil {
		t.Fatal("duplicate revoke scope was stored")
	}
}

func assertRevokeMigrationCycle(t *testing.T) {
	t.Helper()
	pool := setupPostgres(t)
	ctx := context.Background()
	fix := seedFixture(t, pool)
	repo := repository.NewRelationshipRepository(pool)
	cat := newCatalog()
	svc := service.NewRelationshipService(repo, repo, postgresRelationshipConfig(), matchSource{}, cat)
	carrier := insertCarrier(t, pool, fix.TenantA)
	relationshipID, _ := acceptCarrier(t, svc, cat, fix, carrier, "migration-cycle")
	var currentID, requestID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT h.current_version_id, v.source_relationship_request_id
		FROM company.carrier_relationship_heads h
		JOIN company.carrier_relationship_versions v ON v.relationship_version_id = h.current_version_id
		WHERE h.relationship_id = $1
	`, relationshipID).Scan(&currentID, &requestID); err != nil {
		t.Fatal(err)
	}
	orphan := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by, revoke_reason
		) VALUES ($1, $2, 99, 'REVOKED', $3, $4, $5, '   ')
	`, orphan, relationshipID, currentID, requestID, fix.ShipperAdmin); err != nil {
		t.Fatal(err)
	}
	before := historyCounts(t, pool)
	if err := applyRevokeDown(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if revokeObjectsPresent(t, pool) || historyCounts(t, pool) != before {
		t.Fatalf("down changed history or left objects before=%s after=%s", before, historyCounts(t, pool))
	}
	if err := applyRevokeUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !revokeObjectsPresent(t, pool) || historyCounts(t, pool) != before {
		t.Fatal("second up changed historical rows or omitted revoke objects")
	}
	var restored *string
	if err := pool.QueryRow(ctx, `SELECT revoke_reason FROM company.carrier_relationship_versions WHERE relationship_version_id = $1`, orphan).Scan(&restored); err != nil || restored != nil {
		t.Fatalf("empty reason restored as %v err=%v", restored, err)
	}
	revoked, err := svc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, relationshipID, "migration-reason", "ended"))
	if err != nil || revoked.RelationshipVersion != 2 {
		t.Fatalf("reason revoke: %v %+v", err, revoked)
	}
	kept := historyCounts(t, pool)
	if err := applyRevokeDown(ctx, pool); err == nil {
		t.Fatal("down committed while a revoked reason was present")
	}
	if !revokeObjectsPresent(t, pool) || historyCounts(t, pool) != kept || revokeReason(t, pool, relationshipID, 2) != "ended" {
		t.Fatal("refused down changed the reason, the schema, or historical rows")
	}
	if err := applyRevokeUp(ctx, pool); err == nil {
		t.Fatal("second up dropped or duplicated revoke objects")
	}
	if !revokeObjectsPresent(t, pool) || revokeReason(t, pool, relationshipID, 2) != "ended" || historyCounts(t, pool) != kept {
		t.Fatal("failed up changed the stored reason or historical rows")
	}
}

func acceptCarrier(t *testing.T, svc *service.RelationshipService, cat *catalogSource, fix fixture, carrier uuid.UUID, key string) (uuid.UUID, createdRequest) {
	t.Helper()
	created := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, carrier, key+"-create", key)
	out, err := svc.Accept(context.Background(), acceptInput(fix.CarrierAdmin, fix.TenantB, fix.OperatingB, created.Token, key+"-accept"))
	if err != nil || !out.Created || out.RelationshipID == uuid.Nil {
		t.Fatalf("accept %s: %v %+v", key, err, out)
	}
	return out.RelationshipID, created
}

func insertCarrier(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status) VALUES ($1, $2, 'CARRIER', $3, 'ACTIVE')`, id, tenant, id.String()); err != nil {
		t.Fatal(err)
	}
	return id
}

func revokeInput(user, tenant, company, relationship uuid.UUID, key, reason string) service.RevokeRelationshipInput {
	return service.RevokeRelationshipInput{
		Principal: domain.Principal{UserID: user, TenantID: tenant}, SelectedCompanyID: company,
		RelationshipID: relationship, IdempotencyKey: key, Reason: reason,
	}
}

func withExpected(in service.RevokeRelationshipInput, version *int) service.RevokeRelationshipInput {
	in.ExpectedRelationshipVersion = version
	return in
}

func intPtr(v int) *int { return &v }

func assertRevokedHead(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID, version int) {
	t.Helper()
	var status string
	var current int
	if err := pool.QueryRow(context.Background(), `SELECT status, current_version FROM company.carrier_relationship_heads WHERE relationship_id = $1`, relationshipID).Scan(&status, &current); err != nil || status != domain.RelationshipStatusRevoked || current != version {
		t.Fatalf("head status=%s version=%d err=%v", status, current, err)
	}
}

func assertRevokeWrites(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID, versions, audits, keys int) {
	t.Helper()
	if got := revokeVersionCount(t, pool, relationshipID); got != versions {
		t.Fatalf("revoked versions=%d want=%d", got, versions)
	}
	if got := scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_audit WHERE relationship_id = $1 AND action = 'relationship_revoked'`, relationshipID); got != audits {
		t.Fatalf("revoke audit=%d want=%d", got, audits)
	}
	if got := scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_outbox WHERE aggregate_id = $1 AND event_type = 'carrier_relationship.revoked'`, relationshipID); got != audits {
		t.Fatalf("revoke outbox=%d want=%d", got, audits)
	}
	if got := revokeKeyCount(t, pool, relationshipID); got != keys {
		t.Fatalf("revoke keys=%d want=%d", got, keys)
	}
}

func assertRevokePayload(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID) {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), `
		SELECT payload FROM company.carrier_relationship_outbox
		WHERE aggregate_id = $1 AND event_type = 'carrier_relationship.revoked'
	`, relationshipID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 4 || payload["status"] != domain.RelationshipStatusRevoked || payload["relationship_id"] != relationshipID.String() {
		t.Fatalf("outbox payload=%v", payload)
	}
	for _, hidden := range []string{"reason", "acceptance_token", "actor_user_id", "tenant_id", "company_id"} {
		if _, ok := payload[hidden]; ok {
			t.Fatalf("outbox contains %s", hidden)
		}
	}
}

func versionRow(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID, version int) string {
	t.Helper()
	var row string
	if err := pool.QueryRow(context.Background(), `SELECT row_to_json(v)::text FROM company.carrier_relationship_versions v WHERE relationship_id = $1 AND relationship_version = $2`, relationshipID, version).Scan(&row); err != nil {
		t.Fatal(err)
	}
	return row
}

func versionRowStatus(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID, version int) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM company.carrier_relationship_versions WHERE relationship_id = $1 AND relationship_version = $2`, relationshipID, version).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func versionCount(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID) int {
	t.Helper()
	return scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_versions WHERE relationship_id = $1`, relationshipID)
}

func revokeVersionCount(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID) int {
	t.Helper()
	return scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_versions WHERE relationship_id = $1 AND status = 'REVOKED'`, relationshipID)
}

func revokeKeyCount(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID) int {
	t.Helper()
	return scalar(t, pool, `SELECT count(*) FROM company.carrier_relationship_revoke_idempotency WHERE relationship_id = $1`, relationshipID)
}

func scalar(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func historyCounts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var roots, versions, audits, outbox int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM company.carrier_relationships),
			(SELECT count(*) FROM company.carrier_relationship_versions),
			(SELECT count(*) FROM company.carrier_relationship_audit),
			(SELECT count(*) FROM company.carrier_relationship_outbox)
	`).Scan(&roots, &versions, &audits, &outbox); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("roots=%d versions=%d audits=%d outbox=%d", roots, versions, audits, outbox)
}

func revokeObjectsPresent(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var tableName, columnName, triggerName *string
	if err := pool.QueryRow(context.Background(), `
		SELECT to_regclass('company.carrier_relationship_revoke_idempotency')::text,
		       (SELECT column_name FROM information_schema.columns
		         WHERE table_schema = 'company' AND table_name = 'carrier_relationship_versions' AND column_name = 'revoke_reason'),
		       (SELECT tgname FROM pg_trigger WHERE tgname = 'cr_heads_transition_guard' AND NOT tgisinternal)
	`).Scan(&tableName, &columnName, &triggerName); err != nil {
		t.Fatal(err)
	}
	return tableName != nil && columnName != nil && triggerName != nil
}

func revokeReason(t *testing.T, pool *pgxpool.Pool, relationshipID uuid.UUID, version int) string {
	t.Helper()
	var reason *string
	if err := pool.QueryRow(context.Background(), `SELECT revoke_reason FROM company.carrier_relationship_versions WHERE relationship_id = $1 AND relationship_version = $2`, relationshipID, version).Scan(&reason); err != nil || reason == nil {
		t.Fatalf("reason=%v err=%v", reason, err)
	}
	return *reason
}

func tableDigest(t *testing.T, pool *pgxpool.Pool, table string) string {
	t.Helper()
	var digest string
	query := fmt.Sprintf(`SELECT md5(COALESCE(string_agg(row_to_json(src)::text, '|' ORDER BY src.ctid::text), '')) FROM %s src`, table)
	if err := pool.QueryRow(context.Background(), query).Scan(&digest); err != nil {
		t.Fatalf("%s: %v", table, err)
	}
	return digest
}

func stringsContainCompany(err error, company uuid.UUID) bool {
	return err != nil && strings.Contains(err.Error(), company.String())
}

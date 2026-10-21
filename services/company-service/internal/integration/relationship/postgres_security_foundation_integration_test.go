//go:build integration

package relationship

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
)

func TestPostgresSecurityFoundation(t *testing.T) {
	pool := setupSecurityPostgres(t)
	ctx := context.Background()
	var version, dbName string
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_database()`).Scan(&version, &dbName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dbName, "rel_found_") {
		t.Fatalf("refusing non-isolated database %s", dbName)
	}
	t.Logf("POSTGRES_VERSION=%s", version)
	t.Logf("TEST_DATABASE_ISOLATED=%s", dbName)

	marker := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, 'security')`, marker, "sec-"+marker.String()[:8]); err != nil {
		t.Fatal(err)
	}
	legacyExec, legacyRev := insertExecutionRevision(t, pool, marker, marker, uuid.New(), nil, secDigest)
	for _, present := range []bool{false, true, false, true} {
		script := "000103_tms_relationship_pins_security_foundation_v0_1.down.sql"
		if present {
			script = "000103_tms_relationship_pins_security_foundation_v0_1.up.sql"
		}
		if err := execMigration(ctx, pool, script); err != nil {
			t.Fatalf("migration %s present=%v: %v", script, present, err)
		}
		if err := expectSecurityObjects(ctx, pool, present); err != nil {
			t.Fatalf("security objects present=%v: %v", present, err)
		}
	}
	var markerLeft, legacyLeft int
	var legacyFinger *string
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM core.tenants WHERE id = $1`, marker).Scan(&markerLeft); err != nil || markerLeft != 1 {
		t.Fatalf("historical tenant remaining=%d err=%v", markerLeft, err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*), max(pin_set_fingerprint)
		FROM transport.transport_execution_revisions WHERE id = $1
	`, legacyRev).Scan(&legacyLeft, &legacyFinger); err != nil || legacyLeft != 1 || legacyFinger != nil {
		t.Fatalf("legacy revision remaining=%d fingerprint=%v err=%v", legacyLeft, legacyFinger, err)
	}
	if _, _ = insertExecutionRevision(t, pool, marker, marker, uuid.New(), nil, secDigest); triggerCount(t, pool) != 0 {
		t.Fatal("preparatory migration installed trg_new_activation_authorization")
	}
	t.Log("MIGRATION_UP=PASS")
	t.Log("MIGRATION_DOWN=PASS")
	t.Log("MIGRATION_UP_DOWN_UP=PASS")
	t.Log("TAP-30=PASS")

	assertFunctionHardened(t, pool)
	fix := seedFixture(t, pool)
	pinCarrier := insertCarrier(t, pool, fix.TenantA)
	revokeCarrier := insertCarrier(t, pool, fix.TenantA)
	rolePool := poolAs(t, pool, "company_runtime")
	roleRepo := repository.NewRelationshipRepository(rolePool)
	cat := newCatalog()
	roleSvc := service.NewRelationshipService(roleRepo, roleRepo, postgresRelationshipConfig(), matchSource{}, cat)
	cancelReq := createOK(t, roleSvc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "sec-cancel", "v1")
	cancelled, err := roleSvc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, cancelReq.SourceID, cancelReq.RequestID, "sec-cancel-key", "security"))
	if err != nil || cancelled.Status != domain.RequestStatusCancelled {
		t.Fatalf("company_runtime cancel: %v %+v", err, cancelled)
	}
	revokedID, _ := acceptCarrier(t, roleSvc, cat, fix, revokeCarrier, "sec-revoke")
	revoked, err := roleSvc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, revokedID, "sec-revoke-key", "ended"))
	if err != nil || revoked.Status != domain.RelationshipStatusRevoked || revoked.RelationshipVersion != 2 {
		t.Fatalf("company_runtime revoke: %v %+v", err, revoked)
	}
	pinRel, _ := acceptCarrier(t, roleSvc, cat, fix, pinCarrier, "sec-pin")
	if _, err := rolePool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_activation_subjects (
			id, authorization_id, operating_tenant_id, shipment_tenant_id, shipment_id,
			source_company_id, relationship_id, locked_relationship_version_id,
			relationship_version, operating_carrier_company_id
		) VALUES ($1,$1,$2,$3,$1,$4,$5,$1,1,$6)
	`, uuid.New(), fix.TenantB, fix.TenantA, pinCarrier, pinRel, fix.OperatingB); !permissionDenied(err) {
		t.Fatalf("company_runtime subject insert err=%v", err)
	}
	t.Log("TAP-26=PASS")

	shipment := uuid.New()
	if _, _, err := callAuthorizer(ctx, pool, uuid.New(), fix.TenantA, fix.OperatingB, secDigest, secFinger, fix.TenantA, shipment, pinCarrier); err == nil || !strings.Contains(err.Error(), "RELATIONSHIP_NOT_FOUND") {
		t.Fatalf("sibling tenant: %v", err)
	}
	if _, _, err := callAuthorizer(ctx, pool, uuid.New(), fix.TenantB, fix.OperatingB2, secDigest, secFinger, fix.TenantA, shipment, pinCarrier); err == nil || !strings.Contains(err.Error(), "RELATIONSHIP_OPERATING_COMPANY_MISMATCH") {
		t.Fatalf("sibling company: %v", err)
	}
	if _, _, err := callAuthorizer(ctx, pool, uuid.New(), fix.TenantB, fix.OperatingB, secDigest, secFinger, fix.TenantA, uuid.New(), revokeCarrier); err == nil || !strings.Contains(err.Error(), "RELATIONSHIP_REVOKED") {
		t.Fatalf("revoked head: %v", err)
	}
	t.Log("TAP-02=PASS")
	t.Log("TAP-07=PASS")
	t.Log("TAP-08=PASS")
	ownerRejectsHistoricalSubject(t, pool, revokedID, fix, revokeCarrier)
	t.Log("TAP-22=PASS")

	activation := uuid.New()
	outcome, authID, err := callAuthorizer(ctx, pool, activation, fix.TenantB, fix.OperatingB, secDigest, secFinger, fix.TenantA, shipment, pinCarrier)
	if err != nil || outcome != "AUTHORIZED" {
		t.Fatalf("authorizer outcome=%s err=%v", outcome, err)
	}
	var locked, head uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT s.locked_relationship_version_id, h.current_version_id
		FROM company.carrier_relationship_activation_subjects s
		JOIN company.carrier_relationship_heads h ON h.relationship_id = s.relationship_id
		WHERE s.authorization_id = $1
	`, authID).Scan(&locked, &head); err != nil || locked != head {
		t.Fatalf("locked version=%s head=%s err=%v", locked, head, err)
	}
	beforeAuth := scalarInt(t, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := callAuthorizer(ctx, tx, uuid.New(), fix.TenantB, fix.OperatingB, secDigestB, secFinger, fix.TenantA, uuid.New(), pinCarrier); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1/0`); err == nil {
		t.Fatal("expected forced rollback")
	}
	_ = tx.Rollback(ctx)
	if scalarInt(t, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`) != beforeAuth {
		t.Fatal("forced error left an authorization")
	}
	t.Log("TAP-10=PASS")

	pinExec, pinRev := insertExecutionRevision(t, pool, fix.TenantB, fix.OperatingB, activation, nil, secDigest)
	rejectEarlyPin(t, pool, authID, locked, fix, pinCarrier, pinRel)
	var attached int
	if err := pool.QueryRow(ctx, `SELECT transport.attach_activation_pins($1)`, pinRev).Scan(&attached); err != nil || attached != 1 {
		t.Fatalf("attach pins=%d err=%v", attached, err)
	}
	if _, err := pool.Exec(ctx, `SELECT company.record_activation_audit($1,$2,$3,$4,'security')`, authID, fix.ShipperAdmin, fix.TenantA, fix.ShipperA); err != nil {
		t.Fatalf("audit: %v", err)
	}
	var pinVersion uuid.UUID
	var finger string
	if err := pool.QueryRow(ctx, `
		SELECT relationship_version_id, pin_set_fingerprint
		FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id = $1
	`, pinRev).Scan(&pinVersion, &finger); err != nil || pinVersion != head || finger != secFinger {
		t.Fatalf("pin version=%s finger=%s err=%v", pinVersion, finger, err)
	}
	rejectPinIdentity(t, pool, authID, pinRev, locked, fix, pinCarrier, pinRel)
	t.Log("TAP-32=PASS")
	if _, err := pool.Exec(ctx, `
		UPDATE transport.transport_execution_carrier_relationship_pins
		SET pin_set_fingerprint = $2 WHERE execution_revision_id = $1
	`, pinRev, secFingerB); err == nil || !strings.Contains(err.Error(), "activation pin is immutable") {
		t.Fatalf("pin update err=%v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id = $1`, pinRev); err == nil || !strings.Contains(err.Error(), "activation pin is immutable") {
		t.Fatalf("pin delete err=%v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT pin_set_fingerprint FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id = $1`, pinRev).Scan(&finger); err != nil || finger != secFinger {
		t.Fatalf("pin changed to %s err=%v", finger, err)
	}
	t.Log("TAP-11=PASS")

	replayOutcome, replayAuth, err := callAuthorizer(ctx, pool, activation, fix.TenantB, fix.OperatingB, secDigest, secFingerB, fix.TenantA, shipment, pinCarrier)
	if err != nil || replayOutcome != "REPLAY" || replayAuth != authID {
		t.Fatalf("replay outcome=%s auth=%s err=%v", replayOutcome, replayAuth, err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations WHERE source_activation_id = $1`, activation) != 1 {
		t.Fatal("replay wrote a second authorization")
	}
	if _, _, err := callAuthorizer(ctx, pool, activation, fix.TenantB, fix.OperatingB, secDigestB, secFinger, fix.TenantA, shipment, pinCarrier); err == nil || !strings.Contains(err.Error(), "ACTIVATION_BODY_CONFLICT") {
		t.Fatalf("digest conflict: %v", err)
	}

	withRole(t, pool, "shipment_runtime", func(ctx context.Context, tx pgx.Tx) {
		var seq int
		if err := tx.QueryRow(ctx, `
			UPDATE transport.transport_executions
			SET event_seq = event_seq + 1, updated_at = clock_timestamp()
			WHERE id = $1 RETURNING event_seq
		`, pinExec).Scan(&seq); err != nil || seq < 1 {
			t.Fatalf("event_seq update seq=%d err=%v", seq, err)
		}
	})
	pinCount := scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`)
	denyCtx, denyTx := beginRole(t, pool, "shipment_runtime")
	if _, err := denyTx.Exec(denyCtx, `
		INSERT INTO transport.transport_execution_carrier_relationship_pins (
			pin_id, execution_revision_id, authorization_subject_id, operating_tenant_id,
			shipment_tenant_id, shipment_id, source_company_id, relationship_id,
			relationship_version_id, relationship_version, operating_carrier_company_id,
			pin_set_fingerprint
		)
		SELECT $1, execution_revision_id, authorization_subject_id, operating_tenant_id,
		       shipment_tenant_id, $2, source_company_id, relationship_id,
		       relationship_version_id, relationship_version, operating_carrier_company_id,
		       pin_set_fingerprint
		FROM transport.transport_execution_carrier_relationship_pins
		WHERE execution_revision_id = $3
	`, uuid.New(), uuid.New(), pinRev); !permissionDenied(err) {
		t.Fatalf("direct pin insert err=%v", err)
	}
	_ = denyTx.Rollback(denyCtx)
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`) != pinCount {
		t.Fatal("direct pin insert changed the pin count")
	}
	t.Log("TAP-20=PASS")
	t.Log("TAP-25=PASS")

	dropCtx, dropTx := beginRole(t, pool, "shipment_runtime")
	if _, err := dropTx.Exec(dropCtx, `DROP FUNCTION company.authorize_carrier_relationship_activation(uuid, uuid, uuid, text, text, company.activation_subject_input[])`); err == nil {
		t.Fatal("shipment_runtime dropped the authorizer")
	}
	_ = dropTx.Rollback(dropCtx)
	grantCtx, grantTx := beginRole(t, pool, "shipment_runtime")
	if _, err := grantTx.Exec(grantCtx, `GRANT INSERT ON transport.transport_execution_carrier_relationship_pins TO shipment_runtime`); err == nil {
		t.Fatal("shipment_runtime granted itself pin insert")
	}
	_ = grantTx.Rollback(grantCtx)
	var owner string
	if err := pool.QueryRow(ctx, `
		SELECT r.rolname
		FROM pg_proc p
		JOIN pg_roles r ON r.oid = p.proowner
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'company' AND p.proname = 'authorize_carrier_relationship_activation'
	`).Scan(&owner); err != nil || owner != "freight_migrations" {
		t.Fatalf("authorizer owner=%s err=%v", owner, err)
	}
	t.Log("TAP-28=PASS")
	rejectTempHijack(t, pool, fix, pinCarrier)
	t.Log("TAP-27=PASS")

	if _, err := pool.Exec(ctx, `CREATE ROLE freight SUPERUSER NOLOGIN`); err != nil {
		t.Fatal(err)
	}
	if err := applyEnforcementGate(ctx, pool); err == nil || !strings.Contains(err.Error(), "shared freight login can bypass activation enforcement") {
		t.Fatalf("superuser freight gate err=%v", err)
	}
	if triggerCount(t, pool) != 0 {
		t.Fatal("gate installed the trigger while freight was superuser")
	}
	if _, err := pool.Exec(ctx, `ALTER ROLE freight NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `GRANT INSERT, UPDATE, DELETE ON transport.transport_execution_carrier_relationship_pins TO freight`); err != nil {
		t.Fatal(err)
	}
	if err := applyEnforcementGate(ctx, pool); err == nil || !strings.Contains(err.Error(), "shared freight login can bypass activation enforcement") {
		t.Fatalf("freight pin grant gate err=%v", err)
	}
	if triggerCount(t, pool) != 0 {
		t.Fatal("gate installed the trigger while freight could write pins")
	}
	if _, err := pool.Exec(ctx, `REVOKE ALL ON transport.transport_execution_carrier_relationship_pins FROM freight`); err != nil {
		t.Fatal(err)
	}
	if err := applyEnforcementGate(ctx, pool); err != nil {
		t.Fatalf("gate: %v", err)
	}
	if triggerCount(t, pool) != 1 {
		t.Fatal("gate did not install the enforcement trigger")
	}
	for _, flag := range []string{"off", "on"} {
		rejectHeaderless(t, pool, "shipment_runtime", flag)
	}
	t.Log("TAP-31=PASS")

	successor := uuid.New()
	withRole(t, pool, "shipment_runtime", func(ctx context.Context, tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `
			UPDATE transport.transport_execution_revisions
			SET status = 'SUPERSEDED', updated_at = clock_timestamp()
			WHERE id = $1
		`, legacyRev); err != nil {
			t.Fatal(err)
		}
		insertRevisionTx(t, tx, legacyExec, successor, marker, uuid.New(), &legacyRev, secDigest)
		if _, err := tx.Exec(ctx, `
			UPDATE transport.transport_executions SET current_revision_id = $2, updated_at = clock_timestamp() WHERE id = $1
		`, legacyExec, successor); err != nil {
			t.Fatal(err)
		}
	})
	var successorOf uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT supersedes_revision_id FROM transport.transport_execution_revisions WHERE id = $1`, successor).Scan(&successorOf); err != nil || successorOf != legacyRev {
		t.Fatalf("legacy successor parent=%s err=%v", successorOf, err)
	}
	if scalarInt(t, pool, `
		SELECT count(*) FROM company.carrier_relationship_activation_authorizations
		WHERE source_activation_id IN (
			SELECT source_activation_id FROM transport.transport_execution_revisions WHERE id = $1 OR id = $2
		)`, legacyRev, successor) != 0 {
		t.Fatal("legacy successor created an authorization")
	}
	t.Log("TAP-33=PASS")

	rejectClearedSupersedes(t, pool, legacyRev, marker)
	t.Log("TAP-21=PASS")
	rejectRetargetedActivation(t, pool, legacyRev, marker)
	t.Log("TAP-29=PASS")
	rejectRewrittenRevisionID(t, pool, marker)
	t.Log("TAP-34=PASS")
	rejectOperatingTenantChange(t, pool, marker)
	t.Log("TAP-35=PASS")

	authBeforeSuccessor := scalarInt(t, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`)
	pinnedSuccessor := uuid.New()
	var copied int
	copyTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := copyTx.Exec(ctx, `SET LOCAL ROLE shipment_runtime`); err != nil {
		t.Fatal(err)
	}
	if _, err := copyTx.Exec(ctx, `UPDATE transport.transport_execution_revisions SET status = 'SUPERSEDED', updated_at = clock_timestamp() WHERE id = $1`, pinRev); err != nil {
		t.Fatal(err)
	}
	insertRevisionTx(t, copyTx, pinExec, pinnedSuccessor, fix.TenantB, uuid.New(), &pinRev, secDigestB)
	if _, err := copyTx.Exec(ctx, `UPDATE transport.transport_executions SET current_revision_id = $2, updated_at = clock_timestamp() WHERE id = $1`, pinExec, pinnedSuccessor); err != nil {
		t.Fatal(err)
	}
	if err := copyTx.QueryRow(ctx, `SELECT transport.propagate_execution_relationship_pins($1)`, pinnedSuccessor).Scan(&copied); err != nil || copied != 1 {
		t.Fatalf("propagate copied=%d err=%v", copied, err)
	}
	if err := copyTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var current, copiedVersion, originalVersion uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT current_revision_id FROM transport.transport_executions WHERE id = $1`, pinExec).Scan(&current); err != nil || current != pinnedSuccessor {
		t.Fatalf("current revision=%s err=%v", current, err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT p.relationship_version_id, o.relationship_version_id
		FROM transport.transport_execution_carrier_relationship_pins p
		JOIN transport.transport_execution_carrier_relationship_pins o
		  ON o.execution_revision_id = $2 AND o.authorization_subject_id = p.authorization_subject_id
		WHERE p.execution_revision_id = $1
	`, pinnedSuccessor, pinRev).Scan(&copiedVersion, &originalVersion); err != nil || copiedVersion != originalVersion || copiedVersion != head {
		t.Fatalf("copied=%s original=%s head=%s err=%v", copiedVersion, originalVersion, head, err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`) != authBeforeSuccessor {
		t.Fatal("successor created an authorization")
	}
	adminRepo := repository.NewRelationshipRepository(pool)
	adminSvc := service.NewRelationshipService(adminRepo, adminRepo, postgresRelationshipConfig(), matchSource{}, cat)
	if _, err := adminSvc.Revoke(ctx, revokeInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, pinRel, "sec-after-pin", "later")); err != nil {
		t.Fatalf("revoke after pin: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT relationship_version_id FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id = $1`, pinnedSuccessor).Scan(&copiedVersion); err != nil || copiedVersion != head {
		t.Fatalf("successor pin after revoke=%s err=%v", copiedVersion, err)
	}
	if err := pool.QueryRow(ctx, `SELECT relationship_version_id FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id = $1`, pinRev).Scan(&originalVersion); err != nil || originalVersion != head {
		t.Fatalf("original pin after revoke=%s err=%v", originalVersion, err)
	}
	t.Log("TAP-23=PASS")
	rejectSuccessorCopy(t, pool, pinExec, pinnedSuccessor, fix.TenantB)
	t.Log("TAP-24=PASS")

	if _, err := pool.Exec(ctx, `
		GRANT USAGE ON SCHEMA transport TO freight;
		GRANT SELECT, INSERT, UPDATE ON transport.transport_executions TO freight;
		GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_revisions TO freight
	`); err != nil {
		t.Fatal(err)
	}
	rejectHeaderless(t, pool, "freight", "on")
	freightCtx, freightTx := beginRole(t, pool, "freight")
	if _, err := freightTx.Exec(freightCtx, `INSERT INTO transport.transport_execution_carrier_relationship_pins (pin_id) VALUES ($1)`, uuid.New()); !permissionDenied(err) {
		t.Fatalf("freight pin insert err=%v", err)
	}
	_ = freightTx.Rollback(freightCtx)
	blockCtx, blockTx := beginRole(t, pool, "freight")
	if _, err := blockTx.Exec(blockCtx, `ALTER TABLE transport.transport_execution_revisions DISABLE TRIGGER trg_new_activation_authorization`); err == nil {
		t.Fatal("freight disabled the enforcement trigger")
	}
	_ = blockTx.Rollback(blockCtx)
	t.Log("DB_ROLE_ISOLATION=PASS")
	t.Log("POSTGRES_INTEGRATION=PASS")
	_ = legacyExec
}

func ownerRejectsHistoricalSubject(t *testing.T, pool *pgxpool.Pool, relationship uuid.UUID, fix fixture, carrier uuid.UUID) {
	t.Helper()
	ctx, tx := beginRole(t, pool, "freight_migrations")
	defer func() { _ = tx.Rollback(ctx) }()
	var historical uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT relationship_version_id FROM company.carrier_relationship_versions
		WHERE relationship_id = $1 AND relationship_version = 1
	`, relationship).Scan(&historical); err != nil {
		t.Fatal(err)
	}
	header := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_activation_authorizations (
			id, source_activation_id, operating_tenant_id, operating_carrier_company_id,
			contract_sha256, pin_set_fingerprint
		) VALUES ($1,$2,$3,$4,$5,$6)
	`, header, uuid.New(), fix.TenantB, fix.OperatingB, secDigest, secFinger); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_activation_subjects (
			id, authorization_id, operating_tenant_id, shipment_tenant_id, shipment_id,
			source_company_id, relationship_id, locked_relationship_version_id,
			relationship_version, operating_carrier_company_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,$9)
	`, uuid.New(), header, fix.TenantB, fix.TenantA, uuid.New(), carrier, relationship, historical, fix.OperatingB); err == nil || !strings.Contains(err.Error(), "activation subject version is not the active head") {
		t.Fatalf("historical subject err=%v", err)
	}
}

func rejectEarlyPin(t *testing.T, pool *pgxpool.Pool, authID, version uuid.UUID, fix fixture, carrier, relationship uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var subject uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM company.carrier_relationship_activation_subjects WHERE authorization_id = $1`, authID).Scan(&subject); err != nil {
		t.Fatal(err)
	}
	before := scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`)
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_execution_carrier_relationship_pins (
			pin_id, execution_revision_id, authorization_subject_id, operating_tenant_id,
			shipment_tenant_id, shipment_id, source_company_id, relationship_id,
			relationship_version_id, relationship_version, operating_carrier_company_id,
			pin_set_fingerprint
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1,$10,$11)
	`, uuid.New(), uuid.New(), subject, fix.TenantB, fix.TenantA, uuid.New(), carrier, relationship, version, fix.OperatingB, secFinger); err == nil || !strings.Contains(err.Error(), "23503") {
		t.Fatalf("early pin err=%v", err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`) != before {
		t.Fatal("early pin was stored")
	}
}

func rejectPinIdentity(t *testing.T, pool *pgxpool.Pool, authID, revision, version uuid.UUID, fix fixture, carrier, relationship uuid.UUID) {
	t.Helper()
	var subject uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM company.carrier_relationship_activation_subjects WHERE authorization_id = $1`, authID).Scan(&subject); err != nil {
		t.Fatal(err)
	}
	before := scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO transport.transport_execution_carrier_relationship_pins (
			pin_id, execution_revision_id, authorization_subject_id, operating_tenant_id,
			shipment_tenant_id, shipment_id, source_company_id, relationship_id,
			relationship_version_id, relationship_version, operating_carrier_company_id,
			pin_set_fingerprint
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1,$10,$11)
	`, uuid.New(), revision, subject, fix.TenantB, fix.TenantA, uuid.New(), carrier, relationship, version, fix.OperatingB, secFinger); err == nil || !strings.Contains(err.Error(), "23503") {
		t.Fatalf("identity pin err=%v", err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`) != before {
		t.Fatal("rejected identity pin was stored")
	}
}

func rejectTempHijack(t *testing.T, pool *pgxpool.Pool, fix fixture, carrier uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `
		CREATE FUNCTION pg_temp.pg_advisory_xact_lock(bigint) RETURNS void
		LANGUAGE plpgsql AS $hijack$ BEGIN RAISE EXCEPTION 'HIJACKED'; END $hijack$
	`); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, `DROP FUNCTION IF EXISTS pg_temp.pg_advisory_xact_lock(bigint)`)
	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE hashtextextended (id integer)`); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS pg_temp.hashtextextended`)
	_, _, err = callAuthorizer(ctx, conn, uuid.New(), fix.TenantB, fix.OperatingB, secDigest, secFinger, uuid.New(), uuid.New(), carrier)
	if err == nil || strings.Contains(err.Error(), "HIJACKED") || !strings.Contains(err.Error(), "RELATIONSHIP_NOT_FOUND") {
		t.Fatalf("temp hijack err=%v", err)
	}
}

func rejectHeaderless(t *testing.T, pool *pgxpool.Pool, role, flag string) {
	t.Helper()
	ctx, tx := beginRole(t, pool, role)
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('tms.relationship_activation_enabled', $1, true)`, flag); err != nil {
		t.Fatal(err)
	}
	execID, revID, tenant := uuid.New(), uuid.New(), uuid.New()
	insertExecutionTx(t, tx, execID, tenant, tenant)
	insertRevisionTx(t, tx, execID, revID, tenant, uuid.New(), nil, secDigest)
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_executions SET current_revision_id = $2 WHERE id = $1`, execID, revID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err == nil || !strings.Contains(err.Error(), "new activation revision has no authorization header") {
		t.Fatalf("%s flag=%s commit err=%v", role, flag, err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE id = $1`, revID) != 0 {
		t.Fatalf("%s revision remained", role)
	}
}

func rejectClearedSupersedes(t *testing.T, pool *pgxpool.Pool, parent, tenant uuid.UUID) {
	t.Helper()
	ctx, tx := beginRole(t, pool, "shipment_runtime")
	defer func() { _ = tx.Rollback(ctx) }()
	execID, keptExec := uuid.New(), uuid.New()
	revID, kept := uuid.New(), uuid.New()
	insertExecutionTx(t, tx, execID, tenant, tenant)
	insertRevisionTx(t, tx, execID, revID, tenant, uuid.New(), &parent, secDigest)
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_execution_revisions SET supersedes_revision_id = NULL, updated_at = clock_timestamp() WHERE id = $1`, revID); err != nil {
		t.Fatal(err)
	}
	insertExecutionTx(t, tx, keptExec, tenant, tenant)
	insertRevisionTx(t, tx, keptExec, kept, tenant, uuid.New(), &parent, secDigestB)
	if err := tx.Commit(ctx); err == nil || !strings.Contains(err.Error(), "new activation revision has no authorization header") {
		t.Fatalf("cleared supersedes commit err=%v", err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE id = $1 OR id = $2`, revID, kept) != 0 {
		t.Fatal("cleared supersedes revision remained")
	}
}

func rejectRetargetedActivation(t *testing.T, pool *pgxpool.Pool, parent, tenant uuid.UUID) {
	t.Helper()
	ctx, tx := beginRole(t, pool, "shipment_runtime")
	defer func() { _ = tx.Rollback(ctx) }()
	execID, revID := uuid.New(), uuid.New()
	insertExecutionTx(t, tx, execID, tenant, tenant)
	insertRevisionTx(t, tx, execID, revID, tenant, uuid.New(), &parent, secDigest)
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_execution_revisions SET supersedes_revision_id = NULL WHERE id = $1`, revID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_execution_revisions SET source_activation_id = $2 WHERE id = $1`, revID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err == nil || !strings.Contains(err.Error(), "new activation revision has no authorization header") {
		t.Fatalf("retarget commit err=%v", err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE id = $1`, revID) != 0 {
		t.Fatal("retargeted revision remained")
	}
}

func rejectRewrittenRevisionID(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID) {
	t.Helper()
	for _, updates := range []int{1, 2} {
		ctx, tx := beginRole(t, pool, "shipment_runtime")
		execID, revID := uuid.New(), uuid.New()
		insertExecutionTx(t, tx, execID, tenant, tenant)
		insertRevisionTx(t, tx, execID, revID, tenant, uuid.New(), nil, secDigest)
		current := revID
		for i := 0; i < updates; i++ {
			next := uuid.New()
			if _, err := tx.Exec(ctx, `UPDATE transport.transport_execution_revisions SET id = $2 WHERE id = $1`, current, next); err != nil {
				t.Fatalf("id update %d: %v", i, err)
			}
			current = next
		}
		if _, err := tx.Exec(ctx, `UPDATE transport.transport_executions SET current_revision_id = $2 WHERE id = $1`, execID, current); err != nil {
			t.Fatal(err)
		}
		err := tx.Commit(ctx)
		_ = tx.Rollback(ctx)
		if err == nil || !strings.Contains(err.Error(), "new activation revision has no authorization header") {
			t.Fatalf("id updates=%d commit err=%v", updates, err)
		}
		if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_executions WHERE id = $1`, execID) != 0 || scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE execution_id = $1`, execID) != 0 {
			t.Fatalf("id rewrite updates=%d remained", updates)
		}
	}
}

func rejectOperatingTenantChange(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID) {
	t.Helper()
	ctx, tx := beginRole(t, pool, "shipment_runtime")
	defer func() { _ = tx.Rollback(ctx) }()
	execID, revID := uuid.New(), uuid.New()
	otherExec, otherTenant := uuid.New(), uuid.New()
	insertExecutionTx(t, tx, execID, tenant, tenant)
	insertExecutionTx(t, tx, otherExec, otherTenant, otherTenant)
	insertRevisionTx(t, tx, execID, revID, tenant, uuid.New(), nil, secDigest)
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_execution_revisions
		SET execution_id = $2, operating_tenant_id = $3
		WHERE id = $1
	`, revID, otherExec, otherTenant); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err == nil || !strings.Contains(err.Error(), "new activation revision has no authorization header") {
		t.Fatalf("operating tenant commit err=%v", err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE id = $1`, revID) != 0 {
		t.Fatal("operating tenant revision remained")
	}
}

func rejectSuccessorCopy(t *testing.T, pool *pgxpool.Pool, execID, activeRev, tenant uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	successor := uuid.New()
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_execution_revisions SET status = 'SUPERSEDED', updated_at = clock_timestamp() WHERE id = $1`, activeRev); err != nil {
		t.Fatal(err)
	}
	insertRevisionTx(t, tx, execID, successor, tenant, uuid.New(), &activeRev, secDigest)
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_executions SET current_revision_id = $2, carrier_company_id = $3 WHERE id = $1`, execID, successor, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT transport.propagate_execution_relationship_pins($1)`, successor); err == nil || !strings.Contains(err.Error(), "successor pin copy does not match the execution") {
		t.Fatalf("copy err=%v", err)
	}
	_ = tx.Rollback(ctx)
	var status string
	var current uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT status FROM transport.transport_execution_revisions WHERE id = $1`, activeRev).Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatalf("active revision status after failed copy=%s err=%v", status, err)
	}
	if err := pool.QueryRow(ctx, `SELECT current_revision_id FROM transport.transport_executions WHERE id = $1`, execID).Scan(&current); err != nil || current != activeRev {
		t.Fatalf("current revision after failed copy=%s err=%v", current, err)
	}
	if scalarInt(t, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE id = $1`, successor) != 0 {
		t.Fatal("failed successor remained")
	}
}

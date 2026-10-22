//go:build integration

package transportexecution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestActivationRuntimePins(t *testing.T) {
	env, bootstrap := startActivationPostgres(t)
	applyNamedMigrations(t, env,
		"000100_company_carrier_relationship_foundation_v0_1c.up.sql",
		"000101_company_carrier_relationship_cancel_recovery_v0_2.up.sql",
		"000102_company_carrier_relationship_revoke_v0_2.up.sql",
		"000103_tms_relationship_pins_security_foundation_v0_1.up.sql",
		"000104_tms_activation_runtime_grants_v0_1.up.sql",
	)
	if triggerCount(t, env.pool) != 0 {
		t.Fatal("preparatory migrations installed trg_new_activation_authorization")
	}

	legacyShip := seedShipment(t, env, "legacy-ship", "IN_TRANSIT")
	operating := seedTenant(t, env, "legacy-carrier")
	legacyCarrier := seedCompany(t, env, operating, "CARRIER", "Legacy Carrier")
	legacyCmd := oneSubjectCommand(operating, legacyCarrier, legacyShip, nil, nil)
	legacyRepo := repository.NewTransportExecutionRepository(env.pool)
	legacyResult, err := legacyRepo.Project(env.ctx, legacyCmd)
	if err != nil || !legacyResult.Created {
		t.Fatalf("legacy project: %v %+v", err, legacyResult)
	}
	if activationCount(t, env.ctx, env.pool, legacyCmd.ActivationID) != 0 || pinCount(t, env.ctx, env.pool, legacyResult.RevisionID) != 0 {
		t.Fatal("flag-off project wrote activation evidence")
	}
	var finger *string
	if err := env.pool.QueryRow(env.ctx, `SELECT pin_set_fingerprint FROM transport.transport_execution_revisions WHERE id=$1`, legacyResult.RevisionID).Scan(&finger); err != nil || finger != nil {
		t.Fatalf("legacy fingerprint=%v err=%v", finger, err)
	}
	t.Log("TAP-30=PASS")

	blocked := repository.NewTransportExecutionRepository(env.pool)
	blocked.SetRelationshipActivationEnabled(true)
	before := executionCount(t, env.ctx, env.pool)
	if _, err := blocked.Project(env.ctx, oneSubjectCommand(operating, legacyCarrier, seedShipment(t, env, "blocked", "IN_TRANSIT"), nil, nil)); err == nil || appReason(err) != domain.ReasonRelationshipStoreUnavailable {
		t.Fatalf("flag-on without the enforcement gate: %v", err)
	}
	if executionCount(t, env.ctx, env.pool) != before {
		t.Fatal("blocked activation left an execution")
	}

	admin := installActivationGate(t, env, bootstrap)
	if triggerCount(t, admin) != 1 {
		t.Fatal("test database did not install the enforcement gate")
	}
	runtime := runtimePool(t, env)
	runtimeRepo := repository.NewTransportExecutionRepository(runtime)
	runtimeRepo.SetRelationshipActivationEnabled(true)
	disabledRepo := repository.NewTransportExecutionRepository(runtime)

	disabledShip := seedShipmentOn(t, env.ctx, admin, "rt01-disabled", "IN_TRANSIT")
	if _, err := admin.Exec(env.ctx, `ALTER TABLE transport.transport_execution_revisions DISABLE TRIGGER trg_new_activation_authorization`); err != nil {
		t.Fatal(err)
	}
	assertNoActivationWrite(t, env.ctx, admin, runtimeRepo, oneSubjectCommand(operating, legacyCarrier, disabledShip, nil, nil), "RT01_A")
	if _, err := admin.Exec(env.ctx, `ALTER TABLE transport.transport_execution_revisions ENABLE TRIGGER trg_new_activation_authorization`); err != nil {
		t.Fatal(err)
	}
	assertActivationTriggerIdentity(t, env.ctx, admin)
	t.Log("RT01_A=PASS")

	if _, err := admin.Exec(env.ctx, `DROP TRIGGER trg_new_activation_authorization ON transport.transport_execution_revisions`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `
		CREATE FUNCTION transport.decoy_activation_name_trigger()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $fn$
		BEGIN
			RETURN NEW;
		END;
		$fn$
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `
		CREATE CONSTRAINT TRIGGER trg_new_activation_authorization
		    AFTER INSERT ON transport.transport_executions
		    DEFERRABLE INITIALLY DEFERRED
		    FOR EACH ROW
		    EXECUTE FUNCTION transport.decoy_activation_name_trigger()
	`); err != nil {
		t.Fatal(err)
	}
	if triggerCount(t, admin) != 1 || activationTriggerOnRevisions(t, env.ctx, admin) != 0 {
		t.Fatal("RT01_B did not leave the same trigger name only on another table")
	}
	decoyShip := seedShipmentOn(t, env.ctx, admin, "rt01-decoy", "IN_TRANSIT")
	assertNoActivationWrite(t, env.ctx, admin, runtimeRepo, oneSubjectCommand(operating, legacyCarrier, decoyShip, nil, nil), "RT01_B")
	if _, err := admin.Exec(env.ctx, `DROP TRIGGER trg_new_activation_authorization ON transport.transport_executions`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `DROP FUNCTION transport.decoy_activation_name_trigger()`); err != nil {
		t.Fatal(err)
	}
	if err := execSQLFile(env.ctx, admin, filepath.Join(env.migrations, "..", "postgres", "tms_activation_enforcement_gate_v0.1.sql")); err != nil {
		t.Fatal(err)
	}
	assertActivationTriggerIdentity(t, env.ctx, admin)
	t.Log("RT01_B=PASS")

	headerlessShip := seedShipmentOn(t, env.ctx, admin, "headerless", "IN_TRANSIT")
	headerlessBefore := executionCount(t, env.ctx, admin)
	if _, err := disabledRepo.Project(env.ctx, oneSubjectCommand(operating, legacyCarrier, headerlessShip, nil, nil)); err == nil {
		t.Fatal("headerless project committed after the enforcement trigger")
	}
	if executionCount(t, env.ctx, admin) != headerlessBefore {
		t.Fatal("headerless project left an execution")
	}
	t.Log("TAP-31=PASS")

	legacySuccessor, err := runtimeRepo.CreateSuccessorRevision(env.ctx, successorFromProjection(operating, legacyResult, legacyShip))
	if err != nil {
		t.Fatalf("legacy successor: %v", err)
	}
	if activationCount(t, env.ctx, admin, legacyCmd.ActivationID) != 0 || pinCount(t, env.ctx, admin, legacySuccessor.RevisionID) != 0 {
		t.Fatal("legacy successor created authorization or pins")
	}
	t.Log("TAP-33=PASS")

	live := seedRelationship(t, env.ctx, admin, "live")
	driver := seedAssignment(t, env.ctx, admin, live.operating, live.operatingCarrier, "driver")
	vehicle := seedAssignment(t, env.ctx, admin, live.operating, live.operatingCarrier, "vehicle")
	otherDriver := seedAssignment(t, env.ctx, admin, live.operating, live.operatingCarrier, "other-driver")
	cmd := oneSubjectCommand(live.operating, live.operatingCarrier, live.shipment, &driver, &vehicle)
	created, err := runtimeRepo.Project(env.ctx, cmd)
	if err != nil || !created.Created {
		t.Fatalf("pinned project: %v %+v", err, created)
	}
	assertPinned(t, env.ctx, admin, created, live, cmd.ActivationID, 1)
	assertActivationTriggerIdentity(t, env.ctx, admin)
	if executionCountForActivation(t, env.ctx, admin, cmd.ActivationID) != 1 {
		t.Fatal("RT01_C execution count")
	}
	t.Log("RT01_C=PASS")
	var executionDriver, executionVehicle *uuid.UUID
	if err := admin.QueryRow(env.ctx, `SELECT driver_id, vehicle_id FROM transport.transport_executions WHERE id=$1`, created.ExecutionID).Scan(&executionDriver, &executionVehicle); err != nil || executionDriver == nil || *executionDriver != driver || executionVehicle == nil || *executionVehicle != vehicle {
		t.Fatalf("assignment driver=%v vehicle=%v err=%v", executionDriver, executionVehicle, err)
	}
	if shipperParticipantCount(t, env.ctx, admin, live.shipment.tenantID) != 1 || shipperParticipantCount(t, env.ctx, admin, live.operating) != 0 {
		t.Fatal("shipper participation was not isolated")
	}
	var participantDriverCols int
	if err := admin.QueryRow(env.ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='transport' AND table_name='transport_execution_participants'
		  AND column_name IN ('driver_id', 'vehicle_id')
	`).Scan(&participantDriverCols); err != nil || participantDriverCols != 0 {
		t.Fatalf("participant driver columns=%d err=%v", participantDriverCols, err)
	}
	t.Log("TAP-01=PASS")

	replayed, err := runtimeRepo.Project(env.ctx, cmd)
	if err != nil || replayed.Created || replayed.ExecutionID != created.ExecutionID || replayed.RevisionID != created.RevisionID {
		t.Fatalf("replay: %v %+v", err, replayed)
	}
	if activationCount(t, env.ctx, admin, cmd.ActivationID) != 1 || pinCount(t, env.ctx, admin, created.RevisionID) != 1 {
		t.Fatal("replay wrote evidence")
	}
	t.Log("TAP-05=PASS")

	conflict := cmd
	conflict.DriverID = &otherDriver
	if _, err := runtimeRepo.Project(env.ctx, conflict); appReason(err) != domain.ReasonActivationBodyConflict {
		t.Fatalf("digest conflict: %v", err)
	}
	if activationCount(t, env.ctx, admin, cmd.ActivationID) != 1 || pinCount(t, env.ctx, admin, created.RevisionID) != 1 {
		t.Fatal("digest conflict wrote evidence")
	}
	t.Log("TAP-17=PASS")

	sibling := cmd
	sibling.ActivationID = uuid.New()
	sibling.CarrierCompanyID = live.siblingCarrier
	if _, err := runtimeRepo.Project(env.ctx, sibling); appReason(err) != domain.ReasonRelationshipOperatingCompanyMismatch {
		t.Fatalf("sibling company: %v", err)
	}
	t.Log("TAP-08=PASS")

	foreign := oneSubjectCommand(uuid.New(), live.operatingCarrier, seedShipmentOn(t, env.ctx, admin, "foreign", "IN_TRANSIT"), nil, nil)
	if _, err := admin.Exec(env.ctx, `UPDATE transport.shipments SET carrier_company_id=$2 WHERE id=$1`, foreign.ExecutionSubjects[0].ExecutionShipmentID, live.sourceCarrier); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeRepo.Project(env.ctx, foreign); appReason(err) != domain.ReasonTenantDenied && appReason(err) != domain.ReasonRelationshipNotFound {
		t.Fatalf("foreign tenant: %v", err)
	}
	shipperActing := oneSubjectCommand(live.shipment.tenantID, live.sourceCarrier, live.shipment, nil, nil)
	shipperActing.ActivationID = uuid.New()
	if _, err := runtimeRepo.Project(env.ctx, shipperActing); appReason(err) != domain.ReasonRelationshipNotFound && appReason(err) != domain.ReasonExecutionPlanConflict {
		t.Fatalf("shipper tenant as operating tenant: %v", err)
	}
	t.Log("TAP-07=PASS")

	badDriverShip := seedRelatedShipment(t, env.ctx, admin, live.shipment, live.sourceCarrier, "bad-driver")
	foreignDriver := seedAssignment(t, env.ctx, admin, live.shipment.tenantID, live.sourceCarrier, "foreign-driver")
	badDriver := oneSubjectCommand(live.operating, live.operatingCarrier, badDriverShip, &foreignDriver, nil)
	if _, err := runtimeRepo.Project(env.ctx, badDriver); appReason(err) != domain.ReasonOperatingDriverNotValid {
		t.Fatalf("foreign driver: %v", err)
	}
	if activationCount(t, env.ctx, admin, badDriver.ActivationID) != 0 {
		t.Fatal("invalid driver left an authorization")
	}
	badVehicleShip := seedRelatedShipment(t, env.ctx, admin, live.shipment, live.sourceCarrier, "bad-vehicle")
	badVehicle := oneSubjectCommand(live.operating, live.operatingCarrier, badVehicleShip, nil, &foreignDriver)
	if _, err := runtimeRepo.Project(env.ctx, badVehicle); appReason(err) != domain.ReasonOperatingVehicleNotValid {
		t.Fatalf("foreign vehicle: %v", err)
	}
	t.Log("TAP-09=PASS")

	nullShip := seedRelatedShipment(t, env.ctx, admin, live.shipment, live.sourceCarrier, "null-assignment")
	nullCmd := oneSubjectCommand(live.operating, live.operatingCarrier, nullShip, nil, nil)
	nullResult, err := runtimeRepo.Project(env.ctx, nullCmd)
	if err != nil || !nullResult.Created {
		t.Fatalf("null assignment: %v %+v", err, nullResult)
	}
	var nullDriver, nullVehicle *uuid.UUID
	if err := admin.QueryRow(env.ctx, `SELECT driver_id, vehicle_id FROM transport.transport_executions WHERE id=$1`, nullResult.ExecutionID).Scan(&nullDriver, &nullVehicle); err != nil || nullDriver != nil || nullVehicle != nil {
		t.Fatalf("null assignment stored driver=%v vehicle=%v err=%v", nullDriver, nullVehicle, err)
	}

	restore := repository.SetDriverStopTaskMaterializeForTest(func(context.Context, pgx.Tx, uuid.UUID, time.Time) error {
		return errors.New("forced rollback")
	})
	rollbackShip := seedRelatedShipment(t, env.ctx, admin, live.shipment, live.sourceCarrier, "rollback")
	rollbackCmd := oneSubjectCommand(live.operating, live.operatingCarrier, rollbackShip, &driver, &vehicle)
	authBefore := scalar(t, env.ctx, admin, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`)
	if _, err := runtimeRepo.Project(env.ctx, rollbackCmd); err == nil {
		restore()
		t.Fatal("forced error committed")
	}
	restore()
	if scalar(t, env.ctx, admin, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`) != authBefore || activationCount(t, env.ctx, admin, rollbackCmd.ActivationID) != 0 {
		t.Fatal("forced error left authorization")
	}
	t.Log("TAP-10=PASS")

	if _, err := runtime.Exec(env.ctx, `INSERT INTO transport.transport_execution_carrier_relationship_pins (pin_id) VALUES ($1)`, uuid.New()); !permissionDenied(err) {
		t.Fatalf("direct pin insert err=%v", err)
	}
	t.Log("TAP-20=PASS")
	headerlessCtx, cancel := context.WithTimeout(env.ctx, 15*time.Second)
	defer cancel()
	headerlessTx, err := runtime.Begin(headerlessCtx)
	if err != nil {
		t.Fatal(err)
	}
	headerlessExec, headerlessRev := uuid.New(), uuid.New()
	if _, err := headerlessTx.Exec(headerlessCtx, `
		INSERT INTO transport.transport_executions (
			id, operating_tenant_id, carrier_company_id, current_revision_id, created_at, updated_at
		) VALUES ($1,$2,$3,NULL,clock_timestamp(),clock_timestamp())
	`, headerlessExec, live.operating, live.operatingCarrier); err != nil {
		t.Fatal(err)
	}
	if _, err := headerlessTx.Exec(headerlessCtx, `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			supersedes_revision_id, status, version, contract_sha256, created_at, updated_at
		) VALUES ($1,$2,$3,$4,1,$5,1,'direct','DEPOT_START',NULL,'ACTIVE',1,$6,clock_timestamp(),clock_timestamp())
	`, headerlessRev, headerlessExec, live.operating, uuid.New(), uuid.New(), repeatHex('a')); err != nil {
		t.Fatal(err)
	}
	if err := headerlessTx.Commit(headerlessCtx); err == nil {
		t.Fatal("headerless revision committed")
	}
	if scalar(t, env.ctx, admin, `SELECT count(*) FROM transport.transport_execution_revisions WHERE id=$1`, headerlessRev) != 0 {
		t.Fatal("headerless revision remained")
	}
	if _, err := admin.Exec(env.ctx, `UPDATE transport.transport_execution_carrier_relationship_pins SET relationship_version=$2 WHERE execution_revision_id=$1`, created.RevisionID, 9); err == nil || !containsText(err, "immutable") {
		t.Fatalf("pin update err=%v", err)
	}
	t.Log("TAP-11=PASS")

	race := seedRelationship(t, env.ctx, admin, "race")
	raceShip := race.shipment
	hold, err := admin.Begin(env.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hold.Exec(env.ctx, `
		SELECT pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(
			'relationship:' || $1::text || ':' || $2::text || ':' || $3::text, 0))
	`, race.shipment.tenantID, race.sourceCarrier, race.operating); err != nil {
		t.Fatal(err)
	}
	if _, err := hold.Exec(env.ctx, `SELECT relationship_id FROM company.carrier_relationship_heads WHERE relationship_id=$1 FOR UPDATE`, race.relationship); err != nil {
		t.Fatal(err)
	}
	raceCmd := oneSubjectCommand(race.operating, race.operatingCarrier, raceShip, nil, nil)
	raceDone := make(chan error, 1)
	raceFinished := make(chan struct{})
	go func() {
		_, projectErr := runtimeRepo.Project(env.ctx, raceCmd)
		raceDone <- projectErr
		close(raceFinished)
	}()
	waitForAdvisoryLock(t, env.ctx, admin, race.shipment.tenantID, race.sourceCarrier, race.operating, 1, 1, raceFinished)
	revokedVersion := uuid.New()
	if _, err := hold.Exec(env.ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by, revoke_reason
		) VALUES ($1,$2,2,'REVOKED',$3,$4,$5,'ended')
	`, revokedVersion, race.relationship, race.version, race.request, race.user); err != nil {
		t.Fatal(err)
	}
	if _, err := hold.Exec(env.ctx, `
		UPDATE company.carrier_relationship_heads
		SET status='REVOKED', current_version=2, current_version_id=$2
		WHERE relationship_id=$1
	`, race.relationship, revokedVersion); err != nil {
		t.Fatal(err)
	}
	if err := hold.Commit(env.ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-raceDone; appReason(err) != domain.ReasonRelationshipRevoked {
		t.Fatalf("revoke before activation: %v", err)
	}
	if activationCount(t, env.ctx, admin, raceCmd.ActivationID) != 0 || executionCountForActivation(t, env.ctx, admin, raceCmd.ActivationID) != 0 {
		t.Fatal("revoke-first activation left an execution")
	}
	t.Log("TAP-02=PASS")
	t.Log("TAP-03=PASS")
	t.Log("RT02_REVOKE_FIRST=PASS")

	activationFirst := seedRelationship(t, env.ctx, admin, "activation-first")
	activationCmd := oneSubjectCommand(activationFirst.operating, activationFirst.operatingCarrier, activationFirst.shipment, nil, nil)
	lockHeld := make(chan struct{})
	releaseProject := make(chan struct{})
	var signalOnce sync.Once
	restoreHook := repository.SetActivationAuthorizedHookForTest(func(ctx context.Context) error {
		signalOnce.Do(func() { close(lockHeld) })
		select {
		case <-releaseProject:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	defer restoreHook()
	activationDone := make(chan error, 1)
	activationFinished := make(chan struct{})
	var activationResult domain.ProjectionResult
	go func() {
		var projectErr error
		activationResult, projectErr = runtimeRepo.Project(env.ctx, activationCmd)
		activationDone <- projectErr
		close(activationFinished)
	}()
	select {
	case <-lockHeld:
	case <-activationFinished:
		t.Fatalf("activation-first project finished before the advisory lock: %v", <-activationDone)
	case <-time.After(30 * time.Second):
		t.Fatal("activation-first project did not reach the advisory lock")
	}
	waitForAdvisoryLock(t, env.ctx, admin, activationFirst.shipment.tenantID, activationFirst.sourceCarrier, activationFirst.operating, 1, 0, activationFinished)
	revokeDone := make(chan error, 1)
	revokeFinished := make(chan struct{})
	go func() {
		revokeDone <- revokeRelationshipAfterLock(env.ctx, admin, activationFirst)
		close(revokeFinished)
	}()
	waitForAdvisoryLock(t, env.ctx, admin, activationFirst.shipment.tenantID, activationFirst.sourceCarrier, activationFirst.operating, 1, 1, activationFinished)
	close(releaseProject)
	if err := <-activationDone; err != nil || !activationResult.Created {
		t.Fatalf("activation-first project: %v %+v", err, activationResult)
	}
	select {
	case err := <-revokeDone:
		if err != nil {
			t.Fatalf("activation-first revoke: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("activation-first revoke did not finish")
	}
	if executionCountForActivation(t, env.ctx, admin, activationCmd.ActivationID) != 1 || activationCount(t, env.ctx, admin, activationCmd.ActivationID) != 1 {
		t.Fatal("activation-first race lost the execution or duplicated authorization")
	}
	var pinnedVersion uuid.UUID
	var headStatus string
	if err := admin.QueryRow(env.ctx, `
		SELECT pin.relationship_version_id, head.status
		FROM transport.transport_execution_carrier_relationship_pins AS pin
		JOIN company.carrier_relationship_heads AS head ON head.relationship_id = $2
		WHERE pin.execution_revision_id = $1
	`, activationResult.RevisionID, activationFirst.relationship).Scan(&pinnedVersion, &headStatus); err != nil || pinnedVersion != activationFirst.version || headStatus != "REVOKED" {
		t.Fatalf("activation-first pin=%s head=%s err=%v", pinnedVersion, headStatus, err)
	}
	if scalar(t, env.ctx, admin, `SELECT count(*) FROM company.carrier_relationship_audit WHERE action='ACTIVATION_AUTHORIZED' AND correlation_id=$1`, activationCmd.ActivationID.String()) != 1 {
		t.Fatal("activation-first duplicated audit")
	}
	if scalar(t, env.ctx, admin, `SELECT count(*) FROM company.carrier_relationship_outbox WHERE event_type='carrier_relationship.activation_authorized' AND payload->>'source_activation_id'=$1`, activationCmd.ActivationID.String()) != 1 {
		t.Fatal("activation-first duplicated outbox")
	}
	if scalar(t, env.ctx, admin, `SELECT count(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND event_type='shipment.execution_plan.created'`, activationResult.ExecutionID) != 1 {
		t.Fatal("activation-first duplicated execution outbox")
	}
	restoreHook()
	t.Log("RT02_ACTIVATION_FIRST=PASS")

	parallelShip := seedRelatedShipment(t, env.ctx, admin, live.shipment, live.sourceCarrier, "parallel")
	parallel := oneSubjectCommand(live.operating, live.operatingCarrier, parallelShip, &driver, &vehicle)
	var wg sync.WaitGroup
	results := make([]domain.ProjectionResult, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = runtimeRepo.Project(env.ctx, parallel)
		}(i)
	}
	wg.Wait()
	createdCount := 0
	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Fatalf("parallel project %d: %v", i, errs[i])
		}
		if results[i].Created {
			createdCount++
		}
		if results[i].ExecutionID != results[0].ExecutionID {
			t.Fatal("parallel project created two executions")
		}
	}
	if createdCount != 1 || activationCount(t, env.ctx, admin, parallel.ActivationID) != 1 {
		t.Fatalf("parallel created=%d auth=%d", createdCount, activationCount(t, env.ctx, admin, parallel.ActivationID))
	}
	t.Log("TAP-19=PASS")

	restore = repository.SetSuccessorPinPropagateForTest(func(context.Context, pgx.Tx, uuid.UUID) error {
		return errors.New("pin copy failed")
	})
	currentBefore := currentRevisionID(t, env.ctx, admin, created.ExecutionID)
	if _, err := runtimeRepo.CreateSuccessorRevision(env.ctx, successorFromProjection(live.operating, created, live.shipment)); err == nil {
		restore()
		t.Fatal("pin copy failure committed")
	}
	restore()
	if currentRevisionID(t, env.ctx, admin, created.ExecutionID) != currentBefore || pinCount(t, env.ctx, admin, created.RevisionID) != 1 {
		t.Fatal("pin copy failure changed the execution")
	}
	t.Log("TAP-24=PASS")

	successor, err := runtimeRepo.CreateSuccessorRevision(env.ctx, successorFromProjection(live.operating, created, live.shipment))
	if err != nil {
		t.Fatalf("pinned successor: %v", err)
	}
	if activationCount(t, env.ctx, admin, cmd.ActivationID) != 1 || pinCount(t, env.ctx, admin, successor.RevisionID) != 1 {
		t.Fatal("successor did not preserve the pin set")
	}
	var originalVersion, copiedVersion uuid.UUID
	if err := admin.QueryRow(env.ctx, `
		SELECT o.relationship_version_id, p.relationship_version_id
		FROM transport.transport_execution_carrier_relationship_pins o
		JOIN transport.transport_execution_carrier_relationship_pins p
		  ON p.authorization_subject_id = o.authorization_subject_id
		WHERE o.execution_revision_id=$1 AND p.execution_revision_id=$2
	`, created.RevisionID, successor.RevisionID).Scan(&originalVersion, &copiedVersion); err != nil || originalVersion != copiedVersion || originalVersion != live.version {
		t.Fatalf("pin versions original=%s copied=%s head=%s err=%v", originalVersion, copiedVersion, live.version, err)
	}
	laterVersion := uuid.New()
	if _, err := admin.Exec(env.ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by, revoke_reason
		) VALUES ($1,$2,2,'REVOKED',$3,$4,$5,'later')
	`, laterVersion, live.relationship, live.version, live.request, live.user); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `
		UPDATE company.carrier_relationship_heads
		SET status='REVOKED', current_version=2, current_version_id=$2
		WHERE relationship_id=$1
	`, live.relationship, laterVersion); err != nil {
		t.Fatal(err)
	}
	var afterOriginal, afterCopy uuid.UUID
	if err := admin.QueryRow(env.ctx, `SELECT relationship_version_id FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id=$1`, created.RevisionID).Scan(&afterOriginal); err != nil || afterOriginal != originalVersion {
		t.Fatalf("original pin after revoke=%s err=%v", afterOriginal, err)
	}
	if err := admin.QueryRow(env.ctx, `SELECT relationship_version_id FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id=$1`, successor.RevisionID).Scan(&afterCopy); err != nil || afterCopy != copiedVersion {
		t.Fatalf("successor pin after revoke=%s err=%v", afterCopy, err)
	}
	t.Log("TAP-12=PASS")
	t.Log("TAP-13=PASS")
	t.Log("TAP-23=PASS")
}

type relationshipFixture struct {
	operating        uuid.UUID
	operatingCarrier uuid.UUID
	siblingCarrier   uuid.UUID
	sourceCarrier    uuid.UUID
	shipment         seededShipment
	relationship     uuid.UUID
	version          uuid.UUID
	request          uuid.UUID
	user             uuid.UUID
}

func applyNamedMigrations(t *testing.T, env *execEnv, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func startActivationPostgres(t *testing.T) (*execEnv, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(cancel)
	adminURL := ""
	if os.Getenv("CI") == "true" {
		adminURL = strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
		if adminURL == "" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
	} else {
		adminURL = startEmbeddedActivationPostgres(t)
	}
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	dbName := "act_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	testURL, cleanup := createDatabase(t, ctx, adminURL, dbName)
	t.Cleanup(func() { cleanup() })
	bootstrap, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bootstrap.Close)
	if _, err := bootstrap.Exec(ctx, `CREATE ROLE freight LOGIN SUPERUSER PASSWORD 'freight'`); err != nil {
		t.Fatal(err)
	}
	freightURL := fmt.Sprintf("postgres://freight:freight@%s:%d/%s?sslmode=disable", cfg.ConnConfig.Host, cfg.ConnConfig.Port, dbName)
	pool, err := pgxpool.New(ctx, freightURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migrations, err := locateMigrations()
	if err != nil {
		t.Fatal(err)
	}
	env := &execEnv{ctx: ctx, pool: pool, migrations: migrations}
	if err := applySelectedMigrations(ctx, pool, migrations); err != nil {
		t.Fatal(err)
	}
	return env, bootstrap
}

func startEmbeddedActivationPostgres(t *testing.T) string {
	t.Helper()
	port := freePort(t)
	runtimePath := filepath.Join(t.TempDir(), "pg-runtime")
	dataPath := filepath.Join(t.TempDir(), "pg-data")
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(runtimePath).
		DataPath(dataPath).
		Database("freight_activation_test").
		Username("postgres").
		Password("postgres").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	return fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/postgres?sslmode=disable", port)
}

func installActivationGate(t *testing.T, env *execEnv, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	if _, err := admin.Exec(env.ctx, `GRANT shipment_runtime TO freight`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `ALTER TABLE transport.transport_execution_revisions OWNER TO freight_migrations`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `ALTER ROLE freight NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(env.ctx, `
		REVOKE ALL ON transport.transport_execution_carrier_relationship_pins FROM freight;
		REVOKE ALL ON company.carrier_relationship_activation_authorizations FROM freight;
		REVOKE ALL ON company.carrier_relationship_activation_subjects FROM freight
	`); err != nil {
		t.Fatal(err)
	}
	if err := execSQLFile(env.ctx, admin, filepath.Join(env.migrations, "..", "postgres", "tms_activation_enforcement_gate_v0.1.sql")); err != nil {
		t.Fatal(err)
	}
	return admin
}

func runtimePool(t *testing.T, env *execEnv) *pgxpool.Pool {
	t.Helper()
	port, dbName := serverDatabase(t, env.ctx, env.pool)
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/%s?sslmode=disable", port, dbName))
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE shipment_runtime`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(env.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func serverDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (int, string) {
	t.Helper()
	var port int
	var name string
	if err := pool.QueryRow(ctx, `SELECT inet_server_port(), current_database()`).Scan(&port, &name); err != nil {
		t.Fatal(err)
	}
	return port, name
}

func seedRelationship(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string) relationshipFixture {
	t.Helper()
	operating := insertTenant(t, ctx, pool, prefix+"-op")
	shipperTenant := insertTenant(t, ctx, pool, prefix+"-sh")
	operatingCarrier := insertCompany(t, ctx, pool, operating, "CARRIER", prefix+" operating")
	siblingCarrier := insertCompany(t, ctx, pool, operating, "CARRIER", prefix+" sibling")
	sourceCarrier := insertCompany(t, ctx, pool, shipperTenant, "CARRIER", prefix+" source")
	shipper := insertCompany(t, ctx, pool, shipperTenant, "SHIPPER", prefix+" shipper")
	consignee := insertCompany(t, ctx, pool, shipperTenant, "CONSIGNEE", prefix+" consignee")
	shipment := insertShipment(t, ctx, pool, shipperTenant, shipper, consignee, sourceCarrier, prefix)
	user := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,'Shipper Admin')`, user, shipperTenant, prefix+"-"+user.String()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	request := uuid.New()
	relationship := uuid.New()
	version := uuid.New()
	var token [32]byte
	left, right := uuid.New(), uuid.New()
	copy(token[:16], left[:])
	copy(token[16:], right[:])
	if _, err := pool.Exec(ctx, `
		WITH stamp AS (SELECT clock_timestamp() AS created)
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'v1', $4, $5, $6, $7, stamp.created, stamp.created + interval '72 hours', $6
		FROM stamp
	`, request, shipment.shipmentID, shipperTenant, sourceCarrier, shipper, user, token[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationships (
			relationship_id, shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id, operating_carrier_company_id
		) VALUES ($1,$2,$3,$4,$5)
	`, relationship, shipperTenant, sourceCarrier, operating, operatingCarrier); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status, source_relationship_request_id, created_by
		) VALUES ($1,$2,1,'ACTIVE',$3,$4)
	`, version, relationship, request, user); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_heads (relationship_id, current_version, current_version_id, status)
		VALUES ($1,1,$2,'ACTIVE')
	`, relationship, version); err != nil {
		t.Fatal(err)
	}
	return relationshipFixture{
		operating: operating, operatingCarrier: operatingCarrier, siblingCarrier: siblingCarrier,
		sourceCarrier: sourceCarrier, shipment: shipment, relationship: relationship,
		version: version, request: request, user: user,
	}
}

func insertTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, id, prefix+"-"+id.String()[:8], prefix); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertCompany(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, kind, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`, id, tenantID, name, kind); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertShipment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, shipperID, consigneeID, carrierID uuid.UUID, prefix string) seededShipment {
	t.Helper()
	originID, destID := uuid.New(), uuid.New()
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{{originID, "Origin"}, {destID, "Destination"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, row.id, tenantID, row.name); err != nil {
			t.Fatal(err)
		}
	}
	cargoID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description) VALUES ($1,$2,'GENERAL','Cargo')`, cargoID, tenantID); err != nil {
		t.Fatal(err)
	}
	orderID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode
		) VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')
	`, orderID, tenantID, "TO-"+orderID.String()[:8], shipperID, consigneeID, originID, destID); err != nil {
		t.Fatal(err)
	}
	shipmentID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ROAD','IN_TRANSIT',1)
	`, shipmentID, tenantID, prefix+"-"+shipmentID.String()[:8], orderID, shipperID, consigneeID, carrierID, originID, destID, cargoID); err != nil {
		t.Fatal(err)
	}
	return seededShipment{
		tenantID: tenantID, shipperID: shipperID, consigneeID: consigneeID, originID: originID,
		cargoID: cargoID, shipmentID: shipmentID, subjectID: uuid.New(),
	}
}

func seedRelatedShipment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, base seededShipment, carrierID uuid.UUID, prefix string) seededShipment {
	t.Helper()
	return insertShipment(t, ctx, pool, base.tenantID, base.shipperID, base.consigneeID, carrierID, prefix)
}

func seedShipmentOn(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix, status string) seededShipment {
	t.Helper()
	tenantID := insertTenant(t, ctx, pool, prefix)
	shipperID := insertCompany(t, ctx, pool, tenantID, "SHIPPER", prefix+" shipper")
	consigneeID := insertCompany(t, ctx, pool, tenantID, "CONSIGNEE", prefix+" consignee")
	carrierID := insertCompany(t, ctx, pool, tenantID, "CARRIER", prefix+" carrier")
	row := insertShipment(t, ctx, pool, tenantID, shipperID, consigneeID, carrierID, prefix)
	if _, err := pool.Exec(ctx, `UPDATE transport.shipments SET status=$2 WHERE id=$1`, row.shipmentID, status); err != nil {
		t.Fatal(err)
	}
	return row
}

func seedAssignment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, carrierID uuid.UUID, kind string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var err error
	if kind == "vehicle" {
		_, err = pool.Exec(ctx, `
			INSERT INTO transport.vehicles (id, tenant_id, carrier_company_id, plate_number, vehicle_type, status)
			VALUES ($1,$2,$3,$4,'TRUCK','ACTIVE')
		`, id, tenantID, carrierID, "P-"+id.String()[:8])
	} else {
		_, err = pool.Exec(ctx, `
			INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, full_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')
		`, id, tenantID, carrierID, kind+" "+id.String()[:8])
	}
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func oneSubjectCommand(operating, carrier uuid.UUID, ship seededShipment, driverID, vehicleID *uuid.UUID) domain.ProjectionCommand {
	version := 1
	start, cargoStop, end := uuid.New(), uuid.New(), uuid.New()
	duration := 900
	return domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 2, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 4, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: operating, ContextShipmentID: &ship.shipmentID, ContextShipmentTenantID: &ship.tenantID,
		ContextShipmentVersion: &version, CarrierCompanyID: carrier, VehicleID: vehicleID, DriverID: driverID,
		EvaluationFingerprint: "route-fingerprint",
		ExecutionSubjects:     oneSubjectRows(ship),
		Stops: []domain.ProjectionStop{
			{RoutePlanStopID: start, Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.75, Longitude: 37.62},
			{RoutePlanStopID: cargoStop, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &ship.originID, Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration},
			{RoutePlanStopID: end, Ordinal: 2, StopRole: domain.StopRoleEnd, PointKind: domain.PointKindCanonicalLocation, LocationID: &ship.originID, Latitude: 56.1, Longitude: 38.1},
		},
		Actions: oneSubjectActions(cargoStop, ship),
	}
}

func oneSubjectRows(ship seededShipment) []domain.ProjectionSubject {
	version := 1
	return []domain.ProjectionSubject{materialSubject(ship, version)}
}

func oneSubjectActions(stopID uuid.UUID, ship seededShipment) []domain.ProjectionAction {
	return []domain.ProjectionAction{materialAction(stopID, ship, 1, 0)}
}

func successorFromProjection(operating uuid.UUID, result domain.ProjectionResult, ship seededShipment) domain.SuccessorRevisionCommand {
	return domain.SuccessorRevisionCommand{
		ExecutionID: result.ExecutionID, ExpectedRevisionID: result.RevisionID, OperatingTenantID: operating,
		IdempotencyKey: "succ-" + result.RevisionID.String(), ReasonCode: "REMAINING_ROUTE_CHANGED",
		OccurredAt: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), ActorKind: domain.ActorKindSystem,
		SourceRoutePlanID: uuid.New(), SourceRoutePlanVersion: 2, SourceActivationID: uuid.New(), SourceActivationVersion: 1,
		PlanningMode: domain.PlanningModeDepotStart, EvaluationFingerprint: "successor-fingerprint",
		Stops: []domain.SuccessorStopInput{{
			StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &ship.originID,
			Latitude: 55.76, Longitude: 37.63,
		}},
		Actions: []domain.SuccessorActionInput{{
			StopIndex: 0, ActionType: domain.ActionTypeDelivery, ShipmentID: ship.shipmentID, CargoID: ship.cargoID,
		}},
	}
}

func assertPinned(t *testing.T, ctx context.Context, pool *pgxpool.Pool, result domain.ProjectionResult, live relationshipFixture, activationID uuid.UUID, subjects int) {
	t.Helper()
	if activationCount(t, ctx, pool, activationID) != 1 || pinCount(t, ctx, pool, result.RevisionID) != subjects {
		t.Fatalf("evidence auth=%d pins=%d", activationCount(t, ctx, pool, activationID), pinCount(t, ctx, pool, result.RevisionID))
	}
	var version uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT relationship_version_id FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id=$1`, result.RevisionID).Scan(&version); err != nil || version != live.version {
		t.Fatalf("locked version=%s head=%s err=%v", version, live.version, err)
	}
	if scalar(t, ctx, pool, `SELECT count(*) FROM company.carrier_relationship_audit WHERE action='ACTIVATION_AUTHORIZED' AND correlation_id=$1`, activationID.String()) != 1 {
		t.Fatal("activation audit missing")
	}
	if scalar(t, ctx, pool, `SELECT count(*) FROM company.carrier_relationship_outbox WHERE event_type='carrier_relationship.activation_authorized' AND payload->>'source_activation_id'=$1`, activationID.String()) != 1 {
		t.Fatal("activation outbox missing")
	}
	if scalar(t, ctx, pool, `SELECT count(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND event_type='shipment.execution_plan.created'`, result.ExecutionID) != 1 {
		t.Fatal("execution outbox missing")
	}
}

func triggerCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return scalar(t, context.Background(), pool, `SELECT count(*) FROM pg_trigger WHERE tgname='trg_new_activation_authorization' AND NOT tgisinternal`)
}

func activationTriggerOnRevisions(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	return scalar(t, ctx, pool, `
		SELECT count(*)
		FROM pg_catalog.pg_trigger
		WHERE tgname = 'trg_new_activation_authorization'
		  AND tgrelid = 'transport.transport_execution_revisions'::regclass
		  AND NOT tgisinternal
	`)
}

func assertActivationTriggerIdentity(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var ready bool
	err := pool.QueryRow(ctx, `
		SELECT count(*) = 1
		FROM pg_catalog.pg_trigger AS trg
		JOIN pg_catalog.pg_proc AS fn ON fn.oid = trg.tgfoid
		JOIN pg_catalog.pg_namespace AS fn_ns ON fn_ns.oid = fn.pronamespace
		WHERE trg.tgrelid = 'transport.transport_execution_revisions'::regclass
		  AND trg.tgname = 'trg_new_activation_authorization'
		  AND NOT trg.tgisinternal
		  AND trg.tgenabled IN ('O', 'A')
		  AND trg.tgconstraint <> 0
		  AND trg.tgdeferrable
		  AND trg.tginitdeferred
		  AND fn_ns.nspname = 'transport'
		  AND fn.proname = 'enforce_new_activation_authorization'
		  AND fn.pronargs = 0
	`).Scan(&ready)
	if err != nil || !ready {
		t.Fatalf("activation trigger identity ready=%v err=%v", ready, err)
	}
}

func assertNoActivationWrite(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *repository.TransportExecutionRepository, cmd domain.ProjectionCommand, label string) {
	t.Helper()
	execBefore := executionCount(t, ctx, pool)
	authBefore := scalar(t, ctx, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`)
	pinBefore := scalar(t, ctx, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`)
	if _, err := repo.Project(ctx, cmd); appReason(err) != domain.ReasonRelationshipStoreUnavailable {
		t.Fatalf("%s: %v", label, err)
	}
	if executionCount(t, ctx, pool) != execBefore ||
		scalar(t, ctx, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations`) != authBefore ||
		scalar(t, ctx, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins`) != pinBefore ||
		activationCount(t, ctx, pool, cmd.ActivationID) != 0 ||
		executionCountForActivation(t, ctx, pool, cmd.ActivationID) != 0 {
		t.Fatalf("%s left an execution, authorization, or pin", label)
	}
}

func waitForAdvisoryLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, sourceID, operatingID uuid.UUID, minGranted, minWaiting int, finished <-chan struct{}) {
	t.Helper()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var granted, waiting int
	for {
		select {
		case <-finished:
			t.Fatalf("project finished before advisory barrier granted=%d waiting=%d", granted, waiting)
		default:
		}
		if err := pool.QueryRow(ctx, `
			WITH key AS (
				SELECT pg_catalog.hashtextextended(
					'relationship:' || $1::text || ':' || $2::text || ':' || $3::text, 0
				) AS k
			)
			SELECT
				count(*) FILTER (WHERE l.granted),
				count(*) FILTER (WHERE NOT l.granted)
			FROM key
			JOIN pg_catalog.pg_locks AS l
			  ON l.locktype = 'advisory'
			 AND l.objsubid = 1
			 AND l.classid = ((key.k >> 32) & 4294967295)::oid
			 AND l.objid = (key.k & 4294967295)::oid
		`, tenantID, sourceID, operatingID).Scan(&granted, &waiting); err != nil {
			t.Fatal(err)
		}
		if granted >= minGranted && waiting >= minWaiting {
			return
		}
		select {
		case <-finished:
			t.Fatalf("project finished before advisory barrier granted=%d waiting=%d", granted, waiting)
		case <-deadline.C:
			t.Fatalf("advisory barrier timed out granted=%d waiting=%d want >=%d/>=%d", granted, waiting, minGranted, minWaiting)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func revokeRelationshipAfterLock(ctx context.Context, pool *pgxpool.Pool, fx relationshipFixture) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_catalog.set_config('lock_timeout', '30000', true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		SELECT pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(
			'relationship:' || $1::text || ':' || $2::text || ':' || $3::text, 0))
	`, fx.shipment.tenantID, fx.sourceCarrier, fx.operating); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT relationship_id FROM company.carrier_relationship_heads WHERE relationship_id=$1 FOR UPDATE`, fx.relationship); err != nil {
		return err
	}
	revokedVersion := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO company.carrier_relationship_versions (
			relationship_version_id, relationship_id, relationship_version, status,
			supersedes_relationship_version_id, source_relationship_request_id, created_by, revoke_reason
		) VALUES ($1,$2,2,'REVOKED',$3,$4,$5,'activation-first')
	`, revokedVersion, fx.relationship, fx.version, fx.request, fx.user); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE company.carrier_relationship_heads
		SET status='REVOKED', current_version=2, current_version_id=$2
		WHERE relationship_id=$1 AND status='ACTIVE'
	`, fx.relationship, revokedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("relationship head was not revoked")
	}
	return tx.Commit(ctx)
}

func executionCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	return scalar(t, ctx, pool, `SELECT count(*) FROM transport.transport_executions`)
}

func executionCountForActivation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, activationID uuid.UUID) int {
	t.Helper()
	return scalar(t, ctx, pool, `SELECT count(*) FROM transport.transport_execution_revisions WHERE source_activation_id=$1`, activationID)
}

func activationCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, activationID uuid.UUID) int {
	t.Helper()
	return scalar(t, ctx, pool, `SELECT count(*) FROM company.carrier_relationship_activation_authorizations WHERE source_activation_id=$1`, activationID)
}

func pinCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, revisionID uuid.UUID) int {
	t.Helper()
	return scalar(t, ctx, pool, `SELECT count(*) FROM transport.transport_execution_carrier_relationship_pins WHERE execution_revision_id=$1`, revisionID)
}

func shipperParticipantCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) int {
	t.Helper()
	return scalar(t, ctx, pool, `SELECT count(*) FROM transport.transport_execution_participants WHERE shipment_tenant_id=$1`, tenantID)
}

func currentRevisionID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, executionID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT current_revision_id FROM transport.transport_executions WHERE id=$1`, executionID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func scalar(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func appReason(err error) string {
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr == nil {
		return ""
	}
	if reason, ok := appErr.Details["reason"].(string); ok && reason != "" {
		return reason
	}
	return appErr.Message
}

func permissionDenied(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "42501") || strings.Contains(text, "permission denied")
}

func containsText(err error, text string) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), strings.ToLower(text))
}

func repeatHex(char byte) string {
	return strings.Repeat(string(char), 64)
}

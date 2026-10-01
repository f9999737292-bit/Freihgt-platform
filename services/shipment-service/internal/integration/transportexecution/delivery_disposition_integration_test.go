//go:build integration

package transportexecution

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestDeliveryDisposition(t *testing.T) {
	env := startPostgres(t)
	up := filepath.Join(env.migrations, "000093_tms_delivery_disposition_v0_1.up.sql")
	down := filepath.Join(env.migrations, "000093_tms_delivery_disposition_v0_1.down.sql")
	if err := execSQLFile(env.ctx, env.pool, up); err != nil {
		t.Fatal(err)
	}
	if err := execSQLFile(env.ctx, env.pool, down); err != nil {
		t.Fatal(err)
	}
	if err := execSQLFile(env.ctx, env.pool, up); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewTransportExecutionRepository(env.pool)
	when := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	t.Run("full acceptance opens no case", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		result := recordDelivery(t, repo, env, seed, 10, 0, "", "", "full-accept", domain.ActorKindOperator)
		if result.CaseID != nil || result.Status != "ACCEPTED" {
			t.Fatalf("full accept %+v", result)
		}
		if countWhere(t, env, "transport.delivery_disposition_cases", "execution_id=$1", seed.executionID) != 0 {
			t.Fatal("acceptance created a case")
		}
	})

	t.Run("partial and full rejection conserve quantity", func(t *testing.T) {
		seed := seedDelivery(t, env, 20, true)
		arriveStop(t, env, seed.stopID)
		partial := recordDelivery(t, repo, env, seed, 17, 3, domain.ReasonDamage, "", "partial", domain.ActorKindDriver)
		if partial.CaseID == nil || partial.AcceptedQuantity != 17 || partial.RejectedQuantity != 3 {
			t.Fatalf("partial %+v", partial)
		}
		assertCaseQty(t, env, *partial.CaseID, 17, 3, 3, 0, 0, 0, 0)
		if onboardRemaining(t, env, seed) != 3 {
			t.Fatalf("rejected left onboard, remaining=%d", onboardRemaining(t, env, seed))
		}
		if reasonOf(recordErr(t, repo, env, seed, 0, 4, domain.ReasonDamage, "", "over-rest", domain.ActorKindOperator)) != domain.ReasonQuantityExceeded {
			t.Fatal("second rejection exceeded remaining onboard quantity")
		}
		full := seedDelivery(t, env, 6, true)
		arriveStop(t, env, full.stopID)
		rejected := recordDelivery(t, repo, env, full, 0, 6, domain.ReasonDamage, "", "full-reject", domain.ActorKindOperator)
		if rejected.CaseID == nil || rejected.AcceptedQuantity != 0 || rejected.RejectedQuantity != 6 {
			t.Fatalf("full rejection %+v", rejected)
		}
	})

	t.Run("over onboard zero and bad reason denied", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		if reasonOf(recordErr(t, repo, env, seed, 9, 2, domain.ReasonDamage, "", "over-onboard", domain.ActorKindOperator)) != domain.ReasonQuantityExceeded {
			t.Fatal("expected quantity exceeded")
		}
		if reasonOf(recordErr(t, repo, env, seed, 0, 0, domain.ReasonDamage, "", "zero", domain.ActorKindOperator)) != domain.ReasonQuantityInvalid {
			t.Fatal("expected invalid quantity")
		}
		if reasonOf(recordErr(t, repo, env, seed, 1, -1, domain.ReasonDamage, "", "neg", domain.ActorKindOperator)) != domain.ReasonQuantityInvalid {
			t.Fatal("expected negative denial")
		}
		if reasonOf(recordErr(t, repo, env, seed, 0, 1, "NOPE", "", "bad-reason", domain.ActorKindOperator)) != domain.ReasonReasonUnknown {
			t.Fatal("expected unknown reason")
		}
		if reasonOf(recordErr(t, repo, env, seed, 0, 1, domain.ReasonOther, "", "other-empty", domain.ActorKindOperator)) != domain.ReasonCommentRequired {
			t.Fatal("expected comment")
		}
		ok := recordDelivery(t, repo, env, seed, 0, 1, domain.ReasonOther, "free text", "other-ok", domain.ActorKindOperator)
		if ok.CaseID == nil {
			t.Fatal("other with comment should open a case")
		}
		if reasonOf(recordErr(t, repo, env, seed, 0, 1, domain.ReasonDamage, "", "kg", domain.ActorKindOperator, "KG")) != domain.ReasonUOMInvalid {
			t.Fatal("expected uom denial")
		}
	})

	t.Run("cargo not onboard and foreign identities denied", func(t *testing.T) {
		dry := seedDelivery(t, env, 10, false)
		arriveStop(t, env, dry.stopID)
		if reasonOf(recordErr(t, repo, env, dry, 0, 1, domain.ReasonDamage, "", "not-onboard", domain.ActorKindOperator)) != domain.ReasonCargoNotOnboard {
			t.Fatal("expected not onboard")
		}
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		foreignTenant := recordCommand(seed, 0, 1, domain.ReasonDamage, "", "foreign-tenant", domain.ActorKindOperator, domain.UOMPallet)
		foreignTenant.OperatingTenantID = uuid.New()
		if _, err := repo.RecordDeliveryDisposition(env.ctx, foreignTenant); reasonOf(err) != domain.ReasonTenantDenied {
			t.Fatalf("foreign tenant %v", err)
		}
		missing := foreignTenant
		missing.OperatingTenantID = seed.operating
		missing.ExecutionID = uuid.New()
		missing.IdempotencyKey = "foreign-exec"
		var app *apperrors.AppError
		_, err := repo.RecordDeliveryDisposition(env.ctx, missing)
		if !errorsAsNotFound(err, &app) {
			t.Fatalf("foreign execution %v", err)
		}
		badShip := recordCommand(seed, 0, 1, domain.ReasonDamage, "", "foreign-ship", domain.ActorKindOperator, domain.UOMPallet)
		badShip.ShipmentID = uuid.New()
		if reasonOf(mustErr(repo.RecordDeliveryDisposition(env.ctx, badShip))) != domain.ReasonNotParticipant {
			t.Fatal("expected foreign shipment denial")
		}
		badCargo := recordCommand(seed, 0, 1, domain.ReasonDamage, "", "foreign-cargo", domain.ActorKindOperator, domain.UOMPallet)
		badCargo.CargoID = uuid.New()
		if reasonOf(mustErr(repo.RecordDeliveryDisposition(env.ctx, badCargo))) != domain.ReasonNotParticipant {
			t.Fatal("expected foreign cargo denial")
		}
		badStop := recordCommand(seed, 0, 1, domain.ReasonDamage, "", "foreign-stop", domain.ActorKindOperator, domain.UOMPallet)
		badStop.SourceStopID = uuid.New()
		if reasonOf(mustErr(repo.RecordDeliveryDisposition(env.ctx, badStop))) != domain.ReasonStopNotInRevision {
			t.Fatal("expected stop denial")
		}
		stale := recordCommand(seed, 0, 1, domain.ReasonDamage, "", "stale", domain.ActorKindOperator, domain.UOMPallet)
		stale.ExpectedRevisionID = uuid.New()
		if reasonOf(mustErr(repo.RecordDeliveryDisposition(env.ctx, stale))) != domain.ReasonRevisionConflict {
			t.Fatal("expected revision conflict")
		}
	})

	t.Run("idempotency replay and body conflict", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		cmd := recordCommand(seed, 8, 2, domain.ReasonDamage, "dent", "idem-1", domain.ActorKindOperator, domain.UOMPallet)
		first, err := repo.RecordDeliveryDisposition(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		second, err := repo.RecordDeliveryDisposition(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if !second.Replayed || second.CaseID == nil || *second.CaseID != *first.CaseID {
			t.Fatalf("replay %+v %+v", first, second)
		}
		if countWhere(t, env, "transport.delivery_disposition_cases", "execution_id=$1", seed.executionID) != 1 {
			t.Fatal("replay created a second case")
		}
		if reasonOf(recordErr(t, repo, env, seed, 7, 3, domain.ReasonDamage, "dent", "idem-1", domain.ActorKindOperator)) != domain.ReasonCommandBodyConflict {
			t.Fatal("expected body conflict")
		}
	})

	t.Run("hold does not replan and drivers cannot authorize", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		before := countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID)
		created := recordDelivery(t, repo, env, seed, 9, 1, domain.ReasonCustomerRefusal, "", "hold-case", domain.ActorKindDriver)
		held, err := repo.HoldDisposition(env.ctx, domain.HoldDispositionCommand{
			CaseID: *created.CaseID, ExecutionID: seed.executionID, ExpectedRevisionID: seed.revisionID,
			OperatingTenantID: seed.operating, IdempotencyKey: "hold-1", OccurredAt: when,
			ActorKind: domain.ActorKindOperator, ActorID: uuid.New(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if held.Status != domain.DispositionStatusPending {
			t.Fatalf("hold status %s", held.Status)
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID) != before {
			t.Fatal("hold created a revision")
		}
		auth := authorizeCmd(seed, *created.CaseID, seed.ship.originID, nil, "driver-return")
		auth.ActorKind = domain.ActorKindDriver
		auth.ActorID = uuid.New()
		_, err = repo.AuthorizeReturn(env.ctx, auth)
		if reasonOf(err) != domain.ReasonActorDenied {
			t.Fatalf("driver return %v", err)
		}
		redirect := auth
		redirect.IdempotencyKey = "driver-redirect"
		redirect.TargetLocationID = seed.redirectID
		if reasonOf(mustErr(repo.AuthorizeRedirect(env.ctx, redirect))) != domain.ReasonActorDenied {
			t.Fatal("driver redirect should be denied")
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID) != before {
			t.Fatal("denied authorization changed the route")
		}
	})

	t.Run("service started blocks successor", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		completeStop(t, env, stopByRoleOrdinal(t, env, seed.executionID, domain.StopRoleStart))
		serviceStop(t, env, seed.stopID)
		created := recordDelivery(t, repo, env, seed, 4, 6, domain.ReasonPackagingDamage, "", "in-service", domain.ActorKindOperator)
		before := currentRevision(t, env, seed.executionID)
		_, err := repo.AuthorizeRedirect(env.ctx, authorizeCmd(seed, *created.CaseID, seed.redirectID, []uuid.UUID{seed.endID}, "svc-redirect"))
		if reasonOf(err) != domain.ReasonExecutionStopInService {
			t.Fatalf("service lock %v", err)
		}
		if currentRevision(t, env, seed.executionID) != before {
			t.Fatal("failed authorization moved the revision")
		}
	})

	t.Run("return and redirect preserve future stops", func(t *testing.T) {
		world := seedMultiStop(t, env)
		statusBefore := shipmentStatus(t, env, world.shipB.shipmentID, world.shipB.tenantID)
		completeStop(t, env, world.startID)
		arriveStop(t, env, world.s1)
		recordAt(t, repo, env, world.operating, world.executionID, world.revisionID, world.s1, world.shipA, 10, 0, "", "", "a-accept")
		completeStop(t, env, world.s1)
		s1Fact := stopFact(t, env, world.s1)
		arriveStop(t, env, world.s2)
		rejected := recordAt(t, repo, env, world.operating, world.executionID, world.revisionID, world.s2, world.shipB, 8, 2, domain.ReasonDamage, "", "b-reject")
		completeStop(t, env, world.s2)
		s2Fact := stopFact(t, env, world.s2)
		arriveStop(t, env, world.s4)
		redirected := recordAt(t, repo, env, world.operating, world.executionID, world.revisionID, world.s4, world.shipD, 5, 1, domain.ReasonWrongProduct, "", "d-reject")
		s4Fact := stopFact(t, env, world.s4)
		if onboardRemainingShipment(t, env, world.executionID, world.shipB) != 2 {
			t.Fatal("rejected pallets left the vehicle")
		}
		seqBefore := eventSeq(t, env, world.executionID)
		first, err := repo.AuthorizeRedirect(env.ctx, authorizeCmdFor(world.operating, world.executionID, world.revisionID, *redirected.CaseID, world.redirectID, []uuid.UUID{world.s3, world.s4}, "redirect-d"))
		if err != nil {
			t.Fatal(err)
		}
		if first.RevisionID == world.revisionID || eventSeq(t, env, world.executionID) <= seqBefore {
			t.Fatal("redirect did not create a successor sequence")
		}
		future := introducedStops(t, env, first.RevisionID)
		second, err := repo.AuthorizeReturn(env.ctx, authorizeCmdFor(world.operating, world.executionID, first.RevisionID, *rejected.CaseID, world.shipB.originID, future, "return-after-d"))
		if err != nil {
			t.Fatal(err)
		}
		if second.RevisionID == first.RevisionID || world.executionID != second.ExecutionID {
			t.Fatal("execution id changed")
		}
		if stopFact(t, env, world.s1) != s1Fact || stopFact(t, env, world.s2) != s2Fact || stopFact(t, env, world.s4) != s4Fact {
			t.Fatal("historical stop facts changed")
		}
		if membership(t, env, "transport.transport_execution_revision_stops", world.revisionID, world.s1) != domain.MembershipIntroduced {
			t.Fatal("completed stop membership changed")
		}
		if membershipCount(t, env, "transport.transport_execution_revision_stops", second.RevisionID, world.s1) != 0 {
			t.Fatal("completed stop was linked into the successor")
		}
		if !revisionHasCargo(t, env, second.RevisionID, world.shipC) || !revisionHasCargo(t, env, second.RevisionID, world.shipD) {
			t.Fatal("unrelated future deliveries were dropped")
		}
		if !revisionHasLocation(t, env, second.RevisionID, world.redirectID) || !revisionHasLocation(t, env, second.RevisionID, world.shipB.originID) {
			t.Fatal("redirect or return stop missing")
		}
		assertCaseQty(t, env, *rejected.CaseID, 8, 2, 0, 2, 0, 0, 0)
		assertCaseQty(t, env, *redirected.CaseID, 5, 1, 0, 0, 1, 0, 0)
		if shipmentStatus(t, env, world.shipB.shipmentID, world.shipB.tenantID) != statusBefore {
			t.Fatal("shipment status changed")
		}
		payload := dispositionPayload(t, env, *rejected.CaseID, domain.EventCargoReturnAuthorized)
		raw, _ := json.Marshal(payload)
		for _, forbidden := range []string{world.shipB.tenantID.String(), "latitude", "longitude", "price", "rate", "capacity_snapshot"} {
			if bytes.Contains(raw, []byte(forbidden)) {
				t.Fatalf("event leaked %s", forbidden)
			}
		}
		actionID := introducedActionAt(t, env, second.RevisionID, world.redirectID, world.shipD.cargoID)
		if reasonOf(mustErr(repo.CompleteDisposition(env.ctx, completeCmd(world, *redirected.CaseID, second.RevisionID, actionID, "complete-early")))) != domain.ReasonCompletionEvidenceMissing {
			t.Fatal("completion without evidence should fail")
		}
		completeAction(t, env, actionID)
		done, err := repo.CompleteDisposition(env.ctx, completeCmd(world, *redirected.CaseID, second.RevisionID, actionID, "complete-redirect"))
		if err != nil || done.Status != domain.DispositionStatusResolved {
			t.Fatalf("complete %v %+v", err, done)
		}
		assertCaseQty(t, env, *redirected.CaseID, 5, 1, 0, 0, 0, 0, 1)
	})

	t.Run("return and redirect race has one winner", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		created := recordDelivery(t, repo, env, seed, 8, 2, domain.ReasonMisSort, "", "race-case", domain.ActorKindOperator)
		completeStop(t, env, stopByRoleOrdinal(t, env, seed.executionID, domain.StopRoleStart))
		completeStop(t, env, seed.stopID)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, errs[0] = repo.AuthorizeReturn(env.ctx, authorizeCmd(seed, *created.CaseID, seed.ship.originID, []uuid.UUID{seed.endID}, "race-return"))
		}()
		go func() {
			defer wg.Done()
			_, errs[1] = repo.AuthorizeRedirect(env.ctx, authorizeCmd(seed, *created.CaseID, seed.redirectID, []uuid.UUID{seed.endID}, "race-redirect"))
		}()
		wg.Wait()
		wins := 0
		for _, err := range errs {
			if err == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("wins=%d errs=%v", wins, errs)
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID) != 2 {
			t.Fatal("race created more than one successor")
		}
	})

	t.Run("target validation and http tenant", func(t *testing.T) {
		seed := seedDelivery(t, env, 10, true)
		arriveStop(t, env, seed.stopID)
		created := recordDelivery(t, repo, env, seed, 0, 10, domain.ReasonQualityRejection, "", "targets", domain.ActorKindOperator)
		completeStop(t, env, stopByRoleOrdinal(t, env, seed.executionID, domain.StopRoleStart))
		completeStop(t, env, seed.stopID)
		foreignLoc := uuid.New()
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code, lat, lon) VALUES ($1,$2,'WAREHOUSE','Foreign','RU',1,1)`, foreignLoc, uuid.New()); err != nil {
			t.Fatal(err)
		}
		if reasonOf(mustErr(repo.AuthorizeReturn(env.ctx, authorizeCmd(seed, *created.CaseID, foreignLoc, []uuid.UUID{seed.endID}, "bad-return")))) != domain.ReasonLocationDenied {
			t.Fatal("foreign return target should be denied")
		}
		if reasonOf(mustErr(repo.AuthorizeRedirect(env.ctx, authorizeCmd(seed, *created.CaseID, foreignLoc, []uuid.UUID{seed.endID}, "bad-redirect")))) != domain.ReasonLocationDenied {
			t.Fatal("foreign redirect target should be denied")
		}
		httpSeed := seedDelivery(t, env, 4, true)
		arriveStop(t, env, httpSeed.stopID)
		router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "return-token", repo)
		body := map[string]any{
			"operatingTenantId": uuid.NewString(), "shipmentTenantId": uuid.NewString(),
			"expectedCurrentRevisionId": httpSeed.revisionID.String(), "sourceExecutionStopId": httpSeed.stopID.String(),
			"shipmentId": httpSeed.ship.shipmentID.String(), "cargoId": httpSeed.ship.cargoID.String(),
			"acceptedQuantity": 4, "rejectedQuantity": 0, "uom": "PALLET",
			"idempotencyKey": "http-accept", "occurredAt": when.Format(time.RFC3339Nano),
			"actorKind": domain.ActorKindOperator, "actorId": uuid.NewString(),
			"evidence": []map[string]string{{"evidenceType": "PHOTO", "source": "DRIVER", "referenceId": "photo-1"}},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/"+httpSeed.executionID.String()+"/delivery-dispositions", bytes.NewReader(raw))
		req.Header.Set("X-Internal-Service-Token", "return-token")
		req.Header.Set("X-Tenant-ID", httpSeed.operating.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("http %d %s", rec.Code, rec.Body.String())
		}
	})
}

type deliverySeed struct {
	operating   uuid.UUID
	executionID uuid.UUID
	revisionID  uuid.UUID
	stopID      uuid.UUID
	endID       uuid.UUID
	redirectID  uuid.UUID
	ship        seededShipment
}

type multiWorld struct {
	operating   uuid.UUID
	executionID uuid.UUID
	revisionID  uuid.UUID
	startID     uuid.UUID
	s1          uuid.UUID
	s2          uuid.UUID
	s3          uuid.UUID
	s4          uuid.UUID
	redirectID  uuid.UUID
	shipA       seededShipment
	shipB       seededShipment
	shipC       seededShipment
	shipD       seededShipment
}

func seedDelivery(t *testing.T, env *execEnv, pallets int, onboard bool) deliverySeed {
	t.Helper()
	operating := seedTenant(t, env, "carrier")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Carrier")
	ship := seedShipment(t, env, "ret", "IN_TRANSIT")
	prepareCargo(t, env, ship, pallets, onboard)
	redirectID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code, lat, lon) VALUES ($1,$2,'CUSTOMER_SITE','Redirect','RU',55.9,37.9)`, redirectID, operating); err != nil {
		t.Fatal(err)
	}
	version := 1
	start := uuid.New()
	cargoStop := uuid.New()
	end := uuid.New()
	cmd := domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 1, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 1, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: operating, ContextShipmentID: &ship.shipmentID, ContextShipmentTenantID: &ship.tenantID, ContextShipmentVersion: &version,
		CarrierCompanyID: carrierID, EvaluationFingerprint: "return-fingerprint",
		ExecutionSubjects: []domain.ProjectionSubject{materialSubject(ship, version)},
		Stops: []domain.ProjectionStop{
			{RoutePlanStopID: start, Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6},
			{RoutePlanStopID: cargoStop, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &ship.originID, Latitude: 55.75, Longitude: 37.62},
			{RoutePlanStopID: end, Ordinal: 2, StopRole: domain.StopRoleEnd, PointKind: domain.PointKindCanonicalLocation, LocationID: &ship.originID, Latitude: 55.8, Longitude: 37.7},
		},
		Actions: []domain.ProjectionAction{deliveryAction(cargoStop, ship, version, 0)},
	}
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	stopID := stopByRoleOrdinal(t, env, result.ExecutionID, domain.StopRoleCargo)
	endID := stopByRoleOrdinal(t, env, result.ExecutionID, domain.StopRoleEnd)
	return deliverySeed{operating: operating, executionID: result.ExecutionID, revisionID: result.RevisionID, stopID: stopID, endID: endID, redirectID: redirectID, ship: ship}
}

func seedMultiStop(t *testing.T, env *execEnv) multiWorld {
	t.Helper()
	operating := seedTenant(t, env, "multi-carrier")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Multi Carrier")
	ships := []seededShipment{
		seedShipment(t, env, "a", "IN_TRANSIT"),
		seedShipment(t, env, "b", "IN_TRANSIT"),
		seedShipment(t, env, "c", "IN_TRANSIT"),
		seedShipment(t, env, "d", "IN_TRANSIT"),
	}
	pallets := []int{10, 10, 4, 6}
	for i := range ships {
		prepareCargo(t, env, ships[i], pallets[i], true)
	}
	redirectID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code, lat, lon) VALUES ($1,$2,'CUSTOMER_SITE','Redirect D','RU',56.2,38.2)`, redirectID, operating); err != nil {
		t.Fatal(err)
	}
	version := 1
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	stops := []domain.ProjectionStop{
		{RoutePlanStopID: ids[0], Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6},
	}
	var actions []domain.ProjectionAction
	var subjects []domain.ProjectionSubject
	for i, ship := range ships {
		origin := ship.originID
		stops = append(stops, domain.ProjectionStop{
			RoutePlanStopID: ids[i+1], Ordinal: i + 1, StopRole: domain.StopRoleCargo,
			PointKind: domain.PointKindCanonicalLocation, LocationID: &origin, Latitude: 55.8 + float64(i), Longitude: 37.7,
		})
		actions = append(actions, deliveryAction(ids[i+1], ship, version, 0))
		subjects = append(subjects, materialSubject(ship, version))
	}
	cmd := domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 1, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 1, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: operating, ContextShipmentID: &ships[0].shipmentID, ContextShipmentTenantID: &ships[0].tenantID, ContextShipmentVersion: &version,
		CarrierCompanyID: carrierID, EvaluationFingerprint: "multi-return",
		ExecutionSubjects: subjects, Stops: stops, Actions: actions,
	}
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	found := stopsByOrdinal(t, env, result.ExecutionID)
	return multiWorld{
		operating: operating, executionID: result.ExecutionID, revisionID: result.RevisionID,
		startID: found[0], s1: found[1], s2: found[2], s3: found[3], s4: found[4], redirectID: redirectID,
		shipA: ships[0], shipB: ships[1], shipC: ships[2], shipD: ships[3],
	}
}

func prepareCargo(t *testing.T, env *execEnv, ship seededShipment, pallets int, onboard bool) {
	t.Helper()
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.cargoes SET pallet_count=$2 WHERE id=$1`, ship.cargoID, pallets); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.locations SET lat=55.75, lon=37.62 WHERE id=$1`, ship.originID); err != nil {
		t.Fatal(err)
	}
	if !onboard {
		return
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.shipment_cargo_execution_evidence (
			id, tenant_id, shipment_id, shipment_version, cargo_id, state, state_version, source, source_event_type, occurred_at
		) VALUES ($1,$2,$3,1,$4,'CONFIRMED_ONBOARD',1,'TMS_RETURN_TEST','test.onboard', now())
	`, uuid.New(), ship.tenantID, ship.shipmentID, ship.cargoID); err != nil {
		t.Fatal(err)
	}
}

func deliveryAction(stopID uuid.UUID, row seededShipment, version, ordinal int) domain.ProjectionAction {
	return domain.ProjectionAction{
		RoutePlanActionID: uuid.New(), RoutePlanStopID: stopID, ActionOrdinal: ordinal, ActionType: domain.ActionTypeDelivery,
		RouteSubjectType: domain.RouteSubjectShipmentCargo, RouteSubjectID: row.subjectID,
		ExecutionShipmentID: &row.shipmentID, ShipmentTenantID: &row.tenantID, ExecutionShipmentVersion: &version,
		CargoID: &row.cargoID, CargoVersion: &version, EvidenceState: "PLANNED", EvidenceStateVersion: &version,
	}
}

func recordDelivery(t *testing.T, repo *repository.TransportExecutionRepository, env *execEnv, seed deliverySeed, accepted, rejected int, reason, comment, key, actor string, uom ...string) domain.DispositionResult {
	t.Helper()
	result, err := repo.RecordDeliveryDisposition(env.ctx, recordCommand(seed, accepted, rejected, reason, comment, key, actor, firstUOM(uom)))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func recordErr(t *testing.T, repo *repository.TransportExecutionRepository, env *execEnv, seed deliverySeed, accepted, rejected int, reason, comment, key, actor string, uom ...string) error {
	t.Helper()
	_, err := repo.RecordDeliveryDisposition(env.ctx, recordCommand(seed, accepted, rejected, reason, comment, key, actor, firstUOM(uom)))
	if err == nil {
		t.Fatal("expected error")
	}
	return err
}

func recordCommand(seed deliverySeed, accepted, rejected int, reason, comment, key, actor, uom string) domain.RecordDeliveryDispositionCommand {
	return domain.RecordDeliveryDispositionCommand{
		ExecutionID: seed.executionID, ExpectedRevisionID: seed.revisionID, OperatingTenantID: seed.operating,
		SourceStopID: seed.stopID, ShipmentID: seed.ship.shipmentID, CargoID: seed.ship.cargoID,
		AcceptedQuantity: accepted, RejectedQuantity: rejected, UOM: uom, ReasonCode: reason, ReasonComment: comment,
		IdempotencyKey: key, OccurredAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		ActorKind: actor, ActorID: uuid.New(),
		Evidence: []domain.DeliveryEvidenceRef{{EvidenceType: domain.EvidencePhoto, Source: "DRIVER", ReferenceID: "photo-ref"}},
	}
}

func recordAt(t *testing.T, repo *repository.TransportExecutionRepository, env *execEnv, operating, executionID, revisionID, stopID uuid.UUID, ship seededShipment, accepted, rejected int, reason, comment, key string) domain.DispositionResult {
	t.Helper()
	result, err := repo.RecordDeliveryDisposition(env.ctx, domain.RecordDeliveryDispositionCommand{
		ExecutionID: executionID, ExpectedRevisionID: revisionID, OperatingTenantID: operating,
		SourceStopID: stopID, ShipmentID: ship.shipmentID, CargoID: ship.cargoID,
		AcceptedQuantity: accepted, RejectedQuantity: rejected, UOM: domain.UOMPallet,
		ReasonCode: reason, ReasonComment: comment, IdempotencyKey: key,
		OccurredAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), ActorKind: domain.ActorKindOperator, ActorID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func authorizeCmd(seed deliverySeed, caseID, target uuid.UUID, future []uuid.UUID, key string) domain.AuthorizeDispositionCommand {
	return authorizeCmdFor(seed.operating, seed.executionID, seed.revisionID, caseID, target, future, key)
}

func authorizeCmdFor(operating, executionID, revisionID, caseID, target uuid.UUID, future []uuid.UUID, key string) domain.AuthorizeDispositionCommand {
	return domain.AuthorizeDispositionCommand{
		CaseID: caseID, ExecutionID: executionID, ExpectedRevisionID: revisionID, OperatingTenantID: operating,
		TargetLocationID: target, FutureStopIDs: future, IdempotencyKey: key,
		OccurredAt: time.Date(2026, 10, 1, 13, 0, 0, 0, time.UTC), ActorKind: domain.ActorKindOperator, ActorID: uuid.New(),
	}
}

func completeCmd(world multiWorld, caseID, revisionID, actionID uuid.UUID, key string) domain.CompleteDispositionCommand {
	return domain.CompleteDispositionCommand{
		CaseID: caseID, ExecutionID: world.executionID, ExpectedRevisionID: revisionID, OperatingTenantID: world.operating,
		ActionID: actionID, IdempotencyKey: key, OccurredAt: time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC),
		ActorKind: domain.ActorKindSystem,
	}
}

func firstUOM(values []string) string {
	if len(values) == 0 || values[0] == "" {
		return domain.UOMPallet
	}
	return values[0]
}

func mustErr(result domain.DispositionResult, err error) error {
	if err == nil {
		return apperrors.Internal("expected an error", nil)
	}
	return err
}

func errorsAsNotFound(err error, app **apperrors.AppError) bool {
	return err != nil && asApp(err, app) && (*app).Code == apperrors.CodeNotFound
}

func asApp(err error, app **apperrors.AppError) bool {
	return errorsAs(err, app)
}

func errorsAs(err error, app **apperrors.AppError) bool {
	if err == nil {
		return false
	}
	switch typed := err.(type) {
	case *apperrors.AppError:
		*app = typed
		return true
	default:
		return false
	}
}

func assertCaseQty(t *testing.T, env *execEnv, caseID uuid.UUID, accepted, rejected, pending, returnQty, redirectQty, holdQty, resolved int) {
	t.Helper()
	var got [7]int
	if err := env.pool.QueryRow(env.ctx, `
		SELECT accepted_quantity::int, rejected_quantity::int, pending_quantity::int,
		       return_reserved_quantity::int, redirect_reserved_quantity::int, hold_reserved_quantity::int, resolved_quantity::int
		FROM transport.delivery_disposition_cases WHERE id=$1
	`, caseID).Scan(&got[0], &got[1], &got[2], &got[3], &got[4], &got[5], &got[6]); err != nil {
		t.Fatal(err)
	}
	want := [7]int{accepted, rejected, pending, returnQty, redirectQty, holdQty, resolved}
	if got != want {
		t.Fatalf("quantities got %v want %v", got, want)
	}
	if got[1] != got[2]+got[3]+got[4]+got[5]+got[6] {
		t.Fatal("rejected quantity is not conserved")
	}
}

func onboardRemaining(t *testing.T, env *execEnv, seed deliverySeed) int {
	t.Helper()
	return onboardRemainingShipment(t, env, seed.executionID, seed.ship)
}

func onboardRemainingShipment(t *testing.T, env *execEnv, executionID uuid.UUID, ship seededShipment) int {
	t.Helper()
	var pallets, accepted int
	if err := env.pool.QueryRow(env.ctx, `SELECT pallet_count FROM transport.cargoes WHERE id=$1`, ship.cargoID).Scan(&pallets); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.QueryRow(env.ctx, `SELECT COALESCE(SUM(accepted_quantity),0)::int FROM transport.delivery_attempt_facts WHERE execution_id=$1 AND cargo_id=$2`, executionID, ship.cargoID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	return pallets - accepted
}

func stopByRoleOrdinal(t *testing.T, env *execEnv, executionID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_stops WHERE execution_id=$1 AND stop_role=$2 ORDER BY ordinal LIMIT 1`, executionID, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func stopsByOrdinal(t *testing.T, env *execEnv, executionID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := env.pool.Query(env.ctx, `SELECT id FROM transport.transport_execution_stops WHERE execution_id=$1 ORDER BY ordinal`, executionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func introducedStops(t *testing.T, env *execEnv, revisionID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := env.pool.Query(env.ctx, `
		SELECT s.id
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		WHERE rs.revision_id=$1 AND rs.membership='INTRODUCED' AND s.status='PLANNED'
		ORDER BY s.ordinal
	`, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func revisionHasCargo(t *testing.T, env *execEnv, revisionID uuid.UUID, ship seededShipment) bool {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `
		SELECT COUNT(*)
		FROM transport.transport_execution_revision_actions ra
		JOIN transport.transport_execution_actions a ON a.id = ra.action_id
		WHERE ra.revision_id=$1 AND ra.membership='INTRODUCED' AND a.shipment_id=$2 AND a.cargo_id=$3
	`, revisionID, ship.shipmentID, ship.cargoID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func revisionHasLocation(t *testing.T, env *execEnv, revisionID, locationID uuid.UUID) bool {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `
		SELECT COUNT(*)
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		WHERE rs.revision_id=$1 AND rs.membership='INTRODUCED' AND s.location_id=$2
	`, revisionID, locationID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func introducedActionAt(t *testing.T, env *execEnv, revisionID, locationID, cargoID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `
		SELECT a.id
		FROM transport.transport_execution_revision_actions ra
		JOIN transport.transport_execution_actions a ON a.id = ra.action_id
		JOIN transport.transport_execution_stops s ON s.id = a.execution_stop_id
		WHERE ra.revision_id=$1 AND ra.membership='INTRODUCED' AND s.location_id=$2 AND a.cargo_id=$3
	`, revisionID, locationID, cargoID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func dispositionPayload(t *testing.T, env *execEnv, caseID uuid.UUID, eventType string) map[string]any {
	t.Helper()
	var raw []byte
	if err := env.pool.QueryRow(env.ctx, `SELECT payload FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND event_type=$2 ORDER BY created_at DESC LIMIT 1`, caseID, eventType).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

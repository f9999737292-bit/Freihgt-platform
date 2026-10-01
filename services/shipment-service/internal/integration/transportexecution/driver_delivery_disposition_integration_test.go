//go:build integration

package transportexecution

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestDriverDeliveryDisposition(t *testing.T) {
	env := startPostgres(t)
	if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000093_tms_delivery_disposition_v0_1.up.sql")); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewTransportExecutionRepository(env.pool)
	commands := repository.NewTransportExecutionCommandRepository(env.pool)
	drivers := repository.NewDriverRepository(env.pool)
	stops := service.NewDriverStopService(drivers, commands)
	stops.BindDeliveryDisposition(repo)
	router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, stops, nil, "test-token", repo)

	t.Run("DRV-RET-01 partial rejection", func(t *testing.T) {
		seed, user, driverID, actionID := assignDriverDelivery(t, env, 10)
		rec := postDriverDisposition(t, router, seed, user, actionID, 8, 2, domain.ReasonDamage, "", "drv-partial")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		body := decodeDispositionHTTP(t, rec)
		if body["status"] != domain.DispositionStatusPending || body["replayed"] != false {
			t.Fatalf("body %+v", body)
		}
		assertDriverActor(t, env, body["caseId"].(string), driverID)
	})

	t.Run("DRV-RET-02 full rejection", func(t *testing.T) {
		seed, user, driverID, actionID := assignDriverDelivery(t, env, 6)
		rec := postDriverDisposition(t, router, seed, user, actionID, 0, 6, domain.ReasonCustomerRefusal, "", "drv-full")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		body := decodeDispositionHTTP(t, rec)
		if body["acceptedQuantity"].(float64) != 0 || body["rejectedQuantity"].(float64) != 6 {
			t.Fatalf("body %+v", body)
		}
		assertDriverActor(t, env, body["caseId"].(string), driverID)
	})

	t.Run("DRV-RET-03 identity from auth", func(t *testing.T) {
		seed, user, driverID, actionID := assignDriverDelivery(t, env, 4)
		rec := postDriverDisposition(t, router, seed, user, actionID, 3, 1, domain.ReasonShortage, "", "drv-identity")
		body := decodeDispositionHTTP(t, rec)
		assertDriverActor(t, env, body["caseId"].(string), driverID)
	})

	t.Run("DRV-RET-04 body cannot spoof actor", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 4)
		path := driverDispositionPath(seed.stopID, actionID)
		body := []byte(`{"shipmentId":"` + seed.ship.shipmentID.String() + `","cargoId":"` + seed.ship.cargoID.String() + `","acceptedQuantity":3,"rejectedQuantity":1,"uom":"PALLET","reasonCode":"DAMAGE","actorId":"` + uuid.NewString() + `","actorKind":"OPERATOR"}`)
		rec := postStopRaw(t, router, seed.operating, user, path, "drv-spoof-actor", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("spoof actor %d %s", rec.Code, rec.Body.String())
		}
		if countWhere(t, env, "transport.delivery_disposition_cases", "execution_id=$1", seed.executionID) != 0 {
			t.Fatal("spoofed actor created a case")
		}
	})

	t.Run("DRV-RET-05 body cannot spoof tenant", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 4)
		path := driverDispositionPath(seed.stopID, actionID)
		body := []byte(`{"shipmentId":"` + seed.ship.shipmentID.String() + `","cargoId":"` + seed.ship.cargoID.String() + `","acceptedQuantity":3,"rejectedQuantity":1,"uom":"PALLET","reasonCode":"DAMAGE","operatingTenantId":"` + uuid.NewString() + `","shipmentTenantId":"` + uuid.NewString() + `","executionId":"` + uuid.NewString() + `"}`)
		rec := postStopRaw(t, router, seed.operating, user, path, "drv-spoof-tenant", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("spoof tenant %d %s", rec.Code, rec.Body.String())
		}
		foreign := seedTenant(t, env, "foreign-drv")
		rec = postDriverDisposition(t, router, deliverySeed{operating: foreign, stopID: seed.stopID, ship: seed.ship}, user, actionID, 3, 1, domain.ReasonDamage, "", "drv-foreign-header")
		if rec.Code == http.StatusOK {
			t.Fatalf("foreign tenant header accepted %s", rec.Body.String())
		}
	})

	t.Run("DRV-RET-06 other execution denied", func(t *testing.T) {
		own, user, _, _ := assignDriverDelivery(t, env, 4)
		other, otherAction := peerDriverDelivery(t, env, own, 4)
		rec := postDriverDisposition(t, router, other, user, otherAction, 3, 1, domain.ReasonDamage, "", "drv-other-exec")
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), domain.ReasonWrongDriver) {
			t.Fatalf("other execution %d %s", rec.Code, rec.Body.String())
		}
		if countWhere(t, env, "transport.delivery_disposition_cases", "execution_id=$1", other.executionID) != 0 {
			t.Fatal("other execution recorded a case")
		}
		if countWhere(t, env, "transport.delivery_disposition_cases", "execution_id=$1", own.executionID) != 0 {
			t.Fatal("own execution was touched")
		}
	})

	t.Run("DRV-RET-07 stop not on assigned execution", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 4)
		rec := postDriverDisposition(t, router, deliverySeed{operating: seed.operating, stopID: uuid.New(), ship: seed.ship}, user, actionID, 3, 1, domain.ReasonDamage, "", "drv-missing-stop")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("missing stop %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DRV-RET-08 wrong cargo and action", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 4)
		_, _, _, otherAction := assignDriverDelivery(t, env, 4)
		rec := postDriverDisposition(t, router, seed, user, otherAction, 3, 1, domain.ReasonDamage, "", "drv-wrong-action")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("wrong action %d %s", rec.Code, rec.Body.String())
		}
		path := driverDispositionPath(seed.stopID, actionID)
		body := []byte(`{"shipmentId":"` + seed.ship.shipmentID.String() + `","cargoId":"` + uuid.NewString() + `","acceptedQuantity":3,"rejectedQuantity":1,"uom":"PALLET","reasonCode":"DAMAGE"}`)
		rec = postStopRaw(t, router, seed.operating, user, path, "drv-wrong-cargo", body)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), domain.ReasonNotParticipant) {
			t.Fatalf("wrong cargo %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DRV-RET-09 rejected above onboard", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 4)
		rec := postDriverDisposition(t, router, seed, user, actionID, 0, 5, domain.ReasonDamage, "", "drv-over")
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), domain.ReasonQuantityExceeded) {
			t.Fatalf("over onboard %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DRV-RET-10 idempotent replay", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 8)
		first := postDriverDisposition(t, router, seed, user, actionID, 6, 2, domain.ReasonPackagingDamage, "", "drv-replay")
		second := postDriverDisposition(t, router, seed, user, actionID, 6, 2, domain.ReasonPackagingDamage, "", "drv-replay")
		if first.Code != http.StatusOK || second.Code != http.StatusOK {
			t.Fatalf("replay %d %s / %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
		}
		a := decodeDispositionHTTP(t, first)
		b := decodeDispositionHTTP(t, second)
		if a["caseId"] != b["caseId"] || b["replayed"] != true {
			t.Fatalf("replay bodies %+v %+v", a, b)
		}
		if countWhere(t, env, "transport.delivery_disposition_cases", "execution_id=$1", seed.executionID) != 1 {
			t.Fatal("replay duplicated the case")
		}
	})

	t.Run("DRV-RET-11 idempotency conflict", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 8)
		ok := postDriverDisposition(t, router, seed, user, actionID, 7, 1, domain.ReasonMisSort, "", "drv-conflict")
		if ok.Code != http.StatusOK {
			t.Fatalf("first %d %s", ok.Code, ok.Body.String())
		}
		conflict := postDriverDisposition(t, router, seed, user, actionID, 6, 2, domain.ReasonMisSort, "", "drv-conflict")
		if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), domain.ReasonCommandBodyConflict) {
			t.Fatalf("conflict %d %s", conflict.Code, conflict.Body.String())
		}
	})

	t.Run("DRV-RET-12 driver cannot authorize return", func(t *testing.T) {
		seed, user, _, _ := assignDriverDelivery(t, env, 4)
		rec := postStopRaw(t, router, seed.operating, user, "/v1/driver/me/delivery-dispositions/"+uuid.NewString()+"/authorize-return", "drv-return", []byte(`{}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("return route %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DRV-RET-13 driver cannot authorize redirect", func(t *testing.T) {
		seed, user, _, actionID := assignDriverDelivery(t, env, 4)
		rec := postStopRaw(t, router, seed.operating, user, "/v1/driver/me/stops/"+seed.stopID.String()+"/actions/"+actionID.String()+"/authorize-redirect", "drv-redirect", []byte(`{}`))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("redirect route %d %s", rec.Code, rec.Body.String())
		}
	})
}

func assignDriverDelivery(t *testing.T, env *execEnv, pallets int) (deliverySeed, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	seed := seedDelivery(t, env, pallets, true)
	arriveStop(t, env, seed.stopID)
	driverID := uuid.New()
	userID := insertDriver(t, env, driverID, seed.operating, carrierOf(t, env, seed.executionID), uuid.New())
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_executions SET driver_id=$2 WHERE id=$1`, seed.executionID, driverID); err != nil {
		t.Fatal(err)
	}
	var actionID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `
		SELECT id FROM transport.transport_execution_actions
		WHERE execution_stop_id=$1 AND action_type='DELIVERY'
	`, seed.stopID).Scan(&actionID); err != nil {
		t.Fatal(err)
	}
	return seed, userID, driverID, actionID
}

func peerDriverDelivery(t *testing.T, env *execEnv, own deliverySeed, pallets int) (deliverySeed, uuid.UUID) {
	t.Helper()
	ship := seedShipment(t, env, "peer", "IN_TRANSIT")
	prepareCargo(t, env, ship, pallets, true)
	version := 1
	start := uuid.New()
	cargoStop := uuid.New()
	end := uuid.New()
	carrierID := carrierOf(t, env, own.executionID)
	otherDriver := uuid.New()
	cmd := domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 1, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 1, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: own.operating, ContextShipmentID: &ship.shipmentID, ContextShipmentTenantID: &ship.tenantID, ContextShipmentVersion: &version,
		CarrierCompanyID: carrierID, DriverID: &otherDriver, EvaluationFingerprint: "peer-driver",
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
	arriveStop(t, env, stopID)
	insertDriver(t, env, otherDriver, own.operating, carrierID, uuid.New())
	var actionID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `
		SELECT id FROM transport.transport_execution_actions
		WHERE execution_stop_id=$1 AND action_type='DELIVERY'
	`, stopID).Scan(&actionID); err != nil {
		t.Fatal(err)
	}
	return deliverySeed{operating: own.operating, executionID: result.ExecutionID, revisionID: result.RevisionID, stopID: stopID, ship: ship}, actionID
}

func driverDispositionPath(stopID, actionID uuid.UUID) string {
	return "/v1/driver/me/stops/" + stopID.String() + "/actions/" + actionID.String() + "/delivery-disposition"
}

func postDriverDisposition(t *testing.T, router http.Handler, seed deliverySeed, userID, actionID uuid.UUID, accepted, rejected int, reason, comment, key string) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]any{
		"shipmentId": seed.ship.shipmentID.String(), "cargoId": seed.ship.cargoID.String(),
		"acceptedQuantity": accepted, "rejectedQuantity": rejected, "uom": domain.UOMPallet,
		"reasonCode": reason, "occurredAt": time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"evidence": []map[string]string{{"evidenceType": domain.EvidencePhoto, "source": "DRIVER", "referenceId": "photo-" + key}},
	}
	if comment != "" {
		payload["reasonComment"] = comment
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return postStopRaw(t, router, seed.operating, userID, driverDispositionPath(seed.stopID, actionID), key, raw)
}

func decodeDispositionHTTP(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func assertDriverActor(t *testing.T, env *execEnv, caseID string, driverID uuid.UUID) {
	t.Helper()
	var kind string
	var actor uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `
		SELECT created_by_actor_kind, created_by_actor_id FROM transport.delivery_disposition_cases WHERE id=$1
	`, caseID).Scan(&kind, &actor); err != nil {
		t.Fatal(err)
	}
	if kind != domain.ActorKindDriver || actor != driverID {
		t.Fatalf("actor %s %s want DRIVER %s", kind, actor, driverID)
	}
}

//go:build integration

package transportexecution

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type actionSpec struct {
	ship int
	kind string
}

type execFixture struct {
	operating uuid.UUID
	driver    uuid.UUID
	execution uuid.UUID
	revision  uuid.UUID
	ships     []seededShipment
	stops     []uuid.UUID
	actions   [][]uuid.UUID
}

func TestTransportExecutionCommands(t *testing.T) {
	env := startPostgres(t)
	commandTestCtx = env.ctx
	commands := service.NewTransportExecutionCommandService(repository.NewTransportExecutionCommandRepository(env.pool))
	when := time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)

	t.Run("ARRIVE_CURRENT_STOP", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		skipStart(t, env, commands, fx, when)
		res := mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "arrive-current", when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if res.StopStatus != domain.StopStatusArrived {
			t.Fatalf("status %s", res.StopStatus)
		}
		if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInPickup {
			t.Fatalf("shipment %s", shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID))
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopArrived) != 1 {
			t.Fatal("arrived event missing")
		}
	})

	t.Run("ARRIVE_OUT_OF_ORDER_DENIED", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypePickup}},
		})
		err := execCmd(commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "ooo-driver", when, stopVersion(t, env, fx.stops[1]), ""))
		if reason(err) != domain.ReasonStopNotCurrent {
			t.Fatalf("reason %s err %v", reason(err), err)
		}
		skipStart(t, env, commands, fx, when)
		err = execCmd(commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "ooo-later", when.Add(time.Minute), stopVersion(t, env, fx.stops[2]), "ROUTE_BLOCKED"))
		if reason(err) != domain.ReasonStopNotCurrent {
			t.Fatalf("later reason %s", reason(err))
		}
		if stopStatusOf(t, env, fx.stops[2]) != domain.StopStatusPlanned {
			t.Fatal("later stop moved")
		}
	})

	t.Run("SEQUENCE_OVERRIDE_AUDITED", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypePickup}},
		})
		skipStart(t, env, commands, fx, when)
		op := fx.operatorCmd(domain.CommandArriveStop, 2, -1, "override-arrive", when.Add(2*time.Minute), stopVersion(t, env, fx.stops[2]), "ROUTE_BLOCKED")
		if _, err := commands.Execute(env.ctx, op); err != nil {
			t.Fatal(err)
		}
		var ordinals []int32
		if err := env.pool.QueryRow(env.ctx, `
			SELECT skipped_ordinals FROM transport.transport_execution_command_audit
			WHERE execution_id=$1 AND reason_code='ROUTE_BLOCKED'
			ORDER BY created_at DESC LIMIT 1
		`, fx.execution).Scan(&ordinals); err != nil {
			t.Fatal(err)
		}
		if len(ordinals) == 0 {
			t.Fatal("skipped ordinals were not audited")
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopSequenceOverridden) < 1 {
			t.Fatal("sequence override event missing")
		}
	})

	t.Run("START_SERVICE", func(t *testing.T) {
		fx := preparePickup(t, env, commands, when)
		res := mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "start-1", when.Add(3*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if res.StopStatus != domain.StopStatusServiceStarted {
			t.Fatalf("status %s", res.StopStatus)
		}
		again := mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "start-again", when.Add(4*time.Minute), 1, ""))
		if again.StopStatus != domain.StopStatusServiceStarted {
			t.Fatal("second start changed the stop")
		}
	})

	t.Run("COMPLETE_WITH_PENDING_ACTION_DENIED", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		err := execCmd(commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "complete-pending", when.Add(5*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if reason(err) != domain.ReasonActionPending {
			t.Fatalf("reason %s", reason(err))
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusServiceStarted {
			t.Fatal("stop moved while an action was pending")
		}
	})

	t.Run("COMPLETE_ALL_ACTIONS_RESOLVED", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "confirm-for-complete", when.Add(6*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		res := mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "complete-ok", when.Add(7*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if res.StopStatus != domain.StopStatusCompleted {
			t.Fatalf("status %s", res.StopStatus)
		}
		if stopReason(t, env, fx.stops[1]) != "" {
			t.Fatalf("reason %s", stopReason(t, env, fx.stops[1]))
		}
	})

	t.Run("PARTIAL_STOP_COMPLETION", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked, domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{
			{0, domain.ActionTypePickup},
			{1, domain.ActionTypePickup},
		}})
		mustExec(t, commands, fx.driverCmd(domain.CommandFailAction, 1, 1, "fail-b", when.Add(8*time.Minute), stopVersion(t, env, fx.stops[1]), "CARGO_ISSUE"))
		mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "confirm-a", when.Add(9*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		res := mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "complete-partial", when.Add(10*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if res.StopStatus != domain.StopStatusCompleted || stopReason(t, env, fx.stops[1]) != domain.StopStatusReasonPartial {
			t.Fatalf("status %s reason %s", res.StopStatus, stopReason(t, env, fx.stops[1]))
		}
		if shipmentStatus(t, env, fx.ships[1].shipmentID, fx.ships[1].tenantID) == domain.ShipmentStatusLoaded {
			t.Fatal("failed participant was loaded")
		}
		if evidenceCount(t, env, fx.ships[1].shipmentID, domain.CargoEvidenceConfirmedOnboard) != 0 {
			t.Fatal("failed action wrote completion evidence")
		}
	})

	t.Run("SKIP_OPERATOR_ONLY", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		err := execCmd(commands, fx.driverCmd(domain.CommandSkipStop, 0, -1, "driver-skip", when, stopVersion(t, env, fx.stops[0]), "ROUTE_BLOCKED"))
		if reason(err) != domain.ReasonActorDenied {
			t.Fatalf("reason %s", reason(err))
		}
		skipStart(t, env, commands, fx, when)
		err = execCmd(commands, fx.operatorCmd(domain.CommandSkipStop, 1, -1, "skip-arrived-prep", when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if reason(err) != domain.ReasonReasonRequired {
			t.Fatalf("missing reason %s", reason(err))
		}
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "arrive-before-skip", when.Add(2*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		err = execCmd(commands, fx.operatorCmd(domain.CommandSkipStop, 1, -1, "skip-arrived", when.Add(3*time.Minute), stopVersion(t, env, fx.stops[1]), "ROUTE_BLOCKED"))
		if reason(err) != domain.ReasonStopTransitionDenied {
			t.Fatalf("arrived skip reason %s", reason(err))
		}
	})

	t.Run("CANCEL_REMAINING", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		skipStart(t, env, commands, fx, when)
		walkPickup(t, env, commands, fx, 1, when.Add(20*time.Minute))
		mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "complete-before-cancel", when.Add(30*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "arrive-middle", when.Add(31*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		err := execCmd(commands, fx.driverCmd(domain.CommandCancelRemainingStops, -1, -1, "driver-cancel-rest", when.Add(32*time.Minute), 1, "OTHER"))
		if reason(err) != domain.ReasonActorDenied {
			t.Fatalf("driver cancel reason %s", reason(err))
		}
		sys := fx.operatorCmd(domain.CommandCancelRemainingStops, -1, -1, "cancel-rest", when.Add(33*time.Minute), 1, "OTHER")
		sys.ActorKind = domain.ActorKindSystem
		sys.ActorID = uuid.Nil
		if _, err := commands.Execute(env.ctx, sys); err != nil {
			t.Fatal(err)
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusCompleted {
			t.Fatal("completed stop was cancelled")
		}
		if stopStatusOf(t, env, fx.stops[2]) != domain.StopStatusArrived {
			t.Fatal("arrived stop was cancelled")
		}
		if stopStatusOf(t, env, fx.stops[3]) != domain.StopStatusCancelled {
			t.Fatalf("future stop %s", stopStatusOf(t, env, fx.stops[3]))
		}
	})

	t.Run("CANCEL_IN_SERVICE_OPERATOR_ONLY", func(t *testing.T) {
		fx := preparePickup(t, env, commands, when)
		err := execCmd(commands, fx.driverCmd(domain.CommandCancelInServiceStop, 1, -1, "driver-cancel-stop", when.Add(40*time.Minute), stopVersion(t, env, fx.stops[1]), "OTHER"))
		if reason(err) != domain.ReasonActorDenied {
			t.Fatalf("reason %s", reason(err))
		}
		res := mustExec(t, commands, fx.operatorCmd(domain.CommandCancelInServiceStop, 1, -1, "op-cancel-stop", when.Add(41*time.Minute), stopVersion(t, env, fx.stops[1]), "CUSTOMER_UNAVAILABLE"))
		if res.StopStatus != domain.StopStatusCancelled {
			t.Fatalf("status %s", res.StopStatus)
		}
		err = execCmd(commands, fx.operatorCmd(domain.CommandArriveStop, 1, -1, "reopen-cancelled", when.Add(42*time.Minute), 1, ""))
		if reason(err) != domain.ReasonTerminalStop {
			t.Fatalf("reopen reason %s", reason(err))
		}
	})

	t.Run("PICKUP_WRITES_CONFIRMED_ONBOARD", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		res := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "pickup-evidence", when.Add(50*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if res.EvidenceID == nil || evidenceState(t, env, *res.EvidenceID) != domain.CargoEvidenceConfirmedOnboard {
			t.Fatal("confirmed onboard evidence missing")
		}
		var tenant uuid.UUID
		if err := env.pool.QueryRow(env.ctx, `SELECT tenant_id FROM transport.shipment_cargo_execution_evidence WHERE id=$1`, *res.EvidenceID).Scan(&tenant); err != nil {
			t.Fatal(err)
		}
		if tenant != fx.ships[0].tenantID || tenant == fx.operating {
			t.Fatalf("evidence tenant %s operating %s shipper %s", tenant, fx.operating, fx.ships[0].tenantID)
		}
	})

	t.Run("DELIVERY_WRITES_UNLOADED", func(t *testing.T) {
		fx := reachUnloading(t, env, commands, when)
		res := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "delivery-evidence", when.Add(80*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if res.EvidenceID == nil || evidenceState(t, env, *res.EvidenceID) != domain.CargoEvidenceUnloaded {
			t.Fatal("unloaded evidence missing")
		}
	})

	t.Run("DUPLICATE_PICKUP_NO_DUPLICATE_EVIDENCE", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		cmd := fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "pickup-replay", when.Add(90*time.Minute), stopVersion(t, env, fx.stops[1]), "")
		first := mustExec(t, commands, cmd)
		second := mustExec(t, commands, cmd)
		if !second.Replayed || first.EvidenceID == nil || second.EvidenceID == nil || *first.EvidenceID != *second.EvidenceID {
			t.Fatal("pickup replay did not return the original evidence")
		}
		if evidenceCount(t, env, fx.ships[0].shipmentID, domain.CargoEvidenceConfirmedOnboard) != 1 {
			t.Fatal("duplicate pickup evidence row")
		}
		cmd.OccurredAt = cmd.OccurredAt.Add(time.Hour)
		err := execCmd(commands, cmd)
		if reason(err) != domain.ReasonCommandBodyConflict {
			t.Fatalf("body conflict reason %s", reason(err))
		}
	})

	t.Run("DUPLICATE_DELIVERY_NO_DUPLICATE_EVIDENCE", func(t *testing.T) {
		fx := reachUnloading(t, env, commands, when.Add(100*time.Minute))
		cmd := fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "delivery-replay", when.Add(110*time.Minute), stopVersion(t, env, fx.stops[2]), "")
		first := mustExec(t, commands, cmd)
		second := mustExec(t, commands, cmd)
		if !second.Replayed || *first.EvidenceID != *second.EvidenceID {
			t.Fatal("delivery replay did not return the original evidence")
		}
		if evidenceCount(t, env, fx.ships[0].shipmentID, domain.CargoEvidenceUnloaded) != 1 {
			t.Fatal("duplicate delivery evidence row")
		}
	})

	t.Run("CONCURRENT_PICKUP_CONFIRM_IDEMPOTENCY", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		version := stopVersion(t, env, fx.stops[1])
		var wg sync.WaitGroup
		results := make([]domain.ExecutionCommandResult, 2)
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i], errs[i] = commands.Execute(env.ctx, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, fmt.Sprintf("concurrent-%d", i), when.Add(120*time.Minute), version, ""))
			}(i)
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("errs %v %v", errs[0], errs[1])
		}
		if results[0].EvidenceID == nil || results[1].EvidenceID == nil || *results[0].EvidenceID != *results[1].EvidenceID {
			t.Fatal("concurrent confirms diverged")
		}
		if evidenceCount(t, env, fx.ships[0].shipmentID, domain.CargoEvidenceConfirmedOnboard) != 1 {
			t.Fatal("concurrent confirms wrote two evidence rows")
		}
	})

	t.Run("FIRST_PICKUP_STATUS_ALIGNMENT", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInPickup {
			t.Fatal("arrival did not enter IN_PICKUP")
		}
		res := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "first-pickup", when.Add(130*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if res.ShipmentStatus != domain.ShipmentStatusLoaded {
			t.Fatalf("status %s", res.ShipmentStatus)
		}
	})

	t.Run("LATER_PICKUP_NO_STATUS_RESET", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypePickup}},
		})
		skipStart(t, env, commands, fx, when)
		walkPickup(t, env, commands, fx, 1, when.Add(140*time.Minute))
		depart := fx.driverCmd(domain.CommandDepartedPickup, -1, -1, "depart-later", when.Add(150*time.Minute), 1, "")
		depart.ShipmentID = fx.ships[0].shipmentID
		depart.ShipmentTenantID = fx.ships[0].tenantID
		res := mustExec(t, commands, depart)
		if res.ShipmentStatus != domain.ShipmentStatusInTransit {
			t.Fatalf("depart status %s", res.ShipmentStatus)
		}
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "arrive-later-pickup", when.Add(151*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "start-later-pickup", when.Add(152*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		res = mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 2, 0, "confirm-later-pickup", when.Add(153*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if res.ShipmentStatus != domain.ShipmentStatusInTransit {
			t.Fatalf("later pickup reset status to %s", res.ShipmentStatus)
		}
		if evidenceCount(t, env, fx.ships[0].shipmentID, domain.CargoEvidenceConfirmedOnboard) != 2 {
			t.Fatal("later pickup did not append evidence")
		}
	})

	t.Run("PARTIAL_DELIVERY_NO_EARLY_DELIVERED", func(t *testing.T) {
		fx := deliveryRoute(t, env, commands, when.Add(160*time.Minute))
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "arrive-partial", when.Add(170*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "start-partial", when.Add(171*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		res := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "confirm-partial", when.Add(172*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if res.ShipmentStatus != domain.ShipmentStatusInTransit {
			t.Fatalf("partial delivery status %s", res.ShipmentStatus)
		}
		if evidenceCount(t, env, fx.ships[0].shipmentID, domain.CargoEvidenceUnloaded) != 1 {
			t.Fatal("partial delivery did not write UNLOADED")
		}
	})

	t.Run("FINAL_DELIVERY_ALIGNMENT", func(t *testing.T) {
		fx := reachUnloading(t, env, commands, when.Add(180*time.Minute))
		if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusUnloading {
			t.Fatal("final service did not enter UNLOADING")
		}
		res := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "final-delivery", when.Add(190*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if res.ShipmentStatus != domain.ShipmentStatusDelivered {
			t.Fatalf("status %s", res.ShipmentStatus)
		}
	})

	t.Run("MULTI_ACTION_FINAL_DELIVERY", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}, {0, domain.ActionTypeDelivery}},
		})
		skipStart(t, env, commands, fx, when.Add(200*time.Minute))
		walkPickup(t, env, commands, fx, 1, when.Add(201*time.Minute))
		depart := fx.driverCmd(domain.CommandDepartedPickup, -1, -1, "depart-multi", when.Add(210*time.Minute), 1, "")
		depart.ShipmentID = fx.ships[0].shipmentID
		depart.ShipmentTenantID = fx.ships[0].tenantID
		mustExec(t, commands, depart)
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "arrive-multi", when.Add(211*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusArrivedAtConsignee {
			t.Fatal("final arrival did not enter ARRIVED_AT_CONSIGNEE")
		}
		mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "start-multi", when.Add(212*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		first := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "multi-first", when.Add(213*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if first.ShipmentStatus != domain.ShipmentStatusUnloading {
			t.Fatalf("first final action delivered early: %s", first.ShipmentStatus)
		}
		second := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 1, "multi-second", when.Add(214*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if second.ShipmentStatus != domain.ShipmentStatusDelivered {
			t.Fatalf("last action status %s", second.ShipmentStatus)
		}
	})

	t.Run("CROSS_SHIPPER_PARTICIPANT_ISOLATION", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked, domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{
			{0, domain.ActionTypePickup},
			{1, domain.ActionTypePickup},
		}})
		before := shipmentStatus(t, env, fx.ships[1].shipmentID, fx.ships[1].tenantID)
		mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "isolate-a", when.Add(220*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if shipmentStatus(t, env, fx.ships[1].shipmentID, fx.ships[1].tenantID) != before {
			t.Fatal("shipper B changed when shipper A was confirmed")
		}
		if evidenceCount(t, env, fx.ships[1].shipmentID, domain.CargoEvidenceConfirmedOnboard) != 0 {
			t.Fatal("shipper B received shipper A evidence")
		}
	})

	t.Run("COMPLETED_STOP_IMMUTABILITY", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "imm-stop-confirm", when.Add(230*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "imm-stop-complete", when.Add(231*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		_, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_stops SET status='PLANNED' WHERE id=$1`, fx.stops[1])
		if !constraintHas(err, "TERMINAL_STOP_IMMUTABLE") {
			t.Fatalf("direct stop mutation err %v", err)
		}
	})

	t.Run("COMPLETED_ACTION_IMMUTABILITY", func(t *testing.T) {
		fx := prepareService(t, env, commands, when, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "imm-action", when.Add(240*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		_, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_actions SET status='FAILED' WHERE id=$1`, fx.actions[1][0])
		if !constraintHas(err, "COMPLETED_ACTION_IMMUTABLE") && !constraintHas(err, "TERMINAL_ACTION_IMMUTABLE") {
			t.Fatalf("direct action mutation err %v", err)
		}
	})

	t.Run("OUTBOX_ATOMICITY", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		skipStart(t, env, commands, fx, when)
		before := countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1", fx.execution)
		restore := repository.SetExecutionOutboxInsertForTest(func(context.Context, pgx.Tx, domain.ShipmentOutboxEvent) error {
			return errors.New("outbox unavailable")
		})
		err := execCmd(commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "outbox-fail", when.Add(250*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		restore()
		if err == nil {
			t.Fatal("outbox failure was ignored")
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusPlanned {
			t.Fatal("stop committed after outbox failure")
		}
		if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusPickupSlotBooked {
			t.Fatal("shipment committed after outbox failure")
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1", fx.execution) != before {
			t.Fatal("outbox row committed after failure")
		}
		if countWhere(t, env, "transport.transport_execution_commands", "execution_id=$1 AND idempotency_key=$2", fx.execution, "outbox-fail") != 0 {
			t.Fatal("command row committed after outbox failure")
		}
	})

	t.Run("SINGLE_LEG_REGRESSION", func(t *testing.T) {
		target, ok, informational := domain.MapDriverEventToTargetStatus("DEPARTED_PICKUP")
		if !ok || informational || target != domain.ShipmentStatusInTransit {
			t.Fatal("single-leg departure map changed")
		}
		fx := preparePickup(t, env, commands, when)
		idle := seedShipment(t, env, "idle", domain.ShipmentStatusLoaded)
		cmd := fx.driverCmd(domain.CommandDepartedPickup, -1, -1, "depart-idle", when.Add(260*time.Minute), 1, "")
		cmd.ShipmentID = idle.shipmentID
		cmd.ShipmentTenantID = idle.tenantID
		err := execCmd(commands, cmd)
		if reason(err) != domain.ReasonNotParticipant {
			t.Fatalf("reason %s", reason(err))
		}
		if shipmentStatus(t, env, idle.shipmentID, idle.tenantID) != domain.ShipmentStatusLoaded {
			t.Fatal("non-participant shipment changed")
		}
	})

	t.Run("POD_GATE_REGRESSION", func(t *testing.T) {
		fx := deliveryRoute(t, env, commands, when.Add(270*time.Minute))
		if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInTransit {
			t.Fatal("route was not in transit before the non-final delivery")
		}
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "pod-arrive-partial", when.Add(280*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "pod-start-partial", when.Add(281*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		res := mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "pod-partial", when.Add(282*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if res.ShipmentStatus == domain.ShipmentStatusDelivered {
			t.Fatal("non-final delivery jumped to DELIVERED")
		}
		mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, 2, -1, "pod-complete-partial", when.Add(283*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 3, -1, "pod-arrive-final", when.Add(284*time.Minute), stopVersion(t, env, fx.stops[3]), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 3, -1, "pod-start-final", when.Add(285*time.Minute), stopVersion(t, env, fx.stops[3]), ""))
		res = mustExec(t, commands, fx.driverCmd(domain.CommandConfirmDelivery, 3, 0, "pod-final", when.Add(286*time.Minute), stopVersion(t, env, fx.stops[3]), ""))
		if res.ShipmentStatus != domain.ShipmentStatusDelivered {
			t.Fatalf("final status %s", res.ShipmentStatus)
		}
		if podCount(t, env, fx.ships[0].shipmentID) != 0 {
			t.Fatal("command created a POD document")
		}
	})

	t.Run("SECURITY_DENIALS", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		unassigned := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_executions SET driver_id=NULL WHERE id=$1`, unassigned.execution); err != nil {
			t.Fatal(err)
		}
		err := execCmd(commands, unassigned.driverCmd(domain.CommandArriveStop, 0, -1, "no-driver", when, 1, ""))
		if reason(err) != domain.ReasonUnassignedDriver {
			t.Fatalf("unassigned reason %s", reason(err))
		}
		wrong := fx.driverCmd(domain.CommandArriveStop, 0, -1, "wrong-driver", when, 1, "")
		wrong.ActorID = uuid.New()
		err = execCmd(commands, wrong)
		if reason(err) != domain.ReasonWrongDriver {
			t.Fatalf("wrong driver reason %s", reason(err))
		}
		foreign := fx.operatorCmd(domain.CommandSkipStop, 0, -1, "foreign-op", when, 1, "OTHER")
		foreign.OperatingTenantID = uuid.New()
		foreign.ActorID = uuid.New()
		err = execCmd(commands, foreign)
		if reason(err) != domain.ReasonTenantDenied {
			t.Fatalf("tenant reason %s", reason(err))
		}
		stale := fx.driverCmd(domain.CommandArriveStop, 0, -1, "stale-rev", when, 1, "")
		stale.RevisionID = uuid.New()
		err = execCmd(commands, stale)
		if reason(err) != domain.ReasonStaleRevision {
			t.Fatalf("stale reason %s", reason(err))
		}
	})

	t.Run("MIGRATION_DOWN_UP", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		down := filepath.Join(env.migrations, "000089_tms_transport_execution_commands_v0_1b.down.sql")
		up := filepath.Join(env.migrations, "000089_tms_transport_execution_commands_v0_1b.up.sql")
		if err := execSQLFile(env.ctx, env.pool, down); err != nil {
			t.Fatal(err)
		}
		if relationExists(t, env, "transport", "transport_execution_commands") {
			t.Fatal("down left the command table")
		}
		if !relationExists(t, env, "transport", "transport_execution_stops") {
			t.Fatal("down removed a 0.1A table")
		}
		if _, err := env.pool.Exec(env.ctx, `SELECT id FROM transport.transport_executions WHERE id=$1`, fx.execution); err != nil {
			t.Fatal(err)
		}
		if err := execSQLFile(env.ctx, env.pool, up); err != nil {
			t.Fatal(err)
		}
		if !relationExists(t, env, "transport", "transport_execution_commands") {
			t.Fatal("up did not restore the command table")
		}
	})
}

func newFixture(t *testing.T, env *execEnv, statuses []string, cargo [][]actionSpec) execFixture {
	t.Helper()
	operating := seedTenant(t, env, "op")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Command Carrier")
	driverID := uuid.New()
	version := 1
	ships := make([]seededShipment, len(statuses))
	subjects := make([]domain.ProjectionSubject, len(statuses))
	for i, status := range statuses {
		ships[i] = seedShipment(t, env, fmt.Sprintf("cmd-%d", i), status)
		subjects[i] = materialSubject(ships[i], version)
	}
	stops := []domain.ProjectionStop{{
		RoutePlanStopID: uuid.New(), Ordinal: 0, StopRole: domain.StopRoleStart,
		PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6,
	}}
	var planned []domain.ProjectionAction
	for _, group := range cargo {
		stopID := uuid.New()
		loc := ships[group[0].ship].originID
		duration := 600
		stops = append(stops, domain.ProjectionStop{
			RoutePlanStopID: stopID, Ordinal: len(stops), StopRole: domain.StopRoleCargo,
			PointKind: domain.PointKindCanonicalLocation, LocationID: &loc,
			Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration,
		})
		for i, spec := range group {
			action := materialAction(stopID, ships[spec.ship], version, i)
			action.ActionType = spec.kind
			planned = append(planned, action)
		}
	}
	endLoc := ships[0].originID
	stops = append(stops, domain.ProjectionStop{
		RoutePlanStopID: uuid.New(), Ordinal: len(stops), StopRole: domain.StopRoleEnd,
		PointKind: domain.PointKindCanonicalLocation, LocationID: &endLoc, Latitude: 56, Longitude: 38,
	})
	cmd := domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 2, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 4, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: operating, ContextShipmentID: &ships[0].shipmentID, ContextShipmentTenantID: &ships[0].tenantID,
		ContextShipmentVersion: &version, CarrierCompanyID: carrierID, DriverID: &driverID,
		EvaluationFingerprint: "command-route", ExecutionSubjects: subjects, Stops: stops, Actions: planned,
	}
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	fx := execFixture{operating: operating, driver: driverID, execution: result.ExecutionID, revision: result.RevisionID, ships: ships}
	rows, err := env.pool.Query(env.ctx, `SELECT id FROM transport.transport_execution_stops WHERE execution_id=$1 ORDER BY ordinal`, result.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		fx.stops = append(fx.stops, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for _, id := range fx.stops {
		actionRows, err := env.pool.Query(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 ORDER BY ordinal`, id)
		if err != nil {
			t.Fatal(err)
		}
		var ids []uuid.UUID
		for actionRows.Next() {
			var actionID uuid.UUID
			if err := actionRows.Scan(&actionID); err != nil {
				actionRows.Close()
				t.Fatal(err)
			}
			ids = append(ids, actionID)
		}
		if err := actionRows.Err(); err != nil {
			actionRows.Close()
			t.Fatal(err)
		}
		actionRows.Close()
		fx.actions = append(fx.actions, ids)
	}
	return fx
}

func (fx execFixture) driverCmd(name string, stopIndex, actionIndex int, key string, occurred time.Time, version int, reasonCode string) domain.ExecutionCommand {
	return fx.baseCmd(name, stopIndex, actionIndex, key, occurred, version, reasonCode, domain.ActorKindDriver, fx.driver)
}

func (fx execFixture) operatorCmd(name string, stopIndex, actionIndex int, key string, occurred time.Time, version int, reasonCode string) domain.ExecutionCommand {
	return fx.baseCmd(name, stopIndex, actionIndex, key, occurred, version, reasonCode, domain.ActorKindOperator, uuid.New())
}

func (fx execFixture) baseCmd(name string, stopIndex, actionIndex int, key string, occurred time.Time, version int, reasonCode, actor string, actorID uuid.UUID) domain.ExecutionCommand {
	cmd := domain.ExecutionCommand{
		Name: name, ExecutionID: fx.execution, RevisionID: fx.revision, IdempotencyKey: key,
		OccurredAt: occurred, ExpectedStopVersion: version, ReasonCode: reasonCode,
		ActorKind: actor, ActorID: actorID, OperatingTenantID: fx.operating,
	}
	if stopIndex >= 0 {
		cmd.StopID = fx.stops[stopIndex]
	}
	if actionIndex >= 0 {
		cmd.ActionID = fx.actions[stopIndex][actionIndex]
	}
	return cmd
}

func skipStart(t *testing.T, env *execEnv, commands *service.TransportExecutionCommandService, fx execFixture, when time.Time) {
	t.Helper()
	mustExec(t, commands, fx.operatorCmd(domain.CommandSkipStop, 0, -1, "skip-start-"+fx.execution.String(), when, stopVersion(t, env, fx.stops[0]), "ROUTE_BLOCKED"))
}

func preparePickup(t *testing.T, env *execEnv, commands *service.TransportExecutionCommandService, when time.Time) execFixture {
	t.Helper()
	fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
	skipStart(t, env, commands, fx, when)
	mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "prep-arrive-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
	return fx
}

func prepareService(t *testing.T, env *execEnv, commands *service.TransportExecutionCommandService, when time.Time, statuses []string, cargo [][]actionSpec) execFixture {
	t.Helper()
	fx := newFixture(t, env, statuses, cargo)
	skipStart(t, env, commands, fx, when)
	mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "prep-svc-arrive-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
	mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "prep-svc-start-"+fx.execution.String(), when.Add(2*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
	return fx
}

func walkPickup(t *testing.T, env *execEnv, commands *service.TransportExecutionCommandService, fx execFixture, stopIndex int, when time.Time) {
	t.Helper()
	mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, stopIndex, -1, "walk-arrive-"+fx.execution.String(), when, stopVersion(t, env, fx.stops[stopIndex]), ""))
	mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, stopIndex, -1, "walk-start-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[stopIndex]), ""))
	mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, stopIndex, 0, "walk-confirm-"+fx.execution.String(), when.Add(2*time.Minute), stopVersion(t, env, fx.stops[stopIndex]), ""))
	mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, stopIndex, -1, "walk-complete-"+fx.execution.String(), when.Add(3*time.Minute), stopVersion(t, env, fx.stops[stopIndex]), ""))
}

func deliveryRoute(t *testing.T, env *execEnv, commands *service.TransportExecutionCommandService, when time.Time) execFixture {
	t.Helper()
	fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
		{{0, domain.ActionTypePickup}},
		{{0, domain.ActionTypeDelivery}},
		{{0, domain.ActionTypeDelivery}},
	})
	skipStart(t, env, commands, fx, when)
	walkPickup(t, env, commands, fx, 1, when.Add(3*time.Minute))
	depart := fx.driverCmd(domain.CommandDepartedPickup, -1, -1, "depart-"+fx.execution.String(), when.Add(10*time.Minute), 1, "")
	depart.ShipmentID = fx.ships[0].shipmentID
	depart.ShipmentTenantID = fx.ships[0].tenantID
	mustExec(t, commands, depart)
	return fx
}

func reachUnloading(t *testing.T, env *execEnv, commands *service.TransportExecutionCommandService, when time.Time) execFixture {
	t.Helper()
	fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
		{{0, domain.ActionTypePickup}},
		{{0, domain.ActionTypeDelivery}},
	})
	skipStart(t, env, commands, fx, when)
	walkPickup(t, env, commands, fx, 1, when.Add(3*time.Minute))
	depart := fx.driverCmd(domain.CommandDepartedPickup, -1, -1, "depart-final-"+fx.execution.String(), when.Add(8*time.Minute), 1, "")
	depart.ShipmentID = fx.ships[0].shipmentID
	depart.ShipmentTenantID = fx.ships[0].tenantID
	mustExec(t, commands, depart)
	mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "final-arrive-"+fx.execution.String(), when.Add(9*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
	mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "final-start-"+fx.execution.String(), when.Add(10*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
	return fx
}

var commandTestCtx context.Context

func execCmd(commands *service.TransportExecutionCommandService, cmd domain.ExecutionCommand) error {
	_, err := commands.Execute(commandTestCtx, cmd)
	return err
}

func mustExec(t *testing.T, commands *service.TransportExecutionCommandService, cmd domain.ExecutionCommand) domain.ExecutionCommandResult {
	t.Helper()
	res, err := commands.Execute(commandTestCtx, cmd)
	if err != nil {
		t.Fatalf("%s: %v", cmd.Name, err)
	}
	return res
}

func stopVersion(t *testing.T, env *execEnv, id uuid.UUID) int {
	t.Helper()
	var version int
	if err := env.pool.QueryRow(env.ctx, `SELECT version FROM transport.transport_execution_stops WHERE id=$1`, id).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func stopStatusOf(t *testing.T, env *execEnv, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := env.pool.QueryRow(env.ctx, `SELECT status FROM transport.transport_execution_stops WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func stopReason(t *testing.T, env *execEnv, id uuid.UUID) string {
	t.Helper()
	var reason *string
	if err := env.pool.QueryRow(env.ctx, `SELECT status_reason FROM transport.transport_execution_stops WHERE id=$1`, id).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if reason == nil {
		return ""
	}
	return *reason
}

func evidenceCount(t *testing.T, env *execEnv, shipmentID uuid.UUID, state string) int {
	t.Helper()
	return countWhere(t, env, "transport.shipment_cargo_execution_evidence", "shipment_id=$1 AND state=$2", shipmentID, state)
}

func evidenceState(t *testing.T, env *execEnv, id uuid.UUID) string {
	t.Helper()
	var state string
	if err := env.pool.QueryRow(env.ctx, `SELECT state FROM transport.shipment_cargo_execution_evidence WHERE id=$1`, id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func podCount(t *testing.T, env *execEnv, shipmentID uuid.UUID) int {
	t.Helper()
	var n int
	err := env.pool.QueryRow(env.ctx, `
		SELECT count(*) FROM documents.documents
		WHERE related_entity_type='SHIPMENT' AND related_entity_id=$1 AND document_type='POD' AND deleted_at IS NULL
	`, shipmentID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func constraintHas(err error, needle string) bool {
	if err == nil {
		return false
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		detail, _ := appErr.Details["detail"].(string)
		if strings.Contains(detail, needle) {
			return true
		}
	}
	return strings.Contains(err.Error(), needle)
}

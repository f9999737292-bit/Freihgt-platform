package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestProjectionDigestIgnoresSubjectOrder(t *testing.T) {
	base := sampleProjection()
	swapped := base
	swapped.ExecutionSubjects = append([]ProjectionSubject(nil), base.ExecutionSubjects...)
	swapped.ExecutionSubjects[0], swapped.ExecutionSubjects[1] = swapped.ExecutionSubjects[1], swapped.ExecutionSubjects[0]
	left, err := CanonicalDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	right, err := CanonicalDigest(swapped)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("digest changed with subject order: %s %s", left, right)
	}
}

func TestProjectionDigestChangesWhenStopMoves(t *testing.T) {
	base := sampleProjection()
	moved := base
	moved.Stops = append([]ProjectionStop(nil), base.Stops...)
	moved.Stops[1].Latitude = 11.5
	left, err := CanonicalDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	right, err := CanonicalDigest(moved)
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("changed stop did not change digest")
	}
}

func TestUnmaterializedCargoActionIsRejected(t *testing.T) {
	cmd := sampleProjection()
	cmd.Actions[0].CargoID = nil
	err := RejectReplayOrCreate(cmd, "", false, "unused")
	if reasonOf(err) != ReasonExecutionSubjectUnmaterialized {
		t.Fatalf("reason=%s err=%v", reasonOf(err), err)
	}
}

func TestExecutionLinkedIsNotAProjectionTrigger(t *testing.T) {
	cmd := sampleProjection()
	cmd.ActivationStatus = "EXECUTION_LINKED"
	err := RejectReplayOrCreate(cmd, "", false, "unused")
	if reasonOf(err) != ReasonActivationStatusRejected {
		t.Fatalf("reason=%s", reasonOf(err))
	}
}

func TestSuccessorProjectionIsNotCreatedInThisWave(t *testing.T) {
	cmd := sampleProjection()
	planID := uuid.New()
	cmd.SupersedesRoutePlanID = &planID
	err := RejectReplayOrCreate(cmd, "", false, "unused")
	if reasonOf(err) != ReasonExecutionPlanConflict {
		t.Fatalf("reason=%s", reasonOf(err))
	}
}

func TestSameActivationBodyConflict(t *testing.T) {
	cmd := sampleProjection()
	err := RejectReplayOrCreate(cmd, "stored-digest", true, "other-digest")
	if reasonOf(err) != ReasonActivationBodyConflict {
		t.Fatalf("reason=%s", reasonOf(err))
	}
	if err := RejectReplayOrCreate(cmd, "same", true, "same"); err != nil {
		t.Fatal(err)
	}
}

func TestDepotStartDoesNotRequireAShipment(t *testing.T) {
	cmd := sampleProjection()
	cmd.PlanningMode = PlanningModeDepotStart
	cmd.ContextShipmentID = nil
	cmd.ContextShipmentTenantID = nil
	cmd.ContextShipmentVersion = nil
	cmd.ExecutionSubjects = nil
	cmd.Actions = nil
	if err := ValidateNewProjection(cmd); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentTripShipmentMustBeAParticipant(t *testing.T) {
	cmd := sampleProjection()
	other := uuid.New()
	cmd.ContextShipmentID = &other
	err := ValidateNewProjection(cmd)
	if reasonOf(err) != ReasonExecutionPlanConflict {
		t.Fatalf("reason=%s err=%v", reasonOf(err), err)
	}
}

func TestLoadOpportunityWithoutCargoIsNotExecutable(t *testing.T) {
	cmd := sampleProjection()
	cmd.ExecutionSubjects[0].RouteSubjectType = RouteSubjectLoadOpportunity
	cmd.ExecutionSubjects[0].CargoID = nil
	cmd.Actions[0].RouteSubjectType = RouteSubjectLoadOpportunity
	cmd.Actions[0].CargoID = nil
	err := ValidateNewProjection(cmd)
	if reasonOf(err) != ReasonExecutionSubjectUnmaterialized {
		t.Fatalf("reason=%s", reasonOf(err))
	}
}

func reasonOf(err error) string {
	var appErr *apperrors.AppError
	if !apperrorsAs(err, &appErr) {
		return ""
	}
	reason, _ := appErr.Details["reason"].(string)
	return reason
}

func apperrorsAs(err error, target **apperrors.AppError) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		return false
	}
	*target = appErr
	return true
}

func sampleProjection() ProjectionCommand {
	shipperA := uuid.New()
	shipperB := uuid.New()
	shipmentA := uuid.New()
	shipmentB := uuid.New()
	cargoA := uuid.New()
	cargoB := uuid.New()
	subjectA := uuid.New()
	subjectB := uuid.New()
	stopStart := uuid.New()
	stopCargo := uuid.New()
	location := uuid.New()
	version := 1
	duration := 600
	arrival := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	return ProjectionCommand{
		ActivationID:            uuid.New(),
		ActivationVersion:       1,
		ActivationStatus:        ActivationStatusPendingExecution,
		RoutePlanID:             uuid.New(),
		RoutePlanVersion:        3,
		PlanningMode:            PlanningModeCurrentTrip,
		OperatingTenantID:       uuid.New(),
		ContextShipmentID:       &shipmentA,
		ContextShipmentTenantID: &shipperA,
		ContextShipmentVersion:  &version,
		CarrierCompanyID:        uuid.New(),
		EvaluationFingerprint:   "fingerprint-1",
		ExecutionSubjects: []ProjectionSubject{
			subject(RouteSubjectShipmentCargo, subjectA, shipmentA, shipperA, cargoA, version),
			subject(RouteSubjectShipmentCargo, subjectB, shipmentB, shipperB, cargoB, version),
		},
		Stops: []ProjectionStop{
			{
				RoutePlanStopID: stopStart,
				Ordinal:         0,
				StopRole:        StopRoleStart,
				PointKind:       PointKindPositionAnchor,
				Latitude:        55.75,
				Longitude:       37.61,
			},
			{
				RoutePlanStopID:        stopCargo,
				Ordinal:                1,
				StopRole:               StopRoleCargo,
				PointKind:              PointKindCanonicalLocation,
				LocationID:             &location,
				Latitude:               55.8,
				Longitude:              37.7,
				PlannedArrival:         &arrival,
				ServiceDurationSeconds: &duration,
			},
		},
		Actions: []ProjectionAction{
			action(stopCargo, subjectA, shipmentA, shipperA, cargoA, version, 0),
			action(stopCargo, subjectB, shipmentB, shipperB, cargoB, version, 1),
		},
	}
}

func subject(kind string, subjectID, shipmentID, tenantID, cargoID uuid.UUID, version int) ProjectionSubject {
	return ProjectionSubject{
		RouteSubjectType:         kind,
		RouteSubjectID:           subjectID,
		RouteSubjectVersion:      version,
		ExecutionShipmentID:      &shipmentID,
		ShipmentTenantID:         &tenantID,
		ExecutionShipmentVersion: &version,
		CargoID:                  &cargoID,
		CargoVersion:             &version,
	}
}

func action(stopID, subjectID, shipmentID, tenantID, cargoID uuid.UUID, version, ordinal int) ProjectionAction {
	return ProjectionAction{
		RoutePlanActionID:        uuid.New(),
		RoutePlanStopID:          stopID,
		ActionOrdinal:            ordinal,
		ActionType:               ActionTypePickup,
		RouteSubjectType:         RouteSubjectShipmentCargo,
		RouteSubjectID:           subjectID,
		ExecutionShipmentID:      &shipmentID,
		ShipmentTenantID:         &tenantID,
		ExecutionShipmentVersion: &version,
		CargoID:                  &cargoID,
		CargoVersion:             &version,
		EvidenceState:            "PLANNED",
		EvidenceStateVersion:     &version,
	}
}

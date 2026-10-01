package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	PlanningModeDepotStart  = "DEPOT_START"
	PlanningModeCurrentTrip = "CURRENT_TRIP"

	ActivationStatusPendingExecution = "PENDING_EXECUTION"

	RevisionStatusActive     = "ACTIVE"
	RevisionStatusSuperseded = "SUPERSEDED"

	StopRoleStart = "START"
	StopRoleCargo = "CARGO"
	StopRoleEnd   = "END"

	PointKindCanonicalLocation = "CANONICAL_LOCATION"
	PointKindPositionAnchor    = "POSITION_ANCHOR"

	StopStatusPlanned        = "PLANNED"
	StopStatusArrived        = "ARRIVED"
	StopStatusServiceStarted = "SERVICE_STARTED"
	StopStatusCompleted      = "COMPLETED"
	StopStatusCancelled      = "CANCELLED"
	StopStatusSkipped        = "SKIPPED"

	ActionTypePickup   = "PICKUP"
	ActionTypeDelivery = "DELIVERY"

	ActionStatusPending   = "PENDING"
	ActionStatusCompleted = "COMPLETED"
	ActionStatusFailed    = "FAILED"
	ActionStatusCancelled = "CANCELLED"

	MembershipIntroduced         = "INTRODUCED"
	MembershipInheritedCompleted = "INHERITED_COMPLETED"
	MembershipInheritedInService = "INHERITED_IN_SERVICE"
	MembershipSuperseded         = "SUPERSEDED"

	RouteSubjectLoadOpportunity = "LOAD_OPPORTUNITY"
	RouteSubjectShipmentCargo   = "SHIPMENT_CARGO"

	ParticipantProvenanceTrustedProjection = "TRUSTED_PROJECTION_CONTRACT"

	ReasonExecutionSubjectUnmaterialized = "EXECUTION_SUBJECT_UNMATERIALIZED"
	ReasonActivationBodyConflict         = "ACTIVATION_BODY_CONFLICT"
	ReasonExecutionPlanConflict          = "EXECUTION_PLAN_CONFLICT"
	ReasonPlanStale                      = "PLAN_STALE"
	ReasonActivationStatusRejected       = "ACTIVATION_STATUS_REJECTED"
)

// ProjectionCommand is the in-process projection contract.
// TMS-MSTOP-0.1A does not expose it over HTTP. shipment_tenant_id is checked against the shipment row.
type ProjectionCommand struct {
	ActivationID            uuid.UUID           `json:"activation_id"`
	ActivationVersion       int                 `json:"activation_version"`
	ActivationStatus        string              `json:"activation_status"`
	RoutePlanID             uuid.UUID           `json:"route_plan_id"`
	RoutePlanVersion        int                 `json:"route_plan_version"`
	PlanningMode            string              `json:"planning_mode"`
	OperatingTenantID       uuid.UUID           `json:"operating_tenant_id"`
	ContextShipmentID       *uuid.UUID          `json:"context_shipment_id"`
	ContextShipmentTenantID *uuid.UUID          `json:"context_shipment_tenant_id"`
	ContextShipmentVersion  *int                `json:"context_shipment_version"`
	CarrierCompanyID        uuid.UUID           `json:"carrier_company_id"`
	VehicleID               *uuid.UUID          `json:"vehicle_id"`
	DriverID                *uuid.UUID          `json:"driver_id"`
	EvaluationFingerprint   string              `json:"evaluation_fingerprint"`
	SupersedesRoutePlanID   *uuid.UUID          `json:"supersedes_route_plan_id"`
	SupersedesActivationID  *uuid.UUID          `json:"supersedes_activation_id"`
	ExecutionSubjects       []ProjectionSubject `json:"execution_subjects"`
	Stops                   []ProjectionStop    `json:"stops"`
	Actions                 []ProjectionAction  `json:"actions"`
}

type ProjectionSubject struct {
	RouteSubjectType         string     `json:"route_subject_type"`
	RouteSubjectID           uuid.UUID  `json:"route_subject_id"`
	RouteSubjectVersion      int        `json:"route_subject_version"`
	ExecutionShipmentID      *uuid.UUID `json:"execution_shipment_id"`
	ShipmentTenantID         *uuid.UUID `json:"shipment_tenant_id"`
	ExecutionShipmentVersion *int       `json:"execution_shipment_version"`
	CargoID                  *uuid.UUID `json:"cargo_id"`
	CargoVersion             *int       `json:"cargo_version"`
}

type ProjectionStop struct {
	RoutePlanStopID        uuid.UUID  `json:"route_plan_stop_id"`
	Ordinal                int        `json:"ordinal"`
	StopRole               string     `json:"stop_role"`
	PointKind              string     `json:"point_kind"`
	LocationID             *uuid.UUID `json:"location_id"`
	Latitude               float64    `json:"latitude"`
	Longitude              float64    `json:"longitude"`
	PlannedArrival         *time.Time `json:"planned_arrival"`
	PlannedDeparture       *time.Time `json:"planned_departure"`
	ServiceDurationSeconds *int       `json:"service_duration_seconds"`
}

type ProjectionAction struct {
	RoutePlanActionID        uuid.UUID  `json:"route_plan_action_id"`
	RoutePlanStopID          uuid.UUID  `json:"route_plan_stop_id"`
	ActionOrdinal            int        `json:"action_ordinal"`
	ActionType               string     `json:"action_type"`
	RouteSubjectType         string     `json:"route_subject_type"`
	RouteSubjectID           uuid.UUID  `json:"route_subject_id"`
	ExecutionShipmentID      *uuid.UUID `json:"execution_shipment_id"`
	ShipmentTenantID         *uuid.UUID `json:"shipment_tenant_id"`
	ExecutionShipmentVersion *int       `json:"execution_shipment_version"`
	CargoID                  *uuid.UUID `json:"cargo_id"`
	CargoVersion             *int       `json:"cargo_version"`
	EvidenceState            string     `json:"evidence_state"`
	EvidenceStateVersion     *int       `json:"evidence_state_version"`
}

type ProjectionResult struct {
	ExecutionID  uuid.UUID
	RevisionID   uuid.UUID
	ActivationID uuid.UUID
	Created      bool
}

type MaterializedSubject struct {
	RouteSubjectType string
	RouteSubjectID   uuid.UUID
	ShipmentID       uuid.UUID
	ShipmentTenantID uuid.UUID
	ShipmentVersion  int
	CargoID          uuid.UUID
	CargoVersion     int
}

func ProjectionConflict(reason string) *apperrors.AppError {
	return apperrors.Conflict(reason, map[string]any{"reason": reason})
}

// CanonicalDigest hashes the projection body. Same activation with a different digest conflicts.
func CanonicalDigest(cmd ProjectionCommand) (string, error) {
	payload, err := json.Marshal(canonicalProjection(cmd))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// RejectReplayOrCreate reports idempotency and shape failures that must happen before any insert.
// Database existence checks stay in the repository transaction.
func RejectReplayOrCreate(cmd ProjectionCommand, storedDigest string, hasStored bool, digest string) error {
	if cmd.ActivationStatus != ActivationStatusPendingExecution {
		return ProjectionConflict(ReasonActivationStatusRejected)
	}
	if hasStored {
		if storedDigest == digest {
			return nil
		}
		return ProjectionConflict(ReasonActivationBodyConflict)
	}
	if cmd.SupersedesRoutePlanID != nil || cmd.SupersedesActivationID != nil {
		return ProjectionConflict(ReasonExecutionPlanConflict)
	}
	return ValidateNewProjection(cmd)
}

func ValidateNewProjection(cmd ProjectionCommand) error {
	if cmd.ActivationID == uuid.Nil {
		return apperrors.Validation("activation_id is required", map[string]any{"field": "activation_id"})
	}
	if cmd.ActivationVersion < 1 {
		return apperrors.Validation("activation_version is required", map[string]any{"field": "activation_version"})
	}
	if cmd.RoutePlanID == uuid.Nil {
		return apperrors.Validation("route_plan_id is required", map[string]any{"field": "route_plan_id"})
	}
	if cmd.RoutePlanVersion < 1 {
		return apperrors.Validation("route_plan_version is required", map[string]any{"field": "route_plan_version"})
	}
	if cmd.PlanningMode != PlanningModeDepotStart && cmd.PlanningMode != PlanningModeCurrentTrip {
		return apperrors.Validation("planning_mode is invalid", map[string]any{"field": "planning_mode"})
	}
	if cmd.OperatingTenantID == uuid.Nil {
		return apperrors.Validation("operating_tenant_id is required", map[string]any{"field": "operating_tenant_id"})
	}
	if cmd.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if strings.TrimSpace(cmd.EvaluationFingerprint) == "" {
		return apperrors.Validation("evaluation_fingerprint is required", map[string]any{"field": "evaluation_fingerprint"})
	}
	if err := validateContext(cmd); err != nil {
		return err
	}
	if len(cmd.Stops) == 0 {
		return apperrors.Validation("stops are required", map[string]any{"field": "stops"})
	}
	stops, err := indexStops(cmd.Stops)
	if err != nil {
		return err
	}
	subjects, err := materializedSubjects(cmd.ExecutionSubjects)
	if err != nil {
		return err
	}
	if err := validateActions(cmd.Actions, stops, subjects); err != nil {
		return err
	}
	if cmd.PlanningMode == PlanningModeCurrentTrip {
		if !contextIsParticipant(cmd, subjects) {
			return ProjectionConflict(ReasonExecutionPlanConflict)
		}
	}
	if cmd.PlanningMode == PlanningModeDepotStart && len(cmd.Actions) > 0 && len(subjects) == 0 {
		return ProjectionConflict(ReasonExecutionSubjectUnmaterialized)
	}
	return nil
}

func materializedSubjects(rows []ProjectionSubject) ([]MaterializedSubject, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]MaterializedSubject, 0, len(rows))
	seenCargo := map[string]struct{}{}
	seenShipmentVersion := map[string]int{}
	for _, row := range rows {
		if row.ExecutionShipmentID == nil || row.ShipmentTenantID == nil || row.CargoID == nil ||
			*row.ExecutionShipmentID == uuid.Nil || *row.ShipmentTenantID == uuid.Nil || *row.CargoID == uuid.Nil ||
			row.ExecutionShipmentVersion == nil || row.CargoVersion == nil ||
			*row.ExecutionShipmentVersion < 1 || *row.CargoVersion < 1 {
			return nil, ProjectionConflict(ReasonExecutionSubjectUnmaterialized)
		}
		if row.RouteSubjectType != RouteSubjectLoadOpportunity && row.RouteSubjectType != RouteSubjectShipmentCargo {
			return nil, apperrors.Validation("route_subject_type is invalid", map[string]any{"field": "route_subject_type"})
		}
		if row.RouteSubjectID == uuid.Nil || row.RouteSubjectVersion < 1 {
			return nil, apperrors.Validation("route subject identity is required", map[string]any{"field": "route_subject_id"})
		}
		shipmentKey := row.ShipmentTenantID.String() + ":" + row.ExecutionShipmentID.String()
		if prev, ok := seenShipmentVersion[shipmentKey]; ok && prev != *row.ExecutionShipmentVersion {
			return nil, ProjectionConflict(ReasonPlanStale)
		}
		seenShipmentVersion[shipmentKey] = *row.ExecutionShipmentVersion
		cargoKey := shipmentKey + ":" + row.CargoID.String()
		if _, ok := seenCargo[cargoKey]; ok {
			return nil, apperrors.Validation("duplicate execution subject", map[string]any{"field": "execution_subjects"})
		}
		seenCargo[cargoKey] = struct{}{}
		out = append(out, MaterializedSubject{
			RouteSubjectType: row.RouteSubjectType,
			RouteSubjectID:   row.RouteSubjectID,
			ShipmentID:       *row.ExecutionShipmentID,
			ShipmentTenantID: *row.ShipmentTenantID,
			ShipmentVersion:  *row.ExecutionShipmentVersion,
			CargoID:          *row.CargoID,
			CargoVersion:     *row.CargoVersion,
		})
	}
	return out, nil
}

func validateContext(cmd ProjectionCommand) error {
	set := 0
	if cmd.ContextShipmentID != nil {
		set++
	}
	if cmd.ContextShipmentTenantID != nil {
		set++
	}
	if cmd.ContextShipmentVersion != nil {
		set++
	}
	switch cmd.PlanningMode {
	case PlanningModeDepotStart:
		if set != 0 {
			return apperrors.Validation("depot start does not take a context shipment", map[string]any{"field": "context_shipment_id"})
		}
	case PlanningModeCurrentTrip:
		if set != 3 || cmd.ContextShipmentID == nil || *cmd.ContextShipmentID == uuid.Nil ||
			cmd.ContextShipmentTenantID == nil || *cmd.ContextShipmentTenantID == uuid.Nil ||
			cmd.ContextShipmentVersion == nil || *cmd.ContextShipmentVersion < 1 {
			return apperrors.Validation("current trip context shipment is required", map[string]any{"field": "context_shipment_id"})
		}
	}
	return nil
}

func indexStops(rows []ProjectionStop) (map[uuid.UUID]ProjectionStop, error) {
	out := make(map[uuid.UUID]ProjectionStop, len(rows))
	ordinals := map[int]struct{}{}
	for _, row := range rows {
		if row.RoutePlanStopID == uuid.Nil {
			return nil, apperrors.Validation("route_plan_stop_id is required", map[string]any{"field": "route_plan_stop_id"})
		}
		if _, ok := out[row.RoutePlanStopID]; ok {
			return nil, apperrors.Validation("duplicate route_plan_stop_id", map[string]any{"field": "route_plan_stop_id"})
		}
		if row.Ordinal < 0 {
			return nil, apperrors.Validation("ordinal is invalid", map[string]any{"field": "ordinal"})
		}
		if _, ok := ordinals[row.Ordinal]; ok {
			return nil, apperrors.Validation("duplicate stop ordinal", map[string]any{"field": "ordinal"})
		}
		ordinals[row.Ordinal] = struct{}{}
		switch row.StopRole {
		case StopRoleStart, StopRoleCargo, StopRoleEnd:
		default:
			return nil, apperrors.Validation("stop_role is invalid", map[string]any{"field": "stop_role"})
		}
		switch row.PointKind {
		case PointKindCanonicalLocation:
			if row.LocationID == nil || *row.LocationID == uuid.Nil {
				return nil, apperrors.Validation("location_id is required", map[string]any{"field": "location_id"})
			}
		case PointKindPositionAnchor:
		default:
			return nil, apperrors.Validation("point_kind is invalid", map[string]any{"field": "point_kind"})
		}
		if math.IsNaN(row.Latitude) || math.IsNaN(row.Longitude) || math.IsInf(row.Latitude, 0) || math.IsInf(row.Longitude, 0) {
			return nil, apperrors.Validation("stop coordinates are invalid", map[string]any{"field": "latitude"})
		}
		if row.ServiceDurationSeconds != nil && *row.ServiceDurationSeconds < 0 {
			return nil, apperrors.Validation("service_duration_seconds is invalid", map[string]any{"field": "service_duration_seconds"})
		}
		out[row.RoutePlanStopID] = row
	}
	return out, nil
}

func validateActions(rows []ProjectionAction, stops map[uuid.UUID]ProjectionStop, subjects []MaterializedSubject) error {
	ordinals := map[string]struct{}{}
	for _, row := range rows {
		if row.RoutePlanActionID == uuid.Nil || row.RoutePlanStopID == uuid.Nil {
			return ProjectionConflict(ReasonExecutionSubjectUnmaterialized)
		}
		if _, ok := stops[row.RoutePlanStopID]; !ok {
			return apperrors.Validation("action stop is not on the route", map[string]any{"field": "route_plan_stop_id"})
		}
		if row.ActionOrdinal < 0 {
			return apperrors.Validation("action_ordinal is invalid", map[string]any{"field": "action_ordinal"})
		}
		key := row.RoutePlanStopID.String() + ":" + strconv.Itoa(row.ActionOrdinal)
		if _, ok := ordinals[key]; ok {
			return apperrors.Validation("duplicate action ordinal", map[string]any{"field": "action_ordinal"})
		}
		ordinals[key] = struct{}{}
		if row.ActionType != ActionTypePickup && row.ActionType != ActionTypeDelivery {
			return apperrors.Validation("action_type is invalid", map[string]any{"field": "action_type"})
		}
		if row.ExecutionShipmentID == nil || row.ShipmentTenantID == nil || row.CargoID == nil ||
			*row.ExecutionShipmentID == uuid.Nil || *row.ShipmentTenantID == uuid.Nil || *row.CargoID == uuid.Nil ||
			row.CargoVersion == nil || *row.CargoVersion < 1 ||
			row.ExecutionShipmentVersion == nil || *row.ExecutionShipmentVersion < 1 {
			return ProjectionConflict(ReasonExecutionSubjectUnmaterialized)
		}
		if !actionMatchesSubject(row, subjects) {
			return ProjectionConflict(ReasonExecutionSubjectUnmaterialized)
		}
	}
	return nil
}

func actionMatchesSubject(row ProjectionAction, subjects []MaterializedSubject) bool {
	for _, subject := range subjects {
		if subject.ShipmentID == *row.ExecutionShipmentID &&
			subject.ShipmentTenantID == *row.ShipmentTenantID &&
			subject.CargoID == *row.CargoID &&
			subject.CargoVersion == *row.CargoVersion &&
			subject.ShipmentVersion == *row.ExecutionShipmentVersion &&
			subject.RouteSubjectType == row.RouteSubjectType &&
			subject.RouteSubjectID == row.RouteSubjectID {
			return true
		}
	}
	return false
}

func contextIsParticipant(cmd ProjectionCommand, subjects []MaterializedSubject) bool {
	for _, subject := range subjects {
		if subject.ShipmentID == *cmd.ContextShipmentID &&
			subject.ShipmentTenantID == *cmd.ContextShipmentTenantID &&
			subject.ShipmentVersion == *cmd.ContextShipmentVersion {
			return true
		}
	}
	return false
}

type canonProjection struct {
	ActivationID            string         `json:"activation_id"`
	ActivationVersion       int            `json:"activation_version"`
	ActivationStatus        string         `json:"activation_status"`
	RoutePlanID             string         `json:"route_plan_id"`
	RoutePlanVersion        int            `json:"route_plan_version"`
	PlanningMode            string         `json:"planning_mode"`
	OperatingTenantID       string         `json:"operating_tenant_id"`
	ContextShipmentID       string         `json:"context_shipment_id"`
	ContextShipmentTenantID string         `json:"context_shipment_tenant_id"`
	ContextShipmentVersion  *int           `json:"context_shipment_version"`
	CarrierCompanyID        string         `json:"carrier_company_id"`
	VehicleID               string         `json:"vehicle_id"`
	DriverID                string         `json:"driver_id"`
	EvaluationFingerprint   string         `json:"evaluation_fingerprint"`
	SupersedesRoutePlanID   string         `json:"supersedes_route_plan_id"`
	SupersedesActivationID  string         `json:"supersedes_activation_id"`
	ExecutionSubjects       []canonSubject `json:"execution_subjects"`
	Stops                   []canonStop    `json:"stops"`
	Actions                 []canonAction  `json:"actions"`
}

type canonSubject struct {
	RouteSubjectType         string `json:"route_subject_type"`
	RouteSubjectID           string `json:"route_subject_id"`
	RouteSubjectVersion      int    `json:"route_subject_version"`
	ExecutionShipmentID      string `json:"execution_shipment_id"`
	ShipmentTenantID         string `json:"shipment_tenant_id"`
	ExecutionShipmentVersion *int   `json:"execution_shipment_version"`
	CargoID                  string `json:"cargo_id"`
	CargoVersion             *int   `json:"cargo_version"`
}

type canonStop struct {
	RoutePlanStopID        string  `json:"route_plan_stop_id"`
	Ordinal                int     `json:"ordinal"`
	StopRole               string  `json:"stop_role"`
	PointKind              string  `json:"point_kind"`
	LocationID             string  `json:"location_id"`
	Latitude               float64 `json:"latitude"`
	Longitude              float64 `json:"longitude"`
	PlannedArrival         string  `json:"planned_arrival"`
	PlannedDeparture       string  `json:"planned_departure"`
	ServiceDurationSeconds *int    `json:"service_duration_seconds"`
}

type canonAction struct {
	RoutePlanActionID        string `json:"route_plan_action_id"`
	RoutePlanStopID          string `json:"route_plan_stop_id"`
	ActionOrdinal            int    `json:"action_ordinal"`
	ActionType               string `json:"action_type"`
	RouteSubjectType         string `json:"route_subject_type"`
	RouteSubjectID           string `json:"route_subject_id"`
	ExecutionShipmentID      string `json:"execution_shipment_id"`
	ShipmentTenantID         string `json:"shipment_tenant_id"`
	ExecutionShipmentVersion *int   `json:"execution_shipment_version"`
	CargoID                  string `json:"cargo_id"`
	CargoVersion             *int   `json:"cargo_version"`
	EvidenceState            string `json:"evidence_state"`
	EvidenceStateVersion     *int   `json:"evidence_state_version"`
}

func canonicalProjection(cmd ProjectionCommand) canonProjection {
	subjects := slices.Clone(cmd.ExecutionSubjects)
	slices.SortFunc(subjects, func(a, b ProjectionSubject) int {
		return strings.Compare(subjectSortKey(a), subjectSortKey(b))
	})
	stops := slices.Clone(cmd.Stops)
	slices.SortFunc(stops, func(a, b ProjectionStop) int {
		if a.Ordinal != b.Ordinal {
			return a.Ordinal - b.Ordinal
		}
		return strings.Compare(a.RoutePlanStopID.String(), b.RoutePlanStopID.String())
	})
	actions := slices.Clone(cmd.Actions)
	slices.SortFunc(actions, func(a, b ProjectionAction) int {
		if cmp := strings.Compare(a.RoutePlanStopID.String(), b.RoutePlanStopID.String()); cmp != 0 {
			return cmp
		}
		if a.ActionOrdinal != b.ActionOrdinal {
			return a.ActionOrdinal - b.ActionOrdinal
		}
		return strings.Compare(a.RoutePlanActionID.String(), b.RoutePlanActionID.String())
	})
	out := canonProjection{
		ActivationID:            cmd.ActivationID.String(),
		ActivationVersion:       cmd.ActivationVersion,
		ActivationStatus:        cmd.ActivationStatus,
		RoutePlanID:             cmd.RoutePlanID.String(),
		RoutePlanVersion:        cmd.RoutePlanVersion,
		PlanningMode:            cmd.PlanningMode,
		OperatingTenantID:       cmd.OperatingTenantID.String(),
		ContextShipmentID:       uuidString(cmd.ContextShipmentID),
		ContextShipmentTenantID: uuidString(cmd.ContextShipmentTenantID),
		ContextShipmentVersion:  cmd.ContextShipmentVersion,
		CarrierCompanyID:        cmd.CarrierCompanyID.String(),
		VehicleID:               uuidString(cmd.VehicleID),
		DriverID:                uuidString(cmd.DriverID),
		EvaluationFingerprint:   cmd.EvaluationFingerprint,
		SupersedesRoutePlanID:   uuidString(cmd.SupersedesRoutePlanID),
		SupersedesActivationID:  uuidString(cmd.SupersedesActivationID),
	}
	for _, row := range subjects {
		out.ExecutionSubjects = append(out.ExecutionSubjects, canonSubject{
			RouteSubjectType:         row.RouteSubjectType,
			RouteSubjectID:           row.RouteSubjectID.String(),
			RouteSubjectVersion:      row.RouteSubjectVersion,
			ExecutionShipmentID:      uuidString(row.ExecutionShipmentID),
			ShipmentTenantID:         uuidString(row.ShipmentTenantID),
			ExecutionShipmentVersion: row.ExecutionShipmentVersion,
			CargoID:                  uuidString(row.CargoID),
			CargoVersion:             row.CargoVersion,
		})
	}
	for _, row := range stops {
		out.Stops = append(out.Stops, canonStop{
			RoutePlanStopID:        row.RoutePlanStopID.String(),
			Ordinal:                row.Ordinal,
			StopRole:               row.StopRole,
			PointKind:              row.PointKind,
			LocationID:             uuidString(row.LocationID),
			Latitude:               row.Latitude,
			Longitude:              row.Longitude,
			PlannedArrival:         timeString(row.PlannedArrival),
			PlannedDeparture:       timeString(row.PlannedDeparture),
			ServiceDurationSeconds: row.ServiceDurationSeconds,
		})
	}
	for _, row := range actions {
		out.Actions = append(out.Actions, canonAction{
			RoutePlanActionID:        row.RoutePlanActionID.String(),
			RoutePlanStopID:          row.RoutePlanStopID.String(),
			ActionOrdinal:            row.ActionOrdinal,
			ActionType:               row.ActionType,
			RouteSubjectType:         row.RouteSubjectType,
			RouteSubjectID:           row.RouteSubjectID.String(),
			ExecutionShipmentID:      uuidString(row.ExecutionShipmentID),
			ShipmentTenantID:         uuidString(row.ShipmentTenantID),
			ExecutionShipmentVersion: row.ExecutionShipmentVersion,
			CargoID:                  uuidString(row.CargoID),
			CargoVersion:             row.CargoVersion,
			EvidenceState:            row.EvidenceState,
			EvidenceStateVersion:     row.EvidenceStateVersion,
		})
	}
	return out
}

func subjectSortKey(row ProjectionSubject) string {
	return uuidString(row.ShipmentTenantID) + ":" + uuidString(row.ExecutionShipmentID) + ":" + uuidString(row.CargoID) + ":" + row.RouteSubjectID.String()
}

type TrackingStopContext struct {
	ExecutionID           uuid.UUID
	RevisionID            uuid.UUID
	ExecutionStopID       uuid.UUID
	Ordinal               int
	Status                string
	PlannedArrival        *time.Time
	LocationID            *uuid.UUID
	TargetLatitude        *float64
	TargetLongitude       *float64
	DriverID              *uuid.UUID
	VehicleID             *uuid.UUID
	PointKind             string
	StopRole              string
	LiveETAStopID         *uuid.UUID
	LiveETAOrdinal        *int
	LiveETAPlannedArrival *time.Time
	LiveETALocationID     *uuid.UUID
	LiveETALatitude       *float64
	LiveETALongitude      *float64
}

func uuidString(value *uuid.UUID) string {
	if value == nil || *value == uuid.Nil {
		return ""
	}
	return value.String()
}

func timeString(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

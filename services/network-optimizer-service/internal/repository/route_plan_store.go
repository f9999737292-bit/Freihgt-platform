package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RoutePlanRow struct {
	ID                     uuid.UUID  `json:"id"`
	TenantID               uuid.UUID  `json:"tenant_id"`
	Version                int        `json:"version"`
	Status                 string     `json:"status"`
	PlanningMode           string     `json:"planning_mode"`
	ResultStatus           string     `json:"result_status"`
	CapacityID             *uuid.UUID `json:"capacity_id,omitempty"`
	CapacityVersion        *int       `json:"capacity_version,omitempty"`
	ShipmentID             *uuid.UUID `json:"shipment_id,omitempty"`
	ShipmentVersion        *int       `json:"shipment_version,omitempty"`
	VehicleID              *uuid.UUID `json:"vehicle_id,omitempty"`
	ContextFingerprint     string     `json:"context_fingerprint"`
	EvaluationFingerprint  string     `json:"evaluation_fingerprint"`
	AlgorithmPolicyVersion string     `json:"algorithm_policy_version"`
	RoutingPolicyVersion   string     `json:"routing_policy_version"`
	SupersedesPlanID       *uuid.UUID `json:"supersedes_plan_id,omitempty"`
	ExecutionSupported     bool       `json:"execution_supported"`
	ReasonCodes            []string   `json:"reason_codes"`
	CreatedAt              time.Time  `json:"created_at"`
	AcceptedAt             *time.Time `json:"accepted_at,omitempty"`
	CancelledAt            *time.Time `json:"cancelled_at,omitempty"`
	SupersededAt           *time.Time `json:"superseded_at,omitempty"`
}

type RoutePlanActivationRow struct {
	ID                  uuid.UUID  `json:"id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	RoutePlanID         uuid.UUID  `json:"route_plan_id"`
	PlanVersion         int        `json:"plan_version"`
	IdempotencyKey      string     `json:"idempotency_key"`
	ExecutionShipmentID *uuid.UUID `json:"execution_shipment_id,omitempty"`
	EffectiveShipmentID *uuid.UUID `json:"effective_shipment_id,omitempty"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
}

type RouteStopRow struct {
	ID                     uuid.UUID  `json:"id"`
	RoutePlanID            uuid.UUID  `json:"route_plan_id"`
	Ordinal                int        `json:"ordinal"`
	StopRole               string     `json:"stop_role"`
	PointKind              string     `json:"point_kind"`
	LocationID             *uuid.UUID `json:"location_id,omitempty"`
	Latitude               float64    `json:"latitude"`
	Longitude              float64    `json:"longitude"`
	PointSource            string     `json:"point_source"`
	PointObservedAt        *time.Time `json:"point_observed_at,omitempty"`
	PlannedArrival         *time.Time `json:"planned_arrival,omitempty"`
	PlannedDeparture       *time.Time `json:"planned_departure,omitempty"`
	ServiceDurationSeconds *int       `json:"service_duration_seconds,omitempty"`
}

type RouteActionRow struct {
	ID                    uuid.UUID       `json:"id"`
	RoutePlanID           uuid.UUID       `json:"route_plan_id"`
	StopID                uuid.UUID       `json:"stop_id"`
	ActionOrdinal         int             `json:"action_ordinal"`
	ActionType            string          `json:"action_type"`
	SubjectType           string          `json:"subject_type"`
	SubjectID             uuid.UUID       `json:"subject_id"`
	SubjectVersion        int             `json:"subject_version"`
	WeightDeltaKg         *float64        `json:"weight_delta_kg,omitempty"`
	VolumeDeltaM3         *float64        `json:"volume_delta_m3,omitempty"`
	PalletDelta           *float64        `json:"pallet_delta,omitempty"`
	LinearMetersDelta     *float64        `json:"linear_meters_delta,omitempty"`
	WindowStart           *time.Time      `json:"window_start,omitempty"`
	WindowEnd             *time.Time      `json:"window_end,omitempty"`
	SourceShipmentID      *uuid.UUID      `json:"source_shipment_id,omitempty"`
	SourceShipmentVersion *int            `json:"source_shipment_version,omitempty"`
	EvidenceState         string          `json:"evidence_state,omitempty"`
	EvidenceStateVersion  *int            `json:"evidence_state_version,omitempty"`
	EvidenceOccurredAt    *time.Time      `json:"evidence_occurred_at,omitempty"`
	PublicSubjectSnapshot json.RawMessage `json:"public_subject_snapshot,omitempty"`
}

type RouteLegRow struct {
	ID                   uuid.UUID `json:"id"`
	RoutePlanID          uuid.UUID `json:"route_plan_id"`
	Ordinal              int       `json:"ordinal"`
	FromStopID           uuid.UUID `json:"from_stop_id"`
	ToStopID             uuid.UUID `json:"to_stop_id"`
	FromPointFingerprint string    `json:"from_point_fingerprint"`
	ToPointFingerprint   string    `json:"to_point_fingerprint"`
	DistanceM            int       `json:"distance_m"`
	DurationSeconds      int       `json:"duration_seconds"`
	Provider             string    `json:"provider"`
	RequestFingerprint   string    `json:"request_fingerprint"`
	ResponseFingerprint  string    `json:"response_fingerprint"`
	VehicleProfileHash   string    `json:"vehicle_profile_hash"`
	RouteMode            string    `json:"route_mode"`
	TrafficMode          string    `json:"traffic_mode"`
	DepartureBucket      string    `json:"departure_bucket"`
	CalculatedAt         time.Time `json:"calculated_at"`
	ExpiresAt            time.Time `json:"expires_at"`
	ProviderDefaultUsed  bool      `json:"provider_default_used"`
}

type RouteSnapshotRow struct {
	ID                          uuid.UUID `json:"id"`
	RoutePlanID                 uuid.UUID `json:"route_plan_id"`
	SequenceOrdinal             int       `json:"sequence_ordinal"`
	AfterStopID                 uuid.UUID `json:"after_stop_id"`
	AfterActionOrdinal          int       `json:"after_action_ordinal"`
	PayloadStatus               string    `json:"payload_status"`
	PayloadRemainingKg          *float64  `json:"payload_remaining_kg,omitempty"`
	VolumeStatus                string    `json:"volume_status"`
	VolumeRemainingM3           *float64  `json:"volume_remaining_m3,omitempty"`
	PalletStatus                string    `json:"pallet_status"`
	PalletPositionsRemaining    *float64  `json:"pallet_positions_remaining,omitempty"`
	LinearStatus                string    `json:"linear_status"`
	LinearMetersRemaining       *float64  `json:"linear_meters_remaining,omitempty"`
	HeightStatus                string    `json:"height_status"`
	HeightRemainingMM           *float64  `json:"height_remaining_mm,omitempty"`
	TemperatureAllocationStatus string    `json:"temperature_allocation_status"`
	CompatibilityStatus         string    `json:"compatibility_status,omitempty"`
	CompatibilityFingerprint    string    `json:"compatibility_fingerprint,omitempty"`
	TemperatureCheckStatus      string    `json:"temperature_check_status,omitempty"`
	ADRCheckStatus              string    `json:"adr_check_status,omitempty"`
	FoodGradeCheckStatus        string    `json:"food_grade_check_status,omitempty"`
}

type RouteDependencyRow struct {
	ID             uuid.UUID  `json:"id"`
	RoutePlanID    uuid.UUID  `json:"route_plan_id"`
	DependencyKind string     `json:"dependency_kind"`
	SubjectID      *uuid.UUID `json:"subject_id,omitempty"`
	SubjectVersion *int       `json:"subject_version,omitempty"`
	Fingerprint    string     `json:"fingerprint,omitempty"`
}

type RoutePlanGraph struct {
	Plan         RoutePlanRow         `json:"plan"`
	Stops        []RouteStopRow       `json:"stops"`
	Actions      []RouteActionRow     `json:"actions"`
	Legs         []RouteLegRow        `json:"legs"`
	Snapshots    []RouteSnapshotRow   `json:"capacity_snapshots"`
	Dependencies []RouteDependencyRow `json:"dependencies"`
}

func cloneRoutePlans(in map[uuid.UUID]RoutePlanGraph) map[uuid.UUID]RoutePlanGraph {
	out := make(map[uuid.UUID]RoutePlanGraph, len(in))
	for id, graph := range in {
		graph.Stops = append([]RouteStopRow(nil), graph.Stops...)
		graph.Actions = append([]RouteActionRow(nil), graph.Actions...)
		for i := range graph.Actions {
			if graph.Actions[i].PublicSubjectSnapshot != nil {
				graph.Actions[i].PublicSubjectSnapshot = append(json.RawMessage(nil), graph.Actions[i].PublicSubjectSnapshot...)
			}
		}
		graph.Legs = append([]RouteLegRow(nil), graph.Legs...)
		graph.Snapshots = append([]RouteSnapshotRow(nil), graph.Snapshots...)
		graph.Dependencies = append([]RouteDependencyRow(nil), graph.Dependencies...)
		graph.Plan.ReasonCodes = append([]string(nil), graph.Plan.ReasonCodes...)
		out[id] = graph
	}
	return out
}

func (t *memTx) InsertRoutePlan(_ context.Context, graph RoutePlanGraph) error {
	if t.plans == nil {
		t.plans = map[uuid.UUID]RoutePlanGraph{}
	}
	if _, ok := t.plans[graph.Plan.ID]; ok {
		return ErrConflict
	}
	copied := cloneRoutePlans(map[uuid.UUID]RoutePlanGraph{graph.Plan.ID: graph})
	t.plans[graph.Plan.ID] = copied[graph.Plan.ID]
	return nil
}

func (t *memTx) GetRoutePlan(_ context.Context, tenant, id uuid.UUID) (RoutePlanGraph, error) {
	graph, ok := t.plans[id]
	if !ok || graph.Plan.TenantID != tenant {
		return RoutePlanGraph{}, ErrNotFound
	}
	copied := cloneRoutePlans(map[uuid.UUID]RoutePlanGraph{id: graph})
	return copied[id], nil
}

func (t *memTx) LockRoutePlan(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (t *memTx) MarkRoutePlanAccepted(_ context.Context, tenant, id uuid.UUID, version int, at time.Time) error {
	graph, ok := t.plans[id]
	if !ok || graph.Plan.TenantID != tenant {
		return ErrNotFound
	}
	if graph.Plan.Version != version {
		return ErrConflict
	}
	if graph.Plan.Status == "ACCEPTED" {
		return nil
	}
	if graph.Plan.Status != "EVALUATED" {
		return ErrConflict
	}
	stamp := at.UTC()
	graph.Plan.Status = "ACCEPTED"
	graph.Plan.AcceptedAt = &stamp
	t.plans[id] = graph
	return nil
}

func (t *memTx) MarkRoutePlanSuperseded(_ context.Context, tenant, id uuid.UUID, at time.Time) error {
	graph, ok := t.plans[id]
	if !ok || graph.Plan.TenantID != tenant {
		return ErrNotFound
	}
	if graph.Plan.Status == "SUPERSEDED" {
		return nil
	}
	if graph.Plan.Status != "EVALUATED" && graph.Plan.Status != "ACCEPTED" {
		return ErrConflict
	}
	stamp := at.UTC()
	graph.Plan.Status = "SUPERSEDED"
	graph.Plan.SupersededAt = &stamp
	t.plans[id] = graph
	return nil
}

func (t *memTx) InsertRoutePlanActivation(_ context.Context, row RoutePlanActivationRow) error {
	if t.activations == nil {
		t.activations = map[uuid.UUID]RoutePlanActivationRow{}
	}
	for _, existing := range t.activations {
		if existing.TenantID != row.TenantID {
			continue
		}
		if existing.RoutePlanID == row.RoutePlanID || existing.IdempotencyKey == row.IdempotencyKey {
			return ErrConflict
		}
		if row.EffectiveShipmentID != nil && existing.EffectiveShipmentID != nil && *existing.EffectiveShipmentID == *row.EffectiveShipmentID {
			return ErrConflict
		}
	}
	t.activations[row.RoutePlanID] = row
	return nil
}

func (t *memTx) ClearRoutePlanActivationEffect(_ context.Context, tenant, planID uuid.UUID) error {
	row, ok := t.activations[planID]
	if !ok || row.TenantID != tenant {
		return nil
	}
	row.EffectiveShipmentID = nil
	t.activations[planID] = row
	return nil
}

func (t *memTx) GetRoutePlanActivation(_ context.Context, tenant, planID uuid.UUID) (RoutePlanActivationRow, error) {
	row, ok := t.activations[planID]
	if !ok || row.TenantID != tenant {
		return RoutePlanActivationRow{}, ErrNotFound
	}
	return row, nil
}

func (t *memTx) LinkedActivationForShipment(_ context.Context, tenant, shipment uuid.UUID) (RoutePlanActivationRow, error) {
	for _, row := range t.activations {
		if row.TenantID != tenant || row.Status != "EXECUTION_LINKED" || row.EffectiveShipmentID == nil || *row.EffectiveShipmentID != shipment {
			continue
		}
		return row, nil
	}
	return RoutePlanActivationRow{}, ErrNotFound
}

func cloneActivations(in map[uuid.UUID]RoutePlanActivationRow) map[uuid.UUID]RoutePlanActivationRow {
	out := make(map[uuid.UUID]RoutePlanActivationRow, len(in))
	for id, row := range in {
		out[id] = row
	}
	return out
}

func (m *Memory) TestingReplaceRoutePlan(graph RoutePlanGraph) error {
	return m.Within(context.Background(), func(tx Tx) error {
		mem, ok := tx.(*memTx)
		if !ok {
			return ErrConflict
		}
		if mem.plans == nil {
			mem.plans = map[uuid.UUID]RoutePlanGraph{}
		}
		copied := cloneRoutePlans(map[uuid.UUID]RoutePlanGraph{graph.Plan.ID: graph})
		mem.plans[graph.Plan.ID] = copied[graph.Plan.ID]
		return nil
	})
}

func (m *Memory) TestingActivations() []RoutePlanActivationRow {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RoutePlanActivationRow, 0, len(m.routeActivations))
	for _, row := range m.routeActivations {
		out = append(out, row)
	}
	return out
}

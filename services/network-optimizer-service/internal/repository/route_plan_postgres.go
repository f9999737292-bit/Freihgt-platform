package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (t *pgTx) InsertRoutePlan(ctx context.Context, graph RoutePlanGraph) error {
	plan := graph.Plan
	if plan.ReasonCodes == nil {
		plan.ReasonCodes = []string{}
	}
	if _, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.route_plans (
			id, tenant_id, version, status, planning_mode, result_status,
			capacity_id, capacity_version, shipment_id, shipment_version, vehicle_id,
			context_fingerprint, evaluation_fingerprint, algorithm_policy_version, routing_policy_version,
			supersedes_plan_id, execution_supported, reason_codes, created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,$10,$11,
			$12,$13,$14,$15,
			$16,$17,$18,$19
		)`,
		plan.ID, plan.TenantID, plan.Version, plan.Status, plan.PlanningMode, plan.ResultStatus,
		plan.CapacityID, plan.CapacityVersion, plan.ShipmentID, plan.ShipmentVersion, plan.VehicleID,
		plan.ContextFingerprint, plan.EvaluationFingerprint, plan.AlgorithmPolicyVersion, plan.RoutingPolicyVersion,
		plan.SupersedesPlanID, plan.ExecutionSupported, plan.ReasonCodes, plan.CreatedAt,
	); err != nil {
		return err
	}
	for _, stop := range graph.Stops {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO network_optimizer.route_plan_stops (
				id, route_plan_id, ordinal, stop_role, point_kind, location_id, latitude, longitude,
				point_source, point_observed_at, planned_arrival, planned_departure, service_duration_seconds
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			stop.ID, stop.RoutePlanID, stop.Ordinal, stop.StopRole, stop.PointKind, stop.LocationID,
			stop.Latitude, stop.Longitude, stop.PointSource, stop.PointObservedAt, stop.PlannedArrival,
			stop.PlannedDeparture, stop.ServiceDurationSeconds,
		); err != nil {
			return err
		}
	}
	for _, action := range graph.Actions {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO network_optimizer.route_stop_actions (
				id, route_plan_id, stop_id, action_ordinal, action_type, subject_type, subject_id, subject_version,
				weight_delta_kg, volume_delta_m3, pallet_delta, linear_meters_delta, window_start, window_end,
				source_shipment_id, source_shipment_version, evidence_state, evidence_state_version, evidence_occurred_at,
				public_subject_snapshot
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
			action.ID, action.RoutePlanID, action.StopID, action.ActionOrdinal, action.ActionType, action.SubjectType,
			action.SubjectID, action.SubjectVersion, action.WeightDeltaKg, action.VolumeDeltaM3, action.PalletDelta,
			action.LinearMetersDelta, action.WindowStart, action.WindowEnd, action.SourceShipmentID,
			action.SourceShipmentVersion, nullString(action.EvidenceState), action.EvidenceStateVersion, action.EvidenceOccurredAt,
			nullRaw(action.PublicSubjectSnapshot),
		); err != nil {
			return err
		}
	}
	for _, leg := range graph.Legs {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO network_optimizer.route_plan_legs (
				id, route_plan_id, ordinal, from_stop_id, to_stop_id, from_point_fingerprint, to_point_fingerprint,
				distance_m, duration_seconds, provider, request_fingerprint, response_fingerprint,
				vehicle_profile_hash, route_mode, traffic_mode, departure_bucket, calculated_at, expires_at, provider_default_used
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			leg.ID, leg.RoutePlanID, leg.Ordinal, leg.FromStopID, leg.ToStopID, leg.FromPointFingerprint, leg.ToPointFingerprint,
			leg.DistanceM, leg.DurationSeconds, leg.Provider, leg.RequestFingerprint, leg.ResponseFingerprint,
			leg.VehicleProfileHash, leg.RouteMode, leg.TrafficMode, leg.DepartureBucket, leg.CalculatedAt, leg.ExpiresAt,
			leg.ProviderDefaultUsed,
		); err != nil {
			return err
		}
	}
	for _, snap := range graph.Snapshots {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO network_optimizer.route_capacity_snapshots (
				id, route_plan_id, sequence_ordinal, after_stop_id, after_action_ordinal,
				payload_status, payload_remaining_kg, volume_status, volume_remaining_m3,
				pallet_status, pallet_positions_remaining, linear_status, linear_meters_remaining,
				height_status, height_remaining_mm, temperature_allocation_status,
				compatibility_status, compatibility_fingerprint, temperature_check_status, adr_check_status, food_grade_check_status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
			snap.ID, snap.RoutePlanID, snap.SequenceOrdinal, snap.AfterStopID, snap.AfterActionOrdinal,
			snap.PayloadStatus, snap.PayloadRemainingKg, snap.VolumeStatus, snap.VolumeRemainingM3,
			snap.PalletStatus, snap.PalletPositionsRemaining, snap.LinearStatus, snap.LinearMetersRemaining,
			snap.HeightStatus, snap.HeightRemainingMM, snap.TemperatureAllocationStatus,
			nullString(snap.CompatibilityStatus), nullString(snap.CompatibilityFingerprint),
			nullString(snap.TemperatureCheckStatus), nullString(snap.ADRCheckStatus), nullString(snap.FoodGradeCheckStatus),
		); err != nil {
			return err
		}
	}
	for _, dep := range graph.Dependencies {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO network_optimizer.route_plan_dependencies (
				id, route_plan_id, dependency_kind, subject_id, subject_version, fingerprint
			) VALUES ($1,$2,$3,$4,$5,$6)`,
			dep.ID, dep.RoutePlanID, dep.DependencyKind, dep.SubjectID, dep.SubjectVersion, nullString(dep.Fingerprint),
		); err != nil {
			return err
		}
	}
	return nil
}

func (t *pgTx) GetRoutePlan(ctx context.Context, tenant, id uuid.UUID) (RoutePlanGraph, error) {
	var graph RoutePlanGraph
	err := t.tx.QueryRow(ctx, `
		SELECT id, tenant_id, version, status, planning_mode, result_status,
			capacity_id, capacity_version, shipment_id, shipment_version, vehicle_id,
			context_fingerprint, evaluation_fingerprint, algorithm_policy_version, routing_policy_version,
			supersedes_plan_id, execution_supported, reason_codes, created_at
		FROM network_optimizer.route_plans
		WHERE id=$1 AND tenant_id=$2`, id, tenant).Scan(
		&graph.Plan.ID, &graph.Plan.TenantID, &graph.Plan.Version, &graph.Plan.Status, &graph.Plan.PlanningMode, &graph.Plan.ResultStatus,
		&graph.Plan.CapacityID, &graph.Plan.CapacityVersion, &graph.Plan.ShipmentID, &graph.Plan.ShipmentVersion, &graph.Plan.VehicleID,
		&graph.Plan.ContextFingerprint, &graph.Plan.EvaluationFingerprint, &graph.Plan.AlgorithmPolicyVersion, &graph.Plan.RoutingPolicyVersion,
		&graph.Plan.SupersedesPlanID, &graph.Plan.ExecutionSupported, &graph.Plan.ReasonCodes, &graph.Plan.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return RoutePlanGraph{}, ErrNotFound
		}
		return RoutePlanGraph{}, err
	}
	stops, err := t.tx.Query(ctx, `
		SELECT id, route_plan_id, ordinal, stop_role, point_kind, location_id, latitude, longitude,
			point_source, point_observed_at, planned_arrival, planned_departure, service_duration_seconds
		FROM network_optimizer.route_plan_stops WHERE route_plan_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return RoutePlanGraph{}, err
	}
	defer stops.Close()
	for stops.Next() {
		var stop RouteStopRow
		if err := stops.Scan(&stop.ID, &stop.RoutePlanID, &stop.Ordinal, &stop.StopRole, &stop.PointKind, &stop.LocationID,
			&stop.Latitude, &stop.Longitude, &stop.PointSource, &stop.PointObservedAt, &stop.PlannedArrival,
			&stop.PlannedDeparture, &stop.ServiceDurationSeconds); err != nil {
			return RoutePlanGraph{}, err
		}
		graph.Stops = append(graph.Stops, stop)
	}
	actions, err := t.tx.Query(ctx, `
		SELECT id, route_plan_id, stop_id, action_ordinal, action_type, subject_type, subject_id, subject_version,
			weight_delta_kg, volume_delta_m3, pallet_delta, linear_meters_delta, window_start, window_end,
			source_shipment_id, source_shipment_version, evidence_state, evidence_state_version, evidence_occurred_at,
			public_subject_snapshot
		FROM network_optimizer.route_stop_actions WHERE route_plan_id=$1 ORDER BY stop_id, action_ordinal`, id)
	if err != nil {
		return RoutePlanGraph{}, err
	}
	defer actions.Close()
	for actions.Next() {
		var action RouteActionRow
		var evidence *string
		if err := actions.Scan(&action.ID, &action.RoutePlanID, &action.StopID, &action.ActionOrdinal, &action.ActionType,
			&action.SubjectType, &action.SubjectID, &action.SubjectVersion, &action.WeightDeltaKg, &action.VolumeDeltaM3,
			&action.PalletDelta, &action.LinearMetersDelta, &action.WindowStart, &action.WindowEnd, &action.SourceShipmentID,
			&action.SourceShipmentVersion, &evidence, &action.EvidenceStateVersion, &action.EvidenceOccurredAt,
			&action.PublicSubjectSnapshot); err != nil {
			return RoutePlanGraph{}, err
		}
		if evidence != nil {
			action.EvidenceState = *evidence
		}
		graph.Actions = append(graph.Actions, action)
	}
	legs, err := t.tx.Query(ctx, `
		SELECT id, route_plan_id, ordinal, from_stop_id, to_stop_id, from_point_fingerprint, to_point_fingerprint,
			distance_m, duration_seconds, provider, request_fingerprint, response_fingerprint,
			vehicle_profile_hash, route_mode, traffic_mode, departure_bucket, calculated_at, expires_at, provider_default_used
		FROM network_optimizer.route_plan_legs WHERE route_plan_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return RoutePlanGraph{}, err
	}
	defer legs.Close()
	for legs.Next() {
		var leg RouteLegRow
		if err := legs.Scan(&leg.ID, &leg.RoutePlanID, &leg.Ordinal, &leg.FromStopID, &leg.ToStopID, &leg.FromPointFingerprint,
			&leg.ToPointFingerprint, &leg.DistanceM, &leg.DurationSeconds, &leg.Provider, &leg.RequestFingerprint,
			&leg.ResponseFingerprint, &leg.VehicleProfileHash, &leg.RouteMode, &leg.TrafficMode, &leg.DepartureBucket,
			&leg.CalculatedAt, &leg.ExpiresAt, &leg.ProviderDefaultUsed); err != nil {
			return RoutePlanGraph{}, err
		}
		graph.Legs = append(graph.Legs, leg)
	}
	snaps, err := t.tx.Query(ctx, `
		SELECT id, route_plan_id, sequence_ordinal, after_stop_id, after_action_ordinal,
			payload_status, payload_remaining_kg, volume_status, volume_remaining_m3,
			pallet_status, pallet_positions_remaining, linear_status, linear_meters_remaining,
			height_status, height_remaining_mm, temperature_allocation_status,
			compatibility_status, compatibility_fingerprint, temperature_check_status, adr_check_status, food_grade_check_status
		FROM network_optimizer.route_capacity_snapshots WHERE route_plan_id=$1 ORDER BY sequence_ordinal`, id)
	if err != nil {
		return RoutePlanGraph{}, err
	}
	defer snaps.Close()
	for snaps.Next() {
		var snap RouteSnapshotRow
		var compatibility, compatibilityFP, temperatureCheck, adrCheck, foodCheck *string
		if err := snaps.Scan(&snap.ID, &snap.RoutePlanID, &snap.SequenceOrdinal, &snap.AfterStopID, &snap.AfterActionOrdinal,
			&snap.PayloadStatus, &snap.PayloadRemainingKg, &snap.VolumeStatus, &snap.VolumeRemainingM3,
			&snap.PalletStatus, &snap.PalletPositionsRemaining, &snap.LinearStatus, &snap.LinearMetersRemaining,
			&snap.HeightStatus, &snap.HeightRemainingMM, &snap.TemperatureAllocationStatus,
			&compatibility, &compatibilityFP, &temperatureCheck, &adrCheck, &foodCheck); err != nil {
			return RoutePlanGraph{}, err
		}
		snap.CompatibilityStatus = stringValue(compatibility)
		snap.CompatibilityFingerprint = stringValue(compatibilityFP)
		snap.TemperatureCheckStatus = stringValue(temperatureCheck)
		snap.ADRCheckStatus = stringValue(adrCheck)
		snap.FoodGradeCheckStatus = stringValue(foodCheck)
		graph.Snapshots = append(graph.Snapshots, snap)
	}
	deps, err := t.tx.Query(ctx, `
		SELECT id, route_plan_id, dependency_kind, subject_id, subject_version, fingerprint
		FROM network_optimizer.route_plan_dependencies WHERE route_plan_id=$1 ORDER BY dependency_kind, subject_id`, id)
	if err != nil {
		return RoutePlanGraph{}, err
	}
	defer deps.Close()
	for deps.Next() {
		var dep RouteDependencyRow
		var fingerprint *string
		if err := deps.Scan(&dep.ID, &dep.RoutePlanID, &dep.DependencyKind, &dep.SubjectID, &dep.SubjectVersion, &fingerprint); err != nil {
			return RoutePlanGraph{}, err
		}
		if fingerprint != nil {
			dep.Fingerprint = *fingerprint
		}
		graph.Dependencies = append(graph.Dependencies, dep)
	}
	return graph, nil
}

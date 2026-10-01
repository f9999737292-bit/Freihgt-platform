package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type TransportExecutionRepository struct {
	pool *pgxpool.Pool
}

func NewTransportExecutionRepository(pool *pgxpool.Pool) *TransportExecutionRepository {
	return &TransportExecutionRepository{pool: pool}
}

func (r *TransportExecutionRepository) Project(ctx context.Context, cmd domain.ProjectionCommand) (domain.ProjectionResult, error) {
	digest, err := domain.CanonicalDigest(cmd)
	if err != nil {
		return domain.ProjectionResult{}, apperrors.Internal("projection digest failed", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ProjectionResult{}, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	storedID, storedExecution, storedDigest, found, err := findActivation(ctx, tx, cmd.OperatingTenantID, cmd.ActivationID)
	if err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := domain.RejectReplayOrCreate(cmd, storedDigest, found, digest); err != nil {
		return domain.ProjectionResult{}, err
	}
	if found {
		return domain.ProjectionResult{
			ExecutionID:  storedExecution,
			RevisionID:   storedID,
			ActivationID: cmd.ActivationID,
			Created:      false,
		}, nil
	}

	subjects, err := domainSubjects(cmd)
	if err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := verifyMaterializedRows(ctx, tx, subjects); err != nil {
		return domain.ProjectionResult{}, err
	}

	now := time.Now().UTC()
	executionID := uuid.New()
	revisionID := uuid.New()
	if err := insertExecution(ctx, tx, executionID, cmd, now); err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := insertRevision(ctx, tx, revisionID, executionID, cmd, digest, now); err != nil {
		if constraintName(err) == "transport_execution_revisions_activation_uq" {
			return r.replayAfterConflict(ctx, cmd, digest)
		}
		return domain.ProjectionResult{}, mapDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transport.transport_executions
		SET current_revision_id = $2, updated_at = $3
		WHERE id = $1 AND operating_tenant_id = $4
	`, executionID, revisionID, now, cmd.OperatingTenantID); err != nil {
		return domain.ProjectionResult{}, mapDBError(err)
	}
	if err := insertParticipants(ctx, tx, executionID, subjects); err != nil {
		if constraintName(err) == "transport_execution_active_shipments_pkey" {
			return domain.ProjectionResult{}, domain.ProjectionConflict(domain.ReasonExecutionPlanConflict)
		}
		return domain.ProjectionResult{}, mapDBError(err)
	}
	stopIDs, err := insertStops(ctx, tx, executionID, revisionID, cmd, now)
	if err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := insertActions(ctx, tx, revisionID, cmd, stopIDs); err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := materializeDriverStopTasksFn(ctx, tx, executionID, now); err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := emitExecutionPlanCreated(ctx, tx, cmd.OperatingTenantID, executionID, revisionID, now, ""); err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := emitInitialCurrentStop(ctx, tx, cmd.OperatingTenantID, executionID, revisionID, now); err != nil {
		return domain.ProjectionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		if constraintName(err) == "transport_execution_revisions_activation_uq" {
			return r.replayAfterConflict(ctx, cmd, digest)
		}
		return domain.ProjectionResult{}, mapDBError(err)
	}
	return domain.ProjectionResult{
		ExecutionID:  executionID,
		RevisionID:   revisionID,
		ActivationID: cmd.ActivationID,
		Created:      true,
	}, nil
}

func (r *TransportExecutionRepository) replayAfterConflict(ctx context.Context, cmd domain.ProjectionCommand, digest string) (domain.ProjectionResult, error) {
	storedID, storedExecution, storedDigest, found, err := findActivation(ctx, r.pool, cmd.OperatingTenantID, cmd.ActivationID)
	if err != nil {
		return domain.ProjectionResult{}, err
	}
	if !found {
		return domain.ProjectionResult{}, domain.ProjectionConflict(domain.ReasonActivationBodyConflict)
	}
	if storedDigest != digest {
		return domain.ProjectionResult{}, domain.ProjectionConflict(domain.ReasonActivationBodyConflict)
	}
	return domain.ProjectionResult{
		ExecutionID:  storedExecution,
		RevisionID:   storedID,
		ActivationID: cmd.ActivationID,
		Created:      false,
	}, nil
}

type rowQuery interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func findActivation(ctx context.Context, q rowQuery, operatingTenantID, activationID uuid.UUID) (revisionID, executionID uuid.UUID, digest string, found bool, err error) {
	err = q.QueryRow(ctx, `
		SELECT id, execution_id, contract_sha256
		FROM transport.transport_execution_revisions
		WHERE operating_tenant_id = $1 AND source_activation_id = $2
	`, operatingTenantID, activationID).Scan(&revisionID, &executionID, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, "", false, nil
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, "", false, mapDBError(err)
	}
	return revisionID, executionID, digest, true, nil
}

func domainSubjects(cmd domain.ProjectionCommand) ([]domain.MaterializedSubject, error) {
	if err := domain.ValidateNewProjection(cmd); err != nil {
		return nil, err
	}
	subjects := make([]domain.MaterializedSubject, 0, len(cmd.ExecutionSubjects))
	for _, row := range cmd.ExecutionSubjects {
		subjects = append(subjects, domain.MaterializedSubject{
			RouteSubjectType: row.RouteSubjectType,
			RouteSubjectID:   row.RouteSubjectID,
			ShipmentID:       *row.ExecutionShipmentID,
			ShipmentTenantID: *row.ShipmentTenantID,
			ShipmentVersion:  *row.ExecutionShipmentVersion,
			CargoID:          *row.CargoID,
			CargoVersion:     *row.CargoVersion,
		})
	}
	return subjects, nil
}

func verifyMaterializedRows(ctx context.Context, tx pgx.Tx, subjects []domain.MaterializedSubject) error {
	for _, subject := range subjects {
		var shipmentVersion int
		err := tx.QueryRow(ctx, `
			SELECT version
			FROM transport.shipments
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, subject.ShipmentID, subject.ShipmentTenantID).Scan(&shipmentVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProjectionConflict(domain.ReasonExecutionSubjectUnmaterialized)
		}
		if err != nil {
			return mapDBError(err)
		}
		if shipmentVersion != subject.ShipmentVersion {
			return domain.ProjectionConflict(domain.ReasonPlanStale)
		}
		var cargoVersion int
		err = tx.QueryRow(ctx, `
			SELECT version
			FROM transport.cargoes
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, subject.CargoID, subject.ShipmentTenantID).Scan(&cargoVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProjectionConflict(domain.ReasonExecutionSubjectUnmaterialized)
		}
		if err != nil {
			return mapDBError(err)
		}
		if cargoVersion != subject.CargoVersion {
			return domain.ProjectionConflict(domain.ReasonPlanStale)
		}
	}
	return nil
}

func insertExecution(ctx context.Context, tx pgx.Tx, executionID uuid.UUID, cmd domain.ProjectionCommand, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_executions (
			id, operating_tenant_id, carrier_company_id, vehicle_id, driver_id,
			current_revision_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,NULL,$6,$6)
	`, executionID, cmd.OperatingTenantID, cmd.CarrierCompanyID, cmd.VehicleID, cmd.DriverID, now)
	return mapDBError(err)
}

func insertRevision(ctx context.Context, tx pgx.Tx, revisionID, executionID uuid.UUID, cmd domain.ProjectionCommand, digest string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			supersedes_revision_id, status, version, contract_sha256, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,$10,1,$11,$12,$12)
	`, revisionID, executionID, cmd.OperatingTenantID, cmd.RoutePlanID, cmd.RoutePlanVersion,
		cmd.ActivationID, cmd.ActivationVersion, cmd.EvaluationFingerprint, cmd.PlanningMode,
		domain.RevisionStatusActive, digest, now)
	return err
}

func insertParticipants(ctx context.Context, tx pgx.Tx, executionID uuid.UUID, subjects []domain.MaterializedSubject) error {
	seenShipment := map[string]struct{}{}
	for _, subject := range subjects {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_participants (
				execution_id, shipment_id, shipment_tenant_id, shipment_version,
				cargo_id, cargo_version, route_subject_type, route_subject_id, provenance
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, executionID, subject.ShipmentID, subject.ShipmentTenantID, subject.ShipmentVersion,
			subject.CargoID, subject.CargoVersion, subject.RouteSubjectType, subject.RouteSubjectID,
			domain.ParticipantProvenanceTrustedProjection); err != nil {
			return err
		}
		key := subject.ShipmentTenantID.String() + ":" + subject.ShipmentID.String()
		if _, ok := seenShipment[key]; ok {
			continue
		}
		seenShipment[key] = struct{}{}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_active_shipments (
				shipment_tenant_id, shipment_id, execution_id
			) VALUES ($1,$2,$3)
		`, subject.ShipmentTenantID, subject.ShipmentID, executionID); err != nil {
			return err
		}
	}
	return nil
}

func insertStops(ctx context.Context, tx pgx.Tx, executionID, revisionID uuid.UUID, cmd domain.ProjectionCommand, now time.Time) (map[uuid.UUID]uuid.UUID, error) {
	stopIDs := make(map[uuid.UUID]uuid.UUID, len(cmd.Stops))
	for _, stop := range cmd.Stops {
		stopID := uuid.New()
		stopIDs[stop.RoutePlanStopID] = stopID
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_stops (
				id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind, location_id,
				latitude, longitude, planned_arrival, planned_departure, service_duration_seconds,
				status, status_reason, version, arrived_at, service_started_at, completed_at,
				created_at, updated_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULL,1,NULL,NULL,NULL,$14,$14
			)
		`, stopID, executionID, cmd.OperatingTenantID, stop.Ordinal, stop.StopRole, stop.PointKind, stop.LocationID,
			stop.Latitude, stop.Longitude, stop.PlannedArrival, stop.PlannedDeparture, stop.ServiceDurationSeconds,
			domain.StopStatusPlanned, now); err != nil {
			return nil, mapDBError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_revision_stops (
				revision_id, stop_id, source_route_plan_stop_id, source_ordinal, membership
			) VALUES ($1,$2,$3,$4,$5)
		`, revisionID, stopID, stop.RoutePlanStopID, stop.Ordinal, domain.MembershipIntroduced); err != nil {
			return nil, mapDBError(err)
		}
	}
	return stopIDs, nil
}

func insertActions(ctx context.Context, tx pgx.Tx, revisionID uuid.UUID, cmd domain.ProjectionCommand, stopIDs map[uuid.UUID]uuid.UUID) error {
	for _, action := range cmd.Actions {
		actionID := uuid.New()
		stopID := stopIDs[action.RoutePlanStopID]
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_actions (
				id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, cargo_version,
				route_subject_type, route_subject_id, action_type, ordinal, status,
				evidence_id, completed_at, version
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULL,NULL,1)
		`, actionID, stopID, *action.ExecutionShipmentID, *action.ShipmentTenantID, *action.CargoID, *action.CargoVersion,
			action.RouteSubjectType, action.RouteSubjectID, action.ActionType, action.ActionOrdinal, domain.ActionStatusPending); err != nil {
			return mapDBError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transport.transport_execution_revision_actions (
				revision_id, action_id, source_route_plan_action_id, source_action_ordinal, membership
			) VALUES ($1,$2,$3,$4,$5)
		`, revisionID, actionID, action.RoutePlanActionID, action.ActionOrdinal, domain.MembershipIntroduced); err != nil {
			return mapDBError(err)
		}
	}
	return nil
}

func (r *TransportExecutionRepository) GetTrackingContext(ctx context.Context, operatingTenantID, executionID, stopID uuid.UUID) (domain.TrackingStopContext, error) {
	const currentSQL = `
		SELECT e.id, r.id, s.id, rs.source_ordinal, s.status, s.planned_arrival, s.location_id,
		       CASE
		           WHEN s.point_kind = 'CANONICAL_LOCATION' AND loc.lat IS NOT NULL AND loc.lon IS NOT NULL THEN loc.lat
		           WHEN s.point_kind = 'POSITION_ANCHOR' THEN s.latitude
		           ELSE NULL
		       END,
		       CASE
		           WHEN s.point_kind = 'CANONICAL_LOCATION' AND loc.lat IS NOT NULL AND loc.lon IS NOT NULL THEN loc.lon
		           WHEN s.point_kind = 'POSITION_ANCHOR' THEN s.longitude
		           ELSE NULL
		       END,
		       e.driver_id, e.vehicle_id, s.point_kind, s.stop_role
		FROM transport.transport_executions e
		JOIN transport.transport_execution_revisions r
		  ON r.id = e.current_revision_id AND r.status = 'ACTIVE' AND r.execution_id = e.id
		JOIN transport.transport_execution_revision_stops rs
		  ON rs.revision_id = r.id AND rs.stop_id = $3 AND rs.membership <> 'SUPERSEDED'
		JOIN transport.transport_execution_stops s
		  ON s.id = rs.stop_id AND s.execution_id = e.id AND s.operating_tenant_id = e.operating_tenant_id
		LEFT JOIN transport.locations loc ON loc.id = s.location_id AND loc.deleted_at IS NULL
		WHERE e.id = $1 AND e.operating_tenant_id = $2
		  AND rs.source_ordinal = (
		      SELECT MIN(rs2.source_ordinal)
		      FROM transport.transport_execution_revision_stops rs2
		      JOIN transport.transport_execution_stops s2 ON s2.id = rs2.stop_id
		      WHERE rs2.revision_id = r.id
		        AND rs2.membership <> 'SUPERSEDED'
		        AND s2.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		  )`
	var view domain.TrackingStopContext
	err := r.pool.QueryRow(ctx, currentSQL, executionID, operatingTenantID, stopID).Scan(
		&view.ExecutionID, &view.RevisionID, &view.ExecutionStopID, &view.Ordinal, &view.Status,
		&view.PlannedArrival, &view.LocationID, &view.TargetLatitude, &view.TargetLongitude,
		&view.DriverID, &view.VehicleID, &view.PointKind, &view.StopRole,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TrackingStopContext{}, apperrors.NotFound("tracking context not found")
	}
	if err != nil {
		return domain.TrackingStopContext{}, mapDBError(err)
	}
	const liveSQL = `
		SELECT s.id, rs.source_ordinal, s.planned_arrival, s.location_id,
		       CASE WHEN loc.lat IS NOT NULL AND loc.lon IS NOT NULL THEN loc.lat END,
		       CASE WHEN loc.lat IS NOT NULL AND loc.lon IS NOT NULL THEN loc.lon END
		FROM transport.transport_execution_revision_stops rs
		JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
		LEFT JOIN transport.locations loc ON loc.id = s.location_id AND loc.deleted_at IS NULL
		WHERE rs.revision_id = $1
		  AND rs.membership <> 'SUPERSEDED'
		  AND s.point_kind = 'CANONICAL_LOCATION'
		  AND s.stop_role <> 'START'
		  AND s.status IN ('PLANNED', 'ARRIVED', 'SERVICE_STARTED')
		  AND rs.source_ordinal >= $2
		ORDER BY rs.source_ordinal
		LIMIT 1`
	var liveOrdinal int
	err = r.pool.QueryRow(ctx, liveSQL, view.RevisionID, view.Ordinal).Scan(
		&view.LiveETAStopID, &liveOrdinal, &view.LiveETAPlannedArrival, &view.LiveETALocationID,
		&view.LiveETALatitude, &view.LiveETALongitude,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return view, nil
	}
	if err != nil {
		return domain.TrackingStopContext{}, mapDBError(err)
	}
	view.LiveETAOrdinal = &liveOrdinal
	return view, nil
}

// ResolvedDriverExecutionContext is the server-owned stop and, when requested, action.
// Identifiers are copied from TransportExecution rows, not from the caller.
type ResolvedDriverExecutionContext struct {
	ExecutionStopID uuid.UUID
	ActionID        *uuid.UUID
}

// ResolveDriverExecutionContext proves a driver delay or problem context before publication.
// A missing row means the supplied identifier is not owned by this operating tenant, driver,
// shipment, current revision, or stop. Callers must reject that result and must not publish it.
func (r *TransportExecutionRepository) ResolveDriverExecutionContext(
	ctx context.Context,
	operatingTenantID, driverID, shipmentID uuid.UUID,
	executionStopID, actionID *uuid.UUID,
) (ResolvedDriverExecutionContext, error) {
	if executionStopID == nil && actionID == nil {
		return ResolvedDriverExecutionContext{}, apperrors.Validation("execution context is required", map[string]any{"field": "executionStopId"})
	}
	stopFilter := uuid.Nil
	if executionStopID != nil {
		stopFilter = *executionStopID
	}
	actionFilter := uuid.Nil
	if actionID != nil {
		actionFilter = *actionID
	}
	const q = `
		SELECT s.id, a.id
		FROM transport.transport_executions e
		JOIN transport.transport_execution_revisions rev
		  ON rev.id = e.current_revision_id
		 AND rev.execution_id = e.id
		 AND rev.operating_tenant_id = e.operating_tenant_id
		 AND rev.status = 'ACTIVE'
		JOIN transport.transport_execution_revision_stops rs
		  ON rs.revision_id = rev.id
		 AND rs.membership <> 'SUPERSEDED'
		JOIN transport.transport_execution_stops s
		  ON s.id = rs.stop_id
		 AND s.execution_id = e.id
		 AND s.operating_tenant_id = e.operating_tenant_id
		JOIN transport.transport_execution_participants p
		  ON p.execution_id = e.id
		 AND p.shipment_id = $3
		 AND p.shipment_tenant_id = $1
		JOIN transport.transport_execution_actions a
		  ON a.execution_stop_id = s.id
		 AND a.shipment_id = $3
		 AND a.shipment_tenant_id = $1
		JOIN transport.transport_execution_revision_actions ra
		  ON ra.revision_id = rev.id
		 AND ra.action_id = a.id
		 AND ra.membership <> 'SUPERSEDED'
		WHERE e.operating_tenant_id = $1
		  AND e.driver_id = $2
		  AND ($4::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR s.id = $4::uuid)
		  AND ($5::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR a.id = $5::uuid)
		LIMIT 1`
	var stopID, matchedActionID uuid.UUID
	err := r.pool.QueryRow(ctx, q, operatingTenantID, driverID, shipmentID, stopFilter, actionFilter).Scan(&stopID, &matchedActionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResolvedDriverExecutionContext{}, apperrors.NotFound("execution context not found")
	}
	if err != nil {
		return ResolvedDriverExecutionContext{}, mapDBError(err)
	}
	resolved := ResolvedDriverExecutionContext{ExecutionStopID: stopID}
	if actionID != nil {
		verifiedAction := matchedActionID
		resolved.ActionID = &verifiedAction
	}
	return resolved, nil
}

func constraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

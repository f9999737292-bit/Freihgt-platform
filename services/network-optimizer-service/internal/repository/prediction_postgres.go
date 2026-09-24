package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/google/uuid"
)

const predictionColumns = `
id, capacity_id, owner_tenant_id, shipment_id, shipment_version, shipment_status,
vehicle_id, vehicle_version, destination_location_id, destination_latitude, destination_longitude,
eta, eta_observed_at, delivery_window_start, delivery_window_end,
predicted_available_at, availability_window_start, availability_window_end,
unload_duration_seconds, unload_policy_source, uncertainty_seconds, confidence,
prediction_method, input_fingerprint, rule_version, generated_at, supersedes_prediction_id, is_current,
combination_type, body_type, loading_access, unloading_access,
capacity_weight_kg, capacity_volume_m3, temperature_control_mode,
temperature_capability_min_c, temperature_capability_max_c, temperature_zone_count,
independent_temperature_control, current_temperature_setpoint_c, legacy_equipment_type, container_size,
version, created_at`

func (t *pgTx) InsertPrediction(ctx context.Context, p domain.PredictedCapacity) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.predicted_capacities (`+predictionColumns+`)
		VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,
			$39,$40,$41,$42,$43,$44
		)`,
		p.ID, p.CapacityID, p.OwnerTenantID, p.ShipmentID, p.ShipmentVersion, p.ShipmentStatus,
		p.VehicleID, p.VehicleVersion, p.DestinationLocationID, p.DestinationLatitude, p.DestinationLongitude,
		p.ETA, p.ETAObservedAt, p.DeliveryWindowStart, p.DeliveryWindowEnd,
		p.PredictedAvailableAt, p.AvailabilityWindowStart, p.AvailabilityWindowEnd,
		p.UnloadDurationSeconds, p.UnloadPolicySource, p.UncertaintySeconds, p.Confidence,
		p.PredictionMethod, p.InputFingerprint, p.RuleVersion, p.GeneratedAt, p.SupersedesPredictionID, p.IsCurrent,
		p.CombinationType, p.BodyType, p.LoadingAccess, p.UnloadingAccess,
		p.CapacityWeightKg, p.CapacityVolumeM3, p.TemperatureControlMode,
		p.TemperatureCapabilityMinC, p.TemperatureCapabilityMaxC, p.TemperatureZoneCount,
		p.IndependentTemperatureControl, p.CurrentTemperatureSetpointC, p.LegacyEquipmentType, p.ContainerSize,
		p.Version, p.CreatedAt,
	)
	if isConstraint(err, "predicted_capacities_one_current_uidx") {
		return ErrConflict
	}
	return err
}

func (t *pgTx) UpdatePrediction(ctx context.Context, p domain.PredictedCapacity) error {
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.predicted_capacities SET is_current=$2 WHERE id=$1`,
		p.ID, p.IsCurrent)
	if err != nil {
		if isConstraint(err, "predicted_capacities_one_current_uidx") {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (t *pgTx) GetPrediction(ctx context.Context, id uuid.UUID) (domain.PredictedCapacity, error) {
	return scanPrediction(t.tx.QueryRow(ctx, `SELECT `+predictionColumns+` FROM network_optimizer.predicted_capacities WHERE id=$1`, id))
}

func (t *pgTx) CurrentPredictionByShipment(ctx context.Context, tenant, shipment uuid.UUID) (domain.PredictedCapacity, error) {
	return scanPrediction(t.tx.QueryRow(ctx, `
		SELECT `+predictionColumns+` FROM network_optimizer.predicted_capacities
		WHERE owner_tenant_id=$1 AND shipment_id=$2 AND is_current`, tenant, shipment))
}

func (t *pgTx) ListOwnPredictions(ctx context.Context, tenant uuid.UUID, limit, offset int) ([]domain.PredictedCapacity, error) {
	rows, err := t.tx.Query(ctx, `
		SELECT `+predictionColumns+` FROM network_optimizer.predicted_capacities
		WHERE owner_tenant_id=$1
		ORDER BY generated_at DESC, id
		LIMIT $2 OFFSET $3`, tenant, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.PredictedCapacity, 0)
	for rows.Next() {
		prediction, err := scanPrediction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, prediction)
	}
	return out, rows.Err()
}

func scanPrediction(row pgx.Row) (domain.PredictedCapacity, error) {
	var p domain.PredictedCapacity
	err := row.Scan(
		&p.ID, &p.CapacityID, &p.OwnerTenantID, &p.ShipmentID, &p.ShipmentVersion, &p.ShipmentStatus,
		&p.VehicleID, &p.VehicleVersion, &p.DestinationLocationID, &p.DestinationLatitude, &p.DestinationLongitude,
		&p.ETA, &p.ETAObservedAt, &p.DeliveryWindowStart, &p.DeliveryWindowEnd,
		&p.PredictedAvailableAt, &p.AvailabilityWindowStart, &p.AvailabilityWindowEnd,
		&p.UnloadDurationSeconds, &p.UnloadPolicySource, &p.UncertaintySeconds, &p.Confidence,
		&p.PredictionMethod, &p.InputFingerprint, &p.RuleVersion, &p.GeneratedAt, &p.SupersedesPredictionID, &p.IsCurrent,
		&p.CombinationType, &p.BodyType, &p.LoadingAccess, &p.UnloadingAccess,
		&p.CapacityWeightKg, &p.CapacityVolumeM3, &p.TemperatureControlMode,
		&p.TemperatureCapabilityMinC, &p.TemperatureCapabilityMaxC, &p.TemperatureZoneCount,
		&p.IndependentTemperatureControl, &p.CurrentTemperatureSetpointC, &p.LegacyEquipmentType, &p.ContainerSize,
		&p.Version, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PredictedCapacity{}, ErrNotFound
	}
	if err != nil {
		return domain.PredictedCapacity{}, err
	}
	p.CapacitySemantics = domain.CapacitySemanticsNextLoad
	return p, nil
}

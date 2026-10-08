package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// QueryRower is the read surface used for the analytics source snapshot.
type QueryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// OperationsAnalyticsSourceStatement is one PostgreSQL statement. Both
// aggregates share that statement snapshot. It does not write.
const OperationsAnalyticsSourceStatement = `
WITH shipment_counts AS (
    SELECT
        count(*) FILTER (WHERE deleted_at IS NULL) AS shipment_total,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_pickup_at IS NOT NULL
              AND actual_pickup_at IS NOT NULL
        ) AS on_time_pickup_denominator,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_pickup_at IS NOT NULL
              AND actual_pickup_at IS NOT NULL
              AND actual_pickup_at <= planned_pickup_at
        ) AS on_time_pickup_numerator,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_delivery_at IS NOT NULL
              AND actual_delivery_at IS NOT NULL
        ) AS on_time_delivery_denominator,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_delivery_at IS NOT NULL
              AND actual_delivery_at IS NOT NULL
              AND actual_delivery_at <= planned_delivery_at
        ) AS on_time_delivery_numerator
    FROM transport.shipments
    WHERE tenant_id = $1
),
case_counts AS (
    SELECT
        count(*) FILTER (WHERE disposition_type = 'RETURN_TO_ORIGIN') AS return_case_count,
        count(*) FILTER (WHERE disposition_type = 'REDIRECT') AS redirect_case_count
    FROM transport.delivery_disposition_cases
    WHERE operating_tenant_id = $1
),
carrier_counts AS (
    SELECT
        carrier_company_id,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_pickup_at IS NOT NULL
              AND actual_pickup_at IS NOT NULL
        ) AS on_time_pickup_denominator,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_pickup_at IS NOT NULL
              AND actual_pickup_at IS NOT NULL
              AND actual_pickup_at <= planned_pickup_at
        ) AS on_time_pickup_numerator,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_delivery_at IS NOT NULL
              AND actual_delivery_at IS NOT NULL
        ) AS on_time_delivery_denominator,
        count(*) FILTER (
            WHERE deleted_at IS NULL
              AND planned_delivery_at IS NOT NULL
              AND actual_delivery_at IS NOT NULL
              AND actual_delivery_at <= planned_delivery_at
        ) AS on_time_delivery_numerator
    FROM transport.shipments
    WHERE tenant_id = $1
      AND deleted_at IS NULL
      AND carrier_company_id IS NOT NULL
    GROUP BY carrier_company_id
),
carrier_rows AS (
    SELECT COALESCE(
        jsonb_agg(
            jsonb_build_object(
                'carrierCompanyId', carrier_company_id,
                'onTimePickupDenominator', on_time_pickup_denominator,
                'onTimePickupNumerator', on_time_pickup_numerator,
                'onTimeDeliveryDenominator', on_time_delivery_denominator,
                'onTimeDeliveryNumerator', on_time_delivery_numerator
            )
            ORDER BY carrier_company_id
        ),
        '[]'::jsonb
    ) AS carriers
    FROM carrier_counts
)
SELECT
    shipment_counts.shipment_total,
    shipment_counts.on_time_pickup_denominator,
    shipment_counts.on_time_pickup_numerator,
    shipment_counts.on_time_delivery_denominator,
    shipment_counts.on_time_delivery_numerator,
    case_counts.return_case_count,
    case_counts.redirect_case_count,
    carrier_rows.carriers
FROM shipment_counts
CROSS JOIN case_counts
CROSS JOIN carrier_rows
`

type OperationsAnalyticsSourceRepository struct {
	db QueryRower
}

func NewOperationsAnalyticsSourceRepository(db QueryRower) *OperationsAnalyticsSourceRepository {
	return &OperationsAnalyticsSourceRepository{db: db}
}

func (r *OperationsAnalyticsSourceRepository) OperationsFoundation(ctx context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error) {
	var snap domain.OperationsAnalyticsSourceSnapshot
	if r == nil || r.db == nil {
		return snap, apperrors.Internal("analytics source is not configured", nil)
	}
	var carriersJSON []byte
	err := r.db.QueryRow(ctx, OperationsAnalyticsSourceStatement, tenantID).Scan(
		&snap.ShipmentTotal,
		&snap.OnTimePickupDenominator,
		&snap.OnTimePickupNumerator,
		&snap.OnTimeDeliveryDenominator,
		&snap.OnTimeDeliveryNumerator,
		&snap.ReturnCaseCount,
		&snap.RedirectCaseCount,
		&carriersJSON,
	)
	if err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, mapDBError(err)
	}
	if len(carriersJSON) == 0 {
		carriersJSON = []byte("[]")
	}
	if err := json.Unmarshal(carriersJSON, &snap.Carriers); err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, apperrors.Internal("analytics source snapshot is inconsistent", err)
	}
	if snap.Carriers == nil {
		snap.Carriers = []domain.OperationsAnalyticsCarrierSource{}
	}
	snap.TenantID = tenantID
	return snap, nil
}

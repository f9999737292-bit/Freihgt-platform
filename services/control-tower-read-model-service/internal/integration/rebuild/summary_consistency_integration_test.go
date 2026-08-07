//go:build integration

package rebuild

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

func TestSummaryConsistencySingleTransaction(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := repository.NewProjectionRepository(pool)

	tenantID := uuid.New()
	for i, status := range []string{"CARRIER_ASSIGNED", "IN_TRANSIT", "CANCELLED"} {
		shipID := uuid.New()
		_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at,
    projection_source
) VALUES ($1,$2,$3,$4,$5,$6,'shipment.status.changed',NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
			tenantID, shipID, i+1, status, uuid.New(), uuid.New())
		require.NoError(t, err)
	}

	summary, err := repo.GetStatusSummary(ctx, tenantID)
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.TotalShipments)

	var statusSum int64
	for _, count := range summary.ByStatus {
		statusSum += count
	}
	require.Equal(t, summary.TotalShipments, statusSum)
	require.Equal(t, int64(0), summary.IncompleteProjections)
	require.LessOrEqual(t, summary.IncompleteProjections, summary.TotalShipments)
}

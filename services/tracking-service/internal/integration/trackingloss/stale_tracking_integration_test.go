//go:build integration

package trackingloss

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/freight-platform/tracking-service/internal/domain"
)

func TestStaleTrackingEventDoesNotRegressLastSeen(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)

	t2 := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	t1 := t2.Add(-5 * time.Minute)
	receivedAt := time.Date(2026, 8, 16, 10, 1, 0, 0, time.UTC)

	seedTrackingBindingAndState(t, env.pool, fix, t2)

	// Older event arrives after newer state was stored.
	stale := domain.ShipmentTrackingState{
		TenantID: fix.TenantID, ShipmentID: fix.ShipmentID,
		TrackingStatus: "active", ProviderCode: strPtr("driver_mobile"),
		LastLatitude: floatPtr(55.0), LastLongitude: floatPtr(37.0),
		LastRecordedAt: &t1, LastReceivedAt: &receivedAt,
		FreshnessStatus: "fresh", QualityStatus: "good",
		UpdatedAt: receivedAt,
	}
	require.NoError(t, env.repo.UpsertTrackingStateIfNewer(ctx, stale))

	state, err := env.repo.GetTrackingState(ctx, fix.TenantID, fix.ShipmentID)
	require.NoError(t, err)
	require.NotNil(t, state.LastRecordedAt)
	require.True(t, state.LastRecordedAt.Equal(t2))

	// Do not run the loss detector here: last_seen at T2 must remain authoritative even if age exceeds threshold.
	stateAfter, err := env.repo.GetTrackingState(ctx, fix.TenantID, fix.ShipmentID)
	require.NoError(t, err)
	require.True(t, stateAfter.LastRecordedAt.Equal(t2))
}

func strPtr(v string) *string { return &v }
func floatPtr(v float64) *float64 { return &v }

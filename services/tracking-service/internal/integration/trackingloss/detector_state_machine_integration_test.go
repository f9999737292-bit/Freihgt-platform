//go:build integration

package trackingloss

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/freight-platform/tracking-service/internal/repository"
)

func TestTrackingLossDetectorStateMachineT0ThroughT6(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)

	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	// T0: valid recent tracking, state TRACKING_OK
	seedTrackingBindingAndState(t, env.pool, fix, base.Add(-2*time.Minute))
	require.Equal(t, repository.TrackingAutomationOK, automationState(ctx, env.pool, fix))

	// T1: below threshold — no tracking.lost
	env.detector.RunOnceAt(ctx, base.Add(3*time.Minute))
	require.Equal(t, int64(0), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.lost"))
	require.Equal(t, repository.TrackingAutomationOK, automationState(ctx, env.pool, fix))

	// T2: threshold exceeded — exactly one tracking.lost
	env.detector.RunOnceAt(ctx, base.Add(8*time.Minute))
	require.Equal(t, int64(1), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.lost"))
	require.Equal(t, repository.TrackingAutomationLost, automationState(ctx, env.pool, fix))

	// T3: repeated detector without new tracking — no duplicate lost
	env.detector.RunOnceAt(ctx, base.Add(9*time.Minute))
	require.Equal(t, int64(1), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.lost"))

	// T4: fresh tracking arrives — exactly one tracking.restored
	updateTrackingRecordedAt(t, env.pool, fix, base.Add(9*time.Minute))
	env.detector.RunOnceAt(ctx, base.Add(9*time.Minute))
	require.Equal(t, int64(1), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.restored"))
	require.Equal(t, repository.TrackingAutomationOK, automationState(ctx, env.pool, fix))

	// T5: more fresh locations — no duplicate restored
	updateTrackingRecordedAt(t, env.pool, fix, base.Add(10*time.Minute))
	env.detector.RunOnceAt(ctx, base.Add(10*time.Minute))
	require.Equal(t, int64(1), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.restored"))

	// T6: stale again — second outage emits exactly one more lost
	env.detector.RunOnceAt(ctx, base.Add(16*time.Minute))
	require.Equal(t, int64(2), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.lost"))
	env.detector.RunOnceAt(ctx, base.Add(17*time.Minute))
	require.Equal(t, int64(2), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.lost"))
}

func TestTrackingRestoredSingleTransitionPerOutage(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	base := time.Date(2026, 8, 16, 14, 0, 0, 0, time.UTC)

	seedTrackingBindingAndState(t, env.pool, fix, base.Add(-10*time.Minute))
	env.detector.RunOnceAt(ctx, base)
	require.Equal(t, int64(1), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.lost"))

	updateTrackingRecordedAt(t, env.pool, fix, base)
	for i := 0; i < 3; i++ {
		env.detector.RunOnceAt(ctx, base.Add(time.Duration(i)*time.Second))
	}
	require.Equal(t, int64(1), countOutboxEvents(ctx, env.pool, fix.TenantID, fix.ShipmentID, "driver.tracking.restored"))
	require.Equal(t, repository.TrackingAutomationOK, automationState(ctx, env.pool, fix))
}

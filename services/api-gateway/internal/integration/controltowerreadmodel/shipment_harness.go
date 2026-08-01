//go:build integration

package controltowerreadmodelintegration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type shipmentBaselineHarness struct {
	pool            *pgxpool.Pool
	databaseURL     string
	shipmentBaseURL string
}

func newShipmentBaselineHarness(t *testing.T) *shipmentBaselineHarness {
	t.Helper()
	adminURL := strings.TrimSpace(getEnv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping full baseline integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	testURL, dropDB, err := createTempDatabase(ctx, adminURL, "freight_platform_full_baseline_test_")
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })

	pool, err := connectIntegrationPool(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := applyControlTowerMigrations(ctx, pool); err != nil {
		t.Fatalf("apply control tower migrations: %v", err)
	}
	if err := applyShipmentMigrations(ctx, pool); err != nil {
		t.Fatalf("apply shipment migrations: %v", err)
	}

	shipmentURL, _, err := startShipmentProcess(t, testURL)
	if err != nil {
		t.Fatalf("start shipment-service: %v", err)
	}

	return &shipmentBaselineHarness{pool: pool, databaseURL: testURL, shipmentBaseURL: shipmentURL}
}

func getEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

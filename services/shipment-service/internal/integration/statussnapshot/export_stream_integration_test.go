//go:build integration

package statussnapshot

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	snap "github.com/freight-platform/shipment-service/internal/statussnapshot"
	"github.com/freight-platform/statussnapshot"
)

func TestExportProducesValidNDJSONStream(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	_, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-STREAM"), userTransition(user))
	require.NoError(t, err)

	var stdout bytes.Buffer
	exporter := snap.NewExporter(snap.NewPostgresSnapshotRepository(env.pool), &stdout, io.Discard, nil)
	cfg, err := snap.LoadConfig(false, f.TenantA.String(), snap.DefaultBatchSize, snap.DefaultFormat, "-")
	require.NoError(t, err)
	result, err := exporter.Export(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Stats.RowCount)

	stats, err := statussnapshot.ValidateStream(bytes.NewReader(stdout.Bytes()), statussnapshot.DecoderOptions{})
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.RowCount)
	require.NotEmpty(t, stats.ChecksumHex)
}

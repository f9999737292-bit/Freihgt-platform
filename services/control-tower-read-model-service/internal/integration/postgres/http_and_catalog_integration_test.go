//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
)

func TestDuplicateSameEventIDClassifiedAsDuplicateNotStale(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	eventID := uuid.New()
	sourceEventID := uuid.New()

	v3 := sampleProcessInput(tenantID, shipmentID, 3, eventID, sourceEventID, "shipment.status.v1.test", 200)
	_, err := env.Repo.ProcessEvent(ctx, v3)
	require.NoError(t, err)

	replaySameEvent := sampleProcessInput(tenantID, shipmentID, 3, eventID, uuid.New(), "shipment.status.v1.test", 201)
	result, err := env.Repo.ProcessEvent(ctx, replaySameEvent)
	require.NoError(t, err)
	assert.True(t, result.Duplicate)
	assert.NotEqual(t, domain.OutcomeStale, result.Outcome)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.Equal(t, 3, projection.ShipmentVersion)
}

func TestMigration000015CatalogConstraints(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()

	for _, check := range []struct {
		query string
		want  bool
		desc  string
	}{
		{`SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'control_tower')`, true, "schema"},
		{`SELECT EXISTS (
			SELECT 1 FROM pg_constraint c
			JOIN pg_class t ON c.conrelid = t.oid
			JOIN pg_namespace n ON t.relnamespace = n.oid
			WHERE n.nspname = 'control_tower' AND t.relname = 'shipment_status_event_inbox' AND c.contype = 'p')`, true, "inbox pk"},
		{`SELECT EXISTS (
			SELECT 1 FROM pg_constraint c
			JOIN pg_class t ON c.conrelid = t.oid
			JOIN pg_namespace n ON t.relnamespace = n.oid
			WHERE n.nspname = 'control_tower' AND t.relname = 'shipment_status_projection' AND c.contype = 'p')`, true, "projection pk"},
		{`SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'control_tower' AND indexname = 'idx_shipment_status_projection_tenant_status')`, true, "tenant status index"},
		{`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'control_tower'
			  AND table_name = 'shipment_status_event_dead_letter'
			  AND column_name = 'payload')`, false, "no raw payload column"},
		{`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'transport' AND table_name = 'shipments')`, true, "shipments unchanged"},
	} {
		var got bool
		require.NoError(t, env.Pool.QueryRow(ctx, check.query).Scan(&got), check.desc)
		assert.Equal(t, check.want, got, check.desc)
	}
}

func TestInternalHTTPDetailSummaryListLive(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	shipmentID := uuid.New()

	input := sampleProcessInput(tenantA, shipmentID, 1, uuid.New(), uuid.New(), "shipment.status.v1.test", 300)
	input.Event.EventType = domain.EventTypeCreated
	input.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, input)
	require.NoError(t, err)

	freshness := consumer.NewFreshness()
	handler := handlers.NewStatusHandler(env.Repo, freshness)

	t.Run("detail 200 own tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/"+shipmentID.String()+"/status", nil)
		req.Header.Set("X-Tenant-ID", tenantA.String())
		rec := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("shipmentId", shipmentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.GetShipmentStatus(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("detail 404 foreign tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/"+shipmentID.String()+"/status", nil)
		req.Header.Set("X-Tenant-ID", tenantB.String())
		rec := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("shipmentId", shipmentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.GetShipmentStatus(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("detail 401 missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/"+shipmentID.String()+"/status", nil)
		rec := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("shipmentId", shipmentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.GetShipmentStatus(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("detail 400 malformed tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/"+shipmentID.String()+"/status", nil)
		req.Header.Set("X-Tenant-ID", "not-a-uuid")
		rec := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("shipmentId", shipmentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.GetShipmentStatus(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("summary tenant scoped", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/status-summary", nil)
		req.Header.Set("X-Tenant-ID", tenantA.String())
		rec := httptest.NewRecorder()
		handler.GetStatusSummary(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.EqualValues(t, 1, body["totalShipments"])
	})

	t.Run("list bounded with cursor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/statuses?limit=10", nil)
		req.Header.Set("X-Tenant-ID", tenantA.String())
		rec := httptest.NewRecorder()
		handler.ListShipmentStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("list invalid cursor 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/statuses?cursor=!!!", nil)
		req.Header.Set("X-Tenant-ID", tenantA.String())
		rec := httptest.NewRecorder()
		handler.ListShipmentStatuses(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type mockProjectionStore struct {
	getFn     func(ctx context.Context, tenantID, shipmentID uuid.UUID) (*domain.ShipmentStatusProjection, error)
	summaryFn func(ctx context.Context, tenantID uuid.UUID) (repository.StatusSummary, error)
	listFn    func(ctx context.Context, filter repository.ListFilter) ([]repository.ListItem, *repository.ListCursor, error)
}

func (m *mockProjectionStore) GetProjection(ctx context.Context, tenantID, shipmentID uuid.UUID) (*domain.ShipmentStatusProjection, error) {
	if m.getFn != nil {
		return m.getFn(ctx, tenantID, shipmentID)
	}
	return nil, nil
}

func (m *mockProjectionStore) GetStatusSummary(ctx context.Context, tenantID uuid.UUID) (repository.StatusSummary, error) {
	if m.summaryFn != nil {
		return m.summaryFn(ctx, tenantID)
	}
	return repository.StatusSummary{ByStatus: map[string]int64{}}, nil
}

func (m *mockProjectionStore) ListProjections(ctx context.Context, filter repository.ListFilter) ([]repository.ListItem, *repository.ListCursor, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return nil, nil, nil
}

func newStatusTestRouter(t *testing.T, store ProjectionStore) http.Handler {
	t.Helper()
	handler := NewStatusHandler(store, consumer.NewFreshness())
	r := chi.NewRouter()
	r.Get("/internal/v1/control-tower/shipments/{shipmentId}/status", handler.GetShipmentStatus)
	r.Get("/internal/v1/control-tower/status-summary", handler.GetStatusSummary)
	r.Get("/internal/v1/control-tower/shipments/statuses", handler.ListShipmentStatuses)
	return r
}

func sampleProjection(tenantID, shipmentID uuid.UUID) domain.ShipmentStatusProjection {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	return domain.ShipmentStatusProjection{
		TenantID:          tenantID,
		ShipmentID:        shipmentID,
		ShipmentVersion:   2,
		CurrentStatus:     domain.StatusInTransit,
		LastEventType:     domain.EventTypeStatusChanged,
		LastOccurredAt:    now,
		LastConsumedAt:    now,
		Complete:          true,
		GapDetected:       false,
		LastEventID:       uuid.New(),
		LastSourceEventID: uuid.New(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestGetShipmentStatusVerifiedTenantOwnProjectionReturns200(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipmentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	router := newStatusTestRouter(t, &mockProjectionStore{
		getFn: func(_ context.Context, tenant, shipment uuid.UUID) (*domain.ShipmentStatusProjection, error) {
			require.Equal(t, tenantID, tenant)
			require.Equal(t, shipmentID, shipment)
			p := sampleProjection(tenant, shipment)
			return &p, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/"+shipmentID.String()+"/status", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, shipmentID.String(), payload["shipmentId"])
	assert.Equal(t, float64(2), payload["version"])
	assert.Equal(t, domain.StatusInTransit, payload["currentStatus"])
}

func TestGetShipmentStatusForeignTenantReturns404(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipmentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	router := newStatusTestRouter(t, &mockProjectionStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.ShipmentStatusProjection, error) {
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/"+shipmentID.String()+"/status", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetShipmentStatusMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetShipmentStatusMalformedTenantReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/status", nil)
	req.Header.Set("X-Tenant-ID", "bad")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetShipmentStatusInvalidShipmentIDReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/not-a-uuid/status", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetStatusSummaryTenantScopedCounts(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	oldest := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	router := newStatusTestRouter(t, &mockProjectionStore{
		summaryFn: func(_ context.Context, tenant uuid.UUID) (repository.StatusSummary, error) {
			require.Equal(t, tenantID, tenant)
			return repository.StatusSummary{
				TotalShipments:            3,
				ByStatus:                  map[string]int64{domain.StatusInTransit: 2, domain.StatusDelivered: 1},
				IncompleteProjections:     1,
				OldestProjectionUpdatedAt: &oldest,
				LatestProjectionUpdatedAt: &latest,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/status-summary", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, float64(3), payload["totalShipments"])
	assert.Equal(t, float64(1), payload["incompleteProjections"])
	byStatus := payload["byStatus"].(map[string]any)
	assert.Equal(t, float64(2), byStatus[domain.StatusInTransit])
	assert.NotNil(t, payload["freshness"])
}

func TestGetStatusSummaryMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/status-summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListShipmentStatusesTenantScopedWithCursor(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipmentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	updatedAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	nextCursorPayload, _ := json.Marshal(map[string]any{
		"updatedAt":  updatedAt,
		"shipmentId": shipmentID,
	})
	nextCursor := base64.RawURLEncoding.EncodeToString(nextCursorPayload)

	router := newStatusTestRouter(t, &mockProjectionStore{
		listFn: func(_ context.Context, filter repository.ListFilter) ([]repository.ListItem, *repository.ListCursor, error) {
			require.Equal(t, tenantID, filter.TenantID)
			require.Equal(t, domain.StatusInTransit, filter.Status)
			require.Equal(t, 10, filter.Limit)
			p := sampleProjection(tenantID, shipmentID)
			return []repository.ListItem{{Projection: p}}, &repository.ListCursor{UpdatedAt: updatedAt, ShipmentID: shipmentID}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/statuses?status=IN_TRANSIT&limit=10", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	items := payload["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, nextCursor, payload["nextCursor"])
}

func TestListShipmentStatusesInvalidStatusReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/statuses?status=NOT_REAL", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListShipmentStatusesInvalidCursorReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/statuses?cursor=!!!", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListShipmentStatusesInvalidLimitReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/statuses?limit=0", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetShipmentStatusRepoErrorReturns500(t *testing.T) {
	t.Parallel()
	router := newStatusTestRouter(t, &mockProjectionStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.ShipmentStatusProjection, error) {
			return nil, errors.New("db down")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/status", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

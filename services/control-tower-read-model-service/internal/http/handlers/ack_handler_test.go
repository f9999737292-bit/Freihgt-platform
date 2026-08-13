package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type stubAckStore struct {
	upsertInput domain.AcknowledgeCriticalEventInput
	upsertOut   domain.CriticalEventAcknowledgement
	lookupOut   []domain.CriticalEventAcknowledgement
}

func (s *stubAckStore) UpsertAcknowledgement(_ context.Context, input domain.AcknowledgeCriticalEventInput) (domain.CriticalEventAcknowledgement, error) {
	s.upsertInput = input
	return s.upsertOut, nil
}

func (s *stubAckStore) LookupAcknowledgements(_ context.Context, _ uuid.UUID, _ []string) ([]domain.CriticalEventAcknowledgement, error) {
	return s.lookupOut, nil
}

func TestAckHandlerAcknowledgeUsesTrustedHeaders(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shipmentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	occurredAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)

	store := &stubAckStore{
		upsertOut: domain.CriticalEventAcknowledgement{
			TenantID:             tenantID,
			EventID:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ShipmentID:           shipmentID,
			EventType:            "PICKUP_DELAY",
			Source:               "control-tower",
			OccurredAt:           occurredAt,
			AcknowledgedAt:       occurredAt.Add(time.Minute),
			AcknowledgedByUserID: userID,
		},
	}
	handler := &AckHandler{repo: store}

	body, _ := json.Marshal(map[string]string{
		"shipmentId": shipmentID.String(),
		"eventType":  "PICKUP_DELAY",
		"occurredAt": occurredAt.Format(time.RFC3339),
		"source":     "control-tower",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/control-tower/critical-events/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/acknowledge", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("eventId", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	rec := httptest.NewRecorder()
	handler.AcknowledgeCriticalEvent(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, store.upsertInput.TenantID)
	assert.Equal(t, userID, store.upsertInput.UserID)
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", store.upsertInput.EventID)
}

func TestAckHandlerLookupReturnsItems(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ackedAt := time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC)

	store := &stubAckStore{
		lookupOut: []domain.CriticalEventAcknowledgement{{
			EventID:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			AcknowledgedAt:       ackedAt,
			AcknowledgedByUserID: userID,
		}},
	}
	handler := &AckHandler{repo: store}

	body, _ := json.Marshal(map[string]any{"eventIds": []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/control-tower/critical-events/acknowledgements/lookup", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID.String())

	rec := httptest.NewRecorder()
	handler.LookupAcknowledgements(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
}

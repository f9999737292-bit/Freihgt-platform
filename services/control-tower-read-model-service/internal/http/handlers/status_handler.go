package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type ProjectionStore interface {
	GetProjection(ctx context.Context, tenantID, shipmentID uuid.UUID) (*domain.ShipmentStatusProjection, error)
	GetStatusSummary(ctx context.Context, tenantID uuid.UUID) (repository.StatusSummary, error)
	ListProjections(ctx context.Context, filter repository.ListFilter) ([]repository.ListItem, *repository.ListCursor, error)
}

type StatusHandler struct {
	repo      ProjectionStore
	freshness *consumer.Freshness
}

func NewStatusHandler(repo ProjectionStore, freshness *consumer.Freshness) *StatusHandler {
	return &StatusHandler{repo: repo, freshness: freshness}
}

type statusDetailResponse struct {
	ShipmentID     string  `json:"shipmentId"`
	Version        int     `json:"version"`
	CurrentStatus  string  `json:"currentStatus"`
	PreviousStatus *string `json:"previousStatus,omitempty"`
	LastEventType  string  `json:"lastEventType"`
	LastOccurredAt string  `json:"lastOccurredAt"`
	LastConsumedAt string  `json:"lastConsumedAt"`
	Complete       bool    `json:"complete"`
	GapDetected    bool    `json:"gapDetected"`
	GapFromVersion *int    `json:"gapFromVersion,omitempty"`
	GapToVersion   *int    `json:"gapToVersion,omitempty"`
}

func (h *StatusHandler) GetShipmentStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := uuid.Parse(chi.URLParam(r, "shipmentId"))
	if err != nil || shipmentID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid shipmentId", map[string]any{"field": "shipmentId"}))
		return
	}

	projection, err := h.repo.GetProjection(r.Context(), tenantID, shipmentID)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to load projection", err))
		return
	}
	if projection == nil {
		respond.Error(w, apperrors.NotFound("shipment status projection not found"))
		return
	}

	respond.JSON(w, http.StatusOK, toStatusDetailResponse(*projection))
}

type statusSummaryResponse struct {
	TotalShipments            int64                      `json:"totalShipments"`
	ByStatus                  map[string]int64           `json:"byStatus"`
	IncompleteProjections     int64                      `json:"incompleteProjections"`
	OldestProjectionUpdatedAt *string                    `json:"oldestProjectionUpdatedAt,omitempty"`
	LatestProjectionUpdatedAt *string                    `json:"latestProjectionUpdatedAt,omitempty"`
	Freshness                 consumer.FreshnessSnapshot `json:"freshness"`
}

func (h *StatusHandler) GetStatusSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	summary, err := h.repo.GetStatusSummary(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to load status summary", err))
		return
	}

	resp := statusSummaryResponse{
		TotalShipments:        summary.TotalShipments,
		ByStatus:              summary.ByStatus,
		IncompleteProjections: summary.IncompleteProjections,
		Freshness:             h.freshness.Snapshot(),
	}
	if summary.OldestProjectionUpdatedAt != nil {
		s := summary.OldestProjectionUpdatedAt.UTC().Format(time.RFC3339)
		resp.OldestProjectionUpdatedAt = &s
	}
	if summary.LatestProjectionUpdatedAt != nil {
		s := summary.LatestProjectionUpdatedAt.UTC().Format(time.RFC3339)
		resp.LatestProjectionUpdatedAt = &s
	}
	respond.JSON(w, http.StatusOK, resp)
}

type listResponse struct {
	Items      []statusDetailResponse `json:"items"`
	NextCursor *string                `json:"nextCursor,omitempty"`
}

func (h *StatusHandler) ListShipmentStatuses(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	if statusFilter != "" && !domain.IsAllowedShipmentStatus(statusFilter) {
		respond.Error(w, apperrors.Validation("invalid status filter", map[string]any{"field": "status"}))
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			respond.Error(w, apperrors.Validation("invalid limit", map[string]any{"field": "limit"}))
			return
		}
		limit = parsed
	}

	var cursor *repository.ListCursor
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid cursor", map[string]any{"field": "cursor"}))
			return
		}
		var payload struct {
			UpdatedAt  time.Time `json:"updatedAt"`
			ShipmentID uuid.UUID `json:"shipmentId"`
		}
		if err := json.Unmarshal(decoded, &payload); err != nil {
			respond.Error(w, apperrors.Validation("invalid cursor", map[string]any{"field": "cursor"}))
			return
		}
		cursor = &repository.ListCursor{UpdatedAt: payload.UpdatedAt, ShipmentID: payload.ShipmentID}
	}

	items, next, err := h.repo.ListProjections(r.Context(), repository.ListFilter{
		TenantID: tenantID,
		Status:   statusFilter,
		Limit:    limit,
		Cursor:   cursor,
	})
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to list projections", err))
		return
	}

	resp := listResponse{Items: make([]statusDetailResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toStatusDetailResponse(item.Projection))
	}
	if next != nil {
		payload, _ := json.Marshal(map[string]any{
			"updatedAt":  next.UpdatedAt.UTC(),
			"shipmentId": next.ShipmentID,
		})
		encoded := base64.RawURLEncoding.EncodeToString(payload)
		resp.NextCursor = &encoded
	}
	respond.JSON(w, http.StatusOK, resp)
}

func toStatusDetailResponse(p domain.ShipmentStatusProjection) statusDetailResponse {
	return statusDetailResponse{
		ShipmentID:     p.ShipmentID.String(),
		Version:        p.ShipmentVersion,
		CurrentStatus:  p.CurrentStatus,
		PreviousStatus: p.PreviousStatus,
		LastEventType:  p.LastEventType,
		LastOccurredAt: p.LastOccurredAt.UTC().Format(time.RFC3339),
		LastConsumedAt: p.LastConsumedAt.UTC().Format(time.RFC3339),
		Complete:       p.Complete,
		GapDetected:    p.GapDetected,
		GapFromVersion: p.GapFromVersion,
		GapToVersion:   p.GapToVersion,
	}
}

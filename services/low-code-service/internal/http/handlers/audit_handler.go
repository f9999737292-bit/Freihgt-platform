package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/platform/respond"
	"github.com/freight-platform/low-code-service/internal/service"
)

type AuditHandler struct {
	service *service.AuditService
}

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{service: svc}
}

type listAuditEventsResponse struct {
	Items []auditEventResponse `json:"items"`
}

type auditEventResponse struct {
	ID            string                     `json:"id"`
	TenantID      string                     `json:"tenant_id"`
	EntityType    string                     `json:"entity_type"`
	EntityID      string                     `json:"entity_id"`
	Action        string                     `json:"action"`
	Actor         string                     `json:"actor,omitempty"`
	ChangedFields []string                   `json:"changed_fields"`
	OldValues     map[string]json.RawMessage `json:"old_values"`
	NewValues     map[string]json.RawMessage `json:"new_values"`
	CreatedAt     string                     `json:"created_at"`
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	filter := domain.ListAuditEventsFilter{
		TenantID:   tenantID,
		EntityType: r.URL.Query().Get("entity_type"),
		Action:     r.URL.Query().Get("action"),
		Limit:      parseAuditLimit(r.URL.Query().Get("limit")),
	}

	if entityIDRaw := r.URL.Query().Get("entity_id"); entityIDRaw != "" {
		entityID, err := uuid.Parse(entityIDRaw)
		if err != nil {
			respond.Error(w, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id", "value": entityIDRaw}))
			return
		}
		filter.EntityID = &entityID
	}

	items, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	responseItems := make([]auditEventResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, toAuditEventResponse(item))
	}

	respond.JSON(w, http.StatusOK, listAuditEventsResponse{Items: responseItems})
}

func parseAuditLimit(raw string) int {
	if raw == "" {
		return 50
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func toAuditEventResponse(item domain.ConfigurationAuditEntry) auditEventResponse {
	changedFields := domain.ParseAuditChangedFields(item.NewValueJSON)
	if len(changedFields) == 0 {
		changedFields = []string{}
	}

	actor := ""
	if item.ChangedByUserID != nil {
		actor = item.ChangedByUserID.String()
	}

	return auditEventResponse{
		ID:            item.ID.String(),
		TenantID:      item.TenantID.String(),
		EntityType:    item.EntityType,
		EntityID:      item.EntityID.String(),
		Action:        domain.ParseAuditEventAction(item.Action, item.NewValueJSON),
		Actor:         actor,
		ChangedFields: changedFields,
		OldValues:     domain.ParseAuditValuesMap(item.OldValueJSON),
		NewValues:     domain.ParseAuditValuesMap(item.NewValueJSON),
		CreatedAt:     item.ChangedAt.UTC().Format(time.RFC3339),
	}
}

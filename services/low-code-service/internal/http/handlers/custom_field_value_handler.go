package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/platform/respond"
	"github.com/freight-platform/low-code-service/internal/service"
)

type CustomFieldValueHandler struct {
	service *service.CustomFieldValueService
}

func NewCustomFieldValueHandler(svc *service.CustomFieldValueService) *CustomFieldValueHandler {
	return &CustomFieldValueHandler{service: svc}
}

type listCustomFieldValuesResponse struct {
	TenantID   string                         `json:"tenant_id"`
	EntityType string                         `json:"entity_type"`
	EntityID   string                         `json:"entity_id"`
	Items      []customFieldValueItemResponse `json:"items"`
}

type customFieldValueItemResponse struct {
	FieldID   string          `json:"field_id"`
	FieldCode string          `json:"field_code"`
	ValueJSON json.RawMessage `json:"value_json"`
	UpdatedAt string          `json:"updated_at"`
}

type upsertCustomFieldValuesRequest struct {
	EntityType     string                         `json:"entity_type"`
	EntityID       string                         `json:"entity_id"`
	FormTemplateID string                         `json:"form_template_id"`
	Values         []upsertCustomFieldValueItem   `json:"values"`
}

type upsertCustomFieldValueItem struct {
	FieldCode string          `json:"field_code"`
	ValueJSON json.RawMessage `json:"value_json"`
}

type upsertCustomFieldValuesResponse struct {
	Status     string `json:"status"`
	TenantID   string `json:"tenant_id"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	SavedCount int    `json:"saved_count"`
}

func (h *CustomFieldValueHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	entityType := r.URL.Query().Get("entity_type")
	if entityType == "" {
		respond.Error(w, apperrors.Validation("entity_type is required", map[string]any{"field": "entity_type"}))
		return
	}

	entityIDRaw := r.URL.Query().Get("entity_id")
	if entityIDRaw == "" {
		respond.Error(w, apperrors.Validation("entity_id is required", map[string]any{"field": "entity_id"}))
		return
	}
	entityID, err := uuid.Parse(entityIDRaw)
	if err != nil {
		respond.Error(w, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id", "value": entityIDRaw}))
		return
	}

	items, err := h.service.GetByEntity(r.Context(), tenantID, entityType, entityID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	responseItems := make([]customFieldValueItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, customFieldValueItemResponse{
			FieldID:   item.FieldID.String(),
			FieldCode: item.FieldCode,
			ValueJSON: item.ValueJSON,
			UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	respond.JSON(w, http.StatusOK, listCustomFieldValuesResponse{
		TenantID:   tenantID.String(),
		EntityType: entityType,
		EntityID:   entityID.String(),
		Items:      responseItems,
	})
}

func (h *CustomFieldValueHandler) Put(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req upsertCustomFieldValuesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"error": err.Error()}))
		return
	}

	entityID, err := uuid.Parse(req.EntityID)
	if err != nil {
		respond.Error(w, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id", "value": req.EntityID}))
		return
	}
	formTemplateID, err := uuid.Parse(req.FormTemplateID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid form_template_id", map[string]any{"field": "form_template_id"}))
		return
	}

	values := make([]domain.CustomFieldValueInput, 0, len(req.Values))
	for _, item := range req.Values {
		values = append(values, domain.CustomFieldValueInput{
			FieldCode: item.FieldCode,
			ValueJSON: item.ValueJSON,
		})
	}

	result, err := h.service.Upsert(r.Context(), domain.UpsertCustomFieldValuesInput{
		TenantID:       tenantID,
		EntityType:     req.EntityType,
		EntityID:       entityID,
		FormTemplateID: formTemplateID,
		Values:         values,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, upsertCustomFieldValuesResponse{
		Status:     "ok",
		TenantID:   result.TenantID.String(),
		EntityType: result.EntityType,
		EntityID:   result.EntityID.String(),
		SavedCount: result.SavedCount,
	})
}

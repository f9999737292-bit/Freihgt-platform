package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
	"github.com/freight-platform/company-service/internal/repository"
)

const maxCompanyBatchSize = 500

type companyBatchGetter interface {
	BatchGetByIDs(ctx context.Context, tenantID uuid.UUID, companyIDs []uuid.UUID) ([]repository.CompanyDisplayRow, error)
}

type CompanyInternalHandler struct {
	repo companyBatchGetter
}

func NewCompanyInternalHandler(repo companyBatchGetter) *CompanyInternalHandler {
	return &CompanyInternalHandler{repo: repo}
}

type batchCompaniesRequest struct {
	CompanyIDs []string `json:"company_ids"`
}

type batchCompanyItem struct {
	CompanyID   string  `json:"company_id"`
	LegalName   string  `json:"legal_name"`
	ShortName   *string `json:"short_name,omitempty"`
	Status      string  `json:"status"`
}

func parseTrustedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("tenant context is required", map[string]any{"field": "tenant_id"})
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

func (h *CompanyInternalHandler) BatchGet(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req batchCompaniesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	if len(req.CompanyIDs) == 0 {
		respond.Error(w, apperrors.Validation("company_ids is required", map[string]any{"field": "company_ids"}))
		return
	}
	if len(req.CompanyIDs) > maxCompanyBatchSize {
		respond.Error(w, apperrors.Validation("company_ids exceeds batch limit", map[string]any{
			"field": "company_ids",
			"max":   maxCompanyBatchSize,
		}))
		return
	}
	seen := make(map[uuid.UUID]struct{}, len(req.CompanyIDs))
	ids := make([]uuid.UUID, 0, len(req.CompanyIDs))
	for _, raw := range req.CompanyIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || id == uuid.Nil {
			respond.Error(w, apperrors.Validation("invalid company_id", map[string]any{"field": "company_ids"}))
			return
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	rows, err := h.repo.BatchGetByIDs(r.Context(), tenantID, ids)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]batchCompanyItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, batchCompanyItem{
			CompanyID: row.ID.String(),
			LegalName: row.LegalName,
			ShortName: row.ShortName,
			Status:    row.Status,
		})
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

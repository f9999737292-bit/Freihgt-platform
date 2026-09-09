package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type TemplateCloneHandler struct {
	service *service.TemplateCloneService
}

func NewTemplateCloneHandler(svc *service.TemplateCloneService) *TemplateCloneHandler {
	return &TemplateCloneHandler{service: svc}
}

type cloneEventFromTemplateRequest struct {
	TemplateVersionID string  `json:"template_version_id"`
	RfxNumber         string  `json:"rfx_number"`
	RfxType           string  `json:"rfx_type"`
	Category          string  `json:"category"`
	Title             string  `json:"title"`
	Description       *string `json:"description"`
	OwnerCompanyID    string  `json:"owner_company_id"`
	CurrencyCode      *string `json:"currency_code"`
	ValidFrom         *string `json:"valid_from"`
	ValidTo           *string `json:"valid_to"`
	ResponseDeadline  *string `json:"response_deadline"`
}

func (h *TemplateCloneHandler) CreateEventFromTemplate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req cloneEventFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	templateVersionID, err := domain.ParseUUID(req.TemplateVersionID, "template_version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	eventIn, err := parseCreateRfxEventRequest(createRfxEventRequest{
		RfxNumber:        req.RfxNumber,
		RfxType:          req.RfxType,
		Category:         req.Category,
		Title:            req.Title,
		Description:      req.Description,
		OwnerCompanyID:   req.OwnerCompanyID,
		CurrencyCode:     req.CurrencyCode,
		ValidFrom:        req.ValidFrom,
		ValidTo:          req.ValidTo,
		ResponseDeadline: req.ResponseDeadline,
	}, actor.TenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.CloneEventFromTemplate(r.Context(), actor, templateVersionID, eventIn, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toCloneEventFromTemplateResponse(result))
}

func toCloneEventFromTemplateResponse(result *domain.CloneEventFromTemplateResult) map[string]any {
	resp := toRfxEventResponse(&result.Event)
	resp["draft_version_id"] = result.DraftVersion.ID.String()
	resp["source_template_id"] = result.SourceTemplateID.String()
	resp["source_template_version_id"] = result.SourceTemplateVersionID.String()
	resp["source_version_number"] = result.SourceVersionNumber
	resp["source_version_status"] = result.SourceVersionStatus
	if result.SourceVersionWarning {
		resp["source_version_warning"] = true
	}
	return resp
}

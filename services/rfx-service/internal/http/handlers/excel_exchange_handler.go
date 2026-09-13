package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

type ExcelExchangeHandler struct {
	service *service.ExcelExchangeService
}

func NewExcelExchangeHandler(svc *service.ExcelExchangeService) *ExcelExchangeHandler {
	return &ExcelExchangeHandler{service: svc}
}

func (h *ExcelExchangeHandler) ExportBuyerDraftXLSX(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	data, filename, err := h.service.ExportBuyerDraftWorkbook(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.BinaryAttachment(w, http.StatusOK, service.BuyerDraftXLSXContentType, filename, data)
}

func (h *ExcelExchangeHandler) PreviewBuyerImportXLSX(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	fileBytes, err := ReadBuyerXlsxImportFile(w, r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	preview, err := h.service.PreviewBuyerImportWorkbook(r.Context(), actor, eventID, fileBytes)
	if err != nil {
		respond.Error(w, err)
		return
	}

	switch xlsxexchange.ClassifyPreviewErrors(preview.Errors) {
	case xlsxexchange.PreviewErrorClassStructural:
		respond.Error(w, service.StructuralPreviewAppError(preview.Errors[0]))
		return
	case xlsxexchange.PreviewErrorClassDomain:
		respond.JSON(w, http.StatusUnprocessableEntity, preview)
		return
	default:
		respond.JSON(w, http.StatusOK, preview)
	}
}

func (h *ExcelExchangeHandler) CommitBuyerImportXLSX(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var body domain.BuyerImportCommitInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	result, err := h.service.CommitBuyerImportAnalysis(r.Context(), actor, eventID, body, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

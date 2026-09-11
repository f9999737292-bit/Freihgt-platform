package handlers

import (
	"net/http"

	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
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

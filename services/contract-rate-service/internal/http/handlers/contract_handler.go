package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/platform/respond"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

type ContractHandler struct {
	svc    *service.ContractService
	actors *ActorResolver
}

func NewContractHandler(svc *service.ContractService, actors *ActorResolver) *ContractHandler {
	return &ContractHandler{svc: svc, actors: actors}
}

type createContractRequest struct {
	BuyerCompanyID    uuid.UUID `json:"buyer_company_id"`
	CarrierCompanyID  uuid.UUID `json:"carrier_company_id"`
	ContractNumber    string    `json:"contract_number"`
	ExternalReference *string   `json:"external_reference"`
	Name              string    `json:"name"`
	Description       *string   `json:"description"`
	ValidFrom         string    `json:"valid_from"`
	ValidTo           *string   `json:"valid_to"`
	CurrencyCode      string    `json:"currency_code"`
}

func (h *ContractHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	validFrom, err := parseDate(req.ValidFrom, "valid_from")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var validTo *time.Time
	if req.ValidTo != nil {
		vt, err := parseDate(*req.ValidTo, "valid_to")
		if err != nil {
			respond.Error(w, err)
			return
		}
		validTo = &vt
	}
	created, err := h.svc.Create(r.Context(), domain.CreateContractInput{
		TenantID: actor.TenantID, BuyerCompanyID: req.BuyerCompanyID, CarrierCompanyID: req.CarrierCompanyID,
		ContractNumber: req.ContractNumber, ExternalReference: req.ExternalReference, Name: req.Name,
		Description: req.Description, ValidFrom: validFrom, ValidTo: validTo, CurrencyCode: req.CurrencyCode, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, mapContract(created))
}

func (h *ContractHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.svc.List(r.Context(), actor.TenantID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for i := range items {
		out = append(out, mapContract(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *ContractHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	contractID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.svc.Get(r.Context(), actor.TenantID, contractID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapContract(item))
}

func (h *ContractHandler) Patch(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	contractID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		Name              *string         `json:"name"`
		Description       *string         `json:"description"`
		ExternalReference *string         `json:"external_reference"`
		ValidTo           json.RawMessage `json:"valid_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	validToPatch, err := domain.ParseNullableDatePatch(req.ValidTo, "valid_to")
	if err != nil {
		respond.Error(w, err)
		return
	}
	corr := CorrelationID(r)
	if req.Name != nil || validToPatch.Present {
		updated, err := h.svc.UpdateDraft(r.Context(), actor.TenantID, contractID, domain.UpdateContractInput{
			Name: req.Name, Description: req.Description, ExternalReference: req.ExternalReference, ValidTo: validToPatch, Actor: actor,
		}, corr)
		if err != nil {
			respond.Error(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, mapContract(updated))
		return
	}
	updated, err := h.svc.PatchMetadata(r.Context(), actor.TenantID, contractID, domain.PatchContractMetadataInput{
		Description: req.Description, ExternalReference: req.ExternalReference, Actor: actor,
	}, corr)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapContract(updated))
}

func (h *ContractHandler) Activate(w http.ResponseWriter, r *http.Request)   { h.lifecycle(w, r, "activate") }
func (h *ContractHandler) Suspend(w http.ResponseWriter, r *http.Request)    { h.lifecycle(w, r, "suspend") }
func (h *ContractHandler) Reactivate(w http.ResponseWriter, r *http.Request) { h.lifecycle(w, r, "reactivate") }
func (h *ContractHandler) Terminate(w http.ResponseWriter, r *http.Request)  { h.lifecycle(w, r, "terminate") }
func (h *ContractHandler) Cancel(w http.ResponseWriter, r *http.Request)     { h.lifecycle(w, r, "cancel") }

func (h *ContractHandler) lifecycle(w http.ResponseWriter, r *http.Request, action string) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	contractID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	corr := CorrelationID(r)
	var item *domain.TransportContract
	switch action {
	case "activate":
		item, err = h.svc.Activate(r.Context(), actor.TenantID, contractID, actor, corr)
	case "suspend":
		item, err = h.svc.Suspend(r.Context(), actor.TenantID, contractID, actor, corr)
	case "reactivate":
		item, err = h.svc.Reactivate(r.Context(), actor.TenantID, contractID, actor, corr)
	case "terminate":
		var req struct {
			Reason *string `json:"termination_reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		item, err = h.svc.Terminate(r.Context(), actor.TenantID, contractID, actor, req.Reason, corr)
	case "cancel":
		item, err = h.svc.Cancel(r.Context(), actor.TenantID, contractID, actor, corr)
	}
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapContract(item))
}

func mapContract(c *domain.TransportContract) map[string]any {
	return map[string]any{
		"id": c.ID, "tenant_id": c.TenantID, "buyer_company_id": c.BuyerCompanyID,
		"carrier_company_id": c.CarrierCompanyID, "contract_number": c.ContractNumber,
		"external_reference": c.ExternalReference, "name": c.Name, "description": c.Description,
		"status": c.Status, "valid_from": c.ValidFrom.Format("2006-01-02"),
		"valid_to": datePtr(c.ValidTo), "currency_code": c.CurrencyCode,
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt, "version": c.Version,
	}
}

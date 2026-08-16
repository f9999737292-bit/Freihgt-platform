package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type RfxHandler struct {
	service *service.RfxService
}

func NewRfxHandler(svc *service.RfxService) *RfxHandler {
	return &RfxHandler{service: svc}
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func (h *RfxHandler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "rfx-service",
	})
}

type createRfxEventRequest struct {
	TenantID         string  `json:"tenant_id"`
	RfxNumber        string  `json:"rfx_number"`
	RfxType          string  `json:"rfx_type"`
	Category         string  `json:"category"`
	Title            string  `json:"title"`
	Description      *string `json:"description"`
	OwnerCompanyID   string  `json:"owner_company_id"`
	CurrencyCode     *string `json:"currency_code"`
	ValidFrom        *string `json:"valid_from"`
	ValidTo          *string `json:"valid_to"`
	ResponseDeadline *string `json:"response_deadline"`
}

type updateRfxEventRequest struct {
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	ResponseDeadline *string `json:"response_deadline"`
}

type extendDeadlineRequest struct {
	ResponseDeadline string `json:"response_deadline"`
}

type createRfxLotRequest struct {
	TenantID       string   `json:"tenant_id"`
	LotNumber      string   `json:"lot_number"`
	Name           string   `json:"name"`
	Description    *string  `json:"description"`
	Category       *string  `json:"category"`
	EstimatedValue *float64 `json:"estimated_value"`
	CurrencyCode   *string  `json:"currency_code"`
}

type createRfxLaneRequest struct {
	TenantID              string   `json:"tenant_id"`
	OriginLocationID      string   `json:"origin_location_id"`
	DestinationLocationID string   `json:"destination_location_id"`
	TransportMode         string   `json:"transport_mode"`
	EquipmentType         *string  `json:"equipment_type"`
	EstimatedVolume       *float64 `json:"estimated_volume"`
	VolumeUnit            *string  `json:"volume_unit"`
	RequiredServiceLevel  *string  `json:"required_service_level"`
}

type addRfxParticipantRequest struct {
	TenantID        string `json:"tenant_id"`
	CompanyID       string `json:"company_id"`
	ParticipantType string `json:"participant_type"`
}

type createRfxResponseRequest struct {
	TenantID             string `json:"tenant_id"`
	ParticipantCompanyID string `json:"participant_company_id"`
}

func (h *RfxHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var req createRfxEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	if err := rejectBodyTenantMismatch(req.TenantID, actor.TenantID); err != nil {
		respond.Error(w, err)
		return
	}

	input, err := parseCreateRfxEventRequest(req, actor.TenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	event, err := h.service.CreateEvent(r.Context(), actor, input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toRfxEventResponse(event))
}

func (h *RfxHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	event, err := h.service.GetEvent(r.Context(), actor, id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toRfxEventResponse(event))
}

func (h *RfxHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	filter := domain.ListRfxEventsFilter{TenantID: actor.TenantID, Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := strings.TrimSpace(r.URL.Query().Get("rfx_type")); raw != "" {
		filter.RfxType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("category")); raw != "" {
		filter.Category = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("owner_company_id")); raw != "" {
		parsed, err := domain.ParseUUID(raw, "owner_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.OwnerCompanyID = &parsed
	}

	events, total, err := h.service.ListEvents(r.Context(), actor, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(events))
	for i := range events {
		items = append(items, toRfxEventResponse(&events[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *RfxHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req updateRfxEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	input := domain.UpdateRfxEventInput{Title: req.Title, Description: req.Description}
	if req.ResponseDeadline != nil {
		deadline, err := domain.ParseDateTime(*req.ResponseDeadline, "response_deadline")
		if err != nil {
			respond.Error(w, err)
			return
		}
		input.ResponseDeadline = deadline
	}

	event, err := h.service.UpdateEvent(r.Context(), actor, id, input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toRfxEventResponse(event))
}

func (h *RfxHandler) PublishEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	event, err := h.service.PublishEvent(r.Context(), actor, id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     event.ID.String(),
		"status": event.Status,
	})
}

func (h *RfxHandler) CancelEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	event, err := h.service.CancelEvent(r.Context(), actor, id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     event.ID.String(),
		"status": event.Status,
	})
}

func (h *RfxHandler) TransitionEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	command := domain.RfxTransitionCommand(strings.TrimSpace(chi.URLParam(r, "command")))
	event, err := h.service.TransitionEvent(r.Context(), actor, id, command)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     event.ID.String(),
		"status": event.Status,
	})
}

func (h *RfxHandler) OpenQuestions(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandOpenQuestions)
}

func (h *RfxHandler) OpenResponses(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandOpenResponses)
}

func (h *RfxHandler) CloseResponses(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandCloseResponses)
}

func (h *RfxHandler) StartEvaluation(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandStartEvaluation)
}

func (h *RfxHandler) Shortlist(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandShortlist)
}

func (h *RfxHandler) AwardEvent(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandAward)
}

func (h *RfxHandler) ArchiveEvent(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandArchive)
}

func (h *RfxHandler) SendInvitations(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, domain.RfxCommandSendInvitations)
}

func (h *RfxHandler) ExtendDeadline(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req extendDeadlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	deadline, err := domain.ParseDateTime(req.ResponseDeadline, "response_deadline")
	if err != nil {
		respond.Error(w, err)
		return
	}

	event, err := h.service.ExtendDeadline(r.Context(), actor, id, *deadline)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":                event.ID.String(),
		"status":            event.Status,
		"response_deadline": formatDateTime(event.ResponseDeadline),
	})
}

func (h *RfxHandler) transition(w http.ResponseWriter, r *http.Request, command domain.RfxTransitionCommand) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	event, err := h.service.TransitionEvent(r.Context(), actor, id, command)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     event.ID.String(),
		"status": event.Status,
	})
}

func (h *RfxHandler) CreateLot(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req createRfxLotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	if err := rejectBodyTenantMismatch(req.TenantID, actor.TenantID); err != nil {
		respond.Error(w, err)
		return
	}

	lot, err := h.service.CreateLot(r.Context(), actor, eventID, domain.CreateRfxLotInput{
		TenantID:       actor.TenantID,
		LotNumber:      req.LotNumber,
		Name:           req.Name,
		Description:    req.Description,
		Category:       req.Category,
		EstimatedValue: req.EstimatedValue,
		CurrencyCode:   req.CurrencyCode,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toRfxLotResponse(lot))
}

func (h *RfxHandler) ListLots(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	lots, err := h.service.ListLots(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(lots))
	for i := range lots {
		items = append(items, toRfxLotResponse(&lots[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RfxHandler) CreateLane(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	lotID, err := domain.ParseUUID(chi.URLParam(r, "lot_id"), "lot_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req createRfxLaneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	if err := rejectBodyTenantMismatch(req.TenantID, actor.TenantID); err != nil {
		respond.Error(w, err)
		return
	}
	originID, err := domain.ParseUUID(req.OriginLocationID, "origin_location_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	destinationID, err := domain.ParseUUID(req.DestinationLocationID, "destination_location_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	lane, err := h.service.CreateLane(r.Context(), actor, lotID, domain.CreateRfxLaneInput{
		TenantID:              actor.TenantID,
		OriginLocationID:      originID,
		DestinationLocationID: destinationID,
		TransportMode:         domain.NormalizeTransportMode(req.TransportMode),
		EquipmentType:         req.EquipmentType,
		EstimatedVolume:       req.EstimatedVolume,
		VolumeUnit:            req.VolumeUnit,
		RequiredServiceLevel:  req.RequiredServiceLevel,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toRfxLaneResponse(lane))
}

func (h *RfxHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req addRfxParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	if err := rejectBodyTenantMismatch(req.TenantID, actor.TenantID); err != nil {
		respond.Error(w, err)
		return
	}
	companyID, err := domain.ParseUUID(req.CompanyID, "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	participant, err := h.service.AddParticipant(r.Context(), actor, eventID, domain.AddRfxParticipantInput{
		TenantID:        actor.TenantID,
		CompanyID:       companyID,
		ParticipantType: req.ParticipantType,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toRfxParticipantResponse(participant))
}

func (h *RfxHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var status *string
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		status = &raw
	}

	participants, err := h.service.ListParticipants(r.Context(), actor, eventID, status)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(participants))
	for i := range participants {
		items = append(items, toRfxParticipantResponse(&participants[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RfxHandler) CreateResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req createRfxResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	if err := rejectBodyTenantMismatch(req.TenantID, actor.TenantID); err != nil {
		respond.Error(w, err)
		return
	}
	participantCompanyID, err := domain.ParseUUID(req.ParticipantCompanyID, "participant_company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	response, err := h.service.CreateResponse(r.Context(), actor, eventID, domain.CreateRfxResponseInput{
		TenantID:             actor.TenantID,
		ParticipantCompanyID: participantCompanyID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toRfxResponseResponse(response))
}

func (h *RfxHandler) SubmitResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	responseID, err := domain.ParseUUID(chi.URLParam(r, "response_id"), "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	response, err := h.service.SubmitResponse(r.Context(), actor, responseID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     response.ID.String(),
		"status": response.Status,
	})
}

func parseCreateRfxEventRequest(req createRfxEventRequest, tenantID uuid.UUID) (domain.CreateRfxEventInput, error) {
	ownerCompanyID, err := domain.ParseUUID(req.OwnerCompanyID, "owner_company_id")
	if err != nil {
		return domain.CreateRfxEventInput{}, err
	}

	var validFrom, validTo, responseDeadline *time.Time
	if req.ValidFrom != nil {
		validFrom, err = domain.ParseDate(*req.ValidFrom, "valid_from")
		if err != nil {
			return domain.CreateRfxEventInput{}, err
		}
	}
	if req.ValidTo != nil {
		validTo, err = domain.ParseDate(*req.ValidTo, "valid_to")
		if err != nil {
			return domain.CreateRfxEventInput{}, err
		}
	}
	if req.ResponseDeadline != nil {
		responseDeadline, err = domain.ParseDateTime(*req.ResponseDeadline, "response_deadline")
		if err != nil {
			return domain.CreateRfxEventInput{}, err
		}
	}

	return domain.CreateRfxEventInput{
		TenantID:         tenantID,
		RfxNumber:        req.RfxNumber,
		RfxType:          req.RfxType,
		Category:         req.Category,
		Title:            req.Title,
		Description:      req.Description,
		OwnerCompanyID:   ownerCompanyID,
		CurrencyCode:     req.CurrencyCode,
		ValidFrom:        validFrom,
		ValidTo:          validTo,
		ResponseDeadline: responseDeadline,
	}, nil
}

func toRfxEventResponse(event *domain.RfxEvent) map[string]any {
	return map[string]any{
		"id":                event.ID.String(),
		"tenant_id":         event.TenantID.String(),
		"rfx_number":        event.RfxNumber,
		"rfx_type":          event.RfxType,
		"category":          event.Category,
		"title":             event.Title,
		"description":       event.Description,
		"owner_company_id":  event.OwnerCompanyID.String(),
		"status":            event.Status,
		"currency_code":     event.CurrencyCode,
		"valid_from":        formatDate(event.ValidFrom),
		"valid_to":          formatDate(event.ValidTo),
		"response_deadline": formatDateTime(event.ResponseDeadline),
		"created_at":        event.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":        event.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":           event.Version,
	}
}

func toRfxLotResponse(lot *domain.RfxLot) map[string]any {
	return map[string]any{
		"id":              lot.ID.String(),
		"tenant_id":       lot.TenantID.String(),
		"rfx_event_id":    lot.RfxEventID.String(),
		"lot_number":      lot.LotNumber,
		"name":            lot.Name,
		"description":     lot.Description,
		"category":        lot.Category,
		"estimated_value": lot.EstimatedValue,
		"currency_code":   lot.CurrencyCode,
		"status":          lot.Status,
	}
}

func toRfxLaneResponse(lane *domain.RfxLane) map[string]any {
	resp := map[string]any{
		"id":                     lane.ID.String(),
		"tenant_id":              lane.TenantID.String(),
		"rfx_lot_id":             lane.RfxLotID.String(),
		"transport_mode":         lane.TransportMode,
		"equipment_type":         lane.EquipmentType,
		"estimated_volume":       lane.EstimatedVolume,
		"volume_unit":            lane.VolumeUnit,
		"required_service_level": lane.RequiredServiceLevel,
	}
	if lane.OriginLocationID != nil {
		resp["origin_location_id"] = lane.OriginLocationID.String()
	}
	if lane.DestinationLocationID != nil {
		resp["destination_location_id"] = lane.DestinationLocationID.String()
	}
	return resp
}

func toRfxParticipantResponse(participant *domain.RfxParticipant) map[string]any {
	return map[string]any{
		"id":               participant.ID.String(),
		"tenant_id":        participant.TenantID.String(),
		"rfx_event_id":     participant.RfxEventID.String(),
		"company_id":       participant.CompanyID.String(),
		"participant_type": participant.ParticipantType,
		"status":           participant.Status,
		"invited_at":       participant.InvitedAt,
	}
}

func toRfxResponseResponse(response *domain.RfxResponse) map[string]any {
	return map[string]any{
		"id":                     response.ID.String(),
		"tenant_id":              response.TenantID.String(),
		"rfx_event_id":           response.RfxEventID.String(),
		"participant_company_id": response.ParticipantCompanyID.String(),
		"status":                 response.Status,
		"submitted_at":           formatDateTime(response.SubmittedAt),
		"created_at":             response.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":             response.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":                response.Version,
	}
}

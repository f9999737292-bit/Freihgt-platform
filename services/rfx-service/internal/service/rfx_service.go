package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type RfxStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	CreateEvent(ctx context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error)
	GetEventByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error)
	ListEvents(ctx context.Context, filter domain.ListRfxEventsFilter) ([]domain.RfxEvent, int, error)
	UpdateEvent(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateRfxEventInput) (*domain.RfxEvent, error)
	UpdateEventStatus(ctx context.Context, id, tenantID uuid.UUID, expectedStatus, newStatus string) (*domain.RfxEvent, error)
	UpdateEventResponseDeadline(ctx context.Context, id, tenantID uuid.UUID, deadline *time.Time, expectedVersion int) (*domain.RfxEvent, error)
	CountLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) (int, error)
	CloseExpiredResponses(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error)
	CreateLot(ctx context.Context, in domain.CreateRfxLotInput) (*domain.RfxLot, error)
	ListLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error)
	CreateLane(ctx context.Context, in domain.CreateRfxLaneInput) (*domain.RfxLane, error)
	AddParticipant(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error)
	ListParticipants(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxParticipant, error)
	ParticipantExists(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (bool, error)
	CreateResponse(ctx context.Context, in domain.CreateRfxResponseInput) (*domain.RfxResponse, error)
	GetResponseByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error)
	SubmitResponse(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.RfxResponse, error)
}

type RfxService struct {
	repo  RfxStore
	audit AuditRecorder
}

func NewRfxService(repo RfxStore, audit AuditRecorder) *RfxService {
	return &RfxService{repo: repo, audit: audit}
}

func (s *RfxService) CreateEvent(ctx context.Context, actor domain.ActorContext, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	if err := domain.ValidateCreateRfxEventInput(in); err != nil {
		return nil, err
	}
	exists, err := s.repo.CompanyExists(ctx, in.OwnerCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("owner_company_id not found")
	}
	event, err := s.repo.CreateEvent(ctx, in)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", event.ID, "create", map[string]any{"rfx_type": event.RfxType})
	return event, nil
}

func (s *RfxService) GetEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.repo.GetEventByID(ctx, id, actor.TenantID)
}

func (s *RfxService) ListEvents(ctx context.Context, actor domain.ActorContext, filter domain.ListRfxEventsFilter) ([]domain.RfxEvent, int, error) {
	if err := actor.Validate(); err != nil {
		return nil, 0, err
	}
	filter.TenantID = actor.TenantID
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListRfxEventsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.ListEvents(ctx, filter)
}

func (s *RfxService) UpdateEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID, in domain.UpdateRfxEventInput) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateUpdateRfxEvent(event.Status); err != nil {
		return nil, err
	}
	if in.ResponseDeadline != nil {
		if err := domain.ValidateFutureDeadline(in.ResponseDeadline, "response_deadline"); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateEvent(ctx, id, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", id, "update", nil)
	return updated, nil
}

func (s *RfxService) PublishEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	lotCount, err := s.repo.CountLotsByEvent(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePublishRfxEventWithLots(event, lotCount); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateEventStatus(ctx, id, actor.TenantID, domain.RfxStatusDraft, domain.RfxStatusPublished)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", id, "publish", nil)
	return updated, nil
}

func (s *RfxService) CancelEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCancelRfxEventExtended(event.Status); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateEventStatus(ctx, id, actor.TenantID, event.Status, domain.RfxStatusCancelled)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", id, "cancel", nil)
	return updated, nil
}

func (s *RfxService) TransitionEvent(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, command domain.RfxTransitionCommand) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	profile := domain.LifecycleProfileForType(event.RfxType)
	target, err := domain.ResolveRfxTransitionTarget(profile, event.Status, command)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateEventStatus(ctx, eventID, actor.TenantID, event.Status, target)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", eventID, "transition", map[string]any{
		"command": string(command),
		"from":    event.Status,
		"to":      target,
	})
	return updated, nil
}

func (s *RfxService) ExtendDeadline(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, deadline time.Time) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateDeadlineExtensionStatus(event.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateFutureDeadline(&deadline, "response_deadline"); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateEventResponseDeadline(ctx, eventID, actor.TenantID, &deadline, event.Version)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", eventID, "extend_deadline", map[string]any{
		"response_deadline": deadline.UTC().Format(time.RFC3339),
	})
	return updated, nil
}

func (s *RfxService) CloseExpiredResponses(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error) {
	if tenantID == uuid.Nil {
		return 0, apperrors.Unauthorized("tenant context is required")
	}
	return s.repo.CloseExpiredResponses(ctx, tenantID, now)
}

func (s *RfxService) CreateLot(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, in domain.CreateRfxLotInput) (*domain.RfxLot, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.RfxEventID = eventID
	if _, err := s.repo.GetEventByID(ctx, eventID, in.TenantID); err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateRfxLotInput(in); err != nil {
		return nil, err
	}
	return s.repo.CreateLot(ctx, in)
}

func (s *RfxService) ListLots(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) ([]domain.RfxLot, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID); err != nil {
		return nil, err
	}
	return s.repo.ListLotsByEvent(ctx, eventID, actor.TenantID)
}

func (s *RfxService) CreateLane(ctx context.Context, actor domain.ActorContext, lotID uuid.UUID, in domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.RfxLotID = lotID
	if err := domain.ValidateCreateRfxLaneInput(in); err != nil {
		return nil, err
	}
	return s.repo.CreateLane(ctx, in)
}

func (s *RfxService) AddParticipant(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.RfxEventID = eventID
	if err := domain.ValidateAddRfxParticipantInput(in); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetEventByID(ctx, eventID, in.TenantID); err != nil {
		return nil, err
	}
	exists, err := s.repo.CompanyExists(ctx, in.CompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("company not found")
	}
	participant, err := s.repo.AddParticipant(ctx, in)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_event", eventID, "add_participant", map[string]any{
		"company_id": in.CompanyID.String(),
	})
	return participant, nil
}

func (s *RfxService) ListParticipants(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, status *string) ([]domain.RfxParticipant, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID); err != nil {
		return nil, err
	}
	participants, err := s.repo.ListParticipants(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return participants, nil
	}
	filtered := make([]domain.RfxParticipant, 0)
	for _, p := range participants {
		if p.Status == *status {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func (s *RfxService) CreateResponse(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, in domain.CreateRfxResponseInput) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.RfxEventID = eventID
	if err := domain.ValidateCreateRfxResponseInput(in); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateRfxResponse(event.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateResponseDeadlineOpen(event.ResponseDeadline, nowUTC()); err != nil {
		return nil, err
	}
	exists, err := s.repo.ParticipantExists(ctx, eventID, in.ParticipantCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("participant not found")
	}
	response, err := s.repo.CreateResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_response", response.ID, "create", map[string]any{
		"rfx_event_id": eventID.String(),
	})
	return response, nil
}

func (s *RfxService) SubmitResponse(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSubmitRfxResponse(response.Status); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, response.RfxEventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSubmissionBeforeDeadline(event.ResponseDeadline, nowUTC()); err != nil {
		return nil, err
	}
	submitted, err := s.repo.SubmitResponse(ctx, id, actor.TenantID, auditUser(actor))
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "rfx_response", id, "submit", nil)
	return submitted, nil
}

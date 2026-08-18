package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
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
	ListExpiredResponseOpenEvents(ctx context.Context, now time.Time, limit int) ([]domain.ExpiredResponseOpenEvent, error)
	CreateLot(ctx context.Context, in domain.CreateRfxLotInput) (*domain.RfxLot, error)
	ListLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error)
	GetLotOwnerContext(ctx context.Context, lotID, tenantID uuid.UUID) (*domain.LotOwnerContext, error)
	CreateLane(ctx context.Context, in domain.CreateRfxLaneInput) (*domain.RfxLane, error)
	AddParticipant(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error)
	ListParticipants(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxParticipant, error)
	ParticipantExists(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (bool, error)
	CreateResponse(ctx context.Context, in domain.CreateRfxResponseInput) (*domain.RfxResponse, error)
	GetResponseByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error)
	SubmitResponse(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.RfxResponse, error)
}

type RfxService struct {
	repo   RfxStore
	audit  AuditRecorder
	actors ActorResolver
	atomic *atomicServices
}

func NewRfxService(repo RfxStore, audit AuditRecorder, actors ActorResolver) *RfxService {
	return &RfxService{repo: repo, audit: audit, actors: actors}
}

func NewRfxServiceWithAtomic(pool *pgxpool.Pool, rfxRepo *repository.RfxRepository, auditRepo *repository.AuditRepository, actors ActorResolver) *RfxService {
	s := NewRfxService(rfxRepo, auditRepo, actors)
	if pool != nil {
		s.atomic = newAtomicServices(pool, rfxRepo, auditRepo, nil, nil)
	}
	return s
}

func (s *RfxService) runRfx(ctx context.Context, fn func(rfx RfxStore, audit AuditRecorder) error) error {
	if s.atomic != nil {
		return s.atomic.runRfx(ctx, fn)
	}
	return fn(s.repo, s.audit)
}

func (s *RfxService) resolveActor(ctx context.Context, actor domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	if err := actor.Validate(); err != nil {
		return domain.ActorKindUnknown, nil, err
	}
	if s.actors == nil {
		return domain.ActorKindBuyer, nil, nil
	}
	return s.actors.ResolveActorKind(ctx, actor)
}

func (s *RfxService) CreateEvent(ctx context.Context, actor domain.ActorContext, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	ownerCompanyID, err := s.resolveCreateOwnerCompanyID(ctx, actor, in.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	in.OwnerCompanyID = ownerCompanyID
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
	var event *domain.RfxEvent
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		created, err := rfx.CreateEvent(ctx, in)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", created.ID, "create", map[string]any{"rfx_type": created.RfxType}); err != nil {
			return err
		}
		event = created
		return nil
	})
	return event, err
}

func (s *RfxService) GetEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	event, err := s.repo.GetEventByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind == domain.ActorKindBuyer {
		if _, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID); err != nil {
			return nil, err
		}
	}
	return event, nil
}

func (s *RfxService) ListEvents(ctx context.Context, actor domain.ActorContext, filter domain.ListRfxEventsFilter) ([]domain.RfxEvent, int, error) {
	if err := actor.Validate(); err != nil {
		return nil, 0, err
	}
	filter.TenantID = actor.TenantID
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := s.applyBuyerListScope(ctx, actor, &filter); err != nil {
		return nil, 0, err
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
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
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
	var updated *domain.RfxEvent
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		result, err := rfx.UpdateEvent(ctx, id, actor.TenantID, in)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", id, "update", nil); err != nil {
			return err
		}
		updated = result
		return nil
	})
	return updated, err
}

func (s *RfxService) PublishEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
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
	var updated *domain.RfxEvent
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		result, err := rfx.UpdateEventStatus(ctx, id, actor.TenantID, domain.RfxStatusDraft, domain.RfxStatusPublished)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", id, "publish", nil); err != nil {
			return err
		}
		updated = result
		return nil
	})
	return updated, err
}

func (s *RfxService) CancelEvent(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCancelRfxEventExtended(event.Status); err != nil {
		return nil, err
	}
	var updated *domain.RfxEvent
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		result, err := rfx.UpdateEventStatus(ctx, id, actor.TenantID, event.Status, domain.RfxStatusCancelled)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", id, "cancel", nil); err != nil {
			return err
		}
		updated = result
		return nil
	})
	return updated, err
}

func (s *RfxService) TransitionEvent(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, command domain.RfxTransitionCommand) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	profile := domain.LifecycleProfileForType(event.RfxType)
	target, err := domain.ResolveRfxTransitionTarget(profile, event.Status, command)
	if err != nil {
		return nil, err
	}
	var updated *domain.RfxEvent
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		result, err := rfx.UpdateEventStatus(ctx, eventID, actor.TenantID, event.Status, target)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", eventID, "transition", map[string]any{
			"command": string(command),
			"from":    event.Status,
			"to":      target,
		}); err != nil {
			return err
		}
		updated = result
		return nil
	})
	return updated, err
}

func (s *RfxService) ExtendDeadline(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, deadline time.Time) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateDeadlineExtensionStatus(event.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateFutureDeadline(&deadline, "response_deadline"); err != nil {
		return nil, err
	}
	var updated *domain.RfxEvent
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		result, err := rfx.UpdateEventResponseDeadline(ctx, eventID, actor.TenantID, &deadline, event.Version)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", eventID, "extend_deadline", map[string]any{
			"response_deadline": deadline.UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		updated = result
		return nil
	})
	return updated, err
}

func (s *RfxService) CloseExpiredResponses(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error) {
	if tenantID == uuid.Nil {
		return 0, apperrors.Unauthorized("tenant context is required")
	}
	return s.repo.CloseExpiredResponses(ctx, tenantID, now)
}

const autoCloseAuditAction = "auto_close_responses"

func (s *RfxService) ProcessExpiredResponseDeadlines(ctx context.Context, now time.Time, batchSize int) (examined int, closed int, failures int, err error) {
	if batchSize <= 0 {
		batchSize = 50
	}
	events, err := s.repo.ListExpiredResponseOpenEvents(ctx, now, batchSize)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, event := range events {
		if ctx.Err() != nil {
			break
		}
		examined++
		didClose, closeErr := s.autoCloseExpiredResponseEvent(ctx, event, now)
		if closeErr != nil {
			failures++
			continue
		}
		if didClose {
			closed++
		}
	}
	return examined, closed, failures, nil
}

func (s *RfxService) autoCloseExpiredResponseEvent(ctx context.Context, event domain.ExpiredResponseOpenEvent, now time.Time) (bool, error) {
	if event.Status != domain.RfxStatusResponsesOpen {
		return false, nil
	}
	if event.ResponseDeadline == nil || event.ResponseDeadline.After(now.UTC()) {
		return false, nil
	}
	metadata := map[string]any{
		"from_status": domain.RfxStatusResponsesOpen,
		"to_status":   domain.RfxStatusResponsesClosed,
		"actor_type":  domain.AuditActorTypeSystem,
	}
	if event.ResponseDeadline != nil {
		metadata["response_deadline"] = event.ResponseDeadline.UTC().Format(time.RFC3339)
	}
	closed := false
	err := s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		_, err := rfx.UpdateEventStatus(ctx, event.ID, event.TenantID, domain.RfxStatusResponsesOpen, domain.RfxStatusResponsesClosed)
		if err != nil {
			var appErr *apperrors.AppError
			if errors.As(err, &appErr) && appErr.Code == apperrors.CodeConflict {
				return nil
			}
			return err
		}
		if err := recordSystemAudit(ctx, audit, event.TenantID, "rfx_event", event.ID, autoCloseAuditAction, metadata); err != nil {
			return err
		}
		closed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return closed, nil
}

func (s *RfxService) CreateLot(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, in domain.CreateRfxLotInput) (*domain.RfxLot, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.RfxEventID = eventID
	event, err := s.repo.GetEventByID(ctx, eventID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID); err != nil {
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
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind == domain.ActorKindBuyer {
		if _, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListLotsByEvent(ctx, eventID, actor.TenantID)
}

func (s *RfxService) requireLotOwnerCompanyAccess(ctx context.Context, actor domain.ActorContext, lotID uuid.UUID) (*domain.LotOwnerContext, error) {
	lotCtx, err := s.repo.GetLotOwnerContext(ctx, lotID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireOwnerCompanyAccess(ctx, actor, lotCtx.OwnerCompanyID); err != nil {
		return nil, err
	}
	return lotCtx, nil
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
	if _, err := s.requireLotOwnerCompanyAccess(ctx, actor, lotID); err != nil {
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
	event, err := s.repo.GetEventByID(ctx, eventID, in.TenantID)
	if err != nil {
		return nil, err
	}
	ownerCompanyID, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	exists, err := s.repo.CompanyExists(ctx, in.CompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("company not found")
	}
	var participant *domain.RfxParticipant
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		created, err := rfx.AddParticipant(ctx, in)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, ownerCompanyID, "rfx_event", eventID, "add_participant", map[string]any{
			"company_id": in.CompanyID.String(),
		}); err != nil {
			return err
		}
		participant = created
		return nil
	})
	return participant, err
}

func (s *RfxService) ListParticipants(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, status *string) ([]domain.RfxParticipant, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind == domain.ActorKindBuyer {
		if _, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID); err != nil {
			return nil, err
		}
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
	_, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	participantCompanyID, err := domain.ResolveCarrierCompanyID(in.ParticipantCompanyID, carrierIDs)
	if err != nil {
		return nil, err
	}
	in.ParticipantCompanyID = participantCompanyID
	exists, err := s.repo.ParticipantExists(ctx, eventID, in.ParticipantCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("participant not found")
	}
	var response *domain.RfxResponse
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		created, err := rfx.CreateResponse(ctx, in)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, participantCompanyID, "rfx_response", created.ID, "create", map[string]any{
			"rfx_event_id": eventID.String(),
		}); err != nil {
			return err
		}
		response = created
		return nil
	})
	return response, err
}

func (s *RfxService) SubmitResponse(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	_, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	carrierCompanyID, err := domain.ResolveCarrierCompanyID(response.ParticipantCompanyID, carrierIDs)
	if err != nil {
		return nil, apperrors.NotFound("rfx response not found")
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
	var submitted *domain.RfxResponse
	err = s.runRfx(ctx, func(rfx RfxStore, audit AuditRecorder) error {
		result, err := rfx.SubmitResponse(ctx, id, actor.TenantID, auditUser(actor))
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, carrierCompanyID, "rfx_response", id, "submit", nil); err != nil {
			return err
		}
		submitted = result
		return nil
	})
	return submitted, err
}

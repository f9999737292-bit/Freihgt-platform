package service

import (
	"context"

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
	repo RfxStore
}

func NewRfxService(repo RfxStore) *RfxService {
	return &RfxService{repo: repo}
}

func (s *RfxService) CreateEvent(ctx context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
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
	return s.repo.CreateEvent(ctx, in)
}

func (s *RfxService) GetEvent(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	if id == uuid.Nil || tenantID == uuid.Nil {
		return nil, apperrors.Validation("id and tenant_id are required", map[string]any{})
	}
	return s.repo.GetEventByID(ctx, id, tenantID)
}

func (s *RfxService) ListEvents(ctx context.Context, filter domain.ListRfxEventsFilter) ([]domain.RfxEvent, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListRfxEventsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.ListEvents(ctx, filter)
}

func (s *RfxService) UpdateEvent(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateRfxEventInput) (*domain.RfxEvent, error) {
	event, err := s.repo.GetEventByID(ctx, id, tenantID)
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
	return s.repo.UpdateEvent(ctx, id, tenantID, in)
}

func (s *RfxService) PublishEvent(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	event, err := s.repo.GetEventByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePublishRfxEvent(event.Status); err != nil {
		return nil, err
	}
	return s.repo.UpdateEventStatus(ctx, id, tenantID, domain.RfxStatusDraft, domain.RfxStatusPublished)
}

func (s *RfxService) CancelEvent(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	event, err := s.repo.GetEventByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCancelRfxEvent(event.Status); err != nil {
		return nil, err
	}
	return s.repo.UpdateEventStatus(ctx, id, tenantID, event.Status, domain.RfxStatusCancelled)
}

func (s *RfxService) CreateLot(ctx context.Context, eventID uuid.UUID, in domain.CreateRfxLotInput) (*domain.RfxLot, error) {
	in.RfxEventID = eventID
	if _, err := s.repo.GetEventByID(ctx, eventID, in.TenantID); err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateRfxLotInput(in); err != nil {
		return nil, err
	}
	return s.repo.CreateLot(ctx, in)
}

func (s *RfxService) ListLots(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error) {
	if _, err := s.repo.GetEventByID(ctx, eventID, tenantID); err != nil {
		return nil, err
	}
	return s.repo.ListLotsByEvent(ctx, eventID, tenantID)
}

func (s *RfxService) CreateLane(ctx context.Context, lotID uuid.UUID, in domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
	in.RfxLotID = lotID
	if err := domain.ValidateCreateRfxLaneInput(in); err != nil {
		return nil, err
	}
	return s.repo.CreateLane(ctx, in)
}

func (s *RfxService) AddParticipant(ctx context.Context, eventID uuid.UUID, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
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
	return s.repo.AddParticipant(ctx, in)
}

func (s *RfxService) ListParticipants(ctx context.Context, eventID, tenantID uuid.UUID, status *string) ([]domain.RfxParticipant, error) {
	if _, err := s.repo.GetEventByID(ctx, eventID, tenantID); err != nil {
		return nil, err
	}
	participants, err := s.repo.ListParticipants(ctx, eventID, tenantID)
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

func (s *RfxService) CreateResponse(ctx context.Context, eventID uuid.UUID, in domain.CreateRfxResponseInput) (*domain.RfxResponse, error) {
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
	exists, err := s.repo.ParticipantExists(ctx, eventID, in.ParticipantCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("participant not found")
	}
	return s.repo.CreateResponse(ctx, in)
}

func (s *RfxService) SubmitResponse(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error) {
	response, err := s.repo.GetResponseByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSubmitRfxResponse(response.Status); err != nil {
		return nil, err
	}
	return s.repo.SubmitResponse(ctx, id, tenantID, nil)
}

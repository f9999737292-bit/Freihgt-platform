package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type LateSubmissionService struct {
	lateRepo *repository.LateSubmissionRepository
	rfxRepo  *repository.RfxRepository
	idemRepo *repository.IdempotencyRepository
	audit    AuditRecorder
	tx       *repository.TransactionRunner
	auth     *RfxService
	nowFn    func() time.Time
}

func NewLateSubmissionService(
	pool *pgxpool.Pool,
	lateRepo *repository.LateSubmissionRepository,
	rfxRepo *repository.RfxRepository,
	idemRepo *repository.IdempotencyRepository,
	audit AuditRecorder,
	auth *RfxService,
) *LateSubmissionService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &LateSubmissionService{
		lateRepo: lateRepo,
		rfxRepo:  rfxRepo,
		idemRepo: idemRepo,
		audit:    audit,
		tx:       tx,
		auth:     auth,
		nowFn:    nowUTC,
	}
}

func (s *LateSubmissionService) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		s.nowFn = fn
	}
}

func (s *LateSubmissionService) now() time.Time {
	return s.nowFn().UTC()
}

func (s *LateSubmissionService) CreateRequest(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
	idempotencyKey string,
	in domain.CreateLateSubmissionRequestInput,
) (*domain.LateSubmissionRequest, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateLateSubmissionInput(in); err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}

	carrierCompanyID, err := s.auth.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}

	scope := repository.IdempotencyScope{
		TenantID: actor.TenantID, ActorID: actor.UserID,
		Operation: domain.LateSubmissionOperationCreateRequest, AggregateScope: eventID,
	}
	payload := struct {
		CarrierCompanyID uuid.UUID                               `json:"carrier_company_id"`
		Input            domain.CreateLateSubmissionRequestInput `json:"input"`
	}{CarrierCompanyID: carrierCompanyID, Input: in}
	requestBodyHash, err := hashRequestBody(payload)
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadRequestReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("late submission service misconfigured", nil)
	}

	now := s.now()
	var created *domain.LateSubmissionRequest
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		lateRepo := s.lateRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		event, err := lateRepo.LockEventForUpdate(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateLateRequestEligibility(event.ResponseDeadline, now); err != nil {
			return err
		}
		active, err := lateRepo.GetActiveByEventAndCarrier(ctx, actor.TenantID, eventID, carrierCompanyID)
		if err != nil {
			return err
		}
		if err := domain.CanCreateLateSubmissionRequest(active, now); err != nil {
			return err
		}
		participant, err := s.rfxRepo.GetParticipantByEventAndCompany(ctx, eventID, carrierCompanyID, actor.TenantID)
		if err != nil {
			return err
		}
		participantID := participant.ID
		out, err := lateRepo.CreateRequest(ctx, domain.LateSubmissionRequest{
			TenantID: actor.TenantID, RfxEventID: eventID, CarrierCompanyID: carrierCompanyID,
			ParticipantID: &participantID, ReasonCode: in.ReasonCode, ReasonText: strings.TrimSpace(in.ReasonText),
			RequestedUntil: in.RequestedUntil.UTC(), RequestedBy: actor.UserID,
		})
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, s.auditTx(tx), actor, carrierCompanyID, "rfx_late_submission_request", out.ID, domain.LateSubmissionAuditRequested, map[string]any{
			"rfx_event_id": eventID.String(), "reason_code": string(in.ReasonCode),
		}); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return apperrors.Internal("failed to encode idempotent create payload", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID: scope.TenantID, ActorID: scope.ActorID, Operation: scope.Operation,
			AggregateScope: scope.AggregateScope, IdempotencyKey: idempotencyKey,
			RequestBodyHash: requestBodyHash, ResponseStatus: http.StatusCreated, ResponseBody: body,
			ExpiresAt: now.Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		created = out
		return nil
	})
	if err != nil {
		if replay, replayErr := s.loadRequestReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, err
	}
	return created, nil
}

func (s *LateSubmissionService) ListOwnRequests(ctx context.Context, actor domain.ActorContext, eventID, requestedCarrierCompanyID uuid.UUID) ([]domain.LateSubmissionRequest, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.auth.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	return s.lateRepo.ListOwnByEventAndCarrier(ctx, actor.TenantID, eventID, carrierCompanyID)
}

func (s *LateSubmissionService) BuyerListRequests(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) ([]domain.LateSubmissionRequest, error) {
	event, err := s.authorizeBuyerRead(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	_ = event
	return s.lateRepo.ListByEvent(ctx, actor.TenantID, eventID)
}

func (s *LateSubmissionService) Approve(
	ctx context.Context,
	actor domain.ActorContext,
	eventID, requestID uuid.UUID,
	idempotencyKey string,
	in domain.ApproveLateSubmissionInput,
) (*domain.LateSubmissionRequest, error) {
	if err := domain.ValidateApprovedWindowInput(in.ApprovedValidFrom, in.ApprovedValidUntil); err != nil {
		return nil, err
	}
	return s.decide(ctx, actor, eventID, requestID, idempotencyKey, in, domain.LateSubmissionOperationApprove, func(ctx context.Context, lateRepo *repository.LateSubmissionRepository, locked *domain.LateSubmissionRequest, event *domain.RfxEvent) (*domain.LateSubmissionRequest, error) {
		if err := domain.ValidateLateSubmissionTransition(locked.Status, domain.LateSubmissionStatusApproved); err != nil {
			return nil, err
		}
		comment := strings.TrimSpace(in.DecisionComment)
		var commentPtr *string
		if comment != "" {
			commentPtr = &comment
		}
		return lateRepo.Approve(ctx, requestID, actor.TenantID, eventID, in.ExpectedVersion, in.ApprovedValidFrom, in.ApprovedValidUntil, actor.UserID, commentPtr)
	}, domain.LateSubmissionAuditApproved)
}

func (s *LateSubmissionService) Reject(
	ctx context.Context,
	actor domain.ActorContext,
	eventID, requestID uuid.UUID,
	idempotencyKey string,
	in domain.RejectLateSubmissionInput,
) (*domain.LateSubmissionRequest, error) {
	return s.decide(ctx, actor, eventID, requestID, idempotencyKey, in, domain.LateSubmissionOperationReject, func(ctx context.Context, lateRepo *repository.LateSubmissionRepository, locked *domain.LateSubmissionRequest, event *domain.RfxEvent) (*domain.LateSubmissionRequest, error) {
		if err := domain.ValidateLateSubmissionTransition(locked.Status, domain.LateSubmissionStatusRejected); err != nil {
			return nil, err
		}
		comment := strings.TrimSpace(in.DecisionComment)
		var commentPtr *string
		if comment != "" {
			commentPtr = &comment
		}
		return lateRepo.Reject(ctx, requestID, actor.TenantID, eventID, in.ExpectedVersion, actor.UserID, commentPtr)
	}, domain.LateSubmissionAuditRejected)
}

func (s *LateSubmissionService) ResolveActivePermission(
	ctx context.Context,
	tenantID, eventID, carrierCompanyID uuid.UUID,
	deadline *time.Time,
	now time.Time,
) (*domain.LateSubmissionRequest, error) {
	if !domain.ResponseDeadlinePassed(deadline, now) {
		return nil, nil
	}
	req, err := s.lateRepo.GetActiveByEventAndCarrier(ctx, tenantID, eventID, carrierCompanyID)
	if err != nil {
		return nil, err
	}
	if req == nil || req.Status != domain.LateSubmissionStatusApproved {
		return req, nil
	}
	if domain.EffectiveLateSubmissionStatus(req, now) == domain.LateSubmissionStatusExpired {
		return nil, nil
	}
	return req, nil
}

func (s *LateSubmissionService) EnsureApprovedWindowTx(
	ctx context.Context,
	tx pgx.Tx,
	requestID, tenantID, eventID, carrierCompanyID uuid.UUID,
	now time.Time,
) error {
	lateRepo := s.lateRepo.WithTx(tx)
	locked, err := lateRepo.LockApprovedForConsume(ctx, requestID, tenantID, eventID, carrierCompanyID)
	if err != nil {
		return err
	}
	return domain.ValidateApprovedWindowActive(locked, now)
}

func (s *LateSubmissionService) ConsumePermissionWithTx(
	ctx context.Context,
	tx pgx.Tx,
	requestID, tenantID, eventID, carrierCompanyID uuid.UUID,
	now time.Time,
) (*domain.LateSubmissionRequest, error) {
	lateRepo := s.lateRepo.WithTx(tx)
	locked, err := lateRepo.LockApprovedForConsume(ctx, requestID, tenantID, eventID, carrierCompanyID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateApprovedWindowActive(locked, now); err != nil {
		return nil, err
	}
	consumed, err := lateRepo.Consume(ctx, locked.ID, tenantID, eventID, carrierCompanyID, locked.Version, now)
	if err != nil {
		return nil, err
	}
	actor := domain.ActorContext{TenantID: tenantID, UserID: locked.RequestedBy}
	if err := recordAudit(ctx, s.auditTx(tx), actor, carrierCompanyID, "rfx_late_submission_request", locked.ID, domain.LateSubmissionAuditConsumed, map[string]any{
		"rfx_event_id": eventID.String(),
	}); err != nil {
		return nil, err
	}
	return consumed, nil
}

func (s *LateSubmissionService) auditTx(tx pgx.Tx) AuditRecorder {
	if tx == nil {
		return s.audit
	}
	if ar, ok := s.audit.(*repository.AuditRepository); ok {
		return ar.WithTx(tx)
	}
	return s.audit
}

func (s *LateSubmissionService) decide(
	ctx context.Context,
	actor domain.ActorContext,
	eventID, requestID uuid.UUID,
	idempotencyKey string,
	in any,
	operation string,
	apply func(context.Context, *repository.LateSubmissionRepository, *domain.LateSubmissionRequest, *domain.RfxEvent) (*domain.LateSubmissionRequest, error),
	auditAction string,
) (*domain.LateSubmissionRequest, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	event, err := s.authorizeBuyerManage(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	scope := repository.IdempotencyScope{
		TenantID: actor.TenantID, ActorID: actor.UserID,
		Operation: operation, AggregateScope: requestID,
	}
	payload := struct {
		EventID   uuid.UUID `json:"event_id"`
		RequestID uuid.UUID `json:"request_id"`
		Body      any       `json:"body"`
	}{EventID: eventID, RequestID: requestID, Body: in}
	requestBodyHash, err := hashRequestBody(payload)
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadRequestReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("late submission service misconfigured", nil)
	}

	var decided *domain.LateSubmissionRequest
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		lateRepo := s.lateRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		if _, err := lateRepo.LockEventForUpdate(ctx, eventID, actor.TenantID); err != nil {
			return err
		}
		locked, err := lateRepo.LockForDecision(ctx, requestID, actor.TenantID, eventID)
		if err != nil {
			return err
		}
		out, err := apply(ctx, lateRepo, locked, event)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, s.auditTx(tx), actor, event.OwnerCompanyID, "rfx_late_submission_request", out.ID, auditAction, map[string]any{
			"rfx_event_id": eventID.String(), "status": string(out.Status),
		}); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return apperrors.Internal("failed to encode idempotent decision payload", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID: scope.TenantID, ActorID: scope.ActorID, Operation: scope.Operation,
			AggregateScope: scope.AggregateScope, IdempotencyKey: idempotencyKey,
			RequestBodyHash: requestBodyHash, ResponseStatus: http.StatusOK, ResponseBody: body,
			ExpiresAt: s.now().Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		decided = out
		return nil
	})
	if err != nil {
		if replay, replayErr := s.loadRequestReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, err
	}
	return decided, nil
}

func (s *LateSubmissionService) authorizeBuyerRead(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.auth.requireBuyerActor(ctx, actor); err != nil {
		return nil, err
	}
	buyerCompanyIDs, err := s.auth.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !domain.ContainsCompanyID(buyerCompanyIDs, event.OwnerCompanyID) {
		return nil, apperrors.NotFound("rfx event not found")
	}
	return event, nil
}

func (s *LateSubmissionService) authorizeBuyerManage(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, error) {
	event, err := s.authorizeBuyerRead(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	resolver, ok := s.auth.actors.(CompanyMembershipResolver)
	if !ok {
		return nil, apperrors.Forbidden("buyer manage permission is required")
	}
	roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !domain.HasBuyerManageRole(roles) {
		return nil, apperrors.Forbidden("buyer manage permission is required")
	}
	return event, nil
}

func (s *LateSubmissionService) loadRequestReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey string,
	requestBodyHash string,
) (*domain.LateSubmissionRequest, error) {
	record, err := s.idemRepo.Get(ctx, scope, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	if record.RequestBodyHash != requestBodyHash {
		return nil, apperrors.Conflict("idempotency key was already used with a different request body", map[string]any{"field": "Idempotency-Key"})
	}
	var req domain.LateSubmissionRequest
	if err := json.Unmarshal(record.ResponseBody, &req); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent late submission replay payload", err)
	}
	return &req, nil
}

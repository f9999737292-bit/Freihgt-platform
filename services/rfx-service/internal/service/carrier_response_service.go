package service

import (
	"context"
	"encoding/json"
	"errors"
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

type CarrierResponseStore interface {
	GetEventByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error)
	ParticipantExists(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (bool, error)
	CreateResponse(ctx context.Context, in domain.CreateRfxResponseInput) (*domain.RfxResponse, error)
	GetResponseByEventAndCompany(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (*domain.RfxResponse, error)
	GetResponseByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error)
	PinResponseVersion(ctx context.Context, id, tenantID, versionID uuid.UUID) (*domain.RfxResponse, error)
	LockResponseForUpdate(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error)
	UpdateResponseAfterSave(ctx context.Context, id, tenantID uuid.UUID, expectedSaveVersion int64, lastSavedBy uuid.UUID, completionPercent float64) (*domain.RfxResponse, error)
	SubmitResponse(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.RfxResponse, error)
}

type CarrierAnswerStore interface {
	ListByResponse(ctx context.Context, responseID, tenantID uuid.UUID) ([]domain.CarrierAnswer, error)
	UpsertBatch(ctx context.Context, tenantID, responseID uuid.UUID, patches []domain.AnswerPatchItem, updatedBy uuid.UUID) error
	DeleteByQuestionIDs(ctx context.Context, responseID, tenantID uuid.UUID, questionIDs []uuid.UUID) error
}

type CarrierQuestionnaireStore interface {
	GetPublishedVersionForEvent(ctx context.Context, eventID, tenantID uuid.UUID) (*domain.RfxVersion, error)
	LoadQuestionnaire(ctx context.Context, versionID, tenantID uuid.UUID) (*domain.QuestionnaireDefinition, error)
}

type CarrierResponseService struct {
	rfx     CarrierResponseStore
	answers CarrierAnswerStore
	q       CarrierQuestionnaireStore
	audit   AuditRecorder
	actors  ActorResolver
	tx      *repository.TransactionRunner
	auth    *RfxService
	scoring ScoringTrigger
	late    LateSubmissionCoordinator
	idem    *repository.IdempotencyRepository
}

type ScoringTrigger interface {
	CalculateForSubmittedResponse(ctx context.Context, tenantID, responseID uuid.UUID) (*domain.ScoringRunResult, error)
}

type LateSubmissionCoordinator interface {
	ResolveActivePermission(ctx context.Context, tenantID, eventID, carrierCompanyID uuid.UUID, deadline *time.Time, now time.Time) (*domain.LateSubmissionRequest, error)
	EnsureApprovedWindowTx(ctx context.Context, tx pgx.Tx, requestID, tenantID, eventID, carrierCompanyID uuid.UUID, now time.Time) error
	ConsumePermissionWithTx(ctx context.Context, tx pgx.Tx, requestID, tenantID, eventID, carrierCompanyID uuid.UUID, now time.Time) (*domain.LateSubmissionRequest, error)
}

func NewCarrierResponseService(
	pool *pgxpool.Pool,
	rfx *repository.RfxRepository,
	answers *repository.AnswerRepository,
	q *repository.QuestionnaireRepository,
	audit AuditRecorder,
	actors ActorResolver,
	auth *RfxService,
) *CarrierResponseService {
	return NewCarrierResponseServiceWithScoring(pool, rfx, answers, q, audit, actors, auth, nil)
}

func NewCarrierResponseServiceWithScoring(
	pool *pgxpool.Pool,
	rfx *repository.RfxRepository,
	answers *repository.AnswerRepository,
	q *repository.QuestionnaireRepository,
	audit AuditRecorder,
	actors ActorResolver,
	auth *RfxService,
	scoring ScoringTrigger,
) *CarrierResponseService {
	return NewCarrierResponseServiceWithLateSubmission(pool, rfx, answers, q, audit, actors, auth, scoring, nil, nil)
}

func NewCarrierResponseServiceWithLateSubmission(
	pool *pgxpool.Pool,
	rfx *repository.RfxRepository,
	answers *repository.AnswerRepository,
	q *repository.QuestionnaireRepository,
	audit AuditRecorder,
	actors ActorResolver,
	auth *RfxService,
	scoring ScoringTrigger,
	late LateSubmissionCoordinator,
	idem *repository.IdempotencyRepository,
) *CarrierResponseService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &CarrierResponseService{
		rfx: rfx, answers: answers, q: q, audit: audit, actors: actors, tx: tx, auth: auth, scoring: scoring, late: late, idem: idem,
	}
}

func (s *CarrierResponseService) StartOrResume(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
) (*domain.CarrierResponseWorkspace, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, response, questionnaire, err := s.ensureCarrierResponse(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	answerList, err := s.answers.ListByResponse(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, carrierCompanyID, "rfx_response", response.ID, "response.started", map[string]any{
		"rfx_event_id":   eventID.String(),
		"save_version":   response.SaveVersion,
		"rfx_version_id": response.RfxVersionID,
	}); err != nil {
		return nil, err
	}
	return &domain.CarrierResponseWorkspace{
		Response:      *response,
		Questionnaire: *questionnaire,
		Answers:       answerList,
	}, nil
}

func (s *CarrierResponseService) GetWorkspace(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
) (*domain.CarrierResponseWorkspace, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.auth.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	response, err := s.rfx.GetResponseByEventAndCompany(ctx, eventID, carrierCompanyID, actor.TenantID)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, apperrors.NotFound("carrier response not started")
		}
		return nil, err
	}
	if response.RfxVersionID == nil {
		return nil, apperrors.Validation("carrier response is not bound to a published questionnaire version", map[string]any{"field": "rfx_version_id"})
	}
	questionnaire, err := s.q.LoadQuestionnaire(ctx, *response.RfxVersionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	answerList, err := s.answers.ListByResponse(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	return &domain.CarrierResponseWorkspace{
		Response:      *response,
		Questionnaire: *questionnaire,
		Answers:       answerList,
	}, nil
}

func (s *CarrierResponseService) SaveAnswers(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
	in domain.AnswerBatchPatchInput,
) (*domain.ResponseSaveResult, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	workspace, err := s.GetWorkspace(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	response := workspace.Response
	if err := domain.ValidateUpdateQuestionnaireResponse(response.Status); err != nil {
		return nil, err
	}
	if response.SaveVersion != in.ExpectedSaveVersion {
		return nil, saveVersionConflict(response)
	}

	rt := domain.BuildQuestionnaireRuntime(&workspace.Questionnaire)
	existing := answersMap(workspace.Answers)
	mergedPreview := domain.MergeAnswerMaps(existing, in.Answers)
	hiddenSet := make(map[uuid.UUID]struct{}, len(domain.HiddenQuestionIDs(rt, mergedPreview)))
	for _, id := range domain.HiddenQuestionIDs(rt, mergedPreview) {
		hiddenSet[id] = struct{}{}
	}
	filteredPatches := make([]domain.AnswerPatchItem, 0, len(in.Answers))
	for _, patch := range in.Answers {
		if _, hidden := hiddenSet[patch.QuestionID]; hidden {
			continue
		}
		filteredPatches = append(filteredPatches, patch)
	}
	validationErrs := domain.ValidateCarrierAnswerPatches(rt, existing, filteredPatches, false)
	if len(validationErrs) > 0 {
		return nil, validationFailed(validationErrs)
	}

	merged := domain.MergeAnswerMaps(existing, filteredPatches)
	hiddenIDs := domain.HiddenQuestionIDs(rt, merged)
	for _, id := range hiddenIDs {
		delete(merged, id)
	}
	completion := domain.ComputeCompletionPercent(rt, merged)

	event, err := s.rfx.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.auth.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	permission, err := s.resolveLatePermission(ctx, event, carrierCompanyID, now)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCarrierMutationDeadline(event.ResponseDeadline, now, permission); err != nil {
		return nil, err
	}

	lateSave := domain.ResponseDeadlinePassed(event.ResponseDeadline, now)
	var saved *domain.RfxResponse
	err = s.runTx(ctx, func(rfx *repository.RfxRepository, answers *repository.AnswerRepository, audit AuditRecorder, tx pgx.Tx) error {
		locked, err := rfx.LockResponseForUpdate(ctx, response.ID, actor.TenantID)
		if err != nil {
			return err
		}
		if locked.SaveVersion != in.ExpectedSaveVersion {
			return saveVersionConflict(*locked)
		}
		if err := domain.ValidateUpdateQuestionnaireResponse(locked.Status); err != nil {
			return err
		}
		if lateSave {
			if permission == nil || s.late == nil {
				return apperrors.Unprocessable("approved late submission permission is required", map[string]any{"field": "late_submission_request"})
			}
			if err := s.late.EnsureApprovedWindowTx(ctx, tx, permission.ID, actor.TenantID, eventID, carrierCompanyID, now); err != nil {
				return err
			}
		}
		if err := answers.UpsertBatch(ctx, actor.TenantID, locked.ID, filteredPatches, actor.UserID); err != nil {
			return err
		}
		if err := answers.DeleteByQuestionIDs(ctx, locked.ID, actor.TenantID, hiddenIDs); err != nil {
			return err
		}
		updated, err := rfx.UpdateResponseAfterSave(ctx, locked.ID, actor.TenantID, in.ExpectedSaveVersion, actor.UserID, completion)
		if err != nil {
			return err
		}
		saved = updated
		return recordAudit(ctx, audit, actor, locked.ParticipantCompanyID, "rfx_response", locked.ID, "response.answers.saved", map[string]any{
			"save_version":        updated.SaveVersion,
			"completion_percent":  updated.CompletionPercent,
			"answer_patch_count":  len(filteredPatches),
			"hidden_answer_count": len(hiddenIDs),
		})
	})
	if err != nil {
		return nil, err
	}
	lastSavedAt := nowUTC()
	if saved.LastSavedAt != nil {
		lastSavedAt = *saved.LastSavedAt
	}
	return &domain.ResponseSaveResult{
		ResponseID:        saved.ID,
		SaveVersion:       saved.SaveVersion,
		LastSavedAt:       lastSavedAt,
		LastSavedBy:       actor.UserID,
		CompletionPercent: saved.CompletionPercent,
	}, nil
}

func (s *CarrierResponseService) ValidateResponse(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
) (*domain.ResponseValidationResult, error) {
	workspace, err := s.GetWorkspace(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	return s.validateWorkspace(workspace, true)
}

func (s *CarrierResponseService) Submit(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
	expectedSaveVersion int64,
	idempotencyKey string,
) (*domain.SubmitResult, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.auth.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	workspace, err := s.GetWorkspace(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	if workspace.Response.SaveVersion != expectedSaveVersion {
		return nil, saveVersionConflict(workspace.Response)
	}
	result, err := s.validateWorkspace(workspace, true)
	if err != nil {
		return nil, err
	}
	if !result.Valid {
		_ = recordAudit(ctx, s.audit, actor, carrierCompanyID, "rfx_response", workspace.Response.ID, "response.submit.validation_failed", map[string]any{
			"blocking_error_count": result.BlockingErrorCount,
			"save_version":         workspace.Response.SaveVersion,
		})
		return nil, validationFailed(result.Errors)
	}

	event, err := s.rfx.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	lateSubmit := domain.ResponseDeadlinePassed(event.ResponseDeadline, now)
	var scope repository.IdempotencyScope
	var requestBodyHash string
	if lateSubmit {
		idempotencyKey = strings.TrimSpace(idempotencyKey)
		if idempotencyKey == "" {
			return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
		}
		if s.idem != nil {
			scope = repository.IdempotencyScope{
				TenantID: actor.TenantID, ActorID: actor.UserID,
				Operation: domain.LateSubmissionOperationSubmit, AggregateScope: workspace.Response.ID,
			}
			payload := struct {
				EventID             uuid.UUID `json:"event_id"`
				ExpectedSaveVersion int64     `json:"expected_save_version"`
				CarrierCompanyID    uuid.UUID `json:"carrier_company_id"`
			}{EventID: eventID, ExpectedSaveVersion: expectedSaveVersion, CarrierCompanyID: carrierCompanyID}
			requestBodyHash, err = hashRequestBody(payload)
			if err != nil {
				return nil, err
			}
			if replay, replayErr := s.loadSubmitReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
				return replay, replayErr
			}
		}
	}
	permission, err := s.resolveLatePermission(ctx, event, carrierCompanyID, now)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCarrierMutationDeadline(event.ResponseDeadline, now, permission); err != nil {
		return nil, err
	}

	var submitted *domain.RfxResponse
	err = s.runTx(ctx, func(rfx *repository.RfxRepository, _ *repository.AnswerRepository, audit AuditRecorder, tx pgx.Tx) error {
		locked, err := rfx.LockResponseForUpdate(ctx, workspace.Response.ID, actor.TenantID)
		if err != nil {
			return err
		}
		if locked.SaveVersion != expectedSaveVersion {
			return saveVersionConflict(*locked)
		}
		if err := domain.ValidateSubmitRfxResponse(locked.Status); err != nil {
			return err
		}
		if lateSubmit {
			if permission == nil || s.late == nil {
				return apperrors.Unprocessable("approved late submission permission is required", map[string]any{"field": "late_submission_request"})
			}
			if _, err := s.late.ConsumePermissionWithTx(ctx, tx, permission.ID, actor.TenantID, eventID, carrierCompanyID, now); err != nil {
				return err
			}
		}
		userID := actor.UserID
		out, err := rfx.SubmitResponse(ctx, locked.ID, actor.TenantID, &userID)
		if err != nil {
			return err
		}
		submitted = out
		if lateSubmit && s.idem != nil {
			result := &domain.SubmitResult{
				ResponseID: out.ID, Status: domain.MapResponseStatusToProduct(out.Status),
				SubmittedAt: now, SaveVersion: out.SaveVersion,
			}
			if out.SubmittedAt != nil {
				result.SubmittedAt = *out.SubmittedAt
			}
			body, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				return apperrors.Internal("failed to encode idempotent submit payload", marshalErr)
			}
			idemRepo := s.idem.WithTx(tx)
			if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
				TenantID: scope.TenantID, ActorID: scope.ActorID, Operation: scope.Operation,
				AggregateScope: scope.AggregateScope, IdempotencyKey: idempotencyKey,
				RequestBodyHash: requestBodyHash, ResponseStatus: http.StatusOK, ResponseBody: body,
				ExpiresAt: now.Add(24 * time.Hour),
			}); err != nil {
				return err
			}
		}
		return recordAudit(ctx, audit, actor, carrierCompanyID, "rfx_response", locked.ID, "response.submitted", map[string]any{
			"save_version": locked.SaveVersion, "late_submission": lateSubmit,
		})
	})
	if err != nil {
		return nil, err
	}
	if s.scoring != nil && submitted != nil {
		if _, scoreErr := s.scoring.CalculateForSubmittedResponse(ctx, actor.TenantID, submitted.ID); scoreErr != nil {
			_ = recordAudit(ctx, s.audit, actor, carrierCompanyID, "rfx_response", submitted.ID, "response.scoring_failed", map[string]any{
				"error_class": "SCORING_FAILURE",
			})
		}
	}
	submittedAt := nowUTC()
	if submitted.SubmittedAt != nil {
		submittedAt = *submitted.SubmittedAt
	}
	return &domain.SubmitResult{
		ResponseID:  submitted.ID,
		Status:      domain.MapResponseStatusToProduct(submitted.Status),
		SubmittedAt: submittedAt,
		SaveVersion: submitted.SaveVersion,
	}, nil
}

func (s *CarrierResponseService) GetSummary(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
) (*domain.ResponseValidationResult, error) {
	workspace, err := s.GetWorkspace(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	return s.validateWorkspace(workspace, false)
}

func (s *CarrierResponseService) ensureCarrierResponse(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	requestedCarrierCompanyID uuid.UUID,
) (uuid.UUID, *domain.RfxResponse, *domain.QuestionnaireDefinition, error) {
	carrierCompanyID, err := s.auth.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return uuid.Nil, nil, nil, err
	}
	event, err := s.rfx.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return uuid.Nil, nil, nil, err
	}
	if err := domain.ValidateCreateRfxResponse(event.Status); err != nil {
		return uuid.Nil, nil, nil, err
	}
	if err := domain.ValidateResponseDeadlineOpen(event.ResponseDeadline, nowUTC()); err != nil {
		return uuid.Nil, nil, nil, err
	}
	response, err := s.rfx.GetResponseByEventAndCompany(ctx, eventID, carrierCompanyID, actor.TenantID)
	if err != nil {
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			return uuid.Nil, nil, nil, err
		}
		published, err := s.q.GetPublishedVersionForEvent(ctx, eventID, actor.TenantID)
		if err != nil {
			return uuid.Nil, nil, nil, err
		}
		if !published.QuestionnaireEnabled {
			return uuid.Nil, nil, nil, apperrors.Validation("questionnaire is not enabled for this RFx event", map[string]any{"field": "questionnaire_enabled"})
		}
		questionnaire, err := s.q.LoadQuestionnaire(ctx, published.ID, actor.TenantID)
		if err != nil {
			return uuid.Nil, nil, nil, err
		}
		versionID := published.ID
		created, createErr := s.rfx.CreateResponse(ctx, domain.CreateRfxResponseInput{
			TenantID:             actor.TenantID,
			RfxEventID:           eventID,
			ParticipantCompanyID: carrierCompanyID,
			RfxVersionID:         &versionID,
		})
		if createErr != nil {
			var conflict *apperrors.AppError
			if errors.As(createErr, &conflict) && conflict.Code == apperrors.CodeConflict {
				response, err = s.rfx.GetResponseByEventAndCompany(ctx, eventID, carrierCompanyID, actor.TenantID)
				if err != nil {
					return uuid.Nil, nil, nil, err
				}
			} else {
				return uuid.Nil, nil, nil, createErr
			}
		} else {
			response = created
			return carrierCompanyID, response, questionnaire, nil
		}
	}

	if response.RfxVersionID == nil {
		published, err := s.q.GetPublishedVersionForEvent(ctx, eventID, actor.TenantID)
		if err != nil {
			return uuid.Nil, nil, nil, err
		}
		if !published.QuestionnaireEnabled {
			return uuid.Nil, nil, nil, apperrors.Validation("questionnaire is not enabled for this RFx event", map[string]any{"field": "questionnaire_enabled"})
		}
		response, err = s.rfx.PinResponseVersion(ctx, response.ID, actor.TenantID, published.ID)
		if err != nil {
			return uuid.Nil, nil, nil, err
		}
	}

	if response.RfxVersionID == nil {
		return uuid.Nil, nil, nil, apperrors.Validation("carrier response is not bound to a published questionnaire version", map[string]any{"field": "rfx_version_id"})
	}

	questionnaire, err := s.q.LoadQuestionnaire(ctx, *response.RfxVersionID, actor.TenantID)
	if err != nil {
		return uuid.Nil, nil, nil, err
	}
	if !questionnaire.QuestionnaireEnabled {
		return uuid.Nil, nil, nil, apperrors.Validation("questionnaire is not enabled for this RFx event", map[string]any{"field": "questionnaire_enabled"})
	}
	return carrierCompanyID, response, questionnaire, nil
}

func (s *CarrierResponseService) validateWorkspace(workspace *domain.CarrierResponseWorkspace, preSubmit bool) (*domain.ResponseValidationResult, error) {
	rt := domain.BuildQuestionnaireRuntime(&workspace.Questionnaire)
	byID := answersMap(workspace.Answers)
	errs := domain.ValidateCarrierAnswers(rt, byID, preSubmit)
	return &domain.ResponseValidationResult{
		Valid:              len(errs) == 0,
		BlockingErrorCount: len(errs),
		Errors:             errs,
		CompletionPercent:  domain.ComputeCompletionPercent(rt, byID),
	}, nil
}

func (s *CarrierResponseService) runTx(
	ctx context.Context,
	fn func(rfx *repository.RfxRepository, answers *repository.AnswerRepository, audit AuditRecorder, tx pgx.Tx) error,
) error {
	rfxRepo, ok := s.rfx.(*repository.RfxRepository)
	if !ok {
		return apperrors.Internal("carrier response store misconfigured", nil)
	}
	answerRepo, ok := s.answers.(*repository.AnswerRepository)
	if !ok {
		return apperrors.Internal("carrier answer store misconfigured", nil)
	}
	if s.tx == nil {
		return fn(rfxRepo, answerRepo, s.audit, nil)
	}
	return s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(rfxRepo.WithTx(tx), answerRepo.WithTx(tx), s.audit, tx)
	})
}

func (s *CarrierResponseService) resolveLatePermission(
	ctx context.Context,
	event *domain.RfxEvent,
	carrierCompanyID uuid.UUID,
	now time.Time,
) (*domain.LateSubmissionRequest, error) {
	if s.late == nil || !domain.ResponseDeadlinePassed(event.ResponseDeadline, now) {
		return nil, nil
	}
	return s.late.ResolveActivePermission(ctx, event.TenantID, event.ID, carrierCompanyID, event.ResponseDeadline, now)
}

func (s *CarrierResponseService) loadSubmitReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey string,
	requestBodyHash string,
) (*domain.SubmitResult, error) {
	record, err := s.idem.Get(ctx, scope, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	if record.RequestBodyHash != requestBodyHash {
		return nil, apperrors.Conflict("idempotency key was already used with a different request body", map[string]any{"field": "Idempotency-Key"})
	}
	var result domain.SubmitResult
	if err := json.Unmarshal(record.ResponseBody, &result); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent submit replay payload", err)
	}
	return &result, nil
}

func answersMap(answers []domain.CarrierAnswer) map[uuid.UUID]json.RawMessage {
	out := make(map[uuid.UUID]json.RawMessage, len(answers))
	for _, a := range answers {
		out[a.QuestionID] = a.AnswerValueJSON
	}
	return out
}

func validationFailed(details []domain.ValidationErrorDetail) *apperrors.AppError {
	items := make([]apperrors.ValidationErrorItem, 0, len(details))
	for _, d := range details {
		item := apperrors.ValidationErrorItem{
			Field:      d.Field,
			Rule:       d.Rule,
			MessageKey: d.MessageKey,
			Params:     d.Params,
		}
		if d.SectionID != uuid.Nil {
			item.SectionID = d.SectionID.String()
		}
		if d.QuestionID != uuid.Nil {
			item.QuestionID = d.QuestionID.String()
		}
		items = append(items, item)
	}
	return apperrors.ValidationFailed(items)
}

func saveVersionConflict(response domain.RfxResponse) *apperrors.AppError {
	details := map[string]any{
		"field":                "save_version",
		"current_save_version": response.SaveVersion,
	}
	if response.LastSavedAt != nil {
		details["last_saved_at"] = response.LastSavedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return apperrors.Conflict("save version conflict", details)
}

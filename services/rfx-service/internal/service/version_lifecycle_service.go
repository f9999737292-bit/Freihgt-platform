package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

type VersionLifecycleService struct {
	rfxRepo   *repository.RfxRepository
	qRepo     *repository.QuestionnaireRepository
	scoreRepo *repository.ScoreRepository
	idemRepo  *repository.IdempotencyRepository
	auditRepo *repository.AuditRepository
	tx        *repository.TransactionRunner
	auth      *RfxService
}

func NewVersionLifecycleService(
	pool *pgxpool.Pool,
	rfxRepo *repository.RfxRepository,
	qRepo *repository.QuestionnaireRepository,
	scoreRepo *repository.ScoreRepository,
	idemRepo *repository.IdempotencyRepository,
	auditRepo *repository.AuditRepository,
	auth *RfxService,
) *VersionLifecycleService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &VersionLifecycleService{
		rfxRepo:   rfxRepo,
		qRepo:     qRepo,
		scoreRepo: scoreRepo,
		idemRepo:  idemRepo,
		auditRepo: auditRepo,
		tx:        tx,
		auth:      auth,
	}
}

func (s *VersionLifecycleService) ListVersions(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) ([]domain.RfxVersion, error) {
	if _, err := s.authorizeEvent(ctx, actor, eventID); err != nil {
		return nil, err
	}
	versions, err := s.qRepo.ListVersionsForEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.applyVersionActivityFlags(ctx, eventID, actor.TenantID, versions); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s *VersionLifecycleService) GetVersion(ctx context.Context, actor domain.ActorContext, eventID, versionID uuid.UUID) (*domain.VersionDetail, error) {
	if _, err := s.authorizeEvent(ctx, actor, eventID); err != nil {
		return nil, err
	}
	version, err := s.qRepo.GetVersionForEvent(ctx, eventID, versionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.applyVersionFlags(ctx, eventID, actor.TenantID, version); err != nil {
		return nil, err
	}
	questionnaire, err := s.qRepo.LoadQuestionnaire(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	return &domain.VersionDetail{
		Version:       *version,
		Questionnaire: *questionnaire,
	}, nil
}

func (s *VersionLifecycleService) PublishQuestionnaire(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	idempotencyKey string,
	in domain.PublishQuestionnaireInput,
) (*domain.RfxVersion, error) {
	event, err := s.authorizeEvent(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePublishQuestionnaireInput(in); err != nil {
		return nil, err
	}

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	if len(idempotencyKey) > 128 {
		return nil, apperrors.Validation("Idempotency-Key header is too long", map[string]any{"field": "Idempotency-Key"})
	}

	scope := repository.IdempotencyScope{
		TenantID:       actor.TenantID,
		ActorID:        actor.UserID,
		Operation:      domain.VersionLifecycleOperationPublish,
		AggregateScope: eventID,
	}
	requestBodyHash, err := hashRequestBody(in)
	if err != nil {
		return nil, err
	}

	if replay, err := s.loadReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("version lifecycle service misconfigured", nil)
	}

	var published *domain.RfxVersion
	runErr := s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qRepo := s.qRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		auditRepo := s.auditRepo.WithTx(tx)

		state, err := qRepo.LockEventVersionState(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		if state.DraftVersionID == nil {
			return apperrors.Conflict("draft questionnaire version not found", map[string]any{"field": "draft_version_id"})
		}
		if state.EventVersion != in.ExpectedEventVersion {
			return apperrors.Conflict("rfx event was updated by another request", map[string]any{"field": "expected_event_version"})
		}

		draft, err := qRepo.LockVersionByID(ctx, *state.DraftVersionID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateVersionPublishStatus(draft.Status); err != nil {
			return err
		}
		if draft.Version != in.ExpectedDraftVersion {
			return apperrors.Conflict("questionnaire version was updated by another request", map[string]any{"field": "expected_draft_version"})
		}

		definition, err := qRepo.LoadQuestionnaire(ctx, draft.ID, actor.TenantID)
		if err != nil {
			return err
		}
		readiness := domain.EvaluatePublishReadiness(*draft, definition.Sections, definition.Rules)
		if !readiness.Ready {
			return publishReadinessFailed(readiness)
		}

		if state.PublishedVersionID != nil {
			responseCount, scoredResponseCount, err := qRepo.CountResponsesAndScoresForEvent(ctx, eventID, actor.TenantID)
			if err != nil {
				return err
			}
			if responseCount > 0 || scoredResponseCount > 0 {
				return domain.ChangeImpactAnalysisRequired(responseCount, scoredResponseCount)
			}
		}

		out, err := qRepo.PublishVersionTx(ctx, eventID, actor.TenantID, in.ExpectedDraftVersion, in.ChangeSummary, actor.UserID)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, auditRepo, actor, event.OwnerCompanyID, "rfx_questionnaire", out.Published.ID, "rfx.version.published.v1", map[string]any{
			"rfx_event_id":   eventID.String(),
			"version_number": out.Published.VersionNumber,
			"change_summary": strings.TrimSpace(in.ChangeSummary),
		}); err != nil {
			return err
		}
		if out.Superseded != nil {
			if err := recordAudit(ctx, auditRepo, actor, event.OwnerCompanyID, "rfx_questionnaire", out.Superseded.ID, "rfx.version.superseded.v1", map[string]any{
				"rfx_event_id":                 eventID.String(),
				"superseded_by_version_id":     out.Published.ID.String(),
				"superseded_by_version_number": out.Published.VersionNumber,
			}); err != nil {
				return err
			}
		}

		out.Published.IsCurrentPublished = true
		out.Published.IsActiveDraft = false
		payload, err := json.Marshal(out.Published)
		if err != nil {
			return apperrors.Internal("failed to persist publish replay payload", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        actor.TenantID,
			ActorID:         actor.UserID,
			Operation:       domain.VersionLifecycleOperationPublish,
			AggregateScope:  eventID,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  http.StatusOK,
			ResponseBody:    payload,
			ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		published = out.Published
		return nil
	})
	if runErr != nil {
		if replay, err := s.loadReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
			return replay, err
		}
		return nil, runErr
	}
	return published, nil
}

func (s *VersionLifecycleService) ForkDraftFromPublished(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, idempotencyKey string) (*domain.RfxVersion, error) {
	event, err := s.authorizeEvent(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	if len(idempotencyKey) > 128 {
		return nil, apperrors.Validation("Idempotency-Key header is too long", map[string]any{"field": "Idempotency-Key"})
	}
	scope := repository.IdempotencyScope{
		TenantID:       actor.TenantID,
		ActorID:        actor.UserID,
		Operation:      domain.VersionLifecycleOperationForkDraft,
		AggregateScope: eventID,
	}
	requestBodyHash, err := hashRequestBody(struct {
		Operation string `json:"operation"`
	}{
		Operation: domain.VersionLifecycleOperationForkDraft,
	})
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("version lifecycle service misconfigured", nil)
	}

	var draft *domain.RfxVersion
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qRepo := s.qRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		auditRepo := s.auditRepo.WithTx(tx)
		state, err := qRepo.LockEventVersionState(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		if state.PublishedVersionID == nil {
			return apperrors.Conflict("published questionnaire version not found", map[string]any{"field": "published_version_id"})
		}
		out, err := qRepo.ForkDraftFromPublishedTx(ctx, eventID, *state.PublishedVersionID, actor.TenantID)
		if err != nil {
			return err
		}
		out.IsActiveDraft = true
		out.IsCurrentPublished = false
		payload, err := json.Marshal(out)
		if err != nil {
			return apperrors.Internal("failed to persist fork replay payload", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        actor.TenantID,
			ActorID:         actor.UserID,
			Operation:       domain.VersionLifecycleOperationForkDraft,
			AggregateScope:  eventID,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  http.StatusCreated,
			ResponseBody:    payload,
			ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		if err := recordAudit(ctx, auditRepo, actor, event.OwnerCompanyID, "rfx_questionnaire", out.ID, "rfx.version.forked.v1", map[string]any{
			"rfx_event_id":                eventID.String(),
			"source_published_version_id": state.PublishedVersionID.String(),
			"version_number":              out.VersionNumber,
		}); err != nil {
			return err
		}
		draft = out
		return nil
	})
	if err != nil {
		if replay, replayErr := s.loadReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
			return replay, replayErr
		}
		return nil, err
	}
	return draft, nil
}

func (s *VersionLifecycleService) CompareVersions(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	in domain.CompareVersionsInput,
) (*domain.CompareVersionsResult, error) {
	if _, err := s.authorizeEvent(ctx, actor, eventID); err != nil {
		return nil, err
	}
	if err := domain.ValidateCompareVersionsInput(in); err != nil {
		return nil, err
	}

	sourceVersion, err := s.qRepo.GetVersionForEvent(ctx, eventID, in.SourceVersionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	targetVersion, err := s.qRepo.GetVersionForEvent(ctx, eventID, in.TargetVersionID, actor.TenantID)
	if err != nil {
		return nil, err
	}

	sourceQuestionnaire, err := s.qRepo.LoadQuestionnaire(ctx, sourceVersion.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	targetQuestionnaire, err := s.qRepo.LoadQuestionnaire(ctx, targetVersion.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}

	sourceSnapshot := domain.VersionCompareSnapshot{
		Version:       *sourceVersion,
		Questionnaire: *sourceQuestionnaire,
	}
	targetSnapshot := domain.VersionCompareSnapshot{
		Version:       *targetVersion,
		Questionnaire: *targetQuestionnaire,
	}
	if s.scoreRepo != nil {
		sourceScoring, err := s.loadScoringSnapshot(ctx, actor.TenantID, sourceVersion.ID, *sourceQuestionnaire)
		if err != nil {
			return nil, err
		}
		targetScoring, err := s.loadScoringSnapshot(ctx, actor.TenantID, targetVersion.ID, *targetQuestionnaire)
		if err != nil {
			return nil, err
		}
		sourceSnapshot.Scoring = sourceScoring
		targetSnapshot.Scoring = targetScoring
	}

	result := domain.CompareVersionSnapshots(sourceSnapshot, targetSnapshot)
	return &result, nil
}

func (s *VersionLifecycleService) RestoreVersionAsDraft(
	ctx context.Context,
	actor domain.ActorContext,
	eventID, sourceVersionID uuid.UUID,
	idempotencyKey string,
	in domain.RestoreVersionAsDraftInput,
) (*domain.RfxVersion, error) {
	event, err := s.authorizeEvent(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRestoreVersionAsDraftInput(in); err != nil {
		return nil, err
	}
	if _, err := s.qRepo.GetVersionForEvent(ctx, eventID, sourceVersionID, actor.TenantID); err != nil {
		return nil, err
	}

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	if len(idempotencyKey) > 128 {
		return nil, apperrors.Validation("Idempotency-Key header is too long", map[string]any{"field": "Idempotency-Key"})
	}

	scope := repository.IdempotencyScope{
		TenantID:       actor.TenantID,
		ActorID:        actor.UserID,
		Operation:      domain.VersionLifecycleOperationRestoreDraft,
		AggregateScope: eventID,
	}
	requestBodyHash, err := hashRequestBody(in)
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("version lifecycle service misconfigured", nil)
	}

	var draft *domain.RfxVersion
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qRepo := s.qRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		auditRepo := s.auditRepo.WithTx(tx)

		out, err := qRepo.RestoreVersionAsDraftTx(ctx, eventID, sourceVersionID, actor.TenantID, in.ChangeSummary)
		if err != nil {
			return err
		}
		out.IsActiveDraft = true
		out.IsCurrentPublished = false
		payload, err := json.Marshal(out)
		if err != nil {
			return apperrors.Internal("failed to persist restore replay payload", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        actor.TenantID,
			ActorID:         actor.UserID,
			Operation:       domain.VersionLifecycleOperationRestoreDraft,
			AggregateScope:  eventID,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  http.StatusCreated,
			ResponseBody:    payload,
			ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		if err := recordAudit(ctx, auditRepo, actor, event.OwnerCompanyID, "rfx_questionnaire", out.ID, "rfx.version.restored_as_draft.v1", map[string]any{
			"rfx_event_id":       eventID.String(),
			"source_version_id":  sourceVersionID.String(),
			"version_number":     out.VersionNumber,
			"change_summary":     strings.TrimSpace(in.ChangeSummary),
		}); err != nil {
			return err
		}
		draft = out
		return nil
	})
	if err != nil {
		if replay, replayErr := s.loadReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
			return replay, replayErr
		}
		return nil, err
	}
	return draft, nil
}

func (s *VersionLifecycleService) loadScoringSnapshot(
	ctx context.Context,
	tenantID, versionID uuid.UUID,
	questionnaire domain.QuestionnaireDefinition,
) (*domain.ScoringCompareSnapshot, error) {
	model, err := s.scoreRepo.GetPublishedModelForVersion(ctx, tenantID, versionID)
	if err != nil {
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			return nil, err
		}
		model, err = s.scoreRepo.GetDraftModelForVersion(ctx, tenantID, versionID)
		if err != nil {
			if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
				return nil, nil
			}
			return nil, err
		}
	}
	criteria, err := s.scoreRepo.ListCriteriaByModel(ctx, model.ID, tenantID)
	if err != nil {
		return nil, err
	}
	bindings, err := s.scoreRepo.ListBindingsByModel(ctx, model.ID, tenantID)
	if err != nil {
		return nil, err
	}
	questionCodes := make(map[uuid.UUID]string)
	for _, section := range questionnaire.Sections {
		for _, question := range section.Questions {
			questionCodes[question.ID] = question.QuestionCode
		}
	}
	criterionCodes := make(map[uuid.UUID]string, len(criteria))
	for _, criterion := range criteria {
		criterionCodes[criterion.ID] = criterion.CriterionCode
	}
	bindingEntries := make([]domain.ScoreBindingCompareEntry, 0, len(bindings))
	for _, binding := range bindings {
		criterionCode := criterionCodes[binding.CriterionID]
		questionCode := questionCodes[binding.QuestionID]
		bindingEntries = append(bindingEntries, domain.ScoreBindingCompareEntry{
			CriterionCode:    criterionCode,
			QuestionCode:     questionCode,
			BindingType:      binding.BindingType,
			ScoringRuleJSON:  binding.ScoringRuleJSON,
			KnockoutRuleJSON: binding.KnockoutRuleJSON,
		})
	}
	return &domain.ScoringCompareSnapshot{
		ModelVersion: model.ModelVersion,
		Status:       model.Status,
		Criteria:     criteria,
		Bindings:     bindingEntries,
	}, nil
}

func (s *VersionLifecycleService) authorizeEvent(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, error) {
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
		return nil, apperrors.Forbidden("buyer company membership is required for this rfx event")
	}
	return event, nil
}

func (s *VersionLifecycleService) loadReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey string,
	requestBodyHash string,
) (*domain.RfxVersion, error) {
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
	var version domain.RfxVersion
	if err := json.Unmarshal(record.ResponseBody, &version); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent publish replay payload", err)
	}
	return &version, nil
}

func hashRequestBody(v any) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", apperrors.Internal("failed to hash request body", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (s *VersionLifecycleService) applyVersionActivityFlags(ctx context.Context, eventID, tenantID uuid.UUID, versions []domain.RfxVersion) error {
	state, err := s.qRepo.GetEventVersionState(ctx, eventID, tenantID)
	if err != nil {
		return err
	}
	for i := range versions {
		versions[i].IsCurrentPublished = state.PublishedVersionID != nil && versions[i].ID == *state.PublishedVersionID
		versions[i].IsActiveDraft = state.DraftVersionID != nil && versions[i].ID == *state.DraftVersionID
	}
	return nil
}

func (s *VersionLifecycleService) applyVersionFlags(ctx context.Context, eventID, tenantID uuid.UUID, version *domain.RfxVersion) error {
	state, err := s.qRepo.GetEventVersionState(ctx, eventID, tenantID)
	if err != nil {
		return err
	}
	version.IsCurrentPublished = state.PublishedVersionID != nil && version.ID == *state.PublishedVersionID
	version.IsActiveDraft = state.DraftVersionID != nil && version.ID == *state.DraftVersionID
	return nil
}

func publishReadinessFailed(readiness domain.PublishReadinessResult) error {
	items := make([]apperrors.ValidationErrorItem, 0, readiness.BlockingFail)
	for _, item := range readiness.Items {
		if item.Status != domain.PublishCheckFail {
			continue
		}
		items = append(items, apperrors.ValidationErrorItem{
			Field:      "questionnaire",
			Rule:       item.Code,
			MessageKey: item.Message,
			Params:     item.Details,
		})
	}
	if len(items) == 0 {
		items = append(items, apperrors.ValidationErrorItem{
			Field:      "questionnaire",
			Rule:       "PUBLISH_READINESS_FAILED",
			MessageKey: "publish readiness failed",
		})
	}
	return apperrors.ValidationFailed(items)
}

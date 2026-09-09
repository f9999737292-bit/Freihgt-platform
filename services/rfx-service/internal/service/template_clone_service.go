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

type TemplateCloneService struct {
	rfxRepo   *repository.RfxRepository
	qRepo     *repository.QuestionnaireRepository
	tmplRepo  *repository.TemplateLibraryRepository
	tmplQRepo *repository.TemplateQuestionnaireRepository
	idemRepo  *repository.IdempotencyRepository
	auditRepo *repository.AuditRepository
	rfxSvc    *RfxService
	tmplAuth  *TemplateLibraryService
	tx        *repository.TransactionRunner
}

func NewTemplateCloneService(
	pool *pgxpool.Pool,
	rfxRepo *repository.RfxRepository,
	qRepo *repository.QuestionnaireRepository,
	tmplRepo *repository.TemplateLibraryRepository,
	tmplQRepo *repository.TemplateQuestionnaireRepository,
	idemRepo *repository.IdempotencyRepository,
	auditRepo *repository.AuditRepository,
	rfxSvc *RfxService,
	tmplAuth *TemplateLibraryService,
) *TemplateCloneService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &TemplateCloneService{
		rfxRepo: rfxRepo, qRepo: qRepo, tmplRepo: tmplRepo, tmplQRepo: tmplQRepo,
		idemRepo: idemRepo, auditRepo: auditRepo, rfxSvc: rfxSvc, tmplAuth: tmplAuth, tx: tx,
	}
}

func (s *TemplateCloneService) CloneEventFromTemplate(
	ctx context.Context,
	actor domain.ActorContext,
	templateVersionID uuid.UUID,
	eventIn domain.CreateRfxEventInput,
	idempotencyKey string,
) (*domain.CloneEventFromTemplateResult, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if err := s.tmplAuth.requireBuyerManage(ctx, actor); err != nil {
		return nil, err
	}
	eventIn.TenantID = actor.TenantID
	ownerCompanyID, err := s.rfxSvc.resolveCreateOwnerCompanyID(ctx, actor, eventIn.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	eventIn.OwnerCompanyID = ownerCompanyID
	if err := domain.ValidateCloneEventFromTemplate(templateVersionID, eventIn); err != nil {
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
		Operation:      domain.TemplateCloneOperationCloneEventFromTemplate,
		AggregateScope: templateVersionID,
	}
	payload := domain.NewCloneEventFromTemplateIdempotencyPayload(templateVersionID, eventIn)
	requestBodyHash, err := hashRequestBody(payload)
	if err != nil {
		return nil, err
	}
	if replay, replayErr := s.loadCloneReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
		return replay, replayErr
	}
	if s.tx == nil {
		return nil, apperrors.Internal("template clone service misconfigured", nil)
	}

	var result *domain.CloneEventFromTemplateResult
	runErr := s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rfxRepo := s.rfxRepo.WithTx(tx)
		qRepo := s.qRepo.WithTx(tx)
		tRepo := s.tmplRepo.WithTx(tx)
		tqRepo := s.tmplQRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)

		sourceVersion, lookupErr := tRepo.GetVersionByID(ctx, templateVersionID, actor.TenantID)
		if lookupErr != nil {
			return lookupErr
		}

		tmpl, lockErr := tRepo.LockTemplateByID(ctx, sourceVersion.TemplateID, actor.TenantID)
		if lockErr != nil {
			return lockErr
		}
		if err := s.tmplAuth.requireTemplateCompanyAccess(ctx, actor, tmpl, true); err != nil {
			return err
		}
		if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
			return err
		}

		lockedSource, versionErr := tRepo.LockVersionByID(ctx, templateVersionID, actor.TenantID)
		if versionErr != nil {
			return versionErr
		}
		if lockedSource.TemplateID != tmpl.ID {
			return apperrors.NotFound("rfx template version not found")
		}
		if err := domain.EnsureTemplateVersionEventCloneSource(lockedSource.Status); err != nil {
			return err
		}

		exists, companyErr := rfxRepo.CompanyExists(ctx, eventIn.OwnerCompanyID, actor.TenantID)
		if companyErr != nil {
			return companyErr
		}
		if !exists {
			return apperrors.NotFound("owner_company_id not found")
		}

		event, createErr := rfxRepo.CreateEventWithProvenance(ctx, eventIn, templateVersionID)
		if createErr != nil {
			return createErr
		}
		draft, draftErr := qRepo.CreateInitialDraftVersion(ctx, actor.TenantID, event.ID)
		if draftErr != nil {
			return draftErr
		}
		if _, copyErr := repository.CopyTemplateGraphToEventVersion(ctx, tqRepo, qRepo, actor.TenantID, tmpl.ID, lockedSource.ID, draft.ID); copyErr != nil {
			return copyErr
		}

		warning := lockedSource.Status == domain.RfxVersionStatusSuperseded
		out := &domain.CloneEventFromTemplateResult{
			Event:                   *event,
			DraftVersion:            *draft,
			SourceTemplateID:        tmpl.ID,
			SourceTemplateVersionID: lockedSource.ID,
			SourceVersionNumber:     lockedSource.VersionNumber,
			SourceVersionStatus:     lockedSource.Status,
			SourceVersionWarning:    warning,
		}

		responseBody, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return apperrors.Internal("failed to persist clone replay payload", marshalErr)
		}
		if storeErr := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        actor.TenantID,
			ActorID:         actor.UserID,
			Operation:       domain.TemplateCloneOperationCloneEventFromTemplate,
			AggregateScope:  templateVersionID,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  http.StatusCreated,
			ResponseBody:    responseBody,
			ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		}); storeErr != nil {
			return storeErr
		}

		auditPayload := map[string]any{
			"event_id":                   event.ID.String(),
			"source_template_id":         tmpl.ID.String(),
			"source_template_version_id": lockedSource.ID.String(),
			"source_version_number":      lockedSource.VersionNumber,
			"source_version_status":      lockedSource.Status,
			"actor_id":                   actor.UserID.String(),
		}
		if tmpl.OwnerCompanyID != nil {
			auditPayload["owner_company_id"] = tmpl.OwnerCompanyID.String()
		}
		if auditErr := recordAudit(ctx, aRepo, actor, eventIn.OwnerCompanyID, "rfx_event", event.ID, "rfx.event.created_from_template.v1", auditPayload); auditErr != nil {
			return auditErr
		}

		result = out
		return nil
	})
	if runErr != nil {
		if replay, replayErr := s.loadCloneReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
			return replay, replayErr
		}
		return nil, runErr
	}
	return result, nil
}

func (s *TemplateCloneService) loadCloneReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
) (*domain.CloneEventFromTemplateResult, error) {
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
	var out domain.CloneEventFromTemplateResult
	if err := json.Unmarshal(record.ResponseBody, &out); err != nil {
		return nil, apperrors.Internal("failed to decode clone replay payload", err)
	}
	return &out, nil
}

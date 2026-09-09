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

type TemplateLibraryService struct {
	tmplRepo  *repository.TemplateLibraryRepository
	qRepo     *repository.TemplateQuestionnaireRepository
	idemRepo  *repository.IdempotencyRepository
	auditRepo *repository.AuditRepository
	auth      *RfxService
	tx        *repository.TransactionRunner
}

func NewTemplateLibraryService(
	pool *pgxpool.Pool,
	tmplRepo *repository.TemplateLibraryRepository,
	qRepo *repository.TemplateQuestionnaireRepository,
	idemRepo *repository.IdempotencyRepository,
	auditRepo *repository.AuditRepository,
	auth *RfxService,
) *TemplateLibraryService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &TemplateLibraryService{tmplRepo: tmplRepo, qRepo: qRepo, idemRepo: idemRepo, auditRepo: auditRepo, auth: auth, tx: tx}
}

func (s *TemplateLibraryService) CreateTemplate(ctx context.Context, actor domain.ActorContext, in domain.CreateTemplateInput) (*domain.TemplateDetail, error) {
	if err := s.requireBuyerManage(ctx, actor); err != nil {
		return nil, err
	}
	if err := domain.ValidateCreateTemplateInput(in); err != nil {
		return nil, err
	}
	nameI18n, err := domain.ValidateTemplateI18nMap(in.NameI18n, "name_i18n", true)
	if err != nil {
		return nil, err
	}
	var descriptionI18n []byte
	if len(in.DescriptionI18n) > 0 {
		descriptionI18n, err = domain.ValidateTemplateI18nMap(in.DescriptionI18n, "description_i18n", false)
		if err != nil {
			return nil, err
		}
	}
	ownerCompanyID, err := s.resolveOwnerCompany(ctx, actor, in.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("template library service misconfigured", nil)
	}
	var tmpl *domain.RfxTemplate
	var draft *domain.RfxTemplateVersion
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tRepo := s.tmplRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)
		var createErr error
		tmpl, draft, createErr = tRepo.CreateTemplateWithDraft(ctx, actor.TenantID, in, ownerCompanyID, actor.UserID, nameI18n, descriptionI18n)
		if createErr != nil {
			return createErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(ownerCompanyID), "rfx_template", tmpl.ID, "rfx.template.created.v1", map[string]any{
			"template_code":    tmpl.TemplateCode,
			"draft_version_id": draft.ID.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return s.buildDetail(ctx, tmpl, draft, nil, []domain.RfxTemplateVersion{*draft})
}

func (s *TemplateLibraryService) ListTemplates(ctx context.Context, actor domain.ActorContext, filter domain.TemplateListFilter) ([]domain.RfxTemplate, int, error) {
	if err := s.applyTemplateListScope(ctx, actor, &filter); err != nil {
		return nil, 0, err
	}
	return s.tmplRepo.ListTemplates(ctx, actor.TenantID, filter)
}

func (s *TemplateLibraryService) GetTemplate(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.TemplateDetail, error) {
	tmpl, err := s.authorizeTemplateRead(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	return s.loadDetail(ctx, tmpl)
}

func (s *TemplateLibraryService) UpdateTemplate(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, in domain.UpdateTemplateInput) (*domain.RfxTemplate, error) {
	tmpl, err := s.authorizeTemplateManage(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateUpdateTemplateInput(in); err != nil {
		return nil, err
	}
	var nameI18n, descriptionI18n []byte
	var validateErr error
	if len(in.NameI18n) > 0 {
		nameI18n, validateErr = domain.ValidateTemplateI18nMap(in.NameI18n, "name_i18n", true)
		if validateErr != nil {
			return nil, validateErr
		}
	}
	if len(in.DescriptionI18n) > 0 {
		descriptionI18n, validateErr = domain.ValidateTemplateI18nMap(in.DescriptionI18n, "description_i18n", false)
		if validateErr != nil {
			return nil, validateErr
		}
	}
	if s.tx == nil {
		return nil, apperrors.Internal("template library service misconfigured", nil)
	}
	var updated *domain.RfxTemplate
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tRepo := s.tmplRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)
		if _, lockErr := tRepo.LockTemplateByID(ctx, templateID, actor.TenantID); lockErr != nil {
			return lockErr
		}
		var updateErr error
		updated, updateErr = tRepo.UpdateTemplate(ctx, templateID, actor.TenantID, in, nameI18n, descriptionI18n)
		if updateErr != nil {
			return updateErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template", updated.ID, "rfx.template.updated.v1", nil)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TemplateLibraryService) DeleteTemplate(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) error {
	tmpl, err := s.authorizeTemplateManage(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
		return err
	}
	if s.tx == nil {
		return apperrors.Internal("template library service misconfigured", nil)
	}
	return s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tRepo := s.tmplRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)
		if _, lockErr := tRepo.LockTemplateByID(ctx, templateID, actor.TenantID); lockErr != nil {
			return lockErr
		}
		everPublished, pubErr := tRepo.HasEverPublished(ctx, templateID, actor.TenantID)
		if pubErr != nil {
			return pubErr
		}
		if everPublished {
			return apperrors.Conflict("template with published versions cannot be deleted", map[string]any{"field": "template_id"})
		}
		if delErr := tRepo.SoftDeleteTemplate(ctx, templateID, actor.TenantID); delErr != nil {
			return delErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template", templateID, "rfx.template.deleted.v1", nil)
	})
}

func (s *TemplateLibraryService) ArchiveTemplate(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.RfxTemplate, error) {
	tmpl, err := s.authorizeTemplateManage(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if tmpl.Status == domain.RfxTemplateStatusArchived {
		return nil, apperrors.Conflict("template is already archived", map[string]any{"field": "status"})
	}
	if s.tx == nil {
		return nil, apperrors.Internal("template library service misconfigured", nil)
	}
	var archived *domain.RfxTemplate
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tRepo := s.tmplRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)
		if _, lockErr := tRepo.LockTemplateByID(ctx, templateID, actor.TenantID); lockErr != nil {
			return lockErr
		}
		var archiveErr error
		archived, archiveErr = tRepo.ArchiveTemplate(ctx, templateID, actor.TenantID)
		if archiveErr != nil {
			return archiveErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template", archived.ID, "rfx.template.archived.v1", nil)
	})
	if err != nil {
		return nil, err
	}
	return archived, nil
}

func (s *TemplateLibraryService) PublishTemplateVersion(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, idempotencyKey string, in domain.PublishTemplateVersionInput) (*domain.RfxTemplateVersion, error) {
	if _, err := s.authorizeTemplateManage(ctx, actor, templateID); err != nil {
		return nil, err
	}
	if err := domain.ValidatePublishTemplateVersionInput(in); err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	scope := repository.IdempotencyScope{
		TenantID: actor.TenantID, ActorID: actor.UserID,
		Operation: domain.TemplateLifecycleOperationPublish, AggregateScope: templateID,
	}
	requestBodyHash, err := hashRequestBody(in)
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadTemplateVersionReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("template library service misconfigured", nil)
	}

	var published *domain.RfxTemplateVersion
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tRepo := s.tmplRepo.WithTx(tx)
		qRepo := s.qRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)

		lockedTmpl, draft, lockErr := s.LockTemplateAndDraftForPublish(ctx, tRepo, actor, templateID, in.ExpectedTemplateVersion, in.ExpectedDraftVersion)
		if lockErr != nil {
			return lockErr
		}

		definition, loadErr := qRepo.LoadQuestionnaire(ctx, templateID, draft.ID, actor.TenantID)
		if loadErr != nil {
			return loadErr
		}
		readiness := domain.EvaluatePublishReadiness(domain.RfxVersion{QuestionnaireEnabled: true}, domain.ToQuestionnaireSections(definition.Sections), domain.ToQuestionnaireRules(definition.Rules))
		if !readiness.Ready {
			return domain.TemplatePublishReadinessFailure(readiness)
		}

		out, pubErr := tRepo.PublishLockedDraftVersion(ctx, templateID, actor.TenantID, draft, in.ChangeSummary, actor.UserID)
		if pubErr != nil {
			return pubErr
		}
		payload, marshalErr := json.Marshal(out.Published)
		if marshalErr != nil {
			return apperrors.Internal("failed to persist publish replay payload", marshalErr)
		}
		if storeErr := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID: actor.TenantID, ActorID: actor.UserID, Operation: domain.TemplateLifecycleOperationPublish,
			AggregateScope: templateID, IdempotencyKey: idempotencyKey, RequestBodyHash: requestBodyHash,
			ResponseStatus: http.StatusOK, ResponseBody: payload, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}); storeErr != nil {
			return storeErr
		}
		if auditErr := recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(lockedTmpl.OwnerCompanyID), "rfx_template_version", out.Published.ID, "rfx.template.version.published.v1", map[string]any{
			"template_id": templateID.String(), "version_number": out.Published.VersionNumber,
		}); auditErr != nil {
			return auditErr
		}
		published = out.Published
		return nil
	})
	if err != nil {
		if replay, replayErr := s.loadTemplateVersionReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
			return replay, replayErr
		}
		return nil, err
	}
	return published, nil
}

func (s *TemplateLibraryService) ForkDraftFromPublished(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, idempotencyKey string) (*domain.RfxTemplateVersion, error) {
	tmpl, err := s.authorizeTemplateManage(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	scope := repository.IdempotencyScope{
		TenantID: actor.TenantID, ActorID: actor.UserID,
		Operation: domain.TemplateLifecycleOperationForkDraft, AggregateScope: templateID,
	}
	requestBodyHash, err := hashRequestBody(struct {
		Operation string `json:"operation"`
	}{Operation: domain.TemplateLifecycleOperationForkDraft})
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadTemplateVersionReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}
	if s.tx == nil {
		return nil, apperrors.Internal("template library service misconfigured", nil)
	}

	var draft *domain.RfxTemplateVersion
	err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tRepo := s.tmplRepo.WithTx(tx)
		qRepo := s.qRepo.WithTx(tx)
		idemRepo := s.idemRepo.WithTx(tx)
		aRepo := s.auditRepo.WithTx(tx)
		out, forkErr := tRepo.ForkDraftFromPublishedTx(ctx, templateID, actor.TenantID, actor.UserID, qRepo)
		if forkErr != nil {
			return forkErr
		}
		payload, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return apperrors.Internal("failed to persist fork replay payload", marshalErr)
		}
		if storeErr := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID: actor.TenantID, ActorID: actor.UserID, Operation: domain.TemplateLifecycleOperationForkDraft,
			AggregateScope: templateID, IdempotencyKey: idempotencyKey, RequestBodyHash: requestBodyHash,
			ResponseStatus: http.StatusCreated, ResponseBody: payload, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}); storeErr != nil {
			return storeErr
		}
		if auditErr := recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_version", out.ID, "rfx.template.version.forked.v1", map[string]any{
			"template_id": templateID.String(), "version_number": out.VersionNumber,
		}); auditErr != nil {
			return auditErr
		}
		draft = out
		return nil
	})
	if err != nil {
		if replay, replayErr := s.loadTemplateVersionReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil || replay != nil {
			return replay, replayErr
		}
		return nil, err
	}
	return draft, nil
}

func (s *TemplateLibraryService) loadDetail(ctx context.Context, tmpl *domain.RfxTemplate) (*domain.TemplateDetail, error) {
	draft, err := s.tmplRepo.GetDraftVersion(ctx, tmpl.ID, tmpl.TenantID)
	if err != nil {
		return nil, err
	}
	published, err := s.tmplRepo.GetPublishedVersion(ctx, tmpl.ID, tmpl.TenantID)
	if err != nil {
		return nil, err
	}
	versions, err := s.tmplRepo.ListVersionsForTemplate(ctx, tmpl.ID, tmpl.TenantID)
	if err != nil {
		return nil, err
	}
	return s.buildDetail(ctx, tmpl, draft, published, versions)
}

func (s *TemplateLibraryService) buildDetail(_ context.Context, tmpl *domain.RfxTemplate, draft, published *domain.RfxTemplateVersion, versions []domain.RfxTemplateVersion) (*domain.TemplateDetail, error) {
	for i := range versions {
		if draft != nil && versions[i].ID == draft.ID {
			versions[i].IsActiveDraft = true
		}
		if published != nil && versions[i].ID == published.ID {
			versions[i].IsPublished = true
		}
	}
	return &domain.TemplateDetail{Template: *tmpl, DraftVersion: draft, PublishedVersion: published, Versions: versions}, nil
}

func (s *TemplateLibraryService) authorizeTemplateRead(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.RfxTemplate, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if err := s.denyCarrierOnly(ctx, actor); err != nil {
		return nil, err
	}
	tmpl, err := s.tmplRepo.GetTemplateByID(ctx, templateID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTemplateCompanyAccess(ctx, actor, tmpl, false); err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (s *TemplateLibraryService) authorizeTemplateManage(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.RfxTemplate, error) {
	tmpl, err := s.authorizeTemplateRead(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.requireBuyerManage(ctx, actor); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// LockTemplateAndDraftForPublish locks the template aggregate and current DRAFT version for publish,
// re-validating tenant authorization, ACTIVE status, and optimistic versions inside the transaction.
func (s *TemplateLibraryService) LockTemplateAndDraftForPublish(
	ctx context.Context,
	tRepo *repository.TemplateLibraryRepository,
	actor domain.ActorContext,
	templateID uuid.UUID,
	expectedTemplateVersion, expectedDraftVersion int,
) (*domain.RfxTemplate, *domain.RfxTemplateVersion, error) {
	if err := actor.Validate(); err != nil {
		return nil, nil, err
	}
	if err := s.denyCarrierOnly(ctx, actor); err != nil {
		return nil, nil, err
	}
	if err := s.requireBuyerManage(ctx, actor); err != nil {
		return nil, nil, err
	}
	tmpl, draft, err := tRepo.LockTemplateAndDraftForPublish(ctx, templateID, actor.TenantID, expectedTemplateVersion, expectedDraftVersion)
	if err != nil {
		return nil, nil, err
	}
	if err := s.requireTemplateCompanyAccess(ctx, actor, tmpl, true); err != nil {
		return nil, nil, err
	}
	return tmpl, draft, nil
}

// LockMutableDraftForGraphMutation locks the template aggregate and current DRAFT version
// inside an open transaction and re-validates tenant, authorization, and editability.
// Lock order matches PublishTemplateVersionTx: template row, then draft version row.
func (s *TemplateLibraryService) LockMutableDraftForGraphMutation(
	ctx context.Context,
	tRepo *repository.TemplateLibraryRepository,
	actor domain.ActorContext,
	templateID uuid.UUID,
) (*domain.RfxTemplate, *domain.RfxTemplateVersion, error) {
	if err := actor.Validate(); err != nil {
		return nil, nil, err
	}
	if err := s.denyCarrierOnly(ctx, actor); err != nil {
		return nil, nil, err
	}
	if err := s.requireBuyerManage(ctx, actor); err != nil {
		return nil, nil, err
	}
	tmpl, err := tRepo.LockTemplateByID(ctx, templateID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.requireTemplateCompanyAccess(ctx, actor, tmpl, true); err != nil {
		return nil, nil, err
	}
	if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
		return nil, nil, err
	}
	draft, err := tRepo.GetDraftVersion(ctx, templateID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if draft == nil {
		return nil, nil, apperrors.Conflict("draft template version not found", map[string]any{"field": "draft_version_id"})
	}
	draft, err = tRepo.LockVersionByID(ctx, draft.ID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if draft.TemplateID != templateID || draft.TenantID != actor.TenantID {
		return nil, nil, apperrors.Conflict("draft template version not found", map[string]any{"field": "draft_version_id"})
	}
	if err := domain.EnsureTemplateVersionDraft(draft.Status); err != nil {
		return nil, nil, err
	}
	draft.IsActiveDraft = true
	return tmpl, draft, nil
}

func (s *TemplateLibraryService) requireTemplateCompanyAccess(ctx context.Context, actor domain.ActorContext, tmpl *domain.RfxTemplate, manage bool) error {
	if err := s.auth.requireBuyerActor(ctx, actor); err != nil {
		return err
	}
	if tmpl.OwnerCompanyID == nil {
		return nil
	}
	buyerCompanyIDs, err := s.auth.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return err
	}
	if !domain.ContainsCompanyID(buyerCompanyIDs, *tmpl.OwnerCompanyID) {
		if manage {
			return apperrors.Forbidden("buyer company membership is required for this template")
		}
		return apperrors.Forbidden("buyer company membership is required for this template")
	}
	return nil
}

func (s *TemplateLibraryService) applyTemplateListScope(ctx context.Context, actor domain.ActorContext, filter *domain.TemplateListFilter) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if err := s.denyCarrierOnly(ctx, actor); err != nil {
		return err
	}
	if err := s.auth.requireBuyerActor(ctx, actor); err != nil {
		return err
	}
	buyerCompanyIDs, err := s.auth.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return err
	}
	if len(buyerCompanyIDs) == 0 {
		return apperrors.Forbidden("buyer company membership is required")
	}
	filter.IncludeTenantWide = true
	if filter.OwnerCompanyID != nil {
		if !domain.ContainsCompanyID(buyerCompanyIDs, *filter.OwnerCompanyID) {
			filter.DenyAll = true
			return nil
		}
		filter.AccessibleOwnerCompanyIDs = []uuid.UUID{*filter.OwnerCompanyID}
		filter.IncludeTenantWide = false
		return nil
	}
	filter.AccessibleOwnerCompanyIDs = buyerCompanyIDs
	return nil
}

func (s *TemplateLibraryService) requireBuyerRead(ctx context.Context, actor domain.ActorContext) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	return s.denyCarrierOnly(ctx, actor)
}

func (s *TemplateLibraryService) requireBuyerManage(ctx context.Context, actor domain.ActorContext) error {
	if err := s.auth.requireBuyerActor(ctx, actor); err != nil {
		return err
	}
	return s.denyCarrierOnly(ctx, actor)
}

func (s *TemplateLibraryService) denyCarrierOnly(ctx context.Context, actor domain.ActorContext) error {
	resolver, ok := s.auth.actors.(CompanyMembershipResolver)
	if !ok {
		return apperrors.Forbidden("buyer role is required")
	}
	roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return err
	}
	if domain.HasCarrierRole(roles) && !domain.HasBuyerRole(roles) {
		return apperrors.Forbidden("carrier access to templates is forbidden")
	}
	if !domain.HasBuyerRole(roles) {
		return apperrors.Forbidden("buyer role is required")
	}
	return nil
}

func (s *TemplateLibraryService) resolveOwnerCompany(ctx context.Context, actor domain.ActorContext, requested *uuid.UUID) (*uuid.UUID, error) {
	if requested == nil {
		return nil, nil
	}
	buyerCompanyIDs, err := s.auth.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	resolved, err := domain.ResolveBuyerCompanyID(*requested, buyerCompanyIDs)
	if err != nil {
		return nil, err
	}
	id := resolved
	return &id, nil
}

func (s *TemplateLibraryService) loadTemplateVersionReplay(ctx context.Context, scope repository.IdempotencyScope, idempotencyKey, requestBodyHash string) (*domain.RfxTemplateVersion, error) {
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
	var version domain.RfxTemplateVersion
	if err := json.Unmarshal(record.ResponseBody, &version); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent replay payload", err)
	}
	return &version, nil
}

func ownerCompanyIDValue(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

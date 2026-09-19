package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

type ErpUpdateCommitResponse struct {
	RfxEventID uuid.UUID `json:"rfx_event_id"`
	AppliedAt  time.Time `json:"applied_at"`
}

func (s *ErpIntegrationService) CommitUpdateDraft(
	ctx context.Context,
	actor IntegrationActor,
	eventID uuid.UUID,
	in domain.ErpUpdateCommitInput,
	idempotencyKey string,
) (*ErpUpdateCommitResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if !actor.HasScopes(domain.ScopeDraftCommit, domain.ScopeDraftRead) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	if eventID == uuid.Nil {
		return nil, apperrors.Validation("invalid event id", map[string]any{"field": "id"})
	}
	if err := domain.ValidateErpUpdateCommitInput(in); err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	if len(idempotencyKey) > 128 {
		return nil, apperrors.Validation("Idempotency-Key header is too long", map[string]any{"field": "Idempotency-Key"})
	}
	if s.txRunner == nil || s.importAnalysisRepo == nil || s.rfxRepo == nil || s.qRepo == nil {
		return nil, apperrors.Internal("erp update commit is not configured", nil)
	}
	if s.idemRepo == nil || s.linkRepo == nil || s.mappingRepo == nil || s.auditRepo == nil {
		return nil, apperrors.Internal("erp update commit repositories are not configured", nil)
	}

	scope := repository.IdempotencyScope{
		TenantID:               actor.TenantID,
		IntegrationPrincipalID: actor.PrincipalID,
		OwnerKind:              domain.OwnerKindIntegrationPrincipal,
		Operation:              domain.ERPBuyerUpdateCommitOperation,
		AggregateScope:         eventID,
	}
	requestBodyHash, err := hashRequestBody(domain.NewErpUpdateCommitIdempotencyPayload(in.AnalysisID))
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadUpdateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}

	var result *ErpUpdateCommitResponse
	runErr := s.txRunner.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		idemRepo := s.idemRepo.WithTx(tx)
		auditRepo := s.auditRepo.WithTx(tx)
		importRepo := s.importAnalysisRepo.WithTx(tx)
		rfxRepo := s.rfxRepo.WithTx(tx)
		qRepo := s.qRepo.WithTx(tx)
		linkRepo := s.linkRepo.WithTx(tx)
		mappingRepo := s.mappingRepo.WithTx(tx)

		if record, err := idemRepo.Get(ctx, scope, idempotencyKey); err != nil {
			return err
		} else if record != nil {
			if record.RequestBodyHash != requestBodyHash {
				return commitConflict(domain.MachineCodeIdempotencyConflict, "idempotency key was already used with a different request body")
			}
			var replay ErpUpdateCommitResponse
			if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
				return apperrors.Internal("failed to decode idempotent commit replay payload", err)
			}
			result = &replay
			return nil
		}
		if hook := s.afterUpdateCommitIdempotencyMiss; hook != nil {
			hook()
		}

		eventState, err := qRepo.LockEventVersionState(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		if eventState.DraftVersionID == nil {
			return commitConflict(domain.MachineCodeStaleTarget, "draft questionnaire version not found")
		}
		event, err := rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
		if err != nil {
			if isNotFound(err) {
				return apperrors.NotFound("rfx event not found")
			}
			return err
		}
		if event.OwnerCompanyID != actor.CompanyID {
			return apperrors.NotFound("rfx event not found")
		}
		if event.Status != domain.RfxStatusDraft {
			return commitConflict(domain.MachineCodeStaleTarget, "rfx event is not a mutable draft")
		}

		draft, err := qRepo.LockVersionByID(ctx, *eventState.DraftVersionID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.EnsureDraftVersionMutable(draft.Status); err != nil {
			return commitConflict(domain.MachineCodeProposalRevalidation, "draft questionnaire version is not mutable")
		}

		analysis, err := importRepo.LockImportAnalysisForUpdate(ctx, in.AnalysisID, actor.TenantID)
		if err != nil {
			if isNotFound(err) {
				return analysisNotFound()
			}
			return err
		}
		if err := s.validateUpdateCommitAnalysis(ctx, actor, eventID, analysis, mappingRepo); err != nil {
			return err
		}

		stored, err := erpjson.ParseStoredUpdateCanonical(analysis.CanonicalPayloadJSON)
		if err != nil {
			return err
		}
		if err := validateStoredUpdateOperation(stored); err != nil {
			return err
		}

		if stored.EventRowVersion != eventState.EventVersion {
			return commitConflict(domain.MachineCodeStaleTarget, "event_row_version is stale")
		}
		if stored.DraftRowVersion != draft.Version {
			return commitConflict(domain.MachineCodeProposalRevalidation, "draft_row_version is stale")
		}

		currentLots, err := rfxRepo.LockEventLotsForUpdate(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		currentFingerprint, err := xlsxexchange.ComputeBaselineLotsFingerprint(currentLots)
		if err != nil {
			return apperrors.Internal("failed to compute baseline lots fingerprint", err)
		}
		if stored.BaselineLotsFingerprint != currentFingerprint {
			return commitConflict(domain.MachineCodeStaleTarget, "baseline_lots_fingerprint is stale")
		}

		if err := revalidateStoredUpdateGraph(stored); err != nil {
			return err
		}

		now := s.nowFn().UTC()
		title := strings.TrimSpace(stored.Event.Title)
		if title != "" {
			var description *string
			if trimmed := strings.TrimSpace(stored.Event.Description); trimmed != "" {
				description = &trimmed
			}
			if _, err := rfxRepo.UpdateEvent(ctx, eventID, actor.TenantID, domain.UpdateRfxEventInput{
				Title:            &title,
				Description:      description,
				ResponseDeadline: stored.Event.Deadline,
			}); err != nil {
				return err
			}
		}

		graph := xlsxexchange.StoredImportPayloadFromERPGraph(erpLotsFromCanonical(stored.Lots), parseERPQuestionnaireDraft(stored.Questionnaire))
		changes := buyerImportCommitChanges{}
		if len(stored.Lots) > 0 {
			if err := reconcileImportLots(ctx, rfxRepo, actor.TenantID, eventID, graph, &changes.Lots); err != nil {
				return err
			}
		}
		if len(stored.Questionnaire) > 0 {
			if err := reconcileImportQuestionnaire(ctx, qRepo, actor.TenantID, draft.ID, graph, &changes); err != nil {
				return err
			}
		}

		if _, err := qRepo.TouchDraftVersion(ctx, draft.ID, actor.TenantID, draft.Version); err != nil {
			return err
		}

		if err := s.applyUpdateExternalLink(ctx, linkRepo, actor, eventID, analysis.CanonicalHash, stored); err != nil {
			return err
		}

		if err := recordIntegrationAudit(ctx, auditRepo, actor, eventID, domain.ERPUpdateCommitAuditAction, map[string]any{
			"rfx_event_id":             eventID.String(),
			"analysis_id":              analysis.ID.String(),
			"integration_principal_id": actor.PrincipalID.String(),
			"actor_kind":               domain.AuditActorKindIntegration,
			"canonical_hash":           analysis.CanonicalHash,
			"applied_at":               now.Format(time.RFC3339Nano),
			"changes":                  changes,
		}); err != nil {
			return err
		}

		if hook := s.afterUpdateCommitApply; hook != nil {
			if hookErr := hook(); hookErr != nil {
				return hookErr
			}
		}

		if _, err := importRepo.MarkConsumed(ctx, analysis.ID, actor.TenantID, domain.ImportTargetTypeDraftEvent, eventID, now); err != nil {
			return err
		}

		response := &ErpUpdateCommitResponse{
			RfxEventID: eventID,
			AppliedAt:  now,
		}
		responseBody, err := json.Marshal(response)
		if err != nil {
			return apperrors.Internal("failed to marshal commit response", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:               scope.TenantID,
			IntegrationPrincipalID: scope.IntegrationPrincipalID,
			OwnerKind:              scope.OwnerKind,
			Operation:              scope.Operation,
			AggregateScope:         scope.AggregateScope,
			IdempotencyKey:         idempotencyKey,
			RequestBodyHash:        requestBodyHash,
			ResponseStatus:         200,
			ResponseBody:           responseBody,
			ExpiresAt:              now.Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		result = response
		return nil
	})
	if runErr != nil {
		return s.resolveUpdateCommitAfterTx(ctx, scope, idempotencyKey, requestBodyHash, runErr)
	}
	return result, nil
}

func (s *ErpIntegrationService) resolveUpdateCommitAfterTx(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
	runErr error,
) (*ErpUpdateCommitResponse, error) {
	replay, replayErr := s.loadUpdateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash)
	if replayErr != nil {
		return nil, replayErr
	}
	if replay != nil {
		return replay, nil
	}
	if errors.Is(runErr, repository.ErrIdempotencyRecordActive) {
		return nil, apperrors.Internal("idempotency winner record not found after concurrent store conflict", runErr)
	}
	return nil, runErr
}

func (s *ErpIntegrationService) validateUpdateCommitAnalysis(
	ctx context.Context,
	actor IntegrationActor,
	eventID uuid.UUID,
	analysis *domain.ImportAnalysis,
	mappingRepo *repository.ReferenceMappingRepository,
) error {
	if analysis.ActorCompanyID != actor.CompanyID {
		return analysisNotFound()
	}
	if analysis.WorkbookType != domain.WorkbookTypeERPBuyerJSON ||
		analysis.SchemaVersion != domain.SchemaVersionERPJSONV1 ||
		analysis.TargetType != domain.ImportTargetTypeDraftEvent {
		return analysisNotFound()
	}
	if analysis.TargetID == nil || *analysis.TargetID != eventID {
		return commitConflict(domain.MachineCodeStaleTarget, "import analysis target event mismatch")
	}
	if analysis.IntegrationPrincipalID == nil || *analysis.IntegrationPrincipalID != actor.PrincipalID || analysis.ActorID != uuid.Nil {
		return commitConflict(domain.MachineCodeActorBindingDenied, "import analysis actor binding denied")
	}

	now := s.nowFn().UTC()
	if !now.Before(analysis.ExpiresAt) {
		return commitConflict(domain.MachineCodeAnalysisExpired, "import analysis expired")
	}
	if analysis.Status != domain.ImportAnalysisStatusPreviewed {
		return commitConflict(domain.MachineCodeAnalysisAlreadyConsumed, "import analysis already consumed")
	}

	recomputed, err := erpjson.StableHash(analysis.CanonicalPayloadJSON)
	if err != nil || !strings.EqualFold(recomputed, analysis.CanonicalHash) {
		return commitConflict(domain.MachineCodeCanonicalHashMismatch, "stored import analysis hash mismatch")
	}

	stored, err := erpjson.ParseStoredUpdateCanonical(analysis.CanonicalPayloadJSON)
	if err != nil {
		return err
	}
	return verifyPinnedMappingSets(ctx, mappingRepo, stored.MappingContext)
}

func validateStoredUpdateOperation(stored *erpjson.StoredUpdateCanonical) error {
	if stored == nil {
		return analysisNotFound()
	}
	if stored.SchemaVersion != domain.SchemaVersionERPJSONV1 || stored.RequestedOperation != erpjson.OperationUpdateDraft {
		return analysisNotFound()
	}
	if strings.TrimSpace(stored.Event.Type) == "" || strings.TrimSpace(stored.Event.Title) == "" {
		return analysisNotFound()
	}
	return nil
}

func revalidateStoredUpdateGraph(stored *erpjson.StoredUpdateCanonical) error {
	if stored == nil || strings.TrimSpace(stored.Event.Title) == "" || strings.TrimSpace(stored.Event.Type) == "" {
		return commitConflict(domain.MachineCodeProposalRevalidation, "stored import proposal failed revalidation")
	}
	for _, lot := range stored.Lots {
		if strings.TrimSpace(lot.LotNumber) == "" || strings.TrimSpace(lot.Name) == "" {
			return commitConflict(domain.MachineCodeProposalRevalidation, "stored import proposal failed revalidation")
		}
	}
	if len(stored.Questionnaire) > 0 {
		var probe map[string]any
		if err := json.Unmarshal(stored.Questionnaire, &probe); err != nil {
			return commitConflict(domain.MachineCodeProposalRevalidation, "stored import proposal failed revalidation")
		}
	}
	return nil
}

func erpLotsFromCanonical(lots []erpjson.LotPayload) []xlsxexchange.ERPLotDraft {
	out := make([]xlsxexchange.ERPLotDraft, 0, len(lots))
	for _, lot := range lots {
		out = append(out, xlsxexchange.ERPLotDraft{
			LotNumber:      strings.TrimSpace(lot.LotNumber),
			Name:           strings.TrimSpace(lot.Name),
			Description:    strings.TrimSpace(lot.Description),
			Category:       strings.TrimSpace(lot.Category),
			EstimatedValue: lot.EstimatedValue,
			CurrencyCode:   strings.TrimSpace(lot.CurrencyCode),
		})
	}
	return out
}

type erpQuestionnaireWire struct {
	Sections []struct {
		SectionCode string `json:"section_code"`
		Title       string `json:"title"`
		Description string `json:"description"`
		SortOrder   int    `json:"sort_order"`
		Questions   []struct {
			QuestionCode   string          `json:"question_code"`
			QuestionType   string          `json:"question_type"`
			Label          string          `json:"label"`
			HelpText       string          `json:"help_text"`
			Required       bool            `json:"required"`
			SortOrder      int             `json:"sort_order"`
			ValidationJSON json.RawMessage `json:"validation_json"`
		} `json:"questions"`
	} `json:"sections"`
	Questions []xlsxexchange.ERPQuestionDraft `json:"questions"`
	Options   []xlsxexchange.ERPOptionDraft   `json:"options"`
	Rules     []struct {
		RuleCode           string          `json:"rule_code"`
		SourceQuestionCode string          `json:"source_question_code"`
		TargetQuestionCode string          `json:"target_question_code"`
		Action             string          `json:"action"`
		ConditionJSON      json.RawMessage `json:"condition"`
		SortOrder          int             `json:"sort_order"`
	} `json:"rules"`
}

func parseERPQuestionnaireDraft(raw json.RawMessage) xlsxexchange.ERPQuestionnaireDraft {
	if len(raw) == 0 {
		return xlsxexchange.ERPQuestionnaireDraft{}
	}
	var doc erpQuestionnaireWire
	if err := json.Unmarshal(raw, &doc); err != nil {
		return xlsxexchange.ERPQuestionnaireDraft{}
	}
	out := xlsxexchange.ERPQuestionnaireDraft{
		Questions: append([]xlsxexchange.ERPQuestionDraft(nil), doc.Questions...),
		Options:   append([]xlsxexchange.ERPOptionDraft(nil), doc.Options...),
	}
	for i, sec := range doc.Sections {
		sortOrder := sec.SortOrder
		if sortOrder == 0 {
			sortOrder = i + 1
		}
		out.Sections = append(out.Sections, xlsxexchange.ERPSectionDraft{
			SectionCode: strings.TrimSpace(sec.SectionCode),
			Title:       strings.TrimSpace(sec.Title),
			Description: strings.TrimSpace(sec.Description),
			SortOrder:   sortOrder,
		})
		for j, q := range sec.Questions {
			qSort := q.SortOrder
			if qSort == 0 {
				qSort = j + 1
			}
			out.Questions = append(out.Questions, xlsxexchange.ERPQuestionDraft{
				SectionCode:    strings.TrimSpace(sec.SectionCode),
				QuestionCode:   strings.TrimSpace(q.QuestionCode),
				QuestionType:   strings.TrimSpace(q.QuestionType),
				Label:          strings.TrimSpace(q.Label),
				HelpText:       strings.TrimSpace(q.HelpText),
				Required:       q.Required,
				SortOrder:      qSort,
				ValidationJSON: q.ValidationJSON,
			})
		}
	}
	for _, rule := range doc.Rules {
		out.Rules = append(out.Rules, xlsxexchange.ERPRuleDraft{
			RuleCode:           strings.TrimSpace(rule.RuleCode),
			SourceQuestionCode: strings.TrimSpace(rule.SourceQuestionCode),
			TargetQuestionCode: strings.TrimSpace(rule.TargetQuestionCode),
			Action:             strings.TrimSpace(rule.Action),
			ConditionJSON:      rule.ConditionJSON,
			SortOrder:          rule.SortOrder,
		})
	}
	return out
}

func (s *ErpIntegrationService) applyUpdateExternalLink(
	ctx context.Context,
	linkRepo *repository.ExternalObjectLinkRepository,
	actor IntegrationActor,
	eventID uuid.UUID,
	payloadHash string,
	stored *erpjson.StoredUpdateCanonical,
) error {
	var identity *erpjson.ExternalRef
	if stored != nil && stored.External != nil &&
		strings.TrimSpace(stored.External.System) != "" &&
		strings.TrimSpace(stored.External.ObjectID) != "" {
		identity = stored.External
	}

	if identity != nil {
		existing, err := linkRepo.GetByStableIdentity(
			ctx, actor.TenantID, actor.PrincipalID,
			identity.System, domain.ExternalObjectTypeRfxEvent, identity.ObjectID,
		)
		if err != nil && !isNotFound(err) {
			return err
		}
		if existing != nil && existing.RfxEventID != eventID {
			return commitConflict(domain.MachineCodeExternalIDConflict, "stable external identity already exists")
		}
		revision := strings.TrimSpace(identity.Revision)
		if existing != nil {
			if revision == "" {
				revision = existing.ExternalRevision
			}
			if revision == "" {
				revision = existing.ExternalVersion
			}
			if revision == "" {
				revision = "1"
			}
			updated, err := linkRepo.UpdateLinkMetadataInPlace(
				ctx, existing.ID, actor.TenantID, actor.PrincipalID, eventID, revision, payloadHash,
			)
			if err != nil {
				return err
			}
			if revision != existing.ExternalRevision {
				if _, err := linkRepo.RecordRevision(ctx, domain.ExternalObjectLinkRevision{
					TenantID:         actor.TenantID,
					LinkID:           updated.ID,
					ExternalRevision: revision,
					PayloadHash:      payloadHash,
				}); err != nil {
					return err
				}
			}
			return nil
		}
		bound, err := linkRepo.GetByRfxEventID(ctx, actor.TenantID, actor.PrincipalID, eventID)
		if err != nil && !isNotFound(err) {
			return err
		}
		if bound != nil {
			return commitConflict(domain.MachineCodeExternalIDConflict, "stable external identity already exists")
		}
		if revision == "" {
			revision = "1"
		}
		created, err := linkRepo.InsertLink(ctx, domain.ExternalObjectLink{
			TenantID:               actor.TenantID,
			IntegrationPrincipalID: actor.PrincipalID,
			ExternalSystem:         identity.System,
			ExternalObjectType:     domain.ExternalObjectTypeRfxEvent,
			ExternalObjectID:       identity.ObjectID,
			ExternalVersion:        revision,
			ExternalRevision:       revision,
			PayloadHash:            payloadHash,
			RfxEventID:             eventID,
		})
		if err != nil {
			return err
		}
		_, err = linkRepo.RecordRevision(ctx, domain.ExternalObjectLinkRevision{
			TenantID:         actor.TenantID,
			LinkID:           created.ID,
			ExternalRevision: revision,
			PayloadHash:      payloadHash,
		})
		return err
	}

	existing, err := linkRepo.GetByRfxEventID(ctx, actor.TenantID, actor.PrincipalID, eventID)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	revision := existing.ExternalRevision
	if revision == "" {
		revision = existing.ExternalVersion
	}
	if revision == "" {
		revision = "1"
	}
	_, err = linkRepo.UpdateLinkMetadataInPlace(
		ctx, existing.ID, actor.TenantID, actor.PrincipalID, eventID, revision, payloadHash,
	)
	return err
}

func (s *ErpIntegrationService) loadUpdateCommitReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
) (*ErpUpdateCommitResponse, error) {
	if s.idemRepo == nil {
		return nil, nil
	}
	record, err := s.idemRepo.Get(ctx, scope, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	if record.RequestBodyHash != requestBodyHash {
		return nil, commitConflict(domain.MachineCodeIdempotencyConflict, "idempotency key was already used with a different request body")
	}
	var replay ErpUpdateCommitResponse
	if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent commit replay payload", err)
	}
	return &replay, nil
}

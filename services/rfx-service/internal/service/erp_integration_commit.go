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
)

type ErpCreateCommitResponse struct {
	RfxEventID       uuid.UUID `json:"rfx_event_id"`
	ExternalLinkID   uuid.UUID `json:"external_link_id"`
	CreationChannel  string    `json:"creation_channel"`
	ExternalRevision string    `json:"external_revision"`
}

func (s *ErpIntegrationService) CommitCreateDraft(
	ctx context.Context,
	actor IntegrationActor,
	in domain.ErpCreateCommitInput,
	idempotencyKey string,
) (*ErpCreateCommitResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if !actor.HasScopes(domain.ScopeDraftCommit, domain.ScopeDraftCreate) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	if err := domain.ValidateErpCreateCommitInput(in); err != nil {
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
		return nil, apperrors.Internal("erp create commit is not configured", nil)
	}
	if s.idemRepo == nil || s.linkRepo == nil || s.mappingRepo == nil || s.auditRepo == nil {
		return nil, apperrors.Internal("erp create commit repositories are not configured", nil)
	}

	scope := repository.IdempotencyScope{
		TenantID:               actor.TenantID,
		IntegrationPrincipalID: actor.PrincipalID,
		OwnerKind:              domain.OwnerKindIntegrationPrincipal,
		Operation:              domain.ERPBuyerCreateCommitOperation,
		AggregateScope:         uuid.Nil,
	}
	requestBodyHash, err := hashRequestBody(domain.NewErpCreateCommitIdempotencyPayload(in.AnalysisID))
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadCreateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}

	var result *ErpCreateCommitResponse
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
			var replay ErpCreateCommitResponse
			if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
				return apperrors.Internal("failed to decode idempotent commit replay payload", err)
			}
			result = &replay
			return nil
		}

		analysis, err := importRepo.LockImportAnalysisForUpdate(ctx, in.AnalysisID, actor.TenantID)
		if err != nil {
			if isNotFound(err) {
				return analysisNotFound()
			}
			return err
		}
		if err := s.validateCreateCommitAnalysis(ctx, actor, analysis, mappingRepo); err != nil {
			return err
		}

		stored, err := erpjson.ParseStoredCreateCanonical(analysis.CanonicalPayloadJSON)
		if err != nil {
			return err
		}
		if err := validateStoredCreateOperation(stored); err != nil {
			return err
		}

		now := s.nowFn().UTC()
		eventInput := createEventInputFromCanonical(actor, stored)
		if err := domain.ValidateCreateRfxEventInput(eventInput); err != nil {
			return err
		}
		event, err := rfxRepo.CreateEventWithChannel(ctx, eventInput, domain.CreationChannelERP)
		if err != nil {
			return err
		}
		if _, err := qRepo.CreateInitialDraftVersion(ctx, actor.TenantID, event.ID); err != nil {
			return err
		}
		if err := createLotsFromCanonical(ctx, rfxRepo, actor.TenantID, event.ID, stored.Lots); err != nil {
			return err
		}

		revision := stored.External.Revision
		if revision == "" {
			revision = "1"
		}
		link, err := linkRepo.InsertLink(ctx, domain.ExternalObjectLink{
			TenantID:               actor.TenantID,
			IntegrationPrincipalID: actor.PrincipalID,
			ExternalSystem:         stored.External.System,
			ExternalObjectType:     domain.ExternalObjectTypeRfxEvent,
			ExternalObjectID:       stored.External.ObjectID,
			ExternalVersion:        revision,
			ExternalRevision:       revision,
			PayloadHash:            analysis.CanonicalHash,
			RfxEventID:             event.ID,
		})
		if err != nil {
			return err
		}

		if err := recordIntegrationAudit(ctx, auditRepo, actor, event.ID, domain.ERPCreateCommitAuditAction, map[string]any{
			"rfx_event_id":             event.ID.String(),
			"analysis_id":              analysis.ID.String(),
			"external_link_id":         link.ID.String(),
			"creation_channel":         domain.CreationChannelERP,
			"external_revision":        revision,
			"integration_principal_id": actor.PrincipalID.String(),
			"actor_kind":               domain.AuditActorKindIntegration,
			"canonical_hash":           analysis.CanonicalHash,
			"committed_at":             now.Format(time.RFC3339Nano),
		}); err != nil {
			return err
		}

		if _, err := importRepo.MarkConsumed(ctx, analysis.ID, actor.TenantID, domain.ImportTargetTypeNewEvent, event.ID, now); err != nil {
			return err
		}

		response := &ErpCreateCommitResponse{
			RfxEventID:       event.ID,
			ExternalLinkID:   link.ID,
			CreationChannel:  domain.CreationChannelERP,
			ExternalRevision: revision,
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
			ResponseStatus:         201,
			ResponseBody:           responseBody,
			ExpiresAt:              now.Add(24 * time.Hour),
		}); err != nil && !errors.Is(err, repository.ErrIdempotencyRecordActive) {
			return err
		}
		result = response
		return nil
	})
	if runErr != nil {
		if replay, replayErr := s.loadCreateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, runErr
	}
	return result, nil
}

func (s *ErpIntegrationService) validateCreateCommitAnalysis(
	ctx context.Context,
	actor IntegrationActor,
	analysis *domain.ImportAnalysis,
	mappingRepo *repository.ReferenceMappingRepository,
) error {
	if analysis.ActorCompanyID != actor.CompanyID {
		return analysisNotFound()
	}
	if analysis.WorkbookType != domain.WorkbookTypeERPBuyerJSON ||
		analysis.SchemaVersion != domain.SchemaVersionERPJSONV1 ||
		analysis.TargetType != domain.ImportTargetTypeNewEvent {
		return analysisNotFound()
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

	stored, err := erpjson.ParseStoredCreateCanonical(analysis.CanonicalPayloadJSON)
	if err != nil {
		return err
	}
	return verifyPinnedMappingSets(ctx, mappingRepo, stored.MappingContext)
}

func validateStoredCreateOperation(stored *erpjson.StoredCreateCanonical) error {
	if stored == nil || stored.External == nil || strings.TrimSpace(stored.External.System) == "" || strings.TrimSpace(stored.External.ObjectID) == "" {
		return analysisNotFound()
	}
	if stored.SchemaVersion != domain.SchemaVersionERPJSONV1 || stored.RequestedOperation != erpjson.OperationCreateDraft {
		return analysisNotFound()
	}
	if strings.TrimSpace(stored.Event.Type) == "" || strings.TrimSpace(stored.Event.Title) == "" {
		return analysisNotFound()
	}
	return nil
}

func verifyPinnedMappingSets(ctx context.Context, mappingRepo *repository.ReferenceMappingRepository, pin erpjson.MappingContextPin) error {
	type pinRef struct {
		id      uuid.UUID
		version int
	}
	refs := make([]pinRef, 0, len(pin.Pins)+1)
	if pin.MappingSetID != uuid.Nil {
		refs = append(refs, pinRef{id: pin.MappingSetID, version: pin.MappingSetVersion})
	}
	for _, item := range pin.Pins {
		if item.MappingSetID != uuid.Nil {
			refs = append(refs, pinRef{id: item.MappingSetID, version: item.MappingSetVersion})
		}
	}
	if len(refs) == 0 {
		return commitConflict(domain.MachineCodeStaleMappingContext, "mapping context is missing required pins")
	}
	seen := map[uuid.UUID]struct{}{}
	for _, ref := range refs {
		if _, ok := seen[ref.id]; ok {
			continue
		}
		seen[ref.id] = struct{}{}
		set, err := mappingRepo.GetSetByID(ctx, ref.id)
		if err != nil {
			if isNotFound(err) {
				return commitConflict(domain.MachineCodeStaleMappingContext, "mapping set is no longer available")
			}
			return err
		}
		if ref.version > 0 && set.Version != ref.version {
			return commitConflict(domain.MachineCodeStaleMappingContext, "mapping set version no longer matches the preview pin")
		}
		if set.Status == domain.ReferenceMappingSetStatusRetired {
			return commitConflict(domain.MachineCodeStaleMappingContext, "mapping set was retired after preview")
		}
		if set.Status != domain.ReferenceMappingSetStatusActive {
			return commitConflict(domain.MachineCodeStaleMappingContext, "mapping set is not active")
		}
	}
	return nil
}

func createEventInputFromCanonical(actor IntegrationActor, stored *erpjson.StoredCreateCanonical) domain.CreateRfxEventInput {
	var description *string
	if trimmed := strings.TrimSpace(stored.Event.Description); trimmed != "" {
		description = &trimmed
	}
	var currency *string
	if trimmed := strings.TrimSpace(stored.Event.Currency); trimmed != "" {
		currency = &trimmed
	}
	return domain.CreateRfxEventInput{
		TenantID:         actor.TenantID,
		RfxNumber:        "ERP-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12],
		RfxType:          strings.TrimSpace(stored.Event.Type),
		Category:         domain.ERPCreateDefaultCategory,
		Title:            strings.TrimSpace(stored.Event.Title),
		Description:      description,
		OwnerCompanyID:   actor.CompanyID,
		CurrencyCode:     currency,
		ResponseDeadline: stored.Event.Deadline,
	}
}

func createLotsFromCanonical(ctx context.Context, rfxRepo *repository.RfxRepository, tenantID, eventID uuid.UUID, lots []erpjson.LotPayload) error {
	for _, lot := range lots {
		in := domain.CreateRfxLotInput{
			TenantID:   tenantID,
			RfxEventID: eventID,
			LotNumber:  strings.TrimSpace(lot.LotNumber),
			Name:       strings.TrimSpace(lot.Name),
		}
		if trimmed := strings.TrimSpace(lot.Description); trimmed != "" {
			in.Description = &trimmed
		}
		if trimmed := strings.TrimSpace(lot.Category); trimmed != "" {
			in.Category = &trimmed
		}
		if trimmed := strings.TrimSpace(lot.CurrencyCode); trimmed != "" {
			in.CurrencyCode = &trimmed
		}
		in.EstimatedValue = lot.EstimatedValue
		if _, err := rfxRepo.CreateLot(ctx, in); err != nil {
			return err
		}
	}
	return nil
}

func (s *ErpIntegrationService) loadCreateCommitReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
) (*ErpCreateCommitResponse, error) {
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
	var replay ErpCreateCommitResponse
	if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent commit replay payload", err)
	}
	return &replay, nil
}

func recordIntegrationAudit(
	ctx context.Context,
	audit AuditRecorder,
	actor IntegrationActor,
	entityID uuid.UUID,
	action string,
	metadata map[string]any,
) error {
	if audit == nil {
		return nil
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["actor_kind"] = domain.AuditActorKindIntegration
	metadata["integration_principal_id"] = actor.PrincipalID.String()
	companyID := actor.CompanyID
	return audit.Record(ctx, repository.AuditRecord{
		TenantID:       actor.TenantID,
		EntityType:     "rfx_event",
		EntityID:       entityID,
		Action:         action,
		ActorCompanyID: &companyID,
		Metadata:       metadata,
	})
}

func commitConflict(machineCode, message string) error {
	return apperrors.Conflict(message, map[string]any{"machine_code": machineCode})
}

func analysisNotFound() error {
	return &apperrors.AppError{
		Code:    apperrors.CodeNotFound,
		Message: "import analysis not found",
		Details: map[string]any{"machine_code": domain.MachineCodeAnalysisNotFound},
	}
}

func isNotFound(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

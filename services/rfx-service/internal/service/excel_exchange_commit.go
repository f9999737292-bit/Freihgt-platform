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
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

// BuyerImportCommitResponse is the successful commit envelope.
type BuyerImportCommitResponse struct {
	EventID        uuid.UUID                `json:"event_id"`
	DraftVersionID uuid.UUID                `json:"draft_version_id"`
	AnalysisID     uuid.UUID                `json:"analysis_id"`
	EventVersion   int                      `json:"event_version"`
	DraftVersion   int                      `json:"draft_version"`
	CommittedAt    time.Time                `json:"committed_at"`
	Changes        buyerImportCommitChanges `json:"changes"`
}

// CommitBuyerImportAnalysis applies a persisted preview analysis to the active draft atomically.
func (s *ExcelExchangeService) CommitBuyerImportAnalysis(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	in domain.BuyerImportCommitInput,
	idempotencyKey string,
) (*BuyerImportCommitResponse, error) {
	event, err := s.authorizeBuyerManage(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateBuyerImportCommitInput(in); err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, apperrors.Validation("Idempotency-Key header is required", map[string]any{"field": "Idempotency-Key"})
	}
	if len(idempotencyKey) > 128 {
		return nil, apperrors.Validation("Idempotency-Key header is too long", map[string]any{"field": "Idempotency-Key"})
	}
	if s.txRunner == nil || s.importAnalysisRepo == nil || s.idemRepo == nil || s.auditRepo == nil {
		return nil, apperrors.Internal("buyer import commit is not configured", nil)
	}
	if s.rfxRepoFull == nil || s.qRepoFull == nil {
		return nil, apperrors.Internal("buyer import commit repositories are not configured", nil)
	}

	scope := repository.IdempotencyScope{
		TenantID:       actor.TenantID,
		ActorID:        actor.UserID,
		Operation:      domain.BuyerXlsxImportCommitOperation,
		AggregateScope: eventID,
	}
	requestBodyHash, err := hashRequestBody(domain.NewBuyerImportCommitIdempotencyPayload(in.AnalysisID))
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}

	var result *BuyerImportCommitResponse
	runErr := s.txRunner.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		idemRepo := s.idemRepo.WithTx(tx)
		auditRepo := s.auditRepo.WithTx(tx)
		importRepo := s.importAnalysisRepo.WithTx(tx)
		qRepo := s.qRepoFull.WithTx(tx)
		rfxRepo := s.rfxRepoFull.WithTx(tx)

		if record, err := idemRepo.Get(ctx, scope, idempotencyKey); err != nil {
			return err
		} else if record != nil {
			if record.RequestBodyHash != requestBodyHash {
				return apperrors.Conflict("idempotency key was already used with a different request body", map[string]any{
					"field":        "Idempotency-Key",
					"machine_code": domain.MachineCodeIdempotencyConflict,
				})
			}
			var replay BuyerImportCommitResponse
			if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
				return apperrors.Internal("failed to decode idempotent commit replay payload", err)
			}
			result = &replay
			return nil
		}

		eventState, err := qRepo.LockEventVersionState(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		if eventState.DraftVersionID == nil {
			return apperrors.Conflict("draft questionnaire version not found", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}

		draft, err := qRepo.LockVersionByID(ctx, *eventState.DraftVersionID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.EnsureDraftVersionMutable(draft.Status); err != nil {
			return err
		}

		analysis, err := importRepo.LockImportAnalysisForUpdate(ctx, in.AnalysisID, actor.TenantID)
		if err != nil {
			return err
		}
		if analysis.ActorID != actor.UserID {
			return apperrors.Forbidden("import analysis actor binding denied")
		}
		if analysis.ActorCompanyID != event.OwnerCompanyID {
			return apperrors.Forbidden("import analysis actor company binding denied")
		}
		if analysis.TargetVersion == nil || *analysis.TargetVersion != draft.VersionNumber {
			return apperrors.Conflict("import analysis target version mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if analysis.TargetID == nil || *analysis.TargetID != eventID {
			return apperrors.Conflict("import analysis target event mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if analysis.TargetType != domain.ImportTargetTypeDraftEvent {
			return apperrors.Conflict("import analysis target type mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}

		now := s.nowFn().UTC()
		if !now.Before(analysis.ExpiresAt) {
			return apperrors.Conflict("import analysis expired", map[string]any{
				"machine_code": domain.MachineCodeAnalysisExpired,
			})
		}
		if analysis.Status == domain.ImportAnalysisStatusConsumed {
			return apperrors.Conflict("import analysis already consumed", map[string]any{
				"machine_code": domain.MachineCodeAnalysisAlreadyConsumed,
			})
		}
		if analysis.Status != domain.ImportAnalysisStatusPreviewed {
			return apperrors.Conflict("import analysis is not consumable", map[string]any{
				"machine_code": domain.MachineCodeAnalysisAlreadyConsumed,
			})
		}

		if err := xlsxexchange.VerifyStoredCanonicalPayloadHash(analysis.CanonicalPayloadJSON, analysis.CanonicalHash); err != nil {
			return apperrors.Unprocessable("stored import analysis hash mismatch", map[string]any{
				"machine_code": domain.MachineCodeCanonicalHashMismatch,
			})
		}
		stored, err := xlsxexchange.ParseStoredImportPayload(analysis.CanonicalPayloadJSON)
		if err != nil {
			return apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
			})
		}
		if stored.TargetEventID != eventID ||
			stored.TargetDraftVersionID != draft.ID ||
			stored.EventRowVersion != eventState.EventVersion ||
			stored.DraftRowVersion != draft.Version ||
			stored.TargetVersionNumber != draft.VersionNumber {
			return apperrors.Conflict("import analysis baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
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
			return apperrors.Conflict("import analysis lot baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}

		target := xlsxexchange.TargetDraftBaseline{
			TenantID:           actor.TenantID,
			EventID:            eventID,
			DraftVersionID:     draft.ID,
			DraftVersionNumber: draft.VersionNumber,
			EventRowVersion:    eventState.EventVersion,
			DraftRowVersion:    draft.Version,
		}
		proposal := xlsxexchange.ProposalFromStoredPayload(stored, target)
		if validationErrors := xlsxexchange.ValidateCommitProposal(ctx, target, proposal); len(validationErrors) > 0 {
			return apperrors.Unprocessable("stored import proposal failed revalidation", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"errors":       validationErrors,
			})
		}

		oldEventVersion := eventState.EventVersion
		oldDraftVersion := draft.Version
		changes := buyerImportCommitChanges{}
		if err := reconcileImportLots(ctx, rfxRepo, actor.TenantID, eventID, stored, &changes.Lots); err != nil {
			return err
		}
		if err := reconcileImportQuestionnaire(ctx, qRepo, actor.TenantID, draft.ID, stored, &changes); err != nil {
			return err
		}

		updatedDraft, err := qRepo.TouchDraftVersion(ctx, draft.ID, actor.TenantID, draft.Version)
		if err != nil {
			return err
		}

		if err := recordAudit(ctx, auditRepo, actor, event.OwnerCompanyID, "rfx_event", eventID, "rfx.buyer_xlsx_import.committed.v1", map[string]any{
			"event_id":          eventID.String(),
			"analysis_id":       analysis.ID.String(),
			"actor_id":          actor.UserID.String(),
			"draft_version_id":  draft.ID.String(),
			"old_event_version": oldEventVersion,
			"new_event_version": eventState.EventVersion,
			"old_draft_version": oldDraftVersion,
			"new_draft_version": updatedDraft.Version,
			"canonical_hash":    analysis.CanonicalHash,
			"committed_at":      now.Format(time.RFC3339Nano),
			"changes":           changes,
		}); err != nil {
			return err
		}

		if _, err := importRepo.MarkConsumed(ctx, analysis.ID, actor.TenantID, domain.ImportTargetTypeDraftEvent, draft.ID, now); err != nil {
			return err
		}

		response := &BuyerImportCommitResponse{
			EventID:        eventID,
			DraftVersionID: draft.ID,
			AnalysisID:     analysis.ID,
			EventVersion:   eventState.EventVersion,
			DraftVersion:   updatedDraft.Version,
			CommittedAt:    now,
			Changes:        changes,
		}
		responseBody, err := json.Marshal(response)
		if err != nil {
			return apperrors.Internal("failed to marshal commit response", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        scope.TenantID,
			ActorID:         scope.ActorID,
			Operation:       scope.Operation,
			AggregateScope:  scope.AggregateScope,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  200,
			ResponseBody:    responseBody,
			ExpiresAt:       now.Add(24 * time.Hour),
		}); err != nil && !errors.Is(err, repository.ErrIdempotencyRecordActive) {
			return err
		}
		result = response
		return nil
	})
	if runErr != nil {
		if replay, replayErr := s.loadCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, runErr
	}
	return result, nil
}

func (s *ExcelExchangeService) loadCommitReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
) (*BuyerImportCommitResponse, error) {
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
		return nil, apperrors.Conflict("idempotency key was already used with a different request body", map[string]any{
			"field":        "Idempotency-Key",
			"machine_code": domain.MachineCodeIdempotencyConflict,
		})
	}
	var replay BuyerImportCommitResponse
	if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent commit replay payload", err)
	}
	return &replay, nil
}

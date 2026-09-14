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

// CarrierImportCommitResponse is the successful carrier commit envelope.
type CarrierImportCommitResponse struct {
	EventID     uuid.UUID                  `json:"event_id"`
	ResponseID  uuid.UUID                  `json:"response_id"`
	AnalysisID  uuid.UUID                  `json:"analysis_id"`
	SaveVersion int64                      `json:"save_version"`
	CommittedAt time.Time                  `json:"committed_at"`
	Changes     carrierImportCommitChanges `json:"changes"`
}

// CommitCarrierImportAnalysis applies a persisted carrier preview analysis to a DRAFT response atomically.
func (s *ExcelExchangeService) CommitCarrierImportAnalysis(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	responseID uuid.UUID,
	in domain.CarrierImportCommitInput,
	idempotencyKey string,
) (*CarrierImportCommitResponse, error) {
	event, response, err := s.authorizeCarrierResponsePreview(ctx, actor, eventID, responseID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCarrierImportCommitInput(in); err != nil {
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
		return nil, apperrors.Internal("carrier import commit is not configured", nil)
	}
	if s.rfxRepoFull == nil || s.qRepoFull == nil || s.answerRepoFull == nil {
		return nil, apperrors.Internal("carrier import commit repositories are not configured", nil)
	}

	scope := repository.IdempotencyScope{
		TenantID:       actor.TenantID,
		ActorID:        actor.UserID,
		Operation:      domain.CarrierXlsxImportCommitOperation,
		AggregateScope: responseID,
	}
	requestBodyHash, err := hashRequestBody(domain.NewCarrierImportCommitIdempotencyPayload(in.AnalysisID))
	if err != nil {
		return nil, err
	}
	if replay, err := s.loadCarrierCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}

	now := s.nowFn().UTC()
	permission, err := s.resolveLatePermission(ctx, event, response.ParticipantCompanyID, now)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCarrierMutationDeadline(event.ResponseDeadline, now, permission); err != nil {
		return nil, err
	}
	lateSave := domain.ResponseDeadlinePassed(event.ResponseDeadline, now)

	var result *CarrierImportCommitResponse
	runErr := s.txRunner.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		idemRepo := s.idemRepo.WithTx(tx)
		auditRepo := s.auditRepo.WithTx(tx)
		importRepo := s.importAnalysisRepo.WithTx(tx)
		qRepo := s.qRepoFull.WithTx(tx)
		rfxRepo := s.rfxRepoFull.WithTx(tx)
		answerRepo := s.answerRepoFull.WithTx(tx)

		if record, err := idemRepo.Get(ctx, scope, idempotencyKey); err != nil {
			return err
		} else if record != nil {
			if record.RequestBodyHash != requestBodyHash {
				return apperrors.Conflict("idempotency key was already used with a different request body", map[string]any{
					"field":        "Idempotency-Key",
					"machine_code": domain.MachineCodeIdempotencyConflict,
				})
			}
			var replay CarrierImportCommitResponse
			if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
				return apperrors.Internal("failed to decode idempotent carrier commit replay payload", err)
			}
			result = &replay
			return nil
		}

		analysis, err := importRepo.LockImportAnalysisForUpdate(ctx, in.AnalysisID, actor.TenantID)
		if err != nil {
			return err
		}
		if analysis.ActorID != actor.UserID {
			return apperrors.Forbidden("import analysis actor binding denied")
		}
		if analysis.ActorCompanyID != response.ParticipantCompanyID {
			return apperrors.Forbidden("import analysis actor company binding denied")
		}
		if analysis.WorkbookType != domain.WorkbookTypeCarrierOffer {
			return apperrors.Conflict("import analysis workbook type mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if analysis.SchemaVersion != domain.SchemaVersionCarrierXLSXV1 {
			return apperrors.Conflict("import analysis schema version mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if analysis.TargetType != domain.ImportTargetTypeCarrierResponse {
			return apperrors.Conflict("import analysis target type mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if analysis.TargetID == nil || *analysis.TargetID != responseID {
			return apperrors.Conflict("import analysis target response mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		targetVersion := int(response.SaveVersion)
		if analysis.TargetVersion == nil || *analysis.TargetVersion != targetVersion {
			return apperrors.Conflict("import analysis target save version mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}

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

		if err := xlsxexchange.VerifyStoredCarrierCanonicalPayloadHash(analysis.CanonicalPayloadJSON, analysis.CanonicalHash); err != nil {
			return apperrors.Unprocessable("stored import analysis hash mismatch", map[string]any{
				"machine_code": domain.MachineCodeCanonicalHashMismatch,
			})
		}
		stored, err := xlsxexchange.ParseStoredCarrierPayload(analysis.CanonicalPayloadJSON)
		if err != nil {
			return apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
			})
		}

		locked, err := rfxRepo.LockResponseForUpdate(ctx, responseID, actor.TenantID)
		if err != nil {
			return err
		}
		if locked.Status != domain.RfxResponseStatusDraft {
			return apperrors.Conflict("response is not editable", map[string]any{
				"machine_code": xlsxexchange.MachineCodeResponseNotEditable,
				"status":       locked.Status,
			})
		}
		if locked.RfxEventID != eventID {
			return apperrors.NotFound("rfx response not found")
		}
		if locked.SaveVersion != stored.ResponseSaveVersion {
			return apperrors.Conflict("import analysis baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if locked.RfxVersionID == nil || *locked.RfxVersionID != stored.TargetRfxVersionID {
			return apperrors.Conflict("import analysis questionnaire version mismatch", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}

		version, err := qRepo.GetVersionByID(ctx, *locked.RfxVersionID, actor.TenantID)
		if err != nil {
			return err
		}
		sections, err := qRepo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
		if err != nil {
			return err
		}
		rules, err := qRepo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
		if err != nil {
			return err
		}
		lots, err := rfxRepo.ListLotsByEvent(ctx, eventID, actor.TenantID)
		if err != nil {
			return err
		}
		currentAnswers, err := answerRepo.ListByResponse(ctx, responseID, actor.TenantID)
		if err != nil {
			return err
		}
		currentOfferLines, err := rfxRepo.ListOfferLinesByResponse(ctx, responseID, actor.TenantID)
		if err != nil {
			return err
		}

		target, err := buildTargetCarrierBaseline(event, locked, version, sections, rules, lots, currentAnswers, currentOfferLines)
		if err != nil {
			return apperrors.Internal("failed to build carrier import baseline", err)
		}

		if stored.TargetEventID != eventID ||
			stored.TargetResponseID != responseID ||
			stored.TargetRfxVersionID != *locked.RfxVersionID ||
			stored.TargetVersionNumber != version.VersionNumber ||
			stored.EventRowVersion != event.Version ||
			stored.ResponseSaveVersion != locked.SaveVersion ||
			stored.CarrierCompanyID != locked.ParticipantCompanyID {
			return apperrors.Conflict("import analysis baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if stored.AvailableLotsFingerprint != target.AvailableLotsFingerprint {
			return apperrors.Conflict("import analysis lot baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if stored.AnswersFingerprint != target.AnswersFingerprint {
			return apperrors.Conflict("import analysis answers baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}
		if stored.OfferLinesFingerprint != target.OfferLinesFingerprint {
			return apperrors.Conflict("import analysis offer lines baseline is stale", map[string]any{
				"machine_code": domain.MachineCodeStaleTarget,
			})
		}

		proposal := xlsxexchange.ProposalFromStoredCarrierPayload(stored, target)
		if validationErrors := xlsxexchange.ValidateCarrierCommitProposal(ctx, target, proposal); len(validationErrors) > 0 {
			return apperrors.Unprocessable("stored import proposal failed revalidation", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"errors":       validationErrors,
			})
		}

		if lateSave {
			if permission == nil || s.late == nil {
				return apperrors.Unprocessable("approved late submission permission is required", map[string]any{"field": "late_submission_request"})
			}
			if err := s.late.EnsureApprovedWindowTx(ctx, tx, permission.ID, actor.TenantID, eventID, locked.ParticipantCompanyID, now); err != nil {
				return err
			}
		}

		changes := carrierImportCommitChanges{}
		if err := reconcileCarrierImportAnswers(
			ctx,
			answerRepo,
			actor.TenantID,
			responseID,
			actor.UserID,
			target.Questionnaire,
			currentAnswers,
			proposal,
			&changes.Answers,
		); err != nil {
			return err
		}
		if err := reconcileCarrierImportOfferLines(
			ctx,
			rfxRepo,
			actor.TenantID,
			responseID,
			target,
			currentOfferLines,
			proposal,
			&changes.OfferLines,
		); err != nil {
			return err
		}

		mergedAnswers, err := answerRepo.ListByResponse(ctx, responseID, actor.TenantID)
		if err != nil {
			return err
		}
		rt := domain.BuildQuestionnaireRuntime(&target.Questionnaire)
		merged := answersMap(mergedAnswers)
		completion := domain.ComputeCompletionPercent(rt, merged)

		updated, err := rfxRepo.UpdateResponseAfterSave(ctx, responseID, actor.TenantID, locked.SaveVersion, actor.UserID, completion)
		if err != nil {
			return err
		}

		if err := recordAudit(ctx, auditRepo, actor, locked.ParticipantCompanyID, "rfx_response", responseID, "rfx.carrier_xlsx_import.committed.v1", map[string]any{
			"event_id":         eventID.String(),
			"response_id":      responseID.String(),
			"analysis_id":      analysis.ID.String(),
			"actor_id":         actor.UserID.String(),
			"old_save_version": locked.SaveVersion,
			"new_save_version": updated.SaveVersion,
			"canonical_hash":   analysis.CanonicalHash,
			"committed_at":     now.Format(time.RFC3339Nano),
			"changes":          changes,
		}); err != nil {
			return err
		}

		if _, err := importRepo.MarkConsumed(ctx, analysis.ID, actor.TenantID, domain.ImportTargetTypeCarrierResponse, responseID, now); err != nil {
			return err
		}

		responseBody := &CarrierImportCommitResponse{
			EventID:     eventID,
			ResponseID:  responseID,
			AnalysisID:  analysis.ID,
			SaveVersion: updated.SaveVersion,
			CommittedAt: now,
			Changes:     changes,
		}
		encoded, err := json.Marshal(responseBody)
		if err != nil {
			return apperrors.Internal("failed to marshal carrier commit response", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        scope.TenantID,
			ActorID:         scope.ActorID,
			Operation:       scope.Operation,
			AggregateScope:  scope.AggregateScope,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  200,
			ResponseBody:    encoded,
			ExpiresAt:       now.Add(24 * time.Hour),
		}); err != nil && !errors.Is(err, repository.ErrIdempotencyRecordActive) {
			return err
		}
		result = responseBody
		return nil
	})
	if runErr != nil {
		if replay, replayErr := s.loadCarrierCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, runErr
	}
	return result, nil
}

func (s *ExcelExchangeService) loadCarrierCommitReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
) (*CarrierImportCommitResponse, error) {
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
	var replay CarrierImportCommitResponse
	if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent carrier commit replay payload", err)
	}
	return &replay, nil
}

func (s *ExcelExchangeService) resolveLatePermission(
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

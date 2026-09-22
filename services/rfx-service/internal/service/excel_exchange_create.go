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

// BuyerXlsxCreatePreviewResponse is the CREATE_NEW_DRAFT preview envelope.
type BuyerXlsxCreatePreviewResponse struct {
	Mode                   string                                 `json:"mode"`
	SchemaName             string                                 `json:"schema_name"`
	SchemaVersion          string                                 `json:"schema_version"`
	OwnerCompanyID         uuid.UUID                              `json:"owner_company_id"`
	AnalysisID             *uuid.UUID                             `json:"analysis_id,omitempty"`
	ExpiresAt              *time.Time                             `json:"expires_at,omitempty"`
	ReadyToCommit          bool                                   `json:"ready_to_commit"`
	NormalizedDraftSummary BuyerXlsxCreateDraftSummary            `json:"normalized_draft_summary"`
	ChangeCounts           xlsxexchange.BuyerCreateChangeCounts   `json:"change_counts"`
	Errors                 []xlsxexchange.BuyerImportIssue        `json:"errors"`
	Warnings               []xlsxexchange.BuyerImportIssue        `json:"warnings"`
}

// BuyerXlsxCreateDraftSummary is the human-visible created event shell plus graph counts.
type BuyerXlsxCreateDraftSummary struct {
	RfxNumber        string     `json:"rfx_number"`
	Title            string     `json:"title"`
	RfxType          string     `json:"rfx_type"`
	Category         string     `json:"category"`
	Description      *string    `json:"description,omitempty"`
	ResponseDeadline *time.Time `json:"response_deadline,omitempty"`
	CurrencyCode     *string    `json:"currency_code,omitempty"`
	LotCount         int        `json:"lot_count"`
	SectionCount     int        `json:"section_count"`
	QuestionCount    int        `json:"question_count"`
}

// BuyerXlsxCreateCommitResponse is the successful CREATE commit envelope.
type BuyerXlsxCreateCommitResponse struct {
	EventID              uuid.UUID                `json:"event_id"`
	AnalysisID           uuid.UUID                `json:"analysis_id"`
	CreationChannel      string                   `json:"creation_channel"`
	Status               string                   `json:"status"`
	DraftVersionID       uuid.UUID                `json:"draft_version_id"`
	DraftVersionNumber   int                      `json:"draft_version_number"`
	QuestionnaireEnabled bool                     `json:"questionnaire_enabled"`
	CreatedCounts        buyerImportCommitChanges `json:"created_counts"`
	CommittedAt          time.Time                `json:"committed_at"`
}

// BuyerXlsxCreatePreviewInput is the resolved multipart CREATE preview request.
type BuyerXlsxCreatePreviewInput struct {
	OwnerCompanyID   uuid.UUID
	RfxNumber        string
	Title            string
	RfxType          string
	Category         string
	Description      *string
	ResponseDeadline *time.Time
	CurrencyCode     *string
	WorkbookBytes    []byte
}

// PreviewBuyerXlsxCreateWorkbook parses a V1 workbook for a new DRAFT event without writing RFx rows.
func (s *ExcelExchangeService) PreviewBuyerXlsxCreateWorkbook(
	ctx context.Context,
	actor domain.ActorContext,
	in BuyerXlsxCreatePreviewInput,
) (*BuyerXlsxCreatePreviewResponse, error) {
	ownerCompanyID, err := s.resolveCreateFromXlsxOwner(ctx, actor, in.OwnerCompanyID)
	if err != nil {
		return nil, err
	}
	in.OwnerCompanyID = ownerCompanyID

	preview, err := xlsxexchange.ParseBuyerCreatePreview(ctx, in.WorkbookBytes)
	if err != nil {
		return nil, apperrors.Internal("failed to parse buyer create preview", err)
	}

	shell := xlsxexchange.BuyerCreateEventShell{
		RfxNumber:        strings.TrimSpace(in.RfxNumber),
		Title:            strings.TrimSpace(in.Title),
		RfxType:          strings.TrimSpace(in.RfxType),
		Category:         strings.TrimSpace(in.Category),
		Description:      in.Description,
		ResponseDeadline: in.ResponseDeadline,
		CurrencyCode:     in.CurrencyCode,
		OwnerCompanyID:   ownerCompanyID,
	}
	if shellErr := domain.ValidateCreateRfxEventInput(domain.CreateRfxEventInput{
		TenantID:         actor.TenantID,
		RfxNumber:        shell.RfxNumber,
		RfxType:          shell.RfxType,
		Category:         shell.Category,
		Title:            shell.Title,
		Description:      shell.Description,
		OwnerCompanyID:   ownerCompanyID,
		CurrencyCode:     shell.CurrencyCode,
		ResponseDeadline: shell.ResponseDeadline,
	}); shellErr != nil {
		preview.Errors = append(preview.Errors, eventShellIssue(shellErr))
		preview.ReadyToCommit = false
	}

	response := toBuyerXlsxCreatePreviewResponse(preview, shell)
	if !preview.ReadyToCommit {
		return response, nil
	}
	if s.importAnalysisRepo == nil || s.txRunner == nil {
		return nil, apperrors.Internal("import analysis persistence is not configured", nil)
	}

	createdAt := s.nowFn().UTC()
	expiresAt := createdAt.Add(importAnalysisTTL)
	payloadJSON, err := xlsxexchange.CanonicalCreatePayloadJSON(shell, preview)
	if err != nil {
		return nil, apperrors.Internal("failed to marshal canonical create payload", err)
	}
	canonicalHash, err := xlsxexchange.StableStoredPayloadHash(payloadJSON)
	if err != nil {
		return nil, apperrors.Internal("failed to compute canonical create payload hash", err)
	}
	summaryJSON, err := json.Marshal(response.NormalizedDraftSummary)
	if err != nil {
		return nil, apperrors.Internal("failed to marshal create preview summary", err)
	}

	analysisInput := domain.ImportAnalysis{
		TenantID:             actor.TenantID,
		ActorID:              actor.UserID,
		ActorCompanyID:       ownerCompanyID,
		WorkbookType:         domain.WorkbookTypeBuyerTender,
		SchemaVersion:        domain.SchemaVersionBuyerXLSXV1,
		TargetType:           domain.ImportTargetTypeNewEvent,
		CanonicalPayloadJSON: payloadJSON,
		CanonicalHash:        canonicalHash,
		ValidationSummary:    summaryJSON,
		CreatedAt:            createdAt,
		ExpiresAt:            expiresAt,
	}

	var persisted *domain.ImportAnalysis
	if err := s.txRunner.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		created, createErr := s.importAnalysisRepo.WithTx(tx).CreatePreview(ctx, analysisInput)
		if createErr != nil {
			return createErr
		}
		persisted = created
		return nil
	}); err != nil {
		return nil, apperrors.Internal("failed to persist create import analysis", err)
	}

	response.AnalysisID = &persisted.ID
	response.ExpiresAt = &persisted.ExpiresAt
	return response, nil
}

// CommitBuyerXlsxCreateAnalysis creates a new EXCEL DRAFT event from a persisted CREATE analysis.
func (s *ExcelExchangeService) CommitBuyerXlsxCreateAnalysis(
	ctx context.Context,
	actor domain.ActorContext,
	in domain.BuyerImportCommitInput,
	idempotencyKey string,
) (*BuyerXlsxCreateCommitResponse, error) {
	if err := actor.Validate(); err != nil {
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
		return nil, apperrors.Internal("buyer xlsx create commit is not configured", nil)
	}
	if s.rfxRepoFull == nil || s.qRepoFull == nil {
		return nil, apperrors.Internal("buyer xlsx create commit repositories are not configured", nil)
	}

	scope := repository.IdempotencyScope{
		TenantID:       actor.TenantID,
		ActorID:        actor.UserID,
		Operation:      domain.BuyerXlsxCreateCommitOperation,
		AggregateScope: uuid.Nil,
	}
	requestBodyHash, err := hashRequestBody(domain.NewBuyerImportCommitIdempotencyPayload(in.AnalysisID))
	if err != nil {
		return nil, err
	}
	if _, err := s.importAnalysisRepo.GetByID(ctx, in.AnalysisID, actor.TenantID); err != nil {
		return nil, err
	}
	if err := s.requireBuyerManageActor(ctx, actor); err != nil {
		return nil, err
	}
	if replay, err := s.loadCreateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); err != nil || replay != nil {
		return replay, err
	}

	var result *BuyerXlsxCreateCommitResponse
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
			var replay BuyerXlsxCreateCommitResponse
			if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
				return apperrors.Internal("failed to decode idempotent create commit replay payload", err)
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
		if analysis.ActorCompanyID == uuid.Nil {
			return apperrors.Forbidden("import analysis actor company binding denied")
		}
		if _, err := s.resolveCreateFromXlsxOwner(ctx, actor, analysis.ActorCompanyID); err != nil {
			var app *apperrors.AppError
			if errors.As(err, &app) && app.Code == apperrors.CodeNotFound {
				return apperrors.Forbidden("import analysis actor company binding denied")
			}
			return err
		}
		if analysis.WorkbookType != domain.WorkbookTypeBuyerTender || analysis.SchemaVersion != domain.SchemaVersionBuyerXLSXV1 {
			return apperrors.Validation("unsupported create analysis schema", map[string]any{
				"machine_code": xlsxexchange.MachineCodeUnsupportedSchema,
			})
		}
		if analysis.TargetType != domain.ImportTargetTypeNewEvent || analysis.TargetID != nil || analysis.TargetVersion != nil {
			return apperrors.Conflict("import analysis target mismatch", map[string]any{
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
		stored, err := xlsxexchange.ParseStoredCreatePayload(analysis.CanonicalPayloadJSON)
		if err != nil {
			return apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
			})
		}
		if stored.OwnerCompanyID != analysis.ActorCompanyID {
			return apperrors.Forbidden("import analysis actor company binding denied")
		}

		eventInput := xlsxexchange.EventShellToCreateInput(actor.TenantID, stored)
		if eventInput.ResponseDeadline != nil && !eventInput.ResponseDeadline.After(now) {
			return apperrors.Unprocessable("stored create proposal failed revalidation", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"field":        "response_deadline",
			})
		}
		if err := domain.ValidateCreateRfxEventInput(eventInput); err != nil {
			return apperrors.Unprocessable("stored create proposal failed revalidation", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"errors":       []xlsxexchange.BuyerImportIssue{eventShellIssue(err)},
			})
		}
		proposal := xlsxexchange.ProposalFromStoredCreatePayload(stored)
		if validationErrors := xlsxexchange.ValidateCreateCommitProposal(ctx, proposal); len(validationErrors) > 0 {
			return apperrors.Unprocessable("stored import proposal failed revalidation", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"errors":       validationErrors,
			})
		}

		event, err := rfxRepo.CreateEventWithChannel(ctx, eventInput, domain.CreationChannelExcel)
		if err != nil {
			return mapCreateEventConflict(err)
		}
		draft, err := qRepo.GetOrCreateDraftVersion(ctx, actor.TenantID, event.ID)
		if err != nil {
			return err
		}

		changes := buyerImportCommitChanges{}
		if err := reconcileImportLots(ctx, rfxRepo, actor.TenantID, event.ID, stored.AsImportPayload(), &changes.Lots); err != nil {
			return err
		}
		if err := reconcileImportQuestionnaire(ctx, qRepo, actor.TenantID, draft.ID, stored.AsImportPayload(), &changes); err != nil {
			return err
		}

		if err := recordAudit(ctx, auditRepo, actor, event.OwnerCompanyID, "rfx_event", event.ID, "rfx.buyer_xlsx_import.created.v1", map[string]any{
			"event_id":         event.ID.String(),
			"analysis_id":      analysis.ID.String(),
			"actor_id":         actor.UserID.String(),
			"owner_company_id": event.OwnerCompanyID.String(),
			"creation_channel": domain.CreationChannelExcel,
			"canonical_hash":   analysis.CanonicalHash,
			"committed_at":     now.Format(time.RFC3339Nano),
			"counts":           changes,
		}); err != nil {
			return err
		}

		if _, err := importRepo.MarkConsumed(ctx, analysis.ID, actor.TenantID, domain.ImportTargetTypeNewEvent, event.ID, now); err != nil {
			return err
		}

		response := &BuyerXlsxCreateCommitResponse{
			EventID:              event.ID,
			AnalysisID:           analysis.ID,
			CreationChannel:      domain.CreationChannelExcel,
			Status:               domain.RfxStatusDraft,
			DraftVersionID:       draft.ID,
			DraftVersionNumber:   draft.VersionNumber,
			QuestionnaireEnabled: draft.QuestionnaireEnabled,
			CreatedCounts:        changes,
			CommittedAt:          now,
		}
		responseBody, err := json.Marshal(response)
		if err != nil {
			return apperrors.Internal("failed to marshal create commit response", err)
		}
		if err := idemRepo.Store(ctx, repository.IdempotencyRecord{
			TenantID:        scope.TenantID,
			ActorID:         scope.ActorID,
			Operation:       scope.Operation,
			AggregateScope:  scope.AggregateScope,
			IdempotencyKey:  idempotencyKey,
			RequestBodyHash: requestBodyHash,
			ResponseStatus:  201,
			ResponseBody:    responseBody,
			ExpiresAt:       now.Add(24 * time.Hour),
		}); err != nil {
			return err
		}
		result = response
		return nil
	})
	if errors.Is(runErr, repository.ErrIdempotencyRecordActive) {
		if replay, replayErr := s.loadCreateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr != nil {
			return nil, replayErr
		} else if replay != nil {
			return replay, nil
		}
		return nil, runErr
	}
	if runErr != nil {
		if replay, replayErr := s.loadCreateCommitReplay(ctx, scope, idempotencyKey, requestBodyHash); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, runErr
	}
	return result, nil
}

func (s *ExcelExchangeService) loadCreateCommitReplay(
	ctx context.Context,
	scope repository.IdempotencyScope,
	idempotencyKey, requestBodyHash string,
) (*BuyerXlsxCreateCommitResponse, error) {
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
	var replay BuyerXlsxCreateCommitResponse
	if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
		return nil, apperrors.Internal("failed to decode idempotent create commit replay payload", err)
	}
	return &replay, nil
}

func (s *ExcelExchangeService) requireBuyerManageActor(ctx context.Context, actor domain.ActorContext) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if err := s.auth.requireBuyerActor(ctx, actor); err != nil {
		return err
	}
	resolver, ok := s.auth.actors.(CompanyMembershipResolver)
	if !ok {
		return apperrors.Forbidden("buyer manage permission is required")
	}
	roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return err
	}
	if !domain.HasBuyerManageRole(roles) {
		return apperrors.Forbidden("buyer manage permission is required")
	}
	return nil
}

func (s *ExcelExchangeService) resolveCreateFromXlsxOwner(ctx context.Context, actor domain.ActorContext, requested uuid.UUID) (uuid.UUID, error) {
	if err := s.requireBuyerManageActor(ctx, actor); err != nil {
		return uuid.Nil, err
	}
	if requested == uuid.Nil {
		return uuid.Nil, apperrors.Validation("owner_company_id is required", map[string]any{"field": "owner_company_id"})
	}
	exists, err := s.auth.repo.CompanyExists(ctx, requested, actor.TenantID)
	if err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, apperrors.NotFound("owner_company_id not found")
	}
	memberships, err := s.auth.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	if !domain.ContainsCompanyID(memberships, requested) {
		return uuid.Nil, apperrors.Forbidden("owner_company_id does not match authenticated membership")
	}
	return requested, nil
}

func toBuyerXlsxCreatePreviewResponse(preview xlsxexchange.BuyerCreatePreview, shell xlsxexchange.BuyerCreateEventShell) *BuyerXlsxCreatePreviewResponse {
	errors := preview.Errors
	if errors == nil {
		errors = []xlsxexchange.BuyerImportIssue{}
	}
	warnings := preview.Warnings
	if warnings == nil {
		warnings = []xlsxexchange.BuyerImportIssue{}
	}
	return &BuyerXlsxCreatePreviewResponse{
		Mode:           xlsxexchange.BuyerImportModeCreateNewDraft,
		SchemaName:     preview.SchemaName,
		SchemaVersion:  preview.SchemaVersion,
		OwnerCompanyID: shell.OwnerCompanyID,
		ReadyToCommit:  preview.ReadyToCommit && len(errors) == 0,
		NormalizedDraftSummary: BuyerXlsxCreateDraftSummary{
			RfxNumber:        shell.RfxNumber,
			Title:            shell.Title,
			RfxType:          shell.RfxType,
			Category:         shell.Category,
			Description:      shell.Description,
			ResponseDeadline: shell.ResponseDeadline,
			CurrencyCode:     shell.CurrencyCode,
			LotCount:         preview.ChangeCounts.Lots.Added,
			SectionCount:     preview.ChangeCounts.Sections.Added,
			QuestionCount:    preview.ChangeCounts.Questions.Added,
		},
		ChangeCounts: preview.ChangeCounts,
		Errors:       errors,
		Warnings:     warnings,
	}
}

func eventShellIssue(err error) xlsxexchange.BuyerImportIssue {
	field := "event_shell"
	var app *apperrors.AppError
	if errors.As(err, &app) {
		if raw, ok := app.Details["field"].(string); ok && strings.TrimSpace(raw) != "" {
			field = raw
		}
	}
	return xlsxexchange.BuyerImportIssue{
		Severity:    xlsxexchange.IssueSeverityError,
		MachineCode: xlsxexchange.MachineCodeInvalidEventShell,
		Column:      field,
		MessageKey:  "rfx.buyer_xlsx_create.invalid_event_shell",
		Params:      map[string]any{"reason": err.Error()},
	}
}

func mapCreateEventConflict(err error) error {
	var app *apperrors.AppError
	if errors.As(err, &app) && app.Code == apperrors.CodeConflict {
		return apperrors.Conflict("rfx_number already exists", map[string]any{
			"field":        "rfx_number",
			"machine_code": "duplicate_rfx_number",
		})
	}
	return err
}

func StructuralCreatePreviewAppError(first xlsxexchange.BuyerImportIssue) *apperrors.AppError {
	details := map[string]any{
		"machine_code": first.MachineCode,
		"message_key":  first.MessageKey,
	}
	if first.Sheet != "" {
		details["sheet"] = first.Sheet
	}
	if first.Row > 0 {
		details["row"] = first.Row
	}
	if first.Column != "" {
		details["column"] = first.Column
	}
	return apperrors.Validation("buyer xlsx create preview failed", details)
}

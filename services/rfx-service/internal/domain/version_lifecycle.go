package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	VersionLifecycleMachineCodeChangeImpactRequired = "CHANGE_IMPACT_ANALYSIS_REQUIRED"

	VersionLifecycleOperationPublish      = "PUBLISH_EVENT_VERSION"
	VersionLifecycleOperationForkDraft    = "FORK_EVENT_DRAFT_VERSION"
	VersionLifecycleOperationRestoreDraft = "RESTORE_EVENT_DRAFT_VERSION"
)

type PublishQuestionnaireInput struct {
	ExpectedEventVersion int    `json:"expected_event_version"`
	ExpectedDraftVersion int    `json:"expected_draft_version"`
	ChangeSummary        string `json:"change_summary"`
}

type VersionDetail struct {
	Version       RfxVersion
	Questionnaire QuestionnaireDefinition
}

func ValidatePublishQuestionnaireInput(in PublishQuestionnaireInput) error {
	if in.ExpectedEventVersion <= 0 {
		return apperrors.Validation("expected_event_version must be positive", map[string]any{"field": "expected_event_version"})
	}
	if in.ExpectedDraftVersion <= 0 {
		return apperrors.Validation("expected_draft_version must be positive", map[string]any{"field": "expected_draft_version"})
	}
	if strings.TrimSpace(in.ChangeSummary) == "" {
		return apperrors.Validation("change_summary is required", map[string]any{"field": "change_summary"})
	}
	return nil
}

func ValidateVersionPublishStatus(status string) error {
	if status != RfxVersionStatusDraft {
		return apperrors.Conflict("questionnaire version is not publishable", map[string]any{"field": "status", "status": status})
	}
	return nil
}

type RestoreVersionAsDraftInput struct {
	ChangeSummary string `json:"change_summary"`
}

// RestoreVersionAsDraftIdempotencyPayload is the canonical idempotency identity for restore.
type RestoreVersionAsDraftIdempotencyPayload struct {
	SourceVersionID uuid.UUID `json:"source_version_id"`
	ChangeSummary   string    `json:"change_summary"`
}

func NewRestoreVersionAsDraftIdempotencyPayload(sourceVersionID uuid.UUID, in RestoreVersionAsDraftInput) RestoreVersionAsDraftIdempotencyPayload {
	return RestoreVersionAsDraftIdempotencyPayload{
		SourceVersionID: sourceVersionID,
		ChangeSummary:   strings.TrimSpace(in.ChangeSummary),
	}
}

func ValidateRestoreVersionAsDraftInput(in RestoreVersionAsDraftInput) error {
	if strings.TrimSpace(in.ChangeSummary) == "" {
		return apperrors.Validation("change_summary is required", map[string]any{"field": "change_summary"})
	}
	return nil
}

func ValidateVersionForkSourceStatus(status string) error {
	if status != RfxVersionStatusPublished {
		return apperrors.Conflict("only the published questionnaire version can be forked", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateVersionRestoreSourceStatus(status string) error {
	switch status {
	case RfxVersionStatusPublished, RfxVersionStatusSuperseded:
		return nil
	default:
		return apperrors.Conflict("only published or superseded questionnaire versions can be restored as draft", map[string]any{"field": "status", "status": status})
	}
}

func ChangeImpactAnalysisRequired(responseCount, scoredResponseCount int) error {
	return apperrors.Validation("change impact analysis is required before republishing this questionnaire", map[string]any{
		"code":                  VersionLifecycleMachineCodeChangeImpactRequired,
		"response_count":        responseCount,
		"scored_response_count": scoredResponseCount,
	})
}

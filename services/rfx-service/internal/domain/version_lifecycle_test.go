package domain

import (
	"errors"
	"testing"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestValidatePublishQuestionnaireInput(t *testing.T) {
	t.Parallel()

	valid := PublishQuestionnaireInput{
		ExpectedEventVersion: 2,
		ExpectedDraftVersion: 3,
		ChangeSummary:        "Updated qualification notes",
	}
	if err := ValidatePublishQuestionnaireInput(valid); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	cases := []struct {
		name      string
		input     PublishQuestionnaireInput
		wantField string
	}{
		{
			name: "missing event version",
			input: PublishQuestionnaireInput{
				ExpectedDraftVersion: 1,
				ChangeSummary:        "Initial publish",
			},
			wantField: "expected_event_version",
		},
		{
			name: "missing draft version",
			input: PublishQuestionnaireInput{
				ExpectedEventVersion: 1,
				ChangeSummary:        "Initial publish",
			},
			wantField: "expected_draft_version",
		},
		{
			name: "blank change summary",
			input: PublishQuestionnaireInput{
				ExpectedEventVersion: 1,
				ExpectedDraftVersion: 1,
				ChangeSummary:        "   ",
			},
			wantField: "change_summary",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePublishQuestionnaireInput(tc.input)
			var appErr *apperrors.AppError
			if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
				t.Fatalf("expected validation error, got %v", err)
			}
			if got := appErr.Details["field"]; got != tc.wantField {
				t.Fatalf("field=%v want=%s", got, tc.wantField)
			}
		})
	}
}

func TestValidateVersionPublishStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateVersionPublishStatus(RfxVersionStatusDraft); err != nil {
		t.Fatalf("draft should be publishable: %v", err)
	}
	if err := ValidateVersionPublishStatus(RfxVersionStatusPublished); err == nil {
		t.Fatal("expected published status to be rejected")
	}
}

func TestValidateVersionForkSourceStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateVersionForkSourceStatus(RfxVersionStatusPublished); err != nil {
		t.Fatalf("published should be forkable: %v", err)
	}
	if err := ValidateVersionForkSourceStatus(RfxVersionStatusDraft); err == nil {
		t.Fatal("expected draft status to be rejected")
	}
}

func TestChangeImpactAnalysisRequired(t *testing.T) {
	t.Parallel()

	err := ChangeImpactAnalysisRequired(4, 2)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected app error, got %v", err)
	}
	if appErr.Code != apperrors.CodeValidation {
		t.Fatalf("code=%s", appErr.Code)
	}
	if got := appErr.Details["code"]; got != VersionLifecycleMachineCodeChangeImpactRequired {
		t.Fatalf("details.code=%v", got)
	}
	if got := appErr.Details["response_count"]; got != 4 {
		t.Fatalf("response_count=%v", got)
	}
	if got := appErr.Details["scored_response_count"]; got != 2 {
		t.Fatalf("scored_response_count=%v", got)
	}
}

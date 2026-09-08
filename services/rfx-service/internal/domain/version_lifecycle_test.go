package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

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

func TestRestoreVersionAsDraftIdempotencyPayloadIncludesSourceVersionID(t *testing.T) {
	t.Parallel()

	sourceA := uuid.New()
	sourceB := uuid.New()
	in := RestoreVersionAsDraftInput{ChangeSummary: "Same summary"}

	payloadA := NewRestoreVersionAsDraftIdempotencyPayload(sourceA, in)
	payloadB := NewRestoreVersionAsDraftIdempotencyPayload(sourceB, in)
	if payloadA.SourceVersionID != sourceA || payloadA.ChangeSummary != "Same summary" {
		t.Fatalf("unexpected payload A: %+v", payloadA)
	}
	hashA, err := hashTestPayload(payloadA)
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	hashB, err := hashTestPayload(payloadB)
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hashA == hashB {
		t.Fatal("expected different hashes for different source_version_id")
	}

	payloadSameSource := NewRestoreVersionAsDraftIdempotencyPayload(sourceA, RestoreVersionAsDraftInput{ChangeSummary: "Different summary"})
	hashSameSource, err := hashTestPayload(payloadSameSource)
	if err != nil {
		t.Fatalf("hash same source: %v", err)
	}
	if hashA == hashSameSource {
		t.Fatal("expected different hashes for different change_summary")
	}
}

func hashTestPayload(v any) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
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

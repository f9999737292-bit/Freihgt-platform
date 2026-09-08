package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestCompareVersionSnapshotsAddedSection(t *testing.T) {
	source := baseCompareSnapshot()
	target := baseCompareSnapshot()
	target.Questionnaire.Sections = append(target.Questionnaire.Sections, SectionWithQuestions{
		Section: Section{SectionCode: "HSE", Title: "Health", SortOrder: 2},
	})

	result := CompareVersionSnapshots(source, target)
	if result.Summary.AddedCount < 1 {
		t.Fatalf("expected added diff, summary=%+v", result.Summary)
	}
	found := false
	for _, item := range result.Differences {
		if item.EntityType == "section" && item.SectionCode == "HSE" && item.Change == VersionDiffAdded {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing added section diff: %+v", result.Differences)
	}
}

func TestCompareVersionSnapshotsChangedField(t *testing.T) {
	source := baseCompareSnapshot()
	target := baseCompareSnapshot()
	target.Questionnaire.Sections[0].Section.Title = "Updated General"

	result := CompareVersionSnapshots(source, target)
	found := false
	for _, item := range result.Differences {
		if item.EntityType == "section" && item.SectionCode == "GENERAL" && item.Change == VersionDiffChanged {
			found = true
			if len(item.Fields) == 0 || item.Fields[0] != "title" {
				t.Fatalf("expected title field change, got %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("missing changed section diff")
	}
}

func TestCompareVersionSnapshotsReorderedSection(t *testing.T) {
	source := baseCompareSnapshot()
	target := baseCompareSnapshot()
	target.Questionnaire.Sections[0].Section.SortOrder = 5

	result := CompareVersionSnapshots(source, target)
	found := false
	for _, item := range result.Differences {
		if item.EntityType == "section" && item.SectionCode == "GENERAL" && item.Change == VersionDiffReordered {
			found = true
		}
	}
	if !found {
		t.Fatal("missing reordered section diff")
	}
}

func TestCompareVersionSnapshotsStableCanonicalHash(t *testing.T) {
	left := baseCompareSnapshot()
	right := baseCompareSnapshot()
	first := CompareVersionSnapshots(left, right)
	second := CompareVersionSnapshots(left, right)
	if first.CanonicalDiffHash == "" {
		t.Fatal("expected non-empty hash")
	}
	if first.CanonicalDiffHash != second.CanonicalDiffHash {
		t.Fatalf("hash not stable: %s vs %s", first.CanonicalDiffHash, second.CanonicalDiffHash)
	}
}

func TestCompareVersionSnapshotsReverseDirection(t *testing.T) {
	source := baseCompareSnapshot()
	target := baseCompareSnapshot()
	target.Questionnaire.Sections = append(target.Questionnaire.Sections, SectionWithQuestions{
		Section: Section{SectionCode: "HSE", Title: "Health", SortOrder: 2},
	})

	forward := CompareVersionSnapshots(source, target)
	reverse := CompareVersionSnapshots(target, source)
	if forward.Summary.AddedCount != reverse.Summary.RemovedCount {
		t.Fatalf("forward added=%d reverse removed=%d", forward.Summary.AddedCount, reverse.Summary.RemovedCount)
	}
}

func TestValidateCompareVersionsInputRejectsSameID(t *testing.T) {
	id := uuid.New()
	err := ValidateCompareVersionsInput(CompareVersionsInput{
		SourceVersionID: id,
		TargetVersionID: id,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func baseCompareSnapshot() VersionCompareSnapshot {
	eventID := uuid.New()
	versionID := uuid.New()
	return VersionCompareSnapshot{
		Version: RfxVersion{
			ID:                   versionID,
			RfxEventID:           eventID,
			VersionNumber:        1,
			Status:               RfxVersionStatusPublished,
			QuestionnaireEnabled: true,
		},
		Questionnaire: QuestionnaireDefinition{
			EventID:              eventID,
			RfxVersionID:         versionID,
			VersionNumber:        1,
			QuestionnaireEnabled: true,
			VersionStatus:        RfxVersionStatusPublished,
			Sections: []SectionWithQuestions{{
				Section: Section{SectionCode: "GENERAL", Title: "General", SortOrder: 1},
				Questions: []Question{{
					QuestionCode:       "FLEET_SIZE",
					QuestionType:       QuestionTypeText,
					Label:              "Fleet size",
					Required:           true,
					ValidationRuleJSON: mustCompareJSON(`{"min_length":1}`),
					SortOrder:          1,
				}},
			}},
		},
	}
}

func mustCompareJSON(raw string) json.RawMessage {
	return json.RawMessage(raw)
}

package domain

import "testing"

func TestClassifyChangeImpactLabelOnlyNonMaterial(t *testing.T) {
	diff := CompareVersionsResult{
		Differences: []CompareItemDiff{{
			EntityType:   "question",
			Change:       VersionDiffChanged,
			QuestionCode: "FLEET_SIZE",
			Fields:       []string{"label"},
		}},
	}
	classification := ClassifyChangeImpact(diff, 0, 0)
	if len(classification.ImpactClasses) != 1 || classification.ImpactClasses[0] != ChangeImpactClassNonMaterial {
		t.Fatalf("classes=%v", classification.ImpactClasses)
	}
}

func TestClassifyChangeImpactStructuralNoResponses(t *testing.T) {
	diff := CompareVersionsResult{
		Differences: []CompareItemDiff{{
			EntityType:   "question",
			Change:       VersionDiffAdded,
			QuestionCode: "NEW_Q",
		}},
	}
	classification := ClassifyChangeImpact(diff, 0, 0)
	assertContainsClass(t, classification.ImpactClasses, ChangeImpactClassMaterialNoResponses)
}

func TestClassifyChangeImpactScoringAndKnockout(t *testing.T) {
	diff := CompareVersionsResult{
		Differences: []CompareItemDiff{
			{
				EntityType:    "score_criterion",
				Change:        VersionDiffChanged,
				CriterionCode: "HSE",
				Fields:        []string{"weight"},
			},
			{
				EntityType:    "score_binding",
				Change:        VersionDiffChanged,
				CriterionCode: "HSE",
				QuestionCode:  "HSE_OK",
				Fields:        []string{"knockout_rule_json"},
			},
		},
	}
	classification := ClassifyChangeImpact(diff, 0, 0)
	assertContainsClass(t, classification.ImpactClasses, ChangeImpactClassScoringAffecting)
	assertContainsClass(t, classification.ImpactClasses, ChangeImpactClassKnockoutAffecting)
}

func TestSortImpactClassesDeterministic(t *testing.T) {
	sorted := SortImpactClasses([]string{
		ChangeImpactClassKnockoutAffecting,
		ChangeImpactClassNonMaterial,
		ChangeImpactClassScoringAffecting,
	})
	want := []string{
		ChangeImpactClassNonMaterial,
		ChangeImpactClassScoringAffecting,
		ChangeImpactClassKnockoutAffecting,
	}
	for i := range want {
		if sorted[i] != want[i] {
			t.Fatalf("sorted=%v want=%v", sorted, want)
		}
	}
}

func TestCanonicalDiffHashStableAcrossItemOrder(t *testing.T) {
	itemsA := []CompareItemDiff{
		{EntityType: "question", Change: VersionDiffChanged, QuestionCode: "A", Fields: []string{"label"}},
		{EntityType: "question", Change: VersionDiffChanged, QuestionCode: "B", Fields: []string{"label"}},
	}
	itemsB := []CompareItemDiff{itemsA[1], itemsA[0]}
	if canonicalDiffHash(itemsA) != canonicalDiffHash(itemsB) {
		t.Fatal("hash must be order-independent")
	}
}

func TestComputeRescoringRequired(t *testing.T) {
	if !ComputeRescoringRequired([]string{ChangeImpactClassScoringAffecting}) {
		t.Fatal("expected true for scoring")
	}
	if ComputeRescoringRequired([]string{ChangeImpactClassNonMaterial}) {
		t.Fatal("expected false for non-material")
	}
}

func assertContainsClass(t *testing.T, classes []string, want string) {
	t.Helper()
	for _, class := range classes {
		if class == want {
			return
		}
	}
	t.Fatalf("missing class %s in %v", want, classes)
}

package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestComputeCommercialScoresComparable(t *testing.T) {
	candidates := []EvaluationCandidate{
		{ResponseID: uuid.New(), TotalAmount: 100, CurrencyCode: "RUB"},
		{ResponseID: uuid.New(), TotalAmount: 125, CurrencyCode: "RUB"},
	}
	scored := ComputeCommercialScores(candidates)
	if !scored[0].Comparable || !scored[1].Comparable {
		t.Fatal("expected comparable")
	}
	if scored[0].CommercialScore != 100 {
		t.Fatalf("expected 100 commercial for lowest, got %v", scored[0].CommercialScore)
	}
	if scored[1].CommercialScore != 80 {
		t.Fatalf("expected 80 commercial, got %v", scored[1].CommercialScore)
	}
}

func TestComputeCommercialScoresMixedCurrencyNotComparable(t *testing.T) {
	candidates := []EvaluationCandidate{
		{TotalAmount: 100, CurrencyCode: "RUB"},
		{TotalAmount: 100, CurrencyCode: "USD"},
	}
	scored := ComputeCommercialScores(candidates)
	if scored[0].Comparable || scored[1].Comparable {
		t.Fatal("expected not comparable")
	}
}

func TestRankEvaluationCandidates(t *testing.T) {
	candidates := RankEvaluationCandidates([]EvaluationCandidate{
		{TotalScore: 90, TotalAmount: 100},
		{TotalScore: 80, TotalAmount: 90},
		{TotalScore: 80, TotalAmount: 95},
	})
	if candidates[0].Rank != 1 || candidates[1].Rank != 2 || candidates[2].Rank != 2 {
		t.Fatalf("unexpected ranks: %+v", candidates)
	}
}

func TestValidateOfferLineInputRejectsNegative(t *testing.T) {
	if err := ValidateOfferLineInput(UpsertOfferLineInput{RfxLotID: uuid.New(), Amount: -1, CurrencyCode: "RUB"}, 1); err == nil {
		t.Fatal("expected validation error")
	}
}

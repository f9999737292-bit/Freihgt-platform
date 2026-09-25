package service

import (
	"context"
	"strings"
	"testing"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestBNO317_INVALID_PROFILE_WEIGHT_SUM_REJECTED(t *testing.T) {
	w := newWorld(t)
	profile, err := w.store.ActiveScoreProfile(context.Background(), domain.ProfileBalanced)
	if err != nil {
		t.Fatal(err)
	}
	profile.Components[0].WeightBps = 3400
	w.store.ReplaceScoreProfile(profile)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	policy := radiusPolicy(100, 0)
	policy.ObjectiveProfile = domain.ProfileBalanced
	_, err = w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if reasonOf(err) != "SCORE_PROFILE_INVALID" {
		t.Fatalf("%v", err)
	}
	if w.store.SearchRunCount() != 0 {
		t.Fatal("invalid profile was scored and persisted")
	}
}

func TestBNO318_UNSUPPORTED_ALGORITHM_VERSION_REJECTED(t *testing.T) {
	w := newWorld(t)
	profile, err := w.store.ActiveScoreProfile(context.Background(), domain.ProfileMinDeadhead)
	if err != nil {
		t.Fatal(err)
	}
	profile.AlgorithmVersion = "bno-score-unsupported"
	w.store.ReplaceScoreProfile(profile)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	_, err = w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(100, 0)})
	if reasonOf(err) != "SCORING_ALGORITHM_UNSUPPORTED" {
		t.Fatalf("%v", err)
	}
	if strings.Contains(err.Error(), "bno-score-unsupported") {
		t.Fatalf("response claims the unsupported version was executed: %v", err)
	}
	if w.store.SearchRunCount() != 0 {
		t.Fatal("unsupported algorithm was persisted")
	}
}

func TestMissingScoreProfileStoreFailsClosed(t *testing.T) {
	w := newWorld(t)
	bare := New(w.store, nil)
	bare.UseRouting(w.routes)
	bare.UsePolicies(w.store)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	_, err := bare.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(100, 0)})
	if reasonOf(err) != "SCORE_PROFILE_STORE_UNAVAILABLE" {
		t.Fatalf("%v", err)
	}
	if w.store.SearchRunCount() != 0 {
		t.Fatal("missing store fell back to a built-in profile")
	}
}

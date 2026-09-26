package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/profilefixture"
)

func TestBNO321_MEMORY_PROFILE_STORE_EXPLICIT_SEED(t *testing.T) {
	store := NewMemory()
	if _, err := store.ActiveScoreProfile(context.Background(), domain.ProfileMinDeadhead); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fresh memory store returned a profile: %v", err)
	}
	store.SeedScoreProfiles(profilefixture.V1())
	profile, err := store.ActiveScoreProfile(context.Background(), domain.ProfileMinDeadhead)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.Components) != 1 || profile.Components[0].Code != domain.ComponentDeadheadEfficiency || profile.Components[0].WeightBps != 10000 {
		t.Fatalf("seeded profile %+v", profile)
	}
	loadID := uuid.New()
	scored := domain.ScorePool(profile, []domain.ScoreFacts{{LoadID: loadID, DeadheadKm: floatPointer(12)}}, "")
	if scored[loadID].Total == nil || *scored[loadID].Total != 5000 {
		t.Fatalf("stored profile was not scored %+v", scored[loadID])
	}
	raw, err := os.ReadFile("../domain/score.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Contains(source, "SystemScoreProfiles") || strings.Contains(source, "3500") {
		t.Fatal("scoring domain still contains the production profile weight table")
	}
}

func floatPointer(value float64) *float64 { return &value }

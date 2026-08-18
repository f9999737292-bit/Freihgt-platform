//go:build integration

package laneownership

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestCreateLaneOwnLotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedLaneFixture(t, env)
	ctx := context.Background()

	before, err := countLanes(ctx, env.pool, fix.LotA)
	if err != nil {
		t.Fatalf("count lanes: %v", err)
	}
	_, err = env.rfxSvc.CreateLane(ctx, fix.BuyerA, fix.LotA, laneInput(fix.TenantA, fix.LotA, fix.OriginID, fix.DestID))
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	after, err := countLanes(ctx, env.pool, fix.LotA)
	if err != nil {
		t.Fatalf("count lanes after: %v", err)
	}
	if after != before+1 {
		t.Fatalf("expected one new lane, before=%d after=%d", before, after)
	}
}

func TestCreateLaneCrossCompanyDeniedSameTenant(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedLaneFixture(t, env)
	ctx := context.Background()

	before, err := countLanes(ctx, env.pool, fix.LotB)
	if err != nil {
		t.Fatalf("count lanes: %v", err)
	}
	_, err = env.rfxSvc.CreateLane(ctx, fix.BuyerA, fix.LotB, laneInput(fix.TenantA, fix.LotB, fix.OriginID, fix.DestID))
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
	after, err := countLanes(ctx, env.pool, fix.LotB)
	if err != nil {
		t.Fatalf("count lanes after: %v", err)
	}
	if after != before {
		t.Fatalf("unauthorized lane created: before=%d after=%d", before, after)
	}
}

func TestCreateLaneCrossTenantDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedLaneFixture(t, env)
	ctx := context.Background()

	before, err := countLanes(ctx, env.pool, fix.LotC)
	if err != nil {
		t.Fatalf("count lanes: %v", err)
	}
	_, err = env.rfxSvc.CreateLane(ctx, fix.BuyerA, fix.LotC, laneInput(fix.TenantA, fix.LotC, fix.OriginID, fix.DestID))
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
	after, err := countLanes(ctx, env.pool, fix.LotC)
	if err != nil {
		t.Fatalf("count lanes after: %v", err)
	}
	if after != before {
		t.Fatalf("cross-tenant lane created: before=%d after=%d", before, after)
	}
}

func TestCreateLaneBuyerCOwnLotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedLaneFixture(t, env)
	ctx := context.Background()

	before, err := countLanes(ctx, env.pool, fix.LotC)
	if err != nil {
		t.Fatalf("count lanes: %v", err)
	}
	_, err = env.rfxSvc.CreateLane(ctx, fix.BuyerC, fix.LotC, laneInput(fix.TenantB, fix.LotC, fix.OriginID, fix.DestID))
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	after, err := countLanes(ctx, env.pool, fix.LotC)
	if err != nil {
		t.Fatalf("count lanes after: %v", err)
	}
	if after != before+1 {
		t.Fatalf("expected one new lane, before=%d after=%d", before, after)
	}
}

func TestCreateLaneUnknownLotNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedLaneFixture(t, env)
	ctx := context.Background()

	unknownLot := uuid.New()
	_, err := env.rfxSvc.CreateLane(ctx, fix.BuyerA, unknownLot, laneInput(fix.TenantA, unknownLot, fix.OriginID, fix.DestID))
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCreateLaneIDORAttackerWithKnownForeignLotUUID(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedLaneFixture(t, env)
	ctx := context.Background()

	cases := []struct {
		name  string
		actor domain.ActorContext
		lotID uuid.UUID
	}{
		{"buyer_a_foreign_company_lot", fix.BuyerA, fix.LotB},
		{"buyer_a_cross_tenant_lot", fix.BuyerA, fix.LotC},
		{"buyer_b_foreign_company_lot", fix.BuyerB, fix.LotA},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, err := countLanes(ctx, env.pool, tc.lotID)
			if err != nil {
				t.Fatalf("count lanes: %v", err)
			}
			_, err = env.rfxSvc.CreateLane(ctx, tc.actor, tc.lotID, laneInput(tc.actor.TenantID, tc.lotID, fix.OriginID, fix.DestID))
			assertAppErrorCode(t, err, apperrors.CodeNotFound)
			after, err := countLanes(ctx, env.pool, tc.lotID)
			if err != nil {
				t.Fatalf("count lanes after: %v", err)
			}
			if after != before {
				t.Fatalf("IDOR created lane on lot %s", tc.lotID)
			}
		})
	}
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected code %s, got %v", code, err)
	}
}

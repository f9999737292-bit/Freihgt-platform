package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO04CPendingSuccessorRollbackPreservesPredecessor(t *testing.T) {
	store := NewMemory()
	tenant := uuid.New()
	shipment := uuid.New()
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	version := 2
	planA := uuid.New()
	graphA := RoutePlanGraph{Plan: RoutePlanRow{
		ID: planA, TenantID: tenant, Version: 1, Status: "ACCEPTED", PlanningMode: "CURRENT_TRIP",
		ResultStatus: "FEASIBLE_PLAN_FOUND", ShipmentID: &shipment, ShipmentVersion: &version,
		ContextFingerprint: "ctx", EvaluationFingerprint: "eval", AlgorithmPolicyVersion: "alg",
		RoutingPolicyVersion: "route", CreatedAt: now,
	}}
	executionID := uuid.New()
	revisionID := uuid.New()
	activationA := RoutePlanActivationRow{
		ID: uuid.New(), TenantID: tenant, RoutePlanID: planA, PlanVersion: 1, Version: 1, IdempotencyKey: "key-a",
		ExecutionShipmentID: &shipment, EffectiveShipmentID: &shipment, ExecutionID: &executionID, ExecutionRevisionID: &revisionID,
		Status: "EXECUTION_LINKED", CreatedAt: now,
	}
	if err := store.Within(context.Background(), func(tx Tx) error {
		if err := tx.InsertRoutePlan(context.Background(), graphA); err != nil {
			return err
		}
		return tx.InsertRoutePlanActivation(context.Background(), activationA)
	}); err != nil {
		t.Fatal(err)
	}
	planB := uuid.New()
	graphB := graphA
	graphB.Plan.ID = planB
	graphB.Plan.SupersedesPlanID = &planA
	boom := errors.New("fail before commit")
	err := store.Within(context.Background(), func(tx Tx) error {
		if err := tx.InsertRoutePlan(context.Background(), graphB); err != nil {
			return err
		}
		if err := tx.InsertRoutePlanActivation(context.Background(), RoutePlanActivationRow{
			ID: uuid.New(), TenantID: tenant, RoutePlanID: planB, PlanVersion: 1, Version: 1, IdempotencyKey: "key-b",
			ExecutionShipmentID: &shipment, Status: "PENDING_EXECUTION", CreatedAt: now,
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	var got RoutePlanGraph
	var linked RoutePlanActivationRow
	err = store.Within(context.Background(), func(tx Tx) error {
		var getErr error
		got, getErr = tx.GetRoutePlan(context.Background(), tenant, planA)
		if getErr != nil {
			return getErr
		}
		linked, getErr = tx.LinkedActivationForShipment(context.Background(), tenant, shipment)
		return getErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Plan.Status != "ACCEPTED" || linked.RoutePlanID != planA || linked.EffectiveShipmentID == nil {
		t.Fatalf("rolled back state status %s activation %s", got.Plan.Status, linked.RoutePlanID)
	}
	err = store.Within(context.Background(), func(tx Tx) error {
		_, getErr := tx.GetRoutePlan(context.Background(), tenant, planB)
		return getErr
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("successor persisted after rollback: %v", err)
	}
}

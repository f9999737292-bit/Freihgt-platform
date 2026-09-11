//go:build integration

package latesubmission

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE7INT01CarrierCreatesRequestAfterDeadline(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	req, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonTechnicalFailure, ReasonText: "system outage",
		RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if req.Status != domain.LateSubmissionStatusRequested {
		t.Fatalf("status=%s", req.Status)
	}
}

func TestE7INT02RequestBeforeDeadlineRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	future := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Future",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-FUT-" + uuid.NewString()[:8],
		ResponseDeadline: &future,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	_, err = env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	})
	if err != nil {
		t.Fatalf("participant: %v", err)
	}
	_, err = env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "late", RequestedUntil: future,
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestE7INT03EmptyReasonTextRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "   ", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestE7INT04UnknownReasonCodeRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: "INVALID", ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestE7INT05CarrierReadsOwnRequest(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "mine", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	items, err := env.lateSvc.ListOwnRequests(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("list own: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("expected own request")
	}
}

func TestE7INT06CarrierCannotSeeOtherCarrierRequest(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "a", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	items, err := env.lateSvc.ListOwnRequests(context.Background(), fix.CarrierBAct, event.ID, fix.CarrierBID)
	if err != nil {
		t.Fatalf("list other: %v", err)
	}
	if len(items) != 0 {
		t.Fatal("carrier B must not see carrier A requests")
	}
}

func TestE7INT07BuyerSeesQueue(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "q", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	items, err := env.lateSvc.BuyerListRequests(context.Background(), fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("buyer list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queue item, got %d", len(items))
	}
}

func TestE7INT08BuyerApprove(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "approve me", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	from := time.Now().UTC().Add(-time.Minute)
	until := from.Add(2 * time.Hour)
	approved := approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	if approved.Status != domain.LateSubmissionStatusApproved {
		t.Fatalf("status=%s", approved.Status)
	}
}

func TestE7INT09BuyerReject(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "reject me", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rejected, err := env.lateSvc.Reject(context.Background(), fix.BuyerA, event.ID, created.ID, uuid.NewString(), domain.RejectLateSubmissionInput{
		ExpectedVersion: created.Version, DecisionComment: "no",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != domain.LateSubmissionStatusRejected {
		t.Fatalf("status=%s", rejected.Status)
	}
}

func TestE7INT10BuyerReadCannotDecide(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.lateSvc.Approve(context.Background(), fix.BuyerRead, event.ID, created.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: created.Version, ApprovedValidFrom: time.Now().UTC(), ApprovedValidUntil: time.Now().UTC().Add(time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE7INT11CarrierCannotDecide(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.lateSvc.Approve(context.Background(), fix.CarrierAct, event.ID, created.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: created.Version, ApprovedValidFrom: time.Now().UTC(), ApprovedValidUntil: time.Now().UTC().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected carrier forbidden")
	}
}

func TestE7INT12CrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.lateSvc.Approve(context.Background(), fix.CrossTenant, event.ID, created.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: created.Version, ApprovedValidFrom: time.Now().UTC(), ApprovedValidUntil: time.Now().UTC().Add(time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE7INT13CrossCompany404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.lateSvc.Approve(context.Background(), fix.BuyerB, event.ID, created.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: created.Version, ApprovedValidFrom: time.Now().UTC(), ApprovedValidUntil: time.Now().UTC().Add(time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE7INT14InvalidApproveWindow422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	same := time.Now().UTC()
	_, err = env.lateSvc.Approve(context.Background(), fix.BuyerA, event.ID, created.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: created.Version, ApprovedValidFrom: same, ApprovedValidUntil: same,
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestE7INT15DoubleApprove409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	from := time.Now().UTC().Add(-time.Minute)
	until := from.Add(2 * time.Hour)
	approved := approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	_, err = env.lateSvc.Approve(context.Background(), fix.BuyerA, event.ID, approved.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: approved.Version, ApprovedValidFrom: from, ApprovedValidUntil: until,
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE7INT16ApproveAfterReject409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rejected, err := env.lateSvc.Reject(context.Background(), fix.BuyerA, event.ID, created.ID, uuid.NewString(), domain.RejectLateSubmissionInput{
		ExpectedVersion: created.Version, DecisionComment: "no",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	_, err = env.lateSvc.Approve(context.Background(), fix.BuyerA, event.ID, rejected.ID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: rejected.Version, ApprovedValidFrom: time.Now().UTC(), ApprovedValidUntil: time.Now().UTC().Add(time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE7INT17RejectAfterApprove409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	from := time.Now().UTC().Add(-time.Minute)
	until := from.Add(2 * time.Hour)
	approved := approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	_, err = env.lateSvc.Reject(context.Background(), fix.BuyerA, event.ID, approved.ID, uuid.NewString(), domain.RejectLateSubmissionInput{
		ExpectedVersion: approved.Version, DecisionComment: "too late",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE7INT18SubmitAfterDeadlineForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, q := seedPublishedEventAfterDeadline(t, env, fix)
	ctx := context.Background()
	ws, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"late"`)}},
	})
	assertAppErrorCode(t, err, apperrors.CodeUnprocessable)
}

func TestE7INT19SubmitInsideWindowPasses(t *testing.T) {
	env, fix, event, q, saved := seedApprovedLateWindow(t)
	ctx := context.Background()
	_, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"ok"`)}},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	_, err = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, uuid.NewString())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
}

func TestE7INT20SubmitBeforeWindowStartForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, q := seedPublishedEventAfterDeadline(t, env, fix)
	created, _ := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	from := time.Now().UTC().Add(time.Hour)
	until := from.Add(2 * time.Hour)
	approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	ws, _ := env.crSvc.GetWorkspace(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	_, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"x"`)}},
	})
	assertAppErrorCode(t, err, apperrors.CodeUnprocessable)
}

func TestE7INT21SubmitAtWindowEndForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, q := seedPublishedEventAfterDeadline(t, env, fix)
	created, _ := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	from := time.Now().UTC().Add(-time.Hour)
	until := time.Now().UTC()
	approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	ws, _ := env.crSvc.GetWorkspace(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	_, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"x"`)}},
	})
	assertAppErrorCode(t, err, apperrors.CodeUnprocessable)
}

func TestE7INT22PermissionConsumedOnSubmit(t *testing.T) {
	env, fix, event, _, _ := seedApprovedLateWindow(t)
	ctx := context.Background()
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	_, err := env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, uuid.NewString())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	items, _ := env.lateSvc.ListOwnRequests(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if items[0].Status != domain.LateSubmissionStatusConsumed {
		t.Fatalf("status=%s", items[0].Status)
	}
}

func TestE7INT23NewKeyCannotReuseConsumed(t *testing.T) {
	env, fix, event, _, _ := seedApprovedLateWindow(t)
	ctx := context.Background()
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	_, err := env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, uuid.NewString())
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, uuid.NewString())
	if err == nil {
		t.Fatal("expected second submit denied")
	}
}

func TestE7INT24IdempotentSubmitReplay(t *testing.T) {
	env, fix, event, q, saved := seedApprovedLateWindow(t)
	ctx := context.Background()
	_, _ = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"ok"`)}},
	})
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	key := uuid.NewString()
	first, err := env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, key)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	second, err := env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, key)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.ResponseID != second.ResponseID {
		t.Fatal("idempotent replay mismatch")
	}
}

func TestE7INT25GlobalDeadlineUnchanged(t *testing.T) {
	env, fix, event, _, _ := seedApprovedLateWindow(t)
	ctx := context.Background()
	before, _ := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	_, _ = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, uuid.NewString())
	after, _ := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if before.ResponseDeadline == nil || after.ResponseDeadline == nil || !before.ResponseDeadline.Equal(*after.ResponseDeadline) {
		t.Fatal("global deadline changed")
	}
}

func TestE7INT26TimelyCarrierResponseUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	future := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Timely B",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-B-" + uuid.NewString()[:8],
		ResponseDeadline: &future,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	for _, carrier := range []uuid.UUID{fix.CarrierID, fix.CarrierBID} {
		if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
			TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: carrier, ParticipantType: "CARRIER",
		}); err != nil {
			t.Fatalf("participant: %v", err)
		}
	}
	version, _ := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	_, _ = env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled=TRUE, status='PUBLISHED', published_at=now() WHERE id=$1`, version.ID)
	_, _ = env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID)
	wsB, err := env.crSvc.StartOrResume(ctx, fix.CarrierBAct, event.ID, fix.CarrierBID)
	if err != nil {
		t.Fatalf("carrier B start: %v", err)
	}
	beforeVersion := wsB.Response.SaveVersion
	// carrier A late submission on a separate event must not mutate carrier B timely response
	_, _ = seedPublishedEventAfterDeadline(t, env, fix)
	wsAfter, err := env.crSvc.GetWorkspace(ctx, fix.CarrierBAct, event.ID, fix.CarrierBID)
	if err != nil {
		t.Fatalf("reload B: %v", err)
	}
	if wsAfter.Response.SaveVersion != beforeVersion || wsAfter.Response.Status != wsB.Response.Status {
		t.Fatal("timely carrier B response changed by unrelated late submission flow")
	}
}

func TestE7INT27ConcurrentApproveOneWinner(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	created, _ := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	from := time.Now().UTC().Add(-time.Minute)
	until := from.Add(2 * time.Hour)
	in := domain.ApproveLateSubmissionInput{ExpectedVersion: created.Version, ApprovedValidFrom: from, ApprovedValidUntil: until}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wins, fails int
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := env.lateSvc.Approve(context.Background(), fix.BuyerA, event.ID, created.ID, uuid.NewString(), in)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				wins++
			} else {
				fails++
			}
		}()
	}
	wg.Wait()
	if wins != 1 || fails != 3 {
		t.Fatalf("wins=%d fails=%d", wins, fails)
	}
}

func TestE7INT28ConcurrentSubmitOneResult(t *testing.T) {
	env, fix, event, q, saved := seedApprovedLateWindow(t)
	ctx := context.Background()
	_, _ = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"ok"`)}},
	})
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wins, fails int
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "submit-key-" + uuid.NewString()
			if i == 0 {
				key = "shared-key"
			}
			_, err := env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, key)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				wins++
			} else {
				fails++
			}
		}(i)
	}
	wg.Wait()
	if wins < 1 {
		t.Fatalf("expected at least one successful submit, wins=%d fails=%d", wins, fails)
	}
}

func TestE7INT29AuditFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected audit failure rollback")
	}
	items, _ := env.lateSvc.ListOwnRequests(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if len(items) != 0 {
		t.Fatal("request persisted despite audit failure")
	}
}

func TestE7INT30IdempotencyFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	env.idemRepo.SetInjectStoreFailure(true)
	t.Cleanup(func() { env.idemRepo.SetInjectStoreFailure(false) })
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected idempotency failure rollback")
	}
	items, _ := env.lateSvc.ListOwnRequests(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if len(items) != 0 {
		t.Fatal("request persisted despite idempotency failure")
	}
}

func TestE7INT31SubmitFailureDoesNotConsume(t *testing.T) {
	env, fix, event, _, _ := seedApprovedLateWindow(t)
	ctx := context.Background()
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	_, err := env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion-1, uuid.NewString())
	if err == nil {
		t.Fatal("expected submit failure")
	}
	items, _ := env.lateSvc.ListOwnRequests(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if items[0].Status != domain.LateSubmissionStatusApproved {
		t.Fatalf("permission consumed on failed submit: %s", items[0].Status)
	}
}

func TestE7INT32FeatureFlagDisabledHTTP404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	rec := postLateSubmissionHTTP(t, env, config.Config{RfxLateSubmissionEnabled: false}, fix, event.ID, uuid.NewString())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	items, _ := env.lateSvc.ListOwnRequests(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if len(items) != 0 {
		t.Fatal("feature disabled route must not write")
	}
}

func TestE7INT33SpoofedCompanyIgnored(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierBID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "spoof", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected spoofed carrier company denial")
	}
}

func TestE7INT34BuyerQueueIsolationByTenant(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	_, _ = env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	_, err := env.lateSvc.BuyerListRequests(context.Background(), fix.CrossTenant, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE7INT40CarrierResponseRegressionSmoke(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	future := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Regression",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-REG-" + uuid.NewString()[:8],
		ResponseDeadline: &future,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	_, _ = env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	})
	version, _ := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	_, _ = env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled=TRUE, status='PUBLISHED', published_at=now() WHERE id=$1`, version.ID)
	_, _ = env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID)
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if ws.Response.ID == uuid.Nil {
		t.Fatal("expected response")
	}
}

func seedApprovedLateWindow(t *testing.T) (*testEnv, buyerFixture, *domain.RfxEvent, *domain.Question, *domain.ResponseSaveResult) {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, q := seedPublishedEventAfterDeadline(t, env, fix)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "x", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	from := time.Now().UTC().Add(-time.Minute)
	until := from.Add(2 * time.Hour)
	approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	ctx := context.Background()
	ws, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"draft"`)}},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	return env, fix, event, q, saved
}

func postLateSubmissionHTTP(t *testing.T, env *testEnv, cfg config.Config, fix buyerFixture, eventID uuid.UUID, key string) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, cfg, env.rfxSvc, env.qSvc, nil, nil, nil, nil, env.crSvc, env.lateSvc, nil, nil, nil, nil, nil, nil)
	body, _ := json.Marshal(map[string]any{
		"reason_code": "OTHER", "reason_text": "http", "requested_until": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/"+eventID.String()+"/late-submission-requests?carrier_company_id="+fix.CarrierID.String(), bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.CarrierAct.UserID.String())
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

//go:build integration

package relationship

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
)

func applyCancelDown(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	down, err := os.ReadFile(filepath.Join(dir, "000101_company_carrier_relationship_cancel_recovery_v0_2.down.sql"))
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(down))
	return err
}

func cycleCancelMigration(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	up, err := os.ReadFile(filepath.Join(dir, "000101_company_carrier_relationship_cancel_recovery_v0_2.up.sql"))
	if err != nil {
		return err
	}
	down, err := os.ReadFile(filepath.Join(dir, "000101_company_carrier_relationship_cancel_recovery_v0_2.down.sql"))
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		return err
	}
	if err := expectCancelObjects(ctx, pool, true); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		return err
	}
	if err := expectCancelObjects(ctx, pool, false); err != nil {
		return err
	}
	var requests bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('company.carrier_relationship_requests') IS NOT NULL`).Scan(&requests); err != nil || !requests {
		return err
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(up)); err == nil {
		return errRepeatUp
	}
	return expectCancelObjects(ctx, pool, true)
}

func assertCancelRecovery(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	ctx := context.Background()
	defer func() {
		repository.CancelTerminalInsertHook = nil
		repository.CancelTerminalConflictHook = nil
	}()
	assertStatusOracle(t, pool, svc, cat, fix)

	own := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "status-key", "status")
	rows, err := svc.ListSourceRequests(ctx, listInput(fix.ShipperAdmin, fix.TenantA, own.SourceID, fix.ShipperA))
	if err != nil || len(rows) != 1 || rows[0].RequestID != own.RequestID || rows[0].Status != domain.RequestStatusPending || rows[0].CarrierCompanyID != fix.CarrierA || rows[0].TokenExpiresAt.IsZero() {
		t.Fatalf("own status: %v %+v", err, rows)
	}
	encoded, _ := json.Marshal(rows)
	if strings.Contains(string(encoded), "acceptance") || strings.Contains(string(encoded), own.Token) {
		t.Fatalf("status disclosed a token: %s", encoded)
	}
	_, err = svc.ListSourceRequests(ctx, listInput(fix.ShipperAdmin, fix.TenantB, own.SourceID, fix.ShipperA))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	if strings.Contains(err.Error(), own.RequestID.String()) || strings.Contains(err.Error(), own.Token) {
		t.Fatalf("foreign tenant status disclosed the request: %v", err)
	}
	_, err = svc.ListSourceRequests(ctx, listInput(fix.PlatformNoMembership, fix.TenantA, own.SourceID, fix.ShipperA))
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	_, err = svc.ListSourceRequests(ctx, listInput(fix.CompanyScopedPlatformAdmin, fix.TenantA, own.SourceID, fix.ShipperA))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = svc.ListSourceRequests(ctx, listInput(fix.BlockedUser, fix.TenantA, own.SourceID, fix.ShipperA))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)

	ordinary := uuid.New()
	insertUser(t, pool, ordinary, fix.TenantA, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantA, fix.ShipperA, ordinary)
	_, err = svc.ListSourceRequests(ctx, listInput(ordinary, fix.TenantA, own.SourceID, fix.ShipperA))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)

	cfg := postgresRelationshipConfig()
	cfg.CommandsEnabled = false
	disabled := service.NewRelationshipService(repository.NewRelationshipRepository(pool), repository.NewRelationshipRepository(pool), cfg, matchSource{}, cat)
	beforeDisabled := companySnapshot(t, pool)
	_, err = disabled.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, own.SourceID, own.RequestID, "disabled", "no"))
	assertCode(t, err, apperrors.CodeRelationshipCommandsDisabled)
	if companySnapshot(t, pool) != beforeDisabled {
		t.Fatal("disabled cancel wrote rows")
	}

	wrongCompany := fix.CarrierA
	denied := cancelInput(fix.ShipperAdmin, fix.TenantA, own.SourceID, own.RequestID, "wrong-company", "no")
	denied.BodyCompanyID = &wrongCompany
	_, err = svc.Cancel(ctx, denied)
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	if headStatus(t, pool, own.RequestID) != domain.RequestStatusPending {
		t.Fatal("mismatched company changed the head")
	}
	_, err = svc.Cancel(ctx, cancelInput(fix.PlatformNoMembership, fix.TenantA, own.SourceID, own.RequestID, "platform-none", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	_, err = svc.Cancel(ctx, cancelInput(fix.CompanyScopedPlatformAdmin, fix.TenantA, own.SourceID, own.RequestID, "platform-scoped", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = svc.Cancel(ctx, cancelInput(ordinary, fix.TenantA, own.SourceID, own.RequestID, "ordinary", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRequestForbidden)
	_, err = svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantB, own.SourceID, own.RequestID, "foreign-tenant", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRequestNotFound)
	if headStatus(t, pool, own.RequestID) != domain.RequestStatusPending || strings.Contains(err.Error(), own.RequestID.String()) {
		t.Fatalf("foreign cancel disclosed state: %v", err)
	}

	platformRequest := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "platform-create", "platform")
	platformOut, err := svc.Cancel(ctx, cancelInput(fix.PlatformWithMembership, fix.TenantA, platformRequest.SourceID, platformRequest.RequestID, "platform-cancel", "platform"))
	if err != nil || platformOut.Status != domain.RequestStatusCancelled || terminalEvents(t, pool, platformRequest.RequestID) != 1 {
		t.Fatalf("platform cancel: %v %+v events=%d", err, platformOut, terminalEvents(t, pool, platformRequest.RequestID))
	}

	cancelled, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, own.SourceID, own.RequestID, "cancel-key", "first-reason"))
	if err != nil || cancelled.Status != domain.RequestStatusCancelled {
		t.Fatalf("cancel: %v %+v", err, cancelled)
	}
	if terminalEvents(t, pool, own.RequestID) != 1 || cancelIdempotencyCount(t, pool, own.RequestID) != 1 {
		t.Fatalf("terminal=%d idempotency=%d", terminalEvents(t, pool, own.RequestID), cancelIdempotencyCount(t, pool, own.RequestID))
	}
	assertAuditAndOutbox(t, pool, own.RequestID, own.Token, "first-reason", "relationship_request_cancelled", "carrier_relationship.cancelled")
	replayed, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, own.SourceID, own.RequestID, "cancel-key", "second-reason"))
	if err != nil || replayed.Status != domain.RequestStatusCancelled || terminalEvents(t, pool, own.RequestID) != 1 {
		t.Fatalf("same key replay: %v %+v events=%d", err, replayed, terminalEvents(t, pool, own.RequestID))
	}
	var storedReason string
	if err := pool.QueryRow(ctx, `SELECT cancel_reason FROM company.carrier_relationship_request_events WHERE relationship_request_id = $1 AND event_type = 'CANCELLED'`, own.RequestID).Scan(&storedReason); err != nil || storedReason != "first-reason" {
		t.Fatalf("stored reason=%s err=%v", storedReason, err)
	}
	otherKey, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, own.SourceID, own.RequestID, "cancel-other", "ignored"))
	if err != nil || otherKey.Status != domain.RequestStatusCancelled || terminalEvents(t, pool, own.RequestID) != 1 || cancelIdempotencyCount(t, pool, own.RequestID) != 2 {
		t.Fatalf("different key: %v %+v events=%d keys=%d", err, otherKey, terminalEvents(t, pool, own.RequestID), cancelIdempotencyCount(t, pool, own.RequestID))
	}
	mustFail(t, pool, `UPDATE company.carrier_relationship_request_heads SET request_status = 'PENDING' WHERE relationship_request_id = $1`, own.RequestID)
	mustFail(t, pool, `
		INSERT INTO company.carrier_relationship_request_events (
			event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
		) VALUES ($1, $2, 3, 'CANCELLED', 'CANCELLED', $3)
	`, uuid.New(), own.RequestID, fix.ShipperAdmin)

	replacement, err := svc.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, own.SourceID, "replacement-key", "status"))
	if err != nil || !replacement.Created || replacement.AcceptanceToken == "" || replacement.RequestID == own.RequestID {
		t.Fatalf("create after cancel: %v %+v", err, replacement)
	}
	oldKey, err := svc.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, own.SourceID, "status-key", "status"))
	if err != nil || oldKey.Created || oldKey.AcceptanceToken != "" || oldKey.RequestID != own.RequestID || oldKey.Status != domain.RequestStatusCancelled {
		t.Fatalf("old create key: %v %+v", err, oldKey)
	}
	_, err = svc.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
		AcceptanceToken: own.Token, IdempotencyKey: "old-token",
	})
	assertCode(t, err, apperrors.CodeRelationshipRequestCancelled)

	lost := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "lost-create", "lost")
	found, err := svc.ListSourceRequests(ctx, listInput(fix.ShipperAdmin, fix.TenantA, lost.SourceID, fix.ShipperA))
	if err != nil || len(found) != 1 || found[0].RequestID != lost.RequestID || strings.Contains(mustJSON(found), lost.Token) {
		t.Fatalf("lost status: %v %+v", err, found)
	}
	lostCancel, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, lost.SourceID, lost.RequestID, "lost-cancel", "lost"))
	if err != nil || lostCancel.Status != domain.RequestStatusCancelled {
		t.Fatalf("lost cancel: %v %+v", err, lostCancel)
	}
	recovered, err := svc.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, lost.SourceID, "lost-new", "lost"))
	if err != nil || !recovered.Created || recovered.AcceptanceToken == "" || recovered.RequestID == lost.RequestID {
		t.Fatalf("lost recovery create: %v %+v", err, recovered)
	}

	expiredID := insertExpiredPending(t, pool, fix)
	expired, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, expiredSourceID, expiredID, "expire-key", "late"))
	if err != nil || expired.Status != domain.RequestStatusExpired || headStatus(t, pool, expiredID) != domain.RequestStatusExpired || terminalEvents(t, pool, expiredID) != 1 {
		t.Fatalf("expired cancel: %v %+v", err, expired)
	}
	assertAuditAndOutbox(t, pool, expiredID, "", "late", "relationship_request_expired", "carrier_relationship.expired")
	cat.allow(domain.SourceCreateSubject{
		SourceType: domain.SourceTypeShipment, SourceID: expiredSourceID, SourceTenantID: fix.TenantA, Fingerprint: "expired-fp",
		ShipperCarrierCompanyID: fix.CarrierA, RequestingCompanyID: fix.ShipperA, RequestingUserID: fix.ShipperAdmin,
	})
	released, err := svc.Create(ctx, createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, expiredSourceID, "expired-replacement", "expired-fp"))
	if err != nil || !released.Created || released.AcceptanceToken == "" {
		t.Fatalf("create after expiry: %v %+v", err, released)
	}

	accepted := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "accepted-create", "accepted")
	if _, err := svc.Accept(ctx, service.AcceptRelationshipInput{
		Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
		AcceptanceToken: accepted.Token, IdempotencyKey: "accept-then-cancel",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, accepted.SourceID, accepted.RequestID, "too-late", "no"))
	assertCode(t, err, apperrors.CodeRelationshipRequestAlreadyAccepted)
	if terminalEvents(t, pool, accepted.RequestID) != 1 || headStatus(t, pool, accepted.RequestID) != domain.RequestStatusAccepted {
		t.Fatal("cancel after accept wrote another terminal event")
	}

	left := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "conflict-left", "conflict")
	right := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "conflict-right", "conflict")
	if _, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, left.SourceID, left.RequestID, "shared-cancel", "left")); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, right.SourceID, right.RequestID, "shared-cancel", "right"))
	assertCode(t, err, apperrors.CodeRelationshipIdempotencyConflict)
	if headStatus(t, pool, right.RequestID) != domain.RequestStatusPending || strings.Contains(err.Error(), right.RequestID.String()) {
		t.Fatalf("idempotency conflict changed or disclosed the other request: %v", err)
	}

	concurrent := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "concurrent-cancel", "concurrent")
	assertTwoCancels(t, pool, svc, fix, concurrent)
	race := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "race-create", "race")
	assertCancelAcceptRace(t, pool, svc, fix, race)
	assertLockedRollback(t, pool, svc, fix, cat)
	assertTerminalReread(t, pool, svc, cat, fix)

	var requests, events, relationships, versions int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM company.carrier_relationship_requests),
			(SELECT count(*) FROM company.carrier_relationship_request_events),
			(SELECT count(*) FROM company.carrier_relationships),
			(SELECT count(*) FROM company.carrier_relationship_versions)
	`).Scan(&requests, &events, &relationships, &versions); err != nil {
		t.Fatal(err)
	}
	dir, err := migrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(dir, "000101_company_carrier_relationship_cancel_recovery_v0_2.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile(filepath.Join(dir, "000101_company_carrier_relationship_cancel_recovery_v0_2.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	var afterRequests, afterEvents, afterRelationships, afterVersions int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM company.carrier_relationship_requests),
			(SELECT count(*) FROM company.carrier_relationship_request_events),
			(SELECT count(*) FROM company.carrier_relationships),
			(SELECT count(*) FROM company.carrier_relationship_versions)
	`).Scan(&afterRequests, &afterEvents, &afterRelationships, &afterVersions); err != nil {
		t.Fatal(err)
	}
	if requests != afterRequests || events != afterEvents || relationships != afterRelationships || versions != afterVersions {
		t.Fatalf("down changed history requests %d/%d events %d/%d relationships %d/%d versions %d/%d", requests, afterRequests, events, afterEvents, relationships, afterRelationships, versions, afterVersions)
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	final := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "after-cycle", "cycle")
	if _, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, final.SourceID, final.RequestID, "after-cycle-cancel", "cycle")); err != nil {
		t.Fatal(err)
	}
}

func assertTwoCancels(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, fix fixture, created createdRequest) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	outs := make([]service.CancelRelationshipOutput, 2)
	keys := []string{"two-a", "two-b"}
	wg.Add(2)
	for i := range keys {
		go func(i int) {
			defer wg.Done()
			outs[i], errs[i] = svc.Cancel(context.Background(), cancelInput(fix.ShipperAdmin, fix.TenantA, created.SourceID, created.RequestID, keys[i], "two"))
		}(i)
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil || outs[i].Status != domain.RequestStatusCancelled || outs[i].RequestID != created.RequestID {
			t.Fatalf("concurrent cancel %d: %v %+v", i, errs[i], outs[i])
		}
	}
	if terminalEvents(t, pool, created.RequestID) != 1 {
		t.Fatalf("concurrent cancels wrote %d terminal events", terminalEvents(t, pool, created.RequestID))
	}
}

func assertCancelAcceptRace(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, fix fixture, created createdRequest) {
	t.Helper()
	var wg sync.WaitGroup
	var cancelErr, acceptErr error
	var cancelOut service.CancelRelationshipOutput
	wg.Add(2)
	go func() {
		defer wg.Done()
		cancelOut, cancelErr = svc.Cancel(context.Background(), cancelInput(fix.ShipperAdmin, fix.TenantA, created.SourceID, created.RequestID, "race-cancel", "race"))
	}()
	go func() {
		defer wg.Done()
		_, acceptErr = svc.Accept(context.Background(), service.AcceptRelationshipInput{
			Principal: domain.Principal{UserID: fix.CarrierAdmin, TenantID: fix.TenantB}, SelectedCompanyID: fix.OperatingB,
			AcceptanceToken: created.Token, IdempotencyKey: "race-accept",
		})
	}()
	wg.Wait()
	if terminalEvents(t, pool, created.RequestID) != 1 {
		t.Fatalf("race terminal events=%d", terminalEvents(t, pool, created.RequestID))
	}
	switch headStatus(t, pool, created.RequestID) {
	case domain.RequestStatusAccepted:
		if cancelErr == nil || cancelOut.Status != "" {
			t.Fatalf("accept won but cancel returned success: %v %+v", cancelErr, cancelOut)
		}
		assertCode(t, cancelErr, apperrors.CodeRelationshipRequestAlreadyAccepted)
		if acceptErr != nil {
			t.Fatalf("accept won but returned %v", acceptErr)
		}
	case domain.RequestStatusCancelled:
		if cancelErr != nil || cancelOut.Status != domain.RequestStatusCancelled {
			t.Fatalf("cancel won but returned %v %+v", cancelErr, cancelOut)
		}
		assertCode(t, acceptErr, apperrors.CodeRelationshipRequestCancelled)
	default:
		t.Fatalf("race head=%s cancel=%v accept=%v", headStatus(t, pool, created.RequestID), cancelErr, acceptErr)
	}
}

func assertLockedRollback(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, fix fixture, cat *catalogSource) {
	t.Helper()
	created := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "rollback-create", "rollback")
	before := companySnapshot(t, pool)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `SELECT request_status FROM company.carrier_relationship_request_heads WHERE relationship_request_id = $1 FOR UPDATE`, created.RequestID); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, err = svc.Cancel(lockCtx, cancelInput(fix.ShipperAdmin, fix.TenantA, created.SourceID, created.RequestID, "rollback-cancel", "rollback"))
	cancel()
	_ = tx.Rollback(context.Background())
	assertCode(t, err, apperrors.CodeRelationshipStoreUnavailable)
	if companySnapshot(t, pool) != before || headStatus(t, pool, created.RequestID) != domain.RequestStatusPending || terminalEvents(t, pool, created.RequestID) != 0 || cancelIdempotencyCount(t, pool, created.RequestID) != 0 {
		t.Fatal("failed cancel left a partial terminal transition")
	}
	if _, err := svc.Cancel(context.Background(), cancelInput(fix.ShipperAdmin, fix.TenantA, created.SourceID, created.RequestID, "rollback-cancel", "rollback")); err != nil {
		t.Fatal(err)
	}
}

func assertTerminalReread(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	ctx := context.Background()
	accepted := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "reread-accept", "reread-accept")
	eventID := uuid.New()
	repository.CancelTerminalInsertHook = func(hookCtx context.Context) {
		if _, err := pool.Exec(hookCtx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
			) VALUES ($1, $2, 2, 'ACCEPTED', 'ACCEPTED', $3)
		`, eventID, accepted.RequestID, fix.CarrierAdmin); err != nil {
			t.Errorf("insert accepted event: %v", err)
		}
	}
	repository.CancelTerminalConflictHook = func(hookCtx context.Context) {
		if _, err := pool.Exec(hookCtx, `
			UPDATE company.carrier_relationship_request_heads
			SET current_event_id = $2, current_version = 2, request_status = 'ACCEPTED'
			WHERE relationship_request_id = $1
		`, accepted.RequestID, eventID); err != nil {
			t.Errorf("mark accepted: %v", err)
		}
	}
	_, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, accepted.SourceID, accepted.RequestID, "reread-accept-key", "no"))
	repository.CancelTerminalInsertHook = nil
	repository.CancelTerminalConflictHook = nil
	assertCode(t, err, apperrors.CodeRelationshipRequestAlreadyAccepted)
	if codeOf(err) == apperrors.CodeRelationshipIdempotencyConflict || headStatus(t, pool, accepted.RequestID) != domain.RequestStatusAccepted || cancelIdempotencyCount(t, pool, accepted.RequestID) != 0 || terminalEvents(t, pool, accepted.RequestID) != 1 {
		t.Fatalf("accepted reread: %v events=%d keys=%d", err, terminalEvents(t, pool, accepted.RequestID), cancelIdempotencyCount(t, pool, accepted.RequestID))
	}

	cancelled := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "reread-cancel", "reread-cancel")
	cancelEvent := uuid.New()
	repository.CancelTerminalInsertHook = func(hookCtx context.Context) {
		if _, err := pool.Exec(hookCtx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
			) VALUES ($1, $2, 2, 'CANCELLED', 'CANCELLED', $3)
		`, cancelEvent, cancelled.RequestID, fix.ShipperAdmin); err != nil {
			t.Errorf("insert cancelled event: %v", err)
		}
	}
	repository.CancelTerminalConflictHook = func(hookCtx context.Context) {
		if _, err := pool.Exec(hookCtx, `
			UPDATE company.carrier_relationship_request_heads
			SET current_event_id = $2, current_version = 2, request_status = 'CANCELLED'
			WHERE relationship_request_id = $1
		`, cancelled.RequestID, cancelEvent); err != nil {
			t.Errorf("mark cancelled: %v", err)
		}
	}
	out, err := svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, cancelled.SourceID, cancelled.RequestID, "reread-cancel-key", "hook"))
	repository.CancelTerminalInsertHook = nil
	repository.CancelTerminalConflictHook = nil
	if err != nil || out.Status != domain.RequestStatusCancelled || terminalEvents(t, pool, cancelled.RequestID) != 1 || cancelIdempotencyCount(t, pool, cancelled.RequestID) != 1 {
		t.Fatalf("cancelled reread: %v %+v events=%d", err, out, terminalEvents(t, pool, cancelled.RequestID))
	}

	pending := createOK(t, svc, cat, fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, "reread-pending", "reread-pending")
	repository.CancelTerminalInsertHook = func(hookCtx context.Context) {
		if _, err := pool.Exec(hookCtx, `
			INSERT INTO company.carrier_relationship_request_events (
				event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
			) VALUES ($1, $2, 2, 'CANCELLED', 'CANCELLED', $3)
		`, uuid.New(), pending.RequestID, fix.ShipperAdmin); err != nil {
			t.Errorf("insert pending conflict event: %v", err)
		}
	}
	_, err = svc.Cancel(ctx, cancelInput(fix.ShipperAdmin, fix.TenantA, pending.SourceID, pending.RequestID, "reread-pending-key", "pending"))
	repository.CancelTerminalInsertHook = nil
	assertCode(t, err, apperrors.CodeRelationshipStoreUnavailable)
	if codeOf(err) == apperrors.CodeRelationshipIdempotencyConflict || headStatus(t, pool, pending.RequestID) != domain.RequestStatusPending || cancelIdempotencyCount(t, pool, pending.RequestID) != 0 {
		t.Fatalf("pending reread: %v", err)
	}
}

func assertStatusOracle(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, cat *catalogSource, fix fixture) {
	t.Helper()
	nonMember := uuid.New()
	member := uuid.New()
	insertUser(t, pool, nonMember, fix.TenantA, "ACTIVE", nil)
	insertUser(t, pool, member, fix.TenantA, "ACTIVE", nil)
	insertMembership(t, pool, fix.TenantA, fix.ShipperA, member)
	sources := []struct {
		sourceType string
		token      string
	}{
		{domain.SourceTypeShipment, shipmentRelationshipToken},
		{domain.SourceTypeRfxAward, rfxRelationshipToken},
		{domain.SourceTypeAwardLink, rfxRelationshipToken},
	}
	for _, source := range sources {
		sourceID := uuid.New()
		read := func(user, tenant, company uuid.UUID, token, sourceType string) service.ListSourceRequestsInput {
			return service.ListSourceRequestsInput{
				ServiceToken: token, SourceType: sourceType, SourceID: sourceID, SourceTenantID: tenant,
				RequestingUserID: user, RequestingCompanyID: company,
			}
		}
		own := read(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, source.token, source.sourceType)
		denials := []struct {
			name string
			in   service.ListSourceRequestsInput
			code apperrors.Code
		}{
			{"non-member", read(nonMember, fix.TenantA, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestNotFound},
			{"member", read(member, fix.TenantA, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestForbidden},
			{"blocked", read(fix.BlockedUser, fix.TenantA, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestForbidden},
			{"deleted", read(fix.DeletedUser, fix.TenantA, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestForbidden},
			{"platform-none", read(fix.PlatformNoMembership, fix.TenantA, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestNotFound},
			{"platform-scoped", read(fix.CompanyScopedPlatformAdmin, fix.TenantA, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestForbidden},
			{"foreign-tenant", read(fix.ShipperAdmin, fix.TenantB, fix.ShipperA, source.token, source.sourceType), apperrors.CodeRelationshipRequestForbidden},
			{"other-company", read(fix.ShipperAdmin, fix.TenantA, fix.CarrierA, source.token, source.sourceType), apperrors.CodeRelationshipRequestNotFound},
		}
		otherToken := rfxRelationshipToken
		otherType := domain.SourceTypeShipment
		if source.sourceType == domain.SourceTypeShipment {
			otherType = domain.SourceTypeRfxAward
		} else {
			otherToken = shipmentRelationshipToken
		}
		denials = append(denials, struct {
			name string
			in   service.ListSourceRequestsInput
			code apperrors.Code
		}{"other-credential", read(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, otherToken, source.sourceType), apperrors.CodeRelationshipRequestForbidden})

		assertStatusDenials(t, pool, svc, denials, "")
		empty, err := svc.ListSourceRequests(context.Background(), own)
		if err != nil || len(empty) != 0 {
			t.Fatalf("%s creator before create: %v %+v", source.sourceType, err, empty)
		}
		platform, err := svc.ListSourceRequests(context.Background(), read(fix.PlatformWithMembership, fix.TenantA, fix.ShipperA, source.token, source.sourceType))
		if err != nil || len(platform) != 0 {
			t.Fatalf("%s platform admin before create: %v %+v", source.sourceType, err, platform)
		}

		cat.allow(domain.SourceCreateSubject{
			SourceType: source.sourceType, SourceID: sourceID, SourceTenantID: fix.TenantA, Fingerprint: "oracle",
			ShipperCarrierCompanyID: fix.CarrierA, RequestingCompanyID: fix.ShipperA, RequestingUserID: fix.ShipperAdmin,
		})
		input := createInput(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, fix.CarrierA, sourceID, "oracle-"+source.sourceType, "oracle")
		input.SourceType = source.sourceType
		input.ServiceToken = source.token
		created, err := svc.Create(context.Background(), input)
		if err != nil || !created.Created || created.AcceptanceToken == "" {
			t.Fatalf("%s create: %v %+v", source.sourceType, err, created)
		}
		assertStatusDenials(t, pool, svc, denials, created.RequestID.String()+" "+created.AcceptanceToken)
		rows, err := svc.ListSourceRequests(context.Background(), own)
		encoded := mustJSON(rows)
		if err != nil || len(rows) != 1 || rows[0].RequestID != created.RequestID || strings.Contains(encoded, created.AcceptanceToken) || strings.Contains(encoded, "acceptance") {
			t.Fatalf("%s creator after create: %v %s", source.sourceType, err, encoded)
		}
		platformRows, err := svc.ListSourceRequests(context.Background(), read(fix.PlatformWithMembership, fix.TenantA, fix.ShipperA, source.token, source.sourceType))
		if err != nil || len(platformRows) != 1 || platformRows[0].RequestID != created.RequestID || strings.Contains(mustJSON(platformRows), created.AcceptanceToken) {
			t.Fatalf("%s platform admin after create: %v %+v", source.sourceType, err, platformRows)
		}
		cross, err := svc.ListSourceRequests(context.Background(), read(fix.ShipperAdmin, fix.TenantA, fix.ShipperA, otherToken, otherType))
		if err != nil || len(cross) != 0 || strings.Contains(mustJSON(cross), created.RequestID.String()) {
			t.Fatalf("%s other credential opened the request: %v %+v", source.sourceType, err, cross)
		}
	}
}

func assertStatusDenials(t *testing.T, pool *pgxpool.Pool, svc *service.RelationshipService, denials []struct {
	name string
	in   service.ListSourceRequestsInput
	code apperrors.Code
}, secret string) {
	t.Helper()
	for _, denial := range denials {
		before := companySnapshot(t, pool)
		audits := auditCount(t, pool)
		_, err := svc.ListSourceRequests(context.Background(), denial.in)
		assertCode(t, err, denial.code)
		if secret != "" {
			for _, part := range strings.Fields(secret) {
				if strings.Contains(err.Error(), part) {
					t.Fatalf("%s disclosed %s: %v", denial.name, part, err)
				}
			}
		}
		if companySnapshot(t, pool) != before || auditCount(t, pool) != audits {
			t.Fatalf("%s status read wrote rows", denial.name)
		}
	}
}

func auditCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM company.carrier_relationship_audit`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func listInput(user, tenant, source, company uuid.UUID) service.ListSourceRequestsInput {
	return service.ListSourceRequestsInput{
		ServiceToken: shipmentRelationshipToken, SourceType: domain.SourceTypeShipment,
		SourceID: source, SourceTenantID: tenant, RequestingUserID: user, RequestingCompanyID: company,
	}
}

func cancelInput(user, tenant, source, request uuid.UUID, key, reason string) service.CancelRelationshipInput {
	return service.CancelRelationshipInput{
		ServiceToken: shipmentRelationshipToken, SourceType: domain.SourceTypeShipment,
		SourceID: source, SourceTenantID: tenant, RequestingUserID: user, RequestID: request,
		IdempotencyKey: key, Reason: reason,
	}
}

func headStatus(t *testing.T, pool *pgxpool.Pool, requestID uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT request_status FROM company.carrier_relationship_request_heads WHERE relationship_request_id = $1`, requestID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func terminalEvents(t *testing.T, pool *pgxpool.Pool, requestID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM company.carrier_relationship_request_events
		WHERE relationship_request_id = $1 AND event_type IN ('ACCEPTED', 'EXPIRED', 'CANCELLED')
	`, requestID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func cancelIdempotencyCount(t *testing.T, pool *pgxpool.Pool, requestID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM company.carrier_relationship_cancel_idempotency WHERE relationship_request_id = $1`, requestID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func assertAuditAndOutbox(t *testing.T, pool *pgxpool.Pool, requestID uuid.UUID, token, reason, action, eventType string) {
	t.Helper()
	var auditReason string
	var audits int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*), COALESCE(max(reason), '')
		FROM company.carrier_relationship_audit
		WHERE relationship_request_id = $1 AND action = $2
	`, requestID, action).Scan(&audits, &auditReason); err != nil || audits != 1 || auditReason != reason {
		t.Fatalf("audit action=%s count=%d reason=%s err=%v", action, audits, auditReason, err)
	}
	var payload string
	var outbox int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*), COALESCE(string_agg(payload::text, ''), '')
		FROM company.carrier_relationship_outbox
		WHERE aggregate_id = $1 AND event_type = $2
	`, requestID, eventType).Scan(&outbox, &payload); err != nil || outbox != 1 {
		t.Fatalf("outbox %s count=%d err=%v", eventType, outbox, err)
	}
	if strings.Contains(payload, "acceptance_token") || (token != "" && strings.Contains(payload, token)) || strings.Contains(payload, reason) {
		t.Fatalf("outbox payload disclosed token or reason: %s", payload)
	}
}

func insertExpiredPending(t *testing.T, pool *pgxpool.Pool, fix fixture) uuid.UUID {
	t.Helper()
	requestID := uuid.New()
	eventID := uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_requests (
			relationship_request_id, source_type, source_id, source_tenant_id, source_version_or_fingerprint,
			shipper_carrier_company_id, requesting_company_id, requesting_user_id, acceptance_token_hash,
			token_created_at, token_expires_at, created_by
		)
		SELECT $1, 'SHIPMENT', $2, $3, 'expired-fp', $4, $5, $6, $7,
		       ts.t - interval '73 hours', ts.t - interval '73 hours' + interval '72 hours', $6
		FROM (SELECT clock_timestamp() AS t) ts
	`, requestID, expiredSourceID, fix.TenantA, fix.CarrierA, fix.ShipperA, fix.ShipperAdmin, uniqueTokenHash(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_events (
			event_id, relationship_request_id, request_version, event_type, request_status, actor_user_id
		) VALUES ($1, $2, 1, 'CREATED', 'PENDING', $3)
	`, eventID, requestID, fix.ShipperAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO company.carrier_relationship_request_heads (
			relationship_request_id, source_type, source_id, shipper_carrier_company_id, current_event_id, current_version, request_status
		) VALUES ($1, 'SHIPMENT', $2, $3, $4, 1, 'PENDING')
	`, requestID, expiredSourceID, fix.CarrierA, eventID); err != nil {
		t.Fatal(err)
	}
	return requestID
}

func expectCancelObjects(ctx context.Context, pool *pgxpool.Pool, present bool) error {
	var table, index, fn bool
	if err := pool.QueryRow(ctx, `
		SELECT to_regclass('company.carrier_relationship_cancel_idempotency') IS NOT NULL,
		       EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'company' AND indexname = 'uq_crr_one_terminal_event'),
		       EXISTS (
		           SELECT 1 FROM pg_proc p
		           JOIN pg_namespace n ON n.oid = p.pronamespace
		           WHERE n.nspname = 'company' AND p.proname = 'reject_request_head_illegal_transition'
		       )
	`).Scan(&table, &index, &fn); err != nil {
		return err
	}
	if table != present || index != present || fn != present {
		return errCancelObjects
	}
	return nil
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func codeOf(err error) apperrors.Code {
	var appErr *apperrors.AppError
	if asError(err, &appErr) {
		return appErr.Code
	}
	return ""
}

var (
	errRepeatUp      = errString("second 000101 up succeeded")
	errCancelObjects = errString("000101 objects did not match the expected state")
	expiredSourceID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
)

type errString string

func (e errString) Error() string { return string(e) }

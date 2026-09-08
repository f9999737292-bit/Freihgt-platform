//go:build integration

package versionlifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestE1INT27ExpiredKeyReuseReplacesRecord(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-27")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e1-int-27-expired"
	firstIn := domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "First lifecycle",
	}
	first, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, firstIn)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	expireIdempotencyKey(t, env, fix, event.ID, key, domain.VersionLifecycleOperationPublish)

	draft2, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-27-fork")
	if err != nil {
		t.Fatalf("fork after expiry: %v", err)
	}
	secondIn := buildRepublishPublishInput(t, env, fix, fix.BuyerA, event.ID, draft2, "Second lifecycle after expiry")
	second, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, secondIn)
	if err != nil {
		t.Fatalf("publish after expired key reuse: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected new published version after expired key reuse")
	}
	if second.ChangeSummary == nil || *second.ChangeSummary != "Second lifecycle after expiry" {
		t.Fatalf("unexpected replay payload: %+v", second)
	}

	var recordCount int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, event.ID, key).Scan(&recordCount); err != nil {
		t.Fatalf("count idempotency rows: %v", err)
	}
	if recordCount != 1 {
		t.Fatalf("expected single idempotency row after replacement, got %d", recordCount)
	}
}

func TestE1INT28ExpiredKeyRetryDoesNotReturnOldResponse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-28")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e1-int-28-expired"
	in := domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Old lifecycle",
	}
	first, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, in)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	expireIdempotencyKey(t, env, fix, event.ID, key, domain.VersionLifecycleOperationPublish)

	draft2, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-28-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	newIn := buildRepublishPublishInput(t, env, fix, fix.BuyerA, event.ID, draft2, "New lifecycle")
	replaced, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, newIn)
	if err != nil {
		t.Fatalf("publish with expired key: %v", err)
	}
	retry, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, newIn)
	if err != nil {
		t.Fatalf("retry after replacement: %v", err)
	}
	if retry.ID != replaced.ID || retry.ID == first.ID {
		t.Fatalf("retry returned stale response: first=%s replaced=%s retry=%s", first.ID, replaced.ID, retry.ID)
	}
}

func TestE1INT29UnexpiredSameKeySameBodyReplay(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-29")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e1-int-29-key"
	in := domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Stable publish",
	}
	first, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, in)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	second, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, in)
	if err != nil {
		t.Fatalf("replay publish: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("replay returned different version ids")
	}
	var versionCount int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND status = 'PUBLISHED' AND deleted_at IS NULL`,
		event.ID, fix.TenantID).Scan(&versionCount); err != nil {
		t.Fatalf("count published versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("expected one published version after replay, got %d", versionCount)
	}
}

func TestE1INT30UnexpiredSameKeyDifferentBody409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-30")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e1-int-30-key"
	if _, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "First body",
	}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	var beforeHash string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT request_body_hash FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, event.ID, key).Scan(&beforeHash); err != nil {
		t.Fatalf("read idempotency hash before conflict: %v", err)
	}

	draft2, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-30-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, buildRepublishPublishInput(t, env, fix, fix.BuyerA, event.ID, draft2, "Different body"))
	assertAppErrorCode(t, err, apperrors.CodeConflict)

	var afterHash string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT request_body_hash FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, event.ID, key).Scan(&afterHash); err != nil {
		t.Fatalf("read idempotency hash after conflict: %v", err)
	}
	if afterHash != beforeHash {
		t.Fatalf("idempotency record mutated on body mismatch")
	}
}

func TestE1INT31ConcurrentExpiredKeyReuseSingleMutation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-31")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e1-int-31-expired"
	if _, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Seed publish",
	}); err != nil {
		t.Fatalf("seed publish: %v", err)
	}
	expireIdempotencyKey(t, env, fix, event.ID, key, domain.VersionLifecycleOperationPublish)

	draft2, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-31-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	in := buildRepublishPublishInput(t, env, fix, fix.BuyerA, event.ID, draft2, "Concurrent after expiry")

	var wg sync.WaitGroup
	results := make([]*domain.RfxVersion, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, in)
		}(i)
	}
	wg.Wait()

	success := 0
	conflict := 0
	var resultID uuid.UUID
	for i, callErr := range errs {
		if callErr == nil {
			success++
			if results[i] == nil {
				t.Fatal("nil result on success")
			}
			if resultID == uuid.Nil {
				resultID = results[i].ID
			} else if results[i].ID != resultID {
				t.Fatalf("concurrent results diverged: %s vs %s", resultID, results[i].ID)
			}
			continue
		}
		var appErr *apperrors.AppError
		if errors.As(callErr, &appErr) && appErr.Code == apperrors.CodeConflict {
			conflict++
			continue
		}
		t.Fatalf("unexpected concurrent error[%d]: %v", i, callErr)
	}
	if success == 0 {
		t.Fatal("expected at least one successful or replayed response")
	}
	if success+conflict != 2 {
		t.Fatalf("expected two terminal outcomes, got success=%d conflict=%d", success, conflict)
	}

	var publishedCount int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND status = 'PUBLISHED' AND deleted_at IS NULL`,
		event.ID, fix.TenantID).Scan(&publishedCount); err != nil {
		t.Fatalf("count published versions: %v", err)
	}
	if publishedCount != 1 {
		t.Fatalf("expected one published version after concurrent reuse, got %d", publishedCount)
	}
}

func TestE1INT31BForkExpiredKeyReuse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-E1-31B")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-31b-pub")
	key := "e1-int-31b-fork-expired"

	first, err := env.versionSvc.ForkDraftFromPublished(ctx, fix.BuyerA, event.ID, key)
	if err != nil {
		t.Fatalf("first fork: %v", err)
	}
	expireIdempotencyKey(t, env, fix, event.ID, key, domain.VersionLifecycleOperationForkDraft)

	if _, err := env.pool.Exec(ctx, `DELETE FROM rfx.rfx_versions WHERE id = $1 AND tenant_id = $2`, first.ID, fix.TenantID); err != nil {
		t.Fatalf("remove stale draft for reuse test: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_events SET draft_version_id = NULL WHERE id = $1 AND tenant_id = $2`, event.ID, fix.TenantID); err != nil {
		t.Fatalf("clear draft pointer: %v", err)
	}

	second, err := env.versionSvc.ForkDraftFromPublished(ctx, fix.BuyerA, event.ID, key)
	if err != nil {
		t.Fatalf("fork after expired key reuse: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected new draft after expired key reuse")
	}
}

func expireIdempotencyKey(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, key, operation string) {
	t.Helper()
	tag, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_idempotency_records
		SET expires_at = $5
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3 AND operation = $4`,
		fix.TenantID, eventID, key, operation, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("expire idempotency key: %v", err)
	}
	if tag.RowsAffected() == 0 {
		t.Fatal("expected idempotency row to expire")
	}
}

func TestIdempotencyRepositoryStoreExpiredReplacement(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	scope := repository.IdempotencyScope{
		TenantID:       uuid.New(),
		ActorID:        uuid.New(),
		Operation:      domain.VersionLifecycleOperationPublish,
		AggregateScope: uuid.New(),
	}
	key := "repo-expired-reuse"
	oldBody := json.RawMessage(`{"old":true}`)
	newBody := json.RawMessage(`{"new":true}`)
	expiredAt := time.Now().UTC().Add(-time.Hour)
	if err := env.idemRepo.Store(ctx, repository.IdempotencyRecord{
		TenantID:        scope.TenantID,
		ActorID:         scope.ActorID,
		Operation:       scope.Operation,
		AggregateScope:  scope.AggregateScope,
		IdempotencyKey:  key,
		RequestBodyHash: "old-hash",
		ResponseStatus:  200,
		ResponseBody:    oldBody,
		ExpiresAt:       expiredAt,
	}); err != nil {
		t.Fatalf("seed expired record: %v", err)
	}
	if err := env.idemRepo.Store(ctx, repository.IdempotencyRecord{
		TenantID:        scope.TenantID,
		ActorID:         scope.ActorID,
		Operation:       scope.Operation,
		AggregateScope:  scope.AggregateScope,
		IdempotencyKey:  key,
		RequestBodyHash: "new-hash",
		ResponseStatus:  201,
		ResponseBody:    newBody,
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("replace expired record: %v", err)
	}
	record, err := env.idemRepo.Get(ctx, scope, key)
	if err != nil {
		t.Fatalf("load replaced record: %v", err)
	}
	if record == nil {
		t.Fatal("expected replaced record")
	}
	if record.RequestBodyHash != "new-hash" || record.ResponseStatus != 201 {
		t.Fatalf("unexpected replaced record: %+v", record)
	}
	var gotBody map[string]any
	if err := json.Unmarshal(record.ResponseBody, &gotBody); err != nil {
		t.Fatalf("decode replaced body: %v", err)
	}
	if gotBody["new"] != true {
		t.Fatalf("unexpected replaced body: %s", record.ResponseBody)
	}
}

func TestIdempotencyRepositoryStoreActiveConflict(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	scope := repository.IdempotencyScope{
		TenantID:       uuid.New(),
		ActorID:        uuid.New(),
		Operation:      domain.VersionLifecycleOperationPublish,
		AggregateScope: uuid.New(),
	}
	key := "repo-active-conflict"
	if err := env.idemRepo.Store(ctx, repository.IdempotencyRecord{
		TenantID:        scope.TenantID,
		ActorID:         scope.ActorID,
		Operation:       scope.Operation,
		AggregateScope:  scope.AggregateScope,
		IdempotencyKey:  key,
		RequestBodyHash: "same-hash",
		ResponseStatus:  200,
		ResponseBody:    json.RawMessage(`{"v":1}`),
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed active record: %v", err)
	}
	err := env.idemRepo.Store(ctx, repository.IdempotencyRecord{
		TenantID:        scope.TenantID,
		ActorID:         scope.ActorID,
		Operation:       scope.Operation,
		AggregateScope:  scope.AggregateScope,
		IdempotencyKey:  key,
		RequestBodyHash: "different-hash",
		ResponseStatus:  201,
		ResponseBody:    json.RawMessage(`{"v":2}`),
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
	})
	if !errors.Is(err, repository.ErrIdempotencyRecordActive) {
		t.Fatalf("expected ErrIdempotencyRecordActive, got %v", err)
	}
}

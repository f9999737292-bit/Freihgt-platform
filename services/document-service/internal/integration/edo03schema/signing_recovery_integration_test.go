//go:build integration

package edo03schema

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/repository"
)

func TestSigningRecoveryIsolatedPostgres(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))

	t.Run("expired_without_signatures_blocks_create_until_recovery", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		setDocumentStatus(t, pool, fx, domain.DocumentStatusReadyForSigning)
		created := mustCreateSession(t, pool, fx)
		if created.ID == fx.session || created.CompletedSignersCount != 0 {
			t.Fatalf("first session=%+v", created)
		}
		expireSession(t, pool, created.ID, fx.tenant)
		if _, err := signingService(pool).CreateSession(ctx, fx.doc, createInput(fx, futureExpiry())); err == nil {
			t.Fatal("create accepted while SIGNING_IN_PROGRESS")
		}
		doc, session := mustRecover(t, pool, fx, created.ID, 2)
		if doc.DocumentStatus != domain.DocumentStatusReadyForSigning || doc.Version != 3 {
			t.Fatalf("recovered document=%+v", doc)
		}
		if session.Status != domain.SigningSessionStatusExpired || session.CompletedSignersCount != 0 {
			t.Fatalf("expired session changed=%+v", session)
		}
		next := mustCreateSession(t, pool, fx)
		if next.ID == created.ID || next.CompletedSignersCount != 0 {
			t.Fatalf("new session=%+v", next)
		}
		state := readSessionState(t, pool, created.ID, fx.tenant)
		if state.sessionStatus != domain.SigningSessionStatusExpired || state.completed != 0 || state.signatures != 0 {
			t.Fatalf("old session state=%+v", state)
		}
	})

	t.Run("expired_partial_signatures_stay_on_old_session", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 2, 1, pastExpiry())
		signatureID := mustInsertLegacySignature(t, pool, fx.tenant, fx.session, fx.doc)
		doc, session := mustRecover(t, pool, fx, fx.session, 1)
		if doc.Version != 2 || session.CompletedSignersCount != 1 || session.Status != domain.SigningSessionStatusExpired {
			t.Fatalf("doc=%+v session=%+v", doc, session)
		}
		if readSessionState(t, pool, fx.session, fx.tenant).signatures != 1 {
			t.Fatal("historical signature was removed")
		}
		next := mustCreateSession(t, pool, fx)
		nextState := readSessionState(t, pool, next.ID, fx.tenant)
		if nextState.completed != 0 || nextState.signatures != 0 {
			t.Fatalf("new session inherited history=%+v", nextState)
		}
		var still uuid.UUID
		allow(t, pool.QueryRow(ctx, `SELECT id FROM documents.signatures WHERE id = $1 AND signing_session_id = $2`, signatureID, fx.session).Scan(&still))
	})

	t.Run("repeat_and_stale_version_do_not_move_again", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		mustRecover(t, pool, fx, fx.session, 1)
		_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, recoverInput(fx, fx.session, 1))
		if !apperrors.IsConflict(err) {
			t.Fatalf("repeat err=%v", err)
		}
		status, version := readDocument(t, pool, fx)
		if status != domain.DocumentStatusReadyForSigning || version != 2 {
			t.Fatalf("status=%s version=%d", status, version)
		}
		_, _, err = repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, recoverInput(fx, fx.session, 2))
		if err == nil || apperrors.IsConflict(err) {
			t.Fatalf("post-recovery version err=%v", err)
		}
		status, version = readDocument(t, pool, fx)
		if status != domain.DocumentStatusReadyForSigning || version != 2 {
			t.Fatalf("second status=%s version=%d", status, version)
		}
	})

	t.Run("old_session_cannot_reset_a_newer_attempt", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		mustRecover(t, pool, fx, fx.session, 1)
		next := mustCreateSession(t, pool, fx)
		_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, recoverInput(fx, fx.session, 3))
		if !apperrors.IsConflict(err) {
			t.Fatalf("old session recovery err=%v", err)
		}
		status, version := readDocument(t, pool, fx)
		if status != domain.DocumentStatusSigningInProgress || version != 3 {
			t.Fatalf("status=%s version=%d", status, version)
		}
		if readSessionState(t, pool, next.ID, fx.tenant).sessionStatus != domain.SigningSessionStatusCreated {
			t.Fatal("newer session was rolled back")
		}
	})

	t.Run("active_completed_and_disallowed_statuses_block", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		active := uuid.New()
		allow(t, exec(pool, `
			INSERT INTO documents.signing_sessions (
				id, tenant_id, document_id, status, required_signers_count, created_at
			) VALUES ($1,$2,$3,'CREATED',1, now() + interval '1 minute')`, active, fx.tenant, fx.doc))
		_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, recoverInput(fx, fx.session, 1))
		if !apperrors.IsConflict(err) {
			t.Fatalf("active err=%v", err)
		}
		allow(t, exec(pool, `DELETE FROM documents.signing_sessions WHERE id = $1`, active))
		done := uuid.New()
		allow(t, exec(pool, `
			INSERT INTO documents.signing_sessions (
				id, tenant_id, document_id, status, required_signers_count, created_at
			) VALUES ($1,$2,$3,'COMPLETED',1, now() - interval '1 hour')`, done, fx.tenant, fx.doc))
		_, _, err = repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, recoverInput(fx, fx.session, 1))
		if !apperrors.IsConflict(err) {
			t.Fatalf("completed err=%v", err)
		}
		allow(t, exec(pool, `DELETE FROM documents.signing_sessions WHERE id = $1`, done))
		for _, status := range []string{
			domain.DocumentStatusSigned, domain.DocumentStatusAccepted, domain.DocumentStatusArchived,
			domain.DocumentStatusCancelled, domain.DocumentStatusSentToOperator,
		} {
			setDocumentStatus(t, pool, fx, status)
			_, _, err = repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, recoverInput(fx, fx.session, 1))
			if err == nil {
				t.Fatalf("status %s was recovered", status)
			}
			got, version := readDocument(t, pool, fx)
			if got != status || version != 1 {
				t.Fatalf("status changed to %s version %d", got, version)
			}
		}
	})

	t.Run("foreign_tenant_and_document_mismatch", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		foreign := recoverInput(fx, fx.session, 1)
		foreign.TenantID = fx.other
		_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, fx.doc, foreign)
		if err == nil {
			t.Fatal("foreign tenant recovered the document")
		}
		otherDoc := uuid.New()
		allow(t, exec(pool, `
			INSERT INTO documents.documents (
				id, tenant_id, document_number, document_type, document_status, owner_company_id
			) VALUES ($1,$2,$3,'ETRN','SIGNING_IN_PROGRESS',$4)`,
			otherDoc, fx.tenant, "DOC-"+otherDoc.String()[:8], fx.company))
		_, _, err = repository.NewSigningRepository(pool).RecoverExpiredSigning(ctx, otherDoc, recoverInput(fx, fx.session, 1))
		if err == nil {
			t.Fatal("session recovered a different document")
		}
		status, version := readDocument(t, pool, fx)
		if status != domain.DocumentStatusSigningInProgress || version != 1 {
			t.Fatalf("owner changed status=%s version=%d", status, version)
		}
	})

	t.Run("past_expiry_does_not_create_the_next_session", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		mustRecover(t, pool, fx, fx.session, 1)
		_, err := signingService(pool).CreateSession(ctx, fx.doc, createInput(fx, pastExpiry()))
		if err == nil {
			t.Fatal("past expiry created a session")
		}
		if countSessions(t, pool, fx.doc) != 1 {
			t.Fatal("rejected expiry still inserted a session")
		}
	})

	t.Run("recovery_versus_recovery", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		errs := raceTwo(func() error {
			_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, fx.session, 1))
			return err
		}, func() error {
			_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, fx.session, 1))
			return err
		})
		successes := 0
		for _, err := range errs {
			if err == nil {
				successes++
			}
		}
		if successes != 1 {
			t.Fatalf("successes=%d errs=%v", successes, errs)
		}
		status, version := readDocument(t, pool, fx)
		if status != domain.DocumentStatusReadyForSigning || version != 2 || countSessions(t, pool, fx.doc) != 1 {
			t.Fatalf("status=%s version=%d sessions=%d", status, version, countSessions(t, pool, fx.doc))
		}
	})

	t.Run("recovery_versus_create", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		raceTwo(func() error {
			_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, fx.session, 1))
			return err
		}, func() error {
			_, err := repository.NewSigningRepository(pool).CreateSession(context.Background(), fx.doc, createInput(fx, futureExpiry()))
			return err
		})
		status, _ := readDocument(t, pool, fx)
		active := countActiveSessions(t, pool, fx.doc)
		if active > 1 {
			t.Fatalf("active sessions=%d", active)
		}
		if status == domain.DocumentStatusReadyForSigning && active != 0 {
			t.Fatalf("ready document has active sessions=%d", active)
		}
		if status == domain.DocumentStatusSigningInProgress && active != 1 {
			t.Fatalf("signing document active=%d", active)
		}
		if readSessionState(t, pool, fx.session, fx.tenant).sessionStatus != domain.SigningSessionStatusExpired {
			t.Fatal("source session was reopened")
		}
	})

	t.Run("create_versus_create", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		allow(t, exec(pool, `DELETE FROM documents.signing_sessions WHERE document_id = $1`, fx.doc))
		setDocumentStatus(t, pool, fx, domain.DocumentStatusReadyForSigning)
		errs := raceTwo(func() error {
			_, err := repository.NewSigningRepository(pool).CreateSession(context.Background(), fx.doc, createInput(fx, futureExpiry()))
			return err
		}, func() error {
			_, err := repository.NewSigningRepository(pool).CreateSession(context.Background(), fx.doc, createInput(fx, futureExpiry()))
			return err
		})
		successes := 0
		for _, err := range errs {
			if err == nil {
				successes++
			}
		}
		if successes != 1 || countSessions(t, pool, fx.doc) != 1 || countActiveSessions(t, pool, fx.doc) != 1 {
			t.Fatalf("successes=%d sessions=%d errs=%v", successes, countSessions(t, pool, fx.doc), errs)
		}
	})

	t.Run("recovery_does_not_roll_back_completed_signing", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusInProgress, 1, 0, futureExpiry())
		errs := raceTwo(func() error {
			_, _, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, fx.session, 1))
			return err
		}, func() error {
			_, _, _, err := signingService(pool).AddSignature(context.Background(), fx.session, signatureInput(fx))
			return err
		})
		status, _ := readDocument(t, pool, fx)
		state := readSessionState(t, pool, fx.session, fx.tenant)
		if status != domain.DocumentStatusSigned || state.sessionStatus != domain.SigningSessionStatusCompleted || state.signatures != 1 {
			t.Fatalf("status=%s state=%+v errs=%v", status, state, errs)
		}
	})

	t.Run("lock_wait_past_expiry_does_not_recover_unexpired_session", func(t *testing.T) {
		deadline := time.Now().UTC().Add(200 * time.Millisecond)
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusCreated, 1, 0, &deadline)
		hold, err := pool.Begin(ctx)
		allow(t, err)
		defer hold.Rollback(ctx)
		var locked uuid.UUID
		allow(t, hold.QueryRow(ctx, `SELECT id FROM documents.documents WHERE id = $1 FOR UPDATE`, fx.doc).Scan(&locked))
		done := make(chan error, 1)
		go func() {
			_, _, recErr := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, fx.session, 1))
			done <- recErr
		}()
		time.Sleep(500 * time.Millisecond)
		allow(t, hold.Rollback(ctx))
		if err := <-done; err == nil {
			t.Fatal("recovery accepted a session that was not persisted EXPIRED")
		}
		state := readSessionState(t, pool, fx.session, fx.tenant)
		status, version := readDocument(t, pool, fx)
		if state.sessionStatus != domain.SigningSessionStatusCreated || status != domain.DocumentStatusSigningInProgress || version != 1 {
			t.Fatalf("state=%+v status=%s version=%d", state, status, version)
		}
	})

	t.Run("committed_conflict_leaves_no_partial_recovery", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusExpired, 1, 0, pastExpiry())
		hold, err := pool.Begin(ctx)
		allow(t, err)
		var locked uuid.UUID
		allow(t, hold.QueryRow(ctx, `SELECT id FROM documents.documents WHERE id = $1 FOR UPDATE`, fx.doc).Scan(&locked))
		done := make(chan error, 1)
		go func() {
			_, _, recErr := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, fx.session, 1))
			done <- recErr
		}()
		time.Sleep(200 * time.Millisecond)
		allow(t, hold.QueryRow(ctx, `
			UPDATE documents.documents
			SET document_status = 'SIGNED', version = version + 1, updated_at = now()
			WHERE id = $1
			RETURNING id`, fx.doc).Scan(&locked))
		allow(t, hold.Commit(ctx))
		if err := <-done; err == nil {
			t.Fatal("recovery committed over a newer signed document")
		}
		status, version := readDocument(t, pool, fx)
		if status != domain.DocumentStatusSigned || version != 2 {
			t.Fatalf("status=%s version=%d", status, version)
		}
		if readSessionState(t, pool, fx.session, fx.tenant).sessionStatus != domain.SigningSessionStatusExpired {
			t.Fatal("expired session changed")
		}
	})
}

func mustRecover(t *testing.T, pool *pgxpool.Pool, fx signingFixture, sessionID uuid.UUID, version int) (*domain.Document, *domain.SigningSession) {
	t.Helper()
	doc, session, err := repository.NewSigningRepository(pool).RecoverExpiredSigning(context.Background(), fx.doc, recoverInput(fx, sessionID, version))
	allow(t, err)
	return doc, session
}

func mustCreateSession(t *testing.T, pool *pgxpool.Pool, fx signingFixture) *domain.SigningSession {
	t.Helper()
	session, err := signingService(pool).CreateSession(context.Background(), fx.doc, createInput(fx, futureExpiry()))
	allow(t, err)
	return session
}

func createInput(fx signingFixture, expires *time.Time) domain.CreateSigningSessionInput {
	return domain.CreateSigningSessionInput{TenantID: fx.tenant, RequiredSignersCount: 1, ExpiresAt: expires}
}

func recoverInput(fx signingFixture, sessionID uuid.UUID, version int) domain.RecoverSigningInput {
	return domain.RecoverSigningInput{TenantID: fx.tenant, SigningSessionID: sessionID, ExpectedVersion: version}
}

func expireSession(t *testing.T, pool *pgxpool.Pool, id, tenant uuid.UUID) {
	t.Helper()
	allow(t, exec(pool, `
		UPDATE documents.signing_sessions
		SET status = 'EXPIRED', expires_at = now() - interval '1 minute'
		WHERE id = $1 AND tenant_id = $2`, id, tenant))
}

func setDocumentStatus(t *testing.T, pool *pgxpool.Pool, fx signingFixture, status string) {
	t.Helper()
	allow(t, exec(pool, `UPDATE documents.documents SET document_status = $2 WHERE id = $1 AND tenant_id = $3`, fx.doc, status, fx.tenant))
}

func readDocument(t *testing.T, pool *pgxpool.Pool, fx signingFixture) (string, int) {
	t.Helper()
	var status string
	var version int
	allow(t, pool.QueryRow(context.Background(), `SELECT document_status, version FROM documents.documents WHERE id = $1`, fx.doc).Scan(&status, &version))
	return status, version
}

func readSessionState(t *testing.T, pool *pgxpool.Pool, sessionID, tenant uuid.UUID) signingState {
	t.Helper()
	var state signingState
	err := pool.QueryRow(context.Background(), `
		SELECT s.status, s.completed_signers_count, d.document_status,
			(SELECT count(*) FROM documents.signatures g WHERE g.signing_session_id = s.id)
		FROM documents.signing_sessions s
		JOIN documents.documents d ON d.id = s.document_id
		WHERE s.id = $1 AND s.tenant_id = $2`, sessionID, tenant).Scan(
		&state.sessionStatus, &state.completed, &state.documentStatus, &state.signatures)
	allow(t, err)
	return state
}

func countSessions(t *testing.T, pool *pgxpool.Pool, documentID uuid.UUID) int {
	t.Helper()
	var n int
	allow(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM documents.signing_sessions WHERE document_id = $1`, documentID).Scan(&n))
	return n
}

func countActiveSessions(t *testing.T, pool *pgxpool.Pool, documentID uuid.UUID) int {
	t.Helper()
	var n int
	allow(t, pool.QueryRow(context.Background(), `
		SELECT count(*) FROM documents.signing_sessions
		WHERE document_id = $1 AND status IN ('CREATED', 'IN_PROGRESS')`, documentID).Scan(&n))
	return n
}

func exec(pool *pgxpool.Pool, query string, args ...any) error {
	_, err := pool.Exec(context.Background(), query, args...)
	return err
}

func raceTwo(left, right func() error) [2]error {
	start := make(chan struct{})
	var wg sync.WaitGroup
	var errs [2]error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs[0] = left()
	}()
	go func() {
		defer wg.Done()
		<-start
		errs[1] = right()
	}()
	close(start)
	wg.Wait()
	return errs
}

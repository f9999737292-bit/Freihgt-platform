//go:build integration

package edo03schema

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

type signingFixture struct {
	tenant  uuid.UUID
	other   uuid.UUID
	user    uuid.UUID
	company uuid.UUID
	doc     uuid.UUID
	session uuid.UUID
}

func TestSigningExpiryIsolatedPostgres(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))

	t.Run("expired_created_rejects_and_get_persists_expired", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusCreated, 1, 0, pastExpiry())
		assertRejectedSignature(t, pool, fx)
		assertExpiredAfterRepeatedGet(t, pool, fx)
	})

	t.Run("expired_in_progress_rejects_without_signed_document", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusInProgress, 2, 1, pastExpiry())
		assertRejectedSignature(t, pool, fx)
		state := readSigningState(t, pool, fx)
		if state.sessionStatus != domain.SigningSessionStatusExpired || state.completed != 1 || state.signatures != 0 || state.documentStatus == domain.DocumentStatusSigned {
			t.Fatalf("in-progress expiry state=%+v", state)
		}
	})

	t.Run("expires_at_equal_now_rejects_signature", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusCreated, 1, 0, nil)
		var expires time.Time
		allow(t, pool.QueryRow(ctx, `
			UPDATE documents.signing_sessions
			SET expires_at = now()
			WHERE id = $1 AND tenant_id = $2
			RETURNING expires_at`, fx.session, fx.tenant).Scan(&expires))
		var atDeadline, predicateAccepts bool
		allow(t, pool.QueryRow(ctx, `
			UPDATE documents.signing_sessions
			SET expires_at = now()
			WHERE id = $1 AND tenant_id = $2
			RETURNING expires_at = now(),
				(expires_at IS NULL OR expires_at > now())`, fx.session, fx.tenant).Scan(&atDeadline, &predicateAccepts))
		if !atDeadline || predicateAccepts {
			t.Fatalf("deadline predicate atDeadline=%v accepts=%v", atDeadline, predicateAccepts)
		}
		_, _, _, err := signingService(pool).AddSignature(ctx, fx.session, signatureInput(fx))
		if err == nil {
			t.Fatal("signature accepted at expires_at = now()")
		}
		state := readSigningState(t, pool, fx)
		if state.signatures != 0 || state.completed != 0 || state.documentStatus == domain.DocumentStatusSigned {
			t.Fatalf("boundary write leaked state=%+v", state)
		}
	})

	t.Run("other_tenant_cannot_expire_or_sign", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusCreated, 1, 0, futureExpiry())
		repo := repository.NewSigningRepository(pool)
		foreign, err := repo.MarkSessionExpired(ctx, fx.session, fx.other)
		if err == nil || foreign != nil {
			t.Fatalf("foreign expire returned session=%v err=%v", foreign, err)
		}
		owned := mustLoadSession(t, pool, fx.session, fx.tenant)
		stolen := *owned
		stolen.TenantID = fx.other
		_, _, _, err = repo.AddSignature(ctx, &stolen, signatureInput(fx))
		if err == nil {
			t.Fatal("foreign tenant signature was accepted")
		}
		state := readSigningState(t, pool, fx)
		if state.sessionStatus != domain.SigningSessionStatusCreated || state.completed != 0 || state.signatures != 0 || state.documentStatus != domain.DocumentStatusSigningInProgress {
			t.Fatalf("foreign update changed owner state=%+v", state)
		}
		_, _, _, err = signingService(pool).AddSignature(ctx, fx.session, domain.AddSignatureInput{
			TenantID: fx.other, SignerUserID: fx.user, SignerCompanyID: fx.company,
			SignatureType: domain.SignatureTypeSimpleElectronic,
		})
		if err == nil {
			t.Fatal("service accepted another tenant")
		}
		state = readSigningState(t, pool, fx)
		if state.sessionStatus != domain.SigningSessionStatusCreated || state.signatures != 0 || state.documentStatus != domain.DocumentStatusSigningInProgress {
			t.Fatalf("service foreign call changed owner state=%+v", state)
		}
	})

	t.Run("concurrent_add_and_expire_leave_no_signature", func(t *testing.T) {
		for attempt := 0; attempt < 12; attempt++ {
			fx := seedSigningFixture(t, pool, domain.SigningSessionStatusInProgress, 1, 0, pastExpiry())
			repo := repository.NewSigningRepository(pool)
			loaded := mustLoadSession(t, pool, fx.session, fx.tenant)
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				_, _, _, _ = repo.AddSignature(context.Background(), loaded, signatureInput(fx))
			}()
			go func() {
				defer wg.Done()
				<-start
				_, _ = repo.MarkSessionExpired(context.Background(), fx.session, fx.tenant)
			}()
			close(start)
			wg.Wait()
			state := readSigningState(t, pool, fx)
			if state.signatures != 0 || state.completed != 0 || state.documentStatus == domain.DocumentStatusSigned {
				t.Fatalf("attempt %d torn state=%+v", attempt, state)
			}
			if state.sessionStatus != domain.SigningSessionStatusExpired {
				t.Fatalf("attempt %d status=%s", attempt, state.sessionStatus)
			}
		}
	})

	t.Run("lock_wait_across_deadline_does_not_sign", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusCreated, 1, 0, nil)
		allow(t, pool.QueryRow(ctx, `
			UPDATE documents.signing_sessions
			SET expires_at = clock_timestamp() + interval '200 milliseconds'
			WHERE id = $1 RETURNING expires_at`, fx.session).Scan(new(time.Time)))
		conn, err := pool.Acquire(ctx)
		allow(t, err)
		defer conn.Release()
		tx, err := conn.Begin(ctx)
		allow(t, err)
		_, err = tx.Exec(ctx, `SELECT id FROM documents.signing_sessions WHERE id = $1 FOR UPDATE`, fx.session)
		allow(t, err)

		repo := repository.NewSigningRepository(pool)
		loaded := mustLoadSession(t, pool, fx.session, fx.tenant)
		done := make(chan error, 1)
		go func() {
			_, _, _, err := repo.AddSignature(context.Background(), loaded, signatureInput(fx))
			done <- err
		}()
		time.Sleep(500 * time.Millisecond)
		allow(t, tx.Rollback(ctx))
		err = <-done
		if err == nil {
			t.Fatal("signature committed after the deadline passed while the row lock was held")
		}
		state := readSigningState(t, pool, fx)
		if state.signatures != 0 || state.completed != 0 || state.documentStatus == domain.DocumentStatusSigned {
			t.Fatalf("deadline race wrote state=%+v", state)
		}
	})

	t.Run("concurrent_signatures_before_expiry_keep_counter", func(t *testing.T) {
		fx := seedSigningFixture(t, pool, domain.SigningSessionStatusCreated, 2, 0, futureExpiry())
		secondUser := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO core.users (id, tenant_id, email, full_name)
			VALUES ($1,$2,$3,'Second Signer')`, secondUser, fx.tenant, secondUser.String()+"@example.test")
		allow(t, err)
		svc := signingService(pool)
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		users := []uuid.UUID{fx.user, secondUser}
		for i := range users {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, _, _, errs[i] = svc.AddSignature(context.Background(), fx.session, domain.AddSignatureInput{
					TenantID: fx.tenant, SignerUserID: users[i], SignerCompanyID: fx.company,
					SignatureType: domain.SignatureTypeSimpleElectronic,
				})
			}(i)
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("signer %d: %v", i, err)
			}
		}
		state := readSigningState(t, pool, fx)
		if state.signatures != 2 || state.completed != 2 || state.sessionStatus != domain.SigningSessionStatusCompleted || state.documentStatus != domain.DocumentStatusSigned {
			t.Fatalf("concurrent signatures diverged state=%+v", state)
		}
	})
}

func assertRejectedSignature(t *testing.T, pool *pgxpool.Pool, fx signingFixture) {
	t.Helper()
	_, _, _, err := signingService(pool).AddSignature(context.Background(), fx.session, signatureInput(fx))
	var appErr *apperrors.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation rejection, got %v", err)
	}
	state := readSigningState(t, pool, fx)
	if state.signatures != 0 || state.documentStatus == domain.DocumentStatusSigned {
		t.Fatalf("rejected signature persisted state=%+v", state)
	}
}

func assertExpiredAfterRepeatedGet(t *testing.T, pool *pgxpool.Pool, fx signingFixture) {
	t.Helper()
	svc := signingService(pool)
	first, err := svc.GetSession(context.Background(), fx.session, fx.tenant)
	allow(t, err)
	second, err := svc.GetSession(context.Background(), fx.session, fx.tenant)
	allow(t, err)
	if first.Status != domain.SigningSessionStatusExpired || second.Status != domain.SigningSessionStatusExpired {
		t.Fatalf("get status first=%s second=%s", first.Status, second.Status)
	}
	state := readSigningState(t, pool, fx)
	if state.sessionStatus != domain.SigningSessionStatusExpired {
		t.Fatalf("database status=%s", state.sessionStatus)
	}
}

func signingService(pool *pgxpool.Pool) *service.SigningService {
	repo := repository.NewSigningRepository(pool)
	return service.NewSigningService(repo, repository.NewDocumentRepository(pool))
}

func signatureInput(fx signingFixture) domain.AddSignatureInput {
	return domain.AddSignatureInput{
		TenantID: fx.tenant, SignerUserID: fx.user, SignerCompanyID: fx.company,
		SignatureType: domain.SignatureTypeSimpleElectronic,
	}
}

func pastExpiry() *time.Time {
	value := time.Now().UTC().Add(-time.Minute)
	return &value
}

func futureExpiry() *time.Time {
	value := time.Now().UTC().Add(2 * time.Minute)
	return &value
}

func seedSigningFixture(t *testing.T, pool *pgxpool.Pool, sessionStatus string, required, completed int, expires *time.Time) signingFixture {
	t.Helper()
	ctx := context.Background()
	fx := signingFixture{
		tenant: uuid.New(), other: uuid.New(), user: uuid.New(), company: uuid.New(),
		doc: uuid.New(), session: uuid.New(),
	}
	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,'Tenant A'), ($3,$4,'Tenant B')`,
		fx.tenant, "ta"+fx.tenant.String()[:8], fx.other, "tb"+fx.other.String()[:8])
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, legal_name, company_type)
		VALUES ($1,$2,'Owner','SHIPPER')`, fx.company, fx.tenant)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO core.users (id, tenant_id, email, full_name)
		VALUES ($1,$2,$3,'Signer')`, fx.user, fx.tenant, fx.user.String()+"@example.test")
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status, owner_company_id
		) VALUES ($1,$2,$3,'ETRN','SIGNING_IN_PROGRESS',$4)`,
		fx.doc, fx.tenant, "DOC-"+fx.doc.String()[:8], fx.company)
	allow(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signing_sessions (
			id, tenant_id, document_id, status, required_signers_count, completed_signers_count, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		fx.session, fx.tenant, fx.doc, sessionStatus, required, completed, expires)
	allow(t, err)
	return fx
}

type signingState struct {
	sessionStatus  string
	completed      int
	documentStatus string
	signatures     int
}

func readSigningState(t *testing.T, pool *pgxpool.Pool, fx signingFixture) signingState {
	t.Helper()
	var state signingState
	err := pool.QueryRow(context.Background(), `
		SELECT s.status, s.completed_signers_count, d.document_status,
			(SELECT count(*) FROM documents.signatures g WHERE g.signing_session_id = s.id)
		FROM documents.signing_sessions s
		JOIN documents.documents d ON d.id = s.document_id
		WHERE s.id = $1 AND s.tenant_id = $2`, fx.session, fx.tenant).Scan(
		&state.sessionStatus, &state.completed, &state.documentStatus, &state.signatures)
	allow(t, err)
	return state
}

func mustLoadSession(t *testing.T, pool *pgxpool.Pool, id, tenant uuid.UUID) *domain.SigningSession {
	t.Helper()
	session, err := repository.NewSigningRepository(pool).GetSessionByIDAndTenant(context.Background(), id, tenant)
	allow(t, err)
	return session
}

//go:build integration

package freightpaymentscore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func validateCanonicalObligationPaid(ctx context.Context, pool *pgxpool.Pool, tenantID, registerID uuid.UUID) error {
	var status, original, paid, outstanding string
	err := pool.QueryRow(ctx, `
		SELECT status, original_amount::text, paid_amount::text, outstanding_amount::text
		FROM billing.payment_obligations
		WHERE tenant_id = $1 AND source_type = 'BILLING_REGISTER' AND source_id = $2`,
		tenantID, registerID,
	).Scan(&status, &original, &paid, &outstanding)
	if errors.Is(err, pgx.ErrNoRows) {
		return errObligationNotFound
	}
	if err != nil {
		return err
	}
	if status != "PAID" {
		return errObligationNotPaid
	}
	if paid != original {
		return errObligationIntegrity
	}
	if outstanding != "0" && outstanding != "0.00" {
		return errObligationIntegrity
	}
	return nil
}

var (
	errObligationNotFound  = errors.New("payment obligation not found")
	errObligationNotPaid   = errors.New("payment obligation is not PAID")
	errObligationIntegrity = errors.New("payment obligation integrity violation")
)

func billingSyncPaidHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimSuffix(r.URL.Path, "/")
		const prefix = "/internal/v1/billing-registers/"
		if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/sync-paid") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		registerPart := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/sync-paid")
		registerID, err := uuid.Parse(registerPart)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tenantRaw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
		if tenantRaw == "" {
			var body struct {
				TenantID string `json:"tenant_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			tenantRaw = strings.TrimSpace(body.TenantID)
		}
		tenantID, err := uuid.Parse(tenantRaw)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		var registerStatus string
		if err := pool.QueryRow(ctx, `SELECT status FROM billing.billing_registers WHERE id=$1 AND tenant_id=$2`, registerID, tenantID).Scan(&registerStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if registerStatus == "PAID" || registerStatus == "CLOSED" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"` + registerStatus + `"}`))
			return
		}
		if err := validateCanonicalObligationPaid(ctx, pool, tenantID, registerID); err != nil {
			if errors.Is(err, errObligationNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusConflict)
			return
		}
		if registerStatus != "SIGNED_BY_COUNTERPARTY" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		tag, err := pool.Exec(ctx, `
			UPDATE billing.billing_registers
			SET status='PAID', version = version + 1, updated_at = now()
			WHERE id=$1 AND tenant_id=$2 AND status='SIGNED_BY_COUNTERPARTY'`, registerID, tenantID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			w.WriteHeader(http.StatusConflict)
			return
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO billing.billing_register_audit_events (
				id, tenant_id, register_id, event_type, actor_user_id, actor_company_id, payload
			) VALUES ($1,$2,$3,'MARKED_PAID',$4,$5,$6)`,
			uuid.New(), tenantID, registerID, uuid.Nil, uuid.Nil, []byte(`{"sync_source":"payment_obligation"}`),
		); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"PAID"}`))
	}
}

func countMarkPaidAuditEvents(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM billing.billing_register_audit_events WHERE register_id=$1 AND event_type='MARKED_PAID'`, registerID).Scan(&count); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return count
}

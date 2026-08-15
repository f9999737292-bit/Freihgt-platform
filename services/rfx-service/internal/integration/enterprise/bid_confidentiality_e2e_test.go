//go:build integration

package enterprise

import (
	"context"
	"testing"

	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

func TestBidConfidentialityCarrierScope(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "confidential")

	viewer := fix.Carriers[0].CompanyID
	other := fix.Carriers[1].CompanyID

	visibleCount, err := carrierScopedResponseQuery(ctx, env.pool, fix.EventID, fix.TenantID, viewer)
	if err != nil {
		t.Fatalf("carrier scoped list: %v", err)
	}
	if visibleCount != 1 {
		t.Fatalf("carrier A should see exactly 1 response, got %d", visibleCount)
	}

	canViewOwn, err := carrierCanAccessResponse(ctx, env.pool, fix.Carriers[0].ResponseID, fix.TenantID, viewer)
	if err != nil {
		t.Fatalf("access own response: %v", err)
	}
	if !canViewOwn {
		t.Fatal("carrier A must access own response")
	}

	canViewOther, err := carrierCanAccessResponse(ctx, env.pool, fix.Carriers[1].ResponseID, fix.TenantID, viewer)
	if err != nil {
		t.Fatalf("access other response: %v", err)
	}
	if canViewOther {
		t.Fatal("carrier A must not access carrier B response when scoped by participant_company_id")
	}

	var otherPrice *float64
	err = env.pool.QueryRow(ctx, `
		SELECT price_amount FROM rfx.rfx_responses
		WHERE id = $1 AND tenant_id = $2 AND participant_company_id = $3
	`, fix.Carriers[1].ResponseID, fix.TenantID, viewer).Scan(&otherPrice)
	if err == nil {
		t.Fatal("carrier-scoped price lookup for foreign response should return no rows")
	}

	// Owner/tenant-wide view sees all bids — shipper evaluation path.
	var total int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_responses
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, fix.EventID, fix.TenantID).Scan(&total); err != nil {
		t.Fatalf("tenant-wide response count: %v", err)
	}
	if total != 4 {
		t.Fatalf("evaluator tenant view expects 4 responses, got %d", total)
	}

	t.Run("api_list_boundary", func(t *testing.T) {
		respRepo := repository.NewResponseRevisionRepository(env.pool)
		svc := service.NewResponseBidService(respRepo)
		carrierA := fix.Carriers[0].CompanyID
		bids, err := svc.ListEventBids(ctx, fix.EventID, fix.TenantID, &carrierA)
		if err != nil {
			t.Fatalf("carrier scoped list: %v", err)
		}
		if len(bids) != 1 {
			t.Fatalf("carrier A API list must return exactly 1 bid, got %d", len(bids))
		}
	})

	_ = other
}

func TestBidConfidentialityRevisionHistory(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "conf-rev")

	_, bidID := createFreightRequestBid(t, env.pool, fix.TenantID, fix.Carriers[0].CompanyID, "rev")

	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.bid_revisions (
			tenant_id, bid_id, revision_number, is_active, total_amount, idempotency_key
		) VALUES ($1, $2, 1, true, 1000, $3)
	`, fix.TenantID, bidID, "rev-1-"+bidID.String())
	if err != nil {
		t.Fatalf("insert revision 1: %v", err)
	}

	var foreignCount int
	err = env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.bid_revisions br
		JOIN rfx.bids b ON b.id = br.bid_id
		WHERE b.freight_request_id = (
			SELECT freight_request_id FROM rfx.bids WHERE id = $1
		) AND b.carrier_company_id <> $2
	`, bidID, fix.Carriers[0].CompanyID).Scan(&foreignCount)
	if err != nil {
		t.Fatalf("foreign revision count: %v", err)
	}
	if foreignCount > 0 {
		t.Fatalf("carrier-scoped revision query leaked %d foreign revisions", foreignCount)
	}
}

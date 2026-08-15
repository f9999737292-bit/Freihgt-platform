//go:build integration

package enterprise

import (
	"context"
	"testing"
)

func TestMigration000036Schema(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tables := []string{
		"scoring_templates",
		"scoring_template_versions",
		"tender_evaluations",
		"tender_qualification_results",
		"tender_carrier_scores",
		"allocation_scenarios",
		"allocation_results",
		"quota_balance_policies",
		"quota_balance_targets",
		"quota_balance_positions",
		"quota_ledger_entries",
		"award_proposals",
		"award_proposal_lines",
		"awards",
		"award_transport_orders",
		"bid_revisions",
		"rfx_response_revisions",
	}
	for _, table := range tables {
		exists, err := tableExists(ctx, env.pool, "rfx", table)
		if err != nil {
			t.Fatalf("table %s lookup: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table rfx.%s from migration 000036", table)
		}
	}

	constraints := []string{
		"uq_scoring_template_code",
		"chk_scoring_template_status",
		"uq_scoring_template_version",
		"chk_tender_evaluation_status",
		"uq_tender_qualification",
		"chk_tender_qualification_result",
		"uq_tender_carrier_score",
		"chk_allocation_strategy",
		"chk_allocation_scenario_status",
		"uq_allocation_result",
		"chk_quota_period_type",
		"uq_quota_balance_target",
		"uq_quota_balance_position",
		"chk_quota_balance_status",
		"uq_award_proposal_idempotency",
		"chk_award_proposal_status",
		"uq_award_proposal_line",
		"uq_award_idempotency",
		"chk_award_status",
		"uq_award_transport_order",
		"uq_award_transport_order_pair",
		"uq_bid_revision",
		"uq_bid_revision_idempotency",
		"uq_rfx_response_revision",
		"uq_rfx_response_revision_idempotency",
	}
	for _, name := range constraints {
		exists, err := constraintExists(ctx, env.pool, "rfx", name)
		if err != nil {
			t.Fatalf("constraint %s lookup: %v", name, err)
		}
		if !exists {
			t.Fatalf("expected constraint %s from migration 000036", name)
		}
	}

	indexes := []string{
		"idx_scoring_templates_tenant",
		"idx_scoring_template_versions_tenant",
		"idx_tender_evaluations_event",
		"idx_tender_evaluations_tenant",
		"idx_tender_qualification_evaluation",
		"idx_tender_carrier_scores_evaluation",
		"idx_allocation_scenarios_evaluation",
		"idx_allocation_results_scenario",
		"idx_quota_balance_policies_event",
		"idx_quota_ledger_policy",
		"idx_award_proposals_event",
		"uq_bid_active_revision",
		"uq_rfx_response_active_revision",
		"uq_bid_idempotency",
	}
	for _, name := range indexes {
		exists, err := indexExists(ctx, env.pool, "rfx", name)
		if err != nil {
			t.Fatalf("index %s lookup: %v", name, err)
		}
		if !exists {
			t.Fatalf("expected index %s from migration 000036", name)
		}
	}

	alteredColumns := []struct {
		table  string
		column string
	}{
		{"rfx_responses", "price_amount"},
		{"rfx_responses", "active_revision_number"},
		{"bids", "idempotency_key"},
		{"bids", "active_revision_number"},
		{"freight_requests", "rfx_event_id"},
		{"rfx_events", "scoring_template_version_id"},
		{"rfx_events", "bidding_closed_at"},
	}
	for _, col := range alteredColumns {
		exists, err := columnExists(ctx, env.pool, "rfx", col.table, col.column)
		if err != nil {
			t.Fatalf("column %s.%s lookup: %v", col.table, col.column, err)
		}
		if !exists {
			t.Fatalf("expected column rfx.%s.%s from migration 000036", col.table, col.column)
		}
	}

	var permissionCount int
	err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM core.permissions
		WHERE code IN ('rfx.evaluate', 'rfx.approve_award')
	`).Scan(&permissionCount)
	if err != nil {
		t.Fatalf("permissions lookup: %v", err)
	}
	if permissionCount != 2 {
		t.Fatalf("expected permissions rfx.evaluate and rfx.approve_award, got count=%d", permissionCount)
	}
}

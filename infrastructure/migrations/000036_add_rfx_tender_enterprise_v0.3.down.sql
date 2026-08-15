DELETE FROM core.role_permissions
WHERE permission_id IN (SELECT id FROM core.permissions WHERE code IN ('rfx.evaluate', 'rfx.approve_award'));

DELETE FROM core.permissions WHERE code IN ('rfx.evaluate', 'rfx.approve_award');

ALTER TABLE rfx.rfx_events DROP COLUMN IF EXISTS scoring_template_version_id;
ALTER TABLE rfx.rfx_events DROP COLUMN IF EXISTS bidding_closed_at;
ALTER TABLE rfx.freight_requests DROP COLUMN IF EXISTS rfx_event_id;

DROP INDEX IF EXISTS rfx.uq_bid_idempotency;
ALTER TABLE rfx.bids
    DROP COLUMN IF EXISTS capacity_units,
    DROP COLUMN IF EXISTS transit_hours,
    DROP COLUMN IF EXISTS sla_score_input,
    DROP COLUMN IF EXISTS carrier_kpi_score_input,
    DROP COLUMN IF EXISTS reliability_score_input,
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS active_revision_number;

DROP TABLE IF EXISTS rfx.rfx_response_revisions;
ALTER TABLE rfx.rfx_responses
    DROP COLUMN IF EXISTS price_amount,
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS capacity_units,
    DROP COLUMN IF EXISTS transit_hours,
    DROP COLUMN IF EXISTS sla_score_input,
    DROP COLUMN IF EXISTS carrier_kpi_score_input,
    DROP COLUMN IF EXISTS reliability_score_input,
    DROP COLUMN IF EXISTS active_revision_number;

DROP TABLE IF EXISTS rfx.bid_revisions;
DROP TABLE IF EXISTS rfx.award_transport_orders;
DROP TABLE IF EXISTS rfx.awards;
DROP TABLE IF EXISTS rfx.award_proposal_lines;
DROP TABLE IF EXISTS rfx.award_proposals;
DROP TABLE IF EXISTS rfx.quota_ledger_entries;
DROP TABLE IF EXISTS rfx.quota_balance_positions;
DROP TABLE IF EXISTS rfx.quota_balance_targets;
DROP TABLE IF EXISTS rfx.quota_balance_policies;
DROP TABLE IF EXISTS rfx.allocation_results;
DROP TABLE IF EXISTS rfx.allocation_scenarios;
DROP TABLE IF EXISTS rfx.tender_carrier_scores;
DROP TABLE IF EXISTS rfx.tender_qualification_results;
DROP TABLE IF EXISTS rfx.tender_evaluations;
DROP TABLE IF EXISTS rfx.scoring_template_versions;
DROP TABLE IF EXISTS rfx.scoring_templates;

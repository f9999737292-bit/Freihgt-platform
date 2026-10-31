-- Drops only the budget objects from migration 0002. There is no CASCADE.

DROP POLICY IF EXISTS budget_reservations_select ON autopilot_queue.budget_reservations;
DROP POLICY IF EXISTS budget_ledger_select ON autopilot_queue.budget_tenant_ledger;
DROP TABLE IF EXISTS autopilot_queue.budget_reservations;
DROP TABLE IF EXISTS autopilot_queue.budget_tenant_ledger;

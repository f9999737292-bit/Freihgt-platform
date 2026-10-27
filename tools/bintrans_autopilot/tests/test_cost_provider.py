"""Cost, provider, and spend-cap regressions. Cursor stays mocked."""

from __future__ import annotations

import json
import unittest
from datetime import datetime, timezone

from bintrans_autopilot.budget import assess_budget, budget_windows
from bintrans_autopilot.ci_gate import RELEASE_GATES
from bintrans_autopilot.config import load_config, load_pricing
from bintrans_autopilot.models import (
    PROVIDER_UNVERIFIED,
    UNKNOWN_COST,
    ModelPolicyError,
    _known_rank,
    distinct_provider_status,
    parse_model_catalog,
    resolve_model,
    role_for,
    task_class_for,
    unpaid_model_reason,
)
from test_dispatcher import NOW, FakeCursor, FakeGithub, enabled_config, ready_issue, run_dispatch, state_of
from test_models import AGENT, MAIN, contract, fixture_catalog, routing


def _now():
    return datetime.fromisoformat(NOW.replace("Z", "+00:00"))


class CostProviderTests(unittest.TestCase):
    def test_unknown_cost_is_not_free_and_blocks_cursor_default(self):
        self.assertEqual(UNKNOWN_COST, "UNKNOWN")
        self.assertNotIn(UNKNOWN_COST, {"FREE", "LOW_COST"})
        self.assertIsNone(_known_rank("default"))
        self.assertIsNone(_known_rank("unknown"))
        self.assertNotEqual(_known_rank("unknown"), 0)
        catalog = parse_model_catalog(fixture_catalog())
        choice = resolve_model(routing(), contract(), "implementation", catalog)
        self.assertTrue(choice.omit)
        self.assertEqual(choice.cost_status, UNKNOWN_COST)
        self.assertIsNone(choice.cost_rank)
        self.assertEqual(unpaid_model_reason(load_config(), choice), "unknown_cost")
        priced = routing({
            "tms.ROUTINE": {
                "model_id": "not-returned-by-api",
                "cost_class": "standard",
                "fallback": "cursor_default",
                "fallback_cost_class": "default",
            }
        })
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(priced, contract(), "implementation", catalog)
        self.assertEqual(raised.exception.code, "unknown_cost")
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        result = run_dispatch(github, cursor, enabled_config(test_model_spend_harness=False))
        self.assertEqual(cursor.creates, [])
        self.assertIn("unknown_cost:BT-100", result.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        loaded = load_config()
        self.assertEqual(loaded.pricing.source, "unverified")
        self.assertFalse(loaded.pricing.catalog_is_price_source)
        self.assertEqual(load_pricing({"source": "verified_pricing_api", "billing_class": "low"}).source, "unverified")
        self.assertFalse(loaded.test_model_spend_harness)
        self.assertFalse(loaded.autopilot_enabled)
        self.assertEqual(loaded.budget_max_charged_cents, 0)

    def test_missing_pin_does_not_fall_back_to_cursor_default(self):
        catalog = parse_model_catalog(fixture_catalog())
        state = {
            "model_pin": {
                "omit": False,
                "id": "fixture-removed",
                "params": [],
                "provider": "cursor",
                "cost_class": "standard",
            }
        }
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(routing(), contract(), "implementation", catalog, state)
        self.assertEqual(raised.exception.code, "pinned_model_unavailable")
        issue = ready_issue()
        issue.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps({
            "agent_id": AGENT,
            "allowed_paths": ["services/shipment-service/internal/foo.go"],
            "base_sha": MAIN,
            "branch": "autopilot/bt-100",
            "model_pin": state["model_pin"],
            "run_id": "",
            "status": "CLAIMED",
            "task_id": "BT-100",
        }) + "\n-->\n"
        issue.labels = ["autopilot", "autopilot-contract-approved", "autopilot:CLAIMED"]
        cursor = FakeCursor()
        cursor.model_catalog = fixture_catalog()
        github = FakeGithub([issue])
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("pinned_model_unavailable:BT-100", result.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")

    def test_pin_rejects_an_alias_that_points_at_another_model(self):
        catalog = parse_model_catalog({
            "items": [
                {
                    "id": "fixture-expensive",
                    "aliases": ["fixture-routine-old"],
                    "variants": [{"params": [], "isDefault": True}],
                },
                {
                    "id": "fixture-routine",
                    "variants": [{"params": [{"id": "fast", "value": "true"}], "isDefault": True}],
                },
            ]
        })
        self.assertEqual(catalog.find("fixture-routine-old").id, "fixture-expensive")
        self.assertIsNone(catalog.find_canonical("fixture-routine-old"))
        upgraded = routing({
            "tms.ROUTINE": {
                "model_id": "fixture-expensive",
                "cost_class": "expensive",
                "cost_approved": True,
                "provider": "cursor",
            }
        })
        alias_pin = {
            "omit": False,
            "id": "fixture-routine-old",
            "params": [],
            "cost_class": "standard",
            "cost_status": "UNKNOWN",
        }
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(upgraded, contract(), "implementation", catalog, {"model_pin": alias_pin})
        self.assertEqual(raised.exception.code, "pinned_model_unavailable")
        exact = resolve_model(
            upgraded,
            contract(),
            "implementation",
            catalog,
            {"model_pin": {"omit": False, "id": "fixture-routine", "params": [{"id": "fast", "value": "true"}], "cost_class": "standard"}},
        )
        self.assertEqual(exact.model_id, "fixture-routine")
        self.assertEqual(exact.source, "pinned")
        self.assertEqual(exact.cost_class, "standard")
        self.assertEqual(exact.cost_status, UNKNOWN_COST)
        self.assertEqual(unpaid_model_reason(load_config(), exact), "unknown_cost")
        distinct = routing({
            "independent_qa.INDEPENDENT_QA": {
                "model_id": "fixture-expensive",
                "provider": "anthropic",
                "require_distinct_provider": True,
                "cost_class": "standard",
                "cost_approved": True,
            }
        })
        with self.assertRaises(ModelPolicyError) as provider_error:
            resolve_model(distinct, contract(), "qa", catalog, {"model_pin": alias_pin})
        self.assertEqual(provider_error.exception.code, "provider_unverified")
        issue = ready_issue()
        issue.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps({
            "agent_id": AGENT,
            "allowed_paths": ["services/shipment-service/internal/foo.go"],
            "base_sha": MAIN,
            "branch": "autopilot/bt-100",
            "model_pin": alias_pin,
            "run_id": "",
            "status": "CLAIMED",
            "task_id": "BT-100",
        }) + "\n-->\n"
        issue.labels = ["autopilot", "autopilot-contract-approved", "autopilot:CLAIMED"]
        cursor = FakeCursor()
        cursor.model_catalog = {
            "items": [
                {
                    "id": "fixture-expensive",
                    "aliases": ["fixture-routine-old"],
                    "variants": [{"params": [], "isDefault": True}],
                }
            ]
        }
        github = FakeGithub([issue])
        result = run_dispatch(github, cursor, enabled_config(model_routing=upgraded))
        saved = state_of(github.issues[0])
        self.assertEqual(cursor.creates, [])
        self.assertEqual(cursor.follows, [])
        self.assertEqual(cursor.follow_attempts, 0)
        self.assertEqual(result.launched, [])
        self.assertIn("pinned_model_unavailable:BT-100", result.skipped)
        self.assertEqual(saved["status"], "NEEDS_CONTROLLER")
        self.assertEqual(saved["model_pin"]["id"], "fixture-routine-old")
        self.assertEqual(saved["last_error"], "pinned_model_unavailable")

    def test_expensive_fallback_stays_rejected(self):
        catalog = parse_model_catalog(fixture_catalog())
        policy = routing({
            "tms.ROUTINE": {
                "model_id": "not-returned-by-api",
                "cost_class": "standard",
                "fallback": "fixture-expensive",
                "fallback_cost_class": "expensive",
                "fallback_cost_approved": False,
            }
        })
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(policy, contract(), "implementation", catalog)
        self.assertEqual(raised.exception.code, "expensive_fallback_rejected")

    def test_config_provider_string_is_not_independence(self):
        catalog = parse_model_catalog({
            "items": [
                {"id": "fixture-routine", "provider": "anthropic", "priceCents": 1, "variants": [{"params": [], "isDefault": True}]},
                {"id": "fixture-qa", "provider": "other", "variants": [{"params": [], "isDefault": True}]},
            ]
        })
        self.assertFalse(hasattr(catalog.find("fixture-routine"), "provider"))
        spoofed = routing({
            "independent_qa.INDEPENDENT_QA": {
                "model_id": "fixture-qa",
                "provider": "anthropic",
                "require_distinct_provider": True,
                "cost_class": "standard",
                "cost_approved": True,
            }
        })
        state = {"model_pin": {"omit": False, "id": "fixture-routine", "params": [], "provider": "cursor", "cost_class": "standard"}}
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(spoofed, contract(), "qa", catalog, state)
        self.assertEqual(raised.exception.code, "provider_unverified")
        self.assertEqual(distinct_provider_status("fixture-routine", "fixture-qa"), PROVIDER_UNVERIFIED)
        self.assertNotEqual(distinct_provider_status("fixture-routine", "fixture-qa"), "DISTINCT")
        open_route = routing({
            "independent_qa.INDEPENDENT_QA": {
                "model_id": "fixture-qa",
                "provider": "anthropic",
                "require_distinct_provider": False,
                "cost_class": "standard",
                "cost_approved": True,
            }
        })
        choice = resolve_model(open_route, contract(), "qa", catalog, state)
        self.assertEqual(choice.model_id, "fixture-qa")
        self.assertEqual(choice.provider_status, PROVIDER_UNVERIFIED)
        self.assertEqual(choice.verified_provider, "")
        self.assertNotIn("anthropic", choice.pin().values())

    def test_finance_role_uses_service_boundaries(self):
        billing = contract(MODULE="billing-register", ALLOWED_PATHS="- services/billing-register-service/internal/invoice.go")
        shipment = contract(MODULE="shipment", ALLOWED_PATHS="- services/shipment-service/internal/billing_export.go")
        named = contract(MODULE="tms", ALLOWED_PATHS="- services/shipment-service/internal/billing-register.go")
        payment = contract(MODULE="payment", ALLOWED_PATHS="- services/payment-service/internal/charge.go")
        nlo = contract(MODULE="nlo", ALLOWED_PATHS="- services/network-optimizer-service/internal/route.go")
        analytics = contract(MODULE="analytics", ALLOWED_PATHS="- services/analytics-service/internal/score.go")
        tms = contract(MODULE="shipment", ALLOWED_PATHS="- services/shipment-service/internal/foo.go")
        self.assertEqual(role_for(billing), "finance")
        self.assertEqual(task_class_for(billing, "implementation"), "FINANCIAL_CRITICAL")
        self.assertEqual(role_for(payment), "finance")
        self.assertEqual(task_class_for(payment, "implementation"), "FINANCIAL_CRITICAL")
        self.assertEqual(role_for(shipment), "tms")
        self.assertNotEqual(task_class_for(shipment, "implementation"), "FINANCIAL_CRITICAL")
        self.assertEqual(role_for(named), "tms")
        self.assertEqual(task_class_for(named, "implementation"), "ROUTINE")
        self.assertEqual(role_for(nlo), "nlo_optimizer")
        self.assertEqual(role_for(analytics), "analytics")
        self.assertEqual(role_for(tms), "tms")

    def test_admin_api_plan_and_lag_fail_closed(self):
        now = _now()
        windows = budget_windows(now)
        ultra = FakeCursor()
        ultra.has_admin_key = False
        decision = assess_budget(enabled_config(cursor_plan="ultra"), ultra, windows, now, 0)
        self.assertEqual(decision.reason, "personal_ultra_no_team_admin_api")
        self.assertEqual(ultra.spend_calls, [])
        unknown_plan = FakeCursor()
        decision = assess_budget(
            enabled_config(test_budget_harness=False, cursor_plan=""),
            unknown_plan,
            windows,
            now,
            0,
        )
        self.assertEqual(decision.reason, "cursor_plan_unverified")
        self.assertEqual(unknown_plan.spend_calls, [])
        lagged = FakeCursor()
        lagged.spend_events_lagged = True
        decision = assess_budget(enabled_config(cursor_plan="ultra"), lagged, windows, now, 0)
        self.assertEqual(decision.reason, "spend_observation_stale")
        self.assertTrue(decision.preserve_task_state)
        self.assertFalse(decision.allowed)
        cap = assess_budget(
            enabled_config(test_budget_harness=False, cursor_plan="ultra"),
            FakeCursor(),
            windows,
            now,
            0,
        )
        self.assertEqual(cap.reason, "provider_spend_cap_unconfirmed")
        self.assertTrue(cap.preserve_task_state)

    def test_reserve_blocks_several_agents_and_restart_does_not_create_again(self):
        now = _now()
        windows = budget_windows(now)
        decision = assess_budget(
            enabled_config(
                budget_max_charged_cents=0,
                budget_max_charged_cents_daily=60,
                budget_max_charged_cents_monthly=10_000_000,
                budget_launch_reserve_cents=10,
                budget_reserve_per_active_agent_cents=20,
            ),
            FakeCursor(),
            windows,
            now,
            3,
        )
        self.assertEqual(decision.reason, "daily_budget_exceeded")
        self.assertEqual(decision.charged_cents, 70)
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        again = run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        self.assertIn("agent_still_active:BT-100", again.skipped)
        self.assertEqual(RELEASE_GATES, ("scripts-check",))


if __name__ == "__main__":
    unittest.main()

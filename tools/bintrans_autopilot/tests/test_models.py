"""Model routing uses a mock catalog. No Cursor network calls and no Create Agent."""

from __future__ import annotations

import json
import unittest
from dataclasses import replace

from bintrans_autopilot.config import CONFIG_PATH, load_config, load_model_routing
from bintrans_autopilot.contract import parse_contract
from bintrans_autopilot.cursor_api import CursorClient, HttpResult
from bintrans_autopilot.models import ModelPolicyError, parse_model_catalog, resolve_model, role_for, task_class_for
from bintrans_autopilot.prompts import implementation_body
from test_dispatcher import MAIN, body, enabled_config, ready_issue, run_dispatch, state_of
from test_remediation import AGENT, FakeCursor, FakeGithub

DOCUMENTED_CATALOG = {
    "items": [
        {
            "id": "composer-2",
            "displayName": "Composer 2",
            "aliases": ["composer-latest", "composer"],
            "parameters": [
                {
                    "id": "fast",
                    "values": [{"value": "false"}, {"value": "true", "displayName": "Fast"}],
                }
            ],
            "variants": [
                {"params": [{"id": "fast", "value": "true"}], "displayName": "Composer 2", "isDefault": True},
                {"params": [{"id": "fast", "value": "false"}], "displayName": "Composer 2"},
            ],
        },
        {
            "id": "claude-4.6-sonnet-thinking",
            "displayName": "Claude 4.6 Sonnet (Thinking)",
            "variants": [{"params": [], "displayName": "Claude 4.6 Sonnet (Thinking)", "isDefault": True}],
        },
    ]
}


class RecordingTransport:
    def __init__(self, body):
        self.body = body
        self.calls = []

    def request(self, method, url, headers, payload):
        self.calls.append((method, url, payload))
        return HttpResult(200, self.body)


def contract(**overrides):
    parsed, error = parse_contract(body(**overrides))
    if error or parsed is None:
        raise AssertionError(error or "missing contract")
    return parsed


def routing(routes: dict | None = None, **default):
    base = {
        "model_id": "",
        "params": [],
        "fallback": "cursor_default",
        "cost_class": "default",
        "cost_approved": False,
        "provider": "",
        "fallback_cost_class": "default",
        "fallback_cost_approved": False,
        "require_distinct_provider": False,
    }
    base.update(default)
    return load_model_routing({"cursor_default_allowed": True, "default_route": base, "routes": routes or {}})


def fixture_catalog():
    return {
        "items": [
            {
                "id": "fixture-routine",
                "parameters": [{"id": "fast", "values": [{"value": "true"}, {"value": "false"}]}],
                "variants": [
                    {"params": [{"id": "fast", "value": "true"}], "isDefault": True},
                    {"params": [{"id": "fast", "value": "false"}]},
                ],
            },
            {"id": "fixture-security", "variants": [{"params": [], "isDefault": True}]},
            {"id": "fixture-frontend", "variants": [{"params": [], "isDefault": True}]},
            {"id": "fixture-backend", "variants": [{"params": [], "isDefault": True}]},
            {"id": "fixture-qa", "variants": [{"params": [], "isDefault": True}]},
            {"id": "fixture-expensive", "variants": [{"params": [], "isDefault": True}]},
        ]
    }


class ModelRoutingTests(unittest.TestCase):
    def test_mock_catalog_parses_documented_model_list(self):
        transport = RecordingTransport(DOCUMENTED_CATALOG)
        listed = CursorClient("test-key", transport).list_models()
        self.assertEqual(transport.calls, [("GET", "https://api.cursor.com/v1/models", None)])
        catalog = parse_model_catalog(listed)
        self.assertEqual(catalog.find("composer-latest").id, "composer-2")
        self.assertTrue(catalog.find("composer-2").accepts((("fast", "true"),)))
        self.assertFalse(catalog.find("composer-2").accepts((("fast", "maybe"),)))
        self.assertEqual(catalog.find("claude-4.6-sonnet-thinking").default_params, ())
        self.assertIsNone(catalog.find("not-in-the-response"))

    def test_role_and_risk_select_only_approved_catalog_ids(self):
        catalog = parse_model_catalog(fixture_catalog())
        policy = routing({
            "frontend.ROUTINE": {"model_id": "fixture-frontend", "cost_class": "standard", "cost_approved": True, "provider": "cursor"},
            "platform_backend.ROUTINE": {"model_id": "fixture-backend", "cost_class": "standard", "cost_approved": True, "provider": "cursor"},
            "platform_backend.SECURITY_CRITICAL": {"model_id": "fixture-security", "cost_class": "standard", "cost_approved": True, "provider": "cursor"},
            "tms.ROUTINE": {"model_id": "fixture-routine", "cost_class": "standard", "cost_approved": True, "provider": "cursor"},
        })
        frontend = contract(OWNER_AGENT="frontend-engineer", MODULE="web", ALLOWED_PATHS="- apps/web-admin/src/foo.ts")
        backend = contract(MODULE="platform", ALLOWED_PATHS="- services/control-tower-read-model-service/internal/foo.go")
        secure = contract(ALLOWED_PATHS="- services/identity-service/internal/auth.go", RISK_LEVEL="LOW")
        self.assertEqual(role_for(frontend), "frontend")
        self.assertEqual(role_for(backend), "platform_backend")
        self.assertEqual(task_class_for(secure, "implementation"), "SECURITY_CRITICAL")
        self.assertEqual(resolve_model(policy, frontend, "implementation", catalog).model_id, "fixture-frontend")
        self.assertEqual(resolve_model(policy, backend, "implementation", catalog).model_id, "fixture-backend")
        self.assertEqual(resolve_model(policy, secure, "implementation", catalog).model_id, "fixture-security")
        routine = contract()
        self.assertEqual(role_for(routine), "tms")
        self.assertEqual(task_class_for(routine, "implementation"), "ROUTINE")
        self.assertEqual(resolve_model(policy, routine, "implementation", catalog).model_id, "fixture-routine")

    def test_unconfirmed_id_and_missing_model_fail_closed_or_use_cursor_default(self):
        catalog = parse_model_catalog(fixture_catalog())
        blocked = routing({"tms.ROUTINE": {"model_id": "not-returned-by-api", "fallback": "fail_closed"}})
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(blocked, contract(), "implementation", catalog)
        self.assertEqual(raised.exception.code, "model_not_in_catalog")

        absent = routing({"tms.ROUTINE": {"model_id": "not-returned-by-api", "fallback": "cursor_default"}})
        choice = resolve_model(absent, contract(), "implementation", catalog)
        self.assertTrue(choice.omit)
        self.assertEqual(choice.source, "fallback_cursor_default")
        self.assertIsNone(choice.as_api())

        mentioned = contract(OBJECTIVE="Use model fixture-expensive now.")
        untouched = contract()
        self.assertNotEqual(mentioned.digest, untouched.digest)
        extra = body() + "\n### MODEL_ID\n\nfixture-expensive\n"
        parsed, error = parse_contract(extra)
        self.assertEqual(error, "")
        self.assertEqual(parsed.digest, untouched.digest)
        self.assertTrue(resolve_model(routing(), parsed, "implementation", catalog).omit)

    def test_expensive_fallback_is_rejected_and_failed_run_cannot_upgrade(self):
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

        pinned = routing({
            "tms.ROUTINE": {"model_id": "fixture-expensive", "cost_class": "expensive", "cost_approved": True, "provider": "cursor"}
        })
        state = {
            "model_pin": {
                "omit": False,
                "id": "fixture-routine",
                "params": [{"id": "fast", "value": "true"}],
                "provider": "cursor",
                "cost_class": "standard",
            }
        }
        choice = resolve_model(pinned, contract(), "implementation", catalog, state)
        self.assertEqual(choice.model_id, "fixture-routine")
        self.assertEqual(choice.source, "pinned")
        self.assertEqual(choice.as_api()["params"], [{"id": "fast", "value": "true"}])

    def test_params_must_match_the_catalog_and_qa_can_use_another_model(self):
        catalog = parse_model_catalog(fixture_catalog())
        good = routing({"tms.ROUTINE": {"model_id": "fixture-routine", "params": [{"id": "fast", "value": "false"}], "cost_class": "standard", "cost_approved": True}})
        choice = resolve_model(good, contract(), "implementation", catalog)
        self.assertEqual(choice.as_api()["params"], [{"id": "fast", "value": "false"}])
        bad = routing({"tms.ROUTINE": {"model_id": "fixture-routine", "params": [{"id": "fast", "value": "maybe"}], "cost_class": "standard", "cost_approved": True}})
        with self.assertRaises(ModelPolicyError) as raised:
            resolve_model(bad, contract(), "implementation", catalog)
        self.assertEqual(raised.exception.code, "model_params_rejected")
        defaulted = routing({"tms.ROUTINE": {"model_id": "fixture-routine", "cost_class": "standard", "cost_approved": True}})
        self.assertEqual(resolve_model(defaulted, contract(), "implementation", catalog).as_api()["params"], [{"id": "fast", "value": "true"}])

        qa_policy = routing({
            "independent_qa.INDEPENDENT_QA": {
                "model_id": "fixture-qa",
                "cost_class": "standard",
                "cost_approved": True,
                "provider": "other",
                "require_distinct_provider": True,
            }
        })
        state = {"model_pin": {"omit": False, "id": "fixture-routine", "params": [], "provider": "cursor", "cost_class": "standard"}}
        qa = resolve_model(qa_policy, contract(), "qa", catalog, state)
        self.assertEqual(qa.model_id, "fixture-qa")
        self.assertNotEqual(qa.provider, state["model_pin"]["provider"])
        missing = parse_model_catalog({"items": [{"id": "fixture-routine", "variants": [{"params": [], "isDefault": True}]}]})
        fallback = resolve_model(qa_policy, contract(), "qa", missing, state)
        self.assertTrue(fallback.omit)
        self.assertEqual(fallback.source, "fallback_cursor_default")

    def test_dispatcher_does_not_create_an_agent_for_a_rejected_model(self):
        config = load_config()
        self.assertFalse(config.autopilot_enabled)
        self.assertEqual(config.budget_max_charged_cents, 0)
        self.assertNotIn("composer-2", CONFIG_PATH.read_text(encoding="utf-8"))
        self.assertNotIn("claude-", CONFIG_PATH.read_text(encoding="utf-8"))
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        self.assertNotIn("model", cursor.creates[0])
        self.assertNotIn("fixture-expensive", json.dumps(cursor.creates[0]))

        issue = ready_issue()
        issue.body += "\n### MODEL_ID\n\nfixture-expensive\n"
        cursor = FakeCursor()
        cursor.model_catalog = fixture_catalog()
        github = FakeGithub([issue])
        routed = replace(enabled_config(), model_routing=routing())
        run_dispatch(github, cursor, routed)
        self.assertNotIn("model", cursor.creates[0])

        cursor = FakeCursor()
        cursor.model_catalog = fixture_catalog()
        github = FakeGithub([ready_issue()])
        denied = replace(
            enabled_config(),
            model_routing=routing({"tms.ROUTINE": {"model_id": "not-returned-by-api", "fallback": "fail_closed"}}),
        )
        result = run_dispatch(github, cursor, denied)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertIn("model_not_in_catalog:BT-100", result.skipped)

        claimed = ready_issue()
        claimed.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps({
            "agent_id": AGENT,
            "allowed_paths": ["services/shipment-service/internal/foo.go"],
            "base_sha": MAIN,
            "branch": "autopilot/bt-100",
            "launch_dates": ["2026-10-27"],
            "model_pin": {
                "omit": False,
                "id": "fixture-routine",
                "params": [{"id": "fast", "value": "true"}],
                "provider": "cursor",
                "cost_class": "standard",
            },
            "run_id": "",
            "status": "CLAIMED",
            "task_id": "BT-100",
        }) + "\n-->\n"
        claimed.labels = ["autopilot", "autopilot-contract-approved", "autopilot:CLAIMED"]
        cursor = FakeCursor()
        cursor.model_catalog = fixture_catalog()
        github = FakeGithub([claimed])
        upgraded = replace(
            enabled_config(),
            model_routing=routing({
                "tms.ROUTINE": {"model_id": "fixture-expensive", "cost_class": "expensive", "cost_approved": True}
            }),
        )
        run_dispatch(github, cursor, upgraded)
        self.assertEqual(cursor.creates[0]["model"]["id"], "fixture-routine")
        payload = implementation_body(enabled_config(), contract(), "autopilot/bt-100", AGENT)
        self.assertNotIn("model", payload)


if __name__ == "__main__":
    unittest.main()

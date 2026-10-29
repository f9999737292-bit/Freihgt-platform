"""Historical usage journal. Missing amounts stay null."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from bintrans_autopilot.config import load_config
from bintrans_autopilot.usage_journal import JOURNAL_PATH, UsageJournalError, load_records, record_usage

PILOT = {
    "agent_id": "bc-33a0edbc-8fed-47fa-85b7-cc139e900375",
    "run_id": "run-a51658e1-665d-4f7c-9135-c0d720b2210b",
    "input_tokens": 39535,
    "output_tokens": 12668,
    "cache_read_tokens": 1144992,
    "cache_write_tokens": 0,
    "total_tokens": 1197195,
    "raw_cost_cents": "86.83995",
    "charged_cents": "0",
    "source": "GET /v1/agents/{id}/usage",
    "source_http_status": 200,
}


class UsageJournalTests(unittest.TestCase):
    def test_committed_pilot_record_is_the_only_entry(self) -> None:
        records = load_records(JOURNAL_PATH)
        self.assertEqual(len(records), 1)
        stored = records[0]
        self.assertEqual(stored.agent_id, PILOT["agent_id"])
        self.assertEqual(stored.run_id, PILOT["run_id"])
        self.assertEqual(stored.input_tokens, 39535)
        self.assertEqual(stored.output_tokens, 12668)
        self.assertEqual(stored.cache_read_tokens, 1144992)
        self.assertEqual(stored.cache_write_tokens, 0)
        self.assertEqual(stored.total_tokens, 1197195)
        self.assertEqual(stored.raw_cost_cents, "86.83995")
        self.assertEqual(stored.charged_cents, "0")
        self.assertEqual(stored.source_http_status, 200)
        text = JOURNAL_PATH.read_text(encoding="utf-8")
        self.assertNotIn("cursor_", text)
        self.assertNotIn("eyJ", text)
        self.assertEqual(load_config().budget_max_charged_cents, 0)

    def test_same_agent_and_run_is_not_duplicated(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "usage_journal.json"
            path.write_bytes(JOURNAL_PATH.read_bytes())
            again = record_usage(path, PILOT)
            self.assertEqual(again.status, "unchanged")
            self.assertEqual(len(load_records(path)), 1)
            with self.assertRaises(UsageJournalError) as raised:
                record_usage(path, {**PILOT, "charged_cents": "1"})
            self.assertEqual(raised.exception.code, "conflict")
            self.assertEqual(len(json.loads(path.read_text(encoding="utf-8"))["records"]), 1)

    def test_unknown_future_spend_stays_null(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "usage_journal.json"
            raw = {
                "agent_id": "bc-11111111-1111-1111-1111-111111111111",
                "run_id": "run-22222222-2222-2222-2222-222222222222",
                "input_tokens": None,
                "output_tokens": None,
                "cache_read_tokens": None,
                "cache_write_tokens": None,
                "total_tokens": None,
                "raw_cost_cents": None,
                "charged_cents": None,
                "source": "GET /v1/agents/{id}/usage",
                "source_http_status": 200,
            }
            self.assertEqual(record_usage(path, raw).status, "created")
            stored = load_records(path)[0]
            self.assertIsNone(stored.raw_cost_cents)
            self.assertIsNone(stored.charged_cents)
            self.assertIsNone(stored.total_tokens)
            body = path.read_text(encoding="utf-8")
            self.assertIn('"charged_cents": null', body)
            self.assertIn('"raw_cost_cents": null', body)

    def test_secret_material_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "usage_journal.json"
            with self.assertRaises(UsageJournalError) as raised:
                record_usage(path, {**PILOT, "source": "cursor_example"})
            self.assertEqual(raised.exception.code, "secret_rejected")
            self.assertFalse(path.exists())

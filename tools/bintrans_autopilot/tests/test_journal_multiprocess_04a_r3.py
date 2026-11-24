"""Two processes append to one journal. The chain stays linear."""

from __future__ import annotations

import multiprocessing
import tempfile
import unittest
from pathlib import Path

from bintrans_autopilot.audit_journal import AuditJournal
from bintrans_autopilot.testing import append_packet_batch

_GENESIS = "0" * 64


class JournalMultiprocessTests(unittest.TestCase):
    def test_two_processes_append_one_chain(self) -> None:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        path = Path(directory.name) / "shared.jsonl"
        AuditJournal.create(path)
        context = multiprocessing.get_context("spawn")
        first = context.Process(target=append_packet_batch, args=(str(path), "a", 20))
        second = context.Process(target=append_packet_batch, args=(str(path), "b", 20))
        first.start()
        second.start()
        first.join(60)
        second.join(60)
        self.assertFalse(first.is_alive())
        self.assertFalse(second.is_alive())
        self.assertEqual(first.exitcode, 0)
        self.assertEqual(second.exitcode, 0)
        records = AuditJournal(path).verify()
        self.assertEqual(len(records), 40)
        prev = _GENESIS
        labels = set()
        for record in records:
            self.assertEqual(record["prev"], prev)
            prev = record["hash"]
            labels.add(record["body"]["label"])
        self.assertEqual(labels, {"a", "b"})
        self.assertTrue(path.exists())


if __name__ == "__main__":
    unittest.main()

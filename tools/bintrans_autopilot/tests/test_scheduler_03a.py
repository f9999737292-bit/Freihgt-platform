"""Dry-run queue and scheduler. No Cursor client and no merge."""

from __future__ import annotations

import tempfile
import threading
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

from bintrans_autopilot.scheduler import Policy, Scheduler, SchedulerModeError, dry_run_main
from bintrans_autopilot.task_queue import QueueError, Task, TaskStore

BASE = "2a9c656ca7fce67bf80a7dccfe8114834b08388b"
SIDE_EFFECTS = {"merge", "deploy", "staging_change", "production_change", "cloud_agent_create"}


class Clock:
    def __init__(self) -> None:
        self.moment = datetime(2026, 10, 29, 12, 0, tzinfo=timezone.utc)

    def now(self) -> datetime:
        return self.moment

    def advance(self, seconds: int) -> None:
        self.moment += timedelta(seconds=seconds)


def policy(**overrides) -> Policy:
    raw = dict(
        mode="DRY_RUN",
        autopilot_enabled=False,
        paid_automation_enabled=False,
        launch_budget_cents=0,
        observed_charged_cents=None,
        approved_prefixes=("docs/",),
        forbidden_prefixes=(".env", "ops/"),
    )
    raw.update(overrides)
    return Policy(**raw)


def task(**overrides) -> Task:
    raw = dict(
        id="queue-1",
        agent="H",
        priority=10,
        project="bintrans",
        task_type="docs",
        allowed_paths=("docs/engineering/autopilot/SCHEDULER_0_3A.md",),
        base_sha=BASE,
        acceptance=("dry-run only",),
        requested_actions=(),
        depends_on=(),
        max_budget_cents=0,
        requires_cloud_agent=False,
    )
    raw.update(overrides)
    return Task(**raw)


class SchedulerTests(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.clock = Clock()
        self.store = TaskStore(Path(self.directory.name) / "queue.json")

    def tearDown(self) -> None:
        self.directory.cleanup()

    def scheduler(self, name: str = "s1", **overrides) -> Scheduler:
        return Scheduler(self.store, name, policy(**overrides), self.clock.now, lease_seconds=30)

    def test_two_schedulers_cannot_claim_one_task(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        barrier = threading.Barrier(2)
        results = []

        def run(name: str) -> None:
            barrier.wait()
            results.append(self.scheduler(name).tick(stop_after="claimed"))

        threads = [threading.Thread(target=run, args=(name,)) for name in ("s1", "s2")]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        claimed = [item for item in results if item.reason == "claimed"]
        self.assertEqual(len(claimed), 1)
        self.assertEqual(claimed[0].task_id, "queue-1")
        self.assertEqual(self.store.get("queue-1", self.clock.now()).status, "CLAIMED")

    def test_restart_does_not_duplicate(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        reopened = TaskStore(self.store.path)
        with self.assertRaises(QueueError) as raised:
            reopened.enqueue(task(), self.clock.now(), "s2")
        self.assertEqual(raised.exception.code, "duplicate_task")
        self.scheduler().tick()
        self.scheduler("s2").tick()
        tasks = reopened.list_tasks(self.clock.now())
        self.assertEqual([item.id for item in tasks], ["queue-1"])
        self.assertEqual(tasks[0].status, "REVIEW_REQUIRED")

    def test_expired_lease_recovers_and_old_owner_cannot_finish(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        first = self.scheduler("s1")
        claimed = first.tick(stop_after="claimed")
        self.assertEqual(claimed.reason, "claimed")
        self.clock.advance(31)
        second = self.scheduler("s2")
        recovered = second.tick(stop_after="claimed")
        self.assertEqual(recovered.reason, "claimed")
        self.assertEqual(recovered.status, "CLAIMED")
        lost = first.finish("queue-1")
        self.assertFalse(lost.ok)
        self.assertEqual(lost.reason, "lease_lost")
        stored = self.store.get("queue-1", self.clock.now())
        self.assertEqual(stored.status, "CLAIMED")
        self.assertEqual(stored.lease_owner, "s2")
        self.assertIn("lease_expired", [entry["reason"] for entry in stored.history])

    def test_forbidden_action_is_not_executed(self) -> None:
        self.store.enqueue(task(id="merge-1", requested_actions=("merge", "deploy")), self.clock.now(), "s1")
        result = self.scheduler().tick()
        self.assertEqual(result.reason, "forbidden_action")
        self.assertFalse(result.merge_attempted)
        self.assertFalse(result.deploy_attempted)
        self.assertEqual(result.cloud_agents_started, 0)
        stored = self.store.get("merge-1", self.clock.now())
        self.assertEqual(stored.status, "BLOCKED")
        self.assertNotIn(stored.status, {"RUNNING", "COMPLETED"})
        actions = {entry["action"] for entry in self.store.journal(self.clock.now())}
        self.assertTrue(actions.isdisjoint(SIDE_EFFECTS))

    def test_zero_budget_blocks_paid_launch_without_inventing_spend(self) -> None:
        self.store.enqueue(
            task(id="paid-1", requires_cloud_agent=True, max_budget_cents=0),
            self.clock.now(),
            "s1",
        )
        result = self.scheduler(paid_automation_enabled=True, observed_charged_cents=None).tick()
        self.assertEqual(result.reason, "paid_budget_zero")
        self.assertEqual(result.cloud_agents_started, 0)
        stored = self.store.get("paid-1", self.clock.now())
        self.assertEqual(stored.status, "BLOCKED")
        self.assertEqual(stored.max_budget_cents, 0)
        self.assertIsNone(self.scheduler().policy.observed_charged_cents)

    def test_unknown_budget_and_unknown_spend_are_not_zero(self) -> None:
        self.store.enqueue(task(id="unknown-budget", max_budget_cents=None), self.clock.now(), "s1")
        blocked = self.scheduler().tick()
        self.assertEqual(blocked.reason, "unknown_budget")
        stored = self.store.get("unknown-budget", self.clock.now())
        self.assertIsNone(stored.max_budget_cents)
        self.store.enqueue(
            task(id="unknown-spend", requires_cloud_agent=True, max_budget_cents=1),
            self.clock.now(),
            "s1",
        )
        spend = self.scheduler(
            paid_automation_enabled=True,
            launch_budget_cents=1,
            observed_charged_cents=None,
        ).tick()
        self.assertEqual(spend.reason, "unknown_spend")
        self.assertEqual(spend.cloud_agents_started, 0)
        self.assertEqual(self.store.get("unknown-spend", self.clock.now()).max_budget_cents, 1)
        self.assertIsNone(self.scheduler().policy.observed_charged_cents)

    def test_failed_ci_is_not_pass_and_does_not_complete(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        scheduler = self.scheduler()
        self.assertEqual(scheduler.tick().status, "REVIEW_REQUIRED")
        recorded = scheduler.record_ci("queue-1", "failure")
        self.assertEqual(recorded.reason, "FAIL")
        stored = self.store.get("queue-1", self.clock.now())
        self.assertNotEqual(stored.ci_verdict, "PASS")
        self.assertNotEqual(stored.status, "COMPLETED")
        accepted = scheduler.accept_review("queue-1")
        self.assertEqual(accepted.reason, "ci_not_pass")
        self.assertEqual(self.store.get("queue-1", self.clock.now()).status, "BLOCKED")

    def test_dependency_waits_until_completed(self) -> None:
        self.store.enqueue(task(id="parent", priority=5), self.clock.now(), "s1")
        self.store.enqueue(
            task(id="child", priority=1, depends_on=("parent",)),
            self.clock.now(),
            "s1",
        )
        result = self.scheduler().tick()
        self.assertEqual(result.task_id, "parent")
        self.assertEqual(self.store.get("child", self.clock.now()).status, "CREATED")

    def test_live_mode_is_refused(self) -> None:
        with self.assertRaises(SchedulerModeError):
            policy(mode="LIVE")

    def test_dry_run_command_stays_closed(self) -> None:
        self.assertEqual(dry_run_main([]), 0)


if __name__ == "__main__":
    unittest.main()

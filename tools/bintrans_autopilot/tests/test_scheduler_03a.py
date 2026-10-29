"""Dry-run queue and scheduler. No Cursor client and no merge."""

from __future__ import annotations

import multiprocessing
import os
import subprocess
import tempfile
import threading
import time
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest import mock

from bintrans_autopilot.scheduler import (
    Policy,
    Scheduler,
    SchedulerModeError,
    dry_run_main,
    resolve_task_path,
)
from bintrans_autopilot.task_queue import FileLock, QueueError, Task, TaskStore

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


def _claim_in_process(store_path: str, name: str, stamp: str) -> None:
    moment = datetime(2026, 10, 29, 12, 0, tzinfo=timezone.utc)
    store = TaskStore(Path(store_path))
    result = Scheduler(store, name, policy(), lambda: moment, lease_seconds=30).tick(stop_after="claimed")
    Path(stamp).write_text(result.reason, encoding="ascii")


def _hold_lock(lock_path: str, ready_path: str) -> None:
    lock = FileLock(Path(lock_path))
    lock.acquire(timeout=2)
    os.lseek(lock._fd, 0, os.SEEK_SET)
    os.write(lock._fd, b"1 2000-01-01T00:00:00Z\n")
    Path(ready_path).write_text("ready", encoding="ascii")
    time.sleep(20)
    lock.release()


class RemediationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.clock = Clock()
        self.store = TaskStore(Path(self.directory.name) / "queue.json")

    def tearDown(self) -> None:
        self.directory.cleanup()

    def scheduler(self, name: str = "s1", **overrides) -> Scheduler:
        return Scheduler(self.store, name, policy(**overrides), self.clock.now, lease_seconds=30)

    def test_path_traversal_and_allowlist_bypass_are_rejected(self) -> None:
        scheduler_source = Path(
            __import__("bintrans_autopilot.scheduler", fromlist=["scheduler"]).__file__
        ).read_text(encoding="utf-8")
        self.assertNotIn("lstrip", scheduler_source)
        cases = {
            "../secret": "invalid_path",
            "/etc/passwd": "invalid_path",
            "C:\\Windows\\system32": "invalid_path",
            "\\\\server\\share\\file": "invalid_path",
            ".env": "forbidden_path",
            "docs/../../": "invalid_path",
            "docs/../../etc/passwd": "invalid_path",
            "ops/secret": "forbidden_path",
            "ops": "forbidden_path",
            "docs-evil/file": "unapproved_path",
            "documentary/file": "unapproved_path",
            ".environment": "unapproved_path",
            "docs/./file": "invalid_path",
            "docs//file": "invalid_path",
        }
        for index, path in enumerate(cases):
            self.store.enqueue(task(id=f"path-{index}", allowed_paths=(path,)), self.clock.now(), "s1")
        self.scheduler().tick()
        for index, (path, reason) in enumerate(cases.items()):
            stored = self.store.get(f"path-{index}", self.clock.now())
            self.assertEqual(stored.status, "BLOCKED", path)
            self.assertEqual(stored.block_reason, reason, path)
            self.assertNotEqual(stored.status, "CLAIMED", path)

    def test_symlink_escape_is_rejected_before_claim(self) -> None:
        root = Path(self.directory.name) / "repo"
        outside = Path(self.directory.name) / "outside"
        root.mkdir()
        outside.mkdir()
        (outside / "secret.txt").write_text("secret", encoding="utf-8")
        link = root / "docs"
        try:
            os.symlink(outside, link, target_is_directory=True)
        except OSError:
            completed = subprocess.run(
                ["cmd", "/c", "mklink", "/J", str(link), str(outside)],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(completed.returncode, 0, completed.stderr or completed.stdout)
        self.assertIsNone(resolve_task_path(root, "docs/secret.txt"))
        safe = root / "notes"
        safe.mkdir()
        (safe / "ok.txt").write_text("ok", encoding="utf-8")
        resolved = resolve_task_path(root, "notes/ok.txt")
        self.assertIsNotNone(resolved)
        self.assertEqual(resolved, (safe / "ok.txt").resolve())
        self.store.enqueue(task(id="escape", allowed_paths=("docs/secret.txt",)), self.clock.now(), "s1")
        result = self.scheduler(workspace_root=str(root)).tick()
        self.assertEqual(result.reason, "path_escape")
        self.assertEqual(result.cloud_agents_started, 0)
        stored = self.store.get("escape", self.clock.now())
        self.assertEqual(stored.status, "BLOCKED")
        self.assertEqual(stored.block_reason, "path_escape")

    def test_two_processes_cannot_claim_one_task(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        context = multiprocessing.get_context("spawn")
        stamps = [Path(self.directory.name) / "s1.txt", Path(self.directory.name) / "s2.txt"]
        processes = [
            context.Process(target=_claim_in_process, args=(str(self.store.path), name, str(stamp)))
            for name, stamp in zip(("s1", "s2"), stamps)
        ]
        for process in processes:
            process.start()
        try:
            for process in processes:
                process.join(20)
                self.assertFalse(process.is_alive())
                self.assertEqual(process.exitcode, 0)
        finally:
            for process in processes:
                if process.is_alive():
                    process.terminate()
                    process.join(5)
        reasons = [stamp.read_text(encoding="ascii") for stamp in stamps]
        self.assertEqual(reasons.count("claimed"), 1)
        self.assertEqual(self.store.get("queue-1", self.clock.now()).status, "CLAIMED")

    def test_expired_lock_text_does_not_transfer_ownership(self) -> None:
        context = multiprocessing.get_context("spawn")
        lock_path = Path(self.directory.name) / "held.lock"
        ready_path = Path(self.directory.name) / "held.ready"
        holder = context.Process(target=_hold_lock, args=(str(lock_path), str(ready_path)))
        holder.start()
        try:
            deadline = time.monotonic() + 10
            while not ready_path.exists():
                if time.monotonic() >= deadline:
                    break
                time.sleep(0.05)
            self.assertTrue(ready_path.exists())
            waiter = FileLock(lock_path)
            with self.assertRaises(QueueError) as raised:
                waiter.acquire(timeout=0.4)
            self.assertEqual(raised.exception.code, "lock_timeout")
            self.assertTrue(lock_path.exists())
            holder.terminate()
            holder.join(10)
            self.assertFalse(holder.is_alive())
            recovered = FileLock(lock_path)
            recovered.acquire(timeout=2)
            recovered.release()
            self.assertIn(b"2000-01-01T00:00:00Z", lock_path.read_bytes())
        finally:
            if holder.is_alive():
                holder.terminate()
                holder.join(5)

    def test_permission_error_does_not_remove_another_lock(self) -> None:
        lock_path = Path(self.directory.name) / "denied.lock"
        lock_path.write_text("other-owner\n", encoding="ascii")
        with mock.patch("bintrans_autopilot.task_queue.os.open", side_effect=PermissionError):
            with self.assertRaises(QueueError) as raised:
                FileLock(lock_path).acquire(timeout=0.2)
        self.assertEqual(raised.exception.code, "lock_unavailable")
        self.assertEqual(lock_path.read_text(encoding="ascii"), "other-owner\n")

    def test_old_fence_cannot_finish_after_reclaim_with_same_scheduler_id(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        first = self.scheduler("s1")
        self.assertEqual(first.tick(stop_after="claimed").reason, "claimed")
        generation = self.store.get("queue-1", self.clock.now()).lease_generation
        self.clock.advance(31)
        second = self.scheduler("s1")
        self.assertEqual(second.tick(stop_after="claimed").reason, "claimed")
        stored = self.store.get("queue-1", self.clock.now())
        self.assertGreater(stored.lease_generation, generation)
        lost = first.finish("queue-1")
        self.assertEqual(lost.reason, "lease_lost")
        self.assertFalse(lost.merge_attempted)
        self.assertEqual(lost.cloud_agents_started, 0)
        after = self.store.get("queue-1", self.clock.now())
        self.assertEqual(after.status, "CLAIMED")
        self.assertEqual(after.lease_owner, "s1")
        self.assertEqual(after.lease_generation, stored.lease_generation)
        self.assertEqual(second.finish("queue-1").reason, "review_required")

    def test_late_ci_failure_keeps_completed_and_loses_confirmation(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        scheduler = self.scheduler()
        self.assertEqual(scheduler.tick().status, "REVIEW_REQUIRED")
        self.assertEqual(scheduler.record_ci("queue-1", "success").reason, "SUCCESS")
        self.assertEqual(scheduler.accept_review("queue-1").reason, "completed")
        late = scheduler.record_ci("queue-1", "failure")
        stored = self.store.get("queue-1", self.clock.now())
        self.assertEqual(late.reason, "ci_confirmation_lost")
        self.assertEqual(late.status, "COMPLETED")
        self.assertEqual(stored.status, "COMPLETED")
        self.assertEqual(stored.ci_verdict, "FAIL")
        self.assertEqual(stored.ci_confirmation, "LOST")
        self.assertIn("ci_confirmation_lost", [entry["reason"] for entry in stored.history])
        self.assertIn("ci_confirmation_lost", [entry["action"] for entry in self.store.journal(self.clock.now())])
        self.assertEqual(scheduler.accept_review("queue-1").reason, "terminal_immutable")
        self.assertEqual(scheduler.record_ci("queue-1", "success").reason, "terminal_immutable")
        self.assertEqual(self.store.get("queue-1", self.clock.now()).ci_confirmation, "LOST")
        again = scheduler.tick()
        self.assertNotEqual(again.reason, "claimed")
        self.assertEqual(again.cloud_agents_started, 0)
        self.assertFalse(again.merge_attempted)
        self.assertFalse(again.deploy_attempted)
        self.assertEqual(self.store.get("queue-1", self.clock.now()).status, "COMPLETED")

    def test_corrupt_store_recovers_without_losing_the_original(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        self.store.enqueue(task(id="queue-2"), self.clock.now(), "s1")
        observed = []
        real_replace = os.replace

        def spy_replace(src, dst):
            observed.append((Path(src).parent, Path(dst).parent, Path(src).name))
            return real_replace(src, dst)

        with mock.patch("bintrans_autopilot.task_queue.os.fsync", wraps=os.fsync) as fsync:
            with mock.patch("bintrans_autopilot.task_queue.os.replace", side_effect=spy_replace):
                self.store.enqueue(task(id="queue-3"), self.clock.now(), "s1")
        self.assertGreater(fsync.call_count, 0)
        self.assertTrue(observed)
        self.assertTrue(all(src_dir == dst_dir == self.store.path.parent for src_dir, dst_dir, _name in observed))
        self.assertTrue(any(name.endswith(".tmp") for _src, _dst, name in observed))
        stamp = self.store.path.stat().st_mtime_ns
        self.assertEqual(self.store.get("queue-3", self.clock.now()).id, "queue-3")
        self.assertEqual(self.store.list_tasks(self.clock.now())[0].id, "queue-1")
        self.store.journal(self.clock.now())
        self.assertEqual(self.store.path.stat().st_mtime_ns, stamp)
        torn = b'{"version":1,"tasks":'
        self.store.path.write_bytes(torn)
        self.assertEqual(self.store.get("queue-3", self.clock.now()).id, "queue-3")
        self.assertEqual(self.store.path.read_bytes(), torn)
        self.assertFalse(self.store._corrupt_path().exists())
        backup = self.store._backup_path()
        backup_bytes = backup.read_bytes()
        backup.write_bytes(b"{")
        with self.assertRaises(QueueError) as raised:
            self.store.get("queue-1", self.clock.now())
        self.assertEqual(raised.exception.code, "store_corrupt")
        self.assertEqual(self.store.path.read_bytes(), torn)
        self.assertEqual(backup.read_bytes(), b"{")
        self.assertEqual(backup_bytes[:1], b"{")

    def test_rewrite_preserves_corrupt_primary_once(self) -> None:
        self.store.enqueue(task(), self.clock.now(), "s1")
        torn = b'{"version":1,"tasks":'
        self.store.path.write_bytes(torn)
        self.store.enqueue(task(id="queue-2"), self.clock.now(), "s1")
        self.assertEqual(self.store._corrupt_path().read_bytes(), torn)
        self.assertIsNotNone(self.store.get("queue-1", self.clock.now()))
        self.assertIsNotNone(self.store.get("queue-2", self.clock.now()))
        self.store.path.write_bytes(b"{")
        with self.assertRaises(QueueError) as raised:
            self.store.enqueue(task(id="queue-3"), self.clock.now(), "s1")
        self.assertEqual(raised.exception.code, "store_corrupt")
        self.assertEqual(self.store.path.read_bytes(), b"{")
        self.assertEqual(self.store._corrupt_path().read_bytes(), torn)
        self.assertIsNone(self.store.get("queue-3", self.clock.now()))

    def test_unknown_actions_and_side_effects_are_refused(self) -> None:
        refused = ("merge", "deploy", "cloud_agent_create", "production_change", "surprise")
        for index, action in enumerate(refused):
            self.store.enqueue(
                task(id=f"act-{index}", requested_actions=(action,)),
                self.clock.now(),
                "s1",
            )
        result = self.scheduler().tick()
        self.assertEqual(result.reason, "forbidden_action")
        self.assertEqual(result.cloud_agents_started, 0)
        self.assertFalse(result.merge_attempted)
        self.assertFalse(result.deploy_attempted)
        for index, _action in enumerate(refused):
            stored = self.store.get(f"act-{index}", self.clock.now())
            self.assertEqual(stored.status, "BLOCKED")
            self.assertEqual(stored.block_reason, "forbidden_action")
        scheduler = self.scheduler()

        def apply(data: dict) -> None:
            for action in refused:
                scheduler._log(data, self.clock.now(), "act-0", action, "no")

        self.store.update(self.clock.now(), apply)
        logged = {entry["action"] for entry in self.store.journal(self.clock.now())}
        self.assertTrue(logged.isdisjoint(set(refused)))
        self.assertIn("refused", logged)

    def test_queue_command_does_not_construct_cursor_client(self) -> None:
        import bintrans_autopilot.__main__ as entry

        def boom(*_args, **_kwargs):
            raise AssertionError("CursorClient constructed")

        original = entry.CursorClient
        entry.CursorClient = boom
        try:
            self.assertEqual(entry.main(["queue-dry-run"]), 0)
        finally:
            entry.CursorClient = original
        from bintrans_autopilot import scheduler as scheduler_module

        text = Path(scheduler_module.__file__).read_text(encoding="utf-8")
        self.assertNotIn("CursorClient", text)
        self.assertNotIn("cursor_api", text)


if __name__ == "__main__":
    unittest.main()

"""Dry-run worker tests. Database cases use the isolated queue DSN and skip when it is absent."""

from __future__ import annotations

import os
import threading
import unittest
from pathlib import Path

from bintrans_autopilot.postgres_queue import PENDING, RUNNING, SUCCEEDED, PostgresQueue, QueueError
from bintrans_autopilot.tests.test_postgres_queue_03b import (
    _cell,
    _dsn,
    _ensure_tenant_roles,
    _owner_connect,
    _tenant_connect,
)
from bintrans_autopilot.worker import AutopilotWorker, WorkerError


class _Crash(BaseException):
    pass


def _connect_a(dsn: str):
    return _tenant_connect(dsn, "tenant_a_runtime", "tenant_a_test")


def _connect_b(dsn: str):
    return _tenant_connect(dsn, "tenant_b_runtime", "tenant_b_test")


def _expire(dsn: str, job_id: str) -> None:
    connection = _connect_a(dsn)
    try:
        updated = connection.execute(
            """
            UPDATE autopilot_queue.jobs
            SET lease_expires_at = clock_timestamp() - interval '1 second'
            WHERE job_id = %s
            """,
            (job_id,),
        )
        if updated.rowcount != 1:
            raise AssertionError("lease was not expired")
        connection.commit()
    finally:
        connection.close()


def _token(dsn: str, job_id: str) -> str:
    connection = _connect_a(dsn)
    try:
        row = connection.execute(
            "SELECT lease_token FROM autopilot_queue.jobs WHERE job_id = %s",
            (job_id,),
        ).fetchone()
        connection.rollback()
    finally:
        connection.close()
    return str(_cell(row))


class WorkerStaticTests(unittest.TestCase):
    def test_import_does_not_enable_switches_or_external_calls(self) -> None:
        source = Path(__file__).resolve().parents[1].joinpath("worker.py").read_text(encoding="utf-8")
        for banned in ("subprocess", "cursor_api", "paramiko", "socket", "os.system", "Popen"):
            self.assertNotIn(banned, source)
        queue = PostgresQueue(lambda: None)
        worker = AutopilotWorker(queue, lambda: None, mode="dry_run", tenant_id="tenant-a", owner="worker-a")
        self.assertFalse(worker.worker_enabled)
        self.assertFalse(worker.autopilot_enabled)
        self.assertFalse(worker.paid_automation_enabled)
        self.assertEqual(worker.external_calls, 0)
        self.assertEqual(worker.cloud_agents_started, 0)

    def test_live_mode_and_environment_cannot_enable_the_worker(self) -> None:
        queue = PostgresQueue(lambda: None)
        with self.assertRaises(WorkerError) as raised:
            AutopilotWorker(queue, lambda: None, mode="live", tenant_id="tenant-a", owner="worker-a")
        self.assertEqual(raised.exception.code, "worker_disabled")
        os.environ["PAID_AUTOMATION_ENABLED"] = "true"
        try:
            with self.assertRaises(WorkerError) as raised:
                AutopilotWorker(queue, lambda: None, mode="dry_run", tenant_id="tenant-a", owner="worker-a")
            self.assertEqual(raised.exception.code, "worker_disabled")
        finally:
            del os.environ["PAID_AUTOMATION_ENABLED"]
        with self.assertRaises(WorkerError) as raised:
            AutopilotWorker(
                queue,
                lambda: None,
                mode="dry_run",
                tenant_id="tenant-a",
                owner="worker-a",
                handlers={"shell": lambda _ctx: None},
            )
        self.assertEqual(raised.exception.code, "handler_rejected")


class WorkerTests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        from bintrans_autopilot.postgres_queue import MigrationRunner

        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
        cls.runner.up()
        _ensure_tenant_roles(cls.dsn, cls.runner)

    def setUp(self) -> None:
        if not self.dsn:
            self.skipTest("postgres_unavailable")
        connection = _owner_connect(self.dsn)
        try:
            connection.execute("TRUNCATE autopilot_queue.jobs")
            connection.commit()
        finally:
            connection.close()
        _ensure_tenant_roles(self.dsn, self.runner)
        self.queue = PostgresQueue(lambda: _connect_a(self.dsn))
        self.queue_b = PostgresQueue(lambda: _connect_b(self.dsn))

    def _worker(self, **kwargs) -> AutopilotWorker:
        values = {
            "mode": "dry_run",
            "tenant_id": "tenant-a",
            "owner": "worker-a",
            "lease_seconds": 30,
        }
        values.update(kwargs)
        return AutopilotWorker(self.queue, lambda: _connect_a(self.dsn), **values)

    def _enqueue(self, key: str, *, job_type: str = "ping", max_attempts: int = 3, payload: dict | None = None):
        return self.queue.enqueue(
            tenant_id="tenant-a",
            job_type=job_type,
            payload={"n": 1} if payload is None else payload,
            idempotency_key=key,
            max_attempts=max_attempts,
            backoff_seconds=0,
        )

    def test_one_worker_completes_a_dry_run_job(self) -> None:
        created = self._enqueue("one")
        worker = self._worker()
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "completed")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, SUCCEEDED)
        self.assertEqual(worker.metrics.jobs_completed_total, 1)
        self.assertEqual(worker.external_calls, 0)
        self.assertEqual(worker.cloud_agents_started, 0)
        text = "\n".join(logs.output)
        self.assertIn("event=worker_started", text)
        self.assertIn("event=job_claimed", text)
        self.assertIn("event=job_completed", text)
        self.assertNotIn("postgresql://", text)

    def test_two_workers_cannot_both_own_one_job(self) -> None:
        self._enqueue("shared")
        self._enqueue("shared-b")
        first = self._worker(owner="worker-a")
        second = self._worker(owner="worker-b")
        barrier = threading.Barrier(2)
        results = []

        def run(worker) -> None:
            barrier.wait(5)
            results.append((worker.owner, worker.run_once()))

        threads = [threading.Thread(target=run, args=(worker,)) for worker in (first, second)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(5)
        self.assertEqual(sorted(outcome for _owner, outcome in results), ["completed", "completed"])
        self.assertEqual(first.metrics.jobs_completed_total + second.metrics.jobs_completed_total, 2)

    def test_one_job_has_a_single_winner(self) -> None:
        created = self._enqueue("only-one")
        first = self._worker(owner="worker-a")
        second = self._worker(owner="worker-b")
        barrier = threading.Barrier(2)
        results = []

        def run(worker) -> None:
            barrier.wait(5)
            results.append(worker.run_once())

        threads = [threading.Thread(target=run, args=(worker,)) for worker in (first, second)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(5)
        self.assertEqual(sorted(results), ["completed", "idle"])
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, SUCCEEDED)

    def test_crash_before_claim_leaves_the_job_pending(self) -> None:
        created = self._enqueue("before-claim")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, PENDING)

    def test_crash_after_claim_is_recovered_by_another_worker(self) -> None:
        created = self._enqueue("after-claim")
        crashed = self.queue.claim("tenant-a", "worker-crashed", 30)
        self.assertIsNotNone(crashed)
        _expire(self.dsn, created.job_id)
        with self.assertRaises(QueueError) as raised:
            self.queue.complete("tenant-a", created.job_id, "worker-crashed", crashed.lease_token)
        self.assertEqual(raised.exception.code, "lease_expired")
        worker = self._worker(owner="worker-recovered")
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "completed")
        self.assertIn("event=recovery_completed", "\n".join(logs.output))
        self.assertGreaterEqual(worker.metrics.lease_expirations, 1)
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, SUCCEEDED)
        self.assertGreaterEqual(self.queue.get_status("tenant-a", created.job_id).attempt_count, 2)

    def test_crash_during_handler_does_not_complete_with_the_old_token(self) -> None:
        created = self._enqueue("during")

        def crash(_context) -> None:
            raise _Crash()

        worker = self._worker(handlers={"ping": crash})
        with self.assertRaises(_Crash):
            worker.run_once()
        token = _token(self.dsn, created.job_id)
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, RUNNING)
        _expire(self.dsn, created.job_id)
        with self.assertRaises(QueueError) as raised:
            self.queue.complete("tenant-a", created.job_id, "worker-a", token)
        self.assertEqual(raised.exception.code, "lease_expired")
        recovered = self._worker(owner="worker-next")
        self.assertEqual(recovered.run_once(), "completed")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, SUCCEEDED)

    def test_lost_connection_leaves_the_job_pending(self) -> None:
        created = self._enqueue("offline")
        gate = {"fail": True}

        def connect():
            if gate["fail"]:
                raise OSError("lost")
            return _connect_a(self.dsn)

        worker = AutopilotWorker(
            PostgresQueue(connect),
            connect,
            mode="dry_run",
            tenant_id="tenant-a",
            owner="worker-a",
        )
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "unavailable")
        self.assertIn("event=database_unavailable", "\n".join(logs.output))
        self.assertNotIn("lost", "\n".join(logs.output))
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, PENDING)

    def test_expired_lease_and_failed_renew_do_not_complete(self) -> None:
        created = self._enqueue("renew")

        def renew_once(context) -> None:
            _expire(self.dsn, context.job_id)
            context.renew()

        worker = self._worker(handlers={"ping": renew_once})
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "failed")
        text = "\n".join(logs.output)
        self.assertIn("event=job_failed", text)
        self.assertNotIn("event=job_completed", text)
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, RUNNING)
        self.assertEqual(worker.metrics.jobs_completed_total, 0)
        replacement = self._worker(owner="worker-b")
        self.assertEqual(replacement.run_once(), "completed")

    def test_retries_reach_dead(self) -> None:
        created = self._enqueue("dead", max_attempts=2)

        def fail(_context) -> None:
            raise RuntimeError("handler")

        worker = self._worker(handlers={"ping": fail})
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "retry")
            self.assertEqual(worker.run_once(), "dead")
        text = "\n".join(logs.output)
        self.assertIn("event=retry_scheduled", text)
        self.assertIn("event=job_dead", text)
        status = self.queue.get_status("tenant-a", created.job_id)
        self.assertEqual(status.status, "DEAD")
        self.assertEqual(status.attempt_count, 2)

    def test_graceful_stop_finishes_the_current_job_only(self) -> None:
        first = self._enqueue("stop-current")
        second = self._enqueue("stop-next")
        started = threading.Event()
        release = threading.Event()

        def handler(_context) -> None:
            started.set()
            self.assertTrue(release.wait(5))

        worker = self._worker(handlers={"ping": handler})
        outcome = []

        def run() -> None:
            outcome.append(worker.run_once())

        thread = threading.Thread(target=run)
        thread.start()
        self.assertTrue(started.wait(5))
        worker.stop()
        release.set()
        thread.join(5)
        self.assertEqual(outcome, ["completed"])
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "stopped")
        self.assertIn("event=worker_stopped", "\n".join(logs.output))
        self.assertEqual(self.queue.get_status("tenant-a", first.job_id).status, SUCCEEDED)
        self.assertEqual(self.queue.get_status("tenant-a", second.job_id).status, PENDING)

    def test_restart_continues_the_remaining_job(self) -> None:
        first = self._enqueue("restart-a")
        second = self._enqueue("restart-b")
        worker = self._worker()
        self.assertEqual(worker.run_pending(limit=1), 1)
        worker.stop()
        self.assertEqual(worker.run_once(), "stopped")
        restarted = self._worker(owner="worker-restarted")
        self.assertEqual(restarted.run_once(), "completed")
        seen = {
            self.queue.get_status("tenant-a", first.job_id).status,
            self.queue.get_status("tenant-a", second.job_id).status,
        }
        self.assertEqual(seen, {SUCCEEDED})

    def test_in_flight_limit_blocks_a_second_claim(self) -> None:
        self._enqueue("slot-a")
        held = self._enqueue("slot-b")
        started = threading.Event()
        release = threading.Event()

        def handler(_context) -> None:
            started.set()
            self.assertTrue(release.wait(5))

        worker = self._worker(handlers={"ping": handler}, max_in_flight=1)
        thread = threading.Thread(target=worker.run_once)
        thread.start()
        self.assertTrue(started.wait(5))
        self.assertEqual(worker.run_once(), "saturated")
        release.set()
        thread.join(5)
        self.assertEqual(self.queue.get_status("tenant-a", held.job_id).status, PENDING)

    def test_snapshot_and_logs_omit_payload(self) -> None:
        created = self._enqueue("metrics", job_type="docs", payload={"marker": "visible-marker", "n": 1})
        worker = self._worker()
        metrics = worker.snapshot()
        self.assertEqual(metrics.queue_depth, 1)
        self.assertGreaterEqual(metrics.oldest_pending_age, 0)
        self.assertGreater(metrics.worker_heartbeat, 0)
        with self.assertLogs("bintrans_autopilot.worker", level="INFO") as logs:
            self.assertEqual(worker.run_once(), "completed")
        text = "\n".join(logs.output)
        self.assertNotIn("visible-marker", text)
        self.assertNotIn("postgresql://", text)
        self.assertEqual(worker.snapshot().queue_depth, 0)
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, SUCCEEDED)

    def test_other_tenant_job_is_not_claimed_or_visible(self) -> None:
        created = self.queue_b.enqueue(
            tenant_id="tenant-b",
            job_type="ping",
            payload={"n": 1},
            idempotency_key="foreign",
            backoff_seconds=0,
        )
        worker = self._worker()
        self.assertEqual(worker.run_once(), "idle")
        self.assertEqual(self.queue_b.get_status("tenant-b", created.job_id).status, PENDING)
        connection = _connect_a(self.dsn)
        try:
            visible = connection.execute("SELECT count(*) FROM autopilot_queue.jobs").fetchone()
            self.assertEqual(int(_cell(visible)), 0)
            updated = connection.execute(
                "UPDATE autopilot_queue.jobs SET last_error = 'crossed' WHERE job_id = %s",
                (created.job_id,),
            )
            self.assertEqual(updated.rowcount, 0)
            connection.rollback()
        finally:
            connection.close()

    def test_payload_cannot_request_an_external_command(self) -> None:
        with self.assertRaises(QueueError) as raised:
            self.queue.enqueue(
                tenant_id="tenant-a",
                job_type="ping",
                payload={"n": 1, "command": "git push"},
                idempotency_key="command",
            )
        self.assertEqual(raised.exception.code, "invalid_input")
        worker = self._worker(allow={"ping"})
        created = self._enqueue("docs-not-allowed", job_type="docs", payload={"marker": "kept", "n": 1})
        self.assertEqual(worker.run_once(), "failed")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, "FAILED")
        self.assertEqual(worker.external_calls, 0)


if __name__ == "__main__":
    unittest.main()

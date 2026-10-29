"""Isolated PostgreSQL tests for the 0.3B queue.

Integration cases use BINTRANS_AUTOPILOT_QUEUE_DSN. They do not open the
platform database. Without that variable they are skipped, not passed.
"""

from __future__ import annotations

import os
import threading
import time
import unittest
from pathlib import Path

from bintrans_autopilot.postgres_queue import (
    DEAD,
    FAILED,
    PENDING,
    RUNNING,
    SUCCEEDED,
    DryRunWorker,
    MigrationRunner,
    PostgresQueue,
    QueueError,
)
from bintrans_autopilot.task_queue import FileLock, TaskStore


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


def _owner_connect(dsn: str):
    import psycopg

    return psycopg.connect(dsn, connect_timeout=3)


def _runtime_connect(dsn: str):
    connection = _owner_connect(dsn)
    connection.execute("SET ROLE autopilot_queue_runtime")
    connection.commit()
    return connection


class StaticQueueTests(unittest.TestCase):
    def test_file_queue_and_lock_remain(self) -> None:
        self.assertTrue(callable(FileLock))
        self.assertTrue(callable(TaskStore))

    def test_new_queue_does_not_use_file_locks(self) -> None:
        source = Path(
            __import__("bintrans_autopilot.postgres_queue", fromlist=["postgres_queue"]).__file__
        ).read_text(encoding="utf-8")
        self.assertNotIn("FileLock", source)
        self.assertNotIn("fcntl", source)
        self.assertNotIn("LockFileEx", source)
        self.assertNotIn("cursor_api", source)
        self.assertNotIn("CursorClient", source)
        self.assertIn("FOR UPDATE SKIP LOCKED", source)

    def test_platform_migration_head_stays_000105(self) -> None:
        root = Path(__file__).resolve().parents[3]
        files = sorted((root / "infrastructure" / "migrations").glob("*.up.sql"))
        self.assertEqual(len(files), 105)
        self.assertTrue(files[-1].name.startswith("000105_"))
        self.assertEqual(list((root / "infrastructure" / "migrations").glob("000106*")), [])
        guard = (root / "tests" / "bintrans-e2e-transport-flow" / "stand_guard.go").read_text(encoding="utf-8")
        contract = (root / "tests" / "bintrans-e2e-transport-flow" / "stand_contract_test.go").read_text(encoding="utf-8")
        self.assertIn("standSchemaVersion = 105", guard)
        self.assertIn('HasPrefix(last, "000105_")', contract)

    def test_autopilot_and_paid_switches_are_refused(self) -> None:
        with self.assertRaises(QueueError) as raised:
            PostgresQueue(lambda: None, autopilot_enabled=True)
        self.assertEqual(raised.exception.code, "automation_disabled")
        with self.assertRaises(QueueError) as raised:
            PostgresQueue(lambda: None, paid_automation_enabled=True)
        self.assertEqual(raised.exception.code, "automation_disabled")

    def test_secret_payload_is_rejected_before_a_connection(self) -> None:
        opened = {"count": 0}

        def connect():
            opened["count"] += 1
            raise AssertionError("connection opened")

        queue = PostgresQueue(connect)
        with self.assertRaises(QueueError) as raised:
            queue.enqueue(
                tenant_id="tenant-a",
                job_type="docs",
                payload={"token": "cursor_example"},
                idempotency_key="job-secret",
            )
        self.assertEqual(raised.exception.code, "secret_rejected")
        self.assertEqual(opened["count"], 0)

    def test_postgres_unavailable_fails_closed(self) -> None:
        try:
            import psycopg
        except ImportError:
            self.skipTest("postgres_driver_unavailable")
        queue = PostgresQueue(
            lambda: psycopg.connect(
                "postgresql://queue_test:queue_test@127.0.0.1:1/none",
                connect_timeout=2,
            )
        )
        with self.assertRaises(QueueError) as raised:
            queue.enqueue(
                tenant_id="tenant-a",
                job_type="docs",
                payload={"n": 1},
                idempotency_key="job-down",
            )
        self.assertEqual(raised.exception.code, "store_unavailable")


class PostgresQueueTests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
        cls.runner.up()

    def setUp(self) -> None:
        if not self.dsn:
            self.skipTest("postgres_unavailable")
        connection = _owner_connect(self.dsn)
        try:
            connection.execute("TRUNCATE autopilot_queue.jobs")
            connection.commit()
        finally:
            connection.close()
        self.queue = PostgresQueue(lambda: _runtime_connect(self.dsn))

    def test_enqueue_claim_and_complete(self) -> None:
        with self.assertLogs("bintrans_autopilot.postgres_queue", level="INFO") as captured:
            created = self.queue.enqueue(
                tenant_id="tenant-a",
                job_type="docs",
                payload={"marker": "queue-marker-value"},
                idempotency_key="job-1",
            )
        self.assertTrue(created.created)
        self.assertNotIn("queue-marker-value", "\n".join(captured.output))
        worker = DryRunWorker(self.queue)
        lease = worker.handle_once("tenant-a", "worker-a")
        self.assertIsNotNone(lease)
        self.assertEqual(worker.external_calls, 0)
        self.assertEqual(worker.handled, 1)
        status = self.queue.get_status("tenant-a", created.job_id)
        self.assertIsNotNone(status)
        assert status is not None
        self.assertEqual(status.status, SUCCEEDED)
        self.assertEqual(lease.cloud_agents_started, 0)
        self.assertEqual(lease.external_effects, 0)

    def test_two_workers_do_not_both_claim(self) -> None:
        created = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="job-race",
        )
        barrier = threading.Barrier(2)
        results = []

        def run(owner: str) -> None:
            barrier.wait()
            results.append(self.queue.claim("tenant-a", owner, 30))

        threads = [threading.Thread(target=run, args=(owner,)) for owner in ("worker-a", "worker-b")]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        claimed = [item for item in results if item is not None]
        self.assertEqual(len(claimed), 1)
        self.assertEqual(claimed[0].job_id, created.job_id)
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, RUNNING)

    def test_crash_expiry_renew_and_stale_token(self) -> None:
        created = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="job-lease",
            max_attempts=3,
            backoff_seconds=0,
        )
        first = self.queue.claim("tenant-a", "worker-a", 30)
        self.assertIsNotNone(first)
        renewed = self.queue.renew_lease("tenant-a", first.job_id, "worker-a", first.lease_token, 30)
        self.assertEqual(renewed.lease_token, first.lease_token)
        short = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 2},
            idempotency_key="job-expire",
            max_attempts=3,
            backoff_seconds=0,
        )
        crashed = self.queue.claim("tenant-a", "worker-a", 1)
        self.assertEqual(crashed.job_id, short.job_id)
        time.sleep(1.5)
        with self.assertRaises(QueueError) as expired:
            self.queue.renew_lease("tenant-a", short.job_id, "worker-a", crashed.lease_token, 30)
        self.assertEqual(expired.exception.code, "lease_expired")
        self.assertEqual(self.queue.requeue_expired("tenant-a"), 1)
        self.assertEqual(self.queue.get_status("tenant-a", short.job_id).status, PENDING)
        second = self.queue.claim("tenant-a", "worker-b", 30)
        self.assertEqual(second.job_id, short.job_id)
        self.assertNotEqual(second.lease_token, crashed.lease_token)
        with self.assertRaises(QueueError) as raised:
            self.queue.complete("tenant-a", short.job_id, "worker-a", crashed.lease_token)
        self.assertEqual(raised.exception.code, "stale_lease")
        self.queue.complete("tenant-a", short.job_id, "worker-b", second.lease_token)
        self.assertEqual(self.queue.get_status("tenant-a", short.job_id).status, SUCCEEDED)
        self.queue.complete("tenant-a", created.job_id, "worker-a", first.lease_token)

    def test_same_idempotency_key_does_not_duplicate(self) -> None:
        payload = {"n": 1}
        first = self.queue.enqueue(tenant_id="tenant-a", job_type="docs", payload=payload, idempotency_key="same-key")
        second = self.queue.enqueue(tenant_id="tenant-a", job_type="docs", payload=payload, idempotency_key="same-key")
        self.assertEqual(first.job_id, second.job_id)
        self.assertFalse(second.created)
        other = self.queue.enqueue(tenant_id="tenant-b", job_type="docs", payload=payload, idempotency_key="same-key")
        self.assertNotEqual(other.job_id, first.job_id)
        with self.assertRaises(QueueError) as raised:
            self.queue.enqueue(
                tenant_id="tenant-a",
                job_type="docs",
                payload={"n": 2},
                idempotency_key="same-key",
            )
        self.assertEqual(raised.exception.code, "idempotency_conflict")

    def test_retries_end_dead_and_nonretry_fails(self) -> None:
        created = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="job-dead",
            max_attempts=2,
            backoff_seconds=0,
        )
        first = self.queue.claim("tenant-a", "worker-a", 30)
        failed = self.queue.fail("tenant-a", first.job_id, "worker-a", first.lease_token, "handler_failed")
        self.assertEqual(failed.status, PENDING)
        self.assertEqual(failed.attempt_count, 1)
        second = self.queue.claim("tenant-a", "worker-b", 30)
        dead = self.queue.fail("tenant-a", second.job_id, "worker-b", second.lease_token, "handler_failed")
        self.assertEqual(dead.status, DEAD)
        self.assertIsNone(self.queue.claim("tenant-a", "worker-c", 30))
        terminal = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 2},
            idempotency_key="job-failed",
            max_attempts=3,
            backoff_seconds=0,
        )
        lease = self.queue.claim("tenant-a", "worker-a", 30)
        status = self.queue.fail("tenant-a", lease.job_id, "worker-a", lease.lease_token, "policy_rejected", retryable=False)
        self.assertEqual(status.status, FAILED)
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, DEAD)
        self.assertEqual(self.queue.get_status("tenant-a", terminal.job_id).status, FAILED)

    def test_tenant_cannot_read_or_finish_another_tenant(self) -> None:
        created = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="job-tenant",
        )
        self.queue.enqueue(tenant_id="tenant-b", job_type="docs", payload={"n": 1}, idempotency_key="job-tenant-b")
        self.assertIsNone(self.queue.get_status("tenant-b", created.job_id))
        lease = self.queue.claim("tenant-a", "worker-a", 30)
        self.assertEqual(lease.job_id, created.job_id)
        with self.assertRaises(QueueError) as raised:
            self.queue.complete("tenant-b", created.job_id, "worker-a", lease.lease_token)
        self.assertEqual(raised.exception.code, "not_found")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, RUNNING)
        foreign = self.queue.claim("tenant-b", "worker-b", 30)
        self.assertNotEqual(foreign.job_id, created.job_id)

    def test_migration_is_isolated_and_reversible(self) -> None:
        self.assertEqual(self.runner.up(), 1)
        connection = _owner_connect(self.dsn)
        try:
            platform = connection.execute("SELECT to_regclass('public.schema_migrations') AS name").fetchone()
            queue = connection.execute("SELECT to_regclass('autopilot_queue.schema_migrations') AS name").fetchone()
            self.assertIsNone(platform[0] if not isinstance(platform, dict) else platform["name"])
            self.assertIsNotNone(queue[0] if not isinstance(queue, dict) else queue["name"])
            connection.commit()
        finally:
            connection.close()
        try:
            self.assertEqual(self.runner.down(), 0)
            self.assertEqual(self.runner.up(), 1)
            self.assertEqual(self.runner.down(), 0)
            self.assertEqual(self.runner.up(), 1)
        finally:
            self.runner.up()

    def test_runtime_role_cannot_delete_or_migrate(self) -> None:
        import psycopg

        connection = _runtime_connect(self.dsn)
        try:
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute("DELETE FROM autopilot_queue.jobs")
            connection.rollback()
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute(
                    "INSERT INTO autopilot_queue.schema_migrations (version, name) VALUES (99, 'denied')"
                )
            connection.rollback()
        finally:
            connection.close()


if __name__ == "__main__":
    unittest.main()

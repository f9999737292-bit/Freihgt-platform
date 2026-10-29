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
from urllib.parse import urlsplit

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
    _statements,
)
from bintrans_autopilot.task_queue import FileLock, TaskStore

_TENANTS = {
    "tenant-a": ("tenant_a_runtime", "tenant_a_test"),
    "tenant-b": ("tenant_b_runtime", "tenant_b_test"),
}


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


def _owner_connect(dsn: str):
    import psycopg

    return psycopg.connect(dsn, connect_timeout=3)


def _tenant_connect(dsn: str, user: str, password: str):
    import psycopg

    parts = urlsplit(dsn)
    return psycopg.connect(
        host=parts.hostname or "localhost",
        port=parts.port or 5432,
        dbname=parts.path.lstrip("/"),
        user=user,
        password=password,
        connect_timeout=3,
    )


def _cell(row, index: int = 0, name: str = ""):
    if isinstance(row, dict):
        return row[name or next(iter(row))]
    return row[index]


def _ensure_tenant_roles(dsn: str, runner: MigrationRunner) -> None:
    from psycopg import sql

    connection = _owner_connect(dsn)
    try:
        for _tenant, (role, password) in _TENANTS.items():
            found = connection.execute("SELECT 1 FROM pg_roles WHERE rolname = %s", (role,)).fetchone()
            statement = "ALTER ROLE {} PASSWORD {}" if found else (
                "CREATE ROLE {} LOGIN PASSWORD {} NOSUPERUSER NOBYPASSRLS "
                "NOCREATEROLE NOCREATEDB NOREPLICATION INHERIT"
            )
            connection.execute(sql.SQL(statement).format(sql.Identifier(role), sql.Literal(password)))
        connection.commit()
    finally:
        connection.close()
    for tenant, (role, _password) in _TENANTS.items():
        runner.register_principal(tenant, role)


def _detach_runtime(connection) -> None:
    from psycopg import sql

    rows = connection.execute(
        """
        SELECT member.rolname AS member_name
        FROM pg_auth_members m
        JOIN pg_roles granted ON granted.oid = m.roleid
        JOIN pg_roles member ON member.oid = m.member
        WHERE granted.rolname = 'autopilot_queue_runtime'
        """
    ).fetchall()
    for row in rows:
        connection.execute(
            sql.SQL("REVOKE autopilot_queue_runtime FROM {}").format(sql.Identifier(_cell(row, 0, "member_name")))
        )


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
        self.assertNotIn("app.tenant_id", source)

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

    def test_sql_enables_forced_rls_without_cascade(self) -> None:
        root = Path(__file__).resolve().parents[1] / "sql" / "autopilot_queue"
        up = (root / "0001_queue.up.sql").read_text(encoding="utf-8")
        down = (root / "0001_queue.down.sql").read_text(encoding="utf-8")
        statements = _statements(up)
        self.assertTrue(any("ENABLE ROW LEVEL SECURITY" in item for item in statements))
        self.assertTrue(any("FORCE ROW LEVEL SECURITY" in item for item in statements))
        self.assertTrue(any("RAISE EXCEPTION 'tenant_immutable'" in item for item in statements))
        down_statements = _statements(down)
        self.assertNotIn("app.tenant_id", up)
        self.assertFalse(any("CASCADE" in item for item in statements))
        self.assertFalse(any("CASCADE" in item for item in down_statements))
        self.assertIn("session_user", up)
        runner = Path(__file__).resolve().parents[1] / "postgres_queue.py"
        runner_source = runner.read_text(encoding="utf-8")
        self.assertIn("relkind = 'S'", runner_source)
        self.assertIn("pg_attribute", runner_source)
        self.assertIn("pg_depend", runner_source)
        self.assertIn("deptype", runner_source)

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
        rejected = [
            {"token": "cursor_example"},
            {"password": "hunter2"},
            {"API-KEY": "sk-live-abc"},
            {"nested": {"private_key": "abcd"}},
            {"items": [{"authorization": "raw"}]},
            {"dsn": "postgresql://queue_test:queue_test@localhost:5432/autopilot_queue_test"},
            {"marker": "postgresql://user:secret@localhost/db"},
            {"note": "Bearer raw-token"},
        ]
        for payload in rejected:
            with self.subTest(payload=sorted(payload)):
                with self.assertRaises(QueueError) as raised:
                    queue.enqueue(
                        tenant_id="tenant-a",
                        job_type="docs",
                        payload=payload,
                        idempotency_key="job-secret",
                    )
                self.assertEqual(raised.exception.code, "secret_rejected")
                self.assertIsNone(raised.exception.__cause__)
        with self.assertRaises(QueueError) as raised:
            queue.enqueue(
                tenant_id="tenant-a",
                job_type="other",
                payload={"n": 1},
                idempotency_key="job-unknown",
            )
        self.assertEqual(raised.exception.code, "invalid_input")
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
        self.assertIsNone(raised.exception.__cause__)
        self.assertNotIn("queue_test", str(raised.exception))


class PostgresQueueTests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
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
        self.queue = PostgresQueue(lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"))
        self.queue_b = PostgresQueue(lambda: _tenant_connect(self.dsn, "tenant_b_runtime", "tenant_b_test"))

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
            barrier.wait(timeout=10)
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
        other = self.queue_b.enqueue(tenant_id="tenant-b", job_type="docs", payload=payload, idempotency_key="same-key")
        self.assertNotEqual(other.job_id, first.job_id)
        with self.assertRaises(QueueError) as raised:
            self.queue.enqueue(
                tenant_id="tenant-a",
                job_type="docs",
                payload={"n": 2},
                idempotency_key="same-key",
            )
        self.assertEqual(raised.exception.code, "idempotency_conflict")

    def test_idempotency_requires_every_execution_field(self) -> None:
        changes = [
            {"job_type": "ping"},
            {"payload_version": 2},
            {"payload": {"n": 2}},
            {"priority": 200},
            {"max_attempts": 4},
            {"backoff_seconds": 2},
        ]
        for index, change in enumerate(changes):
            with self.subTest(**change):
                key = f"idem-field-{index}"
                base = {
                    "tenant_id": "tenant-a",
                    "job_type": "docs",
                    "payload": {"n": 1},
                    "idempotency_key": key,
                    "payload_version": 1,
                    "priority": 100,
                    "max_attempts": 3,
                    "backoff_seconds": 1,
                }
                first = self.queue.enqueue(**base)
                repeated = self.queue.enqueue(**base)
                self.assertEqual(repeated.job_id, first.job_id)
                self.assertFalse(repeated.created)
                with self.assertRaises(QueueError) as raised:
                    self.queue.enqueue(**{**base, **change})
                self.assertEqual(raised.exception.code, "idempotency_conflict")

    def test_concurrent_enqueue_of_one_key(self) -> None:
        barrier = threading.Barrier(2)
        results = []
        errors = []

        def run(priority: int, key: str) -> None:
            try:
                barrier.wait(timeout=10)
                result = self.queue.enqueue(
                    tenant_id="tenant-a",
                    job_type="docs",
                    payload={"n": 1},
                    idempotency_key=key,
                    priority=priority,
                )
                results.append(result)
            except QueueError as exc:
                errors.append(exc.code)

        same = [threading.Thread(target=run, args=(100, "concurrent-same")) for _ in range(2)]
        for thread in same:
            thread.start()
        for thread in same:
            thread.join()
        self.assertEqual(errors, [])
        self.assertEqual(len(results), 2)
        self.assertEqual(len({item.job_id for item in results}), 1)
        self.assertEqual(sorted(item.created for item in results), [False, True])

        results.clear()
        errors.clear()
        barrier = threading.Barrier(2)
        conflict = [
            threading.Thread(target=run, args=(priority, "concurrent-conflict"))
            for priority in (100, 200)
        ]
        for thread in conflict:
            thread.start()
        for thread in conflict:
            thread.join()
        self.assertEqual(errors, ["idempotency_conflict"])
        self.assertEqual(len(results), 1)
        self.assertTrue(results[0].created)

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
        other = self.queue_b.enqueue(tenant_id="tenant-b", job_type="docs", payload={"n": 1}, idempotency_key="job-tenant-b")
        with self.assertRaises(QueueError) as raised:
            self.queue.get_status("tenant-b", other.job_id)
        self.assertEqual(raised.exception.code, "tenant_mismatch")
        self.assertIsNone(self.queue_b.get_status("tenant-b", created.job_id))
        lease = self.queue.claim("tenant-a", "worker-a", 30)
        self.assertEqual(lease.job_id, created.job_id)
        with self.assertRaises(QueueError) as raised:
            self.queue.complete("tenant-b", created.job_id, "worker-a", lease.lease_token)
        self.assertEqual(raised.exception.code, "tenant_mismatch")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, RUNNING)
        foreign = self.queue_b.claim("tenant-b", "worker-b", 30)
        self.assertEqual(foreign.job_id, other.job_id)

    def test_direct_sql_cannot_cross_tenants_or_forge_context(self) -> None:
        import psycopg

        self.queue.enqueue(tenant_id="tenant-a", job_type="docs", payload={"n": 1}, idempotency_key="sql-a")
        self.queue_b.enqueue(tenant_id="tenant-b", job_type="docs", payload={"n": 1}, idempotency_key="sql-b")
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            connection.execute("SET app.tenant_id = 'tenant-b'")
            self.assertEqual(_cell(connection.execute("SELECT autopilot_queue.bound_tenant()").fetchone()), "tenant-a")
            self.assertEqual(_cell(connection.execute("SELECT count(*) FROM autopilot_queue.jobs").fetchone()), 1)
            self.assertEqual(
                _cell(connection.execute("SELECT count(*) FROM autopilot_queue.jobs WHERE tenant_id = 'tenant-b'").fetchone()),
                0,
            )
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute(
                    """
                    INSERT INTO autopilot_queue.jobs (
                        job_id, tenant_id, job_type, payload_version, payload, payload_digest,
                        status, priority, created_at, available_at, attempt_count, max_attempts,
                        backoff_seconds, idempotency_key, updated_at
                    ) VALUES (
                        'foreign-job', 'tenant-b', 'docs', 1, '{"n": 1}'::jsonb, 'abc',
                        'PENDING', 100, clock_timestamp(), clock_timestamp(), 0, 3,
                        0, 'foreign-key', clock_timestamp()
                    )
                    """
                )
            connection.rollback()
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute("UPDATE autopilot_queue.jobs SET tenant_id = 'tenant-b'")
            connection.rollback()
            updated = connection.execute(
                "UPDATE autopilot_queue.jobs SET last_error = 'crossed' WHERE tenant_id = 'tenant-b'"
            )
            self.assertEqual(updated.rowcount, 0)
            connection.rollback()
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute("DELETE FROM autopilot_queue.jobs")
            connection.rollback()
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute("SELECT * FROM autopilot_queue.tenant_principals")
            connection.rollback()
            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute("SET ROLE tenant_b_runtime")
            connection.rollback()
            connection.execute("SET ROLE autopilot_queue_runtime")
            self.assertEqual(
                _cell(connection.execute("SELECT count(*) FROM autopilot_queue.jobs WHERE tenant_id = 'tenant-b'").fetchone()),
                0,
            )
            connection.rollback()
        finally:
            connection.close()
        privileged = PostgresQueue(lambda: _owner_connect(self.dsn))
        with self.assertRaises(QueueError) as raised:
            privileged.enqueue(tenant_id="tenant-a", job_type="docs", payload={"n": 1}, idempotency_key="owner-path")
        self.assertEqual(raised.exception.code, "runtime_privileged")
        self.assertIsNone(raised.exception.__cause__)

        def bypass_connect():
            connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
            connection.execute("SET ROLE autopilot_queue_bypass")
            connection.commit()
            return connection

        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                "CREATE ROLE autopilot_queue_bypass NOLOGIN BYPASSRLS NOSUPERUSER "
                "NOCREATEROLE NOCREATEDB NOREPLICATION"
            )
            owner.execute("GRANT autopilot_queue_bypass TO tenant_a_runtime")
            owner.commit()
        finally:
            owner.close()
        try:
            bypass = PostgresQueue(bypass_connect)
            with self.assertRaises(QueueError) as raised:
                bypass.enqueue(
                    tenant_id="tenant-a",
                    job_type="docs",
                    payload={"n": 1},
                    idempotency_key="bypass-path",
                )
            self.assertEqual(raised.exception.code, "runtime_privileged")
        finally:
            owner = _owner_connect(self.dsn)
            try:
                owner.execute("REVOKE autopilot_queue_bypass FROM tenant_a_runtime")
                owner.execute("DROP ROLE IF EXISTS autopilot_queue_bypass")
                owner.commit()
            finally:
                owner.close()

        def row_security_off():
            connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
            connection.execute("SET row_security = off")
            connection.commit()
            return connection

        closed = PostgresQueue(row_security_off)
        with self.assertRaises(QueueError) as raised:
            closed.enqueue(tenant_id="tenant-a", job_type="docs", payload={"n": 1}, idempotency_key="row-security-off")
        self.assertEqual(raised.exception.code, "runtime_privileged")

    def test_rejected_secret_is_not_stored(self) -> None:
        connection = _owner_connect(self.dsn)
        try:
            before = _cell(connection.execute("SELECT count(*) FROM autopilot_queue.jobs").fetchone())
            connection.rollback()
        finally:
            connection.close()
        with self.assertRaises(QueueError) as raised:
            self.queue.enqueue(
                tenant_id="tenant-a",
                job_type="docs",
                payload={"password": "hunter2"},
                idempotency_key="stored-secret",
            )
        self.assertEqual(raised.exception.code, "secret_rejected")
        connection = _owner_connect(self.dsn)
        try:
            after = _cell(connection.execute("SELECT count(*) FROM autopilot_queue.jobs").fetchone())
            connection.rollback()
        finally:
            connection.close()
        self.assertEqual(before, after)

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
            if self.runner.version() != 1:
                self.runner.up()
            _ensure_tenant_roles(self.dsn, self.runner)

    def test_rollback_blocks_foreign_objects_and_job_rows(self) -> None:
        created = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="keep-me",
        )
        with self.assertRaises(QueueError) as raised:
            self.runner.down()
        self.assertEqual(raised.exception.code, "migration_blocked")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, PENDING)
        connection = _owner_connect(self.dsn)
        try:
            forced = connection.execute(
                """
                SELECT c.relforcerowsecurity
                FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'autopilot_queue' AND c.relname = 'jobs'
                """
            ).fetchone()
            self.assertTrue(bool(_cell(forced)))
            connection.execute("CREATE TABLE autopilot_queue.foreign_marker (id integer)")
            connection.execute(
                "CREATE VIEW public.autopilot_queue_foreign_view AS SELECT job_id FROM autopilot_queue.jobs"
            )
            connection.commit()
            with self.assertRaises(QueueError) as raised:
                self.runner.down(allow_destructive_test=True)
            self.assertEqual(raised.exception.code, "migration_blocked")
            self.assertIsNotNone(connection.execute("SELECT to_regclass('autopilot_queue.foreign_marker')").fetchone()[0])
            connection.execute("DROP VIEW public.autopilot_queue_foreign_view")
            connection.execute("DROP TABLE autopilot_queue.foreign_marker")
            connection.commit()
            self.assertEqual(self.runner.down(allow_destructive_test=True), 0)
            self.runner.up()
        finally:
            connection.rollback()
            marker = connection.execute("SELECT to_regclass('autopilot_queue.foreign_marker')").fetchone()
            if _cell(marker):
                connection.execute("DROP TABLE autopilot_queue.foreign_marker")
            connection.execute("DROP VIEW IF EXISTS public.autopilot_queue_foreign_view")
            connection.commit()
            connection.close()
            if self.runner.version() != 1:
                self.runner.up()
            _ensure_tenant_roles(self.dsn, self.runner)

    def test_owned_sequence_blocks_rollback_and_is_preserved(self) -> None:
        def preserved(connection) -> None:
            sequence = connection.execute("SELECT to_regclass('autopilot_queue.owned_seq')").fetchone()
            column = connection.execute(
                """
                SELECT a.attname
                FROM pg_attribute a
                JOIN pg_class c ON c.oid = a.attrelid
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'autopilot_queue'
                  AND c.relname = 'jobs'
                  AND a.attname = 'extra_n'
                  AND NOT a.attisdropped
                """
            ).fetchone()
            owned = connection.execute(
                """
                SELECT dep.deptype
                FROM pg_depend dep
                JOIN pg_class seq ON seq.oid = dep.objid
                JOIN pg_namespace seq_ns ON seq_ns.oid = seq.relnamespace
                WHERE seq_ns.nspname = 'autopilot_queue'
                  AND seq.relname = 'owned_seq'
                  AND dep.deptype = 'a'
                """
            ).fetchone()
            self.assertIsNotNone(_cell(sequence))
            self.assertEqual(_cell(column), "extra_n")
            self.assertEqual(_cell(owned), "a")

        self._blocked_rollback(
            (
                "ALTER TABLE autopilot_queue.jobs ADD COLUMN extra_n integer",
                "CREATE SEQUENCE autopilot_queue.owned_seq",
                "ALTER SEQUENCE autopilot_queue.owned_seq OWNED BY autopilot_queue.jobs.extra_n",
            ),
            (
                "DROP SEQUENCE IF EXISTS autopilot_queue.owned_seq",
                "ALTER TABLE autopilot_queue.jobs DROP COLUMN IF EXISTS extra_n",
            ),
            preserved,
        )

    def test_standalone_sequence_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            sequence = connection.execute("SELECT to_regclass('autopilot_queue.standalone_seq')").fetchone()
            self.assertIsNotNone(_cell(sequence))

        self._blocked_rollback(
            ("CREATE SEQUENCE autopilot_queue.standalone_seq",),
            ("DROP SEQUENCE IF EXISTS autopilot_queue.standalone_seq",),
            preserved,
        )

    def test_extra_column_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            column = connection.execute(
                """
                SELECT a.attname
                FROM pg_attribute a
                JOIN pg_class c ON c.oid = a.attrelid
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'autopilot_queue' AND c.relname = 'jobs' AND a.attname = 'extra_n'
                """
            ).fetchone()
            self.assertEqual(_cell(column), "extra_n")

        self._blocked_rollback(
            ("ALTER TABLE autopilot_queue.jobs ADD COLUMN extra_n integer",),
            ("ALTER TABLE autopilot_queue.jobs DROP COLUMN IF EXISTS extra_n",),
            preserved,
        )

    def test_extra_index_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            index = connection.execute("SELECT to_regclass('autopilot_queue.jobs_extra_idx')").fetchone()
            self.assertIsNotNone(_cell(index))

        self._blocked_rollback(
            ("CREATE INDEX jobs_extra_idx ON autopilot_queue.jobs (priority)",),
            ("DROP INDEX IF EXISTS autopilot_queue.jobs_extra_idx",),
            preserved,
        )

    def test_extra_constraint_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            found = connection.execute(
                """
                SELECT con.conname
                FROM pg_constraint con
                JOIN pg_namespace n ON n.oid = con.connamespace
                WHERE n.nspname = 'autopilot_queue' AND con.conname = 'jobs_extra_check'
                """
            ).fetchone()
            self.assertEqual(_cell(found), "jobs_extra_check")

        self._blocked_rollback(
            ("ALTER TABLE autopilot_queue.jobs ADD CONSTRAINT jobs_extra_check CHECK (priority >= 0)",),
            ("ALTER TABLE autopilot_queue.jobs DROP CONSTRAINT IF EXISTS jobs_extra_check",),
            preserved,
        )

    def test_extra_trigger_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            trigger = connection.execute(
                """
                SELECT t.tgname
                FROM pg_trigger t
                JOIN pg_class c ON c.oid = t.tgrelid
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'autopilot_queue' AND t.tgname = 'jobs_extra' AND NOT t.tgisinternal
                """
            ).fetchone()
            function = connection.execute("SELECT to_regprocedure('autopilot_queue.extra_touch()')").fetchone()
            self.assertEqual(_cell(trigger), "jobs_extra")
            self.assertIsNotNone(_cell(function))

        self._blocked_rollback(
            (
                "CREATE FUNCTION autopilot_queue.extra_touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$",
                "CREATE TRIGGER jobs_extra BEFORE UPDATE ON autopilot_queue.jobs FOR EACH ROW EXECUTE FUNCTION autopilot_queue.extra_touch()",
            ),
            (
                "DROP TRIGGER IF EXISTS jobs_extra ON autopilot_queue.jobs",
                "DROP FUNCTION IF EXISTS autopilot_queue.extra_touch()",
            ),
            preserved,
        )

    def test_view_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            view = connection.execute("SELECT to_regclass('autopilot_queue.jobs_v')").fetchone()
            self.assertIsNotNone(_cell(view))

        self._blocked_rollback(
            ("CREATE VIEW autopilot_queue.jobs_v AS SELECT job_id FROM autopilot_queue.jobs",),
            ("DROP VIEW IF EXISTS autopilot_queue.jobs_v",),
            preserved,
        )

    def test_materialized_view_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            view = connection.execute("SELECT to_regclass('autopilot_queue.jobs_mv')").fetchone()
            self.assertIsNotNone(_cell(view))

        self._blocked_rollback(
            ("CREATE MATERIALIZED VIEW autopilot_queue.jobs_mv AS SELECT job_id FROM autopilot_queue.jobs",),
            ("DROP MATERIALIZED VIEW IF EXISTS autopilot_queue.jobs_mv",),
            preserved,
        )

    def test_composite_type_blocks_rollback(self) -> None:
        def preserved(connection) -> None:
            found = connection.execute("SELECT to_regtype('autopilot_queue.extra_pair')").fetchone()
            self.assertIsNotNone(_cell(found))

        self._blocked_rollback(
            ("CREATE TYPE autopilot_queue.extra_pair AS (left_n integer, right_n integer)",),
            ("DROP TYPE IF EXISTS autopilot_queue.extra_pair",),
            preserved,
        )

    def test_external_view_blocks_rollback_and_is_preserved(self) -> None:
        def preserved(connection) -> None:
            view = connection.execute("SELECT to_regclass('public.autopilot_queue_external_view')").fetchone()
            self.assertIsNotNone(_cell(view))

        self._blocked_rollback(
            ("CREATE VIEW public.autopilot_queue_external_view AS SELECT job_id FROM autopilot_queue.jobs",),
            ("DROP VIEW IF EXISTS public.autopilot_queue_external_view",),
            preserved,
            external=True,
        )

    def test_external_sql_function_blocks_rollback_and_is_preserved(self) -> None:
        def preserved(connection) -> None:
            found = connection.execute("SELECT to_regprocedure('public.autopilot_queue_sql_ref()')").fetchone()
            self.assertIsNotNone(_cell(found))

        self._blocked_rollback(
            (
                "CREATE FUNCTION public.autopilot_queue_sql_ref() RETURNS text LANGUAGE sql "
                "AS $$ SELECT job_id FROM autopilot_queue.jobs LIMIT 1 $$",
            ),
            ("DROP FUNCTION IF EXISTS public.autopilot_queue_sql_ref()",),
            preserved,
            external=True,
        )

    def test_external_plpgsql_function_blocks_rollback_and_is_preserved(self) -> None:
        def preserved(connection) -> None:
            found = connection.execute("SELECT to_regprocedure('public.autopilot_queue_plpgsql_ref()')").fetchone()
            self.assertIsNotNone(_cell(found))

        self._blocked_rollback(
            (
                "CREATE FUNCTION public.autopilot_queue_plpgsql_ref() RETURNS text LANGUAGE plpgsql "
                "AS $$ BEGIN RETURN (SELECT job_id FROM autopilot_queue.jobs LIMIT 1); END $$",
            ),
            ("DROP FUNCTION IF EXISTS public.autopilot_queue_plpgsql_ref()",),
            preserved,
            external=True,
        )

    def test_nonempty_queue_blocks_rollback(self) -> None:
        created = self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="rollback-keeps-row",
        )
        with self.assertRaises(QueueError) as raised:
            self.runner.down()
        self.assertEqual(raised.exception.code, "migration_blocked")
        self.assertEqual(self.queue.get_status("tenant-a", created.job_id).status, PENDING)
        connection = _owner_connect(self.dsn)
        try:
            versions = connection.execute("SELECT version FROM autopilot_queue.schema_migrations").fetchall()
            self.assertEqual([_cell(row) for row in versions], [1])
        finally:
            connection.close()

    def test_blocked_rollback_keeps_forced_rls(self) -> None:
        self.queue.enqueue(
            tenant_id="tenant-a",
            job_type="docs",
            payload={"n": 1},
            idempotency_key="rollback-keeps-rls",
        )
        with self.assertRaises(QueueError) as raised:
            self.runner.down()
        self.assertEqual(raised.exception.code, "migration_blocked")
        connection = _owner_connect(self.dsn)
        try:
            self._assert_version_and_rls(connection)
        finally:
            connection.close()

    def test_clean_rollback_down_up(self) -> None:
        try:
            self.assertEqual(self.runner.down(), 0)
            connection = _owner_connect(self.dsn)
            try:
                self.assertIsNone(_cell(connection.execute("SELECT to_regclass('autopilot_queue.jobs')").fetchone()))
                connection.rollback()
            finally:
                connection.close()
            self.assertEqual(self.runner.up(), 1)
            connection = _owner_connect(self.dsn)
            try:
                self._assert_version_and_rls(connection)
                columns = connection.execute(
                    """
                    SELECT count(*)
                    FROM pg_attribute a
                    JOIN pg_class c ON c.oid = a.attrelid
                    JOIN pg_namespace n ON n.oid = c.relnamespace
                    WHERE n.nspname = 'autopilot_queue'
                      AND c.relname = 'jobs'
                      AND a.attnum > 0
                      AND NOT a.attisdropped
                    """
                ).fetchone()
                self.assertEqual(int(_cell(columns)), 19)
                connection.rollback()
            finally:
                connection.close()
        finally:
            if self.runner.version() != 1:
                self.runner.up()
            _ensure_tenant_roles(self.dsn, self.runner)

    def test_runtime_role_cannot_delete_or_migrate(self) -> None:
        import psycopg

        connection = _owner_connect(self.dsn)
        try:
            connection.execute("SET ROLE autopilot_queue_runtime")
            connection.commit()
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

    def test_existing_login_role_is_rejected(self) -> None:
        self._reject_precreated_role(
            "CREATE ROLE autopilot_queue_runtime LOGIN PASSWORD 'login_test' "
            "NOSUPERUSER NOBYPASSRLS NOCREATEROLE NOCREATEDB NOREPLICATION",
            "rolcanlogin",
        )

    def test_existing_privileged_role_is_rejected(self) -> None:
        self._reject_precreated_role("CREATE ROLE autopilot_queue_runtime SUPERUSER LOGIN", "rolsuper")

    def test_existing_role_membership_is_rejected(self) -> None:
        self.runner.down()
        connection = _owner_connect(self.dsn)
        try:
            _detach_runtime(connection)
            connection.execute("DROP ROLE IF EXISTS autopilot_queue_runtime")
            connection.execute(
                "CREATE ROLE autopilot_queue_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS "
                "NOCREATEROLE NOCREATEDB NOREPLICATION"
            )
            connection.execute("GRANT pg_read_all_data TO autopilot_queue_runtime")
            connection.commit()
            with self.assertRaises(QueueError) as raised:
                self.runner.up()
            self.assertEqual(raised.exception.code, "role_rejected")
            membership = connection.execute(
                """
                SELECT granted.rolname
                FROM pg_auth_members m
                JOIN pg_roles granted ON granted.oid = m.roleid
                JOIN pg_roles member ON member.oid = m.member
                WHERE member.rolname = 'autopilot_queue_runtime'
                """
            ).fetchone()
            self.assertEqual(_cell(membership), "pg_read_all_data")
        finally:
            connection.close()
            self._restore_runtime()

    def _blocked_rollback(self, setup, cleanup, preserved, *, external: bool = False) -> None:
        connection = _owner_connect(self.dsn)
        try:
            for statement in setup:
                connection.execute(statement)
            connection.commit()
            with self.assertRaises(QueueError) as raised:
                self.runner.down()
            self.assertEqual(raised.exception.code, "migration_blocked")
            self._assert_version_and_rls(connection)
            preserved(connection)
        finally:
            try:
                connection.rollback()
                jobs = _cell(connection.execute("SELECT to_regclass('autopilot_queue.jobs')").fetchone())
                if jobs or external:
                    for statement in cleanup:
                        connection.execute(statement)
                    connection.commit()
            finally:
                connection.close()
                if self.runner.version() != 1:
                    self.runner.up()
                    _ensure_tenant_roles(self.dsn, self.runner)

    def _assert_version_and_rls(self, connection) -> None:
        versions = connection.execute("SELECT version FROM autopilot_queue.schema_migrations").fetchall()
        self.assertEqual([_cell(row) for row in versions], [1])
        flags = connection.execute(
            """
            SELECT c.relrowsecurity, c.relforcerowsecurity
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue' AND c.relname = 'jobs'
            """
        ).fetchone()
        self.assertTrue(bool(_cell(flags, 0, "relrowsecurity")))
        self.assertTrue(bool(_cell(flags, 1, "relforcerowsecurity")))
        self.assertIsNotNone(_cell(connection.execute("SELECT to_regclass('autopilot_queue.jobs')").fetchone()))

    def _reject_precreated_role(self, create_sql: str, column: str) -> None:
        self.runner.down()
        connection = _owner_connect(self.dsn)
        try:
            _detach_runtime(connection)
            connection.execute("DROP ROLE IF EXISTS autopilot_queue_runtime")
            connection.execute(create_sql)
            connection.commit()
            with self.assertRaises(QueueError) as raised:
                self.runner.up()
            self.assertEqual(raised.exception.code, "role_rejected")
            row = connection.execute(
                f"SELECT {column} FROM pg_roles WHERE rolname = 'autopilot_queue_runtime'"
            ).fetchone()
            self.assertTrue(bool(_cell(row)))
            self.assertIsNone(_cell(connection.execute("SELECT to_regclass('autopilot_queue.jobs')").fetchone()))
        finally:
            connection.close()
            self._restore_runtime()

    def _restore_runtime(self) -> None:
        connection = _owner_connect(self.dsn)
        try:
            _detach_runtime(connection)
            connection.execute("DROP ROLE IF EXISTS autopilot_queue_runtime")
            connection.commit()
        finally:
            connection.close()
        self.runner.up()
        _ensure_tenant_roles(self.dsn, self.runner)


if __name__ == "__main__":
    unittest.main()

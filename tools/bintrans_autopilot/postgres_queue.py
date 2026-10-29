"""PostgreSQL queue for autopilot phase 0.3B.

The 0.3A file lock is not used here. Claim commits before a handler runs.
Delivery is at-least-once: a stored row is not proof that an external side
effect ran once. This module does not start a worker, a Cloud Agent, a merge,
or a deploy.
"""

from __future__ import annotations

import hashlib
import json
import logging
import re
import threading
import uuid
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator

logger = logging.getLogger("bintrans_autopilot.postgres_queue")

PENDING = "PENDING"
RUNNING = "RUNNING"
SUCCEEDED = "SUCCEEDED"
FAILED = "FAILED"
DEAD = "DEAD"
STATUSES = (PENDING, RUNNING, SUCCEEDED, FAILED, DEAD)

_SQL_DIR = Path(__file__).resolve().parent / "sql" / "autopilot_queue"
_ID = re.compile(r"^[A-Za-z0-9._:-]{1,80}$")
_ERROR = re.compile(r"^[a-z0-9_]{1,80}$")
_SECRET = re.compile(r"(cursor_|eyJ|Bearer\s|api_key\s*=|password\s*=|POSTGRES_PASSWORD\s*=)", re.I)
_ADVISORY_LOCK = 831703


class QueueError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass
class QueueMetrics:
    enqueued: int = 0
    claimed: int = 0
    renewed: int = 0
    completed: int = 0
    failed: int = 0
    requeued: int = 0
    dead: int = 0
    rejected: int = 0


@dataclass(frozen=True)
class EnqueueResult:
    job_id: str
    tenant_id: str
    created: bool
    status: str


@dataclass(frozen=True)
class JobLease:
    job_id: str
    tenant_id: str
    lease_token: str
    lease_owner: str
    attempt_count: int
    status: str
    cloud_agents_started: int = 0
    external_effects: int = 0


@dataclass(frozen=True)
class JobStatus:
    job_id: str
    tenant_id: str
    job_type: str
    payload_version: int
    status: str
    attempt_count: int
    max_attempts: int
    last_error: str


def _sqlstate(exc: Exception) -> str:
    return str(getattr(exc, "sqlstate", None) or getattr(exc, "pgcode", None) or "")


def _map_db_error(exc: Exception) -> QueueError:
    state = _sqlstate(exc)
    if state == "42501":
        return QueueError("store_unauthorized")
    if state in {"3D000", "28000", "28P01", "08001", "08006", "57P03", "42P01"}:
        return QueueError("store_unavailable")
    name = type(exc).__name__.lower()
    if "operational" in name or "interface" in name or isinstance(exc, (OSError, ConnectionError, TimeoutError)):
        return QueueError("store_unavailable")
    return QueueError("store_failed")


def _statements(text: str) -> list[str]:
    lines = [line for line in text.splitlines() if not line.strip().startswith("--")]
    return [part.strip() for part in "\n".join(lines).split(";") if part.strip()]


def _reject_secrets(value: object) -> None:
    if isinstance(value, str):
        if _SECRET.search(value):
            raise QueueError("secret_rejected")
        return
    if isinstance(value, dict):
        for key, item in value.items():
            _reject_secrets(key)
            _reject_secrets(item)
        return
    if isinstance(value, (list, tuple)):
        for item in value:
            _reject_secrets(item)


def _identifier(value: str, code: str = "invalid_input") -> str:
    if not isinstance(value, str) or not _ID.fullmatch(value):
        raise QueueError(code)
    _reject_secrets(value)
    return value


def _digest(payload: dict) -> str:
    body = json.dumps(payload, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    if len(body.encode("utf-8")) > 65536:
        raise QueueError("invalid_input")
    return hashlib.sha256(body.encode("utf-8")).hexdigest()


def _one(cursor) -> dict | None:
    row = cursor.fetchone()
    if row is None:
        return None
    if isinstance(row, dict):
        return row
    columns = [item[0] if not hasattr(item, "name") else item.name for item in cursor.description]
    return dict(zip(columns, row))


class MigrationRunner:
    """Applies SQL files in the queue schema only.

    The platform migration table is not read or written.
    """

    def __init__(self, connect):
        self._connect = connect

    def version(self) -> int:
        with self._cursor() as cursor:
            cursor.execute("SELECT to_regclass('autopilot_queue.schema_migrations') AS name")
            found = _one(cursor)
            if not found or not found["name"]:
                return 0
            cursor.execute("SELECT COALESCE(MAX(version), 0) AS version FROM autopilot_queue.schema_migrations")
            current = _one(cursor)
        return int(current["version"]) if current else 0

    def up(self) -> int:
        files = sorted(_SQL_DIR.glob("*.up.sql"))
        if not files:
            raise QueueError("migration_failed")
        with self._cursor() as cursor:
            cursor.execute("SELECT pg_advisory_xact_lock(%s)", (_ADVISORY_LOCK,))
            applied = self._applied(cursor)
            self._ensure_role(cursor)
            for path in files:
                version = int(path.name.split("_", 1)[0])
                if version in applied:
                    continue
                for statement in _statements(path.read_text(encoding="utf-8")):
                    self._execute_statement(cursor, statement)
                cursor.execute(
                    "INSERT INTO autopilot_queue.schema_migrations (version, name) VALUES (%s, %s)",
                    (version, path.name),
                )
                applied.add(version)
            cursor.execute("SELECT COALESCE(MAX(version), 0) AS version FROM autopilot_queue.schema_migrations")
            current = _one(cursor)
        return int(current["version"]) if current else 0

    def down(self) -> int:
        files = sorted(_SQL_DIR.glob("*.down.sql"), reverse=True)
        with self._cursor() as cursor:
            cursor.execute("SELECT pg_advisory_xact_lock(%s)", (_ADVISORY_LOCK,))
            applied = self._applied(cursor)
            if not applied:
                return 0
            latest = max(applied)
            matches = [path for path in files if int(path.name.split("_", 1)[0]) == latest]
            if len(matches) != 1:
                raise QueueError("migration_failed")
            for statement in _statements(matches[0].read_text(encoding="utf-8")):
                cursor.execute(statement)
        return self.version()

    def _applied(self, cursor) -> set[int]:
        cursor.execute("SELECT to_regclass('autopilot_queue.schema_migrations') AS name")
        found = _one(cursor)
        if not found or not found["name"]:
            return set()
        cursor.execute("SELECT version FROM autopilot_queue.schema_migrations")
        return {int(row["version"]) for row in cursor.fetchall()}

    def _ensure_role(self, cursor) -> None:
        cursor.execute("SELECT 1 AS present FROM pg_roles WHERE rolname = 'autopilot_queue_runtime'")
        if _one(cursor) is None:
            cursor.execute("CREATE ROLE autopilot_queue_runtime NOLOGIN")

    def _execute_statement(self, cursor, statement: str) -> None:
        cursor.execute(statement)

    @contextmanager
    def _cursor(self) -> Iterator:
        try:
            connection = self._connect()
        except QueueError:
            raise
        except Exception as exc:
            raise QueueError("store_unavailable") from exc
        try:
            with _cursor_for(connection) as cursor:
                yield cursor
            connection.commit()
        except QueueError:
            connection.rollback()
            raise
        except Exception as exc:
            connection.rollback()
            mapped = _map_db_error(exc)
            if mapped.code == "store_failed":
                raise QueueError("migration_failed") from exc
            raise mapped from exc
        finally:
            connection.close()


class PostgresQueue:
    """Tenant-scoped queue. Construction stays closed unless both switches are off."""

    def __init__(self, connect, *, autopilot_enabled: bool = False, paid_automation_enabled: bool = False):
        if autopilot_enabled or paid_automation_enabled:
            raise QueueError("automation_disabled")
        self._connect = connect
        self.autopilot_enabled = False
        self.paid_automation_enabled = False
        self.metrics = QueueMetrics()
        self._metrics_lock = threading.Lock()

    def enqueue(
        self,
        *,
        tenant_id: str,
        job_type: str,
        payload: dict,
        idempotency_key: str,
        payload_version: int = 1,
        priority: int = 100,
        max_attempts: int = 3,
        backoff_seconds: int = 1,
    ) -> EnqueueResult:
        tenant_id = _identifier(tenant_id)
        job_type = _identifier(job_type)
        idempotency_key = _identifier(idempotency_key)
        if not isinstance(payload, dict) or isinstance(payload_version, bool) or payload_version < 1:
            raise QueueError("invalid_input")
        if isinstance(priority, bool) or not isinstance(priority, int):
            raise QueueError("invalid_input")
        if isinstance(max_attempts, bool) or not isinstance(max_attempts, int) or not 1 <= max_attempts <= 100:
            raise QueueError("invalid_input")
        if isinstance(backoff_seconds, bool) or not isinstance(backoff_seconds, int) or not 0 <= backoff_seconds <= 3600:
            raise QueueError("invalid_input")
        _reject_secrets(payload)
        digest = _digest(payload)
        body = json.dumps(payload, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
        job_id = str(uuid.uuid4())
        with self._cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO autopilot_queue.jobs (
                    job_id, tenant_id, job_type, payload_version, payload, payload_digest,
                    status, priority, created_at, available_at, attempt_count, max_attempts,
                    backoff_seconds, idempotency_key, updated_at
                )
                VALUES (
                    %s, %s, %s, %s, %s::jsonb, %s,
                    'PENDING', %s, clock_timestamp(), clock_timestamp(), 0, %s,
                    %s, %s, clock_timestamp()
                )
                ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
                RETURNING job_id, status
                """,
                (job_id, tenant_id, job_type, payload_version, body, digest, priority, max_attempts, backoff_seconds, idempotency_key),
            )
            created = _one(cursor)
            if created is None:
                cursor.execute(
                    """
                    SELECT job_id, status, payload_digest
                    FROM autopilot_queue.jobs
                    WHERE tenant_id = %s AND idempotency_key = %s
                    """,
                    (tenant_id, idempotency_key),
                )
                existing = _one(cursor)
                if existing is None:
                    raise QueueError("store_failed")
                if existing["payload_digest"] != digest:
                    raise QueueError("idempotency_conflict")
                self._count("rejected")
                self._log("enqueue", tenant_id, existing["job_id"], "unchanged")
                return EnqueueResult(existing["job_id"], tenant_id, False, existing["status"])
        self._count("enqueued")
        self._log("enqueue", tenant_id, created["job_id"], "created")
        return EnqueueResult(created["job_id"], tenant_id, True, created["status"])

    def claim(self, tenant_id: str, owner: str, lease_seconds: int = 30) -> JobLease | None:
        tenant_id = _identifier(tenant_id)
        owner = _identifier(owner)
        lease_seconds = _lease_seconds(lease_seconds)
        token = str(uuid.uuid4())
        with self._cursor() as cursor:
            cursor.execute(
                """
                SELECT job_id
                FROM autopilot_queue.jobs
                WHERE tenant_id = %s
                  AND status = 'PENDING'
                  AND available_at <= clock_timestamp()
                ORDER BY priority, created_at, job_id
                FOR UPDATE SKIP LOCKED
                LIMIT 1
                """,
                (tenant_id,),
            )
            chosen = _one(cursor)
            if chosen is None:
                self._log("claim", tenant_id, "", "empty")
                return None
            cursor.execute(
                """
                UPDATE autopilot_queue.jobs
                SET status = 'RUNNING',
                    attempt_count = attempt_count + 1,
                    lease_owner = %s,
                    lease_token = %s,
                    lease_expires_at = clock_timestamp() + make_interval(secs => %s),
                    updated_at = clock_timestamp()
                WHERE tenant_id = %s AND job_id = %s AND status = 'PENDING'
                RETURNING job_id, attempt_count, lease_token, lease_owner
                """,
                (owner, token, lease_seconds, tenant_id, chosen["job_id"]),
            )
            claimed = _one(cursor)
            if claimed is None:
                raise QueueError("invalid_state")
        self._count("claimed")
        self._log("claim", tenant_id, claimed["job_id"], "running")
        return JobLease(
            job_id=claimed["job_id"],
            tenant_id=tenant_id,
            lease_token=claimed["lease_token"],
            lease_owner=claimed["lease_owner"],
            attempt_count=int(claimed["attempt_count"]),
            status=RUNNING,
        )

    def renew_lease(self, tenant_id: str, job_id: str, owner: str, lease_token: str, lease_seconds: int = 30) -> JobLease:
        tenant_id, job_id, owner, lease_token = _lease_args(tenant_id, job_id, owner, lease_token)
        lease_seconds = _lease_seconds(lease_seconds)
        with self._cursor() as cursor:
            cursor.execute(
                """
                UPDATE autopilot_queue.jobs
                SET lease_expires_at = clock_timestamp() + make_interval(secs => %s),
                    updated_at = clock_timestamp()
                WHERE tenant_id = %s AND job_id = %s AND lease_owner = %s AND lease_token = %s
                  AND status = 'RUNNING' AND lease_expires_at > clock_timestamp()
                RETURNING attempt_count
                """,
                (lease_seconds, tenant_id, job_id, owner, lease_token),
            )
            renewed = _one(cursor)
            if renewed is None:
                raise self._classify(cursor, tenant_id, job_id, owner, lease_token)
        self._count("renewed")
        self._log("renew", tenant_id, job_id, "running")
        return JobLease(job_id, tenant_id, lease_token, owner, int(renewed["attempt_count"]), RUNNING)

    def complete(self, tenant_id: str, job_id: str, owner: str, lease_token: str) -> JobStatus:
        return self._finish(tenant_id, job_id, owner, lease_token, SUCCEEDED, "")

    def fail(self, tenant_id: str, job_id: str, owner: str, lease_token: str, error_code: str, *, retryable: bool = True) -> JobStatus:
        tenant_id, job_id, owner, lease_token = _lease_args(tenant_id, job_id, owner, lease_token)
        if not isinstance(error_code, str) or not _ERROR.fullmatch(error_code):
            raise QueueError("invalid_input")
        _reject_secrets(error_code)
        with self._cursor() as cursor:
            row = self._lock_running(cursor, tenant_id, job_id, owner, lease_token)
            terminal = (not retryable) or int(row["attempt_count"]) >= int(row["max_attempts"])
            status = FAILED if not retryable else DEAD if terminal else PENDING
            cursor.execute(
                """
                UPDATE autopilot_queue.jobs
                SET status = %s,
                    lease_owner = NULL,
                    lease_token = NULL,
                    lease_expires_at = NULL,
                    last_error = %s,
                    available_at = CASE
                        WHEN %s = 'PENDING' THEN clock_timestamp() + make_interval(secs => LEAST(3600, backoff_seconds * (2 ^ GREATEST(attempt_count - 1, 0)))::int)
                        ELSE available_at
                    END,
                    updated_at = clock_timestamp()
                WHERE tenant_id = %s AND job_id = %s AND lease_token = %s AND status = 'RUNNING'
                RETURNING job_type, payload_version, status, attempt_count, max_attempts, last_error
                """,
                (status, error_code, status, tenant_id, job_id, lease_token),
            )
            saved = _one(cursor)
            if saved is None:
                raise QueueError("invalid_state")
        self._count("dead" if saved["status"] == DEAD else "failed")
        self._log("fail", tenant_id, job_id, saved["status"])
        return _status(saved, job_id, tenant_id)

    def requeue_expired(self, tenant_id: str) -> int:
        tenant_id = _identifier(tenant_id)
        moved = 0
        with self._cursor() as cursor:
            cursor.execute(
                """
                SELECT job_id, attempt_count, max_attempts
                FROM autopilot_queue.jobs
                WHERE tenant_id = %s
                  AND status = 'RUNNING'
                  AND lease_expires_at <= clock_timestamp()
                ORDER BY job_id
                FOR UPDATE SKIP LOCKED
                LIMIT 100
                """,
                (tenant_id,),
            )
            rows = cursor.fetchall()
            for raw in rows:
                row = raw if isinstance(raw, dict) else None
                if row is None:
                    columns = [item[0] if not hasattr(item, "name") else item.name for item in cursor.description]
                    row = dict(zip(columns, raw))
                status = DEAD if int(row["attempt_count"]) >= int(row["max_attempts"]) else PENDING
                cursor.execute(
                    """
                    UPDATE autopilot_queue.jobs
                    SET status = %s,
                        lease_owner = NULL,
                        lease_token = NULL,
                        lease_expires_at = NULL,
                        last_error = 'lease_expired',
                        available_at = CASE
                            WHEN %s = 'PENDING' THEN clock_timestamp() + make_interval(secs => LEAST(3600, backoff_seconds * (2 ^ GREATEST(attempt_count - 1, 0)))::int)
                            ELSE available_at
                        END,
                        updated_at = clock_timestamp()
                    WHERE tenant_id = %s AND job_id = %s AND status = 'RUNNING'
                    """,
                    (status, status, tenant_id, row["job_id"]),
                )
                moved += 1
                self._count("dead" if status == DEAD else "requeued")
                self._log("requeue", tenant_id, row["job_id"], status)
        return moved

    def get_status(self, tenant_id: str, job_id: str) -> JobStatus | None:
        tenant_id = _identifier(tenant_id)
        if not isinstance(job_id, str) or not job_id:
            raise QueueError("invalid_input")
        _reject_secrets(job_id)
        with self._cursor() as cursor:
            cursor.execute(
                """
                SELECT job_id, job_type, payload_version, status, attempt_count, max_attempts, last_error
                FROM autopilot_queue.jobs
                WHERE tenant_id = %s AND job_id = %s
                """,
                (tenant_id, job_id),
            )
            row = _one(cursor)
        if row is None:
            return None
        return _status(row, row["job_id"], tenant_id)

    def _finish(self, tenant_id: str, job_id: str, owner: str, lease_token: str, status: str, error_code: str) -> JobStatus:
        tenant_id, job_id, owner, lease_token = _lease_args(tenant_id, job_id, owner, lease_token)
        with self._cursor() as cursor:
            cursor.execute(
                """
                UPDATE autopilot_queue.jobs
                SET status = %s,
                    lease_owner = NULL,
                    lease_token = NULL,
                    lease_expires_at = NULL,
                    last_error = NULLIF(%s, ''),
                    updated_at = clock_timestamp()
                WHERE tenant_id = %s AND job_id = %s AND lease_owner = %s AND lease_token = %s
                  AND status = 'RUNNING' AND lease_expires_at > clock_timestamp()
                RETURNING job_type, payload_version, status, attempt_count, max_attempts, last_error
                """,
                (status, error_code, tenant_id, job_id, owner, lease_token),
            )
            saved = _one(cursor)
            if saved is None:
                raise self._classify(cursor, tenant_id, job_id, owner, lease_token)
        self._count("completed")
        self._log("complete", tenant_id, job_id, status)
        return _status(saved, job_id, tenant_id)

    def _lock_running(self, cursor, tenant_id: str, job_id: str, owner: str, lease_token: str) -> dict:
        cursor.execute(
            """
            SELECT attempt_count, max_attempts, lease_owner, lease_token,
                   lease_expires_at > clock_timestamp() AS lease_valid, status
            FROM autopilot_queue.jobs
            WHERE tenant_id = %s AND job_id = %s
            FOR UPDATE
            """,
            (tenant_id, job_id),
        )
        row = _one(cursor)
        if row is None:
            raise QueueError("not_found")
        if row["status"] != RUNNING:
            raise QueueError("invalid_state")
        if row["lease_owner"] != owner:
            raise QueueError("not_owner")
        if row["lease_token"] != lease_token:
            raise QueueError("stale_lease")
        if not row["lease_valid"]:
            raise QueueError("lease_expired")
        return row

    def _classify(self, cursor, tenant_id: str, job_id: str, owner: str, lease_token: str) -> QueueError:
        cursor.execute(
            """
            SELECT status, lease_owner, lease_token, lease_expires_at <= clock_timestamp() AS expired
            FROM autopilot_queue.jobs
            WHERE tenant_id = %s AND job_id = %s
            """,
            (tenant_id, job_id),
        )
        row = _one(cursor)
        if row is None:
            return QueueError("not_found")
        if row["lease_token"] != lease_token:
            return QueueError("stale_lease")
        if row["lease_owner"] != owner:
            return QueueError("not_owner")
        if row["expired"]:
            return QueueError("lease_expired")
        if row["status"] != RUNNING:
            return QueueError("invalid_state")
        return QueueError("invalid_state")

    def _count(self, name: str) -> None:
        with self._metrics_lock:
            setattr(self.metrics, name, getattr(self.metrics, name) + 1)

    def _log(self, event: str, tenant_id: str, job_id: str, result: str) -> None:
        logger.info("event=%s tenant=%s job=%s result=%s", event, tenant_id, job_id, result)

    @contextmanager
    def _cursor(self) -> Iterator:
        try:
            connection = self._connect()
        except QueueError:
            raise
        except Exception as exc:
            raise QueueError("store_unavailable") from exc
        try:
            with _cursor_for(connection) as cursor:
                yield cursor
            connection.commit()
        except QueueError:
            connection.rollback()
            raise
        except Exception as exc:
            connection.rollback()
            raise _map_db_error(exc) from exc
        finally:
            connection.close()


class DryRunWorker:
    """One in-process dry-run step. It is not a resident worker and it makes no external call."""

    def __init__(self, queue: PostgresQueue):
        if queue.autopilot_enabled or queue.paid_automation_enabled:
            raise QueueError("automation_disabled")
        self.queue = queue
        self.handled = 0
        self.external_calls = 0

    def handle_once(self, tenant_id: str, owner: str, lease_seconds: int = 30) -> JobLease | None:
        lease = self.queue.claim(tenant_id, owner, lease_seconds)
        if lease is None:
            return None
        self.handled += 1
        self.queue.complete(lease.tenant_id, lease.job_id, lease.lease_owner, lease.lease_token)
        return lease


def _lease_seconds(value: int) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or not 1 <= value <= 3600:
        raise QueueError("invalid_input")
    return value


def _lease_args(tenant_id: str, job_id: str, owner: str, lease_token: str) -> tuple[str, str, str, str]:
    tenant_id = _identifier(tenant_id)
    owner = _identifier(owner)
    if not isinstance(job_id, str) or not job_id or not isinstance(lease_token, str) or not lease_token:
        raise QueueError("invalid_input")
    _reject_secrets(job_id)
    _reject_secrets(lease_token)
    return tenant_id, job_id, owner, lease_token


def _status(row: dict, job_id: str, tenant_id: str) -> JobStatus:
    return JobStatus(
        job_id=job_id,
        tenant_id=tenant_id,
        job_type=row["job_type"],
        payload_version=int(row["payload_version"]),
        status=row["status"],
        attempt_count=int(row["attempt_count"]),
        max_attempts=int(row["max_attempts"]),
        last_error=row["last_error"] or "",
    )


def _cursor_for(connection):
    try:
        from psycopg.rows import dict_row

        return connection.cursor(row_factory=dict_row)
    except Exception:
        return connection.cursor()

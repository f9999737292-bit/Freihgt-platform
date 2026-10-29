"""PostgreSQL queue for autopilot phase 0.3B.

The 0.3A file lock is not used here. Claim commits before a handler runs.
Delivery is at-least-once: a stored row is not proof that an external side
effect ran once. This module does not start a worker, a Cloud Agent, a merge,
or a deploy.

Tenant scope is the authenticated database principal recorded in
autopilot_queue.tenant_principals. The queue does not trust a caller-supplied
tenant id or a custom GUC.
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
RUNTIME_ROLE = "autopilot_queue_runtime"

_SQL_DIR = Path(__file__).resolve().parent / "sql" / "autopilot_queue"
_ID = re.compile(r"^[A-Za-z0-9._:-]{1,80}$")
_ROLE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]{0,62}$")
_ERROR = re.compile(r"^[a-z0-9_]{1,80}$")
_MARKER = re.compile(r"^[a-z0-9][a-z0-9._:-]{0,63}$")
_USERINFO = re.compile(r"^[a-z][a-z0-9+.-]*://[^\s/@]+(?::[^\s/@]*)?@", re.I)
_LIBPQ_SECRET = re.compile(r"(?:password|pwd)\s*=", re.I)
_BEARER = re.compile(r"bearer\s+\S", re.I)
_TOKEN_PREFIX = re.compile(r"(cursor_|eyJ|ghp_|github_pat_|sk-|xox)", re.I)
_ADVISORY_LOCK = 831703
_TEST_DATABASE = re.compile(r"[a-z0-9_]+_test")
_UNSAFE_DATABASE = re.compile(r"prod|stag|freight|platform|postgres")
_EXPECTED_TABLES = ["jobs", "schema_migrations", "tenant_principals"]
_EXPECTED_COLUMNS = {
    "jobs": (
        ("job_id", "text", True, ""),
        ("tenant_id", "text", True, ""),
        ("job_type", "text", True, ""),
        ("payload_version", "integer", True, ""),
        ("payload", "jsonb", True, ""),
        ("payload_digest", "text", True, ""),
        ("status", "text", True, ""),
        ("priority", "integer", True, ""),
        ("created_at", "timestamp with time zone", True, ""),
        ("available_at", "timestamp with time zone", True, ""),
        ("attempt_count", "integer", True, ""),
        ("max_attempts", "integer", True, ""),
        ("backoff_seconds", "integer", True, ""),
        ("lease_owner", "text", False, ""),
        ("lease_token", "text", False, ""),
        ("lease_expires_at", "timestamp with time zone", False, ""),
        ("last_error", "text", False, ""),
        ("idempotency_key", "text", True, ""),
        ("updated_at", "timestamp with time zone", True, ""),
    ),
    "schema_migrations": (
        ("version", "integer", True, ""),
        ("name", "text", True, ""),
        ("applied_at", "timestamp with time zone", True, "clock_timestamp()"),
    ),
    "tenant_principals": (
        ("tenant_id", "text", True, ""),
        ("role_name", "text", True, ""),
    ),
}
_EXPECTED_CONSTRAINTS = {
    ("jobs", "jobs_attempts_check"): (
        "c",
        "CHECK (((attempt_count >= 0) AND (max_attempts >= 1) AND (attempt_count <= max_attempts)))",
    ),
    ("jobs", "jobs_lease_token_unique"): ("u", "UNIQUE (lease_token)"),
    ("jobs", "jobs_payload_version_check"): ("c", "CHECK ((payload_version >= 1))"),
    ("jobs", "jobs_pkey"): ("p", "PRIMARY KEY (job_id)"),
    ("jobs", "jobs_status_check"): (
        "c",
        "CHECK ((status = ANY (ARRAY['PENDING'::text, 'RUNNING'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'DEAD'::text])))",
    ),
    ("jobs", "jobs_tenant_idem"): ("u", "UNIQUE (tenant_id, idempotency_key)"),
    ("schema_migrations", "schema_migrations_pkey"): ("p", "PRIMARY KEY (version)"),
    ("tenant_principals", "tenant_principals_pkey"): ("p", "PRIMARY KEY (tenant_id)"),
    ("tenant_principals", "tenant_principals_role_check"): (
        "c",
        "CHECK ((role_name ~ '^[A-Za-z_][A-Za-z0-9_]{0,62}$'::text))",
    ),
    ("tenant_principals", "tenant_principals_role_name_key"): ("u", "UNIQUE (role_name)"),
    ("tenant_principals", "tenant_principals_tenant_check"): (
        "c",
        "CHECK ((tenant_id ~ '^[A-Za-z0-9._:-]{1,80}$'::text))",
    ),
}
_EXPECTED_INDEXES = {
    "jobs_claim_idx": (
        "CREATE INDEX jobs_claim_idx ON autopilot_queue.jobs USING btree "
        "(tenant_id, priority, created_at, job_id) WHERE (status = 'PENDING'::text)"
    ),
    "jobs_lease_token_unique": (
        "CREATE UNIQUE INDEX jobs_lease_token_unique ON autopilot_queue.jobs USING btree (lease_token)"
    ),
    "jobs_pkey": "CREATE UNIQUE INDEX jobs_pkey ON autopilot_queue.jobs USING btree (job_id)",
    "jobs_tenant_idem": (
        "CREATE UNIQUE INDEX jobs_tenant_idem ON autopilot_queue.jobs USING btree (tenant_id, idempotency_key)"
    ),
    "schema_migrations_pkey": (
        "CREATE UNIQUE INDEX schema_migrations_pkey ON autopilot_queue.schema_migrations USING btree (version)"
    ),
    "tenant_principals_pkey": (
        "CREATE UNIQUE INDEX tenant_principals_pkey ON autopilot_queue.tenant_principals USING btree (tenant_id)"
    ),
    "tenant_principals_role_name_key": (
        "CREATE UNIQUE INDEX tenant_principals_role_name_key ON autopilot_queue.tenant_principals "
        "USING btree (role_name)"
    ),
}
_EXPECTED_TRIGGERS = {
    ("jobs", "jobs_tenant_immutable"): (
        "CREATE TRIGGER jobs_tenant_immutable BEFORE UPDATE OF tenant_id ON autopilot_queue.jobs "
        "FOR EACH ROW EXECUTE FUNCTION autopilot_queue.reject_tenant_change()"
    ),
}
_EXPECTED_POLICIES = {
    "jobs_insert": ("a", "", "(tenant_id = autopilot_queue.bound_tenant())"),
    "jobs_select": ("r", "(tenant_id = autopilot_queue.bound_tenant())", ""),
    "jobs_update": (
        "w",
        "(tenant_id = autopilot_queue.bound_tenant())",
        "(tenant_id = autopilot_queue.bound_tenant())",
    ),
}
_EXPECTED_RLS = {
    "jobs": (True, True),
    "schema_migrations": (False, False),
    "tenant_principals": (False, False),
}
_EXPECTED_ROUTINES = {
    "bound_tenant": {
        "kind": "f",
        "language": "sql",
        "arguments": "",
        "result": "text",
        "volatile": "s",
        "definer": True,
        "config": "search_path=pg_catalog",
        "source": (
            "SELECT principals.tenant_id "
            "FROM autopilot_queue.tenant_principals AS principals "
            "WHERE principals.role_name = session_user"
        ),
    },
    "reject_tenant_change": {
        "kind": "f",
        "language": "plpgsql",
        "arguments": "",
        "result": "trigger",
        "volatile": "v",
        "definer": False,
        "config": "search_path=pg_catalog",
        "source": (
            "BEGIN "
            "IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN "
            "RAISE EXCEPTION 'tenant_immutable' USING ERRCODE = '42501'; "
            "END IF; "
            "RETURN NEW; "
            "END;"
        ),
    },
}
_PAYLOAD_SCHEMAS = {
    "docs": {"marker": "marker", "n": "count"},
    "ping": {"n": "count"},
}
_SECRET_PARTS = {
    "password",
    "passwd",
    "pwd",
    "token",
    "secret",
    "credential",
    "credentials",
    "authorization",
    "auth",
    "dsn",
    "apikey",
}
_ROLE_FLAGS = (
    "rolcanlogin",
    "rolsuper",
    "rolbypassrls",
    "rolcreaterole",
    "rolcreatedb",
    "rolreplication",
)


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


def _safe_code(exc: Exception) -> str:
    state = _sqlstate(exc)
    if state == "42501":
        return "store_unauthorized"
    if state in {"3D000", "28000", "28P01", "08001", "08006", "57P03", "42P01"}:
        return "store_unavailable"
    name = type(exc).__name__.lower()
    if "operational" in name or "interface" in name or isinstance(exc, (OSError, ConnectionError, TimeoutError)):
        return "store_unavailable"
    return "store_failed"


def _statements(text: str) -> list[str]:
    lines = [line for line in text.splitlines() if not line.strip().startswith("--")]
    body = "\n".join(lines)
    parts: list[str] = []
    current: list[str] = []
    dollar = None
    index = 0
    while index < len(body):
        if dollar is not None:
            if body.startswith(dollar, index):
                current.append(dollar)
                index += len(dollar)
                dollar = None
                continue
            current.append(body[index])
            index += 1
            continue
        if body[index] == "$":
            match = re.match(r"\$[A-Za-z0-9_]*\$", body[index:])
            if match:
                dollar = match.group(0)
                current.append(dollar)
                index += len(dollar)
                continue
        if body[index] == ";":
            statement = "".join(current).strip()
            if statement:
                parts.append(statement)
            current = []
            index += 1
            continue
        current.append(body[index])
        index += 1
    tail = "".join(current).strip()
    if tail:
        parts.append(tail)
    return parts


def _normalize_key(key: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", key.lower()).strip("_")


def _key_is_secret(key: str) -> bool:
    normal = _normalize_key(key)
    if not normal:
        return False
    parts = set(normal.split("_"))
    if parts & _SECRET_PARTS:
        return True
    if "api" in parts and "key" in parts:
        return True
    if "private" in parts and "key" in parts:
        return True
    if "access" in parts and ("token" in parts or "key" in parts):
        return True
    if "database" in parts and "url" in parts:
        return True
    if "connection" in parts and ("string" in parts or "url" in parts):
        return True
    if "client" in parts and "secret" in parts:
        return True
    return False


def _value_has_embedded_credential(value: str) -> bool:
    return bool(_USERINFO.search(value) or _LIBPQ_SECRET.search(value) or _BEARER.search(value) or _TOKEN_PREFIX.search(value))


def _reject_secrets(value: object, *, key: str | None = None) -> None:
    if key is not None:
        if not isinstance(key, str) or _key_is_secret(key):
            raise QueueError("secret_rejected")
    if isinstance(value, str):
        if _value_has_embedded_credential(value):
            raise QueueError("secret_rejected")
        return
    if isinstance(value, dict):
        for item_key, item in value.items():
            _reject_secrets(item, key=item_key)
        return
    if isinstance(value, (list, tuple)):
        for item in value:
            _reject_secrets(item)
        return
    if value is None or isinstance(value, bool) or isinstance(value, (int, float)):
        return
    raise QueueError("invalid_input")


def _validate_payload(job_type: str, payload: dict) -> None:
    schema = _PAYLOAD_SCHEMAS.get(job_type)
    if schema is None or not payload or set(payload) - set(schema):
        raise QueueError("invalid_input")
    for field, kind in schema.items():
        if field not in payload:
            continue
        value = payload[field]
        if kind == "marker":
            if not isinstance(value, str) or not _MARKER.fullmatch(value):
                raise QueueError("invalid_input")
        elif isinstance(value, bool) or not isinstance(value, int) or not 0 <= value <= 1_000_000:
            raise QueueError("invalid_input")
    if job_type == "ping" and "n" not in payload:
        raise QueueError("invalid_input")


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


def _rows(cursor) -> list[dict]:
    description = cursor.description
    rows = []
    for item in cursor.fetchall():
        if isinstance(item, dict):
            rows.append(item)
            continue
        columns = [column[0] if not hasattr(column, "name") else column.name for column in description]
        rows.append(dict(zip(columns, item)))
    return rows


def _same_execution(existing: dict, expected: dict) -> bool:
    for name, value in expected.items():
        current = existing[name]
        if name in {"job_type", "payload_digest"}:
            if current != value:
                return False
            continue
        if int(current) != int(value):
            return False
    return True


def _normalize(value) -> str:
    return " ".join(str(value or "").split())


class MigrationRunner:
    """Applies SQL files in the queue schema only.

    The platform migration table is not read or written. down() compares the
    catalogs and drops objects in one transaction. pg_depend does not show
    dynamic SQL, so schema rollback stays limited to an isolated test database.
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
                    cursor.execute(statement)
                cursor.execute(
                    "INSERT INTO autopilot_queue.schema_migrations (version, name) VALUES (%s, %s)",
                    (version, path.name),
                )
                applied.add(version)
            cursor.execute("SELECT COALESCE(MAX(version), 0) AS version FROM autopilot_queue.schema_migrations")
            current = _one(cursor)
        return int(current["version"]) if current else 0

    def down(self, *, allow_destructive_test: bool = False) -> int:
        files = sorted(_SQL_DIR.glob("*.down.sql"), reverse=True)
        with self._cursor() as cursor:
            cursor.execute("SELECT pg_advisory_xact_lock(%s)", (_ADVISORY_LOCK,))
            applied = self._applied(cursor)
            if not applied:
                return 0
            if applied != {1}:
                raise QueueError("migration_blocked")
            self._lock_queue_tables(cursor)
            self._assert_down_inventory(cursor)
            self._assert_isolated_test_database(cursor)
            if not allow_destructive_test and self._job_count(cursor):
                raise QueueError("migration_blocked")
            matches = [path for path in files if int(path.name.split("_", 1)[0]) == 1]
            if len(matches) != 1:
                raise QueueError("migration_failed")
            for statement in _statements(matches[0].read_text(encoding="utf-8")):
                cursor.execute(statement)
        return self.version()

    def register_principal(self, tenant_id: str, role_name: str) -> None:
        tenant_id = _identifier(tenant_id)
        if not isinstance(role_name, str) or not _ROLE.fullmatch(role_name) or role_name == RUNTIME_ROLE:
            raise QueueError("role_rejected")
        with self._cursor() as cursor:
            self._assert_migration_authority(cursor)
            attributes = self._role_attributes(cursor, role_name)
            if attributes is None or not attributes["rolcanlogin"] or not attributes["rolinherit"]:
                raise QueueError("role_rejected")
            if any(attributes[name] for name in _ROLE_FLAGS if name != "rolcanlogin"):
                raise QueueError("role_rejected")
            memberships = self._memberships(cursor, role_name)
            if any(name != RUNTIME_ROLE for name in memberships):
                raise QueueError("role_rejected")
            owner = self._table_owner(cursor)
            if role_name == owner:
                raise QueueError("role_rejected")
            if RUNTIME_ROLE not in memberships:
                self._grant_runtime(cursor, role_name)
            cursor.execute(
                "SELECT role_name FROM autopilot_queue.tenant_principals WHERE tenant_id = %s",
                (tenant_id,),
            )
            current = _one(cursor)
            if current is not None and current["role_name"] != role_name:
                raise QueueError("role_rejected")
            cursor.execute(
                "SELECT tenant_id FROM autopilot_queue.tenant_principals WHERE role_name = %s",
                (role_name,),
            )
            reverse = _one(cursor)
            if reverse is not None and reverse["tenant_id"] != tenant_id:
                raise QueueError("role_rejected")
            if current is None:
                cursor.execute(
                    "INSERT INTO autopilot_queue.tenant_principals (tenant_id, role_name) VALUES (%s, %s)",
                    (tenant_id, role_name),
                )

    def _applied(self, cursor) -> set[int]:
        cursor.execute("SELECT to_regclass('autopilot_queue.schema_migrations') AS name")
        found = _one(cursor)
        if not found or not found["name"]:
            return set()
        cursor.execute("SELECT version FROM autopilot_queue.schema_migrations")
        return {int(row["version"]) for row in _rows(cursor)}

    def _ensure_role(self, cursor) -> None:
        attributes = self._role_attributes(cursor, RUNTIME_ROLE)
        if attributes is None:
            cursor.execute(
                "CREATE ROLE autopilot_queue_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS "
                "NOCREATEROLE NOCREATEDB NOREPLICATION"
            )
            return
        if any(attributes[name] for name in _ROLE_FLAGS):
            raise QueueError("role_rejected")
        if self._memberships(cursor, RUNTIME_ROLE):
            raise QueueError("role_rejected")

    def _assert_migration_authority(self, cursor) -> None:
        cursor.execute("SELECT session_user AS session_user")
        session = _one(cursor)
        if session is None or session["session_user"] == RUNTIME_ROLE:
            raise QueueError("runtime_privileged")
        cursor.execute(
            "SELECT 1 AS present FROM autopilot_queue.tenant_principals WHERE role_name = session_user"
        )
        if _one(cursor) is not None:
            raise QueueError("runtime_privileged")
        attributes = self._role_attributes(cursor, session["session_user"])
        owner = self._table_owner(cursor)
        if session["session_user"] != owner and not (attributes and attributes["rolsuper"]):
            raise QueueError("store_unauthorized")

    def _lock_queue_tables(self, cursor) -> None:
        for name in _EXPECTED_TABLES:
            cursor.execute("SELECT to_regclass(%s) AS name", (f"autopilot_queue.{name}",))
            found = _one(cursor)
            if not found or not found["name"]:
                raise QueueError("migration_blocked")
        cursor.execute(
            "LOCK TABLE autopilot_queue.jobs, autopilot_queue.schema_migrations, "
            "autopilot_queue.tenant_principals IN ACCESS EXCLUSIVE MODE"
        )

    def _assert_down_inventory(self, cursor) -> None:
        """Refuse rollback unless version 1 still has exactly its own structure.

        Catalog matches are structural: names alone are not enough. Sequences
        are read from pg_class and from pg_depend, including ownership
        (deptype a) and internal sequence dependencies. Dynamic SQL and names
        buried in strings are not treated as detected. Those references are
        why down() also requires an isolated test database.
        """
        if not self._relations_match(cursor):
            raise QueueError("migration_blocked")
        if not self._columns_match(cursor):
            raise QueueError("migration_blocked")
        if not self._constraints_match(cursor):
            raise QueueError("migration_blocked")
        if not self._indexes_match(cursor):
            raise QueueError("migration_blocked")
        if not self._triggers_match(cursor):
            raise QueueError("migration_blocked")
        if not self._policies_match(cursor):
            raise QueueError("migration_blocked")
        if not self._routines_match(cursor):
            raise QueueError("migration_blocked")
        if not self._types_match(cursor):
            raise QueueError("migration_blocked")
        if not self._sequences_match(cursor):
            raise QueueError("migration_blocked")
        if self._external_dependencies(cursor):
            raise QueueError("migration_blocked")

    def _relations_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relkind AS relkind, c.relname AS relname
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue'
            """
        )
        relations = {(row["relkind"], row["relname"]) for row in _rows(cursor)}
        expected = {("r", name) for name in _EXPECTED_TABLES}
        expected.update(("i", name) for name in _EXPECTED_INDEXES)
        return relations == expected

    def _columns_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relname AS relname,
                   a.attnum AS attnum,
                   a.attname AS attname,
                   format_type(a.atttypid, a.atttypmod) AS type_name,
                   a.attnotnull AS attnotnull,
                   pg_get_expr(d.adbin, d.adrelid) AS default_expr
            FROM pg_attribute a
            JOIN pg_class c ON c.oid = a.attrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
            WHERE n.nspname = 'autopilot_queue'
              AND c.relkind = 'r'
              AND a.attnum > 0
              AND NOT a.attisdropped
            ORDER BY c.relname, a.attnum
            """
        )
        actual: dict[str, list[tuple]] = {name: [] for name in _EXPECTED_COLUMNS}
        for row in _rows(cursor):
            bucket = actual.get(row["relname"])
            if bucket is None:
                return False
            bucket.append(
                (
                    row["attname"],
                    _normalize(row["type_name"]),
                    bool(row["attnotnull"]),
                    _normalize(row["default_expr"]),
                )
            )
        return actual == {name: list(columns) for name, columns in _EXPECTED_COLUMNS.items()}

    def _constraints_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relname AS relname,
                   con.conname AS conname,
                   con.contype AS contype,
                   pg_get_constraintdef(con.oid) AS definition
            FROM pg_constraint con
            JOIN pg_class c ON c.oid = con.conrelid
            JOIN pg_namespace n ON n.oid = con.connamespace
            WHERE n.nspname = 'autopilot_queue'
            """
        )
        actual = {
            (row["relname"], row["conname"]): (_normalize(row["contype"]), _normalize(row["definition"]))
            for row in _rows(cursor)
        }
        expected = {
            key: (_normalize(kind), _normalize(definition))
            for key, (kind, definition) in _EXPECTED_CONSTRAINTS.items()
        }
        return actual == expected

    def _indexes_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relname AS relname, pg_get_indexdef(c.oid) AS definition
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue' AND c.relkind = 'i'
            """
        )
        actual = {row["relname"]: _normalize(row["definition"]) for row in _rows(cursor)}
        expected = {name: _normalize(definition) for name, definition in _EXPECTED_INDEXES.items()}
        return actual == expected

    def _triggers_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relname AS relname,
                   t.tgname AS tgname,
                   t.tgisinternal AS tgisinternal,
                   pg_get_triggerdef(t.oid) AS definition
            FROM pg_trigger t
            JOIN pg_class c ON c.oid = t.tgrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue'
            """
        )
        rows = _rows(cursor)
        if any(row["tgisinternal"] for row in rows):
            return False
        actual = {(row["relname"], row["tgname"]): _normalize(row["definition"]) for row in rows}
        expected = {key: _normalize(definition) for key, definition in _EXPECTED_TRIGGERS.items()}
        return actual == expected

    def _policies_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relname AS relname,
                   c.relrowsecurity AS relrowsecurity,
                   c.relforcerowsecurity AS relforcerowsecurity
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue' AND c.relkind = 'r'
            """
        )
        rls = {
            row["relname"]: (bool(row["relrowsecurity"]), bool(row["relforcerowsecurity"]))
            for row in _rows(cursor)
        }
        if rls != _EXPECTED_RLS:
            return False
        cursor.execute(
            """
            SELECT c.relname AS relname,
                   pol.polname AS polname,
                   pol.polcmd AS polcmd,
                   pg_get_expr(pol.polqual, pol.polrelid) AS using_expr,
                   pg_get_expr(pol.polwithcheck, pol.polrelid) AS check_expr
            FROM pg_policy pol
            JOIN pg_class c ON c.oid = pol.polrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue'
            """
        )
        rows = _rows(cursor)
        if any(row["relname"] != "jobs" for row in rows):
            return False
        actual = {
            row["polname"]: (
                _normalize(row["polcmd"]),
                _normalize(row["using_expr"]),
                _normalize(row["check_expr"]),
            )
            for row in rows
        }
        expected = {
            name: (_normalize(command), _normalize(using), _normalize(check))
            for name, (command, using, check) in _EXPECTED_POLICIES.items()
        }
        return actual == expected

    def _routines_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT p.proname AS proname,
                   p.prokind AS prokind,
                   l.lanname AS lanname,
                   pg_get_function_identity_arguments(p.oid) AS arguments,
                   pg_get_function_result(p.oid) AS result,
                   p.provolatile AS provolatile,
                   p.prosecdef AS prosecdef,
                   p.proconfig AS proconfig,
                   p.prosrc AS prosrc
            FROM pg_proc p
            JOIN pg_namespace n ON n.oid = p.pronamespace
            JOIN pg_language l ON l.oid = p.prolang
            WHERE n.nspname = 'autopilot_queue'
            """
        )
        actual = {}
        for row in _rows(cursor):
            config = row["proconfig"] or []
            actual[row["proname"]] = {
                "kind": _normalize(row["prokind"]),
                "language": _normalize(row["lanname"]),
                "arguments": _normalize(row["arguments"]),
                "result": _normalize(row["result"]),
                "volatile": _normalize(row["provolatile"]),
                "definer": bool(row["prosecdef"]),
                "config": _normalize(" ".join(str(item) for item in config)),
                "source": _normalize(row["prosrc"]),
            }
        expected = {
            name: {**routine, "source": _normalize(routine["source"]), "config": _normalize(routine["config"])}
            for name, routine in _EXPECTED_ROUTINES.items()
        }
        return actual == expected

    def _types_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT rowtype.typname AS row_name, arraytype.typname AS array_name
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            JOIN pg_type rowtype ON rowtype.oid = c.reltype
            LEFT JOIN pg_type arraytype ON arraytype.oid = rowtype.typarray
            WHERE n.nspname = 'autopilot_queue' AND c.relkind = 'r'
            """
        )
        allowed = set()
        for row in _rows(cursor):
            if row["row_name"]:
                allowed.add(row["row_name"])
            if row["array_name"]:
                allowed.add(row["array_name"])
        if len(allowed) != len(_EXPECTED_TABLES) * 2:
            return False
        cursor.execute(
            """
            SELECT t.typname AS typname
            FROM pg_type t
            JOIN pg_namespace n ON n.oid = t.typnamespace
            WHERE n.nspname = 'autopilot_queue'
            """
        )
        return {row["typname"] for row in _rows(cursor)} == allowed

    def _sequences_match(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT c.relname AS relname
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue' AND c.relkind = 'S'
            """
        )
        if _rows(cursor):
            return False
        cursor.execute(
            """
            SELECT seq.relname AS relname
            FROM pg_depend dep
            JOIN pg_class seq ON seq.oid = dep.objid AND dep.classid = 'pg_class'::regclass
            JOIN pg_class referenced ON referenced.oid = dep.refobjid
                AND dep.refclassid = 'pg_class'::regclass
            JOIN pg_namespace referenced_ns ON referenced_ns.oid = referenced.relnamespace
            WHERE seq.relkind = 'S'
              AND referenced_ns.nspname = 'autopilot_queue'
              AND dep.deptype IN ('a', 'i', 'n')
            """
        )
        return not _rows(cursor)

    def _external_dependencies(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT count(*) AS n
            FROM pg_rewrite r
            JOIN pg_class c ON c.oid = r.ev_class
            JOIN pg_namespace n ON n.oid = c.relnamespace
            JOIN pg_depend d ON d.objid = r.oid AND d.classid = 'pg_rewrite'::regclass
            WHERE n.nspname <> 'autopilot_queue'
              AND n.nspname <> 'information_schema'
              AND n.nspname NOT LIKE 'pg!_%' ESCAPE '!'
              AND d.refclassid = 'pg_class'::regclass
              AND d.refobjid IN (
                  SELECT referenced.oid
                  FROM pg_class referenced
                  JOIN pg_namespace referenced_ns ON referenced_ns.oid = referenced.relnamespace
                  WHERE referenced_ns.nspname = 'autopilot_queue'
              )
            """
        )
        views = _one(cursor)
        cursor.execute(
            """
            SELECT count(*) AS n
            FROM pg_constraint fk
            JOIN pg_class referenced ON referenced.oid = fk.confrelid
            JOIN pg_namespace referenced_ns ON referenced_ns.oid = referenced.relnamespace
            JOIN pg_namespace fk_ns ON fk_ns.oid = fk.connamespace
            WHERE referenced_ns.nspname = 'autopilot_queue'
              AND fk.contype = 'f'
              AND fk_ns.nspname <> 'autopilot_queue'
            """
        )
        foreign_keys = _one(cursor)
        cursor.execute(
            """
            SELECT count(*) AS n
            FROM pg_depend d
            JOIN pg_proc p ON p.oid = d.objid AND d.classid = 'pg_proc'::regclass
            JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname <> 'autopilot_queue'
              AND n.nspname NOT IN ('pg_catalog', 'information_schema')
              AND n.nspname NOT LIKE 'pg!_%' ESCAPE '!'
              AND (
                  (
                      d.refclassid = 'pg_class'::regclass
                      AND d.refobjid IN (
                          SELECT c.oid
                          FROM pg_class c
                          JOIN pg_namespace cn ON cn.oid = c.relnamespace
                          WHERE cn.nspname = 'autopilot_queue'
                      )
                  )
                  OR (
                      d.refclassid = 'pg_proc'::regclass
                      AND d.refobjid IN (
                          SELECT routine.oid
                          FROM pg_proc routine
                          JOIN pg_namespace rn ON rn.oid = routine.pronamespace
                          WHERE rn.nspname = 'autopilot_queue'
                      )
                  )
                  OR (
                      d.refclassid = 'pg_type'::regclass
                      AND d.refobjid IN (
                          SELECT t.oid
                          FROM pg_type t
                          JOIN pg_namespace tn ON tn.oid = t.typnamespace
                          WHERE tn.nspname = 'autopilot_queue'
                      )
                  )
              )
            """
        )
        routines = _one(cursor)
        cursor.execute(
            """
            SELECT count(*) AS n
            FROM pg_trigger t
            JOIN pg_proc p ON p.oid = t.tgfoid
            JOIN pg_namespace pn ON pn.oid = p.pronamespace
            JOIN pg_class c ON c.oid = t.tgrelid
            JOIN pg_namespace cn ON cn.oid = c.relnamespace
            WHERE pn.nspname = 'autopilot_queue'
              AND cn.nspname <> 'autopilot_queue'
              AND NOT t.tgisinternal
            """
        )
        triggers = _one(cursor)
        cursor.execute(
            """
            SELECT count(*) AS n
            FROM pg_proc p
            JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname <> 'autopilot_queue'
              AND n.nspname NOT IN ('pg_catalog', 'information_schema')
              AND n.nspname NOT LIKE 'pg!_%' ESCAPE '!'
            """
        )
        external_routines = _one(cursor)
        cursor.execute(
            """
            SELECT count(*) AS n
            FROM pg_attribute a
            JOIN pg_class c ON c.oid = a.attrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            JOIN pg_type t ON t.oid = a.atttypid
            JOIN pg_namespace tn ON tn.oid = t.typnamespace
            WHERE tn.nspname = 'autopilot_queue'
              AND n.nspname <> 'autopilot_queue'
              AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
              AND n.nspname NOT LIKE 'pg!_%' ESCAPE '!'
              AND a.attnum > 0
              AND NOT a.attisdropped
            """
        )
        typed = _one(cursor)
        return any(
            item and int(item["n"])
            for item in (views, foreign_keys, routines, triggers, external_routines, typed)
        )

    def _assert_isolated_test_database(self, cursor) -> None:
        cursor.execute("SELECT current_database() AS name")
        current = _one(cursor)
        name = "" if current is None else str(current["name"])
        if not _TEST_DATABASE.fullmatch(name) or _UNSAFE_DATABASE.search(name):
            raise QueueError("migration_blocked")
        cursor.execute("SELECT to_regclass('public.schema_migrations') AS name")
        platform = _one(cursor)
        if platform and platform["name"]:
            raise QueueError("migration_blocked")
        cursor.execute(
            """
            SELECT n.nspname AS nspname
            FROM pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast', 'public', 'autopilot_queue')
              AND n.nspname NOT LIKE 'pg!_%' ESCAPE '!'
            """
        )
        if _rows(cursor):
            raise QueueError("migration_blocked")
        cursor.execute(
            """
            SELECT c.relname AS relname
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'public' AND c.relkind IN ('r', 'v', 'm', 'f')
            """
        )
        if _rows(cursor):
            raise QueueError("migration_blocked")

    def _job_count(self, cursor) -> int:
        cursor.execute("ALTER TABLE autopilot_queue.jobs NO FORCE ROW LEVEL SECURITY")
        cursor.execute("ALTER TABLE autopilot_queue.jobs DISABLE ROW LEVEL SECURITY")
        try:
            cursor.execute("SELECT count(*) AS n FROM autopilot_queue.jobs")
            counted = _one(cursor)
        finally:
            cursor.execute("ALTER TABLE autopilot_queue.jobs ENABLE ROW LEVEL SECURITY")
            cursor.execute("ALTER TABLE autopilot_queue.jobs FORCE ROW LEVEL SECURITY")
        return int(counted["n"]) if counted else 0

    def _role_attributes(self, cursor, role_name: str) -> dict | None:
        cursor.execute(
            """
            SELECT rolcanlogin, rolsuper, rolbypassrls, rolcreaterole, rolcreatedb,
                   rolreplication, rolinherit
            FROM pg_roles
            WHERE rolname = %s
            """,
            (role_name,),
        )
        return _one(cursor)

    def _memberships(self, cursor, role_name: str) -> list[str]:
        cursor.execute(
            """
            SELECT granted.rolname AS granted_name
            FROM pg_auth_members m
            JOIN pg_roles granted ON granted.oid = m.roleid
            JOIN pg_roles member ON member.oid = m.member
            WHERE member.rolname = %s
            """,
            (role_name,),
        )
        return [row["granted_name"] for row in _rows(cursor)]

    def _table_owner(self, cursor) -> str:
        cursor.execute(
            """
            SELECT pg_get_userbyid(c.relowner) AS owner_name
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'autopilot_queue' AND c.relname = 'jobs'
            """
        )
        row = _one(cursor)
        if row is None or not row["owner_name"]:
            raise QueueError("migration_failed")
        return str(row["owner_name"])

    def _grant_runtime(self, cursor, role_name: str) -> None:
        from psycopg import sql

        cursor.execute(sql.SQL("GRANT autopilot_queue_runtime TO {}").format(sql.Identifier(role_name)))

    @contextmanager
    def _cursor(self) -> Iterator:
        try:
            connection = self._connect()
        except QueueError:
            raise
        except Exception:
            raise QueueError("store_unavailable") from None
        try:
            with _cursor_for(connection) as cursor:
                yield cursor
            connection.commit()
        except QueueError:
            connection.rollback()
            raise
        except Exception as exc:
            connection.rollback()
            code = _safe_code(exc)
            if code == "store_failed":
                raise QueueError("migration_failed") from None
            raise QueueError(code) from None
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
        _validate_payload(job_type, payload)
        digest = _digest(payload)
        body = json.dumps(payload, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
        expected = {
            "job_type": job_type,
            "payload_version": payload_version,
            "payload_digest": digest,
            "priority": priority,
            "max_attempts": max_attempts,
            "backoff_seconds": backoff_seconds,
        }
        job_id = str(uuid.uuid4())
        with self._cursor() as cursor:
            _assert_runtime_binding(cursor, tenant_id)
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
                    SELECT job_id, status, job_type, payload_version, payload_digest,
                           priority, max_attempts, backoff_seconds
                    FROM autopilot_queue.jobs
                    WHERE tenant_id = %s AND idempotency_key = %s
                    """,
                    (tenant_id, idempotency_key),
                )
                existing = _one(cursor)
                if existing is None:
                    raise QueueError("store_failed")
                if not _same_execution(existing, expected):
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
            _assert_runtime_binding(cursor, tenant_id)
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
            _assert_runtime_binding(cursor, tenant_id)
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
            _assert_runtime_binding(cursor, tenant_id)
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
            _assert_runtime_binding(cursor, tenant_id)
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
            rows = _rows(cursor)
            for row in rows:
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
            _assert_runtime_binding(cursor, tenant_id)
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
            _assert_runtime_binding(cursor, tenant_id)
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
        except Exception:
            raise QueueError("store_unavailable") from None
        try:
            with _cursor_for(connection) as cursor:
                yield cursor
            connection.commit()
        except QueueError:
            connection.rollback()
            raise
        except Exception as exc:
            connection.rollback()
            raise QueueError(_safe_code(exc)) from None
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


def _assert_runtime_binding(cursor, tenant_id: str) -> None:
    cursor.execute(
        """
        SELECT session_role.rolsuper AS session_super,
               session_role.rolbypassrls AS session_bypass,
               session_role.rolcreaterole AS session_createrole,
               session_role.rolcreatedb AS session_createdb,
               session_role.rolreplication AS session_replication,
               active_role.rolsuper AS current_super,
               active_role.rolbypassrls AS current_bypass,
               active_role.rolcreaterole AS current_createrole,
               active_role.rolcreatedb AS current_createdb,
               active_role.rolreplication AS current_replication,
               session_user AS session_user,
               current_user AS current_user,
               current_setting('row_security') AS row_security,
               pg_get_userbyid(jobs.relowner) AS owner_name,
               jobs.relrowsecurity AS row_level_enabled,
               jobs.relforcerowsecurity AS row_level_forced
        FROM pg_roles AS session_role
        JOIN pg_roles AS active_role ON active_role.rolname = current_user
        JOIN pg_class AS jobs ON jobs.relname = 'jobs'
        JOIN pg_namespace AS namespace
          ON namespace.oid = jobs.relnamespace AND namespace.nspname = 'autopilot_queue'
        WHERE session_role.rolname = session_user
        """
    )
    row = _one(cursor)
    privileged = (
        "session_super",
        "session_bypass",
        "session_createrole",
        "session_createdb",
        "session_replication",
        "current_super",
        "current_bypass",
        "current_createrole",
        "current_createdb",
        "current_replication",
    )
    if (
        row is None
        or row["row_security"] != "on"
        or not row["row_level_enabled"]
        or not row["row_level_forced"]
        or any(row[name] for name in privileged)
        or row["session_user"] in {row["owner_name"], RUNTIME_ROLE}
        or row["current_user"] == row["owner_name"]
    ):
        raise QueueError("runtime_privileged")
    cursor.execute("SELECT autopilot_queue.bound_tenant() AS tenant_id")
    bound = _one(cursor)
    if bound is None or not bound["tenant_id"]:
        raise QueueError("tenant_unbound")
    if bound["tenant_id"] != tenant_id:
        raise QueueError("tenant_mismatch")


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

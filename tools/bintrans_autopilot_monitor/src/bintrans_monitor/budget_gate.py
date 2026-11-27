"""Local SQLite budget gate for a future isolated Actions proof.

This module does not call GitHub, does not read credentials, and does not
import the product autopilot package. A missing or damaged ledger denies.
"""

from __future__ import annotations

import os
import sqlite3
import stat
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterator, Mapping, Sequence

TOTAL_LIMIT = 60
MAX_SINGLE_JOB_MINUTES = 10
MAX_CONCURRENT_RUNS = 1
PAID_AI_BUDGET = 0
OVERHEAD_MINUTES = 1
SCHEMA_VERSION = 2

ACTIVE_STATUSES = (
    "RESERVED",
    "DISPATCH_INTENT_RECORDED",
    "DISPATCH_CONFIRMED",
    "DISPATCH_UNKNOWN",
    "RUNNING",
    "TERMINAL_PENDING_RECONCILIATION",
)
TERMINAL_STATUSES = ("RECONCILED", "REJECTED", "CANCELLED_BEFORE_DISPATCH")
STATUSES = ACTIVE_STATUSES + TERMINAL_STATUSES

_TRANSITIONS = {
    "RESERVED": {"DISPATCH_INTENT_RECORDED", "CANCELLED_BEFORE_DISPATCH"},
    "DISPATCH_INTENT_RECORDED": {
        "DISPATCH_CONFIRMED",
        "DISPATCH_UNKNOWN",
        "REJECTED",
        "CANCELLED_BEFORE_DISPATCH",
    },
    "DISPATCH_CONFIRMED": {"RUNNING", "TERMINAL_PENDING_RECONCILIATION", "RECONCILED"},
    "RUNNING": {"TERMINAL_PENDING_RECONCILIATION", "RECONCILED"},
    "DISPATCH_UNKNOWN": {"TERMINAL_PENDING_RECONCILIATION", "RECONCILED"},
    "TERMINAL_PENDING_RECONCILIATION": {"RECONCILED"},
    "RECONCILED": set(),
    "REJECTED": set(),
    "CANCELLED_BEFORE_DISPATCH": set(),
}


class BudgetDeny(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


class DispatchNotSent(BudgetDeny):
    """The adapter rejected the call before any dispatch POST was attempted."""


@dataclass(frozen=True)
class DispatchRequest:
    dispatch_id: str
    idempotency_key: str
    repository: str
    workflow: str
    expected_sha: str
    job_count: int
    per_job_minutes: int
    paid_ai_calls: int = 0
    run_attempt: int = 1


@dataclass(frozen=True)
class DispatchOutcome:
    dispatch_id: str
    status: str
    reserved_minutes: int
    run_id: str | None = None
    run_attempt: int | None = None


@dataclass(frozen=True)
class AdapterResult:
    kind: str
    run_id: str | None = None
    run_attempt: int | None = None
    observed_sha: str | None = None


class DispatchAdapter:
    def dispatch(self, request: DispatchRequest) -> AdapterResult:
        raise BudgetDeny("adapter_not_configured")


class FakeDispatchAdapter(DispatchAdapter):
    """In-process stand-in. It does not open a socket or read a token."""

    def __init__(self, result: AdapterResult | Exception):
        self._result = result
        self.calls = 0

    def dispatch(self, request: DispatchRequest, binding: object | None = None) -> AdapterResult:
        del binding
        self.calls += 1
        if isinstance(self._result, Exception):
            raise self._result
        return self._result


class ResponseLost(Exception):
    pass


class AdapterCrashed(Exception):
    pass


def compute_reserve(job_count: int, per_job_minutes: int) -> int:
    if isinstance(job_count, bool) or isinstance(per_job_minutes, bool):
        raise BudgetDeny("reserve_invalid")
    if not isinstance(job_count, int) or not isinstance(per_job_minutes, int):
        raise BudgetDeny("reserve_invalid")
    if job_count < 1 or per_job_minutes < 1 or per_job_minutes > MAX_SINGLE_JOB_MINUTES:
        raise BudgetDeny("reserve_invalid")
    reserved = job_count * per_job_minutes + OVERHEAD_MINUTES
    if reserved > TOTAL_LIMIT:
        raise BudgetDeny("reserve_invalid")
    return reserved


def initialize_campaign(
    path: Path,
    *,
    campaign_id: str,
    repository: str,
    workflow: str,
    expected_sha: str,
) -> None:
    _identity(campaign_id, "campaign_id")
    _identity(repository, "repository")
    _identity(workflow, "workflow")
    _sha(expected_sha)
    if path.exists():
        raise BudgetDeny("ledger_exists")
    path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(path, timeout=5.0, isolation_level=None)
    try:
        conn.row_factory = sqlite3.Row
        _prepare(conn)
        conn.executescript(_SCHEMA)
        with _tx(conn):
            conn.execute(
                """
                INSERT INTO campaign (
                    campaign_id, repository, workflow, expected_sha,
                    total_limit, max_single_job, max_concurrent, paid_ai_budget,
                    stop_state, blocked, block_reason, recovery_state, created_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'unknown', 0, '', 'ACTIVE', ?)
                """,
                (
                    campaign_id,
                    repository,
                    workflow,
                    expected_sha,
                    TOTAL_LIMIT,
                    MAX_SINGLE_JOB_MINUTES,
                    MAX_CONCURRENT_RUNS,
                    PAID_AI_BUDGET,
                    _now(),
                ),
            )
            _audit(conn, campaign_id, None, "campaign_initialized", campaign_id)
    except Exception:
        conn.close()
        path.unlink(missing_ok=True)
        raise
    else:
        conn.close()
        _restrict_ledger_permissions(path)


class BudgetGate:
    def __init__(self, path: Path, conn: sqlite3.Connection):
        self.path = path
        self._conn = conn

    @classmethod
    def open(cls, path: Path) -> "BudgetGate":
        if not path.exists():
            raise BudgetDeny("missing_ledger")
        try:
            conn = sqlite3.connect(path, timeout=5.0, isolation_level=None)
        except sqlite3.Error as exc:
            raise BudgetDeny("corrupted_ledger") from exc
        conn.row_factory = sqlite3.Row
        try:
            _prepare(conn)
            _require_healthy(conn)
            gate = cls(path, conn)
            gate._recover()
            return gate
        except BudgetDeny:
            conn.close()
            raise
        except sqlite3.Error as exc:
            conn.close()
            raise BudgetDeny("corrupted_ledger") from exc

    def close(self) -> None:
        self._conn.close()

    def set_emergency_stop(self, state: str) -> None:
        if state not in ("off", "on", "unknown"):
            raise BudgetDeny("stop_unknown")
        with _tx(self._conn):
            self._conn.execute("UPDATE campaign SET stop_state = ?", (state,))
            campaign = self._campaign()
            _audit(self._conn, campaign["campaign_id"], None, "stop_set", state)

    def available_minutes(self) -> int:
        with _tx(self._conn):
            campaign = self._campaign()
            return _available(self._conn, campaign["campaign_id"], int(campaign["total_limit"]))

    def reserve(self, request: DispatchRequest) -> DispatchOutcome:
        reserved = compute_reserve(request.job_count, request.per_job_minutes)
        _identity(request.dispatch_id, "dispatch_id")
        _identity(request.idempotency_key, "idempotency_key")
        _sha(request.expected_sha)
        if request.paid_ai_calls != 0 or isinstance(request.paid_ai_calls, bool):
            raise BudgetDeny("paid_ai_denied")
        if request.run_attempt != 1:
            raise BudgetDeny("automatic_retry_denied")
        with _tx(self._conn):
            campaign = self._campaign()
            self._admit(campaign, request, reserved)
            now = _now()
            self._conn.execute(
                """
                INSERT INTO dispatches (
                    dispatch_id, campaign_id, idempotency_key, repository, workflow,
                    expected_sha, reserved_minutes, confirmed_consumed_minutes,
                    run_id, run_attempt, status, reconciliation_status,
                    usage_fingerprint, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, 'RESERVED', 'NOT_STARTED', '', ?, ?)
                """,
                (
                    request.dispatch_id,
                    campaign["campaign_id"],
                    request.idempotency_key,
                    request.repository,
                    request.workflow,
                    request.expected_sha,
                    reserved,
                    now,
                    now,
                ),
            )
            _audit(self._conn, campaign["campaign_id"], request.dispatch_id, "reserved", str(reserved))
        return DispatchOutcome(request.dispatch_id, "RESERVED", reserved)

    def dispatch(
        self,
        request: DispatchRequest,
        adapter: DispatchAdapter,
        before_adapter=None,
        binding: object | None = None,
    ) -> DispatchOutcome:
        row = self._require(request.dispatch_id)
        if row["status"] != "RESERVED":
            raise BudgetDeny("invalid_transition")
        if self._campaign_read()["recovery_state"] != "ACTIVE":
            raise BudgetDeny("recovery_required")
        self._mark_intent_or_cancel(request)
        if before_adapter is not None:
            before_adapter()
        self._cancel_if_stop_before_call(request)
        row = self._require(request.dispatch_id)
        if row["status"] == "CANCELLED_BEFORE_DISPATCH":
            code = "stop_unknown" if self._read_stop() == "unknown" else "emergency_stop"
            raise BudgetDeny(code)
        try:
            result = adapter.dispatch(request, binding)
        except DispatchNotSent as exc:
            self._cancel_unsent(request.dispatch_id)
            raise
        except ResponseLost as exc:
            self._mark_unknown(request.dispatch_id, "response_lost")
            raise BudgetDeny("dispatch_unknown") from exc
        except AdapterCrashed as exc:
            self._mark_unknown(request.dispatch_id, "crash_after_dispatch")
            raise BudgetDeny("dispatch_unknown") from exc
        except BudgetDeny:
            self._mark_unknown(request.dispatch_id, "adapter_denied")
            raise
        except Exception as exc:
            self._mark_unknown(request.dispatch_id, "malformed_response")
            raise BudgetDeny("dispatch_unknown") from exc
        if self._read_stop() != "off":
            self._hold_after_possible_send(request.dispatch_id, "stop_after_possible_dispatch")
            raise BudgetDeny("dispatch_unknown")
        return self._apply_adapter_result(request, result)

    def note_usage_delayed(self, dispatch_id: str) -> DispatchOutcome:
        with _tx(self._conn):
            row = self._lock_dispatch(dispatch_id)
            if row["status"] not in ("RUNNING", "DISPATCH_UNKNOWN", "DISPATCH_CONFIRMED"):
                raise BudgetDeny("invalid_transition")
            self._move(row, "TERMINAL_PENDING_RECONCILIATION", "USAGE_UNKNOWN")
            _audit(self._conn, row["campaign_id"], dispatch_id, "usage_delayed", row["status"])
            held = int(row["reserved_minutes"])
        return DispatchOutcome(dispatch_id, "TERMINAL_PENDING_RECONCILIATION", held)

    def reconcile(
        self,
        dispatch_id: str,
        jobs: Sequence[Mapping[str, object]],
        *,
        run_id: str,
        run_attempt: int,
    ) -> DispatchOutcome:
        _identity(run_id, "run_id")
        if not isinstance(run_attempt, int) or isinstance(run_attempt, bool) or run_attempt < 1:
            raise BudgetDeny("run_attempt_rejected")
        parsed = _parse_jobs(jobs)
        fingerprint = _fingerprint(parsed)
        consumed = sum(minutes for _, minutes, _ in parsed)
        replay: DispatchOutcome | None = None
        denied = ""
        reserved = 0
        with _tx(self._conn):
            row = self._lock_dispatch(dispatch_id)
            reserved = int(row["reserved_minutes"])
            if row["status"] == "RECONCILED":
                if row["usage_fingerprint"] == fingerprint and int(row["confirmed_consumed_minutes"]) == consumed:
                    _audit(self._conn, row["campaign_id"], dispatch_id, "reconcile_replay", str(consumed))
                    replay = DispatchOutcome(dispatch_id, "RECONCILED", reserved, run_id, run_attempt)
                else:
                    self._block(row["campaign_id"], "reconcile_conflict")
                    denied = "reconcile_conflict"
            elif row["status"] not in ("RUNNING", "DISPATCH_UNKNOWN", "TERMINAL_PENDING_RECONCILIATION", "DISPATCH_CONFIRMED"):
                denied = "invalid_transition"
            elif row["run_id"] not in (None, run_id):
                self._block(row["campaign_id"], "run_id_mismatch")
                denied = "run_id_mismatch"
            else:
                unauthorized_attempt = run_attempt != 1
                for job_id, minutes, conclusion in parsed:
                    self._conn.execute(
                        "INSERT INTO dispatch_jobs (dispatch_id, job_id, minutes, conclusion) VALUES (?, ?, ?, ?)",
                        (dispatch_id, job_id, minutes, conclusion),
                    )
                self._conn.execute(
                    """
                    UPDATE dispatches
                    SET confirmed_consumed_minutes = ?, run_id = ?, run_attempt = ?,
                        usage_fingerprint = ?, updated_at = ?
                    WHERE dispatch_id = ?
                    """,
                    (consumed, run_id, run_attempt, fingerprint, _now(), dispatch_id),
                )
                fresh = self._lock_dispatch(dispatch_id)
                self._move(fresh, "RECONCILED", "OVERRUN" if consumed > reserved else "RECONCILED")
                if unauthorized_attempt:
                    self._block(row["campaign_id"], "rerun_without_reservation")
                    _audit(self._conn, row["campaign_id"], dispatch_id, "unauthorized_attempt_charged", str(consumed))
                    denied = "rerun_without_reservation"
                elif consumed > reserved:
                    self._block(row["campaign_id"], "budget_overrun")
                    _audit(self._conn, row["campaign_id"], dispatch_id, "budget_overrun", str(consumed))
                else:
                    _audit(self._conn, row["campaign_id"], dispatch_id, "reconciled", str(consumed))
        if replay is not None:
            return replay
        if denied:
            raise BudgetDeny(denied)
        return DispatchOutcome(dispatch_id, "RECONCILED", reserved, run_id, run_attempt)

    def observe_external_run(
        self,
        *,
        run_id: str,
        run_attempt: int,
        repository: str,
        workflow: str,
        expected_sha: str,
    ) -> None:
        _identity(run_id, "run_id")
        if not isinstance(run_attempt, int) or isinstance(run_attempt, bool) or run_attempt < 1:
            raise BudgetDeny("run_attempt_rejected")
        reason = ""
        with _tx(self._conn):
            campaign = self._campaign()
            identity_ok = (
                repository == campaign["repository"]
                and workflow == campaign["workflow"]
                and expected_sha == campaign["expected_sha"]
            )
            row = self._conn.execute(
                "SELECT * FROM dispatches WHERE run_id = ? AND run_attempt = ?",
                (run_id, run_attempt),
            ).fetchone()
            if row is None or not identity_ok or run_attempt != 1:
                reason = "rerun_without_reservation" if run_attempt != 1 else "untracked_run"
                if row is not None and row["expected_sha"] != expected_sha:
                    reason = "sha_mismatch"
                self._block(campaign["campaign_id"], reason)
                _audit(self._conn, campaign["campaign_id"], None, "security_incident", reason + ":" + run_id)
        if reason:
            raise BudgetDeny(reason)

    def snapshot(self, dispatch_id: str) -> DispatchOutcome:
        row = self._require(dispatch_id)
        return DispatchOutcome(
            dispatch_id,
            row["status"],
            int(row["reserved_minutes"]),
            row["run_id"],
            row["run_attempt"],
        )

    def blocked(self) -> bool:
        return bool(self._campaign_read()["blocked"])

    def replay_adapter_result(self, dispatch_id: str, result: AdapterResult) -> None:
        denied = ""
        with _tx(self._conn):
            row = self._lock_dispatch(dispatch_id)
            if row["status"] == "RUNNING" and row["run_id"] == result.run_id and result.run_attempt == 1:
                _audit(self._conn, row["campaign_id"], dispatch_id, "duplicate_response", result.run_id or "")
            else:
                self._block(row["campaign_id"], "unexpected_run")
                denied = "unexpected_run"
        if denied:
            raise BudgetDeny(denied)

    def confirmed_minutes(self, dispatch_id: str) -> int | None:
        row = self._require(dispatch_id)
        value = row["confirmed_consumed_minutes"]
        if value is None:
            return None
        return int(value)

    def _campaign_read(self) -> sqlite3.Row:
        row = self._conn.execute("SELECT * FROM campaign").fetchone()
        if row is None:
            raise BudgetDeny("corrupted_ledger")
        return row

    def _campaign(self) -> sqlite3.Row:
        return self._campaign_read()

    def _require(self, dispatch_id: str) -> sqlite3.Row:
        row = self._conn.execute("SELECT * FROM dispatches WHERE dispatch_id = ?", (dispatch_id,)).fetchone()
        if row is None:
            raise BudgetDeny("unknown_dispatch")
        return row

    def _lock_dispatch(self, dispatch_id: str) -> sqlite3.Row:
        return self._require(dispatch_id)

    def _admit(self, campaign: sqlite3.Row, request: DispatchRequest, reserved: int) -> None:
        if campaign["recovery_state"] != "ACTIVE":
            raise BudgetDeny("recovery_required")
        if campaign["stop_state"] == "unknown":
            raise BudgetDeny("stop_unknown")
        if campaign["stop_state"] != "off":
            raise BudgetDeny("emergency_stop")
        if int(campaign["blocked"]):
            raise BudgetDeny("campaign_blocked")
        if (
            request.repository != campaign["repository"]
            or request.workflow != campaign["workflow"]
            or request.expected_sha != campaign["expected_sha"]
        ):
            raise BudgetDeny("identity_rejected")
        existing_id = self._conn.execute(
            "SELECT dispatch_id FROM dispatches WHERE dispatch_id = ?",
            (request.dispatch_id,),
        ).fetchone()
        if existing_id is not None:
            raise BudgetDeny("duplicate_dispatch_id")
        existing_key = self._conn.execute(
            "SELECT idempotency_key FROM dispatches WHERE idempotency_key = ?",
            (request.idempotency_key,),
        ).fetchone()
        if existing_key is not None:
            raise BudgetDeny("duplicate_idempotency_key")
        unknown = self._conn.execute(
            """
            SELECT dispatch_id FROM dispatches
            WHERE status IN ('DISPATCH_UNKNOWN', 'TERMINAL_PENDING_RECONCILIATION')
               OR reconciliation_status = 'USAGE_UNKNOWN'
            """
        ).fetchone()
        if unknown is not None:
            raise BudgetDeny("unknown_usage")
        active = self._conn.execute(
            f"SELECT COUNT(*) AS n FROM dispatches WHERE status IN ({','.join('?' for _ in ACTIVE_STATUSES)})",
            ACTIVE_STATUSES,
        ).fetchone()
        if int(active["n"]) >= MAX_CONCURRENT_RUNS:
            raise BudgetDeny("concurrent_run")
        available = _available(self._conn, campaign["campaign_id"], int(campaign["total_limit"]))
        if available < reserved:
            raise BudgetDeny("insufficient_minutes")

    def _mark_intent_or_cancel(self, request: DispatchRequest) -> None:
        with _tx(self._conn):
            campaign = self._campaign()
            row = self._lock_dispatch(request.dispatch_id)
            if campaign["stop_state"] != "off":
                self._move(row, "CANCELLED_BEFORE_DISPATCH", "NOT_STARTED", confirmed=0)
                _audit(self._conn, campaign["campaign_id"], request.dispatch_id, "cancelled_before_dispatch", "stop")
                return
            self._move(row, "DISPATCH_INTENT_RECORDED", "NOT_STARTED")
            _audit(self._conn, campaign["campaign_id"], request.dispatch_id, "dispatch_intent", request.dispatch_id)
        with _tx(self._conn):
            campaign = self._campaign()
            row = self._lock_dispatch(request.dispatch_id)
            if row["status"] != "DISPATCH_INTENT_RECORDED":
                return
            if campaign["stop_state"] != "off":
                self._move(row, "CANCELLED_BEFORE_DISPATCH", "NOT_STARTED", confirmed=0)
                _audit(self._conn, campaign["campaign_id"], request.dispatch_id, "cancelled_before_dispatch", "stop_before_adapter")

    def _cancel_if_stop_before_call(self, request: DispatchRequest) -> None:
        with _tx(self._conn):
            campaign = self._campaign()
            row = self._lock_dispatch(request.dispatch_id)
            if row["status"] != "DISPATCH_INTENT_RECORDED":
                return
            if campaign["stop_state"] == "off":
                return
            self._move(row, "CANCELLED_BEFORE_DISPATCH", "NOT_STARTED", confirmed=0)
            _audit(
                self._conn,
                campaign["campaign_id"],
                request.dispatch_id,
                "stop_before_adapter",
                campaign["stop_state"],
            )

    def _read_stop(self) -> str:
        return str(self._campaign_read()["stop_state"])

    def _hold_after_possible_send(self, dispatch_id: str, reason: str) -> None:
        with _tx(self._conn):
            row = self._lock_dispatch(dispatch_id)
            if row["status"] == "DISPATCH_INTENT_RECORDED":
                self._move(row, "DISPATCH_UNKNOWN", "USAGE_UNKNOWN")
            self._block(row["campaign_id"], reason)
            _audit(self._conn, row["campaign_id"], dispatch_id, "stop_after_possible_dispatch", reason)

    def _cancel_unsent(self, dispatch_id: str) -> None:
        with _tx(self._conn):
            row = self._lock_dispatch(dispatch_id)
            if row["status"] != "DISPATCH_INTENT_RECORDED":
                self._block(row["campaign_id"], "dispatch_unknown")
                return
            self._move(row, "CANCELLED_BEFORE_DISPATCH", "NOT_STARTED", confirmed=0)
            _audit(self._conn, row["campaign_id"], dispatch_id, "not_sent", "adapter_rejected_before_post")

    def recovery_state(self) -> str:
        return str(self._campaign_read()["recovery_state"])

    def apply_trusted_usage(self, dispatch_id: str, report: Mapping[str, object]) -> DispatchOutcome:
        if "minutes" in report or "consumed_minutes" in report:
            raise BudgetDeny("usage_untrusted")
        if report.get("source") != "actions_billing" or report.get("billing_authoritative") is not True:
            raise BudgetDeny("usage_untrusted")
        if report.get("estimated_only") is True:
            self.note_usage_delayed(dispatch_id)
            raise BudgetDeny("usage_untrusted")
        if report.get("terminal") is not True or report.get("jobs_complete") is not True or report.get("pages_complete") is not True:
            self.note_usage_delayed(dispatch_id)
            raise BudgetDeny("usage_incomplete")
        jobs = report.get("jobs")
        if not isinstance(jobs, list):
            raise BudgetDeny("usage_untrusted")
        campaign = self._campaign_read()
        if (
            report.get("repository") != campaign["repository"]
            or report.get("workflow") != campaign["workflow"]
            or report.get("expected_sha") != campaign["expected_sha"]
        ):
            raise BudgetDeny("identity_rejected")
        run_id = report.get("run_id")
        run_attempt = report.get("run_attempt")
        if not isinstance(run_id, str) or isinstance(run_attempt, bool) or not isinstance(run_attempt, int):
            raise BudgetDeny("usage_untrusted")
        return self.reconcile(dispatch_id, jobs, run_id=run_id, run_attempt=run_attempt)

    def record_external_consumption(
        self,
        *,
        dispatch_id: str,
        idempotency_key: str,
        run_id: str,
        run_attempt: int,
        jobs: Sequence[Mapping[str, object]],
    ) -> None:
        if self.recovery_state() != "RECOVERY_REQUIRED":
            raise BudgetDeny("invalid_transition")
        _identity(dispatch_id, "dispatch_id")
        _identity(idempotency_key, "idempotency_key")
        _identity(run_id, "run_id")
        if not isinstance(run_attempt, int) or isinstance(run_attempt, bool) or run_attempt < 1:
            raise BudgetDeny("run_attempt_rejected")
        parsed = _parse_jobs(jobs)
        consumed = sum(minutes for _, minutes, _ in parsed)
        if consumed < 1 or consumed > TOTAL_LIMIT:
            raise BudgetDeny("reserve_invalid")
        fingerprint = _fingerprint(parsed)
        with _tx(self._conn):
            campaign = self._campaign()
            if campaign["recovery_state"] != "RECOVERY_REQUIRED":
                raise BudgetDeny("invalid_transition")
            now = _now()
            self._conn.execute(
                """
                INSERT INTO dispatches (
                    dispatch_id, campaign_id, idempotency_key, repository, workflow,
                    expected_sha, reserved_minutes, confirmed_consumed_minutes,
                    run_id, run_attempt, status, reconciliation_status,
                    usage_fingerprint, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'RECONCILED', 'RECONCILED', ?, ?, ?)
                """,
                (
                    dispatch_id,
                    campaign["campaign_id"],
                    idempotency_key,
                    campaign["repository"],
                    campaign["workflow"],
                    campaign["expected_sha"],
                    consumed,
                    consumed,
                    run_id,
                    run_attempt,
                    fingerprint,
                    now,
                    now,
                ),
            )
            for job_id, minutes, conclusion in parsed:
                self._conn.execute(
                    "INSERT INTO dispatch_jobs (dispatch_id, job_id, minutes, conclusion) VALUES (?, ?, ?, ?)",
                    (dispatch_id, job_id, minutes, conclusion),
                )
            _audit(self._conn, campaign["campaign_id"], dispatch_id, "external_consumption_recorded", str(consumed))

    def attest_external_reconciliation(self) -> None:
        with _tx(self._conn):
            campaign = self._campaign()
            if campaign["recovery_state"] != "RECOVERY_REQUIRED":
                raise BudgetDeny("invalid_transition")
            _audit(self._conn, campaign["campaign_id"], None, "external_reconciliation_attested", "operator")

    def release_recovery(self, *, controller_go: bool, external_state_known: bool) -> None:
        if controller_go is not True or external_state_known is not True:
            raise BudgetDeny("recovery_required")
        with _tx(self._conn):
            campaign = self._campaign()
            if campaign["recovery_state"] != "RECOVERY_REQUIRED":
                raise BudgetDeny("invalid_transition")
            attested = self._conn.execute(
                """
                SELECT event_id FROM audit_events
                WHERE campaign_id = ? AND event_type = 'external_reconciliation_attested'
                """,
                (campaign["campaign_id"],),
            ).fetchone()
            if attested is None:
                raise BudgetDeny("recovery_required")
            active = self._conn.execute(
                f"SELECT COUNT(*) AS n FROM dispatches WHERE status IN ({','.join('?' for _ in ACTIVE_STATUSES)})",
                ACTIVE_STATUSES,
            ).fetchone()
            if int(active["n"]) > 0:
                raise BudgetDeny("recovery_required")
            if int(campaign["blocked"]) and campaign["block_reason"] != "recovery_required":
                raise BudgetDeny("campaign_blocked")
            before = _confirmed_total(self._conn, campaign["campaign_id"])
            self._conn.execute(
                """
                UPDATE campaign
                SET recovery_state = 'ACTIVE', blocked = 0, block_reason = ''
                WHERE campaign_id = ? AND recovery_state = 'RECOVERY_REQUIRED'
                """,
                (campaign["campaign_id"],),
            )
            after = _confirmed_total(self._conn, campaign["campaign_id"])
            if after != before:
                raise BudgetDeny("corrupted_ledger")
            _audit(self._conn, campaign["campaign_id"], None, "recovery_released", "controller_go")

    def _apply_adapter_result(self, request: DispatchRequest, result: AdapterResult) -> DispatchOutcome:
        denied = ""
        reserved = 0
        with _tx(self._conn):
            row = self._lock_dispatch(request.dispatch_id)
            reserved = int(row["reserved_minutes"])
            if row["status"] != "DISPATCH_INTENT_RECORDED":
                raise BudgetDeny("invalid_transition")
            if result.kind == "rejected":
                self._move(row, "REJECTED", "NOT_STARTED", confirmed=0)
                _audit(self._conn, row["campaign_id"], request.dispatch_id, "dispatch_rejected", "rejected")
                accepted = False
                rejected = True
            else:
                rejected = False
                accepted = result.kind == "accepted" and result.run_id is not None and result.run_attempt == 1 and result.observed_sha in (None, request.expected_sha)
                if result.kind != "accepted":
                    self._move(row, "DISPATCH_UNKNOWN", "USAGE_UNKNOWN")
                    _audit(self._conn, row["campaign_id"], request.dispatch_id, "dispatch_unknown", result.kind)
                    denied = "dispatch_unknown"
                elif result.run_id is None or result.run_attempt != 1:
                    self._block(row["campaign_id"], "unexpected_run")
                    self._move(row, "DISPATCH_UNKNOWN", "USAGE_UNKNOWN")
                    denied = "unexpected_run"
                elif result.observed_sha not in (None, request.expected_sha):
                    self._block(row["campaign_id"], "sha_mismatch")
                    self._move(row, "DISPATCH_UNKNOWN", "USAGE_UNKNOWN")
                    denied = "sha_mismatch"
                else:
                    _identity(result.run_id, "run_id")
                    now = _now()
                    self._conn.execute(
                        """
                        UPDATE dispatches
                        SET run_id = ?, run_attempt = ?, updated_at = ?
                        WHERE dispatch_id = ?
                        """,
                        (result.run_id, 1, now, request.dispatch_id),
                    )
                    self._move(row, "DISPATCH_CONFIRMED", "NOT_STARTED")
                    fresh = self._lock_dispatch(request.dispatch_id)
                    self._move(fresh, "RUNNING", "NOT_STARTED")
                    _audit(self._conn, row["campaign_id"], request.dispatch_id, "dispatch_confirmed", result.run_id or "")
        if rejected:
            return DispatchOutcome(request.dispatch_id, "REJECTED", reserved)
        if denied or not accepted:
            raise BudgetDeny(denied or "dispatch_unknown")
        return DispatchOutcome(request.dispatch_id, "RUNNING", reserved, result.run_id, 1)

    def _mark_unknown(self, dispatch_id: str, reason: str) -> None:
        with _tx(self._conn):
            row = self._lock_dispatch(dispatch_id)
            if row["status"] == "DISPATCH_UNKNOWN":
                return
            if row["status"] not in ("DISPATCH_INTENT_RECORDED", "DISPATCH_CONFIRMED", "RUNNING"):
                return
            self._move(row, "DISPATCH_UNKNOWN", "USAGE_UNKNOWN")
            _audit(self._conn, row["campaign_id"], dispatch_id, "dispatch_unknown", reason)

    def _recover(self) -> None:
        with _tx(self._conn):
            campaign = self._campaign()
            if campaign["stop_state"] not in ("off", "on", "unknown"):
                raise BudgetDeny("stop_unknown")
            if campaign["stop_state"] != "off":
                _audit(self._conn, campaign["campaign_id"], None, "stop_rechecked", campaign["stop_state"])
            rows = self._conn.execute(
                "SELECT * FROM dispatches WHERE status = 'DISPATCH_INTENT_RECORDED'"
            ).fetchall()
            for row in rows:
                self._move(row, "DISPATCH_UNKNOWN", "USAGE_UNKNOWN")
                _audit(self._conn, campaign["campaign_id"], row["dispatch_id"], "crash_recovery", "intent_unknown")

    def _move(self, row: sqlite3.Row, status: str, reconciliation_status: str, confirmed: int | None = None) -> None:
        current = row["status"]
        if status != current and status not in _TRANSITIONS[current]:
            raise BudgetDeny("invalid_transition")
        if confirmed is None:
            self._conn.execute(
                """
                UPDATE dispatches
                SET status = ?, reconciliation_status = ?, updated_at = ?
                WHERE dispatch_id = ?
                """,
                (status, reconciliation_status, _now(), row["dispatch_id"]),
            )
        else:
            self._conn.execute(
                """
                UPDATE dispatches
                SET status = ?, reconciliation_status = ?, confirmed_consumed_minutes = ?, updated_at = ?
                WHERE dispatch_id = ?
                """,
                (status, reconciliation_status, confirmed, _now(), row["dispatch_id"]),
            )

    def _block(self, campaign_id: str, reason: str) -> None:
        self._conn.execute(
            "UPDATE campaign SET blocked = 1, block_reason = ? WHERE campaign_id = ?",
            (reason, campaign_id),
        )
        _audit(self._conn, campaign_id, None, "campaign_blocked", reason)


def _confirmed_total(conn: sqlite3.Connection, campaign_id: str) -> int:
    row = conn.execute(
        """
        SELECT COALESCE(SUM(confirmed_consumed_minutes), 0) AS n
        FROM dispatches
        WHERE campaign_id = ? AND status = 'RECONCILED'
        """,
        (campaign_id,),
    ).fetchone()
    return int(row["n"])


def _available(conn: sqlite3.Connection, campaign_id: str, total_limit: int) -> int:
    consumed = conn.execute(
        """
        SELECT COALESCE(SUM(confirmed_consumed_minutes), 0) AS n
        FROM dispatches
        WHERE campaign_id = ? AND status = 'RECONCILED'
        """,
        (campaign_id,),
    ).fetchone()
    reserved = conn.execute(
        f"""
        SELECT COALESCE(SUM(reserved_minutes), 0) AS n
        FROM dispatches
        WHERE campaign_id = ? AND status IN ({','.join('?' for _ in ACTIVE_STATUSES)})
        """,
        (campaign_id, *ACTIVE_STATUSES),
    ).fetchone()
    return int(total_limit) - int(consumed["n"]) - int(reserved["n"])


def _parse_jobs(jobs: Sequence[Mapping[str, object]]) -> list[tuple[str, int, str]]:
    if not isinstance(jobs, Sequence) or isinstance(jobs, (str, bytes)) or len(jobs) < 1:
        raise BudgetDeny("usage_incomplete")
    parsed: list[tuple[str, int, str]] = []
    seen: set[str] = set()
    for job in jobs:
        job_id = str(job.get("job_id", ""))
        minutes = job.get("minutes")
        conclusion = str(job.get("conclusion", ""))
        _identity(job_id, "job_id")
        if job_id in seen:
            raise BudgetDeny("duplicate_job")
        seen.add(job_id)
        if isinstance(minutes, bool) or not isinstance(minutes, int) or minutes < 1:
            raise BudgetDeny("reserve_invalid")
        if conclusion not in ("success", "failure", "cancelled", "timed_out"):
            raise BudgetDeny("usage_incomplete")
        parsed.append((job_id, minutes, conclusion))
    return parsed


def _fingerprint(jobs: Sequence[tuple[str, int, str]]) -> str:
    return "|".join(f"{job_id}:{minutes}:{conclusion}" for job_id, minutes, conclusion in sorted(jobs))


def _identity(value: str, code: str) -> None:
    if not isinstance(value, str) or len(value) < 1 or len(value) > 80:
        raise BudgetDeny(code if code.endswith("_rejected") else "identity_rejected")
    allowed = set("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._:/-")
    if any(char not in allowed for char in value):
        raise BudgetDeny("identity_rejected")


def _sha(value: str) -> None:
    if not isinstance(value, str) or len(value) != 40 or any(char not in "0123456789abcdef" for char in value):
        raise BudgetDeny("sha_rejected")


def _now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _prepare(conn: sqlite3.Connection) -> None:
    try:
        conn.execute("PRAGMA foreign_keys = ON")
        conn.execute("PRAGMA busy_timeout = 5000")
        conn.execute("PRAGMA journal_mode = WAL")
    except sqlite3.OperationalError as exc:
        text = str(exc).lower()
        if "locked" in text or "busy" in text:
            raise BudgetDeny("ledger_unavailable") from exc
        raise BudgetDeny("corrupted_ledger") from exc


def _require_healthy(conn: sqlite3.Connection) -> None:
    try:
        check = conn.execute("PRAGMA integrity_check").fetchone()
    except sqlite3.DatabaseError as exc:
        raise BudgetDeny("corrupted_ledger") from exc
    if check is None or check[0] != "ok":
        raise BudgetDeny("corrupted_ledger")
    version = conn.execute(
        "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'schema_meta'"
    ).fetchone()
    if version is None:
        raise BudgetDeny("corrupted_ledger")
    row = conn.execute("SELECT schema_version FROM schema_meta").fetchone()
    if row is None or int(row["schema_version"]) != SCHEMA_VERSION:
        raise BudgetDeny("corrupted_ledger")
    campaign = conn.execute(
        "SELECT campaign_id, stop_state, total_limit, recovery_state FROM campaign"
    ).fetchone()
    if campaign is None or int(campaign["total_limit"]) != TOTAL_LIMIT:
        raise BudgetDeny("corrupted_ledger")
    if campaign["recovery_state"] not in ("ACTIVE", "RECOVERY_REQUIRED"):
        raise BudgetDeny("corrupted_ledger")


def _audit(conn: sqlite3.Connection, campaign_id: str, dispatch_id: str | None, event_type: str, detail: str) -> None:
    conn.execute(
        """
        INSERT INTO audit_events (campaign_id, dispatch_id, event_type, detail, created_at)
        VALUES (?, ?, ?, ?, ?)
        """,
        (campaign_id, dispatch_id, event_type, detail, _now()),
    )


@contextmanager
def _tx(conn: sqlite3.Connection) -> Iterator[None]:
    try:
        conn.execute("BEGIN IMMEDIATE")
    except sqlite3.OperationalError as exc:
        raise BudgetDeny("ledger_unavailable") from exc
    try:
        yield
    except Exception:
        conn.execute("ROLLBACK")
        raise
    else:
        conn.execute("COMMIT")


_SCHEMA = """
CREATE TABLE schema_meta (
    schema_version INTEGER NOT NULL CHECK (schema_version = 2)
);
INSERT INTO schema_meta (schema_version) VALUES (2);
CREATE TABLE campaign (
    campaign_id TEXT PRIMARY KEY,
    repository TEXT NOT NULL,
    workflow TEXT NOT NULL,
    expected_sha TEXT NOT NULL,
    total_limit INTEGER NOT NULL CHECK (total_limit = 60),
    max_single_job INTEGER NOT NULL CHECK (max_single_job = 10),
    max_concurrent INTEGER NOT NULL CHECK (max_concurrent = 1),
    paid_ai_budget INTEGER NOT NULL CHECK (paid_ai_budget = 0),
    stop_state TEXT NOT NULL CHECK (stop_state IN ('off', 'on', 'unknown')),
    blocked INTEGER NOT NULL CHECK (blocked IN (0, 1)),
    block_reason TEXT NOT NULL,
    recovery_state TEXT NOT NULL CHECK (recovery_state IN ('ACTIVE', 'RECOVERY_REQUIRED')),
    created_at TEXT NOT NULL
);
CREATE TABLE dispatches (
    dispatch_id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    repository TEXT NOT NULL,
    workflow TEXT NOT NULL,
    expected_sha TEXT NOT NULL,
    reserved_minutes INTEGER NOT NULL CHECK (reserved_minutes > 0 AND reserved_minutes <= 60),
    confirmed_consumed_minutes INTEGER CHECK (
        confirmed_consumed_minutes IS NULL OR confirmed_consumed_minutes >= 0
    ),
    run_id TEXT,
    run_attempt INTEGER CHECK (run_attempt IS NULL OR run_attempt >= 1),
    status TEXT NOT NULL,
    reconciliation_status TEXT NOT NULL,
    usage_fingerprint TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (campaign_id) REFERENCES campaign(campaign_id)
);
CREATE UNIQUE INDEX dispatches_run ON dispatches(run_id, run_attempt) WHERE run_id IS NOT NULL;
CREATE TABLE dispatch_jobs (
    dispatch_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    minutes INTEGER NOT NULL CHECK (minutes > 0),
    conclusion TEXT NOT NULL,
    PRIMARY KEY (dispatch_id, job_id),
    FOREIGN KEY (dispatch_id) REFERENCES dispatches(dispatch_id)
);
CREATE TABLE audit_events (
    event_id INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id TEXT NOT NULL,
    dispatch_id TEXT,
    event_type TEXT NOT NULL,
    detail TEXT NOT NULL,
    created_at TEXT NOT NULL
);
"""


def _restrict_ledger_permissions(path: Path) -> None:
    try:
        os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)
        os.chmod(path.parent, stat.S_IRUSR | stat.S_IWUSR | stat.S_IXUSR)
    except OSError:
        return


def guarded_ledger_path(path: Path, root: Path) -> Path:
    if not root.exists() or not root.is_dir() or root.is_symlink():
        raise BudgetDeny("unsafe_ledger_path")
    if path.is_symlink():
        raise BudgetDeny("unsafe_ledger_path")
    root_real = root.resolve()
    parent = path.parent
    if parent.is_symlink():
        raise BudgetDeny("unsafe_ledger_path")
    try:
        parent_real = parent.resolve()
    except OSError as exc:
        raise BudgetDeny("unsafe_ledger_path") from exc
    try:
        parent_real.relative_to(root_real)
    except ValueError as exc:
        raise BudgetDeny("unsafe_ledger_path") from exc
    candidate = parent_real / path.name
    if candidate.is_symlink():
        raise BudgetDeny("unsafe_ledger_path")
    return candidate


def _verify_backup(path: Path) -> None:
    try:
        conn = sqlite3.connect(path, timeout=5.0, isolation_level=None)
    except sqlite3.Error as exc:
        raise BudgetDeny("corrupted_ledger") from exc
    try:
        conn.row_factory = sqlite3.Row
        try:
            check = conn.execute("PRAGMA integrity_check").fetchone()
            row = conn.execute("SELECT schema_version FROM schema_meta").fetchone()
        except sqlite3.DatabaseError as exc:
            raise BudgetDeny("corrupted_ledger") from exc
        if check is None or check[0] != "ok" or row is None or int(row["schema_version"]) != SCHEMA_VERSION:
            raise BudgetDeny("corrupted_ledger")
    finally:
        conn.close()


def backup_ledger(source: Path, destination: Path, root: Path) -> None:
    source_path = guarded_ledger_path(source, root)
    destination_path = guarded_ledger_path(destination, root)
    if source_path.resolve() == destination_path.resolve() or destination_path.exists():
        raise BudgetDeny("unsafe_ledger_path")
    if not source_path.is_file():
        raise BudgetDeny("missing_ledger")
    src = sqlite3.connect(source_path, timeout=5.0, isolation_level=None)
    try:
        src.row_factory = sqlite3.Row
        _prepare(src)
        try:
            src.execute("PRAGMA wal_checkpoint(FULL)")
        except sqlite3.OperationalError as exc:
            text = str(exc).lower()
            if "locked" not in text and "busy" not in text:
                raise BudgetDeny("corrupted_ledger") from exc
        dest = sqlite3.connect(destination_path, timeout=5.0, isolation_level=None)
        try:
            src.backup(dest)
        finally:
            dest.close()
    finally:
        src.close()
    try:
        _verify_backup(destination_path)
    except BudgetDeny:
        destination_path.unlink(missing_ok=True)
        raise
    _restrict_ledger_permissions(destination_path)


def restore_ledger(backup: Path, destination: Path, root: Path, *, expected_campaign_id: str) -> None:
    backup_path = guarded_ledger_path(backup, root)
    destination_path = guarded_ledger_path(destination, root)
    if backup_path.resolve() == destination_path.resolve():
        raise BudgetDeny("unsafe_ledger_path")
    _verify_backup(backup_path)
    src = sqlite3.connect(backup_path, timeout=5.0, isolation_level=None)
    try:
        src.row_factory = sqlite3.Row
        identity = src.execute("SELECT campaign_id FROM campaign").fetchone()
        if identity is None or identity["campaign_id"] != expected_campaign_id:
            raise BudgetDeny("campaign_identity_mismatch")
        created = not destination_path.exists()
        dest = sqlite3.connect(destination_path, timeout=5.0, isolation_level=None)
        try:
            dest.row_factory = sqlite3.Row
            src.backup(dest)
            dest.execute("PRAGMA foreign_keys = ON")
            with _tx(dest):
                dest.execute(
                    """
                    UPDATE campaign
                    SET recovery_state = 'RECOVERY_REQUIRED', blocked = 1, block_reason = 'recovery_required'
                    """
                )
                campaign = dest.execute("SELECT campaign_id FROM campaign").fetchone()
                _audit(dest, campaign["campaign_id"], None, "ledger_restored", "recovery_required")
        except Exception:
            dest.close()
            if created:
                destination_path.unlink(missing_ok=True)
            raise
        else:
            dest.close()
    finally:
        src.close()
    _verify_backup(destination_path)
    _restrict_ledger_permissions(destination_path)


def open_expected(path: Path, root: Path, campaign_id: str) -> BudgetGate:
    guarded = guarded_ledger_path(path, root)
    gate = BudgetGate.open(guarded)
    try:
        if gate._campaign_read()["campaign_id"] != campaign_id:
            raise BudgetDeny("campaign_identity_mismatch")
    except BudgetDeny:
        gate.close()
        raise
    return gate

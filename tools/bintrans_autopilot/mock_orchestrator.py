"""Mock-only orchestration over the existing queue, launch protocol, and budget ledger.

This module does not start a resident process, a Cursor Cloud Agent, or a
network call. The committed autopilot flag stays off. A caller must pass the
budget test harness explicitly; that does not authorize 24/7 or paid launch.
"""

from __future__ import annotations

import hashlib
import json
import logging
import os
from dataclasses import dataclass
from pathlib import Path

from bintrans_autopilot.budget_controller import (
    BLOCKED,
    COMMITTED,
    OVERRUN,
    RESERVED,
    BudgetController,
    BudgetControllerError,
    _exception_code,
)
from bintrans_autopilot.config import AutopilotConfig, load_config
from bintrans_autopilot.launch.mock_transport import MockCursorTransport, mock_identity
from bintrans_autopilot.postgres_queue import (
    PostgresQueue,
    QueueError,
    _ERROR,
    _assert_runtime_binding,
    _cursor_for,
    _one,
)
from bintrans_autopilot.usage_verification import usage_verification_refs

logger = logging.getLogger("bintrans_autopilot.mock_orchestrator")

_JOB_TYPE = "mock_launch"
_SCENARIOS = frozenset(
    {
        "create_success",
        "definite_reject",
        "timeout",
        "ambiguous",
        "conflict",
        "crash_before_response",
    }
)
_LOCKED_OFF = (
    "PAID_AUTOMATION_ENABLED",
    "EXTERNAL_TOOLS_ENABLED",
    "AUTO_MERGE_ENABLED",
    "AUTO_DEPLOY_ENABLED",
    "WORKER_ENABLED",
)
_MOCK_BODY_HASH = "a" * 64
_TERMINAL_SETTLED = {COMMITTED, OVERRUN}
_RETRYABLE = {
    "timeout",
    "ambiguous",
    "conflict",
    "handler_failed",
    "store_unavailable",
    "recovery_pending",
    "lease_held_by_other",
    "estop_active",
    "launch_mode_blocked",
}


class OrchestratorError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class MockTask:
    tenant_id: str
    idempotency_key: str
    launch_id: str
    agent_id: str
    run_id: str
    reserve_cents: int
    charged_cents: int
    scenario: str = "create_success"
    priority: int = 100


@dataclass(frozen=True)
class SubmitResult:
    job_id: str
    created: bool
    status: str


class AuditJournal:
    """Append-only orchestration events. Not a budget ledger and not a token store."""

    def __init__(self, path: Path | None = None):
        self.path = path
        self.events: list[dict] = []

    def write(self, event: str, **fields: str) -> None:
        record = {"event": event}
        for key, value in fields.items():
            if not isinstance(value, str):
                raise OrchestratorError("invalid_input")
            lowered = key.lower()
            if "token" in lowered or "secret" in lowered or "password" in lowered or "dsn" in lowered:
                raise OrchestratorError("invalid_input")
            record[key] = value
        self.events.append(record)
        logger.info("event=%s", event)
        if self.path is None:
            return
        self.path.parent.mkdir(parents=True, exist_ok=True)
        with self.path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n")


class MockOrchestrator:
    """One tenant, one lease owner, registered mock handler only."""

    def __init__(
        self,
        queue: PostgresQueue,
        runtime_connect,
        adapter_connect,
        budget: BudgetController,
        recovery_connect,
        *,
        tenant_id: str,
        owner: str,
        config: AutopilotConfig | None = None,
        audit: AuditJournal | None = None,
        lease_seconds: int = 30,
    ):
        if queue.autopilot_enabled or queue.paid_automation_enabled:
            raise OrchestratorError("automation_disabled")
        for name in _LOCKED_OFF:
            if os.environ.get(name, "").strip().lower() == "true":
                raise OrchestratorError("orchestrator_disabled")
        self._config = config or load_config()
        if self._config.autopilot_enabled and not self._config.test_budget_harness:
            raise OrchestratorError("orchestrator_disabled")
        if not callable(runtime_connect) or not callable(adapter_connect) or not callable(recovery_connect):
            raise OrchestratorError("invalid_input")
        self.queue = queue
        self._runtime_connect = runtime_connect
        self._adapter_connect = adapter_connect
        self._recovery_connect = recovery_connect
        self.budget = budget
        self.tenant_id = tenant_id
        self.owner = owner
        self.lease_seconds = lease_seconds
        self.audit = audit or AuditJournal()
        self.autopilot_enabled = False
        self.paid_automation_enabled = False
        self.external_calls = 0
        self.cloud_agents_started = 0
        self.mock_sends = 0
        self._stop = False

    def stop(self) -> None:
        self._stop = True
        self.audit.write("orchestrator_stopped", tenant_id=self.tenant_id, result="stopped")

    def submit(self, task: MockTask) -> SubmitResult:
        if not isinstance(task, MockTask):
            raise OrchestratorError("invalid_input")
        if task.tenant_id != self.tenant_id:
            raise OrchestratorError("tenant_mismatch")
        if task.scenario not in _SCENARIOS:
            raise OrchestratorError("handler_rejected")
        if isinstance(task.reserve_cents, bool) or not isinstance(task.reserve_cents, int) or task.reserve_cents <= 0:
            raise OrchestratorError("invalid_amount")
        if isinstance(task.charged_cents, bool) or not isinstance(task.charged_cents, int) or task.charged_cents < 0:
            raise OrchestratorError("invalid_amount")
        gate = self._control_gate()
        if gate != "allowed":
            self.audit.write("submit_denied", tenant_id=self.tenant_id, launch_id=task.launch_id, result=gate)
            raise OrchestratorError(gate)
        switch = self.budget.evaluate_kill_switch()
        if not switch.allowed:
            self.audit.write(
                "submit_denied", tenant_id=self.tenant_id, launch_id=task.launch_id, result=switch.reason
            )
            raise OrchestratorError(switch.reason)
        payload = {
            "launch_id": task.launch_id,
            "agent_id": task.agent_id,
            "run_id": task.run_id,
            "scenario": task.scenario,
            "reserve_cents": task.reserve_cents,
            "charged_cents": task.charged_cents,
        }
        try:
            queued = self.queue.enqueue(
                tenant_id=task.tenant_id,
                job_type=_JOB_TYPE,
                payload=payload,
                idempotency_key=task.idempotency_key,
                priority=task.priority,
                max_attempts=self._attempt_budget(),
                backoff_seconds=0,
            )
        except QueueError as exc:
            raise OrchestratorError(exc.code) from None
        result = "created" if queued.created else "duplicate"
        self.audit.write(
            "task_submitted",
            tenant_id=task.tenant_id,
            job_id=queued.job_id,
            launch_id=task.launch_id,
            result=result,
        )
        return SubmitResult(queued.job_id, queued.created, queued.status)

    def run_pending(self, *, limit: int = 1) -> int:
        if isinstance(limit, bool) or not isinstance(limit, int) or not 1 <= limit <= 100:
            raise OrchestratorError("invalid_input")
        completed = 0
        for _ in range(limit):
            outcome = self.run_once()
            if outcome in {"committed", "overrun", "rejected"}:
                completed += 1
                continue
            if outcome in {"idle", "stopped", "unavailable"}:
                break
        return completed

    def run_once(self) -> str:
        if self._stop:
            return "stopped"
        try:
            self.queue.requeue_expired(self.tenant_id)
            gate = self._control_gate()
        except QueueError as exc:
            self.audit.write("database_unavailable", tenant_id=self.tenant_id, result=exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        except OrchestratorError as exc:
            self.audit.write("database_unavailable", tenant_id=self.tenant_id, result=exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        if gate != "allowed":
            self.stop()
            self.audit.write("kill_switch", tenant_id=self.tenant_id, result=gate)
            return "stopped"
        try:
            lease = self.queue.claim(self.tenant_id, self.owner, self.lease_seconds)
        except QueueError as exc:
            self.audit.write("database_unavailable", tenant_id=self.tenant_id, result=exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        if lease is None:
            self.audit.write("queue_idle", tenant_id=self.tenant_id, result="idle")
            return "idle"
        self.audit.write("job_claimed", tenant_id=self.tenant_id, job_id=lease.job_id, result="running")
        try:
            payload = self.queue.leased_payload(
                lease.tenant_id, lease.job_id, lease.lease_owner, lease.lease_token
            )
            outcome = self._execute(lease, payload)
        except QueueError as exc:
            self.audit.write("job_failed", tenant_id=self.tenant_id, job_id=lease.job_id, result=exc.code)
            if exc.code in {"lease_expired", "stale_lease", "not_owner"}:
                return "stale"
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        except OrchestratorError as exc:
            return self._fail(lease, exc.code, retryable=exc.code in _RETRYABLE)
        return outcome

    def _execute(self, lease, payload: dict) -> str:
        if set(payload) != {
            "launch_id",
            "agent_id",
            "run_id",
            "scenario",
            "reserve_cents",
            "charged_cents",
        }:
            raise OrchestratorError("handler_rejected")
        scenario = payload["scenario"]
        if scenario not in _SCENARIOS:
            raise OrchestratorError("handler_rejected")
        launch_id = payload["launch_id"]
        agent_id, run_id = mock_identity(self.tenant_id, launch_id)
        reserve_cents = int(payload["reserve_cents"])
        charged_cents = int(payload["charged_cents"])
        gate = self._control_gate()
        if gate != "allowed":
            self.stop()
            raise OrchestratorError(gate)
        state = self._launch_state(launch_id)
        reservation = self.budget.get_reservation(launch_id)
        if reservation is not None and reservation.tenant_id != self.tenant_id:
            raise OrchestratorError("tenant_mismatch")
        if reservation is not None and reservation.state in _TERMINAL_SETTLED:
            return self._complete(lease, launch_id, reservation.state.lower())
        if state == "LAUNCH_REJECTED":
            return self._complete(lease, launch_id, "rejected")
        if reservation is None:
            reservation = self.budget.request_reserve(
                launch_id, self.tenant_id, reserve_cents, agent_id=None, run_id=None
            )
            self.audit.write("budget_reserved", tenant_id=self.tenant_id, launch_id=launch_id, result=reservation.state)
        if reservation.state == BLOCKED:
            raise OrchestratorError(reservation.block_reason or "blocked")
        if reservation.state != RESERVED:
            raise OrchestratorError("invalid_state")
        resume_unconfirmed = state in {"SEND_STARTED", "LAUNCH_UNKNOWN"}
        if state is None:
            self._adapter(
                "SELECT autopilot_queue.launch_protocol_record_intent(%s, %s, %s, %s, %s)",
                (self.tenant_id, launch_id, agent_id, 1, _MOCK_BODY_HASH),
            )
            state = "INTENT"
            self.audit.write("launch_intent", tenant_id=self.tenant_id, launch_id=launch_id, result="INTENT")
        if state == "INTENT":
            self._adapter(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                (self.tenant_id, launch_id),
            )
            state = "SEND_STARTED"
        if resume_unconfirmed:
            self._recover_unconfirmed(launch_id)
        if state == "SEND_STARTED":
            transport = MockCursorTransport(scenario)
            self.mock_sends += 1
            try:
                created = transport.create_agent({"tenant_id": self.tenant_id, "launch_id": launch_id})
            except RuntimeError:
                raise OrchestratorError("handler_failed") from None
            self.external_calls = 0
            self.cloud_agents_started = 0
            if created.proof_class != "MOCK_TRUSTED":
                raise OrchestratorError("invalid_caller_or_state")
            if scenario in {"timeout", "ambiguous"}:
                raise OrchestratorError(scenario)
            if scenario == "conflict":
                raise OrchestratorError("conflict")
            digest = hashlib.sha256(
                json.dumps(created.proof_blob, sort_keys=True, separators=(",", ":")).encode("utf-8")
            ).hexdigest()
            self._adapter(
                "SELECT autopilot_queue.launch_protocol_report_outbound_result(%s, %s, %s, %s)",
                (self.tenant_id, launch_id, created.proof_class, digest),
            )
            if scenario == "definite_reject":
                self._adapter(
                    "SELECT autopilot_queue.launch_protocol_reject_definite(%s, %s, %s)",
                    (self.tenant_id, launch_id, created.proof_class),
                )
                return self._complete(lease, launch_id, "rejected")
            if created.agent_id != agent_id or created.run_id != run_id:
                raise OrchestratorError("identity_mismatch")
            self._adapter(
                "SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s)",
                (self.tenant_id, launch_id, agent_id, run_id),
            )
            state = "LAUNCH_CONFIRMED"
            self.audit.write("launch_confirmed", tenant_id=self.tenant_id, launch_id=launch_id, result="LAUNCH_CONFIRMED")
        if state == "LAUNCH_CONFIRMED":
            return self._settle(lease, launch_id, agent_id, run_id, charged_cents)
        raise OrchestratorError("invalid_state")

    def _settle(self, lease, launch_id: str, agent_id: str, run_id: str, charged_cents: int) -> str:
        gate = self._control_gate()
        if gate != "allowed":
            self.stop()
            raise OrchestratorError(gate)
        charged_text = str(charged_cents)
        payload = {"rawCostCents": charged_text, "chargedCents": charged_text}
        verification_id, payload_digest = usage_verification_refs(agent_id, run_id, payload)
        try:
            row = self.budget.commit_usage(
                launch_id,
                raw_cost_text=charged_text,
                charged_text=charged_text,
                charged_known=True,
                agent_id=agent_id,
                run_id=run_id,
                verification_id=verification_id,
                payload_digest=payload_digest,
                source="mock_orchestrator",
            )
        except BudgetControllerError as exc:
            raise OrchestratorError(exc.code) from None
        result = "overrun" if row.state == OVERRUN else "committed"
        self.audit.write("budget_settled", tenant_id=self.tenant_id, job_id=lease.job_id, launch_id=launch_id, result=result)
        return self._complete(lease, launch_id, result)

    def _complete(self, lease, launch_id: str, result: str) -> str:
        try:
            self.queue.complete(lease.tenant_id, lease.job_id, lease.lease_owner, lease.lease_token)
        except QueueError as exc:
            raise OrchestratorError(exc.code) from None
        self.audit.write(
            "job_completed",
            tenant_id=self.tenant_id,
            job_id=lease.job_id,
            launch_id=launch_id,
            result=result,
        )
        return result

    def _recover_unconfirmed(self, launch_id: str) -> None:
        """One fenced recovery poll. Does not call the mock transport again."""

        state = self._launch_state(launch_id)
        if state == "SEND_STARTED":
            self._adapter(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s, NULL)",
                (self.tenant_id, launch_id),
            )
            self.audit.write("launch_unknown", tenant_id=self.tenant_id, launch_id=launch_id, result="LAUNCH_UNKNOWN")
        elif state != "LAUNCH_UNKNOWN":
            raise OrchestratorError("invalid_state")
        try:
            connection = self._recovery_connect()
        except Exception:
            raise OrchestratorError("store_unavailable") from None
        try:
            with _cursor_for(connection) as cursor:
                cursor.execute(
                    """
                    SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s) AS token
                    """,
                    (self.tenant_id, launch_id),
                )
                acquired = _one(cursor)
                if acquired is None or acquired["token"] is None:
                    raise OrchestratorError("invalid_lease")
                token = acquired["token"]
                decision = ""
                while True:
                    cursor.execute(
                        """
                        SELECT poll_count, decision
                        FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)
                        """,
                        (self.tenant_id, launch_id, token),
                    )
                    polled = _one(cursor)
                    if polled is None:
                        raise OrchestratorError("invalid_state")
                    decision = str(polled["decision"])
                    if decision == "MANUAL":
                        gate = self._control_gate()
                        if gate != "allowed":
                            self.stop()
                            raise OrchestratorError(gate)
                        cursor.execute(
                            """
                            SELECT autopilot_queue.launch_protocol_escalate_manual(%s, %s, %s)
                            """,
                            (self.tenant_id, launch_id, token),
                        )
                        break
                    if int(polled["poll_count"]) > 100:
                        raise OrchestratorError("invalid_state")
            connection.commit()
        except OrchestratorError:
            connection.rollback()
            raise
        except Exception as exc:
            connection.rollback()
            code = _exception_code(exc) or "store_unavailable"
            raise OrchestratorError(code) from None
        finally:
            connection.close()
        self.audit.write("recovery_poll", tenant_id=self.tenant_id, launch_id=launch_id, result=decision)
        if decision == "MANUAL":
            raise OrchestratorError("manual_reconciliation")
        raise OrchestratorError("recovery_pending")

    def _attempt_budget(self) -> int:
        connection = self._runtime_connect()
        try:
            with _cursor_for(connection) as cursor:
                _assert_runtime_binding(cursor, self.tenant_id)
                cursor.execute(
                    """
                    SELECT max_recovery_polls
                    FROM autopilot_queue.autopilot_launch_control
                    WHERE id = 1
                    """
                )
                row = _one(cursor)
            connection.commit()
        except Exception:
            connection.rollback()
            raise OrchestratorError("store_unavailable") from None
        finally:
            connection.close()
        polls = 24 if row is None else int(row["max_recovery_polls"])
        return min(100, polls + 1)

    def _fail(self, lease, code: str, *, retryable: bool) -> str:
        if code in {"estop_active", "launch_mode_blocked"}:
            self.stop()
        try:
            saved = self.queue.fail(
                lease.tenant_id,
                lease.job_id,
                lease.lease_owner,
                lease.lease_token,
                code if _ERROR.fullmatch(code) else "handler_failed",
                retryable=retryable,
            )
        except QueueError as exc:
            self.audit.write("job_failed", tenant_id=self.tenant_id, job_id=lease.job_id, result=exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        self.audit.write("job_failed", tenant_id=self.tenant_id, job_id=lease.job_id, result=code)
        if saved.status == "DEAD":
            return "dead"
        if saved.status == "PENDING":
            return "retry"
        return "failed"

    def _control_gate(self) -> str:
        try:
            connection = self._runtime_connect()
        except QueueError:
            raise
        except Exception:
            raise OrchestratorError("store_unavailable") from None
        try:
            with _cursor_for(connection) as cursor:
                _assert_runtime_binding(cursor, self.tenant_id)
                cursor.execute(
                    """
                    SELECT autopilot_estop, autopilot_launch_mode
                    FROM autopilot_queue.autopilot_launch_control
                    WHERE id = 1
                    """
                )
                row = _one(cursor)
            connection.commit()
        except Exception:
            connection.rollback()
            raise OrchestratorError("store_unavailable") from None
        finally:
            connection.close()
        if row is None:
            return "launch_mode_blocked"
        if row["autopilot_estop"]:
            return "estop_active"
        if row["autopilot_launch_mode"] != "MOCK_ONLY":
            return "launch_mode_blocked"
        return "allowed"

    def _launch_state(self, launch_id: str) -> str | None:
        connection = self._adapter_connect()
        try:
            with _cursor_for(connection) as cursor:
                cursor.execute(
                    """
                    SELECT lifecycle_state
                    FROM autopilot_queue.launch_intents
                    WHERE tenant_id = %s AND launch_id = %s
                    """,
                    (self.tenant_id, launch_id),
                )
                row = _one(cursor)
            connection.commit()
        except Exception as exc:
            connection.rollback()
            code = _exception_code(exc) or "store_unavailable"
            raise OrchestratorError(code) from None
        finally:
            connection.close()
        if row is None:
            return None
        return str(row["lifecycle_state"])

    def _adapter(self, statement: str, args: tuple) -> None:
        connection = self._adapter_connect()
        try:
            with _cursor_for(connection) as cursor:
                cursor.execute(statement, args)
            connection.commit()
        except Exception as exc:
            connection.rollback()
            code = _exception_code(exc) or "invalid_state"
            raise OrchestratorError(code) from None
        finally:
            connection.close()

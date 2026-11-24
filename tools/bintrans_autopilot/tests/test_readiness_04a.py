"""Dry-run 24/7 readiness. These cases do not open PostgreSQL or start a process."""

from __future__ import annotations

import os
import subprocess
import tempfile
import threading
import unittest
from datetime import datetime, timezone
from io import StringIO
from pathlib import Path
from contextlib import redirect_stdout

from bintrans_autopilot.__main__ import main
from bintrans_autopilot.audit_gateway import AuditError, AuditFinding, AuditGateway, AuditPacket
from bintrans_autopilot.audit_journal import AuditJournal, AuditJournalError
from bintrans_autopilot.config import AUDIT_JOURNAL_PATH, RESTART_LEDGER_PATH
from bintrans_autopilot.postgres_queue import JobLease, JobStatus, PostgresQueue, QueueError
from bintrans_autopilot.readiness import ReadinessCycle, readiness_check, readiness_lines
from bintrans_autopilot.scheduler import Policy, Scheduler
from bintrans_autopilot.supervisor import SupervisorError, WorkerObservation, WorkerSupervisor
from bintrans_autopilot.task_queue import QueueError as TaskQueueError
from bintrans_autopilot.task_queue import Task, TaskStore
from bintrans_autopilot.tests._harness import open_gateway, open_supervisor
from bintrans_autopilot.worker import AutopilotWorker, WorkerError

SHA = "a9ef8f40bf0e7f75cb4aee3794e7841723347e4c"
BASE = "2a9c656ca7fce67bf80a7dccfe8114834b08388b"
_LOCKED = (
    "AUTOPILOT_ENABLED",
    "PAID_AUTOMATION_ENABLED",
    "EXTERNAL_TOOLS_ENABLED",
    "AUTO_MERGE",
    "AUTO_MERGE_ENABLED",
    "AUTO_DEPLOY",
    "AUTO_DEPLOY_ENABLED",
    "WORKER_ENABLED",
)


class Clock:
    def __init__(self, now: float = 0.0):
        self.now = now

    def __call__(self) -> float:
        return self.now


def _status(job_id: str, tenant_id: str, status: str, attempt: int = 1) -> JobStatus:
    return JobStatus(job_id, tenant_id, "ping", 1, status, attempt, 3, "")


def _lease(job_id: str, tenant_id: str, owner: str, token: str, attempt: int = 1) -> JobLease:
    return JobLease(job_id, tenant_id, token, owner, attempt, "RUNNING")


class SlotQueue:
    def __init__(self):
        self.autopilot_enabled = False
        self.paid_automation_enabled = False
        self.lock = threading.Lock()
        self.claimed = False
        self.tenants: list[str] = []
        self.completed = 0

    def requeue_expired(self, tenant_id: str) -> int:
        self.tenants.append(tenant_id)
        return 0

    def claim(self, tenant_id: str, owner: str, lease_seconds: int):
        del lease_seconds
        with self.lock:
            self.tenants.append(tenant_id)
            if self.claimed:
                return None
            self.claimed = True
            return _lease("job-1", tenant_id, owner, "token-1")

    def get_status(self, tenant_id: str, job_id: str) -> JobStatus:
        return _status(job_id, tenant_id, "RUNNING")

    def complete(self, tenant_id: str, job_id: str, owner: str, lease_token: str) -> JobStatus:
        del owner, lease_token
        self.completed += 1
        return _status(job_id, tenant_id, "SUCCEEDED")

    def fail(self, *args, **kwargs):
        del args, kwargs
        raise AssertionError("fail was not part of this case")


class ScriptedQueue:
    def __init__(self, *, claim_error: str = "", complete_error: str = "", renew_error: str = ""):
        self.autopilot_enabled = False
        self.paid_automation_enabled = False
        self.claim_error = claim_error
        self.complete_error = complete_error
        self.renew_error = renew_error
        self.claims = 0
        self.completes = 0
        self.fails = 0
        self.tenants: list[str] = []

    def requeue_expired(self, tenant_id: str) -> int:
        self.tenants.append(tenant_id)
        return 1 if self.claims else 0

    def claim(self, tenant_id: str, owner: str, lease_seconds: int):
        del lease_seconds
        self.tenants.append(tenant_id)
        if self.claim_error:
            raise QueueError(self.claim_error)
        self.claims += 1
        return _lease("job-1", tenant_id, owner, f"token-{self.claims}", self.claims)

    def get_status(self, tenant_id: str, job_id: str) -> JobStatus:
        return _status(job_id, tenant_id, "RUNNING", self.claims)

    def renew_lease(self, tenant_id: str, job_id: str, owner: str, lease_token: str, lease_seconds: int):
        del lease_seconds
        if self.renew_error and lease_token == "token-1":
            raise QueueError(self.renew_error)
        return _lease(job_id, tenant_id, owner, lease_token, 2)

    def complete(self, tenant_id: str, job_id: str, owner: str, lease_token: str) -> JobStatus:
        del owner, lease_token
        if self.complete_error:
            raise QueueError(self.complete_error)
        self.completes += 1
        return _status(job_id, tenant_id, "SUCCEEDED", self.claims)

    def fail(self, tenant_id: str, job_id: str, owner: str, lease_token: str, error_code: str, *, retryable: bool):
        del owner, lease_token
        self.fails += 1
        status = "DEAD" if not retryable else "PENDING"
        return _status(job_id, tenant_id, status)


def _worker(queue, **kwargs) -> AutopilotWorker:
    values = {"mode": "dry_run", "tenant_id": "tenant-a", "owner": "worker-a"}
    values.update(kwargs)
    return AutopilotWorker(queue, lambda: None, **values)


def _observe(worker_id: str = "worker-a", **kwargs) -> WorkerObservation:
    values = {"heartbeat_at": 100.0, "running_jobs": 0, "dead_jobs": 0}
    values.update(kwargs)
    return WorkerObservation(worker_id, "tenant-a", **values)


class ReadinessTests(unittest.TestCase):
    def setUp(self) -> None:
        self._saved = {name: os.environ.get(name) for name in _LOCKED}
        for name in _LOCKED:
            os.environ.pop(name, None)
        self._saved_actions = os.environ.get("GITHUB_ACTIONS")
        os.environ.pop("GITHUB_ACTIONS", None)
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.state_path = Path(self._tmp.name) / "restarts.jsonl"
        self.journal_path = Path(self._tmp.name) / "audit.jsonl"

    def tearDown(self) -> None:
        for name, value in self._saved.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value
        if self._saved_actions is None:
            os.environ.pop("GITHUB_ACTIONS", None)
        else:
            os.environ["GITHUB_ACTIONS"] = self._saved_actions

    def _supervisor(self, **kwargs) -> WorkerSupervisor:
        kwargs.setdefault("tenant_id", "tenant-a")
        kwargs.setdefault("state_path", self.state_path)
        return open_supervisor(**kwargs)

    def _gateway(self, **kwargs) -> AuditGateway:
        kwargs.setdefault("journal_path", self.journal_path)
        kwargs.setdefault("observed_head", SHA)
        return open_gateway(**kwargs)

    def test_sources_do_not_spawn_or_enable_switches(self) -> None:
        root = Path(__file__).resolve().parents[1]
        for name in ("supervisor.py", "audit_gateway.py", "readiness.py"):
            source = root.joinpath(name).read_text(encoding="utf-8")
            for banned in ("subprocess", "socket", "cursor_api", "os.system", "Popen", "docker"):
                self.assertNotIn(banned, source)
        supervisor = self._supervisor(clock=Clock(100))
        self.assertFalse(supervisor.worker_enabled)
        self.assertFalse(supervisor.autopilot_enabled)
        self.assertFalse(supervisor.auto_merge_enabled)
        code, lines = readiness_lines({})
        self.assertEqual(code, 0)
        self.assertIn("PRODUCTION_24X7_ENABLED=NO", lines)
        self.assertIn("WORKER_ENABLED=NO", lines)
        self.assertIn("PAID_AUTOMATION_ENABLED=NO", lines)

    def test_enabled_switch_refuses_supervision_and_the_check(self) -> None:
        for name in _LOCKED:
            os.environ[name] = "true"
            try:
                with self.assertRaises(SupervisorError) as raised:
                    WorkerSupervisor(tenant_id="tenant-a")
                self.assertEqual(raised.exception.code, "supervisor_disabled")
                with self.assertRaises(AuditError):
                    AuditGateway()
            finally:
                os.environ.pop(name, None)
            code, lines = readiness_lines({name: "true", "AUTOPILOT_EMERGENCY_STOP": "true"})
            self.assertEqual(code, 2)
            self.assertIn("READINESS_CHECK=REFUSED", lines)
            shown = {"AUTO_MERGE_ENABLED": "AUTO_MERGE", "AUTO_DEPLOY_ENABLED": "AUTO_DEPLOY"}.get(name, name)
            self.assertIn(f"{shown}=YES", lines)
            self.assertNotIn(f"{shown}=NO", lines)
        with self.assertRaises(WorkerError):
            _worker(ScriptedQueue(), mode="live")

    def test_postgres_unavailable_does_not_complete(self) -> None:
        queue = ScriptedQueue(claim_error="store_unavailable")
        worker = _worker(queue)
        self.assertEqual(worker.run_once(), "unavailable")
        self.assertEqual(worker.metrics.jobs_completed_total, 0)
        self.assertEqual(queue.completes, 0)
        self.assertEqual(worker.external_calls, 0)

    def test_worker_crash_restarts_are_limited(self) -> None:
        queue = ScriptedQueue()
        def crash(_context) -> None:
            raise RuntimeError("crashed")

        worker = _worker(queue, handlers={"ping": crash})
        self.assertEqual(worker.run_once(), "retry")
        self.assertEqual(queue.completes, 0)
        self.assertEqual(queue.fails, 1)
        clock = Clock(100)
        supervisor = self._supervisor(clock=clock, max_restarts=3, base_backoff_seconds=1, max_backoff_seconds=8)
        stale = _observe(heartbeat_at=0, running_jobs=1)
        first = supervisor.observe(stale)
        self.assertEqual(first.action, "restart")
        self.assertEqual(first.reason, "stuck_job")
        self.assertFalse(first.completes_job)
        self.assertEqual(supervisor.observe(stale).action, "wait")
        clock.now = 101
        self.assertEqual(supervisor.observe(stale).restart_count, 2)
        clock.now = 103
        self.assertEqual(supervisor.observe(stale).restart_count, 3)
        clock.now = 107
        held = supervisor.observe(stale)
        self.assertEqual(held.action, "hold")
        self.assertEqual(held.reason, "restart_budget_exhausted")
        self.assertEqual(supervisor.observe(stale).restart_count, 3)

    def test_lost_lease_does_not_complete(self) -> None:
        queue = ScriptedQueue(complete_error="lease_expired")
        worker = _worker(queue)
        self.assertEqual(worker.run_once(), "failed")
        self.assertEqual(queue.completes, 0)
        self.assertEqual(worker.metrics.jobs_completed_total, 0)

    def test_two_workers_take_one_claim(self) -> None:
        queue = SlotQueue()
        barrier = threading.Barrier(2)
        results: list[str] = []
        lock = threading.Lock()

        def run(owner: str) -> None:
            worker = _worker(queue, owner=owner)
            barrier.wait(5)
            outcome = worker.run_once()
            with lock:
                results.append(outcome)

        threads = [threading.Thread(target=run, args=(owner,)) for owner in ("worker-a", "worker-b")]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(5)
            self.assertFalse(thread.is_alive())
        self.assertEqual(sorted(results), ["completed", "idle"])
        self.assertEqual(queue.completed, 1)
        self.assertTrue(all(tenant == "tenant-a" for tenant in queue.tenants))

    def test_scheduler_restart_does_not_duplicate(self) -> None:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        root = Path(directory.name) / "workspace"
        allowed = root / "docs" / "engineering" / "autopilot" / "SCHEDULER_0_3A.md"
        allowed.parent.mkdir(parents=True)
        allowed.write_text("dry-run\n", encoding="utf-8")
        moment = datetime(2026, 11, 23, 9, 0, tzinfo=timezone.utc)
        store = TaskStore(Path(directory.name) / "queue.json")
        item = Task(
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
        store.enqueue(item, moment, "s1")
        reopened = TaskStore(store.path)
        with self.assertRaises(TaskQueueError) as raised:
            reopened.enqueue(item, moment, "s2")
        self.assertEqual(raised.exception.code, "duplicate_task")
        policy = Policy(
            approved_prefixes=("docs/",),
            forbidden_prefixes=(".env", "ops/"),
            workspace_root=str(root.resolve()),
        )
        first = Scheduler(store, "s1", policy, lambda: moment).tick()
        second = Scheduler(reopened, "s2", policy, lambda: moment).tick()
        tasks = reopened.list_tasks(moment)
        self.assertEqual([task.id for task in tasks], ["queue-1"])
        self.assertEqual(tasks[0].status, "REVIEW_REQUIRED")
        self.assertEqual(first.cloud_agents_started, 0)
        self.assertEqual(second.cloud_agents_started, 0)
        self.assertFalse(first.merge_attempted)
        self.assertFalse(second.deploy_attempted)

    def test_success_without_confirmation_stays_unconfirmed(self) -> None:
        queue = ScriptedQueue(complete_error="store_unavailable")
        worker = _worker(queue)
        self.assertEqual(worker.run_once(), "unavailable")
        self.assertEqual(worker.metrics.jobs_completed_total, 0)
        self.assertEqual(queue.completes, 0)

    def test_expired_lease_can_be_claimed_again(self) -> None:
        queue = ScriptedQueue(renew_error="lease_expired")

        def handler(context) -> None:
            if context.attempt_count == 1:
                context.renew()

        worker = _worker(queue, handlers={"ping": handler})
        self.assertEqual(worker.run_once(), "failed")
        self.assertEqual(queue.completes, 0)
        self.assertEqual(worker.run_once(), "completed")
        self.assertEqual(queue.completes, 1)
        self.assertEqual(worker.metrics.jobs_completed_total, 1)
        self.assertEqual(queue.claims, 2)

    def test_invalid_payload_never_connects_or_runs(self) -> None:
        def connect():
            raise AssertionError("connected")

        queue = PostgresQueue(connect)
        with self.assertRaises(QueueError) as raised:
            queue.enqueue(
                tenant_id="tenant-a",
                job_type="ping",
                payload={"n": 1, "command": "git push"},
                idempotency_key="bad-payload",
            )
        self.assertEqual(raised.exception.code, "invalid_input")
        scripted = ScriptedQueue()
        worker = _worker(scripted, allow={"ping"})
        scripted.get_status = lambda tenant_id, job_id: JobStatus(job_id, tenant_id, "shell", 1, "RUNNING", 1, 3, "")
        self.assertEqual(worker.run_once(), "dead")
        self.assertEqual(scripted.fails, 1)
        self.assertEqual(scripted.completes, 0)
        self.assertEqual(worker.external_calls, 0)

    def test_forged_reviewer_cannot_clear_and_a_block_sticks(self) -> None:
        gateway = self._gateway()
        command = "python -m unittest discover -s tools/bintrans_autopilot/tests -v"
        forged = gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "PASS"))
        self.assertEqual(forged.reason, "trust_boundary_absent")
        self.assertEqual(forged.finding_gate, "blocked")
        self.assertFalse(forged.reviewer_authenticated)
        self.assertFalse(forged.ci_verified)
        self.assertFalse(forged.merge_authorized)
        spaced = gateway.submit(AuditPacket(SHA, command, "PASS", "  AGENT-J ", "PASS"))
        self.assertEqual(spaced.finding_gate, "blocked")
        self.assertNotEqual(spaced.reason, "recorded_for_controller")
        mismatch = gateway.submit(AuditPacket("0" * 40, "true", "PASS", "agent-j", "PASS"))
        self.assertEqual(mismatch.reason, "sha_mismatch")
        self.assertEqual(mismatch.finding_gate, "blocked")
        negative = gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "CHANGES_REQUIRED"))
        self.assertEqual(negative.reason, "blocking_finding")
        later = gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "PASS"))
        self.assertEqual(later.reason, "prior_block_stands")
        self.assertEqual(later.finding_gate, "blocked")
        critical = self._gateway(journal_path=Path(self._tmp.name) / "severity.jsonl").submit(
            AuditPacket(SHA, command, "PASS", "agent-j", "PASS", (AuditFinding("J-2", "CRITICAL", False),))
        )
        self.assertEqual(critical.reason, "blocking_severity")
        self.assertFalse(critical.production_enabled)
        reloaded = self._gateway()
        self.assertEqual(reloaded.submit(AuditPacket(SHA, command, "PASS", "agent-j", "PASS")).reason, "prior_block_stands")
        with self.assertRaises(AuditJournalError):
            reloaded.journal.clear()
        with self.assertRaises(AuditError):
            gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "PASS", (AuditFinding("J-3", "EXTREME", False),)))

    def test_journal_detects_rewrites_and_keeps_history(self) -> None:
        gateway = self._gateway(journal_path=Path(self._tmp.name) / "chain.jsonl")
        command = "python -m unittest discover -s tools/bintrans_autopilot/tests -v"
        gateway.submit(AuditPacket(SHA, command, "FAIL", "agent-h", "BLOCKED"))
        original = gateway.journal.path.read_text(encoding="utf-8")
        self.assertGreater(len(gateway.packets()), 0)
        gateway.journal.path.write_text(original.replace("BLOCKED", "PASS"), encoding="utf-8")
        with self.assertRaises(AuditError) as raised:
            gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "PASS"))
        self.assertEqual(raised.exception.code, "journal_tampered")
        with self.assertRaises(AuditJournalError):
            gateway.journal.delete()
        with self.assertRaises(AuditJournalError):
            gateway.journal.update({})

    def test_restart_budget_is_not_reset_by_a_new_id_or_instance(self) -> None:
        clock = Clock(100)
        supervisor = self._supervisor(clock=clock, max_restarts=1)
        stale = _observe(heartbeat_at=0, worker_id="worker-a")
        self.assertEqual(supervisor.observe(stale).action, "restart")
        clock.now = 102
        self.assertEqual(supervisor.observe(stale).action, "hold")
        self.assertEqual(supervisor.observe(_observe(heartbeat_at=0, worker_id="worker-b")).action, "hold")
        clock.now = 200
        revived = self._supervisor(clock=clock, max_restarts=1)
        self.assertEqual(revived.observe(_observe(heartbeat_at=0, worker_id="worker-c")).action, "hold")
        fresh = self._supervisor(clock=Clock(50), state_path=Path(self._tmp.name) / "fresh-restarts.jsonl", max_restarts=3)
        future = fresh.observe(_observe(heartbeat_at=1e12))
        self.assertEqual(future.reason, "heartbeat_in_future")
        self.assertNotEqual(future.action, "continue")
        text = self.state_path.read_text(encoding="utf-8")
        self.state_path.write_text(text.replace("restart", "RESTART"), encoding="utf-8")
        tampered = self._supervisor(clock=clock, max_restarts=1)
        self.assertEqual(tampered.observe(stale).reason, "state_tampered")
        with self.assertRaises(SupervisorError):
            tampered._ledger.clear()

    def test_emergency_stop_sources_override_the_caller(self) -> None:
        from unittest.mock import patch

        clock = Clock(100)
        stop = Path(self._tmp.name) / "EMERGENCY_STOP"
        with patch("bintrans_autopilot.config.EMERGENCY_STOP_FILE", stop):
            supervisor = self._supervisor(clock=clock)
            self.assertEqual(supervisor.observe(_observe()).action, "continue")
            stop.write_text("", encoding="utf-8")
            self.assertEqual(supervisor.observe(_observe(), emergency_stop=False).action, "stop")
        os.environ["AUTOPILOT_EMERGENCY_STOP"] = "true"
        try:
            by_env = self._supervisor(clock=clock, state_path=Path(self._tmp.name) / "env-restarts.jsonl")
            self.assertEqual(by_env.observe(_observe(), emergency_stop=False).reason, "emergency_stop")
            report = ReadinessCycle(by_env, self._gateway()).run((_observe(),))
            self.assertEqual(report.decisions[0].action, "stop")
            self.assertFalse(report.merge_authorized)
        finally:
            os.environ.pop("AUTOPILOT_EMERGENCY_STOP", None)

        class _Config:
            emergency_stop = True

        with patch("bintrans_autopilot.config.load_config", return_value=_Config()):
            by_config = self._supervisor(clock=clock, state_path=Path(self._tmp.name) / "config-restarts.jsonl")
            self.assertEqual(by_config.observe(_observe(), emergency_stop=False).action, "stop")

    def test_readiness_lines_report_the_flags_that_are_actually_set(self) -> None:
        code, lines = readiness_lines({"AUTOPILOT_ENABLED": "true"})
        self.assertEqual(code, 2)
        self.assertIn("AUTOPILOT_ENABLED=YES", lines)
        self.assertNotIn("AUTOPILOT_ENABLED=NO", lines)
        code, lines = readiness_lines({"PRODUCTION_24X7_ENABLED": "true"})
        self.assertEqual(code, 2)
        self.assertIn("PRODUCTION_24X7_ENABLED=YES", lines)
        self.assertNotIn("PRODUCTION_24X7_ENABLED=NO", lines)
        os.environ["PRODUCTION_24X7_ENABLED"] = "true"
        try:
            with self.assertRaises(SupervisorError):
                self._supervisor()
            with self.assertRaises(AuditError):
                AuditGateway()
        finally:
            os.environ.pop("PRODUCTION_24X7_ENABLED", None)

    def test_readiness_check_is_behind_the_workflow_guard(self) -> None:
        os.environ["GITHUB_ACTIONS"] = "true"
        buffer = StringIO()
        with redirect_stdout(buffer):
            code = main(["readiness-check"])
        text = buffer.getvalue()
        self.assertEqual(code, 2)
        self.assertIn("UNTRUSTED_WORKFLOW=", text)
        self.assertNotIn("READINESS_CHECK=DRY_RUN", text)
        self.assertNotIn("PAID_AGENTS_LAUNCHED=YES", text)

    def test_resource_limit_and_emergency_stop_hold_the_worker(self) -> None:
        clock = Clock(100)
        supervisor = self._supervisor(clock=clock, max_dead_jobs=2)
        limited = supervisor.observe(_observe(dead_jobs=2))
        self.assertEqual(limited.action, "hold")
        self.assertEqual(limited.reason, "resource_limit")
        self.assertFalse(limited.completes_job)
        self.assertEqual(self._supervisor(clock=clock, max_dead_jobs=2).observe(_observe(dead_jobs=0)).reason, "resource_limit")
        with self.assertRaises(SupervisorError) as raised:
            supervisor.release_hold("worker-a")
        self.assertEqual(raised.exception.code, "hold_requires_controller")
        self.assertFalse(supervisor.worker_enabled)
        stopped = supervisor.observe(_observe(), emergency_stop=True)
        self.assertEqual(stopped.action, "stop")
        cycle = ReadinessCycle(supervisor, self._gateway(journal_path=Path(self._tmp.name) / "cycle.jsonl"))
        report = cycle.run((_observe(),), emergency_stop=True)
        self.assertEqual(report.decisions[0].action, "stop")
        self.assertEqual(report.audit_reason, "audit_not_submitted")
        self.assertFalse(report.merge_authorized)
        self.assertEqual(report.cloud_agents_started, 0)

    def test_readiness_command_stays_closed(self) -> None:
        saved = {name: os.environ.get(name) for name in ("AUTOPILOT_EMERGENCY_STOP", "WORKER_ENABLED")}
        os.environ.pop("WORKER_ENABLED", None)
        os.environ["AUTOPILOT_EMERGENCY_STOP"] = "true"
        try:
            buffer = StringIO()
            with redirect_stdout(buffer):
                code = main(["readiness-check"])
            text = buffer.getvalue()
            self.assertEqual(code, 2)
            self.assertIn("READINESS_CHECK=REFUSED", text)
            self.assertIn("REASON=emergency_stop", text)
            self.assertNotIn("READINESS_CHECK=DRY_RUN", text)
            self.assertIn("EMERGENCY_STOP=YES", text)
            self.assertIn("PRODUCTION_24X7_ENABLED=NO", text)
            self.assertNotIn("AUTOPILOT_ENABLED=YES", text)
        finally:
            for name, value in saved.items():
                if value is None:
                    os.environ.pop(name, None)
                else:
                    os.environ[name] = value

    def test_example_compose_is_not_started_by_default(self) -> None:
        text = Path(__file__).resolve().parents[1].joinpath("deploy", "autopilot-readiness.compose.example.yml").read_text(encoding="utf-8")
        self.assertIn("manual-approval-required", text)
        self.assertIn('restart: "no"', text)
        self.assertIn("example.invalid/bintrans-autopilot-dry-run:disabled", text)
        self.assertIn("mem_limit: 256m", text)
        self.assertIn('AUTOPILOT_EMERGENCY_STOP: "true"', text)
        self.assertNotIn("docker compose up", text)

    def test_config_emergency_stop_refuses_readiness_and_matches_supervisor(self) -> None:
        from unittest.mock import patch

        class _Config:
            emergency_stop = True

        stop = Path(self._tmp.name) / "absent-stop"
        with patch("bintrans_autopilot.config.load_config", return_value=_Config()):
            code, lines = readiness_lines({})
            self.assertEqual(code, 2)
            self.assertIn("READINESS_CHECK=REFUSED", lines)
            self.assertIn("REASON=emergency_stop", lines)
            self.assertIn("EMERGENCY_STOP=YES", lines)
            self.assertNotIn("READINESS_CHECK=DRY_RUN", lines)
            buffer = StringIO()
            with redirect_stdout(buffer):
                exit_code = readiness_check()
            self.assertEqual(exit_code, 2)
            self.assertIn("READINESS_CHECK=REFUSED", buffer.getvalue())
            self.assertNotIn("READINESS_CHECK=DRY_RUN", buffer.getvalue())
            supervisor = self._supervisor(clock=Clock(100), state_path=stop.parent / "config-stop-ledger.jsonl")
            decision = supervisor.observe(_observe(), emergency_stop=False)
        self.assertEqual(decision.action, "stop")
        self.assertEqual(decision.reason, "emergency_stop")

    def test_production_constructors_reject_override_arguments(self) -> None:
        with self.assertRaises(TypeError):
            WorkerSupervisor(tenant_id="tenant-a", env={})
        with self.assertRaises(TypeError):
            WorkerSupervisor(tenant_id="tenant-a", config_stop=False)
        with self.assertRaises(TypeError):
            WorkerSupervisor(tenant_id="tenant-a", state_path=self.state_path)
        with self.assertRaises(TypeError):
            AuditGateway(env={})
        with self.assertRaises(TypeError):
            AuditGateway(journal_path=self.journal_path)
        self.assertEqual(RESTART_LEDGER_PATH.name, "restart-ledger.jsonl")
        self.assertEqual(AUDIT_JOURNAL_PATH.name, "audit-journal.jsonl")
        root = Path(__file__).resolve().parents[1]
        self.assertFalse((root / "testing.py").exists())
        self.assertFalse((root / "test_bind.py").exists())
        for name in ("__main__.py", "readiness.py", "worker.py", "supervisor.py", "audit_gateway.py", "config.py"):
            text = root.joinpath(name).read_text(encoding="utf-8")
            self.assertNotIn("config_stop", text)
            self.assertNotIn("test_bind", text)
            self.assertNotIn("bintrans_autopilot.tests", text)
            self.assertNotIn("open_supervisor", text)
            self.assertNotIn("open_gateway", text)

    def test_torn_truncated_journal_stays_refused_after_restart(self) -> None:
        gateway = self._gateway()
        gateway.submit(AuditPacket(SHA, "true", "FAIL", "agent-h", "BLOCKED"))
        raw = gateway.journal.path.read_bytes()
        gateway.journal.path.write_bytes(raw[:-3])
        with self.assertRaises(AuditJournalError) as raised:
            gateway.journal.verify()
        self.assertEqual(raised.exception.code, "journal_tampered")
        reopened = AuditJournal(gateway.journal.path)
        with self.assertRaises(AuditJournalError) as again:
            reopened.append("packet", {"n": 1})
        self.assertEqual(again.exception.code, "journal_tampered")
        self.assertTrue(gateway.journal.path.exists())
        self.assertGreater(gateway.journal.path.stat().st_size, 0)

    def test_deleted_journal_is_not_recreated_and_blocks_stay_closed(self) -> None:
        gateway = self._gateway()
        command = "python -m unittest discover -s tools/bintrans_autopilot/tests -v"
        gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "CHANGES_REQUIRED"))
        gateway.journal.path.unlink()
        with self.assertRaises(AuditJournalError) as missing:
            AuditJournal(gateway.journal.path)
        self.assertEqual(missing.exception.code, "journal_missing")
        self.assertFalse(gateway.journal.path.exists())
        with self.assertRaises(AuditError) as refused:
            gateway.submit(AuditPacket(SHA, command, "PASS", "agent-j", "PASS"))
        self.assertEqual(refused.exception.code, "journal_missing")
        self.assertFalse(gateway.journal.path.exists())
        self.assertFalse(AUDIT_JOURNAL_PATH.exists())
        with self.assertRaises(AuditError) as production:
            AuditGateway()
        self.assertEqual(production.exception.code, "journal_missing")
        self.assertFalse(AUDIT_JOURNAL_PATH.exists())

    def test_rebuilt_chain_and_valid_prefix_are_not_locally_anchored(self) -> None:
        journal = self._gateway().journal
        journal.append("packet", {"commit_sha": SHA, "note": "claim"})
        journal.append("block", {"commit_sha": SHA, "reason": "blocking_finding"})
        text = journal.path.read_text(encoding="utf-8")
        journal.path.write_text(text.splitlines()[0] + "\n", encoding="utf-8")
        records = journal.verify()
        self.assertEqual(len(records), 1)
        self.assertEqual(records[0]["kind"], "packet")
        replacement = AuditJournal.create(Path(self._tmp.name) / "replacement.jsonl")
        replacement.append("packet", {"commit_sha": SHA, "note": "rebuilt"})
        journal.path.write_bytes(replacement.path.read_bytes())
        rebuilt = journal.verify()
        self.assertEqual(rebuilt[0]["body"]["note"], "rebuilt")
        self.assertNotIn("blocking_finding", journal.path.read_text(encoding="utf-8"))

    def test_valid_prefix_of_restart_ledger_is_not_independently_anchored(self) -> None:
        clock = Clock(100)
        supervisor = self._supervisor(clock=clock, max_restarts=3)
        stale = _observe(heartbeat_at=0)
        self.assertEqual(supervisor.observe(stale).action, "restart")
        clock.now = 102
        self.assertEqual(supervisor.observe(stale).restart_count, 2)
        text = self.state_path.read_text(encoding="utf-8")
        self.state_path.write_text(text.splitlines()[0] + "\n", encoding="utf-8")
        reopened = self._supervisor(clock=Clock(200), max_restarts=3)
        self.assertEqual(reopened._ledger.count("tenant-a"), 1)
        torn = self.state_path.read_bytes()
        self.state_path.write_bytes(torn[:-2])
        damaged = self._supervisor(clock=Clock(300), max_restarts=3)
        self.assertEqual(damaged.observe(stale).reason, "state_tampered")
        self.assertTrue(self.state_path.exists())
        self.assertGreater(self.state_path.stat().st_size, 0)

    def test_runtime_state_files_are_gitignored_and_untracked(self) -> None:
        repo = Path(__file__).resolve().parents[3]
        tracked = subprocess.check_output(
            ["git", "ls-files", "--", "tools/bintrans_autopilot/state"],
            cwd=repo,
            text=True,
        )
        self.assertEqual(tracked.strip(), "")
        for name in ("audit-journal.jsonl", "restart-ledger.jsonl", "audit-journal.jsonl.lock", "restart-ledger.jsonl.lock"):
            completed = subprocess.run(
                ["git", "check-ignore", "-q", "--", f"tools/bintrans_autopilot/state/{name}"],
                cwd=repo,
                check=False,
            )
            self.assertEqual(completed.returncode, 0, name)
        self.assertFalse(AUDIT_JOURNAL_PATH.exists())
        self.assertFalse(RESTART_LEDGER_PATH.exists())


if __name__ == "__main__":
    unittest.main()

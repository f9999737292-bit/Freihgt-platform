"""Read-only monitor core.

Fixture and mock transports are the only inputs in this build.
A live GitHub call raises before any socket is opened.
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass, field

EXPECTED_REPOSITORY = "f9999737292-bit/Freihgt-platform"
NLO_JOB = "network-optimizer-chain-persistence-integration"
NLO_SIGNATURE = "embedded postgres: no version found matching 16.4.0"

EXIT_REPORT_CLEAR = 0
EXIT_CI_NOT_SUCCESS = 1
EXIT_EMERGENCY_STOP = 2
EXIT_UNKNOWN = 3
EXIT_SECURITY = 4

READ_VERBS = frozenset({"load_runs", "load_jobs", "load_pulls"})
INJECTION_MARKERS = (
    "ignore previous",
    "ignore all previous",
    "gh issue",
    "gh pr",
    "gh api",
    "curl ",
    "autopilot_enabled",
    "enable autopilot",
    "rm -rf",
)


class SecurityDeny(Exception):
    """A write or an unlisted verb was requested."""


class MonitorError(Exception):
    """The monitor refused to continue. This is not a CI pass."""


@dataclass(frozen=True)
class StopReading:
    value: str | None
    unreadable: bool = False


@dataclass
class StopSource:
    """Read on every run. A previous allow is not remembered."""

    value: str | None = None
    unreadable: bool = False

    def read(self) -> StopReading:
        if self.unreadable:
            return StopReading(None, unreadable=True)
        return StopReading(self.value, unreadable=False)


def evaluate_stop(reading: StopReading) -> tuple[bool, str]:
    """Allow only the exact string false. Every other state stops."""
    if reading.unreadable:
        return False, "unreadable"
    if reading.value is None:
        return False, "missing"
    if reading.value == "false":
        return True, "explicit_false"
    if reading.value == "true":
        return False, "true"
    return False, "invalid"


def injection_suspected(text: str) -> bool:
    lowered = text.lower()
    return any(marker in lowered for marker in INJECTION_MARKERS)


def reject_mutation(verb: str) -> None:
    if verb not in READ_VERBS:
        raise SecurityDeny(f"github_mutation_forbidden:{verb}")


def _sha_ok(value: str) -> bool:
    if len(value) != 40:
        return False
    try:
        int(value, 16)
    except ValueError:
        return False
    return value.lower() == value


@dataclass
class Report:
    exit_code: int
    ci_result: str
    stop_reason: str
    repository: str
    observations: list
    groups: list
    pull_requests: list
    actions: list = field(default_factory=list)
    cursor_calls: int = 0
    github_mutations: int = 0
    retries: int = 0
    untrusted_text_executed: bool = False
    injection_suspected: bool = False
    error: str = ""
    transport_loads: int = 0

    def digest(self) -> str:
        payload = {
            "actions": self.actions,
            "ci_result": self.ci_result,
            "cursor_calls": self.cursor_calls,
            "error": self.error,
            "exit_code": self.exit_code,
            "github_mutations": self.github_mutations,
            "groups": self.groups,
            "injection_suspected": self.injection_suspected,
            "observations": self.observations,
            "pull_requests": self.pull_requests,
            "repository": self.repository,
            "retries": self.retries,
            "stop_reason": self.stop_reason,
            "untrusted_text_executed": self.untrusted_text_executed,
        }
        raw = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")
        return hashlib.sha256(raw).hexdigest()


def _finish_early(exit_code: int, ci_result: str, stop_reason: str, repository: str, error: str = "") -> Report:
    return Report(
        exit_code=exit_code,
        ci_result=ci_result,
        stop_reason=stop_reason,
        repository=repository,
        observations=[],
        groups=[],
        pull_requests=[],
        error=error,
    )


def classify_job(job: dict) -> dict:
    conclusion = str(job.get("conclusion") or "")
    excerpt = str(job.get("log_excerpt") or "")
    name = str(job.get("name") or "")
    if conclusion == "failure" and name == NLO_JOB and NLO_SIGNATURE in excerpt:
        labels = ["KNOWN_ERROR_SIGNATURE", "ENVIRONMENT_SUSPECTED", "ROOT_CAUSE_UNCONFIRMED"]
        signature = "embedded_postgres_no_version_16.4.0"
        ci_result = "FAILURE"
    elif conclusion == "failure":
        labels = ["ROOT_CAUSE_UNCONFIRMED"]
        signature = "unclassified_failure"
        ci_result = "FAILURE"
    elif conclusion == "cancelled":
        labels = ["ROOT_CAUSE_UNCONFIRMED"]
        signature = "cancelled"
        ci_result = "CANCELLED"
    elif conclusion == "timed_out":
        labels = ["ROOT_CAUSE_UNCONFIRMED"]
        signature = "timed_out"
        ci_result = "TIMED_OUT"
    elif conclusion == "success":
        labels = []
        signature = "success"
        ci_result = "SUCCESS"
    else:
        labels = ["ROOT_CAUSE_UNCONFIRMED"]
        signature = "unknown_conclusion"
        ci_result = "UNKNOWN"
    return {
        "job_name": name,
        "conclusion": conclusion,
        "head_sha": job.get("head_sha"),
        "run_id": job.get("run_id"),
        "workflow_name": job.get("workflow_name"),
        "labels": labels,
        "signature": signature,
        "ci_result": ci_result,
        "root_cause": "UNCONFIRMED",
        "retry": False,
    }


class FixtureTransport:
    def __init__(self, payload: dict):
        self.payload = payload
        self.loads = 0

    def load(self, repository: str) -> dict:
        del repository
        self.loads += 1
        return self.payload


class FailingTransport:
    def __init__(self, exc: BaseException):
        self.exc = exc
        self.loads = 0

    def load(self, repository: str) -> dict:
        del repository
        self.loads += 1
        raise self.exc


class LiveGitHubTransport:
    """This build has no live GitHub authorization."""

    def load(self, repository: str) -> dict:
        del repository
        raise MonitorError("live_github_not_authorized")


def _overall(observations: list[dict]) -> tuple[str, int]:
    if not observations:
        return "UNKNOWN", EXIT_UNKNOWN
    conclusions = {row["conclusion"] for row in observations}
    if "failure" in conclusions or any(row["ci_result"] == "UNKNOWN" for row in observations):
        if "failure" in conclusions:
            return "FAILURE", EXIT_CI_NOT_SUCCESS
        return "UNKNOWN", EXIT_UNKNOWN
    if "timed_out" in conclusions:
        return "TIMED_OUT", EXIT_CI_NOT_SUCCESS
    if "cancelled" in conclusions:
        return "CANCELLED", EXIT_CI_NOT_SUCCESS
    if all(row["conclusion"] == "success" for row in observations):
        return "SUCCESS", EXIT_REPORT_CLEAR
    return "UNKNOWN", EXIT_UNKNOWN


def run_monitor(
    stop: StopSource,
    transport,
    repository: str,
    expected_sha: str | None = None,
    untrusted_texts: list[str] | None = None,
) -> Report:
    reading = stop.read()
    allowed, stop_reason = evaluate_stop(reading)
    if not allowed:
        return _finish_early(EXIT_EMERGENCY_STOP, "DENIED", stop_reason, repository)
    if repository != EXPECTED_REPOSITORY:
        return _finish_early(EXIT_UNKNOWN, "ERROR", stop_reason, repository, "repository_rejected")
    suspected = any(injection_suspected(text) for text in (untrusted_texts or []))
    try:
        payload = transport.load(repository)
    except MonitorError as exc:
        report = _finish_early(EXIT_UNKNOWN, "ERROR", stop_reason, repository, str(exc))
        report.injection_suspected = suspected
        return report
    except (ConnectionError, TimeoutError, OSError) as exc:
        report = _finish_early(
            EXIT_UNKNOWN,
            "UNKNOWN",
            stop_reason,
            repository,
            f"network_error:{type(exc).__name__}",
        )
        report.injection_suspected = suspected
        return report
    loads = getattr(transport, "loads", 0)
    if payload.get("repository") != EXPECTED_REPOSITORY:
        report = _finish_early(EXIT_UNKNOWN, "ERROR", stop_reason, repository, "repository_rejected")
        report.transport_loads = loads
        report.injection_suspected = suspected
        return report
    observations = []
    seen: set[tuple] = set()
    for run in payload.get("runs") or []:
        run_sha = str(run.get("head_sha") or "")
        run_id = run.get("id")
        if not _sha_ok(run_sha) or (expected_sha is not None and run_sha != expected_sha):
            report = _finish_early(EXIT_UNKNOWN, "ERROR", stop_reason, repository, "sha_rejected")
            report.transport_loads = loads
            report.injection_suspected = suspected
            return report
        for job in run.get("jobs") or []:
            job_sha = str(job.get("head_sha") or "")
            if job_sha != run_sha:
                report = _finish_early(EXIT_UNKNOWN, "ERROR", stop_reason, repository, "sha_mismatch")
                report.transport_loads = loads
                report.injection_suspected = suspected
                return report
            key = (run_id, job.get("name"))
            if key in seen:
                continue
            seen.add(key)
            item = dict(job)
            item["run_id"] = run_id
            item["head_sha"] = job_sha
            item["workflow_name"] = run.get("name")
            observations.append(classify_job(item))
    pulls = [
        {
            "number": pull.get("number"),
            "draft": pull.get("draft") is True,
            "state": pull.get("state"),
            "head_sha": pull.get("head_sha"),
        }
        for pull in (payload.get("pulls") or [])
    ]
    ci_result, exit_code = _overall(observations)
    groups_map: dict[tuple, list] = {}
    for row in observations:
        if row["signature"] == "success":
            continue
        groups_map.setdefault((row["signature"], row["job_name"]), []).append(row["run_id"])
    groups = [
        {
            "signature": signature,
            "job_name": job_name,
            "run_ids": run_ids,
            "count": len(run_ids),
            "ci_result": "FAILURE" if signature != "cancelled" and signature != "timed_out" else signature.upper(),
            "retry": False,
        }
        for (signature, job_name), run_ids in sorted(groups_map.items())
    ]
    return Report(
        exit_code=exit_code,
        ci_result=ci_result,
        stop_reason=stop_reason,
        repository=repository,
        observations=observations,
        groups=groups,
        pull_requests=pulls,
        actions=[],
        cursor_calls=0,
        github_mutations=0,
        retries=0,
        untrusted_text_executed=False,
        injection_suspected=suspected,
        transport_loads=loads,
    )

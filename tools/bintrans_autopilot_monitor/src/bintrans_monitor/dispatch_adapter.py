"""Offline GitHub dispatch boundary for the local budget gate.

A real network transport is disabled. This module does not read a token,
does not start a shell, and does not import an HTTP client. Workflow
dispatch accepts a branch or tag name as ref. A commit SHA is not sent as
that ref. The expected SHA is bound by a commit read and by the head_sha
input the candidate workflow already declares.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Mapping, Sequence

from bintrans_monitor.budget_gate import (
    AdapterResult,
    BudgetDeny,
    DispatchAdapter,
    DispatchNotSent,
    DispatchRequest,
)

_SHA = re.compile(r"^[0-9a-f]{40}$")
_TOKEN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$")
_RUN_NAME = re.compile(
    r"^bintrans-budget ([A-Za-z0-9][A-Za-z0-9._-]{0,79}) ([A-Za-z0-9][A-Za-z0-9._-]{0,79}) ([0-9a-f]{40})$"
)
_SECRET = re.compile(r"ghp_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+|Bearer [A-Za-z0-9._\-]+")


@dataclass(frozen=True)
class TrustedRefBinding:
    ref: str
    object_sha: str
    repository: str
    workflow: str


@dataclass(frozen=True)
class TransportResponse:
    status: int
    body: Mapping[str, object] | None = None
    redirected: bool = False
    location: str = ""


class OfflineTransport:
    """Scripted transport. It records calls and does not open a connection."""

    def __init__(self, handler):
        self._handler = handler
        self.calls: list[tuple[str, str]] = []

    def exchange(self, method: str, path: str, body: Mapping[str, object] | None) -> TransportResponse:
        self.calls.append((method, path))
        return self._handler(method, path, body)


class RealGitHubTransport:
    """Fail closed before any connection. No live credential is read."""

    def exchange(self, method: str, path: str, body: Mapping[str, object] | None) -> TransportResponse:
        del method, path, body
        raise BudgetDeny("real_transport_disabled")


class CredentialProvider:
    """Classifications only. No secret value is stored or returned."""

    def monitor_token(self) -> str:
        return "READ_ONLY"

    def dispatch_principal(self) -> str:
        return "SEPARATE"

    def cancel_principal(self) -> str:
        return "SEPARATE_FROM_MONITOR"

    def live_secret(self) -> None:
        raise BudgetDeny("live_credentials_absent")


def redact(text: str) -> str:
    if _SECRET.search(text):
        return "transport_error"
    return "transport_error"


class GitHubReadAdapter:
    def __init__(self, transport: OfflineTransport, *, repository: str, workflow: str, expected_sha: str, allowed_ref: str):
        self.transport = transport
        self.repository = repository
        self.workflow = workflow
        self.expected_sha = expected_sha
        self.allowed_ref = allowed_ref

    def read_commit(self, sha: str) -> Mapping[str, object]:
        path = f"/repos/{self.repository}/git/commits/{sha}"
        return self._get(path)

    def read_run(self, run_id: str) -> Mapping[str, object]:
        path = f"/repos/{self.repository}/actions/runs/{run_id}"
        return self._get(path)

    def read_jobs(self, run_id: str, page: int) -> Mapping[str, object]:
        if not isinstance(page, int) or isinstance(page, bool) or page < 1 or page > 50:
            raise BudgetDeny("usage_incomplete")
        path = f"/repos/{self.repository}/actions/runs/{run_id}/jobs?per_page=100&page={page}"
        return self._get(path)

    def collect_jobs(self, run_id: str) -> list[Mapping[str, object]]:
        found: list[Mapping[str, object]] = []
        total: int | None = None
        page = 1
        while page <= 50:
            body = self.read_jobs(run_id, page)
            jobs = body.get("jobs")
            if not isinstance(jobs, list):
                raise BudgetDeny("usage_incomplete")
            count = body.get("total_count")
            if isinstance(count, bool) or not isinstance(count, int) or count < 0:
                raise BudgetDeny("usage_incomplete")
            total = count
            found.extend(item for item in jobs if isinstance(item, Mapping))
            if len(found) >= total:
                return found
            if not jobs:
                raise BudgetDeny("usage_incomplete")
            page += 1
        raise BudgetDeny("usage_incomplete")

    def duration_report(self, run_id: str) -> dict[str, object]:
        """Job duration is an estimate. It is not billable usage."""
        jobs = self.collect_jobs(run_id)
        return {
            "source": "actions_jobs_duration",
            "estimated_only": True,
            "billing_authoritative": False,
            "jobs_complete": True,
            "pages_complete": True,
            "jobs": jobs,
            "run_id": run_id,
        }

    def _get(self, path: str) -> Mapping[str, object]:
        _assert_allowed("GET", path, self.repository, self.workflow)
        response = self.transport.exchange("GET", path, None)
        if response.redirected:
            raise BudgetDeny("unsafe_redirect")
        if response.body is None:
            raise BudgetDeny("usage_untrusted")
        return response.body


class CancellationInterface:
    """Cancel principal is separate from the monitor read token. No live call is configured."""

    def __init__(self, transport: OfflineTransport, repository: str):
        self.transport = transport
        self.repository = repository
        self.principal = "SEPARATE_FROM_MONITOR"

    def cancel(self, run_id: str) -> None:
        path = f"/repos/{self.repository}/actions/runs/{run_id}/cancel"
        _assert_allowed("POST", path, self.repository, "")
        response = self.transport.exchange("POST", path, {})
        if response.redirected:
            raise BudgetDeny("unsafe_redirect")


class GitHubDispatchAdapter(DispatchAdapter):
    def __init__(self, transport, *, repository: str, workflow: str, allowed_ref: str, expected_sha: str):
        self.transport = transport
        self.repository = repository
        self.workflow = workflow
        self.allowed_ref = allowed_ref
        self.expected_sha = expected_sha
        self.calls = 0
        self._posted: set[str] = set()

    def dispatch(self, request: DispatchRequest, binding: object | None = None) -> AdapterResult:
        self.calls += 1
        if request.dispatch_id in self._posted:
            raise BudgetDeny("automatic_retry_denied")
        if request.repository != self.repository:
            raise DispatchNotSent("repository_rejected")
        if request.workflow != self.workflow:
            raise DispatchNotSent("workflow_rejected")
        if not _TOKEN.fullmatch(request.campaign_id):
            raise DispatchNotSent("campaign_rejected")
        if not _TOKEN.fullmatch(request.dispatch_id):
            raise DispatchNotSent("identity_rejected")
        if request.expected_sha != self.expected_sha or not _SHA.fullmatch(request.expected_sha):
            raise DispatchNotSent("sha_unbound")
        if not isinstance(binding, TrustedRefBinding):
            raise DispatchNotSent("sha_unbound")
        if (
            binding.repository != self.repository
            or binding.workflow != self.workflow
            or binding.ref != self.allowed_ref
            or binding.object_sha != request.expected_sha
            or _SHA.fullmatch(_ref_name(binding.ref))
        ):
            raise DispatchNotSent("sha_unbound")
        commit_path = f"/repos/{self.repository}/git/commits/{request.expected_sha}"
        _assert_allowed("GET", commit_path, self.repository, self.workflow)
        try:
            commit = self.transport.exchange("GET", commit_path, None)
        except BudgetDeny as exc:
            if exc.code == "real_transport_disabled":
                raise DispatchNotSent("real_transport_disabled") from None
            raise DispatchNotSent("sha_unbound") from None
        except Exception as exc:
            raise DispatchNotSent(redact(str(exc))) from None
        if commit.redirected or commit.body is None or commit.body.get("sha") != request.expected_sha:
            raise DispatchNotSent("sha_unbound")
        path = f"/repos/{self.repository}/actions/workflows/{self.workflow}/dispatches"
        _assert_allowed("POST", path, self.repository, self.workflow)
        body = {
            "ref": _ref_name(binding.ref),
            "inputs": {
                "budget_dispatch_id": request.dispatch_id,
                "campaign_id": request.campaign_id,
                "expected_head_sha": request.expected_sha,
            },
        }
        self._posted.add(request.dispatch_id)
        try:
            response = self.transport.exchange("POST", path, body)
        except BudgetDeny as exc:
            if exc.code == "real_transport_disabled":
                raise DispatchNotSent("real_transport_disabled") from None
            raise BudgetDeny("dispatch_unknown") from None
        except Exception as exc:
            raise BudgetDeny(redact(str(exc))) from None
        if response.redirected:
            raise BudgetDeny("unsafe_redirect")
        if response.status == 204 or not response.body or response.body.get("run_id") in (None, ""):
            return AdapterResult(kind="unconfirmed")
        run_id = response.body.get("run_id")
        attempt = response.body.get("run_attempt")
        observed = response.body.get("head_sha")
        if not isinstance(run_id, str) or attempt != 1 or observed != request.expected_sha:
            return AdapterResult(kind="unconfirmed", run_id=run_id if isinstance(run_id, str) else None, run_attempt=attempt if isinstance(attempt, int) else None, observed_sha=observed if isinstance(observed, str) else None)
        return AdapterResult(kind="accepted", run_id=run_id, run_attempt=1, observed_sha=request.expected_sha)

    def perform(self, method: str, path: str) -> TransportResponse:
        _assert_allowed(method, path, self.repository, self.workflow)
        response = self.transport.exchange(method, path, None)
        if response.redirected:
            raise BudgetDeny("unsafe_redirect")
        return response


def correlate_run(
    runs: Sequence[Mapping[str, object]],
    *,
    dispatch_id: str,
    expected_sha: str,
    workflow: str,
    campaign_id: str | None = None,
    repository: str | None = None,
) -> str:
    """Match one Actions run by its visible run name or an explicit correlation field.

    The run name is the value GitHub returns as name/display_title for run-name.
    A match is not authorization. Zero or several matches stay unknown.
    created_at and list position are ignored.
    """
    matches: list[str] = []
    mismatch = ""
    for run in runs:
        kind, run_id, failure = _classify_correlation(
            run,
            dispatch_id=dispatch_id,
            expected_sha=expected_sha,
            workflow=workflow,
            campaign_id=campaign_id,
            repository=repository,
        )
        if kind == "match" and run_id:
            matches.append(run_id)
        elif kind == "mismatch" and not mismatch:
            mismatch = failure or "sha_mismatch"
    if mismatch:
        raise BudgetDeny(mismatch)
    if len(matches) != 1:
        raise BudgetDeny("dispatch_unknown")
    return matches[0]


def _classify_correlation(
    run: Mapping[str, object],
    *,
    dispatch_id: str,
    expected_sha: str,
    workflow: str,
    campaign_id: str | None,
    repository: str | None,
) -> tuple[str, str, str]:
    parsed = _correlation_tokens(run)
    if parsed is None or parsed[1] != dispatch_id:
        return "ignore", "", ""
    run_campaign, _dispatch, named_sha = parsed
    run_id = run.get("id")
    identity = str(run_id) if isinstance(run_id, (str, int)) and not isinstance(run_id, bool) else ""
    workflow_name = str(run.get("workflow") or run.get("path") or "")
    head_sha = run.get("head_sha")
    observed_sha = head_sha if isinstance(head_sha, str) else named_sha
    attempt = run.get("run_attempt")
    repo = run.get("repository")
    failure = ""
    if campaign_id is not None and run_campaign != campaign_id:
        failure = "campaign_rejected"
    elif named_sha not in (None, expected_sha) or observed_sha != expected_sha:
        failure = "sha_mismatch"
    elif workflow and not workflow_name.endswith(workflow):
        failure = "workflow_rejected"
    elif repository is not None and repo not in (None, repository):
        failure = "repository_rejected"
    elif attempt not in (None, 1):
        failure = "rerun_without_reservation"
    elif not identity:
        failure = "dispatch_unknown"
    if failure:
        return "mismatch", "", failure
    return "match", identity, ""


def _correlation_tokens(run: Mapping[str, object]) -> tuple[str | None, str, str | None] | None:
    for key in ("name", "display_title"):
        value = run.get(key)
        if isinstance(value, str):
            parsed = _RUN_NAME.fullmatch(value)
            if parsed:
                return parsed.group(1), parsed.group(2), parsed.group(3)
    field = run.get("budget_dispatch_id")
    if not isinstance(field, str) or not field:
        return None
    campaign = run.get("campaign_id")
    sha = run.get("head_sha")
    return (
        campaign if isinstance(campaign, str) else None,
        field,
        sha if isinstance(sha, str) else None,
    )


def _ref_name(ref: str) -> str:
    if ref.startswith("refs/heads/"):
        return ref[len("refs/heads/") :]
    if ref.startswith("refs/tags/"):
        return ref[len("refs/tags/") :]
    return ref


def _assert_allowed(method: str, path: str, repository: str, workflow: str) -> None:
    if method not in ("GET", "POST") or "://" in path or path.startswith("//") or ".." in path or not path.startswith("/"):
        raise BudgetDeny("endpoint_rejected")
    owner, separator, name = repository.partition("/")
    if separator != "/" or not owner or not name or "/" in name or "/" in workflow:
        raise BudgetDeny("endpoint_rejected")
    base, _, query = path.partition("?")
    parts = [part for part in base.split("/") if part]
    if len(parts) < 5 or parts[0] != "repos" or parts[1] != owner or parts[2] != name:
        raise BudgetDeny("endpoint_rejected")
    rest = parts[3:]
    if method == "POST" and workflow and rest == ["actions", "workflows", workflow, "dispatches"] and query == "":
        return
    if method == "GET" and rest[:2] == ["git", "commits"] and len(rest) == 3 and _SHA.fullmatch(rest[2]) and query == "":
        return
    if method == "GET" and rest[:2] == ["actions", "runs"] and len(rest) == 3 and query == "":
        return
    if method == "GET" and rest[:2] == ["actions", "runs"] and len(rest) == 4 and rest[3] == "jobs":
        if query == "" or _jobs_query(query):
            return
    if method == "POST" and rest[:2] == ["actions", "runs"] and len(rest) == 4 and rest[3] == "cancel" and query == "":
        return
    raise BudgetDeny("endpoint_rejected")


def _jobs_query(query: str) -> bool:
    if not query.startswith("per_page=100&page="):
        return False
    page = query[len("per_page=100&page=") :]
    return page.isdigit() and 1 <= int(page) <= 50

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
class TrustedDispatchPrincipal:
    """Dispatch identity from operator configuration, never from workflow inputs.

    GitHub `actor` is the account that started the workflow run. `triggering_actor`
    is the account that caused this attempt; a rerun can change it while `actor`
    stays unchanged. Acceptance uses these numeric ids. A login is not identity,
    and a matching actor id does not prove the budget gate.
    """

    actor_id: int
    repository_id: int
    workflow_id: int
    head_branch: str


@dataclass(frozen=True)
class RunInspection:
    run_id: str
    disposition: str
    code: str
    incident: bool


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

    def list_runs_path(self, page: int = 1) -> str:
        if not isinstance(page, int) or isinstance(page, bool) or page < 1 or page > 50:
            raise BudgetDeny("usage_incomplete")
        path = f"/repos/{self.repository}/actions/workflows/{self.workflow}/runs?per_page=100&page={page}"
        _reject_filtered_inventory(path)
        return path

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
            raise BudgetDeny("ref_changed")
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
    trusted: TrustedDispatchPrincipal | None = None,
) -> str:
    """Match one Actions run by its visible run name or an explicit correlation field.

    The run name is the value GitHub returns as name/display_title for run-name.
    A match is not authorization. Zero or several matches stay unknown.
    created_at and list position are ignored. When trusted is set, actor, event,
    and the other run fields are mandatory. A run that carries this dispatch id
    is never dropped because its actor or event fails.
    """
    decision, inspections = inspect_run_inventory(
        runs,
        dispatch_id=dispatch_id,
        expected_sha=expected_sha,
        workflow=workflow,
        campaign_id=campaign_id,
        repository=repository,
        trusted=trusted,
    )
    del inspections
    if decision.startswith("run:"):
        return decision[4:]
    raise BudgetDeny(decision)


def inspect_run_inventory(
    runs: Sequence[Mapping[str, object]],
    *,
    dispatch_id: str,
    expected_sha: str,
    workflow: str,
    campaign_id: str | None = None,
    repository: str | None = None,
    trusted: TrustedDispatchPrincipal | None = None,
) -> tuple[str, tuple[RunInspection, ...]]:
    """Inspect every supplied run. Actor and event filters must not remove one first."""
    inspections: list[RunInspection] = []
    matches: list[str] = []
    incident = ""
    for run in runs:
        inspection = _inspect_run(
            run,
            dispatch_id=dispatch_id,
            expected_sha=expected_sha,
            workflow=workflow,
            campaign_id=campaign_id,
            repository=repository,
            trusted=trusted,
        )
        inspections.append(inspection)
        if inspection.disposition == "match":
            matches.append(inspection.run_id)
        elif inspection.incident and not incident:
            incident = inspection.code
    if incident:
        return incident, tuple(inspections)
    if len(matches) != 1:
        return "dispatch_unknown", tuple(inspections)
    return "run:" + matches[0], tuple(inspections)


def _inspect_run(
    run: Mapping[str, object],
    *,
    dispatch_id: str,
    expected_sha: str,
    workflow: str,
    campaign_id: str | None,
    repository: str | None,
    trusted: TrustedDispatchPrincipal | None,
) -> RunInspection:
    run_id = run.get("id")
    identity = str(run_id) if isinstance(run_id, (str, int)) and not isinstance(run_id, bool) else ""
    kind, failure = _classify_correlation(
        run,
        dispatch_id=dispatch_id,
        expected_sha=expected_sha,
        workflow=workflow,
        campaign_id=campaign_id,
        repository=repository,
        trusted=trusted,
    )
    if kind == "match":
        return RunInspection(identity, "match", "match", False)
    if kind == "mismatch":
        return RunInspection(identity, "mismatch", failure or "sha_mismatch", True)
    external = _external_signal(run, workflow=workflow, trusted=trusted)
    if external:
        return RunInspection(identity, "external", external, True)
    return RunInspection(identity, "unrelated", "", False)


def _classify_correlation(
    run: Mapping[str, object],
    *,
    dispatch_id: str,
    expected_sha: str,
    workflow: str,
    campaign_id: str | None,
    repository: str | None,
    trusted: TrustedDispatchPrincipal | None,
) -> tuple[str, str]:
    parsed = _correlation_tokens(run)
    if parsed is None or parsed[1] != dispatch_id:
        return "ignore", ""
    run_campaign, _dispatch, named_sha = parsed
    run_id = run.get("id")
    identity = str(run_id) if isinstance(run_id, (str, int)) and not isinstance(run_id, bool) else ""
    workflow_name = str(run.get("workflow") or run.get("path") or "")
    head_sha = run.get("head_sha")
    observed_sha = head_sha if isinstance(head_sha, str) else named_sha
    attempt = run.get("run_attempt")
    repo_name = _repository_name(run)
    failure = ""
    if trusted is not None:
        failure = _trusted_failure(run, trusted, observed_sha, expected_sha, identity, attempt)
    if not failure and campaign_id is not None and run_campaign != campaign_id:
        failure = "campaign_rejected"
    elif not failure and (named_sha not in (None, expected_sha) or observed_sha != expected_sha):
        failure = "sha_mismatch"
    elif not failure and workflow and not workflow_name.endswith(workflow):
        failure = "workflow_rejected"
    elif not failure and repository is not None and repo_name not in (None, repository):
        failure = "repository_rejected"
    elif not failure and attempt not in (None, 1):
        failure = "rerun_without_reservation"
    elif not failure and not identity:
        failure = "dispatch_unknown"
    if failure:
        return "mismatch", failure
    return "match", ""


def _trusted_failure(
    run: Mapping[str, object],
    trusted: TrustedDispatchPrincipal,
    observed_sha: object,
    expected_sha: str,
    identity: str,
    attempt: object,
) -> str:
    actor_id = _user_id(run.get("actor"))
    triggering_id = _user_id(run.get("triggering_actor"))
    repository_id = _repository_id(run)
    workflow_id = _positive_int(run.get("workflow_id"))
    head_branch = run.get("head_branch")
    event = run.get("event")
    if (
        actor_id is None
        or triggering_id is None
        or repository_id is None
        or workflow_id is None
        or not isinstance(head_branch, str)
        or not head_branch
        or not isinstance(event, str)
        or not event
        or not isinstance(observed_sha, str)
        or not identity
        or not isinstance(attempt, int)
        or isinstance(attempt, bool)
    ):
        return "missing_identity"
    if actor_id != trusted.actor_id:
        return "actor_rejected"
    if triggering_id != trusted.actor_id:
        return "triggering_actor_rejected"
    if event != "workflow_dispatch":
        return "event_rejected"
    if attempt != 1:
        return "rerun_without_reservation"
    if observed_sha != expected_sha:
        return "sha_mismatch"
    if _branch_name(head_branch) != _branch_name(trusted.head_branch):
        return "ref_rejected"
    if repository_id != trusted.repository_id:
        return "repository_rejected"
    if workflow_id != trusted.workflow_id:
        return "workflow_rejected"
    return ""


def _external_signal(
    run: Mapping[str, object],
    *,
    workflow: str,
    trusted: TrustedDispatchPrincipal | None,
) -> str:
    if trusted is None:
        return ""
    workflow_name = str(run.get("workflow") or run.get("path") or "")
    workflow_id = _positive_int(run.get("workflow_id"))
    on_workflow = bool(workflow) and (workflow_name.endswith(workflow) or workflow_id == trusted.workflow_id)
    if not on_workflow:
        return ""
    event = run.get("event")
    attempt = run.get("run_attempt")
    actor_id = _user_id(run.get("actor"))
    if isinstance(attempt, int) and not isinstance(attempt, bool) and attempt != 1:
        return "rerun_without_reservation"
    if isinstance(event, str) and event != "workflow_dispatch":
        return "event_rejected"
    if event == "workflow_dispatch" and actor_id is None:
        return "external_dispatch"
    if actor_id is not None and actor_id != trusted.actor_id:
        return "external_dispatch"
    return ""


def _user_id(value: object) -> int | None:
    if not isinstance(value, Mapping):
        return None
    return _positive_int(value.get("id"))


def _repository_id(run: Mapping[str, object]) -> int | None:
    repository = run.get("repository")
    if isinstance(repository, Mapping):
        found = _positive_int(repository.get("id"))
        if found is not None:
            return found
    return _positive_int(run.get("repository_id"))


def _repository_name(run: Mapping[str, object]) -> str | None:
    repository = run.get("repository")
    if isinstance(repository, str):
        return repository
    if isinstance(repository, Mapping):
        full_name = repository.get("full_name")
        if isinstance(full_name, str):
            return full_name
    return None


def _positive_int(value: object) -> int | None:
    if isinstance(value, bool) or not isinstance(value, int) or value < 1:
        return None
    return value


def _branch_name(value: str) -> str:
    if value.startswith("refs/heads/"):
        return value[len("refs/heads/") :]
    return value


def _reject_filtered_inventory(path: str) -> None:
    if "actor=" in path or "event=" in path or "head_sha=" in path:
        raise BudgetDeny("inventory_filter_rejected")


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
    if method == "GET" and rest[:3] == ["actions", "workflows", workflow] and len(rest) == 4 and rest[3] == "runs":
        _reject_filtered_inventory(path)
        if _unfiltered_runs_query(query):
            return
        raise BudgetDeny("inventory_filter_rejected")
    if method == "GET" and rest[:2] == ["actions", "runs"] and len(rest) == 4 and rest[3] == "jobs":
        if query == "" or _jobs_query(query):
            return
    if method == "POST" and rest[:2] == ["actions", "runs"] and len(rest) == 4 and rest[3] == "cancel" and query == "":
        return
    raise BudgetDeny("endpoint_rejected")


def _unfiltered_runs_query(query: str) -> bool:
    if query == "per_page=100":
        return True
    if not query.startswith("per_page=100&page="):
        return False
    page = query[len("per_page=100&page=") :]
    return page.isdigit() and 1 <= int(page) <= 50


def _jobs_query(query: str) -> bool:
    if not query.startswith("per_page=100&page="):
        return False
    page = query[len("per_page=100&page=") :]
    return page.isdigit() and 1 <= int(page) <= 50

"""GitHub issue queue and atomic claim refs. Used only when the controller enables autopilot."""

from __future__ import annotations

import json
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from bintrans_autopilot.qa_gate import DiffFile
from bintrans_autopilot.queue import QueueIssue


class GithubError(Exception):
    def __init__(self, status: int, message: str):
        super().__init__(message)
        self.status = status


# One hundred files is one page. A pull request with more files is read in full.
# More than this many pages is incomplete data and fails closed.
MAX_PULL_FILE_PAGES = 5
PULL_FILE_PAGE_SIZE = 100


class GithubRest:
    def __init__(self, token: str, repository: str):
        if not token:
            raise GithubError(401, "GITHUB_TOKEN is required")
        owner, name = repository.split("/", 1)
        self._token = token
        self._owner = owner
        self._name = name
        self.truncated = False

    def _request(self, method: str, url: str, payload: dict | None = None):
        data = None if payload is None else json.dumps(payload).encode("utf-8")
        request = Request(
            url,
            data=data,
            method=method,
            headers={
                "Authorization": f"Bearer {self._token}",
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
                "Content-Type": "application/json",
            },
        )
        try:
            with urlopen(request, timeout=30) as response:
                raw = response.read().decode("utf-8")
                return response.status, json.loads(raw) if raw else {}
        except HTTPError as exc:
            raw = exc.read().decode("utf-8", errors="replace")
            raise GithubError(exc.code, raw[:300]) from exc
        except URLError as exc:
            raise GithubError(0, "github network error") from exc

    def list_issues(self):
        collected = []
        for page in range(1, 6):
            status, body = self._request(
                "GET",
                "https://api.github.com/repos/"
                f"{self._owner}/{self._name}/issues?state=all&labels=autopilot&per_page=100&page={page}",
            )
            if status >= 400:
                raise GithubError(status, "issue list failed")
            if not body:
                return collected
            for item in body:
                if "pull_request" in item:
                    continue
                collected.append(
                    QueueIssue(
                        number=item["number"],
                        body=item.get("body") or "",
                        labels=[label["name"] for label in item.get("labels") or []],
                    )
                )
            if len(body) < 100:
                return collected
        self.truncated = True
        return collected

    def update_issue(self, issue) -> None:
        self._request(
            "PATCH",
            f"https://api.github.com/repos/{self._owner}/{self._name}/issues/{issue.number}",
            {"body": issue.body, "labels": issue.labels},
        )

    def comment(self, number: int, body: str) -> None:
        self._request(
            "POST",
            f"https://api.github.com/repos/{self._owner}/{self._name}/issues/{number}/comments",
            {"body": body},
        )

    def create_ref(self, ref: str, sha: str) -> str:
        try:
            self._request(
                "POST",
                f"https://api.github.com/repos/{self._owner}/{self._name}/git/refs",
                {"ref": ref, "sha": sha},
            )
            return "created"
        except GithubError as exc:
            if exc.status == 422:
                return "exists"
            raise

    def get_ref_sha(self, ref: str) -> str:
        suffix = ref.removeprefix("refs/")
        _status, body = self._request(
            "GET",
            f"https://api.github.com/repos/{self._owner}/{self._name}/git/ref/{suffix}",
        )
        return body["object"]["sha"]

    def ref_exists(self, ref: str) -> bool:
        try:
            self.get_ref_sha(ref)
            return True
        except GithubError as exc:
            if exc.status == 404:
                return False
            raise

    def origin_main_sha(self) -> str:
        return self.get_ref_sha("refs/heads/main")

    def pull_files(self, pr_number: int):
        """Every changed file. A short or failed page is not treated as the whole PR."""
        collected: list[DiffFile] = []
        seen: set[str] = set()
        for page in range(1, MAX_PULL_FILE_PAGES + 1):
            try:
                _status, body = self._request(
                    "GET",
                    "https://api.github.com/repos/"
                    f"{self._owner}/{self._name}/pulls/{pr_number}/files"
                    f"?per_page={PULL_FILE_PAGE_SIZE}&page={page}",
                )
            except GithubError as exc:
                raise GithubError(exc.status, "pull files api error") from exc
            if not isinstance(body, list):
                raise GithubError(0, "pull files incomplete")
            for item in body:
                if not isinstance(item, dict):
                    raise GithubError(0, "pull files incomplete")
                name = str(item.get("filename") or "")
                if not name or name in seen:
                    raise GithubError(0, "pull files incomplete")
                seen.add(name)
                patch = item.get("patch")
                # A missing or non-text patch cannot be reviewed safely. Callers reject it.
                collected.append(DiffFile(path=name, patch=patch if isinstance(patch, str) else ""))
            if len(body) < PULL_FILE_PAGE_SIZE:
                return collected
        raise GithubError(0, "pull files truncated")

    def pull(self, pr_number: int) -> dict:
        _status, body = self._request(
            "GET",
            f"https://api.github.com/repos/{self._owner}/{self._name}/pulls/{pr_number}",
        )
        head = body.get("head") or {}
        base = body.get("base") or {}
        head_repo = head.get("repo") or {}
        base_repo = base.get("repo") or {}
        return {
            "number": body.get("number", pr_number),
            "base_sha": base.get("sha") or "",
            "head_sha": head.get("sha") or "",
            "head_branch": head.get("ref") or "",
            "head_repo": head_repo.get("full_name") or "",
            "repo": base_repo.get("full_name") or f"{self._owner}/{self._name}",
            "html_url": body.get("html_url") or "",
            "draft": body.get("draft", False),
            "node_id": body.get("node_id"),
        }

    def issue_events(self, number: int) -> list[dict]:
        collected = []
        for page in range(1, 6):
            _status, body = self._request(
                "GET",
                "https://api.github.com/repos/"
                f"{self._owner}/{self._name}/issues/{number}/events?per_page=100&page={page}",
            )
            if not isinstance(body, list):
                raise GithubError(0, "issue events malformed")
            for item in body:
                label = item.get("label") or {}
                actor = item.get("actor") or {}
                collected.append(
                    {
                        "event": item.get("event") or "",
                        "actor": actor.get("login") or "",
                        "label": label.get("name") or "",
                    }
                )
            if len(body) < 100:
                return collected
        raise GithubError(0, "issue events truncated")

    def issue_comments(self, number: int) -> list[dict]:
        collected = []
        for page in range(1, 6):
            _status, body = self._request(
                "GET",
                "https://api.github.com/repos/"
                f"{self._owner}/{self._name}/issues/{number}/comments?per_page=100&page={page}",
            )
            if not isinstance(body, list):
                raise GithubError(0, "issue comments malformed")
            for item in body:
                user = item.get("user") or {}
                collected.append({"user": user.get("login") or "", "body": item.get("body") or ""})
            if len(body) < 100:
                return collected
        raise GithubError(0, "issue comments truncated")

    def check_runs(self, sha: str) -> list[dict]:
        collected = []
        for page in range(1, 6):
            _status, body = self._request(
                "GET",
                "https://api.github.com/repos/"
                f"{self._owner}/{self._name}/commits/{sha}/check-runs?per_page=100&page={page}",
            )
            runs = body.get("check_runs") or []
            if not isinstance(runs, list):
                raise GithubError(0, "check runs malformed")
            for item in runs:
                collected.append(
                    {
                        "name": item.get("name") or "",
                        "head_sha": item.get("head_sha") or "",
                        "status": item.get("status") or "",
                        "conclusion": item.get("conclusion") or "",
                        "started_at": item.get("started_at") or "",
                        "completed_at": item.get("completed_at") or "",
                    }
                )
            if len(runs) < 100:
                return collected
        raise GithubError(0, "check runs truncated")

    def ensure_draft(self, pr_number: int) -> None:
        current = self.pull(pr_number)
        if current["draft"]:
            return
        self._request(
            "POST",
            "https://api.github.com/graphql",
            {
                "query": "mutation($id:ID!){convertPullRequestToDraft(input:{pullRequestId:$id}){pullRequest{isDraft}}}",
                "variables": {"id": current["node_id"]},
            },
        )

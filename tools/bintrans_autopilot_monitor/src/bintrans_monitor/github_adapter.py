"""Map one GitHub Actions run into the engine payload.

This module does not open a socket. A live GET stays outside the interpreter.
"""

from __future__ import annotations

from bintrans_monitor.engine import EXPECTED_REPOSITORY, MonitorError

PAGE_SIZE = 100


def _job_row(job: dict, run_sha: str, run_id: int) -> dict:
    foreign = job.get("run_id")
    if foreign is not None and int(foreign) != int(run_id):
        raise MonitorError("run_mixed")
    head = str(job.get("head_sha") or "")
    # GET /actions/runs/{run_id}/jobs has no log body. A log_excerpt on the
    # JSON is not a verified log, and this adapter does not download logs.
    return {
        "id": job.get("id"),
        "name": str(job.get("name") or ""),
        "conclusion": job.get("conclusion"),
        "status": job.get("status"),
        "head_sha": head,
        "run_attempt": job.get("run_attempt"),
        "log_excerpt": "",
        "log_excerpt_verified": False,
        "run_sha_expected": run_sha,
    }


def assemble(
    run: dict,
    job_pages: list[dict],
    repository: str,
    requested_run_id: int,
    page_size: int = PAGE_SIZE,
) -> dict:
    """Bind pages to one run id. A short final page ends the list."""
    if repository != EXPECTED_REPOSITORY:
        raise MonitorError("repository_rejected")
    try:
        run_id = int(run.get("id"))
    except (TypeError, ValueError) as exc:
        raise MonitorError("run_id_rejected") from exc
    if run_id != int(requested_run_id):
        raise MonitorError("run_mismatch")
    run_sha = str(run.get("head_sha") or "")
    jobs: list[dict] = []
    total = None
    incomplete = False
    if not job_pages:
        incomplete = True
    for index, page in enumerate(job_pages):
        chunk = page.get("jobs")
        if not isinstance(chunk, list):
            incomplete = True
            chunk = []
        if total is None and isinstance(page.get("total_count"), int):
            total = page["total_count"]
        for job in chunk:
            if not isinstance(job, dict):
                incomplete = True
                continue
            row = _job_row(job, run_sha, run_id)
            row.pop("run_sha_expected", None)
            if row["head_sha"] != run_sha:
                row["head_sha"] = row["head_sha"]
            jobs.append(row)
        last = index == len(job_pages) - 1
        if last and len(chunk) >= page_size and (total is None or len(jobs) < total):
            incomplete = True
        if not last and len(chunk) != page_size:
            incomplete = True
    if total is not None and len(jobs) != total:
        incomplete = True
    return {
        "repository": repository,
        "runs": [
            {
                "id": run_id,
                "name": str(run.get("name") or ""),
                "head_sha": run_sha,
                "event": str(run.get("event") or ""),
                "status": str(run.get("status") or ""),
                "conclusion": run.get("conclusion"),
                "run_attempt": run.get("run_attempt"),
                "jobs_incomplete": incomplete,
                "jobs": jobs,
            }
        ],
        "pulls": [],
    }

"""Parse a Task Contract from a GitHub issue body.

Only the known headings are trusted. Free text, including instructions to
ignore the contract, cannot change machine fields.
"""

from __future__ import annotations

import hashlib
import json
import re
from dataclasses import dataclass

HEADING = re.compile(r"^### ([A-Z0-9_]+)\s*$", re.MULTILINE)
TASK_ID_RE = re.compile(r"^[A-Z][A-Z0-9-]{2,40}$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
STATE_RE = re.compile(
    r"<!-- bintrans-autopilot-state\n(.*?)\n-->",
    re.DOTALL,
)

TRUSTED_FIELDS = (
    "TASK_ID",
    "MODULE",
    "OWNER_AGENT",
    "PRIORITY",
    "RISK_LEVEL",
    "BASE_SHA",
    "ALLOWED_PATHS",
    "FORBIDDEN_PATHS",
    "DEPENDENCIES",
    "ACCEPTANCE_CRITERIA",
    "REQUIRED_TESTS",
    "MAX_RETRIES",
    "STATUS",
    "OBJECTIVE",
)

ALLOWED_OWNERS = {
    "architect",
    "backend-engineer",
    "frontend-engineer",
    "devops-engineer",
    "documentation-engineer",
    "integrator",
}
RISK_LEVELS = {"LOW", "MEDIUM", "HIGH", "CRITICAL"}
PRIORITIES = {"P0", "P1", "P2", "P3"}

# Shell metacharacters are rejected by anchoring the whole command.
TEST_COMMANDS = re.compile(
    r"^(go test \./\.\.\.|go test \./[A-Za-z0-9_./-]*|npm test|npm run test|npm run typecheck|python -m unittest discover -s tools/bintrans_autopilot/tests)$"
)


@dataclass(frozen=True)
class TaskContract:
    task_id: str
    module: str
    owner_agent: str
    priority: str
    risk_level: str
    base_sha: str
    allowed_paths: tuple[str, ...]
    forbidden_paths: tuple[str, ...]
    dependencies: tuple[str, ...]
    acceptance_criteria: tuple[str, ...]
    required_tests: tuple[str, ...]
    max_retries: int
    status: str
    objective: str

    def canonical(self) -> dict[str, object]:
        return {
            "task_id": self.task_id,
            "module": self.module,
            "owner_agent": self.owner_agent,
            "priority": self.priority,
            "risk_level": self.risk_level,
            "base_sha": self.base_sha,
            "allowed_paths": list(self.allowed_paths),
            "forbidden_paths": list(self.forbidden_paths),
            "dependencies": list(self.dependencies),
            "acceptance_criteria": list(self.acceptance_criteria),
            "required_tests": list(self.required_tests),
            "max_retries": self.max_retries,
            "status": self.status,
            "objective": self.objective,
        }

    @property
    def digest(self) -> str:
        payload = json.dumps(self.canonical(), sort_keys=True, separators=(",", ":"))
        return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def parse_headings(body: str) -> dict[str, str]:
    """Return the first occurrence of each trusted heading. Later copies are ignored."""
    matches = list(HEADING.finditer(body))
    found: dict[str, str] = {}
    for index, match in enumerate(matches):
        name = match.group(1)
        if name not in TRUSTED_FIELDS or name in found:
            continue
        start = match.end()
        end = matches[index + 1].start() if index + 1 < len(matches) else len(body)
        found[name] = body[start:end].strip()
    return found


def _lines(value: str) -> tuple[str, ...]:
    items = []
    for raw in value.splitlines():
        line = raw.strip().lstrip("-").strip()
        if not line or line.lower() == "none":
            continue
        items.append(line)
    return tuple(items)


def parse_contract(body: str) -> tuple[TaskContract | None, str]:
    fields = parse_headings(body)
    missing = [name for name in TRUSTED_FIELDS if name != "OBJECTIVE" and not fields.get(name)]
    if missing:
        return None, "missing_fields:" + ",".join(missing)
    task_id = fields["TASK_ID"].splitlines()[0].strip()
    if not TASK_ID_RE.match(task_id):
        return None, "invalid_task_id"
    owner = fields["OWNER_AGENT"].splitlines()[0].strip()
    if owner not in ALLOWED_OWNERS:
        return None, "invalid_owner_agent"
    priority = fields["PRIORITY"].splitlines()[0].strip()
    if priority not in PRIORITIES:
        return None, "invalid_priority"
    risk = fields["RISK_LEVEL"].splitlines()[0].strip()
    if risk not in RISK_LEVELS:
        return None, "invalid_risk_level"
    base_sha = fields["BASE_SHA"].splitlines()[0].strip()
    if not SHA_RE.match(base_sha):
        return None, "invalid_base_sha"
    status = fields["STATUS"].splitlines()[0].strip()
    try:
        retries = int(fields["MAX_RETRIES"].splitlines()[0].strip())
    except ValueError:
        return None, "invalid_max_retries"
    if retries < 0:
        return None, "invalid_max_retries"
    allowed = _lines(fields["ALLOWED_PATHS"])
    forbidden = _lines(fields["FORBIDDEN_PATHS"])
    tests = _lines(fields["REQUIRED_TESTS"])
    criteria = _lines(fields["ACCEPTANCE_CRITERIA"])
    if not allowed or not forbidden or not tests or not criteria:
        return None, "empty_scope"
    for command in tests:
        if not TEST_COMMANDS.match(command):
            return None, "unapproved_test_command"
    dependencies = _lines(fields["DEPENDENCIES"])
    objective = fields.get("OBJECTIVE", "").strip()
    return (
        TaskContract(
            task_id=task_id,
            module=fields["MODULE"].splitlines()[0].strip(),
            owner_agent=owner,
            priority=priority,
            risk_level=risk,
            base_sha=base_sha,
            allowed_paths=allowed,
            forbidden_paths=forbidden,
            dependencies=tuple(dependencies),
            acceptance_criteria=criteria,
            required_tests=tests,
            max_retries=retries,
            status=status,
            objective=objective,
        ),
        "",
    )


def read_state(body: str) -> dict | None:
    match = STATE_RE.search(body)
    if not match:
        return None
    try:
        payload = json.loads(match.group(1))
    except json.JSONDecodeError:
        return None
    if not isinstance(payload, dict):
        return None
    return payload


def write_state(body: str, state: dict) -> str:
    block = "<!-- bintrans-autopilot-state\n" + json.dumps(state, indent=2, sort_keys=True) + "\n-->"
    if STATE_RE.search(body):
        return STATE_RE.sub(lambda _match: block, body, count=1)
    return body.rstrip() + "\n\n" + block + "\n"

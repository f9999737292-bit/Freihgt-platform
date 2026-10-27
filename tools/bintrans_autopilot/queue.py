"""Queue issue record shared by the dispatcher and the GitHub client."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class QueueIssue:
    number: int
    body: str
    labels: list[str] = field(default_factory=list)

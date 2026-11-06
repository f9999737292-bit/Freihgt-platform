"""Sealed mock Cursor Create transport for R6B."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from typing import Any, Literal

MockScenario = Literal[
    "create_success",
    "definite_reject",
    "timeout",
    "ambiguous",
    "conflict",
    "crash_before_response",
]


@dataclass(frozen=True)
class MockCreateResult:
    http_status: int
    error_code: str | None
    agent_created_claim: bool
    agent_id: str | None
    run_id: str | None
    proof_class: str
    proof_blob: dict[str, Any]


def mock_identity(tenant_id: str, launch_id: str) -> tuple[str, str]:
    """Stable mock agent and run ids derived from the trusted launch key.

    The historical transport constants are unique per database, so a second
    confirmed launch cannot reuse them. Payload-supplied ids are not used.
    """

    if not isinstance(tenant_id, str) or not isinstance(launch_id, str):
        raise ValueError("invalid_input")
    if not tenant_id or not launch_id:
        raise ValueError("invalid_input")
    return (
        _mock_token("bc", f"mock-agent:{tenant_id}:{launch_id}"),
        _mock_token("run", f"mock-run:{tenant_id}:{launch_id}"),
    )


def _mock_token(prefix: str, seed: str) -> str:
    digest = hashlib.sha256(seed.encode("utf-8")).hexdigest()
    return (
        f"{prefix}-{digest[0:8]}-{digest[8:12]}-{digest[12:16]}-"
        f"{digest[16:20]}-{digest[20:32]}"
    )


class MockCursorTransport:
    """Exact-type mock transport; subclasses forbidden (ADR RT-31)."""

    __slots__ = ("_scenario",)

    def __init_subclass__(cls, **kwargs: object) -> None:
        raise TypeError("MockCursorTransport must not be subclassed")

    def __init__(self, scenario: MockScenario = "create_success") -> None:
        object.__setattr__(self, "_scenario", scenario)

    @property
    def scenario(self) -> MockScenario:
        return self._scenario  # type: ignore[return-value]

    def create_agent(self, _body: dict[str, Any]) -> MockCreateResult:
        scenario = self.scenario
        if scenario == "create_success":
            tenant_id = _body.get("tenant_id")
            launch_id = _body.get("launch_id")
            if isinstance(tenant_id, str) and tenant_id and isinstance(launch_id, str) and launch_id:
                agent_id, run_id = mock_identity(tenant_id, launch_id)
            else:
                agent_id, run_id = (
                    "bc-33a0edbc-8fed-47fa-85b7-cc139e900375",
                    "run-a51658e1-665d-4f7c-9135-c0d720b2210b",
                )
            return MockCreateResult(
                200,
                None,
                True,
                agent_id,
                run_id,
                "MOCK_TRUSTED",
                {"http_status": 200, "agent_created_claim": True},
            )
        if scenario == "definite_reject":
            return MockCreateResult(
                400,
                "validation_error",
                False,
                None,
                None,
                "MOCK_TRUSTED",
                {
                    "http_status": 400,
                    "error_code": "validation_error",
                    "agent_created_claim": False,
                },
            )
        if scenario == "timeout":
            return MockCreateResult(
                0,
                "timeout",
                False,
                None,
                None,
                "MOCK_TRUSTED",
                {"http_status": 0, "error_code": "timeout", "agent_created_claim": False},
            )
        if scenario == "ambiguous":
            return MockCreateResult(
                500,
                "internal",
                False,
                None,
                None,
                "MOCK_TRUSTED",
                {"http_status": 500, "error_code": "internal", "agent_created_claim": False},
            )
        if scenario == "conflict":
            return MockCreateResult(
                409,
                "conflict",
                False,
                None,
                None,
                "MOCK_TRUSTED",
                {"http_status": 409, "error_code": "conflict", "agent_created_claim": False},
            )
        if scenario == "crash_before_response":
            raise RuntimeError("mock_transport_crash")
        raise ValueError(f"unknown_scenario:{scenario}")

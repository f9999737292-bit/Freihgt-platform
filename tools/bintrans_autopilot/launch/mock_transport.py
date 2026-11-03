"""Sealed mock Cursor Create transport for R6B."""

from __future__ import annotations

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
            return MockCreateResult(
                200,
                None,
                True,
                "bc-33a0edbc-8fed-47fa-85b7-cc139e900375",
                "run-a51658e1-665d-4f7c-9135-c0d720b2210b",
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

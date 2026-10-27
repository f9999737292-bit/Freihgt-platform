"""Cursor API client limited to documented Cloud Agents and spend endpoints.

Spec checked against https://cursor.com/docs/cloud-agent/api/endpoints
and https://cursor.com/docs/account/teams/admin-api on 2026-10-27.
No other paths are called.
"""

from __future__ import annotations

import base64
import json
from dataclasses import dataclass
from typing import Protocol
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

API_ORIGIN = "https://api.cursor.com"

# Documented run statuses. Execution status lives on the run, not only the agent.
ACTIVE_RUN_STATUSES = {"CREATING", "RUNNING", "WAITING_FOR_BACKGROUND_WORK"}
FAILED_RUN_STATUSES = {"ERROR", "EXPIRED"}
TERMINAL_RUN_STATUSES = FAILED_RUN_STATUSES | {"FINISHED", "CANCELLED"}


class CursorApiError(Exception):
    def __init__(self, status: int, code: str, message: str):
        super().__init__(message)
        self.status = status
        self.code = code
        self.message = message

    @property
    def retryable(self) -> bool:
        return self.status >= 500 or self.status == 429 or self.status == 0


@dataclass
class HttpResult:
    status: int
    body: dict


class Transport(Protocol):
    def request(self, method: str, url: str, headers: dict[str, str], payload: dict | None) -> HttpResult:
        ...


class UrllibTransport:
    """Real HTTP transport. Not used by unit tests and not called while autopilot is off."""

    def request(self, method: str, url: str, headers: dict[str, str], payload: dict | None) -> HttpResult:
        data = None if payload is None else json.dumps(payload).encode("utf-8")
        request = Request(url, data=data, headers=headers, method=method)
        try:
            with urlopen(request, timeout=30) as response:
                raw = response.read().decode("utf-8")
                parsed = json.loads(raw) if raw else {}
                if not isinstance(parsed, dict):
                    parsed = {"items": parsed}
                return HttpResult(status=response.status, body=parsed)
        except HTTPError as exc:
            raw = exc.read().decode("utf-8", errors="replace")
            try:
                parsed = json.loads(raw) if raw else {}
            except json.JSONDecodeError:
                parsed = {"message": raw[:300]}
            if not isinstance(parsed, dict):
                parsed = {"message": str(parsed)}
            raise CursorApiError(exc.code, str(parsed.get("code") or parsed.get("error") or ""), str(parsed.get("message") or exc.reason)) from exc
        except URLError as exc:
            raise CursorApiError(0, "network_error", "cursor API network error") from exc


def basic_header(api_key: str) -> str:
    token = base64.b64encode(f"{api_key}:".encode("utf-8")).decode("ascii")
    return f"Basic {token}"


def agent_endpoint(agent_id: str = "", suffix: str = "") -> str:
    if not agent_id:
        return "/v1/agents"
    path = f"/v1/agents/{agent_id}"
    if suffix:
        path += suffix
    return path


class CursorClient:
    def __init__(self, api_key: str, transport: Transport, admin_api_key: str = ""):
        if not api_key:
            raise CursorApiError(401, "missing_api_key", "CURSOR_API_KEY is required")
        self._api_key = api_key
        self._admin_api_key = admin_api_key
        self._transport = transport

    @property
    def has_admin_key(self) -> bool:
        return bool(self._admin_api_key)

    def _headers(self, key: str, json_body: bool) -> dict[str, str]:
        headers = {"Authorization": basic_header(key), "Accept": "application/json"}
        if json_body:
            headers["Content-Type"] = "application/json"
        return headers

    def _call(self, method: str, path: str, payload: dict | None = None, admin: bool = False) -> dict:
        key = self._admin_api_key if admin else self._api_key
        if not key:
            raise CursorApiError(401, "missing_api_key", "required Cursor API key is absent")
        if not path.startswith("/v1/") and path != "/teams/filtered-usage-events":
            raise CursorApiError(0, "undocumented_path", "refusing undocumented Cursor path")
        result = self._transport.request(
            method,
            API_ORIGIN + path,
            self._headers(key, payload is not None),
            payload,
        )
        if result.status >= 400:
            code = str(result.body.get("code") or result.body.get("error") or "")
            message = str(result.body.get("message") or code or result.status)
            raise CursorApiError(result.status, code, message)
        return result.body

    def create_agent(self, body: dict) -> dict:
        # POST /v1/agents — create the agent and enqueue the initial run.
        return self._call("POST", agent_endpoint(), body)

    def list_models(self) -> dict:
        # GET /v1/models — ids and params only. The payload has no price and no provider.
        return self._call("GET", "/v1/models")

    def get_agent(self, agent_id: str) -> dict:
        return self._call("GET", agent_endpoint(agent_id))

    def get_run(self, agent_id: str, run_id: str) -> dict:
        return self._call("GET", agent_endpoint(agent_id, f"/runs/{run_id}"))

    def follow_up(self, agent_id: str, text: str) -> dict:
        # POST /v1/agents/{id}/runs — new run on the same agent. No steer, no extra fields.
        # Model is not repeated here, so a failed run cannot move to a richer model.
        return self._call("POST", agent_endpoint(agent_id, "/runs"), {"prompt": {"text": text}})

    def cancel_run(self, agent_id: str, run_id: str) -> dict:
        return self._call("POST", agent_endpoint(agent_id, f"/runs/{run_id}/cancel"))

    def usage(self, agent_id: str) -> dict:
        return self._call("GET", agent_endpoint(agent_id, "/usage"))

    def cloud_charged_cents(self, start_ms: int, end_ms: int) -> float:
        """Sum chargedCents for Cursor-hosted cloud agent usage in the window.

        POST /teams/filtered-usage-events with the documented filters
        cloudAgentId=* and hostingType=CLOUD. Pages follow hasNextPage.
        The Admin API documents hourly aggregation, so this sum can be late.
        It is not a provider-side hard cap, and a personal plan has no team route.
        """
        if not self._admin_api_key:
            raise CursorApiError(401, "missing_admin_api_key", "CURSOR_ADMIN_API_KEY is required for spend checks")
        total = 0.0
        page = 1
        while page <= 20:
            body = self._call(
                "POST",
                "/teams/filtered-usage-events",
                {
                    "startDate": start_ms,
                    "endDate": end_ms,
                    "cloudAgentId": "*",
                    "hostingType": "CLOUD",
                    "page": page,
                    "pageSize": 100,
                },
                admin=True,
            )
            for event in body.get("usageEvents") or []:
                total += float(event.get("chargedCents") or 0)
            pagination = body.get("pagination") or {}
            if not pagination.get("hasNextPage"):
                return total
            page += 1
        raise CursorApiError(0, "spend_pages_truncated", "spend result was truncated")

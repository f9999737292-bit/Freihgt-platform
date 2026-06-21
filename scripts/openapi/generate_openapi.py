#!/usr/bin/env python3
"""Generate unified and per-service OpenAPI specs for Freight Platform."""

from __future__ import annotations

import textwrap
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OPENAPI_DIR = ROOT / "packages" / "openapi"

TAGS = [
    "Gateway",
    "Auth",
    "Users",
    "Roles",
    "Companies",
    "Memberships",
    "Locations",
    "Cargoes",
    "Transport Orders",
    "RFx",
    "Freight Requests",
    "Bids",
    "Shipments",
    "Drivers",
    "Vehicles",
    "Documents",
    "Signing",
    "Billing Registers",
    "Closing Documents",
]

COMMON_HEADER = """
      parameters:
        - $ref: '#/components/parameters/XRequestID'
        - $ref: '#/components/parameters/XTenantID'
        - $ref: '#/components/parameters/XCompanyID'
        - $ref: '#/components/parameters/XLocale'
        - $ref: '#/components/parameters/Authorization'
"""

SECURITY_BEARER = """
      security:
        - bearerAuth: []
"""

ERROR_RESPONSES = """
        '400':
          description: Validation error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: Not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '409':
          description: Conflict
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
"""

ENDPOINTS: list[tuple[str, str, str, str, bool, bool]] = [
    ("/health", "get", "Gateway health check", "Gateway", False, False),
    ("/ready", "get", "Readiness check for gateway and downstream services", "Gateway", False, False),
    ("/routes", "get", "List gateway route map", "Gateway", False, False),
    ("/openapi", "get", "List available OpenAPI documents", "Gateway", False, False),
    ("/openapi.yaml", "get", "Unified OpenAPI YAML document", "Gateway", False, False),
    ("/openapi.json", "get", "Unified OpenAPI JSON document", "Gateway", False, False),
    ("/docs", "get", "Swagger UI", "Gateway", False, False),
    ("/api/v1/auth/login", "post", "Login and obtain JWT access token", "Auth", False, False),
    ("/api/v1/auth/me", "get", "Get current authenticated user", "Auth", True, True),
    ("/api/v1/users", "post", "Create user", "Users", False, False),
    ("/api/v1/users", "get", "List users", "Users", True, True),
    ("/api/v1/users/{id}", "get", "Get user by ID", "Users", True, True),
    ("/api/v1/users/{id}", "patch", "Update user", "Users", True, True),
    ("/api/v1/users/{id}", "delete", "Delete user", "Users", True, True),
    ("/api/v1/users/{user_id}/companies", "get", "List companies for user", "Users", True, True),
    ("/api/v1/users/{user_id}/companies/{company_id}/roles", "post", "Assign role to user in company", "Roles", True, True),
    ("/api/v1/companies", "post", "Create company", "Companies", True, True),
    ("/api/v1/companies", "get", "List companies", "Companies", True, True),
    ("/api/v1/companies/{id}", "get", "Get company by ID", "Companies", True, True),
    ("/api/v1/companies/{id}", "patch", "Update company", "Companies", True, True),
    ("/api/v1/companies/{id}", "delete", "Delete company", "Companies", True, True),
    ("/api/v1/companies/{company_id}/members", "post", "Add company member", "Memberships", True, True),
    ("/api/v1/companies/{company_id}/members", "get", "List company members", "Memberships", True, True),
    ("/api/v1/locations", "post", "Create location", "Locations", True, True),
    ("/api/v1/locations", "get", "List locations", "Locations", True, True),
    ("/api/v1/locations/{id}", "get", "Get location by ID", "Locations", True, True),
    ("/api/v1/cargoes", "post", "Create cargo", "Cargoes", True, True),
    ("/api/v1/cargoes/{id}", "get", "Get cargo by ID", "Cargoes", True, True),
    ("/api/v1/transport-orders", "post", "Create transport order", "Transport Orders", True, True),
    ("/api/v1/transport-orders", "get", "List transport orders", "Transport Orders", True, True),
    ("/api/v1/transport-orders/{id}", "get", "Get transport order by ID", "Transport Orders", True, True),
    ("/api/v1/transport-orders/{id}", "patch", "Update transport order", "Transport Orders", True, True),
    ("/api/v1/transport-orders/{id}/submit", "post", "Submit transport order", "Transport Orders", True, True),
    ("/api/v1/transport-orders/{id}/cancel", "post", "Cancel transport order", "Transport Orders", True, True),
    ("/api/v1/rfx-events", "post", "Create RFx event", "RFx", True, True),
    ("/api/v1/rfx-events", "get", "List RFx events", "RFx", True, True),
    ("/api/v1/rfx-events/{id}", "get", "Get RFx event by ID", "RFx", True, True),
    ("/api/v1/rfx-events/{id}", "patch", "Update RFx event", "RFx", True, True),
    ("/api/v1/rfx-events/{id}/publish", "post", "Publish RFx event", "RFx", True, True),
    ("/api/v1/rfx-events/{id}/cancel", "post", "Cancel RFx event", "RFx", True, True),
    ("/api/v1/rfx-events/{id}/participants", "post", "Add RFx participant", "RFx", True, True),
    ("/api/v1/rfx-events/{id}/participants", "get", "List RFx participants", "RFx", True, True),
    ("/api/v1/freight-requests/from-transport-order", "post", "Create freight request from transport order", "Freight Requests", True, True),
    ("/api/v1/freight-requests", "get", "List freight requests", "Freight Requests", True, True),
    ("/api/v1/freight-requests/{id}", "get", "Get freight request by ID", "Freight Requests", True, True),
    ("/api/v1/freight-requests/{id}/publish", "post", "Publish freight request", "Freight Requests", True, True),
    ("/api/v1/freight-requests/{id}/bids", "post", "Create bid for freight request", "Bids", True, True),
    ("/api/v1/freight-requests/{id}/bids", "get", "List bids for freight request", "Freight Requests", True, True),
    ("/api/v1/bids/{id}/submit", "post", "Submit bid", "Bids", True, True),
    ("/api/v1/bids/{id}/accept", "post", "Accept bid", "Bids", True, True),
    ("/api/v1/shipments/from-transport-order", "post", "Create shipment from transport order", "Shipments", True, True),
    ("/api/v1/shipments/from-bid", "post", "Create shipment from accepted bid", "Shipments", True, True),
    ("/api/v1/shipments", "get", "List shipments", "Shipments", True, True),
    ("/api/v1/shipments/{id}", "get", "Get shipment by ID", "Shipments", True, True),
    ("/api/v1/shipments/{id}/assign-driver", "post", "Assign driver to shipment", "Shipments", True, True),
    ("/api/v1/shipments/{id}/assign-vehicle", "post", "Assign vehicle to shipment", "Shipments", True, True),
    ("/api/v1/shipments/{id}/accept", "post", "Accept shipment", "Shipments", True, True),
    ("/api/v1/shipments/{id}/status", "patch", "Update shipment status", "Shipments", True, True),
    ("/api/v1/shipments/{id}/cancel", "post", "Cancel shipment", "Shipments", True, True),
    ("/api/v1/drivers", "post", "Create driver", "Drivers", True, True),
    ("/api/v1/drivers", "get", "List drivers", "Drivers", True, True),
    ("/api/v1/drivers/{id}", "get", "Get driver by ID", "Drivers", True, True),
    ("/api/v1/vehicles", "post", "Create vehicle", "Vehicles", True, True),
    ("/api/v1/vehicles", "get", "List vehicles", "Vehicles", True, True),
    ("/api/v1/vehicles/{id}", "get", "Get vehicle by ID", "Vehicles", True, True),
    ("/api/v1/documents", "post", "Create document", "Documents", True, True),
    ("/api/v1/documents", "get", "List documents", "Documents", True, True),
    ("/api/v1/documents/{id}", "get", "Get document by ID", "Documents", True, True),
    ("/api/v1/documents/{id}/versions", "post", "Create document version", "Documents", True, True),
    ("/api/v1/documents/{id}/files", "post", "Add document file metadata", "Documents", True, True),
    ("/api/v1/documents/{id}/ready-for-signing", "post", "Move document to ready for signing", "Documents", True, True),
    ("/api/v1/documents/{id}/signing-sessions", "post", "Create signing session", "Signing", True, True),
    ("/api/v1/documents/{id}/cancel", "post", "Cancel document", "Documents", True, True),
    ("/api/v1/documents/{id}/archive", "post", "Archive document", "Documents", True, True),
    ("/api/v1/signing-sessions/{id}", "get", "Get signing session", "Signing", True, True),
    ("/api/v1/signing-sessions/{id}/signatures", "post", "Add mock signature", "Signing", True, True),
    ("/api/v1/billing-registers", "post", "Create billing register", "Billing Registers", True, True),
    ("/api/v1/billing-registers", "get", "List billing registers", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}", "get", "Get billing register by ID", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/items", "post", "Add shipment item to billing register", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/items", "get", "List billing register items", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{register_id}/items/{item_id}", "delete", "Delete billing register item", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/calculate", "post", "Calculate billing register totals", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/approve", "post", "Approve billing register", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/closing-document-package", "post", "Create closing document package", "Closing Documents", True, True),
    ("/api/v1/billing-registers/{id}/invoices", "post", "Create invoice", "Closing Documents", True, True),
    ("/api/v1/billing-registers/{id}/acts", "post", "Create act", "Closing Documents", True, True),
    ("/api/v1/billing-registers/{id}/vat-invoices", "post", "Create VAT invoice", "Closing Documents", True, True),
    ("/api/v1/billing-registers/{id}/upd", "post", "Create UPD document", "Closing Documents", True, True),
    ("/api/v1/billing-registers/{id}/mark-sent-to-edo", "post", "Mark billing register sent to EDO (mock)", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/mark-signed", "post", "Mark billing register signed (mock)", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/mark-paid", "post", "Mark billing register paid", "Billing Registers", True, True),
    ("/api/v1/billing-registers/{id}/close", "post", "Close billing register", "Billing Registers", True, True),
]

SERVICE_TAGS = {
    "identity-service.yaml": {"Auth", "Users", "Roles"},
    "company-service.yaml": {"Companies", "Memberships"},
    "transport-order-service.yaml": {"Locations", "Cargoes", "Transport Orders"},
    "rfx-service.yaml": {"RFx", "Freight Requests", "Bids"},
    "shipment-service.yaml": {"Shipments", "Drivers", "Vehicles"},
    "document-service.yaml": {"Documents", "Signing"},
    "billing-register-service.yaml": {"Billing Registers", "Closing Documents"},
}


def render_operation(method: str, summary: str, tag: str, with_headers: bool, secured: bool) -> str:
    body = ""
    if method in {"post", "patch", "put"}:
        body = """
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              additionalProperties: true
"""
    success_code = "201" if method == "post" and tag not in {"Gateway", "Auth"} else "200"
    return textwrap.dedent(
        f"""
    {method}:
      tags: [{tag}]
      summary: {summary}
      operationId: {method}_{path_to_id(summary)}
{COMMON_HEADER if with_headers else ''}{body}{SECURITY_BEARER if secured else ''}
      responses:
        '{success_code}':
          description: Successful response
          content:
            application/json:
              schema:
                type: object
                additionalProperties: true
{ERROR_RESPONSES}
"""
    )


def path_to_id(summary: str) -> str:
    return "".join(ch if ch.isalnum() else "_" for ch in summary.lower()).strip("_")


def render_paths(endpoints: list[tuple[str, str, str, str, bool, bool]]) -> str:
    grouped: dict[str, list[str]] = {}
    for path, method, summary, tag, with_headers, secured in endpoints:
        grouped.setdefault(path, []).append(render_operation(method, summary, tag, with_headers, secured))
    chunks = []
    for path, operations in grouped.items():
        chunks.append(f"  {path}:\n" + "".join(operations))
    return "\n".join(chunks)


def components_block() -> str:
    return textwrap.dedent(
        """
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
  parameters:
    XRequestID:
      name: X-Request-ID
      in: header
      required: false
      schema:
        type: string
      description: Correlation / request identifier
    XTenantID:
      name: X-Tenant-ID
      in: header
      required: false
      schema:
        type: string
        format: uuid
      description: Tenant context
    XCompanyID:
      name: X-Company-ID
      in: header
      required: false
      schema:
        type: string
        format: uuid
      description: Active company context
    XLocale:
      name: X-Locale
      in: header
      required: false
      schema:
        type: string
        example: ru-RU
    Authorization:
      name: Authorization
      in: header
      required: false
      schema:
        type: string
      description: Bearer JWT access token
  schemas:
    ErrorResponse:
      type: object
      required: [error]
      properties:
        error:
          type: object
          required: [code, message, details]
          properties:
            code:
              type: string
              enum:
                - VALIDATION_ERROR
                - UNAUTHORIZED
                - FORBIDDEN
                - NOT_FOUND
                - CONFLICT
                - SERVICE_UNAVAILABLE
                - INTERNAL_ERROR
                - ROUTE_NOT_FOUND
            message:
              type: string
            details:
              type: object
              additionalProperties: true
    HealthResponse:
      type: object
      properties:
        status:
          type: string
        service:
          type: string
    PaginatedResponse:
      type: object
      properties:
        items:
          type: array
          items:
            type: object
        total:
          type: integer
        limit:
          type: integer
        offset:
          type: integer
"""
    )


def build_spec(title_suffix: str, description: str, endpoints: list[tuple[str, str, str, str, bool, bool]]) -> str:
    tags_yaml = "\n".join(f"  - name: {tag}" for tag in TAGS)
    return textwrap.dedent(
        f"""
openapi: 3.0.3
info:
  title: Freight Platform API{title_suffix}
  version: 0.1.0
  description: |
    {description}
servers:
  - url: http://localhost:8080
    description: Local API Gateway
tags:
{tags_yaml}
paths:
{render_paths(endpoints)}
{components_block()}
"""
    ).strip() + "\n"


def main() -> None:
    OPENAPI_DIR.mkdir(parents=True, exist_ok=True)
    (OPENAPI_DIR / "schemas").mkdir(exist_ok=True)

    unified = build_spec("", "Unified HTTP API for the Freight Platform exposed via api-gateway.", ENDPOINTS)
    (OPENAPI_DIR / "openapi.yaml").write_text(unified, encoding="utf-8")

    for filename, tags in SERVICE_TAGS.items():
        service_endpoints = [item for item in ENDPOINTS if item[3] in tags]
        title = filename.replace("-service.yaml", "").replace(".yaml", "").replace("-", " ").title()
        spec = build_spec(f" - {title}", f"OpenAPI specification for {title}.", service_endpoints)
        (OPENAPI_DIR / filename).write_text(spec, encoding="utf-8")

    print(f"Generated OpenAPI specs in {OPENAPI_DIR}")


if __name__ == "__main__":
    main()

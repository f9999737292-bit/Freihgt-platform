#!/usr/bin/env python3
"""Generate unified and per-service OpenAPI specs for Freight Platform."""

from __future__ import annotations

import re
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
    "Payment Obligations",
    "Payments",
    "Transport Contracts",
    "Rate Cards",
    "Rate Simulation",
    "Freight Costs",
]

COMMON_HEADER = """      parameters:
        - $ref: '#/components/parameters/XRequestID'
        - $ref: '#/components/parameters/XTenantID'
        - $ref: '#/components/parameters/XCompanyID'
        - $ref: '#/components/parameters/XLocale'
        - $ref: '#/components/parameters/Authorization'
"""

SECURITY_BEARER = """      security:
        - bearerAuth: []
"""

ERROR_RESPONSES = """        '400':
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
        '403':
          description: Forbidden
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

# (path, method, summary, tag, with_headers, secured, profile)
ENDPOINTS: list[tuple[str, str, str, str, bool, bool, str | None]] = [
    ("/health", "get", "Gateway health check", "Gateway", False, False, None),
    ("/ready", "get", "Readiness check for gateway and downstream services", "Gateway", False, False, None),
    ("/routes", "get", "List gateway route map", "Gateway", False, False, None),
    ("/openapi", "get", "List available OpenAPI documents", "Gateway", False, False, None),
    ("/openapi.yaml", "get", "Unified OpenAPI YAML document", "Gateway", False, False, None),
    ("/openapi.json", "get", "Unified OpenAPI JSON document", "Gateway", False, False, None),
    ("/docs", "get", "Swagger UI", "Gateway", False, False, None),
    ("/api/v1/auth/login", "post", "Login and obtain JWT access token", "Auth", False, False, None),
    ("/api/v1/auth/me", "get", "Get current authenticated user", "Auth", True, True, None),
    ("/api/v1/users", "post", "Create user", "Users", False, False, None),
    ("/api/v1/users", "get", "List users", "Users", True, True, None),
    ("/api/v1/users/{id}", "get", "Get user by ID", "Users", True, True, None),
    ("/api/v1/users/{id}", "patch", "Update user", "Users", True, True, None),
    ("/api/v1/users/{id}", "delete", "Delete user", "Users", True, True, None),
    ("/api/v1/users/{user_id}/companies", "get", "List companies for user", "Users", True, True, None),
    ("/api/v1/users/{user_id}/companies/{company_id}/roles", "post", "Assign role to user in company", "Roles", True, True, None),
    ("/api/v1/companies", "post", "Create company", "Companies", True, True, None),
    ("/api/v1/companies", "get", "List companies", "Companies", True, True, None),
    ("/api/v1/companies/{id}", "get", "Get company by ID", "Companies", True, True, None),
    ("/api/v1/companies/{id}", "patch", "Update company", "Companies", True, True, None),
    ("/api/v1/companies/{id}", "delete", "Delete company", "Companies", True, True, None),
    ("/api/v1/companies/{company_id}/members", "post", "Add company member", "Memberships", True, True, None),
    ("/api/v1/companies/{company_id}/members", "get", "List company members", "Memberships", True, True, None),
    ("/api/v1/locations", "post", "Create location", "Locations", True, True, None),
    ("/api/v1/locations", "get", "List locations", "Locations", True, True, None),
    ("/api/v1/locations/{id}", "get", "Get location by ID", "Locations", True, True, None),
    ("/api/v1/cargoes", "post", "Create cargo", "Cargoes", True, True, None),
    ("/api/v1/cargoes/{id}", "get", "Get cargo by ID", "Cargoes", True, True, None),
    ("/api/v1/transport-orders", "post", "Create priced transport order", "Transport Orders", True, True, "priced_transport_order_create"),
    ("/api/v1/transport-orders", "get", "List transport orders", "Transport Orders", True, True, None),
    ("/api/v1/transport-orders/{id}", "get", "Get transport order by ID", "Transport Orders", True, True, None),
    ("/api/v1/transport-orders/{id}", "patch", "Update transport order", "Transport Orders", True, True, None),
    ("/api/v1/transport-orders/{id}/submit", "post", "Submit transport order", "Transport Orders", True, True, None),
    ("/api/v1/transport-orders/{id}/cancel", "post", "Cancel transport order", "Transport Orders", True, True, None),
    ("/api/v1/rfx-events", "post", "Create RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events", "get", "List RFx events", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}", "get", "Get RFx event by ID", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}", "patch", "Update RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/publish", "post", "Publish RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/cancel", "post", "Cancel RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/participants", "post", "Add RFx participant", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/participants", "get", "List RFx participants", "RFx", True, True, None),
    ("/api/v1/freight-requests/from-transport-order", "post", "Create freight request from transport order", "Freight Requests", True, True, None),
    ("/api/v1/freight-requests", "get", "List freight requests", "Freight Requests", True, True, None),
    ("/api/v1/freight-requests/{id}", "get", "Get freight request by ID", "Freight Requests", True, True, None),
    ("/api/v1/freight-requests/{id}/publish", "post", "Publish freight request", "Freight Requests", True, True, None),
    ("/api/v1/freight-requests/{id}/bids", "post", "Create bid for freight request", "Bids", True, True, None),
    ("/api/v1/freight-requests/{id}/bids", "get", "List bids for freight request", "Freight Requests", True, True, None),
    ("/api/v1/bids/{id}/submit", "post", "Submit bid", "Bids", True, True, None),
    ("/api/v1/bids/{id}/accept", "post", "Accept bid", "Bids", True, True, None),
    ("/api/v1/shipments/from-transport-order", "post", "Create shipment from transport order", "Shipments", True, True, None),
    ("/api/v1/shipments/from-bid", "post", "Create shipment from accepted bid", "Shipments", True, True, None),
    ("/api/v1/shipments", "get", "List shipments", "Shipments", True, True, None),
    ("/api/v1/shipments/{id}", "get", "Get shipment by ID", "Shipments", True, True, None),
    ("/api/v1/shipments/{id}/assign-driver", "post", "Assign driver to shipment", "Shipments", True, True, None),
    ("/api/v1/shipments/{id}/assign-vehicle", "post", "Assign vehicle to shipment", "Shipments", True, True, None),
    ("/api/v1/shipments/{id}/accept", "post", "Accept shipment", "Shipments", True, True, None),
    ("/api/v1/shipments/{id}/status", "patch", "Update shipment status", "Shipments", True, True, None),
    ("/api/v1/shipments/{id}/cancel", "post", "Cancel shipment", "Shipments", True, True, None),
    ("/api/v1/drivers", "post", "Create driver", "Drivers", True, True, None),
    ("/api/v1/drivers", "get", "List drivers", "Drivers", True, True, None),
    ("/api/v1/drivers/{id}", "get", "Get driver by ID", "Drivers", True, True, None),
    ("/api/v1/vehicles", "post", "Create vehicle", "Vehicles", True, True, None),
    ("/api/v1/vehicles", "get", "List vehicles", "Vehicles", True, True, None),
    ("/api/v1/vehicles/{id}", "get", "Get vehicle by ID", "Vehicles", True, True, None),
    ("/api/v1/documents", "post", "Create document", "Documents", True, True, None),
    ("/api/v1/documents", "get", "List documents", "Documents", True, True, None),
    ("/api/v1/documents/{id}", "get", "Get document by ID", "Documents", True, True, None),
    ("/api/v1/documents/{id}/versions", "post", "Create document version", "Documents", True, True, None),
    ("/api/v1/documents/{id}/files", "post", "Add document file metadata", "Documents", True, True, None),
    ("/api/v1/documents/{id}/ready-for-signing", "post", "Move document to ready for signing", "Documents", True, True, None),
    ("/api/v1/documents/{id}/signing-sessions", "post", "Create signing session", "Signing", True, True, None),
    ("/api/v1/documents/{id}/cancel", "post", "Cancel document", "Documents", True, True, None),
    ("/api/v1/documents/{id}/archive", "post", "Archive document", "Documents", True, True, None),
    ("/api/v1/signing-sessions/{id}", "get", "Get signing session", "Signing", True, True, None),
    ("/api/v1/signing-sessions/{id}/signatures", "post", "Add mock signature", "Signing", True, True, None),
    ("/api/v1/billing-registers", "post", "Create billing register", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers", "get", "List billing registers", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}", "get", "Get billing register by ID", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/items", "post", "Add shipment item to billing register", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/items", "get", "List billing register items", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{register_id}/items/{item_id}", "delete", "Delete billing register item", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/calculate", "post", "Calculate billing register totals", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/approve", "post", "Approve billing register", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/closing-document-package", "post", "Create closing document package", "Closing Documents", True, True, None),
    ("/api/v1/billing-registers/{id}/invoices", "post", "Create invoice", "Closing Documents", True, True, None),
    ("/api/v1/billing-registers/{id}/acts", "post", "Create act", "Closing Documents", True, True, None),
    ("/api/v1/billing-registers/{id}/vat-invoices", "post", "Create VAT invoice", "Closing Documents", True, True, None),
    ("/api/v1/billing-registers/{id}/upd", "post", "Create UPD document", "Closing Documents", True, True, None),
    ("/api/v1/billing-registers/{id}/mark-sent-to-edo", "post", "Mark billing register sent to EDO (mock)", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/mark-signed", "post", "Mark billing register signed (mock)", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/mark-paid", "post", "Mark billing register paid", "Billing Registers", True, True, None),
    ("/api/v1/billing-registers/{id}/close", "post", "Close billing register", "Billing Registers", True, True, None),
    ("/api/v1/payment-obligations", "get", "List payment obligations", "Payment Obligations", True, True, None),
    ("/api/v1/payment-obligations/{id}", "get", "Get payment obligation by ID", "Payment Obligations", True, True, None),
    ("/api/v1/payment-obligations/{id}/due-date", "patch", "Update payment obligation due date", "Payment Obligations", True, True, None),
    ("/api/v1/payments", "post", "Create manual payment", "Payments", True, True, None),
    ("/api/v1/payments", "get", "List payments", "Payments", True, True, "payment_list"),
    ("/api/v1/payments/{id}", "get", "Get payment by ID", "Payments", True, True, "payment_detail"),
    ("/api/v1/payments/{id}/allocations", "get", "List payment allocations", "Payments", True, True, "payment_allocations_list"),
    ("/api/v1/payments/{id}/allocations", "post", "Allocate payment to obligation", "Payments", True, True, None),
    ("/api/v1/payments/{id}/audit-events", "get", "List payment audit events", "Payments", True, True, "payment_audit_list"),
    ("/api/v1/payments/{id}/eligible-obligations", "get", "List eligible obligations for payment", "Payments", True, True, "payment_eligible_obligations_list"),
    ("/api/v1/payments/{id}/reconcile", "post", "Reconcile fully allocated payment", "Payments", True, True, "reconcile_payment"),
    ("/api/v1/payment-allocations/{id}/void", "post", "Void payment allocation", "Payments", True, True, "void_allocation"),
    ("/api/v1/payments/{id}/void", "post", "Void payment", "Payments", True, True, "void_payment"),
    ("/api/v1/transport-contracts", "get", "List transport contracts", "Transport Contracts", True, True, None),
    ("/api/v1/transport-contracts", "post", "Create transport contract", "Transport Contracts", True, True, "contract_create"),
    ("/api/v1/transport-contracts/{id}", "get", "Get transport contract", "Transport Contracts", True, True, None),
    ("/api/v1/transport-contracts/{id}", "patch", "Patch transport contract", "Transport Contracts", True, True, "contract_patch"),
    ("/api/v1/transport-contracts/{id}/activate", "post", "Activate transport contract", "Transport Contracts", True, True, "contract_lifecycle"),
    ("/api/v1/transport-contracts/{id}/suspend", "post", "Suspend transport contract", "Transport Contracts", True, True, "contract_lifecycle"),
    ("/api/v1/transport-contracts/{id}/reactivate", "post", "Reactivate transport contract", "Transport Contracts", True, True, "contract_lifecycle"),
    ("/api/v1/transport-contracts/{id}/terminate", "post", "Terminate transport contract", "Transport Contracts", True, True, "contract_terminate"),
    ("/api/v1/transport-contracts/{id}/cancel", "post", "Cancel transport contract", "Transport Contracts", True, True, "contract_lifecycle"),
    ("/api/v1/transport-contracts/{contractId}/rate-cards", "get", "List rate cards for contract", "Rate Cards", True, True, None),
    ("/api/v1/transport-contracts/{contractId}/rate-cards", "post", "Create rate card", "Rate Cards", True, True, "rate_card_create"),
    ("/api/v1/rate-cards/{id}", "get", "Get rate card", "Rate Cards", True, True, None),
    ("/api/v1/rate-cards/{id}/versions", "get", "List rate card versions", "Rate Cards", True, True, None),
    ("/api/v1/rate-cards/{id}/versions", "post", "Create draft rate card version", "Rate Cards", True, True, "rate_version_create"),
    ("/api/v1/rate-card-versions/{id}", "get", "Get rate card version", "Rate Cards", True, True, None),
    ("/api/v1/rate-card-versions/{id}", "patch", "Patch draft rate card version", "Rate Cards", True, True, "rate_version_patch"),
    ("/api/v1/rate-card-versions/{id}", "delete", "Discard draft rate card version", "Rate Cards", True, True, None),
    ("/api/v1/rate-card-versions/{id}/activate", "post", "Activate rate card version", "Rate Cards", True, True, "contract_lifecycle"),
    ("/api/v1/rate-card-versions/{id}/rate-lines", "get", "List rate lines", "Rate Cards", True, True, None),
    ("/api/v1/rate-card-versions/{id}/rate-lines", "post", "Create rate line", "Rate Cards", True, True, "rate_line_create"),
    ("/api/v1/rate-lines/{id}", "get", "Get rate line", "Rate Cards", True, True, None),
    ("/api/v1/rate-lines/{id}", "patch", "Patch draft rate line", "Rate Cards", True, True, "rate_line_patch"),
    ("/api/v1/rate-lines/{id}", "delete", "Delete draft rate line", "Rate Cards", True, True, None),
    ("/api/v1/rate-lines/{id}/components", "get", "List rate components", "Rate Cards", True, True, None),
    ("/api/v1/rate-lines/{id}/components", "post", "Create rate component", "Rate Cards", True, True, "rate_component_create"),
    ("/api/v1/rate-components/{id}", "patch", "Patch rate component", "Rate Cards", True, True, "rate_component_patch"),
    ("/api/v1/rate-components/{id}", "delete", "Delete rate component", "Rate Cards", True, True, None),
    ("/api/v1/rates/resolve", "post", "Simulate contract rate resolution", "Rate Simulation", True, True, "rate_resolve"),
    ("/api/v1/freight-costs", "get", "List freight cost workspace summaries", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/summary", "get", "Get freight cost aggregate KPIs", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/transport-orders/{transportOrderId}", "get", "Get freight cost order detail", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/transport-orders/{transportOrderId}/variance-detail", "get", "Get freight cost variance detail", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/accessorials/summary", "get", "Get freight cost accessorial spend summary", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/carriers/performance", "get", "Get freight cost carrier performance rollup", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/lanes/performance", "get", "Get freight cost lane performance rollup", "Freight Costs", True, True, None),
]

SERVICE_TAGS = {
    "identity-service.yaml": {"Auth", "Users", "Roles"},
    "company-service.yaml": {"Companies", "Memberships"},
    "transport-order-service.yaml": {"Locations", "Cargoes", "Transport Orders"},
    "rfx-service.yaml": {"RFx", "Freight Requests", "Bids"},
    "shipment-service.yaml": {"Shipments", "Drivers", "Vehicles"},
    "document-service.yaml": {"Documents", "Signing"},
    "billing-register-service.yaml": {"Billing Registers", "Closing Documents"},
    "payment-service.yaml": {"Payment Obligations", "Payments"},
    "contract-rate-service.yaml": {"Transport Contracts", "Rate Cards", "Rate Simulation"},
    "freight-cost-service.yaml": {"Freight Costs"},
}

VOID_DESCRIPTIONS = {
    "void_allocation": (
        "Voids an active allocation and recomputes payment/obligation balances from remaining active allocations.\n"
        "Append-only reversal. PAID obligation reversal is forbidden. RECONCILED payment mutation is forbidden.\n"
        "Repeat void is idempotent. Actor and tenant context are derived from verified request context."
    ),
    "void_payment": (
        "Voids a RECEIVED payment with zero active allocations.\n"
        "RECONCILED and partially or fully allocated payments cannot be voided.\n"
        "Repeat void is idempotent. Actor and tenant context are derived from verified request context."
    ),
}

RECONCILE_DESCRIPTIONS = {
    "reconcile_payment": (
        "Reconciles a payment after canonical financial confirmation.\n"
        "Requires FULLY_ALLOCATED status with active allocations recomputed from the database.\n"
        "Exact equality is required between payment amount, stored allocated amount, and active allocation sum.\n"
        "Repeat reconciliation is idempotent. Ordinary post-reconcile mutations are forbidden."
    ),
}

PRICED_TRANSPORT_ORDER_DESCRIPTION = (
    "Creates a transport order with mandatory rate resolution and immutable rate snapshot (v2.0C).\n"
    "Requires Idempotency-Key header. Unpriced legacy create is not permitted on this route."
)

PRICED_TRANSPORT_ORDER_REQUEST_BODY = """              type: object
              required:
                - order_number
                - shipper_company_id
                - consignee_company_id
                - origin_location_id
                - destination_location_id
                - cargo_id
                - pricing_context
              properties:
                order_number:
                  type: string
                shipper_company_id:
                  type: string
                  format: uuid
                consignee_company_id:
                  type: string
                  format: uuid
                origin_location_id:
                  type: string
                  format: uuid
                destination_location_id:
                  type: string
                  format: uuid
                cargo_id:
                  type: string
                  format: uuid
                transport_mode:
                  type: string
                  default: ROAD
                equipment_type:
                  type: string
                  description: Case-sensitive exact match after TrimSpace (no case coercion).
                pricing_context:
                  type: object
                  description: Explicit pricing source hints for rate resolution.
                  properties:
                    carrier_company_id:
                      type: string
                      format: uuid
                    award_link_id:
                      type: string
                      format: uuid
                    award_scope_event_id:
                      type: string
                      format: uuid
                    award_scope_lot_id:
                      type: string
                      format: uuid
                    bid_id:
                      type: string
                      format: uuid
                    manual_spot_amount:
                      type: string
                    manual_spot_currency:
                      type: string
                    pricing_source:
                      type: string
              additionalProperties: true"""

NO_REQUEST_BODY_PROFILES = frozenset({"reconcile_payment"})

CONTRACT_RATE_SCHEMA_REFS = {
    "contract_lifecycle": "EmptyLifecycleRequest",
}

CONTRACT_RATE_REQUEST_BODIES = {
    "contract_create": """              $ref: '#/components/schemas/PublicCreateTransportContractRequest'""",
    "contract_patch": """              $ref: '#/components/schemas/PublicPatchTransportContractRequest'""",
    "contract_terminate": """              $ref: '#/components/schemas/PublicTerminateTransportContractRequest'""",
    "rate_card_create": """              $ref: '#/components/schemas/PublicCreateRateCardRequest'""",
    "rate_version_create": """              $ref: '#/components/schemas/PublicCreateRateVersionRequest'""",
    "rate_version_patch": """              $ref: '#/components/schemas/PublicPatchRateVersionRequest'""",
    "rate_line_create": """              $ref: '#/components/schemas/PublicCreateRateLineRequest'""",
    "rate_line_patch": """              $ref: '#/components/schemas/PublicPatchRateLineRequest'""",
    "rate_component_create": """              $ref: '#/components/schemas/PublicCreateRateComponentRequest'""",
    "rate_component_patch": """              $ref: '#/components/schemas/PublicPatchRateComponentRequest'""",
    "rate_resolve": """              $ref: '#/components/schemas/PublicResolveRateRequest'""",
}

READ_RESPONSE_SCHEMAS = {
    "payment_list": "PaymentListResponse",
    "payment_detail": "PaymentRecord",
    "payment_allocations_list": "PaymentAllocationListResponse",
    "payment_audit_list": "PaymentAuditEventListResponse",
    "payment_eligible_obligations_list": "EligiblePaymentObligationListResponse",
}

# Public routes protected by paymentGuard / companycontext.Enforcer (router.go).
PAYMENT_GUARD_OPERATIONS = frozenset({
    ("get", "/api/v1/payment-obligations"),
    ("get", "/api/v1/payment-obligations/{id}"),
    ("patch", "/api/v1/payment-obligations/{id}/due-date"),
    ("post", "/api/v1/payments"),
    ("get", "/api/v1/payments"),
    ("get", "/api/v1/payments/{id}"),
    ("get", "/api/v1/payments/{id}/allocations"),
    ("get", "/api/v1/payments/{id}/audit-events"),
    ("get", "/api/v1/payments/{id}/eligible-obligations"),
    ("post", "/api/v1/payments/{id}/allocations"),
    ("post", "/api/v1/payments/{id}/reconcile"),
    ("post", "/api/v1/payment-allocations/{id}/void"),
    ("post", "/api/v1/payments/{id}/void"),
})

PAYMENT_DETAIL_LIST_QUERY_PROFILES = frozenset({
    "payment_allocations_list",
    "payment_audit_list",
    "payment_eligible_obligations_list",
})

COMPANY_ID_QUERY_DESCRIPTION = (
    "Active company context requested by the authenticated user. "
    "The gateway validates membership and derives trusted internal company/actor context."
)

HEADER_PARAMETER_REFS = [
    "        - $ref: '#/components/parameters/XRequestID'",
    "        - $ref: '#/components/parameters/XTenantID'",
    "        - $ref: '#/components/parameters/XCompanyID'",
    "        - $ref: '#/components/parameters/XLocale'",
    "        - $ref: '#/components/parameters/Authorization'",
]


def path_to_id(summary: str) -> str:
    return "".join(ch if ch.isalnum() else "_" for ch in summary.lower()).strip("_")


def _company_id_query_lines() -> list[str]:
    return [
        "        - name: company_id",
        "          in: query",
        "          required: true",
        "          schema:",
        "            type: string",
        "            format: uuid",
        f"          description: {COMPANY_ID_QUERY_DESCRIPTION}",
    ]


def _pagination_query_lines() -> list[str]:
    return [
        "        - name: limit",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: integer",
        "            default: 20",
        "            maximum: 100",
        "          description: Page size. Non-positive values are normalized to the default.",
        "        - name: offset",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: integer",
        "            minimum: 0",
        "            default: 0",
    ]


def _payment_list_filter_query_lines() -> list[str]:
    return [
        "        - name: status",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: string",
        "            enum:",
        "              - RECEIVED",
        "              - PARTIALLY_ALLOCATED",
        "              - FULLY_ALLOCATED",
        "              - RECONCILED",
        "              - VOIDED",
        "        - name: currency_code",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: string",
        "            minLength: 3",
        "            maxLength: 3",
        "        - name: from_date",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: string",
        "            format: date",
        "        - name: to_date",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: string",
        "            format: date",
        "        - name: q",
        "          in: query",
        "          required: false",
        "          schema:",
        "            type: string",
        "          description: Search payment_number, external_id, external_reference, or reference.",
    ]


def query_parameter_lines(method: str, path: str, profile: str | None) -> list[str]:
    lines: list[str] = []
    if (method, path) in PAYMENT_GUARD_OPERATIONS:
        lines.extend(_company_id_query_lines())
    if profile == "payment_list":
        lines.extend(_payment_list_filter_query_lines())
        lines.extend(_pagination_query_lines())
    elif profile in PAYMENT_DETAIL_LIST_QUERY_PROFILES:
        lines.extend(_pagination_query_lines())
    elif method == "get" and path == "/api/v1/freight-costs":
        lines.extend(_pagination_query_lines())
    return lines


def render_parameters(path: str, method: str, with_headers: bool, profile: str | None) -> str:
    query_lines = query_parameter_lines(method, path, profile)
    path_params = re.findall(r"\{([^}]+)\}", path)
    if not with_headers and not query_lines:
        return ""
    lines = ["      parameters:"]
    if with_headers:
        lines.extend(HEADER_PARAMETER_REFS)
    for param in path_params:
        lines.extend([
            f"        - name: {param}",
            "          in: path",
            "          required: true",
            "          schema:",
            "            type: string",
            "            format: uuid",
        ])
    lines.extend(query_lines)
    if profile == "priced_transport_order_create":
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            "          description: Client-supplied idempotency key for priced order creation (max 128 chars).",
            "          schema:",
            "            type: string",
            "            minLength: 1",
            "            maxLength: 128",
        ])
    return "\n".join(lines) + "\n"


def render_operation(
    path: str,
    method: str,
    summary: str,
    tag: str,
    with_headers: bool,
    secured: bool,
    profile: str | None = None,
) -> str:
    lines = [
        f"    {method}:",
        f"      tags: [{tag}]",
        f"      summary: {summary}",
        f"      operationId: {method}_{path_to_id(summary)}",
    ]

    if profile in VOID_DESCRIPTIONS:
        lines.append("      description: |")
        for desc_line in VOID_DESCRIPTIONS[profile].splitlines():
            lines.append(f"        {desc_line}")
    elif profile in RECONCILE_DESCRIPTIONS:
        lines.append("      description: |")
        for desc_line in RECONCILE_DESCRIPTIONS[profile].splitlines():
            lines.append(f"        {desc_line}")
    elif profile == "priced_transport_order_create":
        lines.append("      description: |")
        for desc_line in PRICED_TRANSPORT_ORDER_DESCRIPTION.splitlines():
            lines.append(f"        {desc_line}")

    parameters = render_parameters(path, method, with_headers, profile)
    if parameters:
        lines.append(parameters.rstrip("\n"))
    elif with_headers:
        lines.append(COMMON_HEADER.rstrip("\n"))

    if method in {"post", "patch", "put"} and profile not in NO_REQUEST_BODY_PROFILES:
        schema_ref = "#/components/schemas/VoidRequest" if profile in VOID_DESCRIPTIONS else None
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
            ]
        )
        if schema_ref:
            lines.append(f"              $ref: '{schema_ref}'")
        elif profile in CONTRACT_RATE_SCHEMA_REFS:
            lines.append(f"              $ref: '#/components/schemas/{CONTRACT_RATE_SCHEMA_REFS[profile]}'")
        elif profile in CONTRACT_RATE_REQUEST_BODIES:
            lines.append(CONTRACT_RATE_REQUEST_BODIES[profile])
        elif profile == "priced_transport_order_create":
            lines.append(PRICED_TRANSPORT_ORDER_REQUEST_BODY)
        else:
            lines.extend(
                [
                    "              type: object",
                    "              additionalProperties: true",
                ]
            )

    if secured:
        lines.append(SECURITY_BEARER.rstrip("\n"))

    success_code = "200" if profile in VOID_DESCRIPTIONS or profile in RECONCILE_DESCRIPTIONS else ("201" if method == "post" and tag not in {"Gateway", "Auth"} else "200")
    success_desc = "Successful response"
    if profile == "void_allocation":
        success_desc = "Allocation voided or idempotent success"
    elif profile == "void_payment":
        success_desc = "Payment voided or idempotent success"
    elif profile == "reconcile_payment":
        success_desc = "Payment reconciled or idempotent success"

    response_schema = READ_RESPONSE_SCHEMAS.get(profile or "")
    if response_schema:
        schema_lines = [
            "              schema:",
            f"                $ref: '#/components/schemas/{response_schema}'",
        ]
    else:
        schema_lines = [
            "              schema:",
            "                type: object",
            "                additionalProperties: true",
        ]

    lines.extend(
        [
            "      responses:",
            f"        '{success_code}':",
            f"          description: {success_desc}",
            "          content:",
            "            application/json:",
            *schema_lines,
            ERROR_RESPONSES.rstrip("\n"),
            "",
        ]
    )
    return "\n".join(lines)


def render_paths(endpoints: list[tuple[str, str, str, str, bool, bool, str | None]]) -> str:
    grouped: dict[str, list[str]] = {}
    for path, method, summary, tag, with_headers, secured, profile in endpoints:
        grouped.setdefault(path, []).append(
            render_operation(path, method, summary, tag, with_headers, secured, profile)
        )
    chunks = []
    for path, operations in grouped.items():
        chunks.append(f"  {path}:\n" + "\n".join(operations))
    return "\n".join(chunks)


def global_components_block() -> str:
    return """
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
    VoidRequest:
      type: object
      required: [reason]
      properties:
        reason:
          type: string
          minLength: 1
          maxLength: 255
          description: Required human-readable void reason
    EmptyLifecycleRequest:
      type: object
      additionalProperties: false
    PublicCreateTransportContractRequest:
      type: object
      required: [buyer_company_id, carrier_company_id, contract_number, name, valid_from, currency_code]
      additionalProperties: false
      properties:
        buyer_company_id: {type: string, format: uuid}
        carrier_company_id: {type: string, format: uuid}
        contract_number: {type: string}
        external_reference: {type: string}
        name: {type: string}
        description: {type: string}
        valid_from: {type: string, format: date}
        valid_to: {type: string, format: date, nullable: true}
        currency_code: {type: string}
    PublicPatchTransportContractRequest:
      type: object
      additionalProperties: false
      properties:
        name: {type: string}
        description: {type: string}
        external_reference: {type: string}
        valid_to: {type: string, format: date, nullable: true}
    PublicTerminateTransportContractRequest:
      type: object
      additionalProperties: false
      properties:
        termination_reason: {type: string}
    PublicCreateRateCardRequest:
      type: object
      required: [name]
      additionalProperties: false
      properties:
        name: {type: string}
        description: {type: string}
    PublicCreateRateVersionRequest:
      type: object
      required: [valid_from]
      additionalProperties: false
      properties:
        valid_from: {type: string, format: date}
        valid_to: {type: string, format: date, nullable: true}
    PublicPatchRateVersionRequest:
      type: object
      additionalProperties: false
      properties:
        valid_from: {type: string, format: date}
        valid_to: {type: string, format: date, nullable: true}
    PublicCreateRateLineRequest:
      type: object
      required: [origin_location_id, destination_location_id, equipment_type, transport_mode]
      additionalProperties: false
      properties:
        origin_location_id: {type: string, format: uuid}
        destination_location_id: {type: string, format: uuid}
        equipment_type: {type: string}
        transport_mode: {type: string}
    PublicPatchRateLineRequest:
      type: object
      additionalProperties: false
      properties:
        origin_location_id: {type: string, format: uuid}
        destination_location_id: {type: string, format: uuid}
        equipment_type: {type: string}
        transport_mode: {type: string}
    PublicCreateRateComponentRequest:
      type: object
      required: [component_type, calculation_method]
      additionalProperties: false
      properties:
        component_type: {type: string}
        calculation_method: {type: string}
        amount: {type: string}
        percent_value: {type: string}
        unit_code: {type: string}
    PublicPatchRateComponentRequest:
      type: object
      additionalProperties: false
      properties:
        amount: {type: string}
        percent_value: {type: string}
        unit_code: {type: string}
    PublicResolveRateRequest:
      type: object
      required: [buyer_company_id, carrier_company_id, origin_location_id, destination_location_id, equipment_type, transport_mode]
      additionalProperties: false
      properties:
        buyer_company_id: {type: string, format: uuid}
        carrier_company_id: {type: string, format: uuid}
        origin_location_id: {type: string, format: uuid}
        destination_location_id: {type: string, format: uuid}
        equipment_type: {type: string}
        transport_mode: {type: string}
        pricing_date: {type: string, format: date}
        currency_code: {type: string}
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


def payment_components_block() -> str:
    return """    PaymentRecord:
      type: object
      properties:
        id:
          type: string
          format: uuid
        tenant_id:
          type: string
          format: uuid
        payment_number:
          type: string
        payer_company_id:
          type: string
          format: uuid
        payee_company_id:
          type: string
          format: uuid
        amount:
          type: string
        currency_code:
          type: string
        payment_date:
          type: string
          format: date
        source:
          type: string
        status:
          type: string
          enum:
            - RECEIVED
            - PARTIALLY_ALLOCATED
            - FULLY_ALLOCATED
            - RECONCILED
            - VOIDED
        allocated_amount:
          type: string
        unallocated_amount:
          type: string
        version:
          type: integer
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        reference:
          type: string
        external_reference:
          type: string
        external_id:
          type: string
        created_by:
          type: string
          format: uuid
        voided_at:
          type: string
          format: date-time
        voided_by:
          type: string
          format: uuid
        void_reason:
          type: string
        reconciled_at:
          type: string
          format: date-time
        reconciled_by:
          type: string
          format: uuid
    PaymentListResponse:
      allOf:
        - $ref: '#/components/schemas/PaginatedResponse'
        - type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/PaymentRecord'
    PaymentAllocationReadRecord:
      type: object
      properties:
        id:
          type: string
          format: uuid
        tenant_id:
          type: string
          format: uuid
        payment_id:
          type: string
          format: uuid
        obligation_id:
          type: string
          format: uuid
        allocated_amount:
          type: string
        currency_code:
          type: string
        created_by:
          type: string
          format: uuid
        created_at:
          type: string
          format: date-time
        voided_at:
          type: string
          format: date-time
        voided_by:
          type: string
          format: uuid
        void_reason:
          type: string
        obligation_number:
          type: string
        obligation_status:
          type: string
        obligation_source_type:
          type: string
        obligation_source_id:
          type: string
          format: uuid
        obligation_outstanding_amount:
          type: string
    PaymentAllocationListResponse:
      allOf:
        - $ref: '#/components/schemas/PaginatedResponse'
        - type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/PaymentAllocationReadRecord'
    PaymentAuditEventRecord:
      type: object
      properties:
        id:
          type: string
        tenant_id:
          type: string
        entity_type:
          type: string
        entity_id:
          type: string
        event_type:
          type: string
        actor_user_id:
          type: string
        actor_company_id:
          type: string
        payload:
          type: object
          additionalProperties: true
        created_at:
          type: string
          format: date-time
    PaymentAuditEventListResponse:
      allOf:
        - $ref: '#/components/schemas/PaginatedResponse'
        - type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/PaymentAuditEventRecord'
    PaymentObligationRecord:
      type: object
      properties:
        id:
          type: string
          format: uuid
        tenant_id:
          type: string
          format: uuid
        obligation_number:
          type: string
        payer_company_id:
          type: string
          format: uuid
        payee_company_id:
          type: string
          format: uuid
        source_type:
          type: string
        source_id:
          type: string
          format: uuid
        currency_code:
          type: string
        original_amount:
          type: string
        paid_amount:
          type: string
        outstanding_amount:
          type: string
        status:
          type: string
        version:
          type: integer
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        due_date:
          type: string
          format: date
    EligiblePaymentObligationListResponse:
      allOf:
        - $ref: '#/components/schemas/PaginatedResponse'
        - type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/PaymentObligationRecord'
"""


def components_block(*, include_payment_components: bool = False) -> str:
    block = global_components_block()
    if include_payment_components:
        block = block.rstrip() + "\n" + payment_components_block()
    return block


def build_spec(
    title_suffix: str,
    description: str,
    endpoints: list[tuple[str, str, str, str, bool, bool, str | None]],
    *,
    include_payment_components: bool = False,
) -> str:
    tags_yaml = "\n".join(f"  - name: {tag}" for tag in TAGS)
    return (
        f"""openapi: 3.0.3
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
{components_block(include_payment_components=include_payment_components)}
"""
    ).strip() + "\n"


SERVICE_DISPLAY_NAMES = {
    "identity-service.yaml": "Identity Service",
    "company-service.yaml": "Company Service",
    "transport-order-service.yaml": "Transport Order Service",
    "rfx-service.yaml": "RFx Service",
    "shipment-service.yaml": "Shipment Service",
    "document-service.yaml": "Document Service",
    "billing-register-service.yaml": "Billing Register Service",
    "payment-service.yaml": "Payment Service",
    "contract-rate-service.yaml": "Contract Rate Service",
    "freight-cost-service.yaml": "Freight Cost Service",
}


def main() -> None:
    OPENAPI_DIR.mkdir(parents=True, exist_ok=True)
    (OPENAPI_DIR / "schemas").mkdir(exist_ok=True)

    unified = build_spec(
        "",
        "Unified HTTP API for the Freight Platform exposed via api-gateway.",
        ENDPOINTS,
        include_payment_components=True,
    )
    (OPENAPI_DIR / "openapi.yaml").write_text(unified, encoding="utf-8")

    for filename, tags in SERVICE_TAGS.items():
        service_endpoints = [item for item in ENDPOINTS if item[3] in tags]
        title = SERVICE_DISPLAY_NAMES.get(filename, filename.replace("-service.yaml", "").replace(".yaml", "").replace("-", " ").title())
        spec = build_spec(
            f" - {title}",
            f"OpenAPI specification for {title}.",
            service_endpoints,
            include_payment_components=(filename == "payment-service.yaml"),
        )
        (OPENAPI_DIR / filename).write_text(spec, encoding="utf-8")

    print(f"Generated OpenAPI specs in {OPENAPI_DIR}")


if __name__ == "__main__":
    main()

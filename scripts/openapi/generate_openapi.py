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
    "Network Optimizer",
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
    ("/api/v1/integrations/oauth/token", "post", "Obtain OAuth access token via client credentials", "Auth", False, False, "oauth_client_credentials"),
    ("/api/v1/integrations/erp/rfx/drafts/preview", "post", "Preview ERP buyer JSON create draft import", "RFx", True, True, "erp_import_preview_create_draft"),
    ("/api/v1/integrations/erp/rfx/drafts/commit", "post", "Commit ERP buyer JSON create draft import", "RFx", True, True, "erp_import_create_commit"),
    ("/api/v1/rfx-events/{id}/erp-import/preview", "post", "Preview ERP buyer JSON update draft import", "RFx", True, True, "erp_import_preview_update_draft"),
    ("/api/v1/rfx-events/{id}/erp-import/commit", "post", "Commit ERP buyer JSON update draft import", "RFx", True, True, "erp_import_update_commit"),
    ("/api/v1/integrations/erp/rfx-events/{id}", "get", "Get ERP RFx draft by internal ID", "RFx", True, True, "erp_get_rfx_event_by_id"),
    ("/api/v1/integrations/erp/rfx/by-external-id", "get", "Get ERP RFx draft by stable external identity", "RFx", True, True, "erp_get_rfx_by_external_id"),
    ("/api/v1/integrations/erp/analyses/{analysis_id}", "get", "Get ERP import analysis status", "RFx", True, True, "erp_get_import_analysis_status"),
    ("/api/v1/integrations/erp/capabilities", "get", "Get ERP integration capabilities", "RFx", True, True, "erp_get_integration_capabilities"),
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
    ("/api/v1/rfx-events/from-template", "post", "Clone RFx event from template", "RFx", True, True, "e5_clone"),
    ("/api/v1/rfx-events", "get", "List RFx events", "RFx", True, True, None),
    ("/api/v1/carrier/rfx-events/{id}", "get", "Get invited RFx event for the authenticated carrier company", "RFx", True, True, "carrier_invited_event_get"),
    ("/api/v1/rfx-events/{id}", "get", "Get RFx event by ID (optional clone provenance and stored creation_channel for buyers)", "RFx", True, True, "rfx_event_detail"),
    ("/api/v1/rfx-events/{id}", "patch", "Update RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/publish", "post", "Publish RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/cancel", "post", "Cancel RFx event", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/participants", "post", "Add RFx participant", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/participants", "get", "List RFx participants", "RFx", True, True, None),
    ("/api/v1/rfx-events/{id}/studio", "get", "Get RFx studio workspace", "RFx", True, True, "q_studio_get"),
    ("/api/v1/rfx-events/{id}/questionnaire", "get", "Get RFx questionnaire definition", "RFx", True, True, "q_questionnaire_get"),
    ("/api/v1/rfx-events/{id}/save-draft", "post", "Save RFx questionnaire draft", "RFx", True, True, "q_save_draft"),
    ("/api/v1/rfx-events/{id}/validate-publish", "post", "Validate RFx publish readiness", "RFx", True, True, "q_validate_publish"),
    ("/api/v1/rfx-events/{id}/questionnaire/publish", "post", "Publish current RFx questionnaire draft", "RFx", True, True, "vl_publish"),
    ("/api/v1/rfx-events/{id}/change-impact/preview", "post", "Preview RFx questionnaire change impact", "RFx", True, True, "vl_change_impact_preview"),
    ("/api/v1/rfx-events/{id}/versions", "get", "List RFx questionnaire versions", "RFx", True, True, "vl_list"),
    ("/api/v1/rfx-events/{id}/versions/fork-draft", "post", "Fork draft from published questionnaire", "RFx", True, True, "vl_fork_draft"),
    ("/api/v1/rfx-events/{id}/versions/compare", "post", "Compare RFx questionnaire versions", "RFx", True, True, "vl_compare"),
    ("/api/v1/rfx-events/{id}/versions/{version_id}/restore-draft", "post", "Restore RFx questionnaire version as new draft", "RFx", True, True, "vl_restore_draft"),
    ("/api/v1/rfx-events/{id}/versions/{version_id}", "get", "Get RFx questionnaire version detail", "RFx", True, True, "vl_detail"),
    ("/api/v1/rfx-templates", "get", "List RFx templates", "RFx", True, True, "tl_list"),
    ("/api/v1/rfx-templates", "post", "Create RFx template", "RFx", True, True, "tl_create"),
    ("/api/v1/rfx-templates/{id}", "get", "Get RFx template detail", "RFx", True, True, "tl_detail"),
    ("/api/v1/rfx-templates/{id}", "patch", "Update RFx template metadata", "RFx", True, True, "tl_update"),
    ("/api/v1/rfx-templates/{id}", "delete", "Soft-delete draft-only RFx template", "RFx", True, True, "tl_delete"),
    ("/api/v1/rfx-templates/{id}/archive", "post", "Archive RFx template", "RFx", True, True, "tl_archive"),
    ("/api/v1/rfx-templates/{id}/versions/publish", "post", "Publish RFx template version", "RFx", True, True, "tl_publish"),
    ("/api/v1/rfx-templates/{id}/versions/fork-draft", "post", "Fork RFx template draft from published", "RFx", True, True, "tl_fork_draft"),
    ("/api/v1/rfx-templates/{id}/questionnaire", "get", "Get RFx template questionnaire", "RFx", True, True, "tl_questionnaire_get"),
    ("/api/v1/rfx-templates/{id}/versions/{version_id}/questionnaire", "get", "Get read-only RFx template version questionnaire graph", "RFx", True, True, "tl_version_questionnaire_get"),
    ("/api/v1/rfx-templates/{id}/sections", "post", "Create RFx template section", "RFx", True, True, "tl_section_create"),
    ("/api/v1/rfx-templates/{id}/sections/{section_id}", "patch", "Update RFx template section", "RFx", True, True, "tl_section_update"),
    ("/api/v1/rfx-templates/{id}/sections/{section_id}", "delete", "Delete RFx template section", "RFx", True, True, "tl_section_delete"),
    ("/api/v1/rfx-templates/{id}/sections/reorder", "post", "Reorder RFx template sections", "RFx", True, True, "tl_section_reorder"),
    ("/api/v1/rfx-templates/{id}/questions", "post", "Create RFx template question", "RFx", True, True, "tl_question_create"),
    ("/api/v1/rfx-templates/{id}/questions/{question_id}", "patch", "Update RFx template question", "RFx", True, True, "tl_question_update"),
    ("/api/v1/rfx-templates/{id}/questions/{question_id}", "delete", "Delete RFx template question", "RFx", True, True, "tl_question_delete"),
    ("/api/v1/rfx-templates/{id}/questions/{question_id}/duplicate", "post", "Duplicate RFx template question", "RFx", True, True, "tl_question_duplicate"),
    ("/api/v1/rfx-templates/{id}/questions/reorder", "post", "Reorder RFx template questions", "RFx", True, True, "tl_question_reorder"),
    ("/api/v1/rfx-templates/{id}/questions/{question_id}/options", "post", "Create RFx template question option", "RFx", True, True, "tl_option_create"),
    ("/api/v1/rfx-templates/{id}/questions/{question_id}/options/{option_id}", "patch", "Update RFx template question option", "RFx", True, True, "tl_option_update"),
    ("/api/v1/rfx-templates/{id}/questions/{question_id}/options/{option_id}", "delete", "Delete RFx template question option", "RFx", True, True, "tl_option_delete"),
    ("/api/v1/rfx-templates/{id}/rules", "post", "Create RFx template rule", "RFx", True, True, "tl_rule_create"),
    ("/api/v1/rfx-templates/{id}/rules/{rule_id}", "patch", "Update RFx template rule", "RFx", True, True, "tl_rule_update"),
    ("/api/v1/rfx-templates/{id}/rules/{rule_id}", "delete", "Delete RFx template rule", "RFx", True, True, "tl_rule_delete"),
    ("/api/v1/rfx-events/{id}/carrier-response", "get", "Get carrier questionnaire response workspace", "RFx", True, True, "cr_workspace_get"),
    ("/api/v1/rfx-events/{id}/carrier-response/start", "post", "Start or resume carrier questionnaire response", "RFx", True, True, "cr_start"),
    ("/api/v1/rfx-events/{id}/carrier-response/answers", "patch", "Atomic batch autosave of carrier answers", "RFx", True, True, "cr_answers_patch"),
    ("/api/v1/rfx-events/{id}/carrier-response/validate", "post", "Pre-submit validation of carrier response", "RFx", True, True, "cr_validate"),
    ("/api/v1/rfx-events/{id}/late-submission-requests", "post", "Create carrier late submission request after response deadline", "RFx", True, True, "ls_create"),
    ("/api/v1/rfx-events/{id}/late-submission-requests/mine", "get", "List own late submission requests for carrier", "RFx", True, True, "ls_list_mine"),
    ("/api/v1/rfx-events/{id}/late-submission-requests", "get", "List late submission requests for buyer review queue", "RFx", True, True, "ls_list_buyer"),
    ("/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/approve", "post", "Approve carrier late submission request", "RFx", True, True, "ls_approve"),
    ("/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/reject", "post", "Reject carrier late submission request", "RFx", True, True, "ls_reject"),
    ("/api/v1/rfx-events/{id}/xlsx-export", "get", "Export buyer draft RFx event as XLSX workbook", "RFx", True, True, "xlsx_export_buyer_draft"),
    ("/api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-export", "get", "Export carrier RFx response as XLSX workbook", "RFx", True, True, "xlsx_export_carrier_response"),
    ("/api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-import/preview", "post", "Preview carrier RFx response XLSX import", "RFx", True, True, "xlsx_import_preview_carrier_response"),
    ("/api/v1/rfx-events/{id}/xlsx-import/preview", "post", "Preview buyer draft RFx event XLSX import", "RFx", True, True, "xlsx_import_preview_buyer_draft"),
    ("/api/v1/rfx-events/{id}/xlsx-import/commit", "post", "Commit buyer draft RFx event XLSX import", "RFx", True, True, "xlsx_import_commit_buyer_draft"),
    ("/api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-import/commit", "post", "Commit carrier RFx response XLSX import", "RFx", True, True, "xlsx_import_commit_carrier_response"),
    ("/api/v1/rfx-events/xlsx-create/template", "get", "Download buyer new RFx event XLSX create template", "RFx", True, True, "xlsx_create_template_buyer_draft"),
    ("/api/v1/rfx-events/xlsx-create/preview", "post", "Preview buyer new RFx event XLSX create", "RFx", True, True, "xlsx_create_preview_buyer_draft"),
    ("/api/v1/rfx-events/xlsx-create/commit", "post", "Commit buyer new RFx event XLSX create", "RFx", True, True, "xlsx_create_commit_buyer_draft"),
    ("/api/v1/rfx-events/{id}/carrier-response/submit", "post", "Submit carrier questionnaire response", "RFx", True, True, "cr_submit"),
    ("/api/v1/rfx-events/{id}/carrier-response/summary", "get", "Carrier response completion summary", "RFx", True, True, "cr_summary_get"),
    ("/api/v1/rfx-events/{id}/score-model", "get", "Get RFx score model", "RFx", True, True, "score_model_get"),
    ("/api/v1/rfx-events/{id}/score-model", "put", "Upsert draft RFx score model", "RFx", True, True, "score_model_put"),
    ("/api/v1/rfx-events/{id}/score-model/validate", "post", "Validate RFx score model readiness", "RFx", True, True, "score_model_validate"),
    ("/api/v1/rfx-events/{id}/score-model/publish", "post", "Publish RFx score model", "RFx", True, True, "score_model_publish"),
    ("/api/v1/rfx-events/{id}/responses/{response_id}/score", "get", "Get v3 response score result", "RFx", True, True, "score_result_get"),
    ("/api/v1/rfx-events/{id}/responses/{response_id}/score/explanation", "get", "Get v3 response score explanation", "RFx", True, True, "score_explanation_get"),
    ("/api/v1/rfx-events/{id}/sections", "post", "Create RFx questionnaire section", "RFx", True, True, "q_section_create"),
    ("/api/v1/rfx-events/{id}/sections/{section_id}", "patch", "Update RFx questionnaire section", "RFx", True, True, "q_section_update"),
    ("/api/v1/rfx-events/{id}/sections/{section_id}", "delete", "Delete RFx questionnaire section", "RFx", True, True, "q_section_delete"),
    ("/api/v1/rfx-events/{id}/sections/reorder", "post", "Reorder RFx questionnaire sections", "RFx", True, True, "q_section_reorder"),
    ("/api/v1/rfx-events/{id}/questions", "post", "Create RFx questionnaire question", "RFx", True, True, "q_question_create"),
    ("/api/v1/rfx-events/{id}/questions/{question_id}", "patch", "Update RFx questionnaire question", "RFx", True, True, "q_question_update"),
    ("/api/v1/rfx-events/{id}/questions/{question_id}", "delete", "Delete RFx questionnaire question", "RFx", True, True, "q_question_delete"),
    ("/api/v1/rfx-events/{id}/questions/{question_id}/duplicate", "post", "Duplicate RFx questionnaire question", "RFx", True, True, "q_question_duplicate"),
    ("/api/v1/rfx-events/{id}/questions/reorder", "post", "Reorder RFx questionnaire questions", "RFx", True, True, "q_question_reorder"),
    ("/api/v1/rfx-events/{id}/questions/{question_id}/options", "post", "Create RFx question option", "RFx", True, True, "q_option_create"),
    ("/api/v1/rfx-events/{id}/questions/{question_id}/options/{option_id}", "patch", "Update RFx question option", "RFx", True, True, "q_option_update"),
    ("/api/v1/rfx-events/{id}/questions/{question_id}/options/{option_id}", "delete", "Delete RFx question option", "RFx", True, True, "q_option_delete"),
    ("/api/v1/rfx-events/{id}/rules", "post", "Create RFx questionnaire rule", "RFx", True, True, "q_rule_create"),
    ("/api/v1/rfx-events/{id}/rules/{rule_id}", "patch", "Update RFx questionnaire rule", "RFx", True, True, "q_rule_update"),
    ("/api/v1/rfx-events/{id}/rules/{rule_id}", "delete", "Delete RFx questionnaire rule", "RFx", True, True, "q_rule_delete"),
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
    ("/api/v1/freight-costs/analytics/overview", "get", "Get freight cost intelligence overview", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/analytics/lanes", "get", "List lane analytics with tenant benchmarks", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/analytics/carriers", "get", "List carrier analytics with lane-normalized comparison", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/analytics/accessorials", "get", "List accessorial analytics breakdown", "Freight Costs", True, True, None),
    ("/api/v1/freight-costs/opportunities", "get", "List explainable savings opportunities", "Freight Costs", True, True, None),
    ("/api/v1/network/load-opportunities", "post", "Create a load opportunity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/load-opportunities", "get", "List own load opportunities", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/load-opportunities/{id}", "get", "Get own load opportunity", "Network Optimizer", True, True, None),
    ("/api/v1/network/load-opportunities/{id}", "patch", "Update own load opportunity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/load-opportunities/{id}/publish", "post", "Publish own load opportunity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/load-opportunities/{id}/withdraw", "post", "Withdraw own load opportunity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/capacities", "post", "Create manual capacity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/capacities", "get", "List own capacities", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/capacities/{id}", "get", "Get own capacity", "Network Optimizer", True, True, None),
    ("/api/v1/network/capacities/{id}", "patch", "Update own capacity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/capacities/{id}/withdraw", "post", "Withdraw own capacity", "Network Optimizer", True, True, "bno_foundation_mutation"),
    ("/api/v1/network/shipments/{shipmentId}/predicted-capacity", "post", "Generate predicted capacity", "Network Optimizer", True, True, "bno_prediction"),
    ("/api/v1/network/predicted-capacities", "get", "List own predicted capacities", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/predicted-capacities/{id}", "get", "Get own predicted capacity", "Network Optimizer", True, True, "bno_prediction"),
    ("/api/v1/network/predicted-capacities/{id}/refresh", "post", "Refresh predicted capacity", "Network Optimizer", True, True, "bno_prediction"),
    ("/api/v1/network/predicted-capacities/{id}/activate", "post", "Activate predicted capacity", "Network Optimizer", True, True, "bno_prediction"),
    ("/api/v1/network/next-load/search", "post", "Search next-load candidates for an available capacity", "Network Optimizer", True, True, "bno_next_load"),
    ("/api/v1/network/consolidation/search", "post", "Search pairwise same-origin consolidation feasibility", "Network Optimizer", True, True, "bno_consolidation"),
    ("/api/v1/network/marketplace/load-opportunities", "get", "List marketplace load opportunities", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/marketplace/load-opportunities/{id}", "get", "Get marketplace load opportunity", "Network Optimizer", True, True, None),
    ("/api/v1/network/marketplace/capacities", "get", "List marketplace capacities", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/marketplace/capacities/{id}", "get", "Get marketplace capacity", "Network Optimizer", True, True, None),
    ("/api/v1/network/compatibility/cargo-types", "get", "List cargo type catalog", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/compatibility/equipment-types", "get", "List equipment type catalog", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/compatibility/pallet-types", "get", "List pallet type catalog", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/compatibility/packaging-types", "get", "List packaging type catalog", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/compatibility/rule-sets", "get", "List compatibility rule sets", "Network Optimizer", True, True, "bno_list"),
    ("/api/v1/network/compatibility/rule-sets", "post", "Create a draft compatibility rule set", "Network Optimizer", True, True, "bno_compatibility"),
    ("/api/v1/network/compatibility/rule-sets/{id}/rules", "post", "Add a draft compatibility rule", "Network Optimizer", True, True, "bno_compatibility"),
    ("/api/v1/network/compatibility/rule-sets/{id}/rules/{ruleCode}", "delete", "Remove a draft compatibility rule", "Network Optimizer", True, True, "bno_compatibility"),
    ("/api/v1/network/compatibility/rule-sets/{id}/activate", "post", "Activate a compatibility rule set", "Network Optimizer", True, True, "bno_compatibility"),
    ("/api/v1/network/compatibility/rule-sets/{id}/retire", "post", "Retire a compatibility rule set", "Network Optimizer", True, True, "bno_compatibility"),
    ("/api/v1/network/compatibility/cargo-equipment/evaluate", "post", "Evaluate cargo equipment compatibility", "Network Optimizer", True, True, "bno_compatibility"),
    ("/api/v1/network/compatibility/groupage/evaluate", "post", "Evaluate groupage compatibility", "Network Optimizer", True, True, "bno_compatibility"),
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
    "network-optimizer-service.yaml": {"Network Optimizer"},
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

NO_REQUEST_BODY_PROFILES = frozenset({
    "reconcile_payment",
    "q_validate_publish",
    "q_question_duplicate",
    "vl_fork_draft",
    "tl_archive",
    "tl_fork_draft",
    "tl_delete",
})

QUESTIONNAIRE_NO_CONTENT_PROFILES = frozenset({
    "q_section_delete",
    "q_section_reorder",
    "q_question_delete",
    "q_question_reorder",
    "q_option_delete",
    "q_rule_delete",
    "tl_delete",
    "tl_section_delete",
    "tl_section_reorder",
    "tl_question_delete",
    "tl_question_reorder",
    "tl_option_delete",
    "tl_rule_delete",
})

QUESTIONNAIRE_CREATED_PROFILES = frozenset({
    "q_section_create",
    "q_question_create",
    "q_option_create",
    "q_rule_create",
    "q_question_duplicate",
    "vl_fork_draft",
    "vl_restore_draft",
    "tl_create",
    "tl_fork_draft",
    "e5_clone",
    "tl_section_create",
    "tl_question_create",
    "tl_option_create",
    "tl_rule_create",
    "tl_question_duplicate",
})

QUESTIONNAIRE_OK_POST_PROFILES = frozenset({
    "q_save_draft",
    "q_validate_publish",
    "vl_publish",
    "vl_compare",
    "vl_change_impact_preview",
    "tl_publish",
    "tl_archive",
})

VERSION_LIFECYCLE_422_PROFILES = frozenset({"vl_publish", "tl_publish"})

IDEMPOTENCY_HEADER_PROFILES = frozenset({
    "priced_transport_order_create",
    "vl_publish",
    "vl_fork_draft",
    "vl_restore_draft",
    "tl_publish",
    "tl_fork_draft",
    "e5_clone",
    "ls_create",
    "ls_approve",
    "ls_reject",
})

LATE_SUBMISSION_IDEMPOTENCY_PROFILES = frozenset({"ls_create", "ls_approve", "ls_reject"})

LATE_SUBMISSION_OK_POST_PROFILES = frozenset({"ls_approve", "ls_reject"})

LATE_SUBMISSION_422_PROFILES = frozenset({
    "ls_create",
    "ls_list_mine",
    "ls_list_buyer",
    "ls_approve",
    "ls_reject",
})

E7_EXCEL_EXCHANGE_ENDPOINT_PROFILES = frozenset({
    "xlsx_export_buyer_draft",
    "xlsx_export_carrier_response",
    "xlsx_import_preview_buyer_draft",
    "xlsx_import_preview_carrier_response",
    "xlsx_import_commit_buyer_draft",
    "xlsx_import_commit_carrier_response",
    "xlsx_create_template_buyer_draft",
    "xlsx_create_preview_buyer_draft",
    "xlsx_create_commit_buyer_draft",
})

BINARY_RESPONSE_PROFILES = frozenset({
    "xlsx_export_buyer_draft",
    "xlsx_export_carrier_response",
    "xlsx_create_template_buyer_draft",
})

EXCEL_EXCHANGE_PREVIEW_PROFILES = frozenset({"xlsx_import_preview_buyer_draft", "xlsx_import_preview_carrier_response"})
XLSX_CREATE_PREVIEW_PROFILES = frozenset({"xlsx_create_preview_buyer_draft"})
XLSX_CREATE_COMMIT_PROFILES = frozenset({"xlsx_create_commit_buyer_draft"})

E7_ERP_PREVIEW_ENDPOINT_PROFILES = frozenset({
    "erp_import_preview_create_draft",
    "erp_import_preview_update_draft",
})

E7_ERP_CREATE_COMMIT_ENDPOINT_PROFILES = frozenset({
    "erp_import_create_commit",
})

E7_ERP_UPDATE_COMMIT_ENDPOINT_PROFILES = frozenset({
    "erp_import_update_commit",
})

E7_ERP_READ_ENDPOINT_PROFILES = frozenset({
    "erp_get_rfx_event_by_id",
    "erp_get_rfx_by_external_id",
    "erp_get_import_analysis_status",
    "erp_get_integration_capabilities",
})

ERP_PREVIEW_PROFILES = frozenset(E7_ERP_PREVIEW_ENDPOINT_PROFILES)
ERP_CREATE_COMMIT_PROFILES = frozenset(E7_ERP_CREATE_COMMIT_ENDPOINT_PROFILES)
ERP_UPDATE_COMMIT_PROFILES = frozenset(E7_ERP_UPDATE_COMMIT_ENDPOINT_PROFILES)
ERP_READ_PROFILES = frozenset(E7_ERP_READ_ENDPOINT_PROFILES)

PROFILE_OPERATION_IDS = {
    "erp_import_preview_create_draft": "postErpRfxDraftCreatePreview",
    "erp_import_preview_update_draft": "postErpRfxDraftUpdatePreview",
    "erp_import_create_commit": "postErpRfxDraftCreateCommit",
    "erp_import_update_commit": "postErpRfxDraftUpdateCommit",
    "erp_get_rfx_event_by_id": "getErpRfxEventById",
    "erp_get_rfx_by_external_id": "getErpRfxByExternalId",
    "erp_get_import_analysis_status": "getErpImportAnalysisStatus",
    "erp_get_integration_capabilities": "getErpIntegrationCapabilities",
    "carrier_invited_event_get": "get_carrier_invited_rfx_event",
    "xlsx_create_template_buyer_draft": "get_buyer_new_rfx_event_xlsx_create_template",
}

EXCEL_EXCHANGE_COMMIT_PROFILES = frozenset({"xlsx_import_commit_buyer_draft", "xlsx_import_commit_carrier_response"})

EXCEL_EXCHANGE_ERROR_RESPONSES = """        '400':
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
        '413':
          description: Request body or uploaded file too large
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'"""

EXCEL_EXCHANGE_COMMIT_ERROR_RESPONSES = """        '400':
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
        '422':
          description: Stored proposal failed revalidation
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'"""

ERP_CREATE_COMMIT_ERROR_RESPONSES = """        '400':
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
          description: Conflict (`external_id_conflict`, `idempotency_conflict`, `stale_mapping_context`, `actor_binding_denied`, `analysis_expired`, `analysis_already_consumed`, `canonical_hash_mismatch`)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: Unprocessable stored analysis
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '429':
          description: Rate limit exceeded
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'"""

ERP_READ_ERROR_RESPONSES = """        '400':
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
          description: Conflict (`event_not_draft` when the target RFx is PUBLISHED or otherwise not DRAFT)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '429':
          description: Rate limit exceeded
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'"""

ERP_UPDATE_COMMIT_ERROR_RESPONSES = """        '400':
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
          description: Conflict (`stale_target`, `proposal_revalidation_failed`, `stale_mapping_context`, `actor_binding_denied`, `analysis_expired`, `analysis_already_consumed`, `canonical_hash_mismatch`, `idempotency_conflict`, `external_id_conflict`)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: Unprocessable stored analysis
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '429':
          description: Rate limit exceeded
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'"""

CARRIER_INVITED_EVENT_DESCRIPTION = """Carrier-scoped read of an RFx event the authenticated company is invited to.

Authorization: **CarrierRead** role required (`CARRIER_ADMIN`, `CARRIER_DISPATCHER`).
Access is fail-closed on tenant isolation and invited-company membership.
The payload is the invited-event list item (own participant/response summary only).
It does not expose participants, foreign responses, rates, or scores.

The buyer route `GET /api/v1/rfx-events/{id}` remains **BuyerRead** and is not opened to carriers."""

EXCEL_EXCHANGE_DESCRIPTIONS = {
    "xlsx_export_buyer_draft": """Export buyer draft RFx event as an XLSX workbook snapshot.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **BuyerManage** role required; buyer read-only roles are denied (**403**).

Scope: exports the active DRAFT questionnaire graph only. No carrier/competitor bid or response data is included in the workbook.

Precondition: an active DRAFT questionnaire version must exist; otherwise returns **409** conflict.""",
    "xlsx_export_carrier_response": """Export own carrier RFx response as an XLSX workbook snapshot.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **CarrierRead** role required; response must belong to the authenticated carrier company.

Scope: exports own questionnaire answers and commercial offer lines only. No competitor IDs, prices, rankings, or scores are included.

DRAFT responses export with `export_mode=DRAFT_EDIT`. SUBMITTED responses export read-only with `export_mode=SUBMITTED_READONLY`.""",
    "xlsx_import_preview_carrier_response": """Preview carrier RFx response XLSX import for UPDATE_CARRIER_DRAFT mode.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **CarrierRespond** role required; response must belong to the authenticated carrier company.

Request: multipart/form-data with required binary `file` field (max 5 MiB). Workbook schema `BINTRANS_RFX_CARRIER_XLSX_V1`.

Successful domain-valid preview returns **200** with `ready_to_commit=true`, persisted `analysis_id`, and server-computed `expires_at` (24h TTL).

Domain-invalid but structurally readable workbooks return **422** with the structured preview envelope and `ready_to_commit=false` without persisting analysis.

Malformed multipart, unsafe ZIP/XLSX, or schema contract failures return **400** / **413** with the standard error envelope.

Precondition: response must be **DRAFT**; SUBMITTED responses return **409** conflict.""",
    "xlsx_import_preview_buyer_draft": """Preview buyer draft RFx event XLSX import for UPDATE_DRAFT mode.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **BuyerManage** role required; buyer read-only roles are denied (**403**).

Request: multipart/form-data with required binary `file` field (max 5 MiB). Workbook schema `BINTRANS_RFX_BUYER_XLSX_V1`.

Successful domain-valid preview returns **200** with `ready_to_commit=true`, persisted `analysis_id`, and server-computed `expires_at` (24h TTL).

Domain-invalid but structurally readable workbooks return **422** with the structured preview envelope and `ready_to_commit=false` without persisting analysis.

Malformed multipart, unsafe ZIP/XLSX, or schema contract failures return **400** / **413** with the standard error envelope.

Precondition: an active DRAFT questionnaire version must exist; otherwise returns **409** conflict.""",
    "xlsx_import_commit_buyer_draft": """Atomically apply a persisted buyer XLSX import preview analysis to the active DRAFT.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **BuyerManage** role required; only the preview creator actor may commit.

Requires `Idempotency-Key` header. Request body contains only `analysis_id`.""",
    "xlsx_create_template_buyer_draft": """Download a blank CREATE-compatible BUYER XLSX V1 workbook.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **BuyerManage** role required (`PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER`); buyer read-only and carrier roles are denied (**403**). Human JWT only. Integration OAuth/API key is not accepted.

Read-only GET. No request body. Query `tenant_id` is rejected (**403**). Tenant and company authority come only from verified auth context.

Success returns **200** binary `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` with `Content-Disposition: attachment; filename="bintrans-rfx-buyer-xlsx-v1-create-template.xlsx"`, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`.

The workbook contains canonical sheets and headers plus Metadata `schema_name` / `schema_version` only. It does not write events, analyses, lots, questionnaire, participants, publish/submit/award, or ERP/TMS state.

Shared gateway rate limit returns **429**.""",
    "xlsx_create_preview_buyer_draft": """Preview buyer new RFx event XLSX create for CREATE_NEW_DRAFT mode.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **BuyerManage** role required; buyer read-only roles are denied (**403**). Human JWT only. Integration OAuth/API key is not accepted.

Request: multipart/form-data. Required fields: `file` (max 5 MiB), `owner_company_id`, `rfx_number`, `title`, `rfx_type`, `category`. Optional: `description`, `response_deadline`, `currency_code`. Workbook schema `BINTRANS_RFX_BUYER_XLSX_V1`. Workbook IDs are not authority.

Successful domain-valid preview returns **200** with `ready_to_commit=true`, persisted `analysis_id`, and server-computed `expires_at` (24h TTL). Preview does not return `canonical_payload_hash` or a reserved event ID.

Domain-invalid but structurally readable workbooks return **422** with the structured preview envelope and `ready_to_commit=false` without persisting analysis.

Malformed multipart, unsafe ZIP/XLSX, formula cells, missing/unexpected sheets, invalid headers, competitor columns, or `unsupported_schema` return **400**. Oversized files return **413**. Gateway rate limit returns **429**.

Zero lots and an empty questionnaire graph are allowed. Preview never creates an event, version, lots, questionnaire, participants, scoring, or RFx-create audit.""",
    "xlsx_create_commit_buyer_draft": """Atomically create a new DRAFT RFx event from a persisted CREATE_NEW_DRAFT XLSX analysis.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **BuyerManage** role required; only the preview creator actor and resolved owner company may commit. Human JWT only.

Requires `Idempotency-Key` header (max 128 chars). Request body contains only `analysis_id`.

First successful commit and same-key replay both return **201** with the same `event_id`. Created event has `creation_channel=EXCEL`, `status=DRAFT`, and `questionnaire_enabled=false`. Lots are optional. Participants and scoring are not created.

Conflict machine codes: `analysis_expired`, `analysis_already_consumed`, `idempotency_conflict`, duplicate tenant `rfx_number`. Revalidation failures including an expired deadline return **422** `proposal_revalidation_failed` or `canonical_hash_mismatch` with zero event writes.

Gateway shared rate limit returns **429**.""",
    "erp_import_preview_create_draft": """Preview ERP buyer JSON import for CREATE_DRAFT mode.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: integration bearer auth with scopes `rfx:draft:preview` and `rfx:draft:create`.

Request: `application/json` body with schema `BINTRANS_RFX_ERP_JSON_V1` and `requested_operation=CREATE_DRAFT` (max 2 MiB).

Successful domain-valid preview returns **200** with `ready_to_commit=true`, persisted `analysis_id`, and server-computed `expires_at` (24h TTL).

Domain-invalid but structurally readable payloads return **422** with the structured preview envelope and `ready_to_commit=false` without persisting analysis (OPTION A).""",
    "erp_import_preview_update_draft": """Preview ERP buyer JSON import for UPDATE_DRAFT mode.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: integration bearer auth with scopes `rfx:draft:preview` and `rfx:draft:read`.

Request: `application/json` body with schema `BINTRANS_RFX_ERP_JSON_V1` and `requested_operation=UPDATE_DRAFT` (max 2 MiB).

Successful domain-valid preview returns **200** with `ready_to_commit=true`, persisted `analysis_id`, and server-computed `expires_at` (24h TTL).

Domain-invalid but structurally readable payloads return **422** with the structured preview envelope and `ready_to_commit=false` without persisting analysis (OPTION A).

State-dependent existing-link rule (not expressible as JSON Schema): if the target event already has an external object link, UPDATE Preview requires `external` with the same normalized `system` and `object_id` and a new `revision` that is not the current link revision and is not already stored in that link's revision history. Missing `external`, missing `revision`, or a repeated revision returns **422** `VALIDATION_ERROR` with `details.field=external.revision` and no new `machine_code`. Changing the stable identity is rejected before analysis persist. Events without a link may omit `external`.

Precondition: target RFx event must be **DRAFT**; PUBLISHED events return **409** `stale_target`.""",
    "erp_import_create_commit": """Apply a persisted ERP CREATE preview analysis and allocate a DRAFT RFx event.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled) and writes zero records.

Authorization: integration bearer auth with scopes `rfx:draft:commit` and `rfx:draft:create`. Tenant, company, and integration principal are taken from trusted gateway context only.

Requires `Idempotency-Key` header (max 128 chars). Request body contains only `analysis_id`.

Successful commit returns **201** with `rfx_event_id`, `external_link_id`, `creation_channel=ERP`, and `external_revision`. The created event remains **DRAFT** (no auto-publish).

Conflict machine codes: `external_id_conflict`, `idempotency_conflict`, `stale_mapping_context`, `actor_binding_denied`, `analysis_expired`, `analysis_already_consumed`, `canonical_hash_mismatch`.""",
    "erp_import_update_commit": """Apply a persisted ERP UPDATE preview analysis to an existing DRAFT RFx event.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled) and writes zero records.

Authorization: integration bearer auth with scopes `rfx:draft:commit` and `rfx:draft:read`. Tenant, company, and integration principal are taken from trusted gateway context only.

Requires `Idempotency-Key` header (max 128 chars). Request body contains only `analysis_id`.

Successful commit returns **200** with `rfx_event_id` and `applied_at`. The event remains **DRAFT** (no auto-publish). Commit revalidates stored baseline tokens `event_row_version`, `draft_row_version`, `baseline_lots_fingerprint`, and when a link existed at Preview `baseline_external_link_id` / `baseline_external_revision`.

If the event already has an external link, an accepted UPDATE must apply a new unused `external.revision`: backfill the previous revision/hash into history when E4 left it only on the link, append the new history row, and update link metadata in place without rebinding `rfx_event_id`. Link or current-revision drift after Preview returns **409** `stale_target` with zero writes. Same-key replay returns the stored **200** without extra history rows. Events without a link may omit `external`; first bind creates the link and the initial history row atomically.

Conflict machine codes: `stale_target`, `proposal_revalidation_failed`, `stale_mapping_context`, `actor_binding_denied`, `analysis_expired`, `analysis_already_consumed`, `canonical_hash_mismatch`, `idempotency_conflict`, `external_id_conflict`.""",
    "erp_get_rfx_event_by_id": """Return an allowlisted ERP DRAFT summary by internal RFx event ID.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: integration bearer auth with scope `rfx:draft:read`. Tenant, company, and integration principal are taken from trusted gateway context only. Human JWT is not accepted.

Response: `ErpRfxDraftSummary` only (not the human UI event DTO). PUBLISHED or non-DRAFT events return **409** `event_not_draft`. Cross-tenant or cross-company lookups return **404**.

E2 per-principal rate limit applies (**429**). Read operations do not write audit events.""",
    "erp_get_rfx_by_external_id": """Return an allowlisted ERP DRAFT summary by stable external identity.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: integration bearer auth with scope `rfx:draft:read`. Lookup key is tenant + principal + `external_system` + `RFX_EVENT` + `external_object_id`. Revision is optional metadata only.

Query `external_revision` matching the current link revision returns **200** even when E4 CREATE left history empty. Any other unrecorded revision returns **404**. PUBLISHED or non-DRAFT events return **409** `event_not_draft`.

E2 per-principal rate limit applies (**429**). Read operations do not write audit events.""",
    "erp_get_import_analysis_status": """Return the persisted ERP import analysis status snapshot.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: integration bearer auth with scope `rfx:status:read`. Only the owning integration principal may read the analysis. Foreign tenant, company, or principal returns **404** without revealing existence.

Semantics are synchronous v1: the handler reads the persisted row and does not imply a background worker.

E2 per-principal rate limit applies (**429**). Read operations do not write audit events.""",
    "erp_get_integration_capabilities": """Return principal-authenticated ERP v1 capabilities.

Feature flag: when `RFX_ERP_INTEGRATION_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: integration bearer auth with scope `rfx:status:read` only. Unauthenticated requests return **401**.

Response includes `schema_version=BINTRANS_RFX_ERP_JSON_V1`, parser limits, supported mapping types, and deferred fields.

E2 per-principal rate limit applies (**429**). Read operations do not write audit events.""",
    "xlsx_import_commit_carrier_response": """Atomically apply a persisted carrier XLSX import preview analysis to a DRAFT response.

Feature flag: when `RFX_EXCEL_EXCHANGE_ENABLED` is false (default), the route returns **404** (feature disabled).

Authorization: **CarrierRespond** role required; only the preview creator actor may commit; response must belong to the authenticated carrier company.

Requires `Idempotency-Key` header. Request body contains only `analysis_id`. Commit applies stored canonical payload only (no XLSX re-parse), updates answers and offer lines, and leaves the response in DRAFT (no auto-submit).""",
}

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

QUESTIONNAIRE_REQUEST_BODIES = {
    "q_save_draft": """              $ref: '#/components/schemas/RfxSaveDraftRequest'""",
    "q_section_create": """              $ref: '#/components/schemas/RfxCreateSectionRequest'""",
    "q_section_update": """              $ref: '#/components/schemas/RfxUpdateSectionRequest'""",
    "q_section_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "q_section_reorder": """              $ref: '#/components/schemas/RfxReorderSectionsRequest'""",
    "q_question_create": """              $ref: '#/components/schemas/RfxCreateQuestionRequest'""",
    "q_question_update": """              $ref: '#/components/schemas/RfxUpdateQuestionRequest'""",
    "q_question_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "q_question_duplicate": """              $ref: '#/components/schemas/RfxDuplicateQuestionRequest'""",
    "q_question_reorder": """              $ref: '#/components/schemas/RfxReorderQuestionsRequest'""",
    "q_option_create": """              $ref: '#/components/schemas/RfxCreateOptionRequest'""",
    "q_option_update": """              $ref: '#/components/schemas/RfxUpdateOptionRequest'""",
    "q_option_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "q_rule_create": """              $ref: '#/components/schemas/RfxCreateRuleRequest'""",
    "q_rule_update": """              $ref: '#/components/schemas/RfxUpdateRuleRequest'""",
    "q_rule_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "vl_publish": """              $ref: '#/components/schemas/RfxPublishQuestionnaireRequest'""",
    "vl_change_impact_preview": """              $ref: '#/components/schemas/RfxChangeImpactPreviewRequest'""",
    "vl_compare": """              $ref: '#/components/schemas/RfxCompareVersionsRequest'""",
    "vl_restore_draft": """              $ref: '#/components/schemas/RfxRestoreVersionAsDraftRequest'""",
}

TEMPLATE_REQUEST_BODIES = {
    "tl_create": """              $ref: '#/components/schemas/RfxCreateTemplateRequest'""",
    "tl_update": """              $ref: '#/components/schemas/RfxUpdateTemplateRequest'""",
    "tl_publish": """              $ref: '#/components/schemas/RfxPublishTemplateVersionRequest'""",
    "tl_section_create": """              $ref: '#/components/schemas/RfxCreateSectionRequest'""",
    "tl_section_update": """              $ref: '#/components/schemas/RfxUpdateSectionRequest'""",
    "tl_section_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "tl_section_reorder": """              $ref: '#/components/schemas/RfxReorderSectionsRequest'""",
    "tl_question_create": """              $ref: '#/components/schemas/RfxCreateTemplateQuestionRequest'""",
    "tl_question_update": """              $ref: '#/components/schemas/RfxUpdateQuestionRequest'""",
    "tl_question_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "tl_question_duplicate": """              $ref: '#/components/schemas/RfxDuplicateQuestionRequest'""",
    "tl_question_reorder": """              $ref: '#/components/schemas/RfxReorderQuestionsRequest'""",
    "tl_option_create": """              $ref: '#/components/schemas/RfxCreateOptionRequest'""",
    "tl_option_update": """              $ref: '#/components/schemas/RfxUpdateOptionRequest'""",
    "tl_option_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
    "tl_rule_create": """              $ref: '#/components/schemas/RfxCreateRuleRequest'""",
    "tl_rule_update": """              $ref: '#/components/schemas/RfxUpdateRuleRequest'""",
    "tl_rule_delete": """              $ref: '#/components/schemas/RfxVersionedMutationRequest'""",
}

E5_CLONE_REQUEST_BODIES = {
    "e5_clone": """              $ref: '#/components/schemas/RfxCloneEventFromTemplateRequest'""",
}

CARRIER_RESPONSE_REQUEST_BODIES = {
    "cr_answers_patch": """              $ref: '#/components/schemas/RfxCarrierAnswerBatchPatch'""",
    "cr_submit": """              type: object
              required: [save_version]
              properties:
                save_version:
                  type: integer
                  format: int64""",
}

CARRIER_RESPONSE_422_PROFILES = frozenset({"cr_answers_patch", "cr_submit"})
CARRIER_WORKSPACE_UNBOUND_422_PROFILES = frozenset({"cr_workspace_get"})
CARRIER_WORKSPACE_UNBOUND_422_DESCRIPTION = (
    "Unbound commercial carrier response cannot be opened as a questionnaire workspace. "
    "Envelope is ErrorResponse with error.code=UNPROCESSABLE_ENTITY and details.field=rfx_version_id. "
    "Other validation failures remain 400."
)

LATE_SUBMISSION_REQUEST_BODIES = {
    "ls_create": """              $ref: '#/components/schemas/RfxCreateLateSubmissionRequest'""",
    "ls_approve": """              $ref: '#/components/schemas/RfxApproveLateSubmissionRequest'""",
    "ls_reject": """              $ref: '#/components/schemas/RfxRejectLateSubmissionRequest'""",
}

LATE_SUBMISSION_SCHEMAS = {
    "ls_create": "RfxLateSubmissionRequest",
    "ls_list_mine": "RfxLateSubmissionRequestList",
    "ls_list_buyer": "RfxLateSubmissionRequestList",
    "ls_approve": "RfxLateSubmissionRequest",
    "ls_reject": "RfxLateSubmissionRequest",
}

CARRIER_RESPONSE_SCHEMAS = {
    "cr_workspace_get": "RfxCarrierResponseWorkspace",
    "cr_start": "RfxCarrierResponseWorkspace",
    "cr_answers_patch": "RfxCarrierResponseSaveResult",
    "cr_validate": "RfxCarrierResponseValidationResult",
    "cr_submit": "RfxCarrierResponseSubmitResult",
    "cr_summary_get": "RfxCarrierResponseValidationResult",
}

QUESTIONNAIRE_RESPONSE_SCHEMAS = {
    "q_studio_get": "RfxStudioResponse",
    "q_questionnaire_get": "RfxQuestionnaireDefinition",
    "q_save_draft": "RfxVersionRecord",
    "q_validate_publish": "RfxPublishReadinessResult",
    "q_section_create": "RfxSection",
    "q_section_update": "RfxSection",
    "q_question_create": "RfxQuestion",
    "q_question_update": "RfxQuestion",
    "q_question_duplicate": "RfxQuestion",
    "q_option_create": "RfxQuestionOption",
    "q_option_update": "RfxQuestionOption",
    "q_rule_create": "RfxQuestionRule",
    "q_rule_update": "RfxQuestionRule",
    "vl_publish": "RfxVersionRecord",
    "vl_change_impact_preview": "RfxChangeImpactAnalysisResponse",
    "vl_list": "RfxVersionListResponse",
    "vl_detail": "RfxVersionDetailResponse",
    "vl_fork_draft": "RfxVersionRecord",
    "vl_compare": "RfxCompareVersionsResponse",
    "vl_restore_draft": "RfxVersionRecord",
}

TEMPLATE_RESPONSE_SCHEMAS = {
    "tl_list": "RfxTemplateListResponse",
    "tl_create": "RfxTemplateDetailResponse",
    "tl_detail": "RfxTemplateDetailResponse",
    "tl_update": "RfxTemplateRecord",
    "tl_archive": "RfxTemplateRecord",
    "tl_publish": "RfxTemplateVersionRecord",
    "tl_fork_draft": "RfxTemplateVersionRecord",
    "tl_questionnaire_get": "RfxTemplateQuestionnaireDefinition",
    "tl_version_questionnaire_get": "RfxTemplateQuestionnaireDefinition",
    "rfx_event_detail": "RfxEventDetailResponse",
    "carrier_invited_event_get": "CarrierInvitedEventResponse",
    "tl_section_create": "RfxTemplateSection",
    "tl_section_update": "RfxTemplateSection",
    "tl_question_create": "RfxTemplateQuestion",
    "tl_question_update": "RfxTemplateQuestion",
    "tl_question_duplicate": "RfxTemplateQuestion",
    "tl_option_create": "RfxTemplateQuestionOption",
    "tl_option_update": "RfxTemplateQuestionOption",
    "tl_rule_create": "RfxTemplateQuestionRule",
    "tl_rule_update": "RfxTemplateQuestionRule",
    "e5_clone": "RfxCloneEventFromTemplateResponse",
}

READ_RESPONSE_SCHEMAS = {
    "payment_list": "PaymentListResponse",
    "payment_detail": "PaymentRecord",
    "payment_allocations_list": "PaymentAllocationListResponse",
    "payment_audit_list": "PaymentAuditEventListResponse",
    "payment_eligible_obligations_list": "EligiblePaymentObligationListResponse",
    **QUESTIONNAIRE_RESPONSE_SCHEMAS,
    **TEMPLATE_RESPONSE_SCHEMAS,
    **CARRIER_RESPONSE_SCHEMAS,
    **LATE_SUBMISSION_SCHEMAS,
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
    elif profile == "bno_list":
        lines.extend(_pagination_query_lines())
    elif profile == "carrier_invited_event_get":
        lines.extend([
            "        - name: carrier_company_id",
            "          in: query",
            "          required: false",
            "          schema:",
            "            type: string",
            "            format: uuid",
        ])
    elif profile in {"ls_create", "ls_list_mine"}:
        lines.extend([
            "        - name: carrier_company_id",
            "          in: query",
            "          required: false",
            "          description: Carrier company context already accepted by the backend handler.",
            "          schema:",
            "            type: string",
            "            format: uuid",
        ])
    elif profile == "erp_get_rfx_by_external_id":
        lines.extend([
            "        - name: external_system",
            "          in: query",
            "          required: true",
            "          schema:",
            "            type: string",
            "            maxLength: 64",
            "        - name: external_object_id",
            "          in: query",
            "          required: true",
            "          schema:",
            "            type: string",
            "            maxLength: 256",
            "        - name: external_revision",
            "          in: query",
            "          required: false",
            "          schema:",
            "            type: string",
        ])
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
    elif profile in {"bno_foundation_mutation", "bno_prediction"} and method == "post":
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: false",
            "          description: Replays the original mutation result when the same key and body are retried.",
            "          schema:",
            "            type: string",
            "            maxLength: 128",
        ])
    elif profile in XLSX_CREATE_COMMIT_PROFILES:
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            "          description: Client-supplied idempotency key for buyer XLSX create commit (max 128 chars).",
            "          schema:",
            "            type: string",
            "            maxLength: 128",
        ])
    elif profile in EXCEL_EXCHANGE_COMMIT_PROFILES:
        commit_label = "buyer XLSX import commit"
        if profile == "xlsx_import_commit_carrier_response":
            commit_label = "carrier XLSX import commit"
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            f"          description: Client-supplied idempotency key for {commit_label} (max 128 chars).",
            "          schema:",
            "            type: string",
            "            maxLength: 128",
        ])
    elif profile in ERP_CREATE_COMMIT_PROFILES:
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            "          description: Client-supplied idempotency key for ERP CREATE commit (max 128 chars).",
            "          schema:",
            "            type: string",
            "            minLength: 1",
            "            maxLength: 128",
        ])
    elif profile in ERP_UPDATE_COMMIT_PROFILES:
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            "          description: Client-supplied idempotency key for ERP UPDATE commit (max 128 chars).",
            "          schema:",
            "            type: string",
            "            minLength: 1",
            "            maxLength: 128",
        ])
    elif profile in LATE_SUBMISSION_IDEMPOTENCY_PROFILES:
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            "          description: Client-supplied idempotency key for late submission mutation (max 128 chars).",
            "          schema:",
            "            type: string",
            "            minLength: 1",
            "            maxLength: 128",
        ])
    elif profile == "cr_submit":
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: false",
            "          description: Required when submitting after the event response deadline (late submission). Client-supplied idempotency key (max 128 chars).",
            "          schema:",
            "            type: string",
            "            minLength: 1",
            "            maxLength: 128",
        ])
    elif profile in IDEMPOTENCY_HEADER_PROFILES - {"priced_transport_order_create"}:
        lines.extend([
            "        - name: Idempotency-Key",
            "          in: header",
            "          required: true",
            "          description: Client-supplied idempotency key for version lifecycle mutation (max 128 chars).",
            "          schema:",
            "            type: string",
            "            minLength: 1",
            "            maxLength: 128",
        ])
    return "\n".join(lines) + "\n"


def _bno_anonymized_geography_lines() -> list[str]:
    return [
        "        Search geography stays on the owner record and is not a public anonymized view.",
        "        ANONYMIZED_MARKETPLACE display omits location_id, latitude, longitude, facility labels, address lines, and postal codes.",
        "        When known, anonymized display returns country_code, region, and city. Missing coarse fields are omitted and are not invented.",
        "        MARKETPLACE and other non-anonymized scopes retain the exact location fields allowed by the visibility matrix.",
        "        Anonymous results do not include an exact deadhead distance.",
    ]


def render_operation(
    path: str,
    method: str,
    summary: str,
    tag: str,
    with_headers: bool,
    secured: bool,
    profile: str | None = None,
) -> str:
    operation_id = PROFILE_OPERATION_IDS.get(profile, f"{method}_{path_to_id(summary)}")
    lines = [
        f"    {method}:",
        f"      tags: [{tag}]",
        f"      summary: {summary}",
        f"      operationId: {operation_id}",
    ]

    if profile == "bno_foundation_mutation":
        lines.append("      description: |")
        lines.append("        BINTRANS Network Optimizer foundation mutation.")
        lines.append("        Tenant and user identity come from the verified gateway JWT. Client identity headers are not trusted.")
        lines.append("        Idempotency-Key replays the original result and does not create a second publication or outbox event.")
        lines.append("        Omitted physical attributes stay unknown. Zero is not a substitute for unknown.")
        lines.append("        A capacity location_id is resolved in the caller tenant against transport.locations. A foreign location id is not found.")
        lines.append("        Source-backed load coordinates are replaced by the tenant-scoped source location snapshot.")
    elif profile == "bno_prediction":
        lines.append("      description: |")
        lines.append("        Owner predictive capacity. A generated prediction is PRIVATE and PREDICTED.")
        lines.append("        It is not marketplace capacity and it does not search or rank next loads.")
        lines.append("        Activation to AVAILABLE stays PRIVATE until a separate publication.")
        lines.append("        Confidence is a versioned rule score, not a calibrated probability.")
    elif profile == "bno_compatibility":
        lines.append("      description: |")
        lines.append("        Compatibility evaluation uses explicit cargo and equipment facts plus the active catalog and rule-set versions.")
        lines.append("        It does not search the marketplace, rank matches, or plan a route.")
        lines.append("        Unknown facts stay unknown. A tenant rule cannot weaken a system or regulatory hard deny.")
    elif profile == "bno_list":
        lines.append("      description: |")
        lines.append("        Tenant-scoped list. Marketplace reads return the visibility projection, not source shipment or transport-order rows.")
        if "/marketplace/" in path:
            lines.extend(_bno_anonymized_geography_lines())
    elif path.startswith("/api/v1/network/marketplace/"):
        lines.append("      description: |")
        lines.append("        Marketplace read. The response is the visibility projection, not the source shipment or transport-order row.")
        lines.extend(_bno_anonymized_geography_lines())
    elif profile in VOID_DESCRIPTIONS:
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
    elif profile in EXCEL_EXCHANGE_DESCRIPTIONS:
        lines.append("      description: |")
        for desc_line in EXCEL_EXCHANGE_DESCRIPTIONS[profile].splitlines():
            if desc_line:
                lines.append(f"        {desc_line}")
            else:
                lines.append("")
    elif profile == "carrier_invited_event_get":
        lines.append("      description: |")
        for desc_line in CARRIER_INVITED_EVENT_DESCRIPTION.splitlines():
            if desc_line:
                lines.append(f"        {desc_line}")
            else:
                lines.append("")

    parameters = render_parameters(path, method, with_headers, profile)
    if parameters:
        lines.append(parameters.rstrip("\n"))
    elif with_headers:
        lines.append(COMMON_HEADER.rstrip("\n"))

    if profile == "oauth_client_credentials":
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/x-www-form-urlencoded:",
                "            schema:",
                "              type: object",
                "              required: [grant_type, client_id, client_secret]",
                "              properties:",
                "                grant_type:",
                "                  type: string",
                "                  enum: [client_credentials]",
                "                client_id:",
                "                  type: string",
                "                client_secret:",
                "                  type: string",
                "                  format: password",
                "      responses:",
                "        '200':",
                "          description: OAuth access token issued",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/OAuthIntegrationTokenResponse'",
                "        '400':",
                "          description: Invalid OAuth request",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/OAuthIntegrationErrorResponse'",
                "        '401':",
                "          description: Client authentication failed",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/OAuthIntegrationErrorResponse'",
                "        '403':",
                "          description: Client IP not allowed",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/OAuthIntegrationErrorResponse'",
                "        '429':",
                "          description: Rate limit exceeded",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/OAuthIntegrationErrorResponse'",
                "",
            ]
        )
        return "\n".join(lines)

    if (
        method in {"post", "patch", "put"}
        and profile not in NO_REQUEST_BODY_PROFILES
        and profile not in EXCEL_EXCHANGE_PREVIEW_PROFILES
        and profile not in XLSX_CREATE_PREVIEW_PROFILES
        and profile not in EXCEL_EXCHANGE_COMMIT_PROFILES
        and profile not in XLSX_CREATE_COMMIT_PROFILES
        and profile not in ERP_PREVIEW_PROFILES
        and profile not in ERP_CREATE_COMMIT_PROFILES
        and profile not in ERP_UPDATE_COMMIT_PROFILES
        and profile not in ERP_READ_PROFILES
    ):
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
        elif profile in QUESTIONNAIRE_REQUEST_BODIES:
            lines.append(QUESTIONNAIRE_REQUEST_BODIES[profile])
        elif profile in TEMPLATE_REQUEST_BODIES:
            lines.append(TEMPLATE_REQUEST_BODIES[profile])
        elif profile in E5_CLONE_REQUEST_BODIES:
            lines.append(E5_CLONE_REQUEST_BODIES[profile])
        elif profile in CARRIER_RESPONSE_REQUEST_BODIES:
            lines.append(CARRIER_RESPONSE_REQUEST_BODIES[profile])
        elif profile in LATE_SUBMISSION_REQUEST_BODIES:
            lines.append(LATE_SUBMISSION_REQUEST_BODIES[profile])
        elif profile == "priced_transport_order_create":
            lines.append(PRICED_TRANSPORT_ORDER_REQUEST_BODY)
        elif profile == "bno_next_load":
            lines.append("              $ref: '#/components/schemas/NextLoadSearchRequest'")
        elif profile == "bno_consolidation":
            lines.append("              $ref: '#/components/schemas/ConsolidationSearchRequest'")
        else:
            lines.extend(
                [
                    "              type: object",
                    "              additionalProperties: true",
                ]
            )
    elif method == "delete" and profile in QUESTIONNAIRE_REQUEST_BODIES:
        lines.extend(
            [
                "      requestBody:",
                "        required: false",
                "        content:",
                "          application/json:",
                "            schema:",
                QUESTIONNAIRE_REQUEST_BODIES[profile],
            ]
        )
    elif method == "delete" and profile in TEMPLATE_REQUEST_BODIES:
        lines.extend(
            [
                "      requestBody:",
                "        required: false",
                "        content:",
                "          application/json:",
                "            schema:",
                TEMPLATE_REQUEST_BODIES[profile],
            ]
        )

    if secured and profile not in EXCEL_EXCHANGE_PREVIEW_PROFILES and profile not in XLSX_CREATE_PREVIEW_PROFILES and profile not in EXCEL_EXCHANGE_COMMIT_PROFILES and profile not in XLSX_CREATE_COMMIT_PROFILES and profile not in ERP_PREVIEW_PROFILES and profile not in ERP_CREATE_COMMIT_PROFILES and profile not in ERP_UPDATE_COMMIT_PROFILES and profile not in ERP_READ_PROFILES:
        lines.append(SECURITY_BEARER.rstrip("\n"))

    if profile in BINARY_RESPONSE_PROFILES:
        binary_200_description = "XLSX workbook attachment"
        if profile == "xlsx_create_template_buyer_draft":
            binary_200_description = (
                '"Blank CREATE-compatible BUYER XLSX V1 workbook download. '
                "Filename bintrans-rfx-buyer-xlsx-v1-create-template.xlsx. "
                "Attachment, Cache-Control no-store, X-Content-Type-Options nosniff. "
                'Read-only; no event or analysis writes."'
            )
        binary_lines = [
            "      responses:",
            "        '200':",
            f"          description: {binary_200_description}",
            "          content:",
            "            application/vnd.openxmlformats-officedocument.spreadsheetml.sheet:",
            "              schema:",
            "                type: string",
            "                format: binary",
            ERROR_RESPONSES.rstrip("\n"),
        ]
        if profile == "xlsx_create_template_buyer_draft":
            binary_lines.extend(
                [
                    "        '429':",
                    "          description: Shared gateway rate limit exceeded",
                    "          content:",
                    "            application/json:",
                    "              schema:",
                    "                $ref: '#/components/schemas/ErrorResponse'",
                ]
            )
        binary_lines.append("")
        lines.extend(binary_lines)
        return "\n".join(lines)

    if profile in EXCEL_EXCHANGE_PREVIEW_PROFILES:
        workbook_label = "BUYER XLSX workbook (max 5 MiB)"
        preview_schema = "RfxBuyerXlsxImportPreviewResponse"
        if profile == "xlsx_import_preview_carrier_response":
            workbook_label = "CARRIER XLSX workbook (max 5 MiB)"
            preview_schema = "RfxCarrierXlsxImportPreviewResponse"
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          multipart/form-data:",
                "            schema:",
                "              type: object",
                "              required: [file]",
                "              properties:",
                "                file:",
                "                  type: string",
                "                  format: binary",
                f"                  description: {workbook_label}",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '200':",
                "          description: Valid import preview with persisted analysis",
                "          content:",
                "            application/json:",
                "              schema:",
                f"                $ref: '#/components/schemas/{preview_schema}'",
                "        '422':",
                "          description: Domain-invalid structured import preview",
                "          content:",
                "            application/json:",
                "              schema:",
                f"                $ref: '#/components/schemas/{preview_schema}'",
                EXCEL_EXCHANGE_ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile in XLSX_CREATE_PREVIEW_PROFILES:
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          multipart/form-data:",
                "            schema:",
                "              type: object",
                "              required: [file, owner_company_id, rfx_number, title, rfx_type, category]",
                "              properties:",
                "                file:",
                "                  type: string",
                "                  format: binary",
                "                  description: BUYER XLSX workbook (max 5 MiB)",
                "                owner_company_id:",
                "                  type: string",
                "                  format: uuid",
                "                rfx_number:",
                "                  type: string",
                "                title:",
                "                  type: string",
                "                rfx_type:",
                "                  type: string",
                "                category:",
                "                  type: string",
                "                description:",
                "                  type: string",
                "                response_deadline:",
                "                  type: string",
                "                  format: date-time",
                "                currency_code:",
                "                  type: string",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '200':",
                "          description: Valid CREATE_NEW_DRAFT preview with persisted analysis",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/RfxBuyerXlsxCreatePreviewResponse'",
                "        '422':",
                "          description: Domain-invalid structured CREATE_NEW_DRAFT preview",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/RfxBuyerXlsxCreatePreviewResponse'",
                EXCEL_EXCHANGE_ERROR_RESPONSES.rstrip("\n"),
                "        '429':",
                "          description: Rate limit exceeded",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErrorResponse'",
                "",
            ]
        )
        return "\n".join(lines)

    if profile in XLSX_CREATE_COMMIT_PROFILES:
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              $ref: '#/components/schemas/RfxBuyerXlsxCreateCommitRequest'",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '201':",
                "          description: New DRAFT RFx event created from XLSX analysis",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/RfxBuyerXlsxCreateCommitResponse'",
                EXCEL_EXCHANGE_COMMIT_ERROR_RESPONSES.rstrip("\n"),
                "        '429':",
                "          description: Rate limit exceeded",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErrorResponse'",
                "",
            ]
        )
        return "\n".join(lines)

    if profile in ERP_PREVIEW_PROFILES:
        preview_422 = [
                "        '422':",
                "          description: Domain-invalid structured ERP import preview",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErpImportPreviewResponse'",
        ]
        if profile == "erp_import_preview_update_draft":
            preview_422 = [
                "        '422':",
                "          description: Domain-invalid structured ERP import preview, or existing-link revision/identity VALIDATION_ERROR",
                "          content:",
                "            application/json:",
                "              schema:",
                "                oneOf:",
                "                  - $ref: '#/components/schemas/ErpImportPreviewResponse'",
                "                  - $ref: '#/components/schemas/ErrorResponse'",
            ]
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              $ref: '#/components/schemas/ErpImportPreviewRequest'",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '200':",
                "          description: Valid ERP import preview with persisted analysis",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErpImportPreviewResponse'",
            ]
        )
        lines.extend(preview_422)
        lines.extend(
            [
                EXCEL_EXCHANGE_ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile in ERP_CREATE_COMMIT_PROFILES:
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              $ref: '#/components/schemas/ErpCreateCommitRequest'",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '201':",
                "          description: ERP CREATE commit allocated a DRAFT event",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErpCreateCommitResponse'",
                ERP_CREATE_COMMIT_ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile in ERP_UPDATE_COMMIT_PROFILES:
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              $ref: '#/components/schemas/ErpUpdateCommitRequest'",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '200':",
                "          description: ERP UPDATE commit applied to a DRAFT event",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErpUpdateCommitResponse'",
                ERP_UPDATE_COMMIT_ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile in ERP_READ_PROFILES:
        response_schema = {
            "erp_get_rfx_event_by_id": "ErpRfxDraftSummary",
            "erp_get_rfx_by_external_id": "ErpRfxDraftSummary",
            "erp_get_import_analysis_status": "ErpAnalysisStatus",
            "erp_get_integration_capabilities": "ErpIntegrationCapabilities",
        }[profile]
        success_desc = {
            "erp_get_rfx_event_by_id": "Allowlisted ERP DRAFT summary",
            "erp_get_rfx_by_external_id": "Allowlisted ERP DRAFT summary for the stable external identity",
            "erp_get_import_analysis_status": "Persisted ERP analysis status snapshot",
            "erp_get_integration_capabilities": "Principal-scoped ERP v1 capabilities",
        }[profile]
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '200':",
                f"          description: {success_desc}",
                "          content:",
                "            application/json:",
                "              schema:",
                f"                $ref: '#/components/schemas/{response_schema}'",
                ERP_READ_ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile in EXCEL_EXCHANGE_COMMIT_PROFILES:
        commit_request_schema = "RfxBuyerXlsxImportCommitRequest"
        commit_response_schema = "RfxBuyerXlsxImportCommitResponse"
        commit_success_description = "Import analysis committed to active draft"
        if profile == "xlsx_import_commit_carrier_response":
            commit_request_schema = "RfxCarrierXlsxImportCommitRequest"
            commit_response_schema = "RfxCarrierXlsxImportCommitResponse"
            commit_success_description = "Import analysis committed to carrier DRAFT response"
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                f"              $ref: '#/components/schemas/{commit_request_schema}'",
            ]
        )
        if secured:
            lines.append(SECURITY_BEARER.rstrip("\n"))
        lines.extend(
            [
                "      responses:",
                "        '200':",
                f"          description: {commit_success_description}",
                "          content:",
                "            application/json:",
                "              schema:",
                f"                $ref: '#/components/schemas/{commit_response_schema}'",
                EXCEL_EXCHANGE_COMMIT_ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile in QUESTIONNAIRE_NO_CONTENT_PROFILES:
        lines.extend(
            [
                "      responses:",
                "        '204':",
                "          description: Successful response",
                ERROR_RESPONSES.rstrip("\n"),
                "",
            ]
        )
        return "\n".join(lines)

    if profile == "bno_prediction" and method == "post" and path.endswith(("/refresh", "/activate")):
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              type: object",
                "              required: [version]",
                "              properties:",
                "                version:",
                "                  type: integer",
                "                  minimum: 1",
            ]
        )
    if profile == "bno_compatibility" and method == "post" and path.endswith("/rules"):
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              $ref: '#/components/schemas/CompatibilityRuleDraft'",
            ]
        )
    elif profile == "bno_compatibility" and method == "post":
        lines.extend(
            [
                "      requestBody:",
                "        required: true",
                "        content:",
                "          application/json:",
                "            schema:",
                "              type: object",
                "              additionalProperties: true",
            ]
        )
    if profile in QUESTIONNAIRE_CREATED_PROFILES:
        success_code = "201"
    elif profile in VOID_DESCRIPTIONS or profile in RECONCILE_DESCRIPTIONS or profile in QUESTIONNAIRE_OK_POST_PROFILES or profile in CARRIER_RESPONSE_SCHEMAS or profile in LATE_SUBMISSION_OK_POST_PROFILES:
        success_code = "200"
    elif profile == "bno_prediction" and ("/refresh" in path or "/activate" in path):
        success_code = "200"
    elif profile == "bno_next_load" or profile == "bno_consolidation":
        success_code = "200"
    elif profile == "bno_compatibility":
        success_code = "200" if method != "post" or path.endswith("/evaluate") or path.endswith("/activate") or path.endswith("/retire") else "201"
    elif method == "post" and tag not in {"Gateway", "Auth"}:
        success_code = "201"
    else:
        success_code = "200"
    success_desc = "Successful response"
    if profile == "void_allocation":
        success_desc = "Allocation voided or idempotent success"
    elif profile == "void_payment":
        success_desc = "Payment voided or idempotent success"
    elif profile == "reconcile_payment":
        success_desc = "Payment reconciled or idempotent success"

    response_schema = READ_RESPONSE_SCHEMAS.get(profile or "")
    if profile == "bno_compatibility" and path.endswith("/evaluate"):
        response_schema = "CompatibilityEvaluation"
    if profile == "bno_next_load":
        response_schema = "NextLoadSearchResponse"
    if profile == "bno_consolidation":
        response_schema = "ConsolidationSearchResponse"
    if method == "get" and path == "/api/v1/network/marketplace/load-opportunities/{id}":
        response_schema = "NetworkMarketplaceLoad"
    elif method == "get" and path == "/api/v1/network/marketplace/capacities/{id}":
        response_schema = "NetworkMarketplaceCapacity"
    elif method == "get" and path == "/api/v1/network/capacities/{id}":
        response_schema = "NetworkCapacity"
    elif method == "post" and path == "/api/v1/network/capacities":
        response_schema = "NetworkCapacity"
    elif method == "get" and path == "/api/v1/network/load-opportunities/{id}":
        response_schema = "NetworkLoadOpportunity"
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
        ]
    )
    if profile in CARRIER_WORKSPACE_UNBOUND_422_PROFILES:
        lines.extend(
            [
                "        '422':",
                "          description: |",
                f"            {CARRIER_WORKSPACE_UNBOUND_422_DESCRIPTION}",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErrorResponse'",
                "              example:",
                "                error:",
                "                  code: UNPROCESSABLE_ENTITY",
                "                  message: carrier response is not bound to a published questionnaire version",
                "                  details:",
                "                    field: rfx_version_id",
            ]
        )
    elif profile in CARRIER_RESPONSE_422_PROFILES:
        lines.extend(
            [
                "        '422':",
                "          description: Carrier answer validation failed",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/RfxCarrierValidationFailed'",
            ]
        )
    elif profile in VERSION_LIFECYCLE_422_PROFILES:
        lines.extend(
            [
                "        '422':",
                "          description: Publish blocked by readiness or impact analysis",
                "          content:",
                "            application/json:",
                "              schema:",
                "                oneOf:",
                "                  - $ref: '#/components/schemas/ErrorResponse'",
                "                  - $ref: '#/components/schemas/ValidationFailedResponse'",
            ]
        )
    elif profile in LATE_SUBMISSION_422_PROFILES:
        lines.extend(
            [
                "        '422':",
                "          description: Unprocessable entity",
                "          content:",
                "            application/json:",
                "              schema:",
                "                $ref: '#/components/schemas/ErrorResponse'",
            ]
        )
    lines.append("")
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


def questionnaire_prefix_schemas_block() -> str:
    return """    RfxSaveDraftRequest:
      type: object
      properties:
        expected_version:
          type: integer
          description: Optimistic-lock version of the draft questionnaire row
    RfxQuestionType:
      type: string
      enum:
        - TEXT
        - LONG_TEXT
        - NUMBER
        - MONEY
        - YES_NO
        - SINGLE_SELECT
        - MULTI_SELECT
        - DATE
        - DATETIME
        - FILE
        - TABLE
        - ADDRESS
        - COUNTRY
        - COMPANY
        - VEHICLE_CATEGORY
        - CERTIFICATE
        - PERCENT
        - RATING
      description: Canonical RFx v3.0B question definition type (deterministic rule engine; no arbitrary code execution).
    RfxRuleAction:
      type: string
      enum: [SHOW, HIDE, REQUIRE]
    RfxConditionOperator:
      type: string
      enum:
        - AND
        - OR
        - EQUALS
        - NOT_EQUALS
        - IN
        - NOT_IN
        - IS_EMPTY
        - IS_NOT_EMPTY
        - GREATER_THAN
        - LESS_THAN
      description: Deterministic conditional operators supported by v3.0B (no arbitrary executable expressions).
    RfxVersionedMutationRequest:
      type: object
      properties:
        expected_version:
          type: integer
          description: Optimistic-lock version of the entity being deleted
    RfxCreateSectionRequest:
      type: object
      required: [section_code, title]
      additionalProperties: false
      properties:
        section_code: {type: string}
        title: {type: string}
        description: {type: string, nullable: true}
        sort_order: {type: integer}
    RfxUpdateSectionRequest:
      type: object
      required: [expected_version]
      additionalProperties: false
      properties:
        title: {type: string}
        description: {type: string, nullable: true}
        sort_order: {type: integer}
        expected_version: {type: integer}
    RfxReorderSectionsRequest:
      type: object
      required: [ordered_ids]
      additionalProperties: false
      properties:
        ordered_ids:
          type: array
          items: {type: string, format: uuid}
    RfxCreateQuestionRequest:
      type: object
      required: [section_id, question_code, question_type, label]
      additionalProperties: false
      properties:
        section_id: {type: string, format: uuid}
        question_code: {type: string}
        question_type:
          $ref: '#/components/schemas/RfxQuestionType'
        label: {type: string}
        help_text: {type: string, nullable: true}
        required: {type: boolean, default: false}
        validation_rule_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
    RfxUpdateQuestionRequest:
      type: object
      required: [expected_version]
      additionalProperties: false
      properties:
        question_type:
          $ref: '#/components/schemas/RfxQuestionType'
        label: {type: string}
        help_text: {type: string, nullable: true}
        required: {type: boolean}
        validation_rule_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
        expected_version: {type: integer}
    RfxDuplicateQuestionRequest:
      type: object
      additionalProperties: false
    RfxReorderQuestionsRequest:
      type: object
      required: [section_id, ordered_ids]
      additionalProperties: false
      properties:
        section_id: {type: string, format: uuid}
        ordered_ids:
          type: array
          items: {type: string, format: uuid}
    RfxCreateOptionRequest:
      type: object
      required: [option_code, label]
      additionalProperties: false
      properties:
        option_code: {type: string}
        label: {type: string}
        sort_order: {type: integer}
    RfxUpdateOptionRequest:
      type: object
      required: [expected_version]
      additionalProperties: false
      properties:
        label: {type: string}
        sort_order: {type: integer}
        expected_version: {type: integer}
    RfxCreateRuleRequest:
      type: object
      required: [rule_code, action]
      additionalProperties: false
      properties:
        rule_code: {type: string}
        action:
          type: string
          enum: [SHOW, HIDE, REQUIRE]
        target_question_code: {type: string}
        condition_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
    RfxUpdateRuleRequest:
      type: object
      required: [expected_version]
      additionalProperties: false
      properties:
        action:
          type: string
          enum: [SHOW, HIDE, REQUIRE]
        target_question_code: {type: string}
        condition_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
        expected_version: {type: integer}
    RfxValidationDefinition:
      type: object
      properties:
        min_length: {type: integer}
        max_length: {type: integer}
        min_value: {type: number}
        max_value: {type: number}
        pattern: {type: string}
        allowed_mime:
          type: array
          items: {type: string}
        max_file_size: {type: integer, format: int64}
    RfxConditionalExpression:
      type: object
      required: [operator]
      properties:
        operator: {type: string}
        source_question_code: {type: string}
        value: {}
        children:
          type: array
          items:
            $ref: '#/components/schemas/RfxConditionalExpression'
"""


def questionnaire_base_version_record_block() -> str:
    return """    RfxVersionRecord:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        rfx_event_id: {type: string, format: uuid}
        version_number: {type: integer}
        status:
          type: string
          enum: [DRAFT, PUBLISHED, SUPERSEDED, ARCHIVED]
        questionnaire_enabled: {type: boolean}
        published_at: {type: string, format: date-time, nullable: true}
        published_by: {type: string, format: uuid, nullable: true}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
"""


def questionnaire_version_lifecycle_e1_schemas_block() -> str:
    return """    RfxPublishQuestionnaireRequest:
      type: object
      required: [expected_event_version, expected_draft_version, change_summary]
      properties:
        expected_event_version:
          type: integer
          minimum: 1
        expected_draft_version:
          type: integer
          minimum: 1
        change_summary:
          type: string
          minLength: 1
        impact_analysis_id:
          type: string
          format: uuid
        canonical_diff_hash:
          type: string
          minLength: 1
    RfxVersionRecord:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        rfx_event_id: {type: string, format: uuid}
        version_number: {type: integer}
        status:
          type: string
          enum: [DRAFT, PUBLISHED, SUPERSEDED, ARCHIVED]
        questionnaire_enabled: {type: boolean}
        is_current_published: {type: boolean}
        is_active_draft: {type: boolean}
        change_summary: {type: string, nullable: true}
        published_at: {type: string, format: date-time, nullable: true}
        published_by: {type: string, format: uuid, nullable: true}
        superseded_at: {type: string, format: date-time, nullable: true}
        superseded_by_version_id: {type: string, format: uuid, nullable: true}
        rescoring_required: {type: boolean}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
    RfxVersionListResponse:
      type: object
      properties:
        versions:
          type: array
          items:
            $ref: '#/components/schemas/RfxVersionRecord'
    RfxVersionDetailResponse:
      type: object
      properties:
        version:
          $ref: '#/components/schemas/RfxVersionRecord'
        questionnaire:
          $ref: '#/components/schemas/RfxQuestionnaireDefinition'
"""


def questionnaire_version_lifecycle_e2_schemas_block() -> str:
    return """    RfxCompareVersionsRequest:
      type: object
      required: [source_version_id, target_version_id]
      properties:
        source_version_id:
          type: string
          format: uuid
        target_version_id:
          type: string
          format: uuid
    RfxRestoreVersionAsDraftRequest:
      type: object
      required: [change_summary]
      properties:
        change_summary:
          type: string
          minLength: 1
    RfxCompareFieldDiff:
      type: object
      properties:
        field: {type: string}
        before: {}
        after: {}
    RfxCompareItemDiff:
      type: object
      properties:
        entity_type: {type: string}
        change:
          type: string
          enum: [ADDED, REMOVED, CHANGED, REORDERED, UNCHANGED]
        section_code: {type: string}
        question_code: {type: string}
        option_code: {type: string}
        rule_code: {type: string}
        criterion_code: {type: string}
        fields:
          type: array
          items: {type: string}
        field_diffs:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareFieldDiff'
    RfxCompareSummary:
      type: object
      properties:
        added_count: {type: integer}
        removed_count: {type: integer}
        changed_count: {type: integer}
        reordered_count: {type: integer}
        unchanged_count: {type: integer}
    RfxCompareScoringDiff:
      type: object
      properties:
        criteria:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        bindings:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        model:
          $ref: '#/components/schemas/RfxCompareItemDiff'
    RfxCompareVersionsResponse:
      type: object
      properties:
        source_version:
          type: object
          properties:
            id: {type: string, format: uuid}
            version_number: {type: integer}
            status: {type: string}
        target_version:
          type: object
          properties:
            id: {type: string, format: uuid}
            version_number: {type: integer}
            status: {type: string}
        source_version_number: {type: integer}
        target_version_number: {type: integer}
        summary:
          $ref: '#/components/schemas/RfxCompareSummary'
        canonical_diff_hash: {type: string}
        differences:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        sections:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        questions:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        options:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        rules:
          type: array
          items:
            $ref: '#/components/schemas/RfxCompareItemDiff'
        scoring:
          $ref: '#/components/schemas/RfxCompareScoringDiff'
"""


def questionnaire_version_lifecycle_e3_schemas_block() -> str:
    return """    RfxChangeImpactPreviewRequest:
      type: object
      required: [candidate_version_id]
      properties:
        candidate_version_id:
          type: string
          format: uuid
    RfxChangeImpactAnalysisResponse:
      type: object
      properties:
        impact_analysis_id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        event_id: {type: string, format: uuid}
        source_version_id: {type: string, format: uuid, nullable: true}
        candidate_version_id: {type: string, format: uuid}
        canonical_diff_hash: {type: string}
        impact_classes:
          type: array
          items:
            type: string
            enum:
              - NON_MATERIAL
              - MATERIAL_NO_RESPONSES
              - MATERIAL_WITH_DRAFT_RESPONSES
              - MATERIAL_WITH_SUBMITTED_RESPONSES
              - SCORING_AFFECTING
              - KNOCKOUT_AFFECTING
        affected_draft_response_count: {type: integer}
        affected_submitted_response_count: {type: integer}
        scoring_affecting: {type: boolean}
        knockout_affecting: {type: boolean}
        expires_at: {type: string, format: date-time}
"""


def questionnaire_entity_schemas_block() -> str:
    return """    RfxSection:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        rfx_version_id: {type: string, format: uuid}
        section_code: {type: string}
        title: {type: string}
        description: {type: string, nullable: true}
        sort_order: {type: integer}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
    RfxQuestionOption:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        question_id: {type: string, format: uuid}
        option_code: {type: string}
        label: {type: string}
        sort_order: {type: integer}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
    RfxQuestion:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        section_id: {type: string, format: uuid}
        question_code: {type: string}
        question_type:
          $ref: '#/components/schemas/RfxQuestionType'
        label: {type: string}
        help_text: {type: string, nullable: true}
        required: {type: boolean}
        validation_rule_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
        options:
          type: array
          items:
            $ref: '#/components/schemas/RfxQuestionOption'
    RfxQuestionRule:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        rfx_version_id: {type: string, format: uuid}
        target_question_id: {type: string, format: uuid, nullable: true}
        rule_code: {type: string}
        action:
          type: string
          enum: [SHOW, HIDE, REQUIRE]
        condition_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
    RfxSectionWithQuestions:
      type: object
      properties:
        section:
          $ref: '#/components/schemas/RfxSection'
        questions:
          type: array
          items:
            $ref: '#/components/schemas/RfxQuestion'
    RfxPublishReadinessItem:
      type: object
      properties:
        code: {type: string}
        status:
          type: string
          enum: [PASS, FAIL, WARN]
        message: {type: string}
        details: {type: object, additionalProperties: true}
    RfxPublishReadinessResult:
      type: object
      properties:
        ready: {type: boolean}
        blocking_fail_count: {type: integer}
        warning_count: {type: integer}
        items:
          type: array
          items:
            $ref: '#/components/schemas/RfxPublishReadinessItem'
    RfxQuestionnaireDefinition:
      type: object
      properties:
        event_id: {type: string, format: uuid}
        rfx_version_id: {type: string, format: uuid}
        version_number: {type: integer}
        questionnaire_enabled: {type: boolean}
        version_status:
          type: string
          enum: [DRAFT, PUBLISHED, SUPERSEDED, ARCHIVED]
        sections:
          type: array
          items:
            $ref: '#/components/schemas/RfxSectionWithQuestions'
        rules:
          type: array
          items:
            $ref: '#/components/schemas/RfxQuestionRule'
    RfxStudioEventRecord:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        rfx_number: {type: string}
        rfx_type: {type: string}
        category: {type: string}
        title: {type: string}
        description: {type: string, nullable: true}
        owner_company_id: {type: string, format: uuid}
        status: {type: string}
        currency_code: {type: string, nullable: true}
        valid_from: {type: string, format: date, nullable: true}
        valid_to: {type: string, format: date, nullable: true}
        response_deadline: {type: string, format: date-time, nullable: true}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
    RfxEventDetailResponse:
      allOf:
        - $ref: '#/components/schemas/RfxStudioEventRecord'
        - type: object
          properties:
            creation_channel:
              type: string
              enum: [MANUAL, TEMPLATE, EXCEL, ERP]
              description: Persisted rfx.rfx_events.creation_channel. Present for rows after migration 000073 (NOT NULL DEFAULT MANUAL). Omitted only when the stored value is empty; the API does not invent a channel.
            source_template_id: {type: string, format: uuid, nullable: true}
            source_template_version_id: {type: string, format: uuid, nullable: true}
            source_version_number: {type: integer, nullable: true}
            source_version_status: {type: string, enum: [PUBLISHED, SUPERSEDED], nullable: true}
            source_version_warning: {type: boolean, nullable: true}
            source_template_code: {type: string, nullable: true}
            source_template_name_i18n: {type: object, additionalProperties: {type: string}, nullable: true}
    CarrierInvitedEventResponse:
      allOf:
        - $ref: '#/components/schemas/RfxStudioEventRecord'
        - type: object
          properties:
            participant_status: {type: string}
            own_response_status: {type: string, enum: [NOT_STARTED, DRAFT, SUBMITTED]}
            own_response_id: {type: string, format: uuid, nullable: true}
            lot_count: {type: integer}
            participant_company_id: {type: string, format: uuid}
    RfxStudioResponse:
      type: object
      properties:
        event:
          $ref: '#/components/schemas/RfxStudioEventRecord'
        draft_version:
          allOf:
            - $ref: '#/components/schemas/RfxVersionRecord'
          nullable: true
        sections:
          type: array
          items:
            $ref: '#/components/schemas/RfxSectionWithQuestions'
        rules:
          type: array
          items:
            $ref: '#/components/schemas/RfxQuestionRule'
"""


def questionnaire_components_block(
    *,
    include_e1_version_lifecycle: bool = False,
    include_e4_template_library: bool = False,
) -> str:
    parts = [
        questionnaire_prefix_schemas_block(),
    ]
    if include_e1_version_lifecycle:
        parts.append(questionnaire_version_lifecycle_e1_schemas_block())
        parts.append(questionnaire_version_lifecycle_e2_schemas_block())
        parts.append(questionnaire_version_lifecycle_e3_schemas_block())
    else:
        parts.append(questionnaire_base_version_record_block())
    parts.append(questionnaire_entity_schemas_block())
    if include_e4_template_library:
        parts.append(template_library_components_block())
    return "".join(parts)


def template_library_components_block() -> str:
    return """    RfxCreateTemplateRequest:
      type: object
      required: [template_code, name_i18n]
      properties:
        template_code: {type: string, maxLength: 128}
        name_i18n: {type: object, additionalProperties: {type: string}}
        description_i18n: {type: object, additionalProperties: {type: string}}
        rfx_type: {type: string}
        owner_company_id: {type: string, format: uuid}
    RfxUpdateTemplateRequest:
      type: object
      required: [expected_version]
      properties:
        name_i18n: {type: object, additionalProperties: {type: string}}
        description_i18n: {type: object, additionalProperties: {type: string}}
        rfx_type: {type: string}
        expected_version: {type: integer, minimum: 1}
    RfxPublishTemplateVersionRequest:
      type: object
      required: [expected_template_version, expected_draft_version, change_summary]
      properties:
        expected_template_version: {type: integer, minimum: 1}
        expected_draft_version: {type: integer, minimum: 1}
        change_summary: {type: string, minLength: 1}
    RfxCloneEventFromTemplateRequest:
      type: object
      required: [template_version_id, rfx_number, rfx_type, category, title, owner_company_id]
      properties:
        template_version_id: {type: string, format: uuid}
        rfx_number: {type: string}
        rfx_type: {type: string}
        category: {type: string}
        title: {type: string}
        description: {type: string, nullable: true}
        owner_company_id: {type: string, format: uuid}
        currency_code: {type: string, nullable: true}
        valid_from: {type: string, format: date-time, nullable: true}
        valid_to: {type: string, format: date-time, nullable: true}
        response_deadline: {type: string, format: date-time, nullable: true}
    RfxCloneEventFromTemplateResponse:
      type: object
      properties:
        id: {type: string, format: uuid}
        draft_version_id: {type: string, format: uuid}
        source_template_id: {type: string, format: uuid}
        source_template_version_id: {type: string, format: uuid}
        source_version_number: {type: integer}
        source_version_status: {type: string, enum: [PUBLISHED, SUPERSEDED]}
        source_version_warning: {type: boolean}
      additionalProperties: true
    RfxTemplateRecord:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        template_code: {type: string}
        name_i18n: {type: object, additionalProperties: {type: string}}
        description_i18n: {type: object, additionalProperties: {type: string}, nullable: true}
        rfx_type: {type: string, nullable: true}
        owner_company_id: {type: string, format: uuid, nullable: true}
        status: {type: string, enum: [ACTIVE, ARCHIVED]}
        version: {type: integer}
        created_by: {type: string, format: uuid}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
    RfxTemplateVersionRecord:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        template_id: {type: string, format: uuid}
        version_number: {type: integer}
        status: {type: string, enum: [DRAFT, PUBLISHED, SUPERSEDED]}
        change_summary: {type: string, nullable: true}
        is_active_draft: {type: boolean}
        is_published: {type: boolean}
        published_at: {type: string, format: date-time, nullable: true}
        published_by: {type: string, format: uuid, nullable: true}
        created_by: {type: string, format: uuid}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
    RfxTemplateDetailResponse:
      type: object
      properties:
        template:
          $ref: '#/components/schemas/RfxTemplateRecord'
        draft_version:
          $ref: '#/components/schemas/RfxTemplateVersionRecord'
        published_version:
          $ref: '#/components/schemas/RfxTemplateVersionRecord'
        versions:
          type: array
          items:
            $ref: '#/components/schemas/RfxTemplateVersionRecord'
    RfxTemplateListResponse:
      allOf:
        - $ref: '#/components/schemas/PaginatedResponse'
        - type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/RfxTemplateRecord'
    RfxCreateTemplateQuestionRequest:
      allOf:
        - $ref: '#/components/schemas/RfxCreateQuestionRequest'
        - type: object
          required: [section_id]
          properties:
            section_id: {type: string, format: uuid}
    RfxTemplateSection:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        template_id: {type: string, format: uuid}
        rfx_template_version_id: {type: string, format: uuid}
        section_code: {type: string}
        title: {type: string}
        description: {type: string, nullable: true}
        sort_order: {type: integer}
        version: {type: integer}
    RfxTemplateQuestion:
      type: object
      properties:
        id: {type: string, format: uuid}
        section_id: {type: string, format: uuid}
        question_code: {type: string}
        question_type: {type: string}
        label: {type: string}
        required: {type: boolean}
        validation_rule_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
        version: {type: integer}
        options:
          type: array
          items:
            $ref: '#/components/schemas/RfxTemplateQuestionOption'
    RfxTemplateQuestionOption:
      type: object
      properties:
        id: {type: string, format: uuid}
        question_id: {type: string, format: uuid}
        option_code: {type: string}
        label: {type: string}
        sort_order: {type: integer}
        version: {type: integer}
    RfxTemplateQuestionRule:
      type: object
      properties:
        id: {type: string, format: uuid}
        template_id: {type: string, format: uuid}
        rfx_template_version_id: {type: string, format: uuid}
        rule_code: {type: string}
        action: {type: string, enum: [SHOW, HIDE, REQUIRE]}
        target_question_id: {type: string, format: uuid, nullable: true}
        condition_json: {type: object, additionalProperties: true}
        sort_order: {type: integer}
        version: {type: integer}
    RfxTemplateQuestionnaireDefinition:
      type: object
      properties:
        template_id: {type: string, format: uuid}
        rfx_template_version_id: {type: string, format: uuid}
        version_number: {type: integer}
        version_status: {type: string}
        sections:
          type: array
          items:
            type: object
            properties:
              section:
                $ref: '#/components/schemas/RfxTemplateSection'
              questions:
                type: array
                items:
                  $ref: '#/components/schemas/RfxTemplateQuestion'
        rules:
          type: array
          items:
            $ref: '#/components/schemas/RfxTemplateQuestionRule'
"""


def carrier_components_block() -> str:
    return """    RfxCarrierAnswerBatchPatch:
      type: object
      required: [save_version, answers]
      properties:
        save_version:
          type: integer
          format: int64
        answers:
          type: array
          items:
            $ref: '#/components/schemas/RfxCarrierAnswerPatchItem'
    RfxCarrierAnswerPatchItem:
      type: object
      required: [question_id, value]
      properties:
        section_id:
          type: string
          format: uuid
        question_id:
          type: string
          format: uuid
        field:
          type: string
        value: {}
    RfxCarrierAnswerRecord:
      type: object
      properties:
        id: {type: string, format: uuid}
        question_id: {type: string, format: uuid}
        value: {}
        answer_source: {type: string}
        validation_version: {type: integer}
        updated_at: {type: string, format: date-time}
        updated_by: {type: string, format: uuid, nullable: true}
        version: {type: integer}
    RfxCarrierResponseWorkspace:
      type: object
      properties:
        id: {type: string, format: uuid}
        tenant_id: {type: string, format: uuid}
        rfx_event_id: {type: string, format: uuid}
        participant_company_id: {type: string, format: uuid}
        rfx_version_id: {type: string, format: uuid, nullable: true}
        status: {type: string}
        product_status: {type: string}
        save_version: {type: integer, format: int64}
        completion_percent: {type: number, format: double}
        last_saved_at: {type: string, format: date-time, nullable: true}
        last_saved_by: {type: string, format: uuid, nullable: true}
        submitted_at: {type: string, format: date-time, nullable: true}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        version: {type: integer}
        questionnaire:
          $ref: '#/components/schemas/RfxQuestionnaireDefinition'
        answers:
          type: array
          items:
            $ref: '#/components/schemas/RfxCarrierAnswerRecord'
    RfxCarrierResponseSaveResult:
      type: object
      properties:
        response_id: {type: string, format: uuid}
        save_version: {type: integer, format: int64}
        last_saved_at: {type: string, format: date-time}
        last_saved_by: {type: string, format: uuid}
        completion_percent: {type: number, format: double}
    RfxCarrierValidationErrorItem:
      type: object
      properties:
        section_id: {type: string, format: uuid}
        question_id: {type: string, format: uuid}
        field: {type: string}
        rule: {type: string}
        message_key: {type: string}
        params:
          type: object
          additionalProperties: true
    RfxCarrierResponseValidationResult:
      type: object
      properties:
        valid: {type: boolean}
        blocking_error_count: {type: integer}
        errors:
          type: array
          items:
            $ref: '#/components/schemas/RfxCarrierValidationErrorItem'
        completion_percent: {type: number, format: double}
    RfxCarrierResponseSubmitResult:
      type: object
      properties:
        response_id: {type: string, format: uuid}
        status: {type: string}
        submitted_at: {type: string, format: date-time}
        save_version: {type: integer, format: int64}
    RfxCarrierValidationFailed:
      type: object
      properties:
        code: {type: string}
        message: {type: string}
        details:
          type: object
          additionalProperties: true
"""


def late_submission_components_block() -> str:
    return """    RfxLateSubmissionRequest:
      type: object
      properties:
        id: {type: string, format: uuid}
        rfx_event_id: {type: string, format: uuid}
        carrier_company_id: {type: string, format: uuid}
        participant_id: {type: string, format: uuid}
        reason_code:
          type: string
          enum: [TECHNICAL_FAILURE, ORGANIZATIONAL_DELAY, BUYER_REQUEST, FORCE_MAJEURE, OTHER]
        reason_text: {type: string}
        requested_until: {type: string, format: date-time}
        status:
          type: string
          enum: [REQUESTED, APPROVED, REJECTED, EXPIRED, CONSUMED]
        approved_valid_from: {type: string, format: date-time}
        approved_valid_until: {type: string, format: date-time}
        decision_comment: {type: string}
        requested_by: {type: string, format: uuid}
        decided_by: {type: string, format: uuid}
        decided_at: {type: string, format: date-time}
        consumed_at: {type: string, format: date-time}
        version: {type: integer}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
    RfxLateSubmissionRequestList:
      type: object
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/RfxLateSubmissionRequest'
    RfxCreateLateSubmissionRequest:
      type: object
      required: [reason_code, reason_text, requested_until]
      properties:
        reason_code:
          type: string
          enum: [TECHNICAL_FAILURE, ORGANIZATIONAL_DELAY, BUYER_REQUEST, FORCE_MAJEURE, OTHER]
        reason_text: {type: string, minLength: 1}
        requested_until: {type: string, format: date-time}
    RfxApproveLateSubmissionRequest:
      type: object
      required: [expected_version, approved_valid_from, approved_valid_until]
      properties:
        expected_version: {type: integer}
        approved_valid_from: {type: string, format: date-time}
        approved_valid_until: {type: string, format: date-time}
        decision_comment: {type: string}
    RfxRejectLateSubmissionRequest:
      type: object
      required: [expected_version]
      properties:
        expected_version: {type: integer}
        decision_comment: {type: string}
"""


def excel_exchange_components_block() -> str:
    return """    RfxBuyerXlsxImportPreviewIssue:
      type: object
      required: [severity, machine_code, message_key]
      properties:
        severity:
          type: string
          enum: [error, warning]
        machine_code: {type: string}
        sheet: {type: string}
        row: {type: integer}
        column: {type: string}
        stable_code: {type: string}
        message_key: {type: string}
        params:
          type: object
          additionalProperties: true
    RfxBuyerXlsxImportPreviewSummary:
      type: object
      properties:
        errors: {type: integer}
        warnings: {type: integer}
        sections_added: {type: integer}
        sections_changed: {type: integer}
        sections_removed: {type: integer}
        questions_added: {type: integer}
        questions_removed: {type: integer}
        lots_added: {type: integer}
        lots_changed: {type: integer}
        lots_removed: {type: integer}
    RfxBuyerXlsxImportPreviewResponse:
      type: object
      required:
        - schema_name
        - schema_version
        - mode
        - target_event_id
        - target_draft_version_id
        - target_version_number
        - target_event_row_version
        - target_draft_row_version
        - ready_to_commit
        - summary
        - questionnaire_diff
        - lots_diff
        - errors
        - warnings
      properties:
        schema_name: {type: string, example: BINTRANS_RFX_BUYER_XLSX_V1}
        schema_version: {type: string, example: "1"}
        mode: {type: string, enum: [UPDATE_DRAFT]}
        target_event_id: {type: string, format: uuid}
        target_draft_version_id: {type: string, format: uuid}
        target_version_number: {type: integer}
        target_event_row_version: {type: integer}
        target_draft_row_version: {type: integer}
        canonical_payload_hash: {type: string}
        analysis_id: {type: string, format: uuid}
        expires_at: {type: string, format: date-time}
        ready_to_commit: {type: boolean}
        summary:
          $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewSummary'
        questionnaire_diff:
          type: object
          additionalProperties: true
        lots_diff:
          type: object
          additionalProperties: true
        errors:
          type: array
          maxItems: 2000
          items:
            $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewIssue'
        warnings:
          type: array
          maxItems: 2000
          items:
            $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewIssue'
    RfxCarrierXlsxImportPreviewSummary:
      type: object
      properties:
        errors: {type: integer}
        warnings: {type: integer}
        answers_added: {type: integer}
        answers_changed: {type: integer}
        answers_removed: {type: integer}
        offer_lines_added: {type: integer}
        offer_lines_changed: {type: integer}
        offer_lines_removed: {type: integer}
    RfxCarrierXlsxImportPreviewResponse:
      type: object
      required:
        - schema_name
        - schema_version
        - mode
        - target_event_id
        - target_response_id
        - target_rfx_version_id
        - target_version_number
        - target_event_row_version
        - target_response_save_version
        - ready_to_commit
        - summary
        - answers_diff
        - offer_lines_diff
        - errors
        - warnings
      properties:
        schema_name: {type: string, example: BINTRANS_RFX_CARRIER_XLSX_V1}
        schema_version: {type: string, example: "1"}
        mode: {type: string, enum: [UPDATE_CARRIER_DRAFT]}
        target_event_id: {type: string, format: uuid}
        target_response_id: {type: string, format: uuid}
        target_rfx_version_id: {type: string, format: uuid}
        target_version_number: {type: integer}
        target_event_row_version: {type: integer}
        target_response_save_version: {type: integer, format: int64}
        canonical_payload_hash: {type: string}
        analysis_id: {type: string, format: uuid}
        expires_at: {type: string, format: date-time}
        ready_to_commit: {type: boolean}
        summary:
          $ref: '#/components/schemas/RfxCarrierXlsxImportPreviewSummary'
        answers_diff:
          type: object
          additionalProperties: true
        offer_lines_diff:
          type: object
          additionalProperties: true
        errors:
          type: array
          maxItems: 2000
          items:
            $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewIssue'
        warnings:
          type: array
          maxItems: 2000
          items:
            $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewIssue'
    RfxBuyerXlsxCreatePreviewDraftSummary:
      type: object
      required: [rfx_number, title, rfx_type, category, lot_count, section_count, question_count]
      properties:
        rfx_number: {type: string}
        title: {type: string}
        rfx_type: {type: string}
        category: {type: string}
        description: {type: string}
        response_deadline: {type: string, format: date-time}
        currency_code: {type: string}
        lot_count: {type: integer}
        section_count: {type: integer}
        question_count: {type: integer}
    RfxBuyerXlsxCreatePreviewResponse:
      type: object
      required:
        - mode
        - schema_name
        - schema_version
        - owner_company_id
        - ready_to_commit
        - normalized_draft_summary
        - change_counts
        - errors
        - warnings
      properties:
        mode: {type: string, enum: [CREATE_NEW_DRAFT]}
        schema_name: {type: string, example: BINTRANS_RFX_BUYER_XLSX_V1}
        schema_version: {type: string, example: "1"}
        owner_company_id: {type: string, format: uuid}
        analysis_id: {type: string, format: uuid}
        expires_at: {type: string, format: date-time}
        ready_to_commit: {type: boolean}
        normalized_draft_summary:
          $ref: '#/components/schemas/RfxBuyerXlsxCreatePreviewDraftSummary'
        change_counts:
          type: object
          additionalProperties: true
        errors:
          type: array
          maxItems: 2000
          items:
            $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewIssue'
        warnings:
          type: array
          maxItems: 2000
          items:
            $ref: '#/components/schemas/RfxBuyerXlsxImportPreviewIssue'
    RfxBuyerXlsxCreateCommitRequest:
      type: object
      required: [analysis_id]
      properties:
        analysis_id:
          type: string
          format: uuid
    RfxBuyerXlsxCreateCommitResponse:
      type: object
      required:
        - event_id
        - analysis_id
        - creation_channel
        - status
        - draft_version_id
        - draft_version_number
        - questionnaire_enabled
        - created_counts
        - committed_at
      properties:
        event_id: {type: string, format: uuid}
        analysis_id: {type: string, format: uuid}
        creation_channel: {type: string, enum: [EXCEL]}
        status: {type: string, enum: [DRAFT]}
        draft_version_id: {type: string, format: uuid}
        draft_version_number: {type: integer}
        questionnaire_enabled: {type: boolean}
        created_counts:
          type: object
          additionalProperties: true
        committed_at: {type: string, format: date-time}
    RfxBuyerXlsxImportCommitRequest:
      type: object
      required: [analysis_id]
      properties:
        analysis_id:
          type: string
          format: uuid
    RfxBuyerXlsxImportEntityChangeCounts:
      type: object
      properties:
        added:
          type: integer
        updated:
          type: integer
        deleted:
          type: integer
    RfxBuyerXlsxImportCommitResponse:
      type: object
      required: [event_id, draft_version_id, analysis_id, event_version, draft_version, committed_at, changes]
      properties:
        event_id:
          type: string
          format: uuid
        draft_version_id:
          type: string
          format: uuid
        analysis_id:
          type: string
          format: uuid
        event_version:
          type: integer
        draft_version:
          type: integer
        committed_at:
          type: string
          format: date-time
        changes:
          type: object
          properties:
            lots:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
            sections:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
            questions:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
            options:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
            rules:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
    RfxCarrierXlsxImportCommitRequest:
      type: object
      required: [analysis_id]
      properties:
        analysis_id:
          type: string
          format: uuid
    RfxCarrierXlsxImportCommitResponse:
      type: object
      required: [event_id, response_id, analysis_id, save_version, committed_at, changes]
      properties:
        event_id:
          type: string
          format: uuid
        response_id:
          type: string
          format: uuid
        analysis_id:
          type: string
          format: uuid
        save_version:
          type: integer
          format: int64
        committed_at:
          type: string
          format: date-time
        changes:
          type: object
          properties:
            answers:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
            offer_lines:
              $ref: '#/components/schemas/RfxBuyerXlsxImportEntityChangeCounts'
"""


def erp_preview_components_block() -> str:
    return """    ErpImportPreviewIssue:
      type: object
      required: [severity, machine_code]
      properties:
        severity:
          type: string
          enum: [error, warning]
        machine_code: {type: string}
        path: {type: string}
        message_key: {type: string}
        external_source: {type: string}
        params:
          type: object
          additionalProperties: true
    ErpImportPreviewRequest:
      type: object
      required: [schema_version, requested_operation, event]
      properties:
        schema_version:
          type: string
          enum: [BINTRANS_RFX_ERP_JSON_V1]
        requested_operation:
          type: string
          enum: [CREATE_DRAFT, UPDATE_DRAFT]
        external:
          type: object
          additionalProperties: false
          properties:
            system: {type: string}
            object_id: {type: string}
            revision: {type: string}
        event:
          type: object
          additionalProperties: false
          required: [type, title, currency, timezone]
          properties:
            type: {type: string}
            title: {type: string}
            description: {type: string}
            currency: {type: string}
            timezone: {type: string}
            deadline: {type: string, format: date-time}
        lots:
          type: array
          maxItems: 200
          items:
            type: object
            additionalProperties: false
            properties:
              lot_number: {type: string}
              name: {type: string}
              description: {type: string}
              category: {type: string}
              estimated_value: {type: number}
              currency_code: {type: string}
        questionnaire:
          type: object
          additionalProperties: true
        template_reference:
          type: object
          additionalProperties: true
        extensions:
          type: object
          additionalProperties: true
      additionalProperties: false
    ErpImportPreviewResponse:
      type: object
      required: [schema_version, ready_to_commit, errors, warnings]
      properties:
        schema_version:
          type: string
          example: BINTRANS_RFX_ERP_JSON_V1
        ready_to_commit: {type: boolean}
        analysis_id: {type: string, format: uuid}
        expires_at: {type: string, format: date-time}
        canonical_payload_hash: {type: string}
        errors:
          type: array
          items:
            $ref: '#/components/schemas/ErpImportPreviewIssue'
        warnings:
          type: array
          items:
            $ref: '#/components/schemas/ErpImportPreviewIssue'
    ErpCreateCommitRequest:
      type: object
      required: [analysis_id]
      additionalProperties: false
      properties:
        analysis_id:
          type: string
          format: uuid
    ErpCreateCommitResponse:
      type: object
      required: [rfx_event_id, external_link_id, creation_channel, external_revision]
      additionalProperties: false
      properties:
        rfx_event_id:
          type: string
          format: uuid
        external_link_id:
          type: string
          format: uuid
        creation_channel:
          type: string
          enum: [ERP]
        external_revision:
          type: string
    ErpUpdateCommitRequest:
      type: object
      required: [analysis_id]
      additionalProperties: false
      properties:
        analysis_id:
          type: string
          format: uuid
    ErpUpdateCommitResponse:
      type: object
      required: [rfx_event_id, applied_at]
      additionalProperties: false
      properties:
        rfx_event_id:
          type: string
          format: uuid
        applied_at:
          type: string
          format: date-time
    ErpExternalLink:
      type: object
      additionalProperties: false
      required: [system, object_type, object_id, revision]
      properties:
        system: {type: string}
        object_type:
          type: string
          enum: [RFX_EVENT]
        object_id: {type: string}
        revision: {type: string}
        requested_revision: {type: string}
    ErpPublishReadinessSummary:
      type: object
      additionalProperties: false
      required: [ready, blocking_fail_count, warning_count]
      properties:
        ready: {type: boolean}
        blocking_fail_count: {type: integer}
        warning_count: {type: integer}
    ErpQuestionnaireCounts:
      type: object
      additionalProperties: false
      required: [section_count, question_count, rule_count]
      properties:
        section_count: {type: integer}
        question_count: {type: integer}
        rule_count: {type: integer}
    ErpRfxDraftSummary:
      type: object
      additionalProperties: false
      required:
        - rfx_event_id
        - status
        - rfx_type
        - title
        - timezone
        - creation_channel
        - publish_readiness_summary
        - lot_count
        - questionnaire_counts
        - event_row_version
        - draft_row_version
      properties:
        rfx_event_id: {type: string, format: uuid}
        status:
          type: string
          enum: [DRAFT]
        rfx_type: {type: string}
        title: {type: string}
        description: {type: string}
        currency_code: {type: string}
        timezone: {type: string}
        response_deadline: {type: string, format: date-time}
        creation_channel:
          type: string
          enum: [MANUAL, TEMPLATE, EXCEL, ERP]
        external_link:
          nullable: true
          allOf:
            - $ref: '#/components/schemas/ErpExternalLink'
        publish_readiness_summary:
          $ref: '#/components/schemas/ErpPublishReadinessSummary'
        lot_count: {type: integer}
        questionnaire_counts:
          $ref: '#/components/schemas/ErpQuestionnaireCounts'
        event_row_version: {type: integer}
        draft_row_version: {type: integer}
    ErpAnalysisStatus:
      type: object
      additionalProperties: false
      required: [analysis_id, status, expires_at, validation_summary, ready_to_commit]
      properties:
        analysis_id: {type: string, format: uuid}
        status:
          type: string
          enum: [PREVIEWED, CONSUMED, EXPIRED]
        expires_at: {type: string, format: date-time}
        consumed_at: {type: string, format: date-time}
        validation_summary:
          type: object
          additionalProperties: true
        ready_to_commit: {type: boolean}
    ErpIntegrationLimits:
      type: object
      additionalProperties: false
      required: [max_body_bytes, max_json_depth, max_lots]
      properties:
        max_body_bytes: {type: integer}
        max_json_depth: {type: integer}
        max_lots: {type: integer}
    ErpIntegrationCapabilities:
      type: object
      additionalProperties: false
      required: [schema_version, limits, supported_mapping_types, deferred_fields]
      properties:
        schema_version:
          type: string
          enum: [BINTRANS_RFX_ERP_JSON_V1]
        limits:
          $ref: '#/components/schemas/ErpIntegrationLimits'
        supported_mapping_types:
          type: array
          items: {type: string}
        deferred_fields:
          type: array
          items: {type: string}
"""


def filter_e7_excel_exchange_endpoints(
    endpoints: list[tuple[str, str, str, str, bool, bool, str | None]],
    include_e7_excel_exchange: bool,
) -> list[tuple[str, str, str, str, bool, bool, str | None]]:
    if include_e7_excel_exchange:
        return endpoints
    return [item for item in endpoints if item[6] not in E7_EXCEL_EXCHANGE_ENDPOINT_PROFILES]


def filter_e7_erp_preview_endpoints(
    endpoints: list[tuple[str, str, str, str, bool, bool, str | None]],
    include_e7_erp_preview: bool,
) -> list[tuple[str, str, str, str, bool, bool, str | None]]:
    if include_e7_erp_preview:
        return endpoints
    excluded = E7_ERP_PREVIEW_ENDPOINT_PROFILES | E7_ERP_CREATE_COMMIT_ENDPOINT_PROFILES | E7_ERP_UPDATE_COMMIT_ENDPOINT_PROFILES | E7_ERP_READ_ENDPOINT_PROFILES
    return [item for item in endpoints if item[6] not in excluded]


def oauth_integration_components_block() -> str:
    return """
    OAuthIntegrationTokenResponse:
      type: object
      required: [access_token, token_type, expires_in]
      properties:
        access_token:
          type: string
        token_type:
          type: string
          enum: [Bearer]
        expires_in:
          type: integer
          format: int64
        scope:
          type: string
    OAuthIntegrationErrorResponse:
      type: object
      required: [error]
      properties:
        error:
          type: string
        error_description:
          type: string
"""


def global_components_block(
    *,
    include_e1_version_lifecycle: bool = False,
    include_e4_template_library: bool = False,
    include_e7_late_submission: bool = False,
    include_e7_excel_exchange: bool = False,
    include_e7_erp_preview: bool = False,
    include_oauth_integration: bool = False,
) -> str:
    rfx_components = (
        questionnaire_components_block(
            include_e1_version_lifecycle=include_e1_version_lifecycle,
            include_e4_template_library=include_e4_template_library,
        )
        + carrier_components_block()
    )
    if include_e7_late_submission:
        rfx_components += late_submission_components_block()
    if include_e7_excel_exchange:
        rfx_components += excel_exchange_components_block()
    if include_e7_erp_preview:
        rfx_components += erp_preview_components_block()
    if include_oauth_integration:
        rfx_components += oauth_integration_components_block()
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
    CompatibilityReason:
      type: object
      required: [reason_code, dimension]
      properties:
        reason_code:
          type: string
        dimension:
          type: string
        rule_code:
          type: string
        rule_set_id:
          type: string
          format: uuid
        rule_set_scope:
          type: string
          enum: [SYSTEM, TENANT]
        rule_set_version:
          type: integer
        required_separation:
          type: string
          description: Structural separation condition. It is not a claim that legislation is satisfied.
        source_reference:
          type: string
    CompatibilityRuleSetRef:
      type: object
      required: [id, scope, version]
      description: Authoritative rule-set identity. Integer rule_set_versions are a legacy summary.
      properties:
        id:
          type: string
          format: uuid
        scope:
          type: string
          enum: [SYSTEM, TENANT]
        tenant_id:
          type: string
          format: uuid
          nullable: true
        version:
          type: integer
          minimum: 1
    CompatibilityCatalogVersionRef:
      type: object
      required: [id, catalog_kind, scope, version]
      description: Authoritative catalog version identity. Integer catalog_versions are a legacy system summary.
      properties:
        id:
          type: string
          format: uuid
        catalog_kind:
          type: string
        scope:
          type: string
          enum: [SYSTEM, TENANT]
        tenant_id:
          type: string
          format: uuid
          nullable: true
        version:
          type: integer
          minimum: 1
    NextLoadSearchRequest:
      type: object
      required: [capacity_id]
      additionalProperties: false
      properties:
        capacity_id: {type: string, format: uuid}
        candidate_limit: {type: integer, minimum: 0}
        policy:
          $ref: '#/components/schemas/NextLoadSearchPolicy'
    NextLoadSearchPolicy:
      type: object
      description: Request policy may tighten hard maxima. It cannot widen them. radius_km is the RADIUS prefilter and is not max_deadhead_km.
      properties:
        search_mode: {type: string, enum: [RADIUS, DIRECTIONAL_CORRIDOR, ROUTE_ELLIPSE]}
        target_location_id: {type: string, format: uuid}
        forward_search_km: {type: number, minimum: 0}
        corridor_deviation_km: {type: number, minimum: 0}
        max_deadhead_km: {type: number, minimum: 0}
        preferred_deadhead_km: {type: number, minimum: 0}
        max_deadhead_minutes: {type: number, minimum: 0}
        min_loaded_distance_km: {type: number, minimum: 0}
        max_route_increase_km: {type: number, minimum: 0}
        radius_km: {type: number, minimum: 0, nullable: true, description: Positive kilometres for RADIUS prefilter. Zero is rejected by the service.}
        objective_profile:
          type: string
          enum: [MIN_DEADHEAD, MAX_CAPACITY_UTILIZATION, RETURN_HOME, MAX_REVENUE, MIN_RISK, BALANCED, MAX_CONTRIBUTION]
          description: Executable system profile. MAX_CONTRIBUTION is reserved and returns OBJECTIVE_PROFILE_NOT_EXECUTABLE until a planning cost provider exists.
        ranking_currency:
          type: string
          pattern: '^[A-Z]{3}$'
          description: Ordinary policy field. Required for MAX_REVENUE. No FX conversion is performed.
    NextLoadSearchResponse:
      type: object
      required: [search_id, capacity_id, capacity_version, search_mode, effective_policy, eligible_candidate_count, ranked_candidate_count, unranked_eligible_count, returned_candidate_count, rejection_counts_by_reason, unranked_counts_by_reason, ranking, candidates]
      properties:
        search_id: {type: string, format: uuid}
        capacity_id: {type: string, format: uuid}
        capacity_version: {type: integer}
        search_mode: {type: string}
        effective_policy: {$ref: '#/components/schemas/NextLoadSearchPolicy'}
        effective_policy_fingerprint: {type: string}
        eligible_candidate_count: {type: integer}
        ranked_candidate_count: {type: integer}
        unranked_eligible_count: {type: integer}
        returned_candidate_count: {type: integer}
        rejection_counts_by_reason:
          type: object
          additionalProperties: {type: integer}
          description: Hard-feasibility reject counts. Rejected load identities are not returned.
        unranked_counts_by_reason:
          type: object
          additionalProperties: {type: integer}
          description: Eligible candidates that could not be ranked. These are not hard rejects.
        ranking:
          $ref: '#/components/schemas/NextLoadSearchRanking'
        candidates:
          type: array
          items: {$ref: '#/components/schemas/NextLoadCandidate'}
    NextLoadCandidate:
      type: object
      required: [load_opportunity_id, eligibility, load, compatibility]
      properties:
        load_opportunity_id: {type: string, format: uuid}
        load_version: {type: integer}
        eligibility: {type: string, enum: [ELIGIBLE]}
        load: {$ref: '#/components/schemas/NetworkMarketplaceLoad'}
        deadhead_bucket: {type: string, enum: ["0-25", "25-50", "50-100", "100-200", "200+"]}
        road_deadhead_km: {type: number}
        road_deadhead_minutes: {type: number}
        forward_progress_km: {type: number, description: Corridor geometry. Not road distance. Omitted for anonymized loads.}
        lateral_distance_km: {type: number, description: Corridor geometry. Not road distance. Omitted for anonymized loads.}
        route_increase_km: {type: number}
        timing: {type: string}
        waiting_minutes: {type: number}
        compatibility: {type: string, enum: [COMPATIBLE, INCOMPATIBLE, INDETERMINATE]}
        explanation: {type: array, items: {type: string}}
        rank: {type: integer, minimum: 1, description: Present for RANKED candidates. Assigned before top-N truncation.}
        score_status: {type: string, enum: [RANKED, UNRANKED]}
        score: {type: integer, minimum: 0, maximum: 10000, description: Fixed-point match score. Omitted when unranked.}
        score_evidence_bps: {type: integer, minimum: 0, maximum: 10000}
        score_components:
          type: array
          items: {$ref: '#/components/schemas/NextLoadScoreComponent'}
        unranked_reason_codes: {type: array, items: {type: string}}
    NextLoadSearchRanking:
      type: object
      required: [objective_profile, profile_version, algorithm_version, profile_fingerprint]
      properties:
        objective_profile: {type: string}
        profile_version: {type: integer, minimum: 1}
        algorithm_version: {type: string}
        profile_fingerprint: {type: string}
        ranking_currency: {type: string, pattern: '^[A-Z]{3}$'}
    NextLoadScoreComponent:
      type: object
      required: [code, status, weight_bps, reason_code, explanation]
      properties:
        code: {type: string}
        status: {type: string, enum: [AVAILABLE, UNAVAILABLE, FALLBACK]}
        raw_value: {type: number, description: Omitted when it would reveal anonymized geography or a hidden commercial amount.}
        raw_unit: {type: string}
        normalized_points: {type: integer, minimum: 0, maximum: 10000}
        weight_bps: {type: integer, minimum: 1}
        weighted_points: {type: integer}
        reason_code: {type: string}
        explanation: {type: string}
    NetworkPlace:
      type: object
      description: Owner and non-anonymized place. Exact search geography. Address lines are not part of this snapshot.
      properties:
        location_id: {type: string, format: uuid}
        label: {type: string}
        latitude: {type: number}
        longitude: {type: number}
        country_code: {type: string, minLength: 2, maxLength: 2}
        region: {type: string}
        city: {type: string}
    NetworkCoarsePlace:
      type: object
      description: Anonymized display geography. Exact coordinates, location ids, and facility labels are absent.
      properties:
        country_code: {type: string, minLength: 2, maxLength: 2}
        region: {type: string}
        city: {type: string}
    NetworkLoadOpportunity:
      type: object
      additionalProperties: true
      properties:
        id: {type: string, format: uuid}
        pickup: {$ref: '#/components/schemas/NetworkPlace'}
        delivery: {$ref: '#/components/schemas/NetworkPlace'}
        consolidation_allowed:
          type: boolean
          description: Owner-controlled same-owner consolidation opt-in. Omitted on create means false. Not a search override.
        cross_shipper_consolidation_allowed:
          type: boolean
          description: Owner-controlled cross-shipper consolidation opt-in. Independent of consolidation_allowed. Not a search override.
    ConsolidationSearchRequest:
      type: object
      required: [capacity_id, pattern]
      additionalProperties: false
      properties:
        capacity_id: {type: string, format: uuid}
        pattern:
          type: string
          enum: [SAME_ORIGIN_SAME_DESTINATION, CURRENT_TRIP_FILL]
          description: NLO-0.3B executes only SAME_ORIGIN_SAME_DESTINATION. CURRENT_TRIP_FILL returns PATTERN_NOT_IMPLEMENTED.
        candidate_limit:
          type: integer
          minimum: 0
          description: Response cap applied after evaluation. It does not change pair generation, counts, or persistence.
    ConsolidationSearchResponse:
      type: object
      required: [search_id, capacity_id, capacity_version, pattern, pool_load_count, evaluated_pair_count, feasible_candidate_count, indeterminate_candidate_count, hard_reject_candidate_count, returned_candidate_count, excluded_counts_by_reason, hard_reject_counts_by_reason, indeterminate_counts_by_reason, candidates]
      properties:
        search_id: {type: string, format: uuid}
        capacity_id: {type: string, format: uuid}
        capacity_version: {type: integer}
        pattern: {type: string, enum: [SAME_ORIGIN_SAME_DESTINATION]}
        pool_load_count: {type: integer}
        evaluated_pair_count: {type: integer}
        feasible_candidate_count: {type: integer}
        indeterminate_candidate_count: {type: integer}
        hard_reject_candidate_count: {type: integer}
        returned_candidate_count: {type: integer}
        excluded_counts_by_reason:
          type: object
          additionalProperties: {type: integer}
        hard_reject_counts_by_reason:
          type: object
          additionalProperties: {type: integer}
        indeterminate_counts_by_reason:
          type: object
          additionalProperties: {type: integer}
        candidates:
          type: array
          items: {$ref: '#/components/schemas/ConsolidationCandidate'}
    ConsolidationCandidate:
      type: object
      required: [candidate_id, status, execution_supported, placement_check, members, compatibility]
      properties:
        candidate_id: {type: string, format: uuid}
        status: {type: string, enum: [FEASIBLE, INDETERMINATE]}
        execution_supported: {type: boolean, enum: [false]}
        placement_check: {type: string, enum: [NOT_EVALUATED]}
        members:
          type: array
          minItems: 2
          maxItems: 2
          items: {$ref: '#/components/schemas/ConsolidationMember'}
        pickup_window_overlap: {$ref: '#/components/schemas/ConsolidationWindowOverlap'}
        delivery_window_overlap: {$ref: '#/components/schemas/ConsolidationWindowOverlap'}
        compatibility: {type: string, enum: [COMPATIBLE, INCOMPATIBLE, INDETERMINATE]}
        capacity_usage: {type: object, additionalProperties: true}
        conditions: {type: array, items: {type: string}}
        indeterminate_reason_codes: {type: array, items: {type: string}}
        explanation: {type: array, items: {type: string}}
    ConsolidationMember:
      type: object
      required: [ordinal, load_opportunity_id, load_version, load]
      properties:
        ordinal: {type: integer, enum: [1, 2]}
        load_opportunity_id: {type: string, format: uuid}
        load_version: {type: integer}
        load: {$ref: '#/components/schemas/NetworkMarketplaceLoad'}
    ConsolidationWindowOverlap:
      type: object
      properties:
        start: {type: string, format: date-time}
        end: {type: string, format: date-time}
    NetworkMarketplaceLoad:
      type: object
      additionalProperties: true
      description: Marketplace projection. Anonymized loads use coarse places and do not expose exact deadhead.
      properties:
        id: {type: string, format: uuid}
        visibility_scope: {type: string}
        pickup: {$ref: '#/components/schemas/NetworkCoarsePlace'}
        delivery: {$ref: '#/components/schemas/NetworkCoarsePlace'}
    NetworkCapacity:
      type: object
      additionalProperties: true
      properties:
        id: {type: string, format: uuid}
        location_id: {type: string, format: uuid, nullable: true}
        location_label: {type: string}
        latitude: {type: number, nullable: true}
        longitude: {type: number, nullable: true}
        country_code: {type: string}
        region: {type: string}
        city: {type: string}
    NetworkMarketplaceCapacity:
      type: object
      additionalProperties: true
      description: Anonymized capacity display omits location_id, coordinates, and facility labels.
      properties:
        id: {type: string, format: uuid}
        country_code: {type: string}
        region: {type: string}
        city: {type: string}
    CompatibilityEvaluation:
      type: object
      required: [status, rule_sets_used, catalog_versions_used, fingerprint]
      properties:
        status:
          type: string
          enum: [COMPATIBLE, INCOMPATIBLE, INDETERMINATE]
        hard_rejects:
          type: array
          items:
            $ref: '#/components/schemas/CompatibilityReason'
        indeterminate_reasons:
          type: array
          items:
            $ref: '#/components/schemas/CompatibilityReason'
        conditions:
          type: array
          items:
            $ref: '#/components/schemas/CompatibilityReason'
        warnings:
          type: array
          items:
            $ref: '#/components/schemas/CompatibilityReason'
        rule_set_versions:
          type: array
          description: Legacy integer versions. rule_sets_used is authoritative.
          items:
            type: integer
        rule_sets_used:
          type: array
          items:
            $ref: '#/components/schemas/CompatibilityRuleSetRef'
        catalog_versions:
          type: object
          description: Legacy system catalog version numbers keyed by kind. catalog_versions_used is authoritative.
          additionalProperties:
            type: integer
        catalog_versions_used:
          type: array
          items:
            $ref: '#/components/schemas/CompatibilityCatalogVersionRef'
        fingerprint:
          type: string
    CompatibilityRuleDraft:
      type: object
      required: [rule_code, rule_kind, decision, reason_code, left_selector_type, right_selector_type]
      properties:
        rule_code:
          type: string
        rule_kind:
          type: string
          enum: [CARGO_CARGO, CARGO_EQUIPMENT]
        decision:
          type: string
          enum: [ALLOW, DENY, REQUIRE_SEPARATION, REQUIRE_CONDITION]
        reason_code:
          type: string
        severity:
          type: string
          enum: [HARD, SOFT]
          default: HARD
        required_separation:
          type: string
          description: Required when decision is REQUIRE_SEPARATION. Absent for every other decision.
        left_selector_type:
          type: string
        left_selector_value:
          type: string
        right_selector_type:
          type: string
        right_selector_value:
          type: string
        priority:
          type: integer
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
                - UNPROCESSABLE_ENTITY
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
""" + rfx_components + """    HealthResponse:
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


def components_block(
    *,
    include_payment_components: bool = False,
    include_e1_version_lifecycle: bool = False,
    include_e4_template_library: bool = False,
    include_e7_late_submission: bool = False,
    include_e7_excel_exchange: bool = False,
    include_e7_erp_preview: bool = False,
    include_oauth_integration: bool = False,
) -> str:
    block = global_components_block(
        include_e1_version_lifecycle=include_e1_version_lifecycle,
        include_e4_template_library=include_e4_template_library,
        include_e7_late_submission=include_e7_late_submission,
        include_e7_excel_exchange=include_e7_excel_exchange,
        include_e7_erp_preview=include_e7_erp_preview,
        include_oauth_integration=include_oauth_integration,
    )
    if include_payment_components:
        block = block.rstrip() + "\n" + payment_components_block()
    return block


def build_spec(
    title_suffix: str,
    description: str,
    endpoints: list[tuple[str, str, str, str, bool, bool, str | None]],
    *,
    include_payment_components: bool = False,
    include_e1_version_lifecycle: bool = False,
    include_e4_template_library: bool = False,
    include_e7_late_submission: bool = False,
    include_e7_excel_exchange: bool = False,
    include_e7_erp_preview: bool = False,
    include_oauth_integration: bool = False,
) -> str:
    endpoints = filter_e7_excel_exchange_endpoints(endpoints, include_e7_excel_exchange)
    endpoints = filter_e7_erp_preview_endpoints(endpoints, include_e7_erp_preview)
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
{components_block(include_payment_components=include_payment_components, include_e1_version_lifecycle=include_e1_version_lifecycle, include_e4_template_library=include_e4_template_library, include_e7_late_submission=include_e7_late_submission, include_e7_excel_exchange=include_e7_excel_exchange, include_e7_erp_preview=include_e7_erp_preview, include_oauth_integration=include_oauth_integration)}
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
    "network-optimizer-service.yaml": "Network Optimizer Service",
}


def main() -> None:
    OPENAPI_DIR.mkdir(parents=True, exist_ok=True)
    (OPENAPI_DIR / "schemas").mkdir(exist_ok=True)

    unified = build_spec(
        "",
        "Unified HTTP API for the Freight Platform exposed via api-gateway.",
        ENDPOINTS,
        include_payment_components=True,
        include_e1_version_lifecycle=True,
        include_e4_template_library=True,
        include_e7_late_submission=True,
        include_e7_excel_exchange=True,
        include_e7_erp_preview=True,
        include_oauth_integration=True,
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
            include_e1_version_lifecycle=(filename == "rfx-service.yaml"),
            include_e4_template_library=(filename == "rfx-service.yaml"),
            include_e7_late_submission=(filename == "rfx-service.yaml"),
            include_e7_excel_exchange=(filename == "rfx-service.yaml"),
            include_e7_erp_preview=(filename == "rfx-service.yaml"),
            include_oauth_integration=(filename == "identity-service.yaml"),
        )
        (OPENAPI_DIR / filename).write_text(spec, encoding="utf-8")

    print(f"Generated OpenAPI specs in {OPENAPI_DIR}")


if __name__ == "__main__":
    main()

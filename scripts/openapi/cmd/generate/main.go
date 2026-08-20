// One-shot OpenAPI generator (mirrors generate_openapi.py for environments without Python).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type endpoint struct {
	path, method, summary, tag string
	withHeaders, secured       bool
	profile                    string
}

var tags = []string{
	"Gateway", "Auth", "Users", "Roles", "Companies", "Memberships",
	"Locations", "Cargoes", "Transport Orders", "RFx", "Freight Requests",
	"Bids", "Shipments", "Drivers", "Vehicles", "Documents", "Signing",
	"Billing Registers", "Closing Documents", "Payment Obligations", "Payments",
}

var endpoints = []endpoint{
	{"/health", "get", "Gateway health check", "Gateway", false, false, ""},
	{"/ready", "get", "Readiness check for gateway and downstream services", "Gateway", false, false, ""},
	{"/routes", "get", "List gateway route map", "Gateway", false, false, ""},
	{"/openapi", "get", "List available OpenAPI documents", "Gateway", false, false, ""},
	{"/openapi.yaml", "get", "Unified OpenAPI YAML document", "Gateway", false, false, ""},
	{"/openapi.json", "get", "Unified OpenAPI JSON document", "Gateway", false, false, ""},
	{"/docs", "get", "Swagger UI", "Gateway", false, false, ""},
	{"/api/v1/auth/login", "post", "Login and obtain JWT access token", "Auth", false, false, ""},
	{"/api/v1/auth/me", "get", "Get current authenticated user", "Auth", true, true, ""},
	{"/api/v1/users", "post", "Create user", "Users", false, false, ""},
	{"/api/v1/users", "get", "List users", "Users", true, true, ""},
	{"/api/v1/users/{id}", "get", "Get user by ID", "Users", true, true, ""},
	{"/api/v1/users/{id}", "patch", "Update user", "Users", true, true, ""},
	{"/api/v1/users/{id}", "delete", "Delete user", "Users", true, true, ""},
	{"/api/v1/users/{user_id}/companies", "get", "List companies for user", "Users", true, true, ""},
	{"/api/v1/users/{user_id}/companies/{company_id}/roles", "post", "Assign role to user in company", "Roles", true, true, ""},
	{"/api/v1/companies", "post", "Create company", "Companies", true, true, ""},
	{"/api/v1/companies", "get", "List companies", "Companies", true, true, ""},
	{"/api/v1/companies/{id}", "get", "Get company by ID", "Companies", true, true, ""},
	{"/api/v1/companies/{id}", "patch", "Update company", "Companies", true, true, ""},
	{"/api/v1/companies/{id}", "delete", "Delete company", "Companies", true, true, ""},
	{"/api/v1/companies/{company_id}/members", "post", "Add company member", "Memberships", true, true, ""},
	{"/api/v1/companies/{company_id}/members", "get", "List company members", "Memberships", true, true, ""},
	{"/api/v1/locations", "post", "Create location", "Locations", true, true, ""},
	{"/api/v1/locations", "get", "List locations", "Locations", true, true, ""},
	{"/api/v1/locations/{id}", "get", "Get location by ID", "Locations", true, true, ""},
	{"/api/v1/cargoes", "post", "Create cargo", "Cargoes", true, true, ""},
	{"/api/v1/cargoes/{id}", "get", "Get cargo by ID", "Cargoes", true, true, ""},
	{"/api/v1/transport-orders", "post", "Create transport order", "Transport Orders", true, true, ""},
	{"/api/v1/transport-orders", "get", "List transport orders", "Transport Orders", true, true, ""},
	{"/api/v1/transport-orders/{id}", "get", "Get transport order by ID", "Transport Orders", true, true, ""},
	{"/api/v1/transport-orders/{id}", "patch", "Update transport order", "Transport Orders", true, true, ""},
	{"/api/v1/transport-orders/{id}/submit", "post", "Submit transport order", "Transport Orders", true, true, ""},
	{"/api/v1/transport-orders/{id}/cancel", "post", "Cancel transport order", "Transport Orders", true, true, ""},
	{"/api/v1/rfx-events", "post", "Create RFx event", "RFx", true, true, ""},
	{"/api/v1/rfx-events", "get", "List RFx events", "RFx", true, true, ""},
	{"/api/v1/rfx-events/{id}", "get", "Get RFx event by ID", "RFx", true, true, ""},
	{"/api/v1/rfx-events/{id}", "patch", "Update RFx event", "RFx", true, true, ""},
	{"/api/v1/rfx-events/{id}/publish", "post", "Publish RFx event", "RFx", true, true, ""},
	{"/api/v1/rfx-events/{id}/cancel", "post", "Cancel RFx event", "RFx", true, true, ""},
	{"/api/v1/rfx-events/{id}/participants", "post", "Add RFx participant", "RFx", true, true, ""},
	{"/api/v1/rfx-events/{id}/participants", "get", "List RFx participants", "RFx", true, true, ""},
	{"/api/v1/freight-requests/from-transport-order", "post", "Create freight request from transport order", "Freight Requests", true, true, ""},
	{"/api/v1/freight-requests", "get", "List freight requests", "Freight Requests", true, true, ""},
	{"/api/v1/freight-requests/{id}", "get", "Get freight request by ID", "Freight Requests", true, true, ""},
	{"/api/v1/freight-requests/{id}/publish", "post", "Publish freight request", "Freight Requests", true, true, ""},
	{"/api/v1/freight-requests/{id}/bids", "post", "Create bid for freight request", "Bids", true, true, ""},
	{"/api/v1/freight-requests/{id}/bids", "get", "List bids for freight request", "Freight Requests", true, true, ""},
	{"/api/v1/bids/{id}/submit", "post", "Submit bid", "Bids", true, true, ""},
	{"/api/v1/bids/{id}/accept", "post", "Accept bid", "Bids", true, true, ""},
	{"/api/v1/shipments/from-transport-order", "post", "Create shipment from transport order", "Shipments", true, true, ""},
	{"/api/v1/shipments/from-bid", "post", "Create shipment from accepted bid", "Shipments", true, true, ""},
	{"/api/v1/shipments", "get", "List shipments", "Shipments", true, true, ""},
	{"/api/v1/shipments/{id}", "get", "Get shipment by ID", "Shipments", true, true, ""},
	{"/api/v1/shipments/{id}/assign-driver", "post", "Assign driver to shipment", "Shipments", true, true, ""},
	{"/api/v1/shipments/{id}/assign-vehicle", "post", "Assign vehicle to shipment", "Shipments", true, true, ""},
	{"/api/v1/shipments/{id}/accept", "post", "Accept shipment", "Shipments", true, true, ""},
	{"/api/v1/shipments/{id}/status", "patch", "Update shipment status", "Shipments", true, true, ""},
	{"/api/v1/shipments/{id}/cancel", "post", "Cancel shipment", "Shipments", true, true, ""},
	{"/api/v1/drivers", "post", "Create driver", "Drivers", true, true, ""},
	{"/api/v1/drivers", "get", "List drivers", "Drivers", true, true, ""},
	{"/api/v1/drivers/{id}", "get", "Get driver by ID", "Drivers", true, true, ""},
	{"/api/v1/vehicles", "post", "Create vehicle", "Vehicles", true, true, ""},
	{"/api/v1/vehicles", "get", "List vehicles", "Vehicles", true, true, ""},
	{"/api/v1/vehicles/{id}", "get", "Get vehicle by ID", "Vehicles", true, true, ""},
	{"/api/v1/documents", "post", "Create document", "Documents", true, true, ""},
	{"/api/v1/documents", "get", "List documents", "Documents", true, true, ""},
	{"/api/v1/documents/{id}", "get", "Get document by ID", "Documents", true, true, ""},
	{"/api/v1/documents/{id}/versions", "post", "Create document version", "Documents", true, true, ""},
	{"/api/v1/documents/{id}/files", "post", "Add document file metadata", "Documents", true, true, ""},
	{"/api/v1/documents/{id}/ready-for-signing", "post", "Move document to ready for signing", "Documents", true, true, ""},
	{"/api/v1/documents/{id}/signing-sessions", "post", "Create signing session", "Signing", true, true, ""},
	{"/api/v1/documents/{id}/cancel", "post", "Cancel document", "Documents", true, true, ""},
	{"/api/v1/documents/{id}/archive", "post", "Archive document", "Documents", true, true, ""},
	{"/api/v1/signing-sessions/{id}", "get", "Get signing session", "Signing", true, true, ""},
	{"/api/v1/signing-sessions/{id}/signatures", "post", "Add mock signature", "Signing", true, true, ""},
	{"/api/v1/billing-registers", "post", "Create billing register", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers", "get", "List billing registers", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}", "get", "Get billing register by ID", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/items", "post", "Add shipment item to billing register", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/items", "get", "List billing register items", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{register_id}/items/{item_id}", "delete", "Delete billing register item", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/calculate", "post", "Calculate billing register totals", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/approve", "post", "Approve billing register", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/closing-document-package", "post", "Create closing document package", "Closing Documents", true, true, ""},
	{"/api/v1/billing-registers/{id}/invoices", "post", "Create invoice", "Closing Documents", true, true, ""},
	{"/api/v1/billing-registers/{id}/acts", "post", "Create act", "Closing Documents", true, true, ""},
	{"/api/v1/billing-registers/{id}/vat-invoices", "post", "Create VAT invoice", "Closing Documents", true, true, ""},
	{"/api/v1/billing-registers/{id}/upd", "post", "Create UPD document", "Closing Documents", true, true, ""},
	{"/api/v1/billing-registers/{id}/mark-sent-to-edo", "post", "Mark billing register sent to EDO (mock)", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/mark-signed", "post", "Mark billing register signed (mock)", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/mark-paid", "post", "Mark billing register paid", "Billing Registers", true, true, ""},
	{"/api/v1/billing-registers/{id}/close", "post", "Close billing register", "Billing Registers", true, true, ""},
	{"/api/v1/payment-obligations", "get", "List payment obligations", "Payment Obligations", true, true, ""},
	{"/api/v1/payment-obligations/{id}", "get", "Get payment obligation by ID", "Payment Obligations", true, true, ""},
	{"/api/v1/payment-obligations/{id}/due-date", "patch", "Update payment obligation due date", "Payment Obligations", true, true, ""},
	{"/api/v1/payments", "post", "Create manual payment", "Payments", true, true, ""},
	{"/api/v1/payments", "get", "List payments", "Payments", true, true, "payment_list"},
	{"/api/v1/payments/{id}", "get", "Get payment by ID", "Payments", true, true, ""},
	{"/api/v1/payments/{id}/allocations", "get", "List payment allocations", "Payments", true, true, "payment_allocations_list"},
	{"/api/v1/payments/{id}/allocations", "post", "Allocate payment to obligation", "Payments", true, true, ""},
	{"/api/v1/payments/{id}/audit-events", "get", "List payment audit events", "Payments", true, true, "payment_audit_list"},
	{"/api/v1/payments/{id}/eligible-obligations", "get", "List eligible obligations for payment", "Payments", true, true, "payment_eligible_obligations_list"},
	{"/api/v1/payments/{id}/reconcile", "post", "Reconcile fully allocated payment", "Payments", true, true, "reconcile_payment"},
	{"/api/v1/payment-allocations/{id}/void", "post", "Void payment allocation", "Payments", true, true, "void_allocation"},
	{"/api/v1/payments/{id}/void", "post", "Void payment", "Payments", true, true, "void_payment"},
}

var serviceTags = map[string][]string{
	"identity-service.yaml":         {"Auth", "Users", "Roles"},
	"company-service.yaml":          {"Companies", "Memberships"},
	"transport-order-service.yaml":  {"Locations", "Cargoes", "Transport Orders"},
	"rfx-service.yaml":              {"RFx", "Freight Requests", "Bids"},
	"shipment-service.yaml":         {"Shipments", "Drivers", "Vehicles"},
	"document-service.yaml":         {"Documents", "Signing"},
	"billing-register-service.yaml": {"Billing Registers", "Closing Documents"},
	"payment-service.yaml":          {"Payment Obligations", "Payments"},
}

var pathParamPattern = regexp.MustCompile(`\{([^}]+)\}`)

var reconcileDescriptions = map[string]string{
	"reconcile_payment": "Reconciles a payment after canonical financial confirmation.\nRequires FULLY_ALLOCATED status with active allocations recomputed from the database.\nExact equality is required between payment amount, stored allocated amount, and active allocation sum.\nRepeat reconciliation is idempotent. Ordinary post-reconcile mutations are forbidden.",
}

var noRequestBodyProfiles = map[string]struct{}{
	"reconcile_payment": {},
}

var readResponseSchemas = map[string]string{
	"payment_list":                       "PaymentListResponse",
	"payment_allocations_list":           "PaymentAllocationListResponse",
	"payment_audit_list":                 "PaymentAuditEventListResponse",
	"payment_eligible_obligations_list": "EligiblePaymentObligationListResponse",
}

var paymentDetailListQueryProfiles = map[string]struct{}{
	"payment_allocations_list":           {},
	"payment_audit_list":                 {},
	"payment_eligible_obligations_list": {},
}

const companyIDQueryDescription = "Active company context requested by the authenticated user. The gateway validates membership and derives trusted internal company/actor context."

var voidDescriptions = map[string]string{
	"void_allocation": "Voids an active allocation and recomputes payment/obligation balances from remaining active allocations.\nAppend-only reversal. PAID obligation reversal is forbidden. RECONCILED payment mutation is forbidden.\nRepeat void is idempotent. Actor and tenant context are derived from verified request context.",
	"void_payment":    "Voids a RECEIVED payment with zero active allocations.\nRECONCILED and partially or fully allocated payments cannot be voided.\nRepeat void is idempotent. Actor and tenant context are derived from verified request context.",
}

func main() {
	root := findRoot()
	outDir := filepath.Join(root, "packages", "openapi")
	if err := os.MkdirAll(filepath.Join(outDir, "schemas"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	unified := buildSpec("", "Unified HTTP API for the Freight Platform exposed via api-gateway.", endpoints, true)
	if err := os.WriteFile(filepath.Join(outDir, "openapi.yaml"), []byte(unified), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write openapi.yaml: %v\n", err)
		os.Exit(1)
	}

	for filename, tagSet := range serviceTags {
		filtered := filterByTags(endpoints, tagSet)
		displayName := serviceDisplayName(filename)
		includePayment := filename == "payment-service.yaml"
		spec := buildSpec(" - "+displayName, "OpenAPI specification for "+displayName+".", filtered, includePayment)
		if err := os.WriteFile(filepath.Join(outDir, filename), []byte(spec), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Generated OpenAPI specs in %s\n", outDir)
}

func serviceDisplayName(filename string) string {
	names := map[string]string{
		"identity-service.yaml":         "Identity Service",
		"company-service.yaml":          "Company Service",
		"transport-order-service.yaml":  "Transport Order Service",
		"rfx-service.yaml":              "RFx Service",
		"shipment-service.yaml":         "Shipment Service",
		"document-service.yaml":         "Document Service",
		"billing-register-service.yaml": "Billing Register Service",
		"payment-service.yaml":          "Payment Service",
	}
	if name, ok := names[filename]; ok {
		return name
	}
	return filename
}

func findRoot() string {
	wd, _ := os.Getwd()
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "packages", "openapi")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return wd
		}
	}
}

func filterByTags(all []endpoint, tagSet []string) []endpoint {
	set := make(map[string]struct{}, len(tagSet))
	for _, t := range tagSet {
		set[t] = struct{}{}
	}
	var out []endpoint
	for _, e := range all {
		if _, ok := set[e.tag]; ok {
			out = append(out, e)
		}
	}
	return out
}

func pathToID(summary string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(summary) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func companyIDQueryLines() []string {
	return []string{
		"        - name: company_id",
		"          in: query",
		"          required: true",
		"          schema:",
		"            type: string",
		"            format: uuid",
		"          description: " + companyIDQueryDescription,
	}
}

func paginationQueryLines() []string {
	return []string{
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
	}
}

func paymentListFilterQueryLines() []string {
	return []string{
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
	}
}

func queryParameterLines(profile string) []string {
	switch profile {
	case "payment_list":
		lines := companyIDQueryLines()
		lines = append(lines, paymentListFilterQueryLines()...)
		return append(lines, paginationQueryLines()...)
	default:
		if _, ok := paymentDetailListQueryProfiles[profile]; ok {
			lines := companyIDQueryLines()
			return append(lines, paginationQueryLines()...)
		}
	}
	return nil
}

func renderParameters(path string, withHeaders bool, profile string) string {
	queryLines := queryParameterLines(profile)
	pathParams := pathParamPattern.FindAllStringSubmatch(path, -1)
	if !withHeaders && len(queryLines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("      parameters:\n")
	if withHeaders {
		sb.WriteString("        - $ref: '#/components/parameters/XRequestID'\n")
		sb.WriteString("        - $ref: '#/components/parameters/XTenantID'\n")
		sb.WriteString("        - $ref: '#/components/parameters/XCompanyID'\n")
		sb.WriteString("        - $ref: '#/components/parameters/XLocale'\n")
		sb.WriteString("        - $ref: '#/components/parameters/Authorization'\n")
	}
	for _, match := range pathParams {
		sb.WriteString(fmt.Sprintf("        - name: %s\n", match[1]))
		sb.WriteString("          in: path\n")
		sb.WriteString("          required: true\n")
		sb.WriteString("          schema:\n")
		sb.WriteString("            type: string\n")
		sb.WriteString("            format: uuid\n")
	}
	for _, line := range queryLines {
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func renderPathParameters(path string, withHeaders bool) string {
	if !withHeaders {
		return ""
	}
	params := pathParamPattern.FindAllStringSubmatch(path, -1)
	if len(params) == 0 {
		return commonHeaders
	}
	var sb strings.Builder
	sb.WriteString("      parameters:\n")
	sb.WriteString("        - $ref: '#/components/parameters/XRequestID'\n")
	sb.WriteString("        - $ref: '#/components/parameters/XTenantID'\n")
	sb.WriteString("        - $ref: '#/components/parameters/XCompanyID'\n")
	sb.WriteString("        - $ref: '#/components/parameters/XLocale'\n")
	sb.WriteString("        - $ref: '#/components/parameters/Authorization'\n")
	for _, match := range params {
		sb.WriteString(fmt.Sprintf("        - name: %s\n", match[1]))
		sb.WriteString("          in: path\n")
		sb.WriteString("          required: true\n")
		sb.WriteString("          schema:\n")
		sb.WriteString("            type: string\n")
		sb.WriteString("            format: uuid\n")
	}
	return sb.String()
}

func renderOperation(path string, e endpoint) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    %s:\n", e.method))
	sb.WriteString(fmt.Sprintf("      tags: [%s]\n", e.tag))
	sb.WriteString(fmt.Sprintf("      summary: %s\n", e.summary))
	sb.WriteString(fmt.Sprintf("      operationId: %s_%s\n", e.method, pathToID(e.summary)))
	if e.profile != "" {
		var desc string
		var ok bool
		if desc, ok = voidDescriptions[e.profile]; !ok {
			desc, ok = reconcileDescriptions[e.profile]
		}
		if ok {
			sb.WriteString("      description: |\n")
			for _, line := range strings.Split(desc, "\n") {
				sb.WriteString("        " + line + "\n")
			}
		}
	}
	if e.withHeaders || len(queryParameterLines(e.profile)) > 0 {
		params := renderParameters(path, e.withHeaders, e.profile)
		if params != "" {
			sb.WriteString(params)
		}
	}
	_, skipBody := noRequestBodyProfiles[e.profile]
	if (e.method == "post" || e.method == "patch" || e.method == "put") && !skipBody {
		sb.WriteString("      requestBody:\n")
		sb.WriteString("        required: true\n")
		sb.WriteString("        content:\n")
		sb.WriteString("          application/json:\n")
		sb.WriteString("            schema:\n")
		if _, isVoid := voidDescriptions[e.profile]; isVoid {
			sb.WriteString("              $ref: '#/components/schemas/VoidRequest'\n")
		} else {
			sb.WriteString("              type: object\n")
			sb.WriteString("              additionalProperties: true\n")
		}
	}
	if e.secured {
		sb.WriteString(securityBearer)
	}
	successCode := "200"
	successDesc := "Successful response"
	if e.profile == "void_allocation" {
		successDesc = "Allocation voided or idempotent success"
	} else if e.profile == "void_payment" {
		successDesc = "Payment voided or idempotent success"
	} else if e.profile == "reconcile_payment" {
		successDesc = "Payment reconciled or idempotent success"
	} else if e.method == "post" && e.tag != "Gateway" && e.tag != "Auth" {
		successCode = "201"
	}
	if schema, ok := readResponseSchemas[e.profile]; ok {
		sb.WriteString(fmt.Sprintf("      responses:\n        '%s':\n          description: %s\n          content:\n            application/json:\n              schema:\n                $ref: '#/components/schemas/%s'\n", successCode, successDesc, schema))
	} else {
		sb.WriteString(fmt.Sprintf("      responses:\n        '%s':\n          description: %s\n          content:\n            application/json:\n              schema:\n                type: object\n                additionalProperties: true\n", successCode, successDesc))
	}
	sb.WriteString(errorResponses)
	return sb.String()
}

func renderPaths(eps []endpoint) string {
	grouped := make(map[string][]string)
	order := make([]string, 0)
	for _, e := range eps {
		if _, ok := grouped[e.path]; !ok {
			order = append(order, e.path)
		}
		grouped[e.path] = append(grouped[e.path], renderOperation(e.path, e))
	}
	var sb strings.Builder
	for _, path := range order {
		sb.WriteString(fmt.Sprintf("  %s:\n", path))
		for _, op := range grouped[path] {
			sb.WriteString(op)
		}
	}
	return sb.String()
}

func buildSpec(titleSuffix, description string, eps []endpoint, includePaymentComponents bool) string {
	var tagsYAML strings.Builder
	for _, tag := range tags {
		tagsYAML.WriteString(fmt.Sprintf("  - name: %s\n", tag))
	}
	return fmt.Sprintf(`openapi: 3.0.3
info:
  title: Freight Platform API%s
  version: 0.1.0
  description: |
    %s
servers:
  - url: http://localhost:8080
    description: Local API Gateway
tags:
%s
paths:
%s
%s
`, titleSuffix, description, tagsYAML.String(), renderPaths(eps), componentsBlock(includePaymentComponents))
}

const commonHeaders = `      parameters:
        - $ref: '#/components/parameters/XRequestID'
        - $ref: '#/components/parameters/XTenantID'
        - $ref: '#/components/parameters/XCompanyID'
        - $ref: '#/components/parameters/XLocale'
        - $ref: '#/components/parameters/Authorization'
`

const securityBearer = `      security:
        - bearerAuth: []
`

const errorResponses = `        '400':
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
`

const globalComponentsBlock = `components:
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
`

const paymentComponentsBlock = `    PaymentRecord:
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
`

func componentsBlock(includePaymentComponents bool) string {
	if includePaymentComponents {
		return strings.TrimRight(globalComponentsBlock, "\n") + "\n" + paymentComponentsBlock
	}
	return globalComponentsBlock
}

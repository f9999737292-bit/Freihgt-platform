// One-shot OpenAPI generator (mirrors generate_openapi.py for environments without Python).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type endpoint struct {
	path, method, summary, tag string
	withHeaders, secured       bool
}

var tags = []string{
	"Gateway", "Auth", "Users", "Roles", "Companies", "Memberships",
	"Locations", "Cargoes", "Transport Orders", "RFx", "Freight Requests",
	"Bids", "Shipments", "Drivers", "Vehicles", "Documents", "Signing",
	"Billing Registers", "Closing Documents",
}

var endpoints = []endpoint{
	{"/health", "get", "Gateway health check", "Gateway", false, false},
	{"/ready", "get", "Readiness check for gateway and downstream services", "Gateway", false, false},
	{"/routes", "get", "List gateway route map", "Gateway", false, false},
	{"/openapi", "get", "List available OpenAPI documents", "Gateway", false, false},
	{"/openapi.yaml", "get", "Unified OpenAPI YAML document", "Gateway", false, false},
	{"/openapi.json", "get", "Unified OpenAPI JSON document", "Gateway", false, false},
	{"/docs", "get", "Swagger UI", "Gateway", false, false},
	{"/api/v1/auth/login", "post", "Login and obtain JWT access token", "Auth", false, false},
	{"/api/v1/auth/me", "get", "Get current authenticated user", "Auth", true, true},
	{"/api/v1/users", "post", "Create user", "Users", false, false},
	{"/api/v1/users", "get", "List users", "Users", true, true},
	{"/api/v1/users/{id}", "get", "Get user by ID", "Users", true, true},
	{"/api/v1/users/{id}", "patch", "Update user", "Users", true, true},
	{"/api/v1/users/{id}", "delete", "Delete user", "Users", true, true},
	{"/api/v1/users/{user_id}/companies", "get", "List companies for user", "Users", true, true},
	{"/api/v1/users/{user_id}/companies/{company_id}/roles", "post", "Assign role to user in company", "Roles", true, true},
	{"/api/v1/companies", "post", "Create company", "Companies", true, true},
	{"/api/v1/companies", "get", "List companies", "Companies", true, true},
	{"/api/v1/companies/{id}", "get", "Get company by ID", "Companies", true, true},
	{"/api/v1/companies/{id}", "patch", "Update company", "Companies", true, true},
	{"/api/v1/companies/{id}", "delete", "Delete company", "Companies", true, true},
	{"/api/v1/companies/{company_id}/members", "post", "Add company member", "Memberships", true, true},
	{"/api/v1/companies/{company_id}/members", "get", "List company members", "Memberships", true, true},
	{"/api/v1/locations", "post", "Create location", "Locations", true, true},
	{"/api/v1/locations", "get", "List locations", "Locations", true, true},
	{"/api/v1/locations/{id}", "get", "Get location by ID", "Locations", true, true},
	{"/api/v1/cargoes", "post", "Create cargo", "Cargoes", true, true},
	{"/api/v1/cargoes/{id}", "get", "Get cargo by ID", "Cargoes", true, true},
	{"/api/v1/transport-orders", "post", "Create transport order", "Transport Orders", true, true},
	{"/api/v1/transport-orders", "get", "List transport orders", "Transport Orders", true, true},
	{"/api/v1/transport-orders/{id}", "get", "Get transport order by ID", "Transport Orders", true, true},
	{"/api/v1/transport-orders/{id}", "patch", "Update transport order", "Transport Orders", true, true},
	{"/api/v1/transport-orders/{id}/submit", "post", "Submit transport order", "Transport Orders", true, true},
	{"/api/v1/transport-orders/{id}/cancel", "post", "Cancel transport order", "Transport Orders", true, true},
	{"/api/v1/rfx-events", "post", "Create RFx event", "RFx", true, true},
	{"/api/v1/rfx-events", "get", "List RFx events", "RFx", true, true},
	{"/api/v1/rfx-events/{id}", "get", "Get RFx event by ID", "RFx", true, true},
	{"/api/v1/rfx-events/{id}", "patch", "Update RFx event", "RFx", true, true},
	{"/api/v1/rfx-events/{id}/publish", "post", "Publish RFx event", "RFx", true, true},
	{"/api/v1/rfx-events/{id}/cancel", "post", "Cancel RFx event", "RFx", true, true},
	{"/api/v1/rfx-events/{id}/participants", "post", "Add RFx participant", "RFx", true, true},
	{"/api/v1/rfx-events/{id}/participants", "get", "List RFx participants", "RFx", true, true},
	{"/api/v1/freight-requests/from-transport-order", "post", "Create freight request from transport order", "Freight Requests", true, true},
	{"/api/v1/freight-requests", "get", "List freight requests", "Freight Requests", true, true},
	{"/api/v1/freight-requests/{id}", "get", "Get freight request by ID", "Freight Requests", true, true},
	{"/api/v1/freight-requests/{id}/publish", "post", "Publish freight request", "Freight Requests", true, true},
	{"/api/v1/freight-requests/{id}/bids", "post", "Create bid for freight request", "Bids", true, true},
	{"/api/v1/freight-requests/{id}/bids", "get", "List bids for freight request", "Freight Requests", true, true},
	{"/api/v1/bids/{id}/submit", "post", "Submit bid", "Bids", true, true},
	{"/api/v1/bids/{id}/accept", "post", "Accept bid", "Bids", true, true},
	{"/api/v1/shipments/from-transport-order", "post", "Create shipment from transport order", "Shipments", true, true},
	{"/api/v1/shipments/from-bid", "post", "Create shipment from accepted bid", "Shipments", true, true},
	{"/api/v1/shipments", "get", "List shipments", "Shipments", true, true},
	{"/api/v1/shipments/{id}", "get", "Get shipment by ID", "Shipments", true, true},
	{"/api/v1/shipments/{id}/assign-driver", "post", "Assign driver to shipment", "Shipments", true, true},
	{"/api/v1/shipments/{id}/assign-vehicle", "post", "Assign vehicle to shipment", "Shipments", true, true},
	{"/api/v1/shipments/{id}/accept", "post", "Accept shipment", "Shipments", true, true},
	{"/api/v1/shipments/{id}/status", "patch", "Update shipment status", "Shipments", true, true},
	{"/api/v1/shipments/{id}/cancel", "post", "Cancel shipment", "Shipments", true, true},
	{"/api/v1/drivers", "post", "Create driver", "Drivers", true, true},
	{"/api/v1/drivers", "get", "List drivers", "Drivers", true, true},
	{"/api/v1/drivers/{id}", "get", "Get driver by ID", "Drivers", true, true},
	{"/api/v1/vehicles", "post", "Create vehicle", "Vehicles", true, true},
	{"/api/v1/vehicles", "get", "List vehicles", "Vehicles", true, true},
	{"/api/v1/vehicles/{id}", "get", "Get vehicle by ID", "Vehicles", true, true},
	{"/api/v1/documents", "post", "Create document", "Documents", true, true},
	{"/api/v1/documents", "get", "List documents", "Documents", true, true},
	{"/api/v1/documents/{id}", "get", "Get document by ID", "Documents", true, true},
	{"/api/v1/documents/{id}/versions", "post", "Create document version", "Documents", true, true},
	{"/api/v1/documents/{id}/files", "post", "Add document file metadata", "Documents", true, true},
	{"/api/v1/documents/{id}/ready-for-signing", "post", "Move document to ready for signing", "Documents", true, true},
	{"/api/v1/documents/{id}/signing-sessions", "post", "Create signing session", "Signing", true, true},
	{"/api/v1/documents/{id}/cancel", "post", "Cancel document", "Documents", true, true},
	{"/api/v1/documents/{id}/archive", "post", "Archive document", "Documents", true, true},
	{"/api/v1/signing-sessions/{id}", "get", "Get signing session", "Signing", true, true},
	{"/api/v1/signing-sessions/{id}/signatures", "post", "Add mock signature", "Signing", true, true},
	{"/api/v1/billing-registers", "post", "Create billing register", "Billing Registers", true, true},
	{"/api/v1/billing-registers", "get", "List billing registers", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}", "get", "Get billing register by ID", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/items", "post", "Add shipment item to billing register", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/items", "get", "List billing register items", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{register_id}/items/{item_id}", "delete", "Delete billing register item", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/calculate", "post", "Calculate billing register totals", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/approve", "post", "Approve billing register", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/closing-document-package", "post", "Create closing document package", "Closing Documents", true, true},
	{"/api/v1/billing-registers/{id}/invoices", "post", "Create invoice", "Closing Documents", true, true},
	{"/api/v1/billing-registers/{id}/acts", "post", "Create act", "Closing Documents", true, true},
	{"/api/v1/billing-registers/{id}/vat-invoices", "post", "Create VAT invoice", "Closing Documents", true, true},
	{"/api/v1/billing-registers/{id}/upd", "post", "Create UPD document", "Closing Documents", true, true},
	{"/api/v1/billing-registers/{id}/mark-sent-to-edo", "post", "Mark billing register sent to EDO (mock)", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/mark-signed", "post", "Mark billing register signed (mock)", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/mark-paid", "post", "Mark billing register paid", "Billing Registers", true, true},
	{"/api/v1/billing-registers/{id}/close", "post", "Close billing register", "Billing Registers", true, true},
}

var serviceTags = map[string][]string{
	"identity-service.yaml":        {"Auth", "Users", "Roles"},
	"company-service.yaml":         {"Companies", "Memberships"},
	"transport-order-service.yaml": {"Locations", "Cargoes", "Transport Orders"},
	"rfx-service.yaml":             {"RFx", "Freight Requests", "Bids"},
	"shipment-service.yaml":        {"Shipments", "Drivers", "Vehicles"},
	"document-service.yaml":        {"Documents", "Signing"},
	"billing-register-service.yaml": {"Billing Registers", "Closing Documents"},
}

func main() {
	root := findRoot()
	outDir := filepath.Join(root, "packages", "openapi")
	if err := os.MkdirAll(filepath.Join(outDir, "schemas"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	unified := buildSpec("", "Unified HTTP API for the Freight Platform exposed via api-gateway.", endpoints)
	if err := os.WriteFile(filepath.Join(outDir, "openapi.yaml"), []byte(unified), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write openapi.yaml: %v\n", err)
		os.Exit(1)
	}

	for filename, tagSet := range serviceTags {
		filtered := filterByTags(endpoints, tagSet)
		displayName := serviceDisplayName(filename)
		spec := buildSpec(" - "+displayName, "OpenAPI specification for "+displayName+".", filtered)
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

func renderOperation(method, summary, tag string, withHeaders, secured bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    %s:\n", method))
	sb.WriteString(fmt.Sprintf("      tags: [%s]\n", tag))
	sb.WriteString(fmt.Sprintf("      summary: %s\n", summary))
	sb.WriteString(fmt.Sprintf("      operationId: %s_%s\n", method, pathToID(summary)))
	if withHeaders {
		sb.WriteString(commonHeaders)
	}
	if method == "post" || method == "patch" || method == "put" {
		sb.WriteString(requestBody)
	}
	if secured {
		sb.WriteString(securityBearer)
	}
	successCode := "200"
	if method == "post" && tag != "Gateway" && tag != "Auth" {
		successCode = "201"
	}
	sb.WriteString(fmt.Sprintf("      responses:\n        '%s':\n          description: Successful response\n          content:\n            application/json:\n              schema:\n                type: object\n                additionalProperties: true\n", successCode))
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
		grouped[e.path] = append(grouped[e.path], renderOperation(e.method, e.summary, e.tag, e.withHeaders, e.secured))
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

func buildSpec(titleSuffix, description string, eps []endpoint) string {
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
`, titleSuffix, description, tagsYAML.String(), renderPaths(eps), componentsBlock)
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

const requestBody = `      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              additionalProperties: true
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

const componentsBlock = `components:
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
`

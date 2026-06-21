# OpenAPI Specifications

Unified and per-service OpenAPI 3.0.3 specs for the Freight Platform HTTP API.

## Files

| File | Description |
|------|-------------|
| `openapi.yaml` | Unified platform API (served by api-gateway) |
| `openapi.json` | JSON version of the unified spec |
| `identity-service.yaml` | Auth, users, roles |
| `company-service.yaml` | Companies, memberships |
| `transport-order-service.yaml` | Locations, cargoes, transport orders |
| `rfx-service.yaml` | RFx, freight requests, bids |
| `shipment-service.yaml` | Shipments, drivers, vehicles |
| `document-service.yaml` | Documents, signing |
| `billing-register-service.yaml` | Billing registers, closing documents |
| `schemas/` | Shared schema fragments |

## Regenerate

```bash
make openapi-check
# or
python scripts/openapi/generate_openapi.py
python scripts/openapi/yaml_to_json.py packages/openapi/openapi.yaml packages/openapi/openapi.json
```

Go fallbacks (when Python/PyYAML is unavailable):

```bash
cd scripts/openapi && go run ./cmd/generate/
cd scripts/openapi && go run ./cmd/yamltojson ../../packages/openapi/openapi.yaml ../../packages/openapi/openapi.json
```

## Served by API Gateway

When `api-gateway` is running:

- Swagger UI: http://localhost:8080/docs
- Unified YAML: http://localhost:8080/openapi.yaml
- Unified JSON: http://localhost:8080/openapi.json
- Document index: http://localhost:8080/openapi

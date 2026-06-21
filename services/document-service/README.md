# document-service

Go microservice for managing legal transport documents, versions, files, and mock electronic signing.

## Purpose

`document-service` handles document lifecycle for freight operations:

- Create and manage documents (ETRN, EPD, POD, invoices, acts, etc.)
- Maintain document versions and file metadata
- Prepare documents for signing and track signing sessions
- Mock signature collection without real EDO/EPD integration

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCUMENT_SERVICE_PORT` | `8086` | HTTP port |
| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `ENVIRONMENT` | `development` | Runtime environment |

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/documents` | Create document with first version |
| GET | `/v1/documents/{id}` | Get document with latest version and files |
| GET | `/v1/documents` | List documents |
| POST | `/v1/documents/{id}/versions` | Create new document version |
| POST | `/v1/documents/{id}/files` | Add file metadata |
| POST | `/v1/documents/{id}/ready-for-signing` | Move document to READY_FOR_SIGNING |
| POST | `/v1/documents/{id}/signing-sessions` | Create signing session |
| GET | `/v1/signing-sessions/{id}` | Get signing session |
| POST | `/v1/signing-sessions/{id}/signatures` | Add mock signature |
| POST | `/v1/documents/{id}/cancel` | Cancel document |
| POST | `/v1/documents/{id}/archive` | Archive document |

## Run locally

```bash
make dev-up
make migrate-up
make run-document-service
```

Health check:

```bash
curl http://localhost:8086/health
```

## Examples

Create ETRN document:

```bash
curl -X POST http://localhost:8086/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "document_number": "DOC-2026-000001",
    "document_type": "ETRN",
    "owner_company_id": "22222222-2222-2222-2222-222222222222",
    "related_entity_type": "SHIPMENT",
    "related_entity_id": "33333333-3333-3333-3333-333333333333",
    "legal_language": "ru-RU",
    "payload_json": {
      "shipment_number": "SH-2026-000001",
      "shipper": "Shipper LLC",
      "carrier": "Carrier LLC",
      "consignee": "Consignee LLC"
    }
  }'
```

Create POD document:

```bash
curl -X POST http://localhost:8086/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "document_number": "DOC-2026-000002",
    "document_type": "POD",
    "owner_company_id": "22222222-2222-2222-2222-222222222222",
    "related_entity_type": "SHIPMENT",
    "related_entity_id": "33333333-3333-3333-3333-333333333333",
    "legal_language": "ru-RU",
    "payload_json": {
      "delivery_confirmed": true
    }
  }'
```

Create new document version:

```bash
curl -X POST http://localhost:8086/v1/documents/{document_id}/versions \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "payload_json": {
      "updated": true,
      "comment": "Fixed consignee details"
    }
  }'
```

Create signing session:

```bash
curl -X POST http://localhost:8086/v1/documents/{document_id}/signing-sessions \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "required_signers_count": 2,
    "expires_at": "2026-07-01T18:00:00Z"
  }'
```

Add signature:

```bash
curl -X POST http://localhost:8086/v1/signing-sessions/{session_id}/signatures \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "signer_user_id": "44444444-4444-4444-4444-444444444444",
    "signer_company_id": "22222222-2222-2222-2222-222222222222",
    "signature_type": "SIMPLE_ELECTRONIC",
    "signature_payload_path": "signatures/test-signature.sig",
    "certificate_fingerprint": "test-fingerprint"
  }'
```

Archive document:

```bash
curl -X POST http://localhost:8086/v1/documents/{document_id}/archive \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111"
  }'
```

## Tests

```bash
make test-document-service
```

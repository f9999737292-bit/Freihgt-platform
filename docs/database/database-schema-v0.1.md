# Database Schema v0.1

> Placeholder — database schema reference for freight-platform.

## Status

Initial schema is applied via `infrastructure/migrations/`. This document will describe tables, relationships, and ownership rules.

## Schemas

| Schema | Owner service (target) | Purpose |
|--------|------------------------|---------|
| `core` | identity-service, company-service | Users, tenants, roles, companies |
| `transport` | transport-order-service, shipment-service | Orders, shipments, fleet |
| `rfx` | rfx-service | RFx events, bids, procurement |
| `documents` | document-service | Documents, files, signatures |
| `billing` | billing-register-service | Invoices, registers, acts |

## Local development

All schemas live in one PostgreSQL instance for local dev. See `infrastructure/README.md`.

## Next steps

- ER diagrams per schema
- Table-level field dictionary
- Migration policy and versioning

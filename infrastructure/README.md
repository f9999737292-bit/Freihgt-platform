# Infrastructure

Local development infrastructure for the freight platform monorepo.

## Contents

| Path | Purpose |
|------|---------|
| `docker-compose/docker-compose.yml` | PostgreSQL 16 and migration tooling |
| `migrations/` | SQL migrations (schemas + tables + seeds) |

## Database schemas

After `make migrate-up`, PostgreSQL contains:

| Schema | Tables (examples) |
|--------|-------------------|
| `core` | users, companies, tenants, roles, permissions, locales |
| `transport` | transport_orders, shipments, cargoes, vehicles, drivers |
| `rfx` | freight_requests, rfx_events, bids, rfx_lots |
| `documents` | documents, document_files, signatures |
| `billing` | invoices, billing_registers, acts, upd_documents |

## Commands (from monorepo root)

```bash
make env-init      # create .env from .env.example
make dev-up        # start PostgreSQL
make migrate-up    # apply migrations
make db-check      # verify schemas and tables
make db-shell      # psql shell
make dev-down      # stop containers
make clean         # stop containers and remove volumes
```

## Connection

Default credentials (see `.env.example`):

- Host: `localhost`
- Port: `5432`
- Database: `freight_platform`
- User: `freight`
- Password: `freight_password`

## Notes

- PostgreSQL image is pulled from `mirror.gcr.io/library/postgres:16` to avoid Docker Hub TLS issues in some networks.
- Migrations run via the compose `migrate` service on the same Docker network as PostgreSQL (works on Windows and Linux).

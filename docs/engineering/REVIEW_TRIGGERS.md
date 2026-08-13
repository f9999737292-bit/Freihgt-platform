# Review Triggers

When to invoke security-auditor or architect before integration.

## Security review triggers

Invoke **security-auditor** when changes touch any of:

- authentication flows or login/session handling
- authorization, RBAC, ABAC, role seeds
- `tenant_id`, `company_id`, membership scoping
- trusted identity headers (`X-Tenant-ID`, `X-User-ID`, JWT propagation)
- API gateway identity propagation to downstream services
- object lookup by ID without tenant predicate (IDOR risk)
- cross-tenant visibility or list/filter scoping
- document access controls
- billing access controls
- driver / vehicle assignment authorization
- admin / elevated privileges
- secrets, credentials, or `.env` handling
- `services/api-gateway/internal/shipmentrbac/**`
- repository queries removing `tenant_id` filters

Security agent is **reviewer by default**, not feature owner, unless Task Contract assigns implementation.

## Architecture review triggers

Invoke **architect** when changes include:

- new microservice under `services/`
- change to domain or service ownership boundaries
- new or changed event contract (Kafka/outbox payloads consumed cross-service)
- change to public API boundary (new aggregated surface, BFF bypass)
- new shared package under `packages/`
- change to database ownership (which service owns which schema/tables)
- cross-service write (service A writes service B's tables)
- new primary dependency (message broker, cache as source of truth)
- ADR-required structural change per `10-architecture.mdc`

## Combined gate

If both trigger lists apply, run **architect** decomposition first, then **security-auditor** on resulting Task Contracts.

## Output

- Security: severity-rated findings or PASS
- Architect: workstream boundaries, contract freeze plan, risks

Neither role merges code unless explicitly assigned.

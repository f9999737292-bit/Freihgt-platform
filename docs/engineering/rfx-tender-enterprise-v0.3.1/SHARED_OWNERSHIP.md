# RFx v0.3.1 Parallel Workstream Ownership

## Shared file owners (single writer)

| File / area | Owner | Notes |
|-------------|-------|-------|
| `services/rfx-service/**` (domain, repo, service, http, config) | **WS A** | Backend authoritative |
| `services/api-gateway/internal/http/proxy.go` (rfx routes) | **WS A** | After routes defined in rfx-service |
| `infrastructure/migrations/000036*` | **read-only** | Do not rewrite |
| `infrastructure/migrations/000037+` | **WS A** | Additive remediation only |
| `services/rfx-service/internal/integration/**` | **WS B** | Tests + test infra only |
| `packages/openapi/rfx-service.yaml` | **WS C** | Must match WS A routes |
| `packages/openapi/openapi.yaml` | **WS C** | Merge rfx paths |
| `apps/web-admin/**` (tender UI) | **WS C** | No frontend-only business rules |
| `docs/engineering/rfx-tender-enterprise-v0.3.1/**` | **Integration owner** | Final certification |

## API contract (WS A defines, WS C documents)

### Bid revisions — enterprise (rfx responses)
- `POST /v1/rfx-events/{event_id}/responses/{response_id}/revisions` — submit/revise bid
- `GET /v1/rfx-events/{event_id}/responses/mine` — carrier current response + active revision
- `GET /v1/rfx-events/{event_id}/responses/{response_id}/revisions` — own history (carrier) or all (shipper)
- `GET /v1/rfx-events/{event_id}/bids` — shipper bid comparison (authorized)

Headers: `X-Tenant-ID`, `X-User-ID`, `X-Carrier-Company-ID` (carrier scope)

### Bid revisions — freight request (mini-tender)
- `POST /v1/bids/{id}/revisions`
- `GET /v1/bids/{id}/revisions/current`
- `GET /v1/bids/{id}/revisions`

### Award
- `POST /v1/award-proposals/{proposal_id}/reject` (new)
- Finalize response includes `conversion` block when applicable

### Conversion policy
- `MINI_TENDER`, `RFQ`, `SPOT_RFQ` + linked freight_request → `IMMEDIATE_ORDER` via shipment `/v1/shipments/from-bid`
- `LANE_TENDER`, `CONTRACT_TENDER`, `RFP` → `ALLOCATION_AGREEMENT` (no auto order)

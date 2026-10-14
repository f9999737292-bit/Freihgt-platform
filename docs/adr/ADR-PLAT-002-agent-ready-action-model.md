# ADR-PLAT-002: Agent-ready action model

## Status

Accepted — strategic architecture freeze; implementation separately authorized

```text
ADR_PLAT_002=ACCEPTED
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
AUTONOMOUS_DOMAIN_MUTATION_AUTHORIZED=NO
CONTROL_TOWER_READ_MODEL_MODE_PRIMARY_AUTHORIZED=NO
```

## Context

BINTRANS Control Tower and related operational surfaces already project exceptions, risk, and workflow state. Product NEXT expects AI-assisted or rule-assisted **recommendations**. Without a frozen action model, agents and UI features tend to:

- mutate domain aggregates from a read model
- treat a recommendation as an executed command
- bypass gateway RBAC or tenant authority
- hide audit of who approved what

The platform trust boundary remains API Gateway JWT auth with trusted downstream identity headers. Domain services own their writes.

## Decision

### 1. Recommendation is not execution

An **ActionProposal** (name illustrative) may be produced by Control Tower logic, analytics signals, NLO search results, or a future assistant. It is advice with:

- target aggregate identity
- proposed command type
- evidence references
- required permission codes
- confidence / fail-closed reason when evidence is incomplete

It is **not** a completed domain mutation.

### 2. Canonical writers stay domain-owned

| Concern | Canonical writer |
| --- | --- |
| Shipment / stop / driver task mutation | shipment-service (and authorized TMS owners) |
| Document / signature / EDO artifact | document-service |
| Settlement / billing register money | billing-register-service (finance owners per Finance-0.1B) |
| NLO capacity / plan / search persistence | network-optimizer-service |
| Membership / roles | company-service / identity-service per ADR-PLAT-001 |

Control Tower read-model services do not become canonical writers of those aggregates.

### 3. Human or authorized workflow executes

Default NEXT posture:

```text
PROPOSAL_REQUIRES_EXPLICIT_ACCEPT=YES
SILENT_AUTO_APPLY_DEFAULT=NO
```

Auto-apply, if ever introduced, needs a separate authorized Task Contract, an allow-listed command set, idempotency, and append-only audit. It is out of scope for this ADR.

### 4. Authority path

1. Authenticate at api-gateway.
2. Authorize RBAC/ABAC for the command.
3. Establish trusted tenant and user identity.
4. Invoke the owning domain service command API.
5. Record proposal id, actor, decision (accept/reject), and resulting command correlation in audit.

Client-supplied tenant or role headers are never authority.

### 5. Fail closed

Missing evidence, indeterminate ownership, unknown tax, unbound provider device, or incomplete source contracts produce **no proposal that claims certainty**, or a proposal marked non-executable. Agents must not invent facts to make an action look ready.

## Consequences

- UI and assistant features may show recommended next steps without writing domain state.
- Implementation of proposal storage, APIs, or auto-apply needs a later Task Contract.
- Existing Control Tower acknowledgement and workflow features are not redefined here; they remain under their own contracts.
- Primary Control Tower read-model mode stays separately gated.

## Non-authorization

This ADR does not authorize product code, migrations, OpenAPI changes, staging mutation, autonomous agents writing to domain APIs, or enabling `CONTROL_TOWER_READ_MODEL_MODE=primary`.

## References

- [BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md](../architecture/BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md)
- [ADR-PLAT-001](ADR-PLAT-001-membership-user-roles-canonical-writer.md)
- Control Tower read-model and shadow rollout docs under `docs/`

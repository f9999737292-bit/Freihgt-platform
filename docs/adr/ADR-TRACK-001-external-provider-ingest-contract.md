# ADR-TRACK-001: External Provider Ingest Contract

## Status

Accepted — provider-neutral contract freeze (TRACKING-EXTERNAL-PROVIDER-CONTRACT-0.2A)

VENDOR_SELECTED=NO
VENDOR_ADAPTER_IMPLEMENTATION_AUTHORIZED=NO
STAGING_PROVIDER_ENABLEMENT_AUTHORIZED=NO
PROVIDER_CREDENTIAL_CREATION_AUTHORIZED=NO
LIVE_INGEST_AUTHORIZED=NO

## Context

`tracking-service` already accepts provider telemetry, ETA, and slot observations through internal push routes. The production registries contain only the `generic` adapter and, for telemetry, the `driver_mobile` adapter. No real vendor adapter is present.

A later vendor must be added as an adapter in front of the existing normalized model. Provider payload must stay outside domain and repository code. Provider fields must not become tenant or shipment authority.

This ADR freezes that boundary. It does not select a vendor, add an adapter, change schema, expose a public route, create a secret, or authorize live ingest.

The behavioral contract is [tracking-external-provider-contract-v0.2a.md](../architecture/tracking-external-provider-contract-v0.2a.md).

## Decision

1. Ingress shape remains:

   external provider payload → provider-specific adapter → normalized tracking model → tracking services and repositories → shipper, carrier, and Control Tower consumers.

2. Three capability ports stay independent: telemetry, ETA, and slots. A provider declares the subset it implements. An unsupported capability endpoint is rejected. It is not forwarded to another provider.

3. `providerCode` is a server-configured exact-match key. It selects the adapter and the provider secret. The caller does not assign it as authority.

4. Provider ingress authentication is `X-Provider-Secret`, loaded from protected environment configuration. It is distinct from `X-Internal-Service-Token` and from human JWT authentication.

5. Tenant and shipment identity for ordinary provider observations are derived from the active binding of `providerCode` plus `providerDeviceId`. Payload tenant, shipment, facility, and location identifiers are data claims, not access authority.

6. An unknown or unbound provider device is rejected. Provider ingest does not create a shipment binding.

7. Current runtime is inbound push only. Polling, websocket, and stream subscription are not implemented.

8. Delivery is at-least-once from the provider side. The server deduplicates. Exactly-once delivery is not claimed. Current state replacement uses source timestamps, source priority, and recorded-time comparison. Network arrival order is not business order.

9. Provider routes stay internal. Direct public exposure is not authorized.

10. The first real adapter does not require a new tracking schema when its payload normalizes into the current telemetry, ETA, and slot models. Customer shipper and carrier reads stay vendor-neutral.

11. Driver-mobile ingestion remains a separate trusted internal path. It does not use the provider secret.

12. Staging enablement of a real provider is a later sequence: frozen provider code, adapter tests, out-of-repo secret, network ingress review, rate-limit decision, and verified sandbox. Agent C does not provision staging. Live provider ingest is not authorized.

## Consequences

- TMS execution, migrations, the API gateway, OpenAPI, and driver-mobile stay unchanged in this stage.
- A missing per-provider rate limit and quota remains an explicit gap. This ADR does not add one.
- Provider secret rotation requires a controlled process restart, because configuration is loaded once at startup.
- Control Tower alert thresholds stay outside the provider adapter. The loss detector remains a tracking-service process with its current source default.

## Non-authorization

This decision does not authorize vendor selection, adapter implementation, credential creation, gateway exposure, schema migration, staging change, or live ingest.

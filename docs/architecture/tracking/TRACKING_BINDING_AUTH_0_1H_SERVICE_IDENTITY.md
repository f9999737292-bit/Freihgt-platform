# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Service Identity

`NAMED_SERVICE_IDENTITY=UNRESOLVED_NO_EXISTING_MECHANISM`

No secret is changed, read out, or rotated by this document. The comparison uses mechanisms that are already in the tree. It does not add a credential.

## 1. Shared internal token

Evidence: `packages/shared-go/internalauth/auth.go:9-25`. Tracking compares the same header at `tracking_handler.go:97-99` and `eta_handler.go:282-288`. One configured value is passed to several handlers and clients (`services/tracking-service/cmd/server/main.go:71-83`).

| Question | Answer |
| --- | --- |
| Confirmed identity | Possession of the secret. No service name. |
| Operations | Every internal tracking route that checks this header, including the binding insert and stop ETA. |
| Tenant scopes | None. `X-Tenant-ID` is a separate caller header. |
| Revocation | Replace the one secret. Every holder loses access together. |
| Rotation | One shared value. There is no overlap designed in this package. |
| Replay | The value is reusable until it is replaced. |
| Transition | This is the current credential. It cannot be the name of a successor. |

A client-supplied service-name header is not added. The shipment relationship client states that it does not send `X-Internal-Service-Name` (`services/shipment-service/internal/service/company_relationship_client.go:43-44`). A name the caller can set is not an identity.

## 2. Dedicated pair credential

Evidence: `CompanyRelationshipCreateClient` uses a shipment credential and refuses the shared `INTERNAL_SERVICE_TOKEN` (`company_relationship_client.go:43-45`). The receiver still sees a bearer secret, not a signed name.

| Question | Answer |
| --- | --- |
| Confirmed identity | Whichever server-side credential matched. The name exists only in the receiver's configuration map. It is not a header. |
| Operations | Only the routes that load that one credential. Today that is the relationship create path, not tracking. |
| Tenant scopes | Not carried by the credential. The relationship request has its own body. |
| Revocation | Remove or replace that pair's credential. Other pairs stay valid. |
| Rotation | Replace that pair's value. This document does not perform it. |
| Replay | The secret is reusable until replaced. Idempotency on the relationship command is a separate control and is not a tracking nonce. |
| Transition | New pair credentials can be introduced beside the shared tracking token. The shared token stays on the calls that already use it until a later change removes it from binding writes and ETA reads. |

This is the smallest existing pattern that can distinguish callers. It becomes a name only when the receiver, not the client, maps the matched credential to a service and an operation allowlist. Tracking has no such map. No tracking operation is allowlisted. Stop ETA and binding writes stay denied to every internal credential, including a future pair credential, until the controller names the service and the operations.

## 3. Integration JWT

Evidence: `packages/shared-go/integrationauth/claims.go:10-35`. Issuer `bintrans/integrations`, audience `bintrans/api`, TTL 15 minutes, claims for tenant, company, scope, and actor kind. The transport-order test rejects those headers when the internal token is absent (`ownership_internal_handler_test.go:84-93`). The test's failure is unauthorized. The headers are not a service login.

| Question | Answer |
| --- | --- |
| Confirmed identity | An external integration principal after gateway validation. Not an internal service. |
| Operations | The scopes on the integration token. Not tracking binding or ETA. |
| Tenant scopes | The tenant claim inside the token. That claim is still not a tracking grant. |
| Revocation | Expire at 15 minutes. A credential id is on the authenticated context (`claims.go:20`). This contract does not invent a revocation list. |
| Rotation | A new token is signed. The signing secret is not discussed here and is not changed. |
| Replay | Bounded by `TokenTTL`. A captured token works until expiry. |
| Transition | Not a successor to the tracking shared token. Putting internal services on the integration issuer would mix external integrations with service calls. |

## 4. Gateway user context

Evidence: `AuthContext` (`auth_context.go:13-16`) and the driver handler (`driver/handler.go:360-364`).

| Question | Answer |
| --- | --- |
| Confirmed identity | The user, after JWT validation, when authentication is enabled. |
| Operations | The gateway route that was authorized, including driver location. |
| Tenant scopes | The tenant on the auth context. |
| Revocation | Session and membership checks already on that route. Not a service credential. |
| Rotation | User tokens. Out of scope. |
| Replay | The gateway session. Not a service replay control. |
| Transition | Remains the user entry. It does not identify shipment-service, tracking-service, or any other process. |

## 5. Contract choice

The controller can accept the dedicated pair credential as the direction, with the name taken from the receiver's map and with an empty tracking allowlist until a service is named. The shared token stays a possession check and loses the binding write and the ETA read in that future change. Integration JWT stays on the external path. Gateway context stays the user path.

Until that choice, no service may create a binding decision, write a binding, or read an ETA. `GetExecutionStopETA` stays blocked. This stage does not select a service name, issue a credential, or edit a secret.

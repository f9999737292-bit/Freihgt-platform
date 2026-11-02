# EDO document requirement rebaseline 0.1

## Status

```text
STAGE=EDO-DOCUMENT-REQUIREMENT-REBASELINE-0.1
PLAN=BINTRANS-PLATFORM-DUAL-TRACK-ACTIVATION-0.1
AGENT=B
BASE_MAIN_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
KIND=DOCS_ONLY_REBASELINE
PRODUCT_CODE_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
OPENAPI_AUTHORIZED=NO
GATEWAY_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
PRODUCT_MIGRATION_HEAD=000105_platform_postgres_role_foundation_v0_2_i1
MIGRATION_NUMBER_ASSIGNED=NONE
GAP_B_001_STATUS=OPEN
GAP_B_001_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_IMPLEMENTATION_AUTHORIZED=NO
PR_272_CONTINUED=NO
```

This note reconciles GAP-B-001 with `origin/main` at `2a99b615a237b6d1f6e6038ca86a7433c567dc78`. It does not add a table, command, route, OpenAPI operation, migration, or gateway change. It does not authorize a completeness runtime. It does not continue pull request #272.

Inventory was read from this tree. Tests in document-service and shipment-service were not re-executed for this note. The push workflow on this same main SHA is recorded below. Staging was not contacted.

## Sources

| Source | SHA on this main | What it contributes |
| --- | --- | --- |
| [edo-document-completeness-source-v0.4b.md](edo-document-completeness-source-v0.4b.md) | `d94e5f6b91fcd0968ef963ca47dbe901b022c16e` | GAP-B-001 source contract. `IMPLEMENTATION_AUTHORIZED=NO` |
| [edo-document-requirement-policy-v0.1a.md](edo-document-requirement-policy-v0.1a.md) | `59797d78dd17337e955af3f92aef7020d1d3340f` | Requirement envelope. `PRODUCT_IMPLEMENTATION_AUTHORIZED=NO` |
| [edo-pod-delivery-action-evidence-binding-v0.1a-freeze.md](edo-pod-delivery-action-evidence-binding-v0.1a-freeze.md) | present on this main | Delivery-action evidence contract. Its own header still says `IMPLEMENTATION_AUTHORIZED=NO` |
| [edo-pod-delivery-action-evidence-binding-v0.1b-document-runtime.md](edo-pod-delivery-action-evidence-binding-v0.1b-document-runtime.md) | present on this main | Document-service runtime description for migration `000099` |
| [TMS_POD_REQUIREMENT_SOURCE_0_4C.md](tms/TMS_POD_REQUIREMENT_SOURCE_0_4C.md) | present on this main | TMS requirement ownership. Header inventory is older than migration `000098` |
| [FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md](finance-accounting/FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md) | `86a6b7b2eaeac3da1f1bc63a046c796eb49d5582` in the file header | `ACCOUNTING_DOCUMENT_POLICY_DECISION_IMPLEMENTED=NO` |
| [ANALYTICS_0_4A_KPI_SOURCE_CONTRACTS.md](analytics/ANALYTICS_0_4A_KPI_SOURCE_CONTRACTS.md) | present on this main | GAP-B-001 remains `CONTRACT_REQUESTED`, implementation `NO` |
| [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md) | accepted EDO-0.2 freeze | document-service does not edit shipment-service |
| Migration `000098_tms_pod_requirement_decision_foundation_v0_4d` | introduced by `cfde3ba9c46a209e412ecb0e2f7054973456ab8b` | Append-only TMS decision table. Later provenance fix `a9f4b370f3b3db7c4c5449df66d325fcb5719300` |
| Migration `000099_edo_pod_delivery_action_evidence_binding_v0_1b` | introduced by `7d5f77486a8643542365e995f5f8895d9b547776` | Action-bound POD intent and evidence. Remediation `2b39c053c32f43fb485e2b86be689f962bc6c6ca` |
| Main push CI | run `41222922070` | `conclusion=success`, `headSha=2a99b615a237b6d1f6e6038ca86a7433c567dc78`, event `push`, branch `main` |

Pull request #272 head `2bc3ba2b962c0dff207e0ce590d004d3128b6a47` is not an ancestor of this main. `git rev-list --count 2bc3ba2b962c0dff207e0ce590d004d3128b6a47..2a99b615a237b6d1f6e6038ca86a7433c567dc78` is 165. The file `docs/architecture/edo-document-requirement-runtime-contract-v0.1b.md` is not on this main.

## Classification

`IMPLEMENTED` means the table, route, or documented flag exists in this tree. `DEFERRED` means a later stage is named and this tree does not start it. `BLOCKED` means a consumer already records that it cannot publish until a missing fact exists. `NOT_VERIFIED` means this pass did not observe it.

| Fact | Class | Evidence on this SHA |
| --- | --- | --- |
| Canonical document types `ETRN`, `EPD`, `WAYBILL`, `POD`, `DISCREPANCY_ACT`, `CLAIM`, `INVOICE`, `VAT_INVOICE`, `ACT`, `UPD`, `ECMR` | IMPLEMENTED | `allowedDocumentTypes` in `services/document-service/internal/domain/document.go` |
| Related-entity values `SHIPMENT`, `TRANSPORT_ORDER`, `BILLING_REGISTER`, `INVOICE`, `COMPANY` | IMPLEMENTED | `allowedRelatedEntityTypes` in the same file. `DELIVERY` and `ACTION` are absent |
| Requirement-policy envelope 0.1A | IMPLEMENTED as documentation | `PRODUCT_IMPLEMENTATION_AUTHORIZED=NO`, `REQUIRED_DOCUMENT_POLICY_RUNTIME_PRESENT=NO`, `GAP_B_001_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO` |
| Completeness source contract 0.4B | IMPLEMENTED as documentation | `IMPLEMENTATION_AUTHORIZED=NO`, `REQUIRED_DOCUMENT_POLICY_PRESENT=NO` |
| `documents.pod_delivery_action_upload_intent` and `documents.pod_delivery_action_evidence` | IMPLEMENTED | Migration `000099`. No foreign key into `transport` |
| document-service routes under `/internal/v1/pod-action-evidence` | IMPLEMENTED | `services/document-service/internal/http/router.go`. Caller gate is shipment-service plus `POD_ACTION_EVIDENCE_SHIPMENT_SERVICE_TOKEN` |
| TMS table `transport.transport_execution_pod_requirement_decisions` | IMPLEMENTED | Migration `000098`. Header comment: this migration does not write known decisions and does not backfill history |
| TMS internal read `GET /internal/v1/transport-executions/{executionId}/actions/{actionId}/pod-requirement` | IMPLEMENTED | `services/shipment-service/internal/http/router.go`. Missing decision becomes synthetic `UNKNOWN` and is not inserted |
| Go writer that inserts a POD requirement decision | DEFERRED | `INSERT INTO transport.transport_execution_pod_requirement_decisions` appears in integration tests only |
| document-service call to that TMS read | DEFERRED | No `pod-requirement` client under `services/document-service` except tests that count the TMS table |
| shipment-service call to `/internal/v1/pod-action-evidence` | DEFERRED | No match under `services/shipment-service` |
| Gateway route for pod-action evidence | DEFERRED | No match under `services/api-gateway` |
| `GetApplicableDocumentRequirements` or `EvaluateDocumentCompleteness` | DEFERRED | Not present as a document-service implementation. 0.1A names both as future interfaces |
| Successor current-document policy | DEFERRED | 0.1A `SUCCESSOR_CURRENT_DOCUMENT_POLICY_FROZEN=NO` |
| Finance `ACCOUNTING_DOCUMENT_POLICY_DECISION` runtime | DEFERRED | Finance 0.1D `ACCOUNTING_DOCUMENT_POLICY_DECISION_IMPLEMENTED=NO` and `PRODUCT_IMPLEMENTATION_AUTHORIZED=NO` |
| `OPS_DOCUMENT_COMPLETENESS`, `CAR_DOCUMENT_COMPLETENESS`, `DOC_DOCUMENT_COMPLETENESS` | BLOCKED | Analytics 0.1B and 0.3A keep them `BLOCKED_BY_SOURCE_GAP` on GAP-B-001. Analytics 0.4A implementation column is `NO` |
| POD satisfaction runtime | DEFERRED | 0.1B `POD_SATISFACTION_RUNTIME=NO` and `POD_ACCEPTANCE_POLICY_EVALUATED=NO` |
| End-to-end driver upload through shipment-service into document-service | DEFERRED | 0.1B `END_TO_END_POD_ACTION_RUNTIME_READY=NO`. The document-service routes exist. The shipment-service proxy does not |
| Staging schema version, document-service login role, and live row counts | NOT_VERIFIED | This pass did not open a database or a staging host |
| Grant of `documents.pod_delivery_action_evidence` to a document-service login | NOT_VERIFIED | `000103` creates `freight_migrations`, `company_runtime`, and `shipment_runtime`. `000105` grants `shipment_runtime` `SELECT` on `documents.documents` and on `transport.transport_execution_pod_requirement_decisions`. It does not name `documents.pod_delivery_action_evidence`. No `document_runtime` role is created there |

## What changed after the 0.4B inventory

Completeness 0.4B recorded migration head `000096_edo_0_3_i4b_signature_verification_foundation` and said no later document migration existed in that tree. This main head is `000105`. Two later migrations matter for GAP-B-001, and neither is a required-document policy.

Migration `000098` stores append-only decision versions for one delivery action. Columns include `decision_state`, nullable `pod_required`, nullable `required_document_type`, and nullable policy identifiers. shipment-service reads the latest version for one operating tenant and action. `PODRequirementService` does not write. A missing row returns synthetic `UNKNOWN` with `pod_required` unset. A pickup action conflicts. The driver milestone flag remains `RequiresPOD: false` in `services/shipment-service/internal/domain/execution_tracking.go`. That flag is still not a stored decision.

Migration `000099` stores an action-bound upload intent and an immutable evidence row. `document_tenant_id` must equal `operating_tenant_id`. `shipment_tenant_id` may differ. One document binds to at most one delivery action. Historical shipment-only POD rows are not backfilled. The 0.1B note says present evidence requires a binding row, intent status `completed`, the pinned file, and a document whose `deleted_at` is null. Status `SIGNED` is not presence. The service does not read or write `transport.transport_execution_pod_requirement_decisions`.

`TestPOD_BIND_16And18` completes action evidence and requires the TMS decision count to stay unchanged. `TestPOD_BIND_17` reads evidence after a known not-required decision and requires that decision row to remain. Those tests are in the tree. They were not run in this pass. An earlier historical upload through `/internal/v1/pod-uploads` is asserted not to create `documents.pod_delivery_action_evidence`.

`OrderExecutionRepository.ListShipmentPODDocuments` still selects `documents.documents` where `related_entity_type = 'SHIPMENT'`, `related_entity_id` is the shipment, and `document_type = 'POD'`. That list is not the action-evidence table and it is not a requirement.

TMS 0.4C still says `POD_REQUIREMENT_TABLE_PRESENT=NO` and `POD_REQUIREMENT_SOURCE_PRESENT=NO`. Those two lines describe the tree at `109ec8117b75cb8205af7667eef6d52b30a11bc6`. They are not a description of `000098` on this main. This note does not edit 0.4C. Ownership in 0.4C remains in force: shipment-service decides whether POD is required, and document-service does not.

## What GAP-B-001 still is

Analytics 0.1A asks for a required document set per shipment or transport order and a server-owned completeness result. Analytics 0.4A asks document-service for `REQUIRED_DOCUMENT_TYPES`, `PRESENT_DOCUMENT_IDS`, `MISSING_DOCUMENT_TYPES`, and `COMPLETENESS_STATUS` of `COMPLETE`, `INCOMPLETE`, or `UNKNOWN`. 0.4B says an unknown policy evaluates to `UNKNOWN`, not `COMPLETE`. 0.1A says document-service must not invent `POD_REQUIRED`, must not treat a document row as a requirement, and must not treat `SIGNED` or a row count as completeness.

Those rules are unchanged by `000099`. Action evidence proves that a completed binding exists for one delivery action. It does not prove that POD was required. A TMS read that returns synthetic `UNKNOWN` does not prove that POD was not required. No evaluator composes the two.

```text
TMS_DECISION_READ=IMPLEMENTED_UNKNOWN_WHEN_ABSENT
ACTION_EVIDENCE_BINDING=IMPLEMENTED
REQUIREMENT_POLICY_REGISTRY=ABSENT
COMPLETENESS_EVALUATOR=ABSENT
GAP_B_001_CLOSED_BY_000099=NO
SIGNED_STATUS_IS_COMPLETENESS=NO
DOCUMENT_COUNT_IS_COMPLETENESS=NO
DOCUMENT_EXISTS_IMPLIES_REQUIREMENT=NO
POD_UPLOAD_WRITES_TMS_REQUIREMENT=NO
TMS_REQUIREMENT_WRITES_DOCUMENT_EVIDENCE=NO
```

The subject mismatch stays unresolved. Completeness 0.4B and Analytics 0.4A name `SHIPMENT` and `TRANSPORT_ORDER`. POD evidence 0.1B names one delivery action. Policy 0.1A classifies `DELIVERY` as `OWNER_DOMAIN_REQUIRED` and `STOP` / `ACTION` as `FUTURE_SUPPORTED`. This rebaseline does not retarget GAP-B-001 and does not add a related-entity value.

Successor satisfaction stays `NOT_EVALUABLE` until a later contract freezes `EDGE_DIRECTION`, `CARDINALITY`, `CURRENT_DOCUMENT_SELECTION`, `PREDECESSOR_VALIDITY`, and `SUCCESSOR_ACCEPTANCE_RULE`. Ambiguous `CORRECTS` or `REPLACES` stays `UNKNOWN`.

## Pull request 272

Draft pull request #272, `docs(edo): freeze requirement runtime task contract`, is not the base for the next stage. Its head is 165 commits off this main and is not contained in this main. Its runtime-contract file is absent here. Continuing that branch would replay a contract written before `000098`, `000099`, and `000105` were the main head.

This rebaseline is the current document. It does not merge #272 and does not copy its selected runtime architecture into this tree.

## Dependencies

| Dependency | Owner | State for the next EDO stage |
| --- | --- | --- |
| Delivery-action evidence read | document-service / Agent B | Present. Internal, shipment-service caller only |
| POD requirement read | shipment-service / Agent C | Present. Missing row is `UNKNOWN`. No known-decision writer |
| Known `pod_required` decision | Agent C | DEFERRED. document-service must not insert it. ADR-EDO-009 forbids an EDO edit of shipment-service for that writer |
| Upload proxy from shipment-service | Agent C | DEFERRED. 0.1A freeze names shipment-service as the upload caller. The proxy is not in this tree |
| Accounting requirement decision | Agent G / Finance | DEFERRED. 0.1D decision is not implemented |
| Successor current-document rule | Agent B, later contract | DEFERRED. Not frozen |
| KPI publication | Agent E | BLOCKED on GAP-B-001 |
| Product migration head `000105` and pull request #322 | outside this stage | Not modified. No migration number `000106` is assigned |

`CROSS_DOMAIN_WRITE_REQUIRED=NO` remains the rule from 0.1A, 0.1B, and TMS 0.4C.

## Open questions

1. A later contract has to say whether POD completeness is evaluated per delivery action, per shipment, or both. This tree now has both a shipment-scoped POD list and an action-scoped evidence table. This note does not choose.
2. The first server-owned writer of `decision_state=KNOWN` is not in this tree. Until Agent C adds that writer, a normal read stays `UNKNOWN`.
3. shipment-service is the authorized caller of the evidence routes and does not call them. Whether the proxy is the next TMS stage is an Agent C decision.
4. `000105` does not grant `documents.pod_delivery_action_evidence`. The login document-service uses when those grants are enforced was not observed.
5. Staging was not read. Schema version, dirty flag, and row counts for `000098` and `000099` are `NOT_VERIFIED`.

## Next stage

```text
NEXT_RECOMMENDED_STAGE=CONTROLLER_REVIEW
AFTER_ACCEPTANCE=EDO-DOCUMENT-COMPLETENESS-COMPOSITION-CONTRACT
IMPLEMENTATION_START_AUTHORIZED_BY_THIS_NOTE=NO
```

Controller review of this rebaseline comes first. After acceptance, the next EDO document is a composition contract. That contract may name how document-service reads the existing TMS pod-requirement route and the existing action-evidence route. It must keep `UNKNOWN` when the TMS decision is absent, must not copy `transport.transport_execution_pod_requirement_decisions` into `documents`, and must not publish `COMPLETE` or `INCOMPLETE` while the successor policy is unfrozen or while Finance 0.1D has no runtime decision. It is still a document stage unless that later contract is separately authorized to change code.

This note does not authorize that composition contract's product code, a migration, OpenAPI, a gateway route, a KPI, or a restart of pull request #272.

## Non-goals

No product code, migration, OpenAPI, gateway, S3, credential, staging, or KPI publication belongs to this stage. `000105` stays the product migration head. Pull request #322 is not edited.

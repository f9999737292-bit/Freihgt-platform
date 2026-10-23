# Pilot Release Manifest v0.1

Formal Pilot release pinning for **Control Tower shadow-observation Pilot** on dedicated Selectel staging VM.

**Not a deployment.** This document records the verified artifact set for go/no-go and rollback reference.

---

## Release Identity

| Field | Value |
| --- | --- |
| PILOT_RELEASE_ID | `CT-PILOT-2026-08-14-b75eb3d` |
| PILOT_GIT_SHA | `b75eb3d` |
| PILOT_SOURCE_BRANCH | `ops/bintrans-ct-staging-pack` (documented staging baseline) |
| PILOT_CREATED_AT | 2026-08-14 |
| ENVIRONMENT | Selectel dedicated VM `161.104.57.152` |
| CONTROL_TOWER_MODE | shadow (PRIMARY disabled) |
| MIGRATION_TARGET | 000019 |

---

## Version Decision

| Comparison | Classification |
| --- | --- |
| `b75eb3d` → `234c8b78` delta | Alert-ack features, docs, parallel-engineering tooling |
| Security / tenant isolation | Present at `b75eb3d` (verified on staging) |
| **Decision** | **Pin `b75eb3d`** — only artifact with actual Selectel verification evidence |
| SHA alignment with main | **NOT_REQUIRED_FOR_CURRENT_PILOT** (COND-003 closed) |

---

## Critical Service Images (Digest-Pinned)

| Service | Image digest |
| --- | --- |
| api-gateway | `cr.selcloud.ru/bintrans-staging/api-gateway@sha256:db9714a5ced2ab26de96fad8ed211cf116a040a7a1b6ea75351e07861f52df8c` |
| identity-service | `cr.selcloud.ru/bintrans-staging/identity-service@sha256:0b4a36601619f2bdad1dda215ae8043dfd8430bf60dc56a85919cf13003a0939` |
| shipment-service | `cr.selcloud.ru/bintrans-staging/shipment-service@sha256:707edb5a492a3c99708365c48c29f2ab9b88007968b5df3cd5c7927bef0a5fae` |
| document-service | `cr.selcloud.ru/bintrans-staging/document-service@sha256:6369d76991f590c5cc26967fb14fd424051e29718b0c645a3cc147fa7362c83a` |
| billing-register-service | `cr.selcloud.ru/bintrans-staging/billing-register-service@sha256:155bdd2fd60f3d56f615f997b062834bc881e281b4c96c82d18d846241bd831e` |
| control-tower-read-model | `cr.selcloud.ru/bintrans-staging/control-tower-read-model-service@sha256:defe3f667e09f818faf75d8983fe4391997c3f3fa14162489007498c2bdc5cbd` |
| company-service | `cr.selcloud.ru/bintrans-staging/company-service@sha256:ea5bce7409521b4e1c6e281953f0bc3b22c5072d9f1ebdf18cfd6b0402eea0ed` |
| transport-order-service | `cr.selcloud.ru/bintrans-staging/transport-order-service@sha256:f3d8c48e72e86e42f3a0ec0104a26aeecf2f836cd01fba3d72ce8b296497adaa` |
| rfx-service | `cr.selcloud.ru/bintrans-staging/rfx-service@sha256:1729e2f933514f4148bb3183f2831a4616d6b9558a05d73e9df913598f93bca6` |
| low-code-service | `cr.selcloud.ru/bintrans-staging/low-code-service@sha256:d91b3a2ea794c6da9987ed3136b1cecb2219e80f3bfcebd591dd94637a66cb56` |
| redpanda | `docker.redpanda.com/redpandadata/redpanda:v24.2.8` (upstream tag) |

```text
IMAGE_DIGEST_PINNING=YES
```

---

## Rollback Release

| Field | Value |
| --- | --- |
| ROLLBACK_RELEASE_ID | `CT-PILOT-2026-08-14-b75eb3d` (same — first verified known-good on dedicated VM) |
| ROLLBACK_GIT_SHA | `b75eb3d` |
| ROLLBACK_IMAGE_DIGESTS | Same as above |
| Prior verified release | **None documented** on dedicated VM |

---

## Approvals

| Gate | Status |
| --- | --- |
| TECHNICAL_APPROVAL | TBD |
| OPERATIONS_APPROVAL | TBD |
| SECURITY_APPROVAL | TBD (security **gate** PASS; formal sign-off TBD) |
| BUSINESS_GO_LIVE_APPROVAL | TBD |

---

## Safety

No secrets in this manifest. Digests collected read-only from staging VM on 2026-08-14.

# RFx v3.0E7 Phase 2 — ERP API Diagrams

**Status:** REMEDIATION_COMPLETE_PENDING_RE_REVIEW
**Parent:** [RFX_V3_0E7_ERP_API.md](./RFX_V3_0E7_ERP_API.md)
**Validation:** `MERMAID_VALIDATION=MANUAL_ONLY` (no repository automated validator)

---

## 1. System context

```mermaid
flowchart TB
    subgraph ERP["Buyer ERP / SAP / 1C / TMS"]
        Adapter["Vendor adapter"]
    end

    subgraph BINTRANS["BINTRANS Platform"]
        GW["API Gateway\nOAuth or API-key principal"]
        RFx["rfx-service\nERP preview/commit"]
        ID["identity-service\nintegration principals NEW"]
        DB[(PostgreSQL)]
    end

    Adapter -->|"BINTRANS_RFX_ERP_JSON_V1"| GW
    GW --> ID
    GW --> RFx
    RFx --> DB
```

---

## 2. Machine authentication (no downgrade)

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant GW as API Gateway
    participant ID as Identity
    participant RFx as rfx-service

    ERP->>GW: POST /integrations/oauth/token
    GW->>ID: Validate OAuth principal
    ID-->>GW: principal + credential_type=OAUTH
    GW-->>ERP: access_token

    Note over ERP,GW: API-key rejected for OAuth-only principal

    ERP->>GW: POST /erp/rfx/drafts/preview\nBearer OAuth token
    GW->>GW: Strip spoofed headers
    GW->>RFx: X-Integration-Principal-ID, X-Tenant-ID, X-Company-ID
    RFx-->>ERP: analysis_id
```

---

## 3. CREATE Preview → Commit (stable identity)

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service
    participant DB as PostgreSQL

    ERP->>RFx: POST create preview
    RFx->>RFx: Map codes + pin mapping_context
    RFx->>DB: INSERT analysis\nintegration_principal_id set\nactor_id NULL

    ERP->>RFx: POST create commit + Idempotency-Key
    RFx->>DB: LOCK analysis
    RFx->>RFx: Verify principal binding
    RFx->>DB: BEGIN
    RFx->>DB: INSERT rfx_event DRAFT
    RFx->>DB: INSERT stable external link
    RFx->>DB: CONSUMED + idempotency
    RFx->>DB: COMMIT
    RFx-->>ERP: 201 rfx_event_id
```

---

## 4. UPDATE Preview → Commit

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service
    participant DB as PostgreSQL

    ERP->>RFx: POST update preview
    RFx->>DB: Load DRAFT baseline versions
    RFx->>DB: INSERT analysis with pinned mapping_context

    ERP->>RFx: POST update commit
    RFx->>RFx: Revalidate baseline tokens
    alt stale draft_row_version
        RFx-->>ERP: 409 proposal_revalidation_failed
    else ok
        RFx->>DB: Apply pinned canonical payload
        RFx-->>ERP: 200 applied
    end
```

---

## 5. Retry after timeout

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service
    participant DB as PostgreSQL

    ERP->>RFx: POST commit Idempotency-Key K1
    RFx->>DB: Success + store idempotency response
    Note over ERP,RFx: Client timeout

    ERP->>RFx: POST commit Idempotency-Key K1
    RFx->>DB: Replay stored 201
    RFx-->>ERP: Same rfx_event_id
```

---

## 6. Concurrent CREATE (winner/loser)

```mermaid
sequenceDiagram
    participant ERP1 as ERP Client A
    participant ERP2 as ERP Client B
    participant RFx as rfx-service
    participant DB as PostgreSQL

    par Parallel previews
        ERP1->>RFx: CREATE preview
        ERP2->>RFx: CREATE preview
    end

    par Parallel commits
        ERP1->>RFx: CREATE commit
        ERP2->>RFx: CREATE commit
    end

    RFx->>DB: Unique stable external key
    Note over DB: One COMMIT wins
    RFx-->>ERP1: 201 created
    RFx-->>ERP2: 409 external_id_conflict
```

---

## 7. Expired analysis

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service

    Note over RFx: expires_at passed
    ERP->>RFx: POST commit expired analysis_id
    RFx-->>ERP: 409 analysis_expired
```

---

## 8. Credential revocation

```mermaid
sequenceDiagram
    participant Admin as Tenant Admin
    participant ID as Identity
    participant ERP as ERP Client
    participant GW as Gateway

    Admin->>ID: Revoke credential
    ERP->>GW: Request with old token
    GW->>ID: Validate
    ID-->>GW: credential_revoked
    GW-->>ERP: 401
```

---

## 9. Mapping version pin / stale mapping

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service
    participant DB as PostgreSQL

    ERP->>RFx: POST preview
    RFx->>DB: Analysis with mapping_context v3

    Note over DB: Mapping set v3 RETIRED

    ERP->>RFx: POST commit
    RFx->>RFx: Mapping set no longer ACTIVE
    RFx-->>ERP: 409 stale_mapping_context
```

---

## 10. Analysis state machine

```mermaid
stateDiagram-v2
    [*] --> PREVIEWED: Preview persisted
    PREVIEWED --> CONSUMED: Commit success
    PREVIEWED --> EXPIRED: expires_at passed
    CONSUMED --> [*]: Terminal single-use
    EXPIRED --> [*]: Terminal

    note right of PREVIEWED
        integration_principal_id or actor_id XOR
        mapping_context pinned in payload
    end note
```

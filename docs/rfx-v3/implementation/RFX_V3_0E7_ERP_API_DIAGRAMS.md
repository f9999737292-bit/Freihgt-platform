# RFx v3.0E7 Phase 2 — ERP API Diagrams

**Status:** Architecture freeze  
**Parent:** [RFX_V3_0E7_ERP_API.md](./RFX_V3_0E7_ERP_API.md)

---

## 1. System context

```mermaid
flowchart TB
    subgraph ERP["Buyer ERP / SAP / 1C / TMS"]
        Adapter["Vendor adapter\n(maps to canonical JSON)"]
    end

    subgraph BINTRANS["BINTRANS Platform"]
        GW["API Gateway\nOAuth / API key / RBAC"]
        RFx["rfx-service\nERP preview/commit"]
        ID["identity-service\n(integration principals)"]
        DB[(PostgreSQL\nrfx_import_analyses\nexternal_object_links)]
    end

    Adapter -->|"HTTPS JSON"| GW
    GW --> ID
    GW --> RFx
    RFx --> DB
```

---

## 2. Machine authentication

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant GW as API Gateway
    participant ID as Identity
    participant RFx as rfx-service

    ERP->>GW: POST /integrations/oauth/token\nclient_credentials
    GW->>ID: Validate client_id + secret
    ID-->>GW: integration_principal + scopes
    GW-->>ERP: access_token (15m TTL)

    ERP->>GW: POST /erp/rfx/drafts/preview\nAuthorization: Bearer token
    GW->>GW: Strip spoofed identity headers
    GW->>GW: Validate token + scopes
    GW->>RFx: X-Tenant-ID, X-Integration-Principal-ID, X-Company-ID
    RFx-->>GW: Preview response
    GW-->>ERP: 200 + analysis_id
```

---

## 3. CREATE Preview → Commit

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant GW as Gateway
    participant RFx as rfx-service
    participant DB as PostgreSQL

    ERP->>GW: POST .../drafts/preview\nBINTRANS_RFX_ERP_JSON_V1
    GW->>RFx: Forward authenticated
    RFx->>RFx: Map reference codes
    RFx->>RFx: Validate domain rules
    alt ready_to_commit
        RFx->>DB: INSERT rfx_import_analyses\nPREVIEWED, hash, TTL 24h
        RFx-->>ERP: analysis_id
    else blocking errors
        RFx-->>ERP: 422 + issues
    end

    ERP->>GW: POST .../drafts/commit\nanalysis_id + Idempotency-Key
    GW->>RFx: Forward
    RFx->>DB: LOCK analysis FOR UPDATE
    RFx->>RFx: Verify hash + binding
    RFx->>DB: BEGIN; CREATE event; reconcile graph/lots
    RFx->>DB: INSERT external_object_link
    RFx->>DB: Mark CONSUMED; idempotency record
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

    ERP->>RFx: POST /rfx-events/{id}/erp-import/preview
    RFx->>DB: Load DRAFT baseline + versions
    RFx->>RFx: Build canonical payload with baseline tokens
    RFx->>DB: INSERT analysis (if ready)

    ERP->>RFx: POST /rfx-events/{id}/erp-import/commit
    RFx->>DB: Lock analysis
    RFx->>RFx: Revalidate baseline vs live DRAFT
    alt stale
        RFx-->>ERP: 409 stale_target
    else ok
        RFx->>DB: Atomic reconcile graph/lots
        RFx->>DB: CONSUMED + audit
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

    ERP->>RFx: POST commit (Idempotency-Key: K1)
    RFx->>DB: Success; store idempotency response
    Note over ERP,RFx: Network timeout before response

    ERP->>RFx: POST commit (Idempotency-Key: K1, same body)
    RFx->>DB: Find idempotency record scope+K1
    RFx-->>ERP: Replay stored 201 (no duplicate event)
```

---

## 6. Concurrent ERP / UI update

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant UI as Buyer UI
    participant RFx as rfx-service

    ERP->>RFx: Preview (captures draft_row_version=5)
    UI->>RFx: Save draft (draft_row_version→6)
    ERP->>RFx: Commit analysis (baseline version=5)
    RFx->>RFx: Live version=6 ≠ baseline=5
    RFx-->>ERP: 409 proposal_revalidation_failed
```

---

## 7. Expired analysis

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service
    participant DB as PostgreSQL

    Note over DB: analysis created_at + 24h < now()
    ERP->>RFx: POST commit (expired analysis_id)
    RFx->>DB: Load analysis status=PREVIEWED
    RFx->>RFx: expires_at check fails
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

    Admin->>ID: Revoke integration credential
    ERP->>GW: Request with old token
    GW->>ID: Validate token
    ID-->>GW: credential_revoked
    GW-->>ERP: 401 credential_revoked
```

---

## 9. Mapping failure (fail closed)

```mermaid
sequenceDiagram
    participant ERP as ERP Client
    participant RFx as rfx-service

    ERP->>RFx: POST preview\n"currency": "SAP:ZZZ"
    RFx->>RFx: Lookup mapping CURRENCY
    RFx->>RFx: No mapping for ZZZ
    RFx-->>ERP: 422 unknown_currency_code\njson_pointer=/event/currency
```

---

## 10. Analysis state machine

```mermaid
stateDiagram-v2
    [*] --> PREVIEWED: Preview persisted\n(ready_to_commit=true)
    PREVIEWED --> CONSUMED: Commit success
    PREVIEWED --> EXPIRED: expires_at passed\n(lazy check at commit)
    CONSUMED --> [*]: Single-use terminal
    EXPIRED --> [*]: Terminal

    note right of PREVIEWED
        Payload immutable
        (DB trigger)
    end note

    note right of CONSUMED
        result_reference set
        consumed_at set
    end note
```

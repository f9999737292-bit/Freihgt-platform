# RFx v3.0A — Architecture Diagrams

**Status:** Architecture draft  
**Format:** Mermaid (render in GitHub / compatible viewers)

---

## 1. System Context

```mermaid
C4Context
  title RFx v3.0A System Context
  Person(buyer, "Buyer", "RFx Studio user")
  Person(carrier, "Carrier", "Response workspace user")
  System_Boundary(bintrans, "BINTRANS Platform") {
    System(gateway, "API Gateway", "JWT auth, RBAC, tenant headers")
    System(rfx, "rfx-service", "RFx domain, scoring, qualification")
    System(docs, "document-service", "Attachments, certificates")
    System(company, "company-service", "Company profile, membership")
    System(shipment, "shipment-service", "KPI, operational data")
    System(ct, "Control Tower", "Observability, alerts")
  }
  System_Ext(email, "Email / notifications", "Invites, reminders")
  Rel(buyer, gateway, "HTTPS")
  Rel(carrier, gateway, "HTTPS")
  Rel(gateway, rfx, "Trusted headers")
  Rel(rfx, docs, "Attachment validation")
  Rel(rfx, company, "Membership, Carrier 360")
  Rel(rfx, shipment, "KPI for scoring")
  Rel(rfx, email, "Outbox events")
  Rel(rfx, ct, "Metrics events")
```

---

## 2. RFx Lifecycle

```mermaid
stateDiagram-v2
  [*] --> DRAFT: Create RFx
  DRAFT --> DRAFT: Autosave / edit
  DRAFT --> PUBLISHED: Publish gate PASS
  PUBLISHED --> RESPONSES_OPEN: Open responses
  RESPONSES_OPEN --> RESPONSES_CLOSED: Deadline
  RESPONSES_CLOSED --> EVALUATION: Evaluate
  EVALUATION --> AWARDED: Award
  EVALUATION --> SHORTLISTED: Shortlist
  AWARDED --> ARCHIVED: Close
  DRAFT --> CANCELLED: Cancel
  PUBLISHED --> NEW_VERSION: Material edit
  NEW_VERSION --> DRAFT: New RfxVersion draft
```

---

## 3. RFI Qualification Flow

```mermaid
flowchart TD
  A[Publish RFI] --> B[Invite carriers]
  B --> C[Carrier starts response]
  C --> D[Autosave valid answers]
  D --> E{Submit gate}
  E -->|errors| D
  E -->|pass| F[Score + knockout]
  F --> G{Qualified?}
  G -->|yes| H[Add to qualification pool]
  G -->|conditional| I[Manual review]
  G -->|no| J[Rejected with evidence]
  H --> K[Invite to RFQ/RFP]
```

---

## 4. Questionnaire Domain

```mermaid
erDiagram
  RFX_EVENT ||--o{ RFX_VERSION : has
  RFX_VERSION ||--o{ RFX_SECTION : contains
  RFX_SECTION ||--o{ RFX_QUESTION : contains
  RFX_QUESTION ||--o{ RFX_QUESTION_OPTION : has
  RFX_VERSION ||--o{ RFX_QUESTION_RULE : defines
  RFX_EVENT ||--o{ RFX_RESPONSE : receives
  RFX_RESPONSE ||--o{ RFX_ANSWER : persists
  RFX_QUESTION ||--o{ RFX_ANSWER : answered_by
  RFX_ANSWER ||--o{ RFX_ANSWER_EVIDENCE : supports
```

---

## 5. Carrier Response Flow

```mermaid
sequenceDiagram
  participant C as Carrier UI
  participant G as API Gateway
  participant R as rfx-service
  participant DB as PostgreSQL
  C->>G: PATCH answers (JWT)
  G->>G: Verify tenant + membership
  G->>R: Trusted identity + participant company
  R->>R: Validate batch L1-L3
  alt validation fail
    R-->>C: 422 VALIDATION_FAILED
  else success
    R->>DB: COMMIT answers + save_version
    R-->>C: 200 + warnings/knockouts
  end
  C->>G: POST validate-submit
  G->>R: L4 submit gate
  R-->>C: submit_allowed + errors
```

---

## 6. Validation / Autosave Flow

```mermaid
flowchart LR
  subgraph Client
    LD[Local draft incl. invalid]
    LV[Last valid server snapshot]
  end
  subgraph Server
    V[L1-L3 validation]
    T[Transaction]
    A[Authoritative answers]
  end
  LD -->|PATCH batch| V
  V -->|422| LD
  V -->|pass| T
  T --> A
  A --> LV
  LD -.->|invalid kept locally| LD
```

---

## 7. Scoring Flow

```mermaid
flowchart TD
  PA[Persisted valid answers] --> SM[Load score_model_version]
  SM --> CR[Apply criteria + weights]
  CR --> NR[Normalize]
  NR --> KO{Knockout rules}
  KO -->|triggered| REJ[REJECTED + evidence]
  KO -->|pass| TH{Thresholds}
  TH -->|pass| Q[QUALIFIED]
  TH -->|partial| CQ[CONDITIONALLY_QUALIFIED]
  CR --> AS[answer_scores + explanation_json]
  AS --> EV[rfx.score.calculated event]
```

---

## 8. Carrier 360 Flow

```mermaid
flowchart LR
  subgraph Sources
    CP[Company profile]
    DOC[Documents]
    SH[Shipment KPI]
  end
  subgraph Carrier360
    AGG[Aggregate + freshness]
  end
  subgraph RFx Response
    AF[Autofill]
    CF[Carrier confirm]
    AN[Answer persist]
  end
  CP --> AGG
  DOC --> AGG
  SH --> AGG
  AGG --> AF
  AF --> CF
  CF --> AN
```

---

## 9. Event Flow

```mermaid
flowchart LR
  DS[Domain service TX] --> OB[(rfx_event_outbox)]
  OB --> WK[Outbox worker]
  WK --> KF[Kafka topic]
  KF --> NS[Notification service]
  KF --> SC[Scoring consumer]
  KF --> AU[Audit indexer]
  KF --> AN[Analytics future]
```

---

## 10. Security / RBAC Boundary

```mermaid
flowchart TD
  EXT[External client] -->|JWT Bearer| GW[API Gateway]
  GW -->|X-Tenant-ID X-User-ID| RS[rfx-service]
  GW -->|Resolve membership| ID[identity-service]
  RS -->|Participant check| RS
  RS -->|Owner company check| RS
  EXT -.->|X-Company-ID untrusted| X[FORBIDDEN]
  RS -->|tenant_id predicate| DB[(PostgreSQL)]
```

**Flags:** `CLIENT_SUPPLIED_COMPANY_AUTHORITY=FORBIDDEN`, `TENANT_AUTHORITY=SERVER_VERIFIED`, `CROSS_TENANT_SPOOF=DENIED`.

---

## Diagram index

| # | Diagram | Section |
|---|---|---|
| 1 | System Context | §1 |
| 2 | RFx Lifecycle | §2 |
| 3 | RFI Qualification Flow | §3 |
| 4 | Questionnaire Domain | §4 |
| 5 | Carrier Response Flow | §5 |
| 6 | Validation / Autosave | §6 |
| 7 | Scoring Flow | §7 |
| 8 | Carrier 360 Flow | §8 |
| 9 | Event Flow | §9 |
| 10 | Security / RBAC | §10 |

---

## References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md)
- [RFX_V3_EVENTS.md](./RFX_V3_EVENTS.md)
- [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md)

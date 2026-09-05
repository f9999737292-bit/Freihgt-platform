# RFx v3.0A — State Machines

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. RFx event (buyer) lifecycle

```
DRAFT ──publish──► PUBLISHED ──cancel──► CANCELLED
  │                    │
  │                    └── material edit ──► NEW_RFX_VERSION (DRAFT fork → publish)
  └── autosave / manual save (while DRAFT)
```

| State | Meaning |
|---|---|
| `DRAFT` | Editable buyer working copy; not visible to carriers as final |
| `PUBLISHED` | Active tender; immutable published version record |
| `CANCELLED` | Closed without award continuation |

**Draft representation:** `DRAFT` status + `last_saved_at` + working version pointer. No separate `DRAFT_SAVED` domain state.

**Publish gate:** `PUBLISH_READINESS_GATE=YES` — blocking errors must be zero.

---

## 2. Carrier response lifecycle

```
NOT_STARTED ──open──► IN_PROGRESS ──submit──► SUBMITTED
                         │    ▲                    │
                         │    │                    └──► LOCKED (buyer/deadline)
                    save/resume              withdraw
                         │                        │
                         └──── continue later ────┘
                                    │
                                    └──► WITHDRAWN
```

| State | Meaning |
|---|---|
| `NOT_STARTED` | Participant invited; no response workspace activity |
| `IN_PROGRESS` | Active editing; draft saves via autosave/manual save |
| `SUBMITTED` | Final carrier submission |
| `WITHDRAWN` | Carrier withdrew participation |
| `LOCKED` | No further edits (deadline passed, buyer lock, etc.) |

### 2.1 Draft save semantics

Draft saving **does not** introduce `DRAFT_SAVED` as a separate domain status.

Draft =:

```
IN_PROGRESS + last_saved_at + save_version + completion_percent
```

Capabilities: `SAVE_DRAFT`, `CONTINUE_LATER`, `RESUME` — see validation contract §14.

### 2.2 Submit gate

From `IN_PROGRESS`, transition to `SUBMITTED` requires:

```
PRE_SUBMIT_VALIDATION_GATE=YES
ERROR_COUNT = 0
```

`SUBMIT_WITH_ERRORS=FORBIDDEN`.

Warnings and knockouts do **not** block submit.

---

## 3. Answer / autosave sub-state (within IN_PROGRESS)

Per-answer UI states (client): `EMPTY`, `DIRTY`, `VALIDATING`, `INVALID`, `VALID`, `SAVING`, `SAVED`, `SAVE_FAILED`.

Server-side response remains `IN_PROGRESS` while invalid local edits exist; only **valid** batches advance `last_saved_at` / `save_version`.

---

## 4. Post-publish RFx version states

| Artifact | Mutability |
|---|---|
| Published `RfxVersion` N | **Immutable** audit record |
| New draft version N+1 | Editable until published |
| Carrier responses bound to version | Impact analysis on material change |

Transitions:

- `NON_MATERIAL` edit — may patch metadata with audit trail (policy TBD per ADR)
- `MATERIAL` edit — **must** create `NEW_RFX_VERSION`; never overwrite published history

---

## 5. Preview / test mode (buyer)

Preview sessions are **orthogonal** to carrier response state machine:

```
PREVIEW_SESSION (ephemeral)
  PREVIEW_DATA_ONLY=YES
  REAL_RESPONSE_CREATED=NO
```

No transition in carrier `Response` state machine occurs from preview activity.

---

## 6. References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_API.md](./RFX_V3_API.md)
- [RFX_V3_UX.md](./RFX_V3_UX.md)
- [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md) §14–20

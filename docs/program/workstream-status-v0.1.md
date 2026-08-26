# BINTRANS Program Workstream Status Model

## Status

Canonical status model — EDO-0.2 freeze

## Purpose

Prevent project confusion across parallel workstreams. **Do not alter active workstream runtime status without evidence.**

## Row schema

Each workstream row carries:

| Column | Description |
|--------|-------------|
| CURRENT_PHASE | Active program phase ID |
| STATUS | Phase status enum |
| OWNER | Responsible workstream lead / role |
| BASE_SHA | Git SHA evidence for status claims |
| BRANCH/PR | Active branch or PR reference |
| DEPENDENCIES | Upstream workstreams or external blockers |
| BLOCKERS | Active blockers |
| NEXT_PHASE | Recommended successor phase |

## Status enum

| STATUS | Meaning |
|--------|---------|
| DISCOVERY_COMPLETE | Read-only architecture discovery done |
| ARCHITECTURE_FREEZE | Contracts frozen; no product code |
| IMPLEMENTATION | Authorized product code in progress |
| VERIFICATION | Test/staging verification |
| BLOCKED | Cannot proceed |
| OPERATIONAL | Running in staging/production subset |

---

## Workstream registry (evidence-based snapshot)

| WS | CURRENT_PHASE | STATUS | OWNER | BASE_SHA | BRANCH/PR | DEPENDENCIES | BLOCKERS | NEXT_PHASE |
|----|---------------|--------|-------|----------|-----------|--------------|----------|------------|
| **PLAT** | PLAT-active | IMPLEMENTATION | Platform team | `a5163c3` (primary dev WT) | `test/control-tower-projection-rebuild-live-acceptance-v0.4` @ `D:\Projects\freight-platform` | — | F-002 dual-write not yet remediated | PLAT-0.1 membership cleanup |
| **LOG** | LOG-active | IMPLEMENTATION | Logistics team | `a5163c3` | same primary dev WT | PLAT | ROAD-only mode | MM-0.2 leg schema (via LOG) |
| **CT** | CT-shadow | OPERATIONAL | Control Tower team | staging pack `4d0cdfb` | `ops/bintrans-ct-staging-pack` | LOG Kafka events | PRIMARY mode disabled; shadow only | CT consume `edo.document.*` (future) |
| **FC** | FC-v2.2 | IMPLEMENTATION | Finance cost team | `d0005bd` | main-aligned | LOG, billing | Mock EDO billing path (F-003) | EDO-0.5 billing bridge |
| **EDO** | EDO-0.2 | ARCHITECTURE_FREEZE | EDO architecture | `d0005bd` | `discovery/edo-ecosystem-architecture-v0.1` | PLAT, INFRA | Legal verification; no archive storage | EDO-0.3 document extensions |
| **TEDO** | TEDO-0.2 | ARCHITECTURE_FREEZE | Transport EDO | `d0005bd` | same discovery WT | EDO, LOG | No operator licensing | TEDO-0.3 ETRN lifecycle design |
| **MM** | MM-0.2 | ARCHITECTURE_FREEZE | Multimodal | `d0005bd` | same discovery WT | LOG (shipment-service) | ROAD-only enforcement in code | MM-0.2 leg implementation request → LOG |
| **FF** | FF-v1.9 | IMPLEMENTATION | Payments/finance | `d0005bd` | payment reconciliation branches | FC | No receivable aggregate yet | FF-0.2 receivable design impl |
| **INFRA** | INFRA-staging | OPERATIONAL | DevOps | `4d0cdfb` staging pack | `ops/bintrans-ct-staging-pack` | — | S3/WORM not configured (F-004) | INFRA-0.1 object storage |

### Notes

- Primary BINTRANS development continues in `D:\Projects\freight-platform` — **untouched by EDO-0.2**.
- EDO discovery/freeze worktree: `D:\Projects\freight-platform-wt\edo-ecosystem-architecture-v0.1`.
- BASE_SHA for EDO/TEDO/MM rows reflects discovery baseline `d0005bd8b055b0d2250e5092a0c1c0484decf540` (= `origin/main` at discovery time).

## Update procedure

1. Change row only with evidence: merged PR SHA, release manifest, or operator sign-off.
2. Reference ADR or Task Contract ID in commit message when updating this file.
3. EDO agents update EDO/TEDO/MM rows only; LOG/CT/PLAT rows require evidence from those streams.

## References

- ADR-EDO-009
- Discovery PROPOSED_WORKSTREAMS
- `docs/program/cross-workstream-request-template.md`

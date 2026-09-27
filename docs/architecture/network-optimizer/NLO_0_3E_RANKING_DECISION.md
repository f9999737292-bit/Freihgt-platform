# NLO-0.3E ranking decision

```text
DOES_0_3E_NEED_RANKING=NO
RANKING_REQUIRED=NO
RANKING_DEFERRED=YES
MATCH_SCORE_REUSED=NO
SEPARATE_CONSOLIDATION_SCORE=NO
WEIGHTS_FROZEN=NO
MAX_CONTRIBUTION=RESERVED
```

ADR-NET-016 already keeps the first waves commercially neutral. The measurements show the cost is enumeration, persistence, and routing-call count. They do not show that a score would reduce that cost unless the score were also the beam key. Beam search is rejected, so a score is not required to keep the search bounded.

## Compared policies

No ranking. Return sets in lexicographic load-id order, then the existing status rank for the page. Deterministic. Auditable from the member ids. No weights. This is the frozen policy.

Deterministic lexicographic ordering. This is the same policy as no ranking. It is a sort, not a score.

A separate `ConsolidationPlanningScore`. Not authorized. Candidate dimensions that a later product rule could consider, without weights:

- incremental road detour
- residual capacity utilization
- waiting time
- new stop count
- risk or unknown count

No weights are assigned. Client-supplied weights are forbidden. BNO `MatchScore` is a next-load commercial score and is not a consolidation score.

## Ranking freeze gate

Because ranking is deferred, the score record stays empty:

```text
SCORE_PURPOSE=NONE
INPUTS=NONE
NORMALIZATION=NONE
MISSING_DATA_POLICY=not applicable
TIE_BREAKING=load id lexicographic, then status rank
VERSIONING=none
PROFILE_OWNERSHIP=none
AUDIT=candidate fingerprint already covers members, versions, and compatibility provenance
PRIVACY=a score must not be added later if it would publish another tenant's private catalog facts
WEIGHTS=UNSET
```

NLO-0.3E remains bounded feasibility only.

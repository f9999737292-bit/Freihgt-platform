# Enterprise completion methodology

Status: not approved for numeric scoring.

`origin/main` at `46e94fe42536ef106f831d70264548ef13e4951e` has no repository document that defines a weighted BINTRANS enterprise completion percent. The autopilot report must not invent one and must not move a percent because a pull request merged.

Until a controller replaces this file with an approved model, the daily report prints:

```text
ENTERPRISE_COMPLETION_PERCENT=UNCHANGED
METHODOLOGY=NOT_APPROVED_FOR_RECOMPUTATION
```

An approved replacement would need frozen weights, a source of evidence for each weight, and a rule that merged autopilot PRs do not edit those weights. That replacement is out of scope for phase 0.1A. Queue counts in the daily report are operational status, not a readiness percent.

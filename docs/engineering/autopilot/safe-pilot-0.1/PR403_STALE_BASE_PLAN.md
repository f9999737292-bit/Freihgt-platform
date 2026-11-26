# PR #403 stale-base plan

Status: plan only. This task does not rebase, merge, push, or edit PR #403.

Checked at the start of this revision:

- `origin/main` = `5432e1444ea4ed607cc8992e967023b241315759`
- PR #403 head = `a068ab6af64f14699304752956a192746dc23bf7`
- PR #403 base = `892361db87985f35132aef86550a025909af648a`
- State = OPEN draft
- J-DA-M003 = stale base

The published feature branch cannot be rebased without a force-push. This repository does not allow that from an agent task. The later update, if a controller orders it, is an ordinary merge of `origin/main` into `feat/bintrans-autopilot-24x7-readiness-v0.4a`, followed by an independent review. No supervisor code is moved onto `main` by that merge. `audit_gateway` stays on the feature branch. Budget flags stay off. Trusted reviewer storage stays unimplemented.

Until that separate task:

- do not merge PR #403;
- do not cherry-pick the trust-storage ADR onto `main`;
- do not treat the draft audit gate as present on `main`.

`AP04A-J-001` remains absent on current `main`.

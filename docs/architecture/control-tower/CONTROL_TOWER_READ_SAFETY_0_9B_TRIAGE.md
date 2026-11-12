# CONTROL-TOWER-READ-SAFETY-0.9B — Triage

Status: triage and test plan. Production handlers, Kafka consumers, SQL, and migrations are unchanged.

| Pin | Value |
|---|---|
| Base | `3f306fc407aeda4bdc652d825954292f4a54fad6` |
| Issue | #334, OPEN, "Control Tower read safety 0.9A follow-ups (3 LOW)" |
| Kafka | Issue #344 and draft PRs #346 and #357 are not part of this change |

## CT-L1 — Absolute workspace URL

`Client.resolveWorkspaceEndpoint` in `services/api-gateway/internal/controltowerreadmodel/client_workspace.go` returns the input unchanged when it contains `://`. It does not compare that URL's scheme or host with `Config.BaseURL`.

Relative paths are still joined to the base. `Config.Validate` rejects a base URL with userinfo, a query, or a fragment. That check does not run on the per-request endpoint.

A caller that already passes `baseURL + path` stays on the configured host because the base is the prefix. A caller that passes a different absolute URL is not forced back onto that host. `doWorkspaceJSON` then gives the string to `http.NewRequestWithContext`.

This is a production-code fix. It is not a test-only fix. The public gateway routes stay as they are. The change is inside the read-model client: an absolute endpoint is allowed only when its scheme and host equal the configured base, with no userinfo. Any other absolute endpoint is rejected before `Do`. Relative paths keep the current join.

## CT-L2 — Path and query encoding

`GetCase` and `CasePath` escape `caseID` with `url.PathEscape`. `CasePath` then appends `suffix` without escaping it.

These handlers build the downstream path from raw `chi.URLParam` values or from `r.URL.RawQuery`:

| Handler | Unescaped input |
|---|---|
| `RemoveCaseLink` | `caseId`, `linkId` |
| `RemoveCaseParticipant` | `caseId`, `userId` |
| `GetCaseTimeline` | `caseId`, full `RawQuery` |
| `FindCaseDuplicates` | full `RawQuery` |
| `UpdateCaseNote` | `noteId` inside `suffix` |
| `UpdateCaseAction`, `CompleteCaseAction` | `actionId` inside `suffix` |
| `UpdateCaseParticipant` | `userId` inside `suffix` |

`ListCases` is not in this set. It copies named query fields and uses `url.Values.Encode`.

The production fix escapes each path segment with `PathEscape` before concatenation. `RawQuery` is not passed through and is not path-escaped as one string. The handler parses the query, keeps an allowlist, and encodes that allowlist. Timeline and duplicate search need their allowlists taken from the current read-model contract, not invented here.

This changes the downstream URL the gateway builds. It does not add or remove a public route. It belongs in a production diff, together with tests. Shipping the tests alone would fail until the handlers change.

## CT-L3 — Three uncalled automation methods

These methods exist only as declarations. A search of `*.go` under the gateway finds no call sites:

- `Service.evaluateAndPersistRisks` in `risk.go`
- `Service.triggerExceptionAutomation` in `automation_domain_triggers.go`
- `Service.triggerExceptionSLAAutomation` in `automation_domain_triggers.go`

`evaluateAndPersistRisks` is the only caller of `triggerDomainAutomation`. That makes `triggerDomainAutomation` unreachable as well. `fireAutomationTrigger` is still called from the live automation path and is not part of this finding.

The safe check that can land before any deletion is the parser test added with this triage. It fails if one of the three names becomes a call or a value reference outside its own declaration. That test does not change runtime behavior.

Deleting the three methods is a later production diff. It must not wire them back into a request path. After deletion, recheck `triggerDomainAutomation`. Remove it in that same behavior-neutral diff only if it has no remaining reference. Do not remove `fireAutomationTrigger`.

## What is test-only now

| Item | This pull request | Later production pull request |
|---|---|---|
| CT-L3 caller absence | Parser test, expected to pass on the current tree | Delete the unused methods after the test is in place |
| CT-L1 host check | Specified below, not added, because it would fail | `resolveWorkspaceEndpoint` plus the tests |
| CT-L2 encoding | Specified below, not added, because it would fail | Segment escaping, query allowlist, plus the tests |

No public route, response code, or read-model mode default changes in this pull request.

## Regression cases for the production fix

Use `httptest`. Do not point the tests at a live broker, staging, or production.

CT-L1:

- A relative path is requested from the configured base host.
- An absolute URL with the same scheme and host as the base is requested only at that host.
- An absolute URL with another host does not open a connection. The client returns an error.
- An absolute URL with userinfo is rejected.
- `http` versus `https` on the same host is rejected when it differs from the base.

CT-L2:

- `caseId`, `linkId`, `userId`, `noteId`, and `actionId` containing `/`, `?`, `#`, or `%` are each one escaped path segment. The downstream path does not gain an extra segment.
- Timeline and duplicates forward only allowlisted query keys. A client-supplied extra key is absent from the downstream URL.
- The encoded query round-trips through `url.Values`, not through string concatenation of `RawQuery`.
- `GetCase` and `ListCases` keep their current escaping behavior.

CT-L3:

- The parser test stays green after the deletion.
- `go test` for `internal/controltower` shows no new call to the deleted names.
- A case, workspace, and automation request still returns the same status as before the deletion.

## Remediation order

1. Keep this triage free of handler and client edits. Review the parser test and this plan.
2. Implement CT-L1 and its tests in the read-model client. Do not include Kafka files.
3. Implement CT-L2 and its tests in the case handlers and `CasePath`. Keep the public routes unchanged.
4. Delete the three uncalled methods, and `triggerDomainAutomation` only if it is then unreferenced. Run the parser test and the existing control-tower package tests.
5. Leave issue #334 open until those three production changes are reviewed. Do not close issue #344.

## Acceptance

- The production diffs do not modify `services/tracking-service`, pull requests #346 or #357, migrations, or staging configuration.
- `go test` for `services/api-gateway/internal/controltower` and `services/api-gateway/internal/controltowerreadmodel` is run and reported as PASS only after it is run.
- Absolute workspace requests to another host are rejected.
- The listed path identifiers are escaped per segment, and timeline and duplicate queries are allowlisted.
- The three methods have no live call before deletion and no declaration after deletion.
- Public Control Tower routes and their status codes for ordinary identifiers stay the same.

## This stage

The parser test was added. The CT-L1 and CT-L2 production changes were not. Kafka, staging, and migrations were not changed.

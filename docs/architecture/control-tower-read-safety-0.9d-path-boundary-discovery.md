# Control Tower read safety 0.9D — path boundary discovery

Status: documentation only. No product code, migration, staging change, or merge.

```text
AGENT=C
STAGE=CONTROL-TOWER-READ-SAFETY-0.9D-PATH-BOUNDARY-DISCOVERY
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
RELATED_PR=398
RELATED_HEAD=b1f8e4749f2b8160e8a0d57061eaebe0361ad51f
PR398_STATE=OPEN_DRAFT
PR398_CHANGED=NO
ISSUE_334=OPEN
ISSUE_344=OPEN
PRODUCT_CODE_CHANGED=NO
EXTERNAL_CALLS=NO
```

`origin/main` at discovery time is `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`. Draft PR #398 remains at `b1f8e4749f2b8160e8a0d57061eaebe0361ad51f` and is not in this baseline. This document reads the gateway and the control-tower read model on `a9ef8f40`, then places the unmerged CT-L1 diff beside that tree.

## 1. Current state

Public Control Tower routes live in `services/api-gateway/internal/http/router.go`. The gateway authorizes the matched route, then builds an internal URL for `control-tower-read-model-service`. The read model takes the tenant and the user from `X-Tenant-ID` and `X-User-ID`. It does not take them from the path or the query.

On this baseline, `resolveWorkspaceEndpoint` still returns an absolute URL unchanged when the string contains `://`. That host check is the unmerged work in PR #398. It does not encode path segments or filter query keys.

Two different builders are in use:

- Named list filters are copied into a struct and encoded with `url.Values`. A client-supplied extra key is not forwarded.
- Some case and workspace paths concatenate `chi.URLParam` or `r.URL.RawQuery` directly.

The read model parses case, link, note, action, participant, view, handoff, rule, playbook, recommendation, execution, and step identifiers with `uuid.Parse` when the intended route matches. `itemType` is not a UUID. Chi matches one segment per parameter and this service does not install a path-cleaning middleware. A request that gains a slash or a `..` segment therefore tends to miss the route and fail, rather than to walk to another handler.

Issue #334 remains the tracking record for these LOW follow-ups. This discovery does not close it.

## 2. Endpoint inventory

Authorization below is the gateway check used when `authEnabled` is true. The read model then requires a UUID tenant header and, for workspace mutations, a UUID user header.

| Downstream URL | Method | Parameter source | Path encoding | Query encoding | Route can change shape | Tests for metacharacters |
| --- | --- | --- | --- | --- | --- | --- |
| `/work-items` | GET | `ParseListQuery` named fields | none in path | `url.Values` | no | ordinary only |
| `/work-items/{itemType}/{itemId}` and claim, assign, unassign | GET, POST | `chi` `itemType`, `itemId` | `fmt.Sprintf`, no `PathEscape` | none | yes, if a segment contains `%2F`, `?`, or `#` | no |
| `/work-items/bulk-action` | POST | JSON body | static path | none | no | no |
| `/workload` | GET | none | static | none | no | no |
| `/views` | GET | `workspaceScope` | static | `url.QueryEscape` | no | no |
| `/views` | POST | JSON body | static | none | no | no |
| `/views/{viewId}` and set-default | PATCH, DELETE, POST | `chi` `viewId` | `fmt.Sprintf`, no `PathEscape` | none | yes | no |
| `/handoffs` | POST | JSON body | static | none | no | no |
| `/handoffs` | GET | entire `r.URL.Query()` | static | `url.Values.Encode` of every key | query keys are not filtered | no |
| `/handoffs/{handoffId}` | GET | `chi` `handoffId` | `fmt.Sprintf`, no `PathEscape` | none | yes | no |
| `/cases` | GET | named case filters | static | `url.Values` | no | no |
| `/cases` | POST | JSON body | static | none | no | no |
| `/cases/{caseId}` and fixed suffixes `claim`, `assign`, `unassign`, `links`, `notes`, `actions`, `decisions`, `resolve`, `close`, `reopen` | GET, POST, PATCH | `CasePath`: `caseID` escaped, suffix is a constant | `url.PathEscape(caseID)` | none | no for a constant suffix | ordinary IDs in `read_safety_test.go` |
| `/cases/{caseId}/notes/{noteId}` | PATCH | `noteId` placed in the `CasePath` suffix | suffix not escaped | none | yes | no |
| `/cases/{caseId}/actions/{actionId}` and `/complete` | PATCH, POST | `actionId` in the suffix | suffix not escaped | none | yes | no |
| `/cases/{caseId}/participants/{userId}` | PATCH | `userId` in the suffix | suffix not escaped | none | yes | no |
| `/cases/{caseId}/links/{linkId}` | DELETE | raw `caseId` and `linkId` | concatenation | none | yes | ordinary IDs only |
| `/cases/{caseId}/participants/{userId}` | DELETE | raw `caseId` and `userId` | concatenation | none | yes | ordinary IDs only |
| `/cases/{caseId}/timeline` | GET | raw `caseId` plus `r.URL.RawQuery` | concatenation | raw query string | yes | ordinary path only |
| `/cases/duplicates` | GET | `r.URL.RawQuery` | static path | raw query string | query string is not parsed | ordinary path only |
| `/cases/kpi` | GET | none | static | none | no | no |
| `/automation/rules/{ruleId}` and `activate`, `disable`, `retire`, `dry-run` | GET, PATCH, POST | `AutomationRulePath` | `PathEscape(ruleID)`, constant suffix | none | no | no |
| `/playbooks/{playbookId}` | GET, PATCH | `FormatPlaybookPath` | `PathEscape` | none | no | no |
| `/automation/recommendations/{recommendationId}` and `accept`, `dismiss` | GET, POST | `AutomationRecommendationPath` | `PathEscape`, constant suffix | none | no | no |
| `/playbook-executions/{executionId}` and `start`, `complete`, `cancel` | GET, POST | `PlaybookExecutionPath` | `PathEscape`, constant suffix | none | no | no |
| `/playbook-executions/{executionId}/steps/{stepId}/start|complete|skip` | POST | `stepId` inside the suffix | execution escaped, step not escaped | none | yes | no |
| automation and playbook lists, KPI, evaluate | GET, POST | named filters or JSON | static | `url.Values` for lists | no | no |

`GetCase` escapes `caseID`. `ListViews` escapes `workspaceScope`. Those two are already safe for the scenarios in section 3.

Adjacent builders in `client_ack.go`, `client_workflow.go`, `client_risk.go`, and `client_exception.go` also interpolate `eventID` and `riskKey` without `PathEscape`. They are outside CT-L2. They are not a reason to widen this fix.

Gateway roles used by the handlers above include `CanViewWorkspace`, `CanClaimWork`, `CanAssignWork`, `CanBulkManageWork`, `CanViewTeamWorkload`, `CanViewCase`, `CanManageCase`, `CanAddCaseNote`, `CanManageCaseActions`, `CanManageCaseParticipants`, `CanResolveCase`, `CanViewAutomation`, `CanManageAutomationRules`, `CanManagePlaybooks`, `CanViewRecommendations`, `CanStartPlaybook`, and `CanManagePlaybookExecution`. Role checks do not inspect the characters inside an identifier.

## 3. Confirmed gaps and hypotheses

A `fmt.Sprintf` of a constant is not a gap. A gap exists only where a request value is copied into the downstream URL without an encoding or an allowlist, and where that value can change the path or the query the read model parses.

### Confirmed construction gaps

These builders copy request data into the downstream URL without `PathEscape` or a query allowlist:

- `case_handler.go` `RemoveCaseLink`, `RemoveCaseParticipant`, `GetCaseTimeline`, and `FindCaseDuplicates`.
- `CasePath` suffixes used for a note, an action, and a participant update. `caseID` is escaped. The second identifier is not.
- `client_workspace.go` formats for `itemType`, `itemId`, `viewId`, and `handoffId`.
- `ListHandoffs` encodes every query key the caller sent.
- `PlaybookExecutionPath` receives `steps/` + raw `stepId` + `/start|complete|skip`.

`GetCaseTimeline` and `FindCaseDuplicates` append `r.URL.RawQuery` as text. The read model currently reads only `page` and `limit` for the timeline, and `itemType`, `itemId`, and `shipmentId` for duplicates. Unknown keys are ignored. The SQL observed for handoffs binds `tenant_id`, `from_user_id`, `to_user_id`, and `limit` as parameters. This is not SQL injection. It is still a query-boundary gap: the gateway forwards keys the read model does not document, and a later reader could honor one of them.

`ListHandoffs` on the read model applies `fromUser` and `toUser` inside the verified tenant. The gateway does not add a second user constraint. That is an existing tenant-scoped filter, not a cross-tenant write. CT-L2 keeps those two keys and does not redesign who may list handoffs.

### Already safe

- List endpoints that copy named fields and call `url.Values.Encode`. An extra public query key stops at the gateway.
- `GetCase`, automation rule, playbook, recommendation, and execution identifiers that pass through `url.PathEscape` with a constant suffix.
- `ListViews` `workspaceScope`.
- Read-model UUID parsers, once the intended route still matches. `..`, an empty string, and a non-UUID fail the parse. Tenant isolation for a matched route stays on the header, not the path.
- A raw `/` or a single `%2F` in a public path does not become one chi parameter. The gateway router splits on the decoded slash first, so the handler is not entered.

### Hypotheses that are not a demonstrated bypass

- `..` as a whole segment is accepted by chi and is not removed by `url.PathEscape`. The read model does not run `path.Clean`. The literal `..` segment fails the UUID parse or misses the route. It is not evidence that the request reaches another handler. CT-L2 still rejects `.` and `..`, because a later cleaner would change that result.
- `%252F` reaches the gateway handler as the characters `%2F`. The read model decodes that into a slash and the segment count changes. On the current chi routes the extra segments do not select a different mounted handler; the likely result is 404. That is a confirmed shape change and not a confirmed data read.
- `%3F` and `%23` can put `?` or `#` into a segment. `http.NewRequest` then treats them as a query or a fragment, so the downstream path is shorter. Where the shortened path is not a mounted route, the call fails. Where it is the same role as the original route, it is not a privilege change. The tests in section 4 have to record the actual downstream path.
- Unicode and fullwidth slashes stay one segment. They are not a second encoding of `/`. UUID fields reject them. `itemType` would currently forward them.
- Control characters are not shown to survive `http.NewRequest`. CT-L2 rejects them before the client call rather than depending on that behavior.
- No cross-tenant path was found. Both services bind the tenant from the authenticated header.

No external service was called. These distinctions are from the route tables, the builders, and the read-model parsers on `a9ef8f40`.

## 4. Adversarial test matrix

The implementation tests use `httptest` only. Each row is one identifier or one query string. The assertion is the downstream URL and the status, not a live read model.

| Input | Expected downstream result |
| --- | --- |
| Ordinary UUID on every builder in section 2 | Same path and status as today. One segment. No new query key. |
| `/` inside an identifier | Gateway route does not treat it as one parameter, or the builder rejects it before `Do`. |
| `%2F` | Does not add a segment on the read-model side. |
| `%252F` | Remains one segment after the read-model decode. It is not decoded back into `/`. |
| `..` and `.` | Rejected. `PathEscape` alone is not sufficient. |
| `?`, `#`, `%` | Rejected or encoded so they do not start a query, a fragment, or a second escape. |
| Empty identifier | Rejected. The public `{param}` route already fails to match an empty segment. |
| Unicode and a fullwidth solidus | Rejected for UUID fields. `itemType` must match one token of letters, digits, `_`, and `-`. |
| CR, LF, NUL, and other controls | Rejected before `http.Client.Do`. |
| Extra path segment produced by any of the above | The recorded downstream path has the same segment count as the ordinary route. |
| Timeline query with `page`, `limit`, and an extra key | Only `page` and `limit` are forwarded. |
| Duplicates query with `itemType`, `itemId`, `shipmentId`, and an extra key | Only those three keys are forwarded. |
| Handoff list with `limit`, `fromUser`, `toUser`, and an extra key | Only those three keys are forwarded. |
| `tenant` or `user` smuggled in the path or query | `X-Tenant-ID` and `X-User-ID` stay the values from the gateway request context. |
| Ordinary workspace GET and POST after the path fix | Same status as the current happy path. |

A test that only checks the gateway status, without recording the URL the read-model double saw, does not satisfy this matrix.

## 5. CT-L1 compatibility

PR #398, unmerged, changes `resolveWorkspaceEndpoint` and the workspace `http.Client` copy in `client_workspace.go`. Relative paths still join `baseURL`. An absolute URL must share scheme, host, and port with that base. Cross-origin redirects are refused before the next request. The caller's `CheckRedirect` is not replaced.

CT-L2 runs before that check, while the URL is still being built:

1. Validate or encode each segment and build a relative path, or an absolute path that already starts with the configured base.
2. Let the CT-L1 resolver accept that URL only when it stays on the configured origin.
3. Do not put userinfo, a foreign host, or a second scheme into the path fix.
4. Do not change the redirect copy or the static error text from #398.

`client_workspace.go` is on both sides. Editing the `a9ef8f40` copy of that file in a CT-L2 branch conflicts with #398. The implementation waits until #398 is merged, or it is stacked on `b1f8e4749f2b8160e8a0d57061eaebe0361ad51f` and touches only the `fmt.Sprintf` path sites. It does not rework `resolveWorkspaceEndpoint`. If the stack does not apply cleanly, implementation stops.

`case_handler.go`, `client_case.go`, and `automation_handler.go` are not part of #398. They can be changed in the same CT-L2 diff after the stack base is chosen. They are not a reason to edit #398 itself.

## 6. CT-L2 implementation scope

Future diff only. This discovery does not apply it.

Allowed files:

- `services/api-gateway/internal/controltower/case_handler.go`
- `services/api-gateway/internal/controltowerreadmodel/client_case.go`
- `services/api-gateway/internal/controltowerreadmodel/client_workspace.go`, limited to the work-item, view, and handoff path formats and to the handoff query copy
- `services/api-gateway/internal/controltower/automation_handler.go`, limited to `stepId` before it enters the suffix
- one new `_test.go` beside the handlers or the read-model client

Allowed behavior:

- UUID fields are parsed before they are placed in a path. A valid UUID keeps today's path text. An invalid UUID is a validation error and does not call the read model.
- `itemType` is one token of ASCII letters, digits, `_`, and `-`. The known values `exception`, `risk`, `driver_event`, and `tracking` remain valid. The check is a token rule, not a frozen enum, so a later type of the same shape does not require another path change.
- `.`, `..`, empty, `/`, `\`, `?`, `#`, `%`, and control characters are rejected for every dynamic segment, including a token that would otherwise pass.
- Each segment that is still interpolated is passed through `url.PathEscape` after validation.
- Timeline forwards only `page` and `limit`. Duplicates forward only `itemType`, `itemId`, and `shipmentId`. Handoffs forward only `limit`, `fromUser`, and `toUser`. Values are encoded with `url.Values`.
- Ordinary successful routes keep their method, path, and status.

Not allowed:

- CT-L3 deletion, Kafka, tracking, producer authority, issuer issuance, or decision consumption.
- Read-model route or SQL changes. The gateway must emit a path the current read model already routes.
- Public route additions or removals.
- A second change to the CT-L1 origin and redirect policy.
- Merging #398 as part of the implementation.

Negative tests are the matrix in section 4. Tenant isolation is the header assertion in that matrix, plus one case where the public query contains another tenant id and the downstream header does not. Compatibility is the ordinary UUID and token rows, run with the existing `internal/controltower` and `internal/controltowerreadmodel` packages.

Stop if a negative input reaches a different mounted read-model route, if a valid current identifier changes its downstream path, if the branch cannot stack on #398 without editing the origin resolver, or if the fix needs a read-model change. Independent review by Agent J happens after the implementation diff and its exact-head CI. J checks the recorded downstream URLs, not only the gateway status codes.

## 7. Risks and blockers

- Implementation on raw `a9ef8f40` conflicts with the open `client_workspace.go` diff in #398. That is a blocker for the workspace half of CT-L2, not for this document.
- `url.PathEscape` does not neutralize `.` or `..`. Validation has to reject them.
- Rejecting every non-UUID at the gateway returns 400 for callers that today receive the read model's validation error. The path of a valid UUID does not change. The stop condition covers a valid identifier whose path does change.
- `itemType` is intentionally not a UUID. A token rule that is narrower than the values already stored would break ordinary work items. The four observed values must be included.
- Handoff `fromUser` and `toUser` remain tenant-scoped filters. Tightening them to the caller would be a product change and is out of scope.
- Event and risk path interpolation stays open and is not part of this recommendation.
- Issue #334 stays open until the production CT-L2 diff and the separate CT-L3 deletion are reviewed. Issue #344 stays open.

## 8. Go / No-Go

The path-boundary contract is ready for architecture review.

```text
GO_FOR_ARCHITECTURE_REVIEW=YES
CT_L2_IMPLEMENTATION=NO_GO_UNTIL_PR398_IS_MERGED_OR_STACKED
CT_L2_READY_FOR_IMPLEMENTATION=NO
CONFIRMED_SECURITY_GAPS=UNESCAPED_SEGMENTS,RAW_QUERY_FORWARDING
CONFIRMED_CROSS_TENANT_BYPASS=NO
CONFIRMED_CROSS_ROUTE_READ=NO
FINAL_VERDICT=READY_FOR_J_ARCHITECTURE_REVIEW
```

CT-L2 implementation starts only on a tree that already contains PR #398, and only in the files in section 6. It does not start by editing the origin resolver, and it does not merge this discovery.

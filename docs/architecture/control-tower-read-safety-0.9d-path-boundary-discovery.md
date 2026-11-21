# Control Tower read safety 0.9D — path boundary discovery

Status: documentation only. Revision R2 after independent review. No product code, migration, staging change, or merge. CT-L2 is not authorized by this revision.

```text
AGENT=C
STAGE=CONTROL-TOWER-READ-SAFETY-0.9D-R2
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
PREVIOUS_HEAD=971309e13815244e8d6f7a298af5c4d95b6522df
RELATED_PR=398
RELATED_HEAD=b1f8e4749f2b8160e8a0d57061eaebe0361ad51f
PR398_CHANGED=NO
J_CT09C_001=OPEN
CT_L2_IMPLEMENTATION_GATE=BLOCKED_UNTIL_CT_L1_SECURITY_REVIEW_AND_INTEGRATION
CT_L2_IMPLEMENTATION_READY=NO
ISSUE_334=OPEN
ISSUE_344=OPEN
PRODUCT_CODE_CHANGED=NO
```

`origin/main` for this revision is `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`. Draft PR #398 remains at `b1f8e4749f2b8160e8a0d57061eaebe0361ad51f`. This document does not edit that branch. The route and parser facts below are from that main tree. The unmerged CT-L1 diff is only a dependency.

The previous revision of this file is withdrawn where it conflicts with the sections below. In particular, this revision does not claim 33 endpoint groups, does not claim that `%252F` stays one segment for the whole decode chain, and does not claim `CONFIRMED_CROSS_ROUTE_ACCESS=NO`.

The per-finding text for `J-CT09D-001` through `J-CT09D-006` was not in the PR review, the issue comments, or the repository. This revision uses the controller summary and the code. It does not invent a severity or a quote for an individual finding id. Where the exact review bytes are missing, the cell says `DETAILS_NOT_AVAILABLE`.

## 1. How the routes were counted

Source file: `services/api-gateway/internal/http/router.go`, the `if controlTower != nil` block, lines 106–182.

Three different counts:

| Term | Rule | Count |
| --- | --- | --- |
| Registered route | One `r.Get`, `r.Post`, `r.Patch`, or `r.Delete` call in that block | 75 |
| CT-L2 public surface | Those registrations whose path is not `/summary`, does not contain `/critical-events/`, and does not contain `/risks` | 64 |
| Dynamic registration | A registration on that 64-route surface whose path contains `{` | 46 |
| Static registration | A registration on that 64-route surface whose path contains no `{` | 18 |
| Endpoint group | Defined in the next paragraph | 27 |

64 + the excluded 11 (1 summary, 6 critical-event, 4 risk) = 75. 46 dynamic + 18 static = 64. The same `{` rule on the full block is 55 dynamic and 20 static. The review’s starting values, 64 public and 46 dynamic, match the CT-L2 surface, not the whole block.

An endpoint group collapses methods and constant suffixes that add no path parameter. A parameter set is split when the query construction differs. Timeline is its own group because it appends `r.URL.RawQuery`. The other `caseId`-only routes share `CasePath` or `url.PathEscape` and stay one group. That rule produces 27 groups. The arithmetic is in the inventory: workspace 8 groups and 15 registrations, cases 9 groups and 24 registrations, automation 10 groups and 25 registrations. 15 + 24 + 25 = 64.

This revision does not restore a count of 33 groups. No counting rule on this tree produces 33.

The read-model router in `services/control-tower-read-model-service/internal/http/router.go` has 86 `Get`/`Post`/`Patch`/`Delete` registrations, two of them conditional on `executionRepo`. That file also mounts shipment status, acknowledgement, and workflow routes that have no twin in the public block above. 86 is not the public count.

Authorization in the table is the gateway check used when `authEnabled` is true. The read model then takes the tenant and the user from `X-Tenant-ID` and `X-User-ID`. It does not take them from the path or the query. `CanViewWorkspace`, `CanClaimWork`, `CanAssignWork`, `CanBulkManageWork`, `CanViewTeamWorkload`, `CanCreateHandoff`, `CanViewCase`, `CanCreateCase`, `CanManageCase`, `CanAssignCase`, `CanAddCaseNote`, `CanManageCaseActions`, `CanResolveCase`, `CanManageCaseParticipants`, `CanViewAutomation`, `CanViewRecommendations`, `CanStartPlaybook`, and `CanManagePlaybookExecution` all reduce to `CanAccessControlTower`. `CanManageAutomationRules` and `CanManagePlaybooks` require `PLATFORM_ADMIN`. None of these checks inspect the characters inside an identifier.

Read-model targets are the handlers on `/internal/v1/control-tower/...`.

## 2. Route inventory

| Route | Method | Group | Dynamic params | Gateway handler | Read-model target | Authorization |
| --- | --- | --- | --- | --- | --- | --- |
| `/api/v1/control-tower/work-items` | GET | G1 work items | none | `ListWorkItems` | `ListWorkItems` | `CanViewWorkspace` |
| `/api/v1/control-tower/work-items/{itemType}/{itemId}` | GET | G2 work item | `itemType`, `itemId` | `GetWorkItem` | `GetWorkItem` | `CanViewWorkspace` |
| `/api/v1/control-tower/work-items/{itemType}/{itemId}/claim` | POST | G2 work item | `itemType`, `itemId` | `ClaimWorkItem` | `ClaimWorkItem` | `CanClaimWork` |
| `/api/v1/control-tower/work-items/{itemType}/{itemId}/assign` | POST | G2 work item | `itemType`, `itemId` | `AssignWorkItem` | `AssignWorkItem` | `CanAssignWork` |
| `/api/v1/control-tower/work-items/{itemType}/{itemId}/unassign` | POST | G2 work item | `itemType`, `itemId` | `UnassignWorkItem` | `UnassignWorkItem` | `CanAssignWork` |
| `/api/v1/control-tower/work-items/bulk-action` | POST | G3 bulk | none | `BulkWorkItemsAction` | `BulkAction` | `CanBulkManageWork` |
| `/api/v1/control-tower/workload` | GET | G4 workload | none | `GetWorkload` | `GetWorkload` | `CanViewTeamWorkload` |
| `/api/v1/control-tower/views` | GET | G5 views | none | `ListSavedViews` | `ListViews` | `CanViewWorkspace` |
| `/api/v1/control-tower/views` | POST | G5 views | none | `CreateSavedView` | `CreateView` | `CanViewWorkspace` |
| `/api/v1/control-tower/views/{viewId}` | PATCH | G6 view | `viewId` | `UpdateSavedView` | `UpdateView` | `CanViewWorkspace` |
| `/api/v1/control-tower/views/{viewId}` | DELETE | G6 view | `viewId` | `DeleteSavedView` | `DeleteView` | `CanViewWorkspace` |
| `/api/v1/control-tower/views/{viewId}/set-default` | POST | G6 view | `viewId` | `SetDefaultSavedView` | `SetDefaultView` | `CanViewWorkspace` |
| `/api/v1/control-tower/handoffs` | GET | G7 handoffs | none | `ListHandoffs` | `ListHandoffs` | `CanViewWorkspace` |
| `/api/v1/control-tower/handoffs` | POST | G7 handoffs | none | `CreateHandoff` | `CreateHandoff` | `CanCreateHandoff` |
| `/api/v1/control-tower/handoffs/{handoffId}` | GET | G8 handoff | `handoffId` | `GetHandoff` | `GetHandoff` | `CanViewWorkspace` |
| `/api/v1/control-tower/cases/kpi` | GET | G9 case kpi | none | `GetCaseKPI` | `GetKPI` | `CanViewCase` |
| `/api/v1/control-tower/cases/duplicates` | GET | G10 duplicates | none | `FindCaseDuplicates` | `FindDuplicates` | `CanViewCase` |
| `/api/v1/control-tower/cases` | GET | G11 cases | none | `ListCases` | `ListCases` | `CanViewCase` |
| `/api/v1/control-tower/cases` | POST | G11 cases | none | `CreateCase` | `CreateCase` | `CanCreateCase` |
| `/api/v1/control-tower/cases/{caseId}` | GET | G12 case | `caseId` | `GetCase` | `GetCase` | `CanViewCase` |
| `/api/v1/control-tower/cases/{caseId}` | PATCH | G12 case | `caseId` | `UpdateCase` | `UpdateCase` | `CanManageCase` |
| `/api/v1/control-tower/cases/{caseId}/claim` | POST | G12 case | `caseId` | `ClaimCase` | `ClaimCase` | `CanAssignCase` |
| `/api/v1/control-tower/cases/{caseId}/assign` | POST | G12 case | `caseId` | `AssignCase` | `AssignCase` | `CanAssignCase` |
| `/api/v1/control-tower/cases/{caseId}/unassign` | POST | G12 case | `caseId` | `UnassignCase` | `UnassignCase` | `CanAssignCase` |
| `/api/v1/control-tower/cases/{caseId}/links` | POST | G12 case | `caseId` | `AddCaseLink` | `AddLink` | `CanManageCase` |
| `/api/v1/control-tower/cases/{caseId}/notes` | POST | G12 case | `caseId` | `CreateCaseNote` | `CreateNote` | `CanAddCaseNote` |
| `/api/v1/control-tower/cases/{caseId}/actions` | POST | G12 case | `caseId` | `CreateCaseAction` | `CreateActionItem` | `CanManageCaseActions` |
| `/api/v1/control-tower/cases/{caseId}/decisions` | POST | G12 case | `caseId` | `CreateCaseDecision` | `CreateDecision` | `CanManageCase` |
| `/api/v1/control-tower/cases/{caseId}/resolve` | POST | G12 case | `caseId` | `ResolveCase` | `ResolveCase` | `CanResolveCase` |
| `/api/v1/control-tower/cases/{caseId}/close` | POST | G12 case | `caseId` | `CloseCase` | `CloseCase` | `CanResolveCase` |
| `/api/v1/control-tower/cases/{caseId}/reopen` | POST | G12 case | `caseId` | `ReopenCase` | `ReopenCase` | `CanResolveCase` |
| `/api/v1/control-tower/cases/{caseId}/participants` | POST | G12 case | `caseId` | `AddCaseParticipant` | `AddParticipant` | `CanManageCaseParticipants` |
| `/api/v1/control-tower/cases/{caseId}/links/{linkId}` | DELETE | G13 link | `caseId`, `linkId` | `RemoveCaseLink` | `RemoveLink` | `CanManageCase` |
| `/api/v1/control-tower/cases/{caseId}/notes/{noteId}` | PATCH | G14 note | `caseId`, `noteId` | `UpdateCaseNote` | `UpdateNote` | `CanAddCaseNote` |
| `/api/v1/control-tower/cases/{caseId}/actions/{actionId}` | PATCH | G15 action | `caseId`, `actionId` | `UpdateCaseAction` | `UpdateActionItem` | `CanManageCaseActions` |
| `/api/v1/control-tower/cases/{caseId}/actions/{actionId}/complete` | POST | G15 action | `caseId`, `actionId` | `CompleteCaseAction` | `CompleteActionItem` | `CanManageCaseActions` |
| `/api/v1/control-tower/cases/{caseId}/participants/{userId}` | PATCH | G16 participant | `caseId`, `userId` | `UpdateCaseParticipant` | `UpdateParticipant` | `CanManageCaseParticipants` |
| `/api/v1/control-tower/cases/{caseId}/participants/{userId}` | DELETE | G16 participant | `caseId`, `userId` | `RemoveCaseParticipant` | `RemoveParticipant` | `CanManageCaseParticipants` |
| `/api/v1/control-tower/cases/{caseId}/timeline` | GET | G17 timeline | `caseId` | `GetCaseTimeline` | `GetTimeline` | `CanViewCase` |
| `/api/v1/control-tower/automation/kpi` | GET | G18 automation kpi | none | `GetAutomationKPI` | `GetAutomationKPI` | `CanViewAutomation` |
| `/api/v1/control-tower/automation/rules` | GET | G19 rules | none | `ListAutomationRules` | `ListRules` | `CanViewAutomation` |
| `/api/v1/control-tower/automation/rules` | POST | G19 rules | none | `CreateAutomationRule` | `CreateRule` | `CanManageAutomationRules` |
| `/api/v1/control-tower/automation/rules/{ruleId}` | GET | G20 rule | `ruleId` | `GetAutomationRule` | `GetRule` | `CanViewAutomation` |
| `/api/v1/control-tower/automation/rules/{ruleId}` | PATCH | G20 rule | `ruleId` | `UpdateAutomationRule` | `UpdateRule` | `CanManageAutomationRules` |
| `/api/v1/control-tower/automation/rules/{ruleId}/activate` | POST | G20 rule | `ruleId` | `ActivateAutomationRule` | `ActivateRule` | `CanManageAutomationRules` |
| `/api/v1/control-tower/automation/rules/{ruleId}/disable` | POST | G20 rule | `ruleId` | `DisableAutomationRule` | `DisableRule` | `CanManageAutomationRules` |
| `/api/v1/control-tower/automation/rules/{ruleId}/retire` | POST | G20 rule | `ruleId` | `RetireAutomationRule` | `RetireRule` | `CanManageAutomationRules` |
| `/api/v1/control-tower/automation/rules/{ruleId}/dry-run` | POST | G20 rule | `ruleId` | `DryRunAutomationRule` | `DryRunRule` | `CanManageAutomationRules` |
| `/api/v1/control-tower/automation/recommendations` | GET | G21 recommendations | none | `ListRecommendations` | `ListRecommendations` | `CanViewRecommendations` |
| `/api/v1/control-tower/automation/recommendations/{recommendationId}` | GET | G22 recommendation | `recommendationId` | `GetRecommendation` | `GetRecommendation` | `CanViewRecommendations` |
| `/api/v1/control-tower/automation/recommendations/{recommendationId}/accept` | POST | G22 recommendation | `recommendationId` | `AcceptRecommendation` | `AcceptRecommendation` | `CanStartPlaybook` |
| `/api/v1/control-tower/automation/recommendations/{recommendationId}/dismiss` | POST | G22 recommendation | `recommendationId` | `DismissRecommendation` | `DismissRecommendation` | `CanViewRecommendations` |
| `/api/v1/control-tower/playbooks` | GET | G23 playbooks | none | `ListPlaybooks` | `ListPlaybooks` | `CanManagePlaybooks` |
| `/api/v1/control-tower/playbooks` | POST | G23 playbooks | none | `CreatePlaybook` | `CreatePlaybook` | `CanManagePlaybooks` |
| `/api/v1/control-tower/playbooks/{playbookId}` | GET | G24 playbook | `playbookId` | `GetPlaybook` | `GetPlaybook` | `CanManagePlaybooks` |
| `/api/v1/control-tower/playbooks/{playbookId}` | PATCH | G24 playbook | `playbookId` | `UpdatePlaybook` | `UpdatePlaybook` | `CanManagePlaybooks` |
| `/api/v1/control-tower/playbook-executions` | GET | G25 executions | none | `ListPlaybookExecutions` | `ListExecutions` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}` | GET | G26 execution | `executionId` | `GetPlaybookExecution` | `GetExecution` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}/start` | POST | G26 execution | `executionId` | `StartPlaybookExecution` | `StartExecution` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}/complete` | POST | G26 execution | `executionId` | `CompletePlaybookExecution` | `CompleteExecution` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}/cancel` | POST | G26 execution | `executionId` | `CancelPlaybookExecution` | `CancelExecution` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}/steps/{stepId}/start` | POST | G27 step | `executionId`, `stepId` | `StartPlaybookExecutionStep` | `StartExecutionStep` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}/steps/{stepId}/complete` | POST | G27 step | `executionId`, `stepId` | `CompletePlaybookExecutionStep` | `CompleteExecutionStep` | `CanManagePlaybookExecution` |
| `/api/v1/control-tower/playbook-executions/{executionId}/steps/{stepId}/skip` | POST | G27 step | `executionId`, `stepId` | `SkipPlaybookExecutionStep` | `SkipExecutionStep` | `CanManagePlaybookExecution` |

The table has 64 rows, 46 of them dynamic, in 27 groups. Summary, critical events, and risks stay outside CT-L2. Their builders in `client_ack.go`, `client_workflow.go`, `client_risk.go`, and `client_exception.go` still interpolate `eventID` and `riskKey` without `PathEscape`. That remains an open adjacent gap. It is not added to the CT-L2 file list.

## 3. URL decoding model

Checked with `net/http`, `net/url`, and chi v5.2.1 against `httptest`. The temporary probe was deleted and is not part of this change. No external host was called.

A raw `/` is a segment boundary before chi. `{param}` does not capture it.

| Input in one public segment | Gateway chi `URLParam` | `URL.Path` at the gateway | After the builder string is passed to `http.NewRequest` | Read-model chi |
| --- | --- | --- | --- | --- |
| `%2F` | The characters `%2F`, and the route still matches | Contains a real `/` | `Path` contains a real `/`. `EscapedPath` keeps `%2F` | `URLParam` is again `%2F`. `Path` contains a real `/` |
| `%252F` | The characters `%2F` | Still contains the characters `%2F` | Same as the `%2F` row: `Path` has a slash, `EscapedPath` keeps `%2F` | Same as the `%2F` row |
| `%3F` | Contains `?` | Contains `?` | `?` starts the query. The path loses everything after it | The shortened path is what the router matches |
| `%23` | Contains `#` | Contains `#` | `#` starts a fragment. The fragment is not sent. The path loses everything after it | The shortened path is what the router matches |
| `%25` followed by hex | Contains `%` plus the following text | Contains `%` | `url.Parse` treats `%` plus two hex digits as one byte | Depends on the rebuilt request line |
| `.` and `..` | The literal `.` or `..` | The same literal segment | `PathEscape(".")` is `.` and `PathEscape("..")` is `..`. Both survive | The literal segment still matches `{param}` |
| `\` (`%5C`) | A backslash | A backslash | `EscapedPath` emits `%5C`. One segment | One segment |
| CR or LF | The control character | The control character | `http.NewRequest` returns `invalid control character in URL` | The call is not sent when `NewRequest` fails |
| Unicode, including U+FF0F and U+2215 | One segment. These are not an ASCII `/` | One segment | One segment, percent-encoded in `EscapedPath` | One segment |

`%252F` is not guaranteed to remain one segment after the whole chain. The gateway parameter is the text `%2F`. `http.NewRequest` then sets `URL.Path` to a path that contains a real slash. Chi’s `URLParam` can still report one parameter because it reads the raw request line, while `URL.Path` has already split. Any later stage that rebuilds from `Path`, or that cleans the path, sees the extra slash. The earlier sentence that `%252F` remains one segment after the read-model decode is withdrawn.

`url.PathEscape` is not a defense against `.` or `..`. It also does not reject `%`. `PathEscape("%2F")` is `%252F`, which is the input that becomes a slash in `URL.Path` on the next parse.

CT-L2 boundary, for every dynamic identifier whose proven contract has no percent sign: reject `%` before the downstream URL is built. UUID text and the `itemType` token contain no `%`, so this reject does not remove a proven identifier. The same pre-check rejects `.`, `..`, an empty string, `/`, `\`, `?`, `#`, and control characters. `PathEscape` runs only after that check, so a surviving character cannot open a query, a fragment, or a second escape. This is a gateway reject. It does not add a read-model route.

CT-L1 does not close this. The origin check and the redirect guard run on the URL after it has been built. They do not stop a query that was created by `?` inside a segment, and they do not restore a path that a `#` or a decoded slash already shortened.

## 4. CONFIRMED_CROSS_ROUTE_ACCESS

```text
CONFIRMED_CROSS_ROUTE_ACCESS=YES
CONFIRMED_PRIVILEGE_ESCALATION=NO
CONFIRMED_CROSS_TENANT_LEAK=NO
```

Agent J’s review established a route transition. The checked case used the same role and the same tenant. It did not establish a privilege increase, and it did not establish a cross-tenant read or write. The request bytes from that review were not in the materials available for this revision (`DETAILS_NOT_AVAILABLE`). The scenario below is the one reproduced from the builders on `a9ef8f40`. It has those same outcome properties. It is the reproducible case this document can show. It is not labeled as a second, stronger finding.

| Field | Value |
| --- | --- |
| Public route | `GET /api/v1/control-tower/cases/{caseId}/timeline` |
| Input | The `caseId` segment is `11111111-1111-1111-1111-111111111111%3F`. Chi’s `URLParam` is `11111111-1111-1111-1111-111111111111?`. The public route still matches, because `%3F` is not a slash. |
| Downstream path | `GetCaseTimeline` concatenates `"/internal/v1/control-tower/cases/" + caseID + "/timeline?" + r.URL.RawQuery`. `http.NewRequest` stops the path at `?`. The path that is sent is `/internal/v1/control-tower/cases/11111111-1111-1111-1111-111111111111`. The characters `/timeline` become the query. |
| Read-model handler | `GET /cases/{caseId}` selects `caseHandler.GetCase`, not `GetTimeline`. |
| Authorization on the source route | `CanViewCase` when `authEnabled` is true. |
| Authorization on the target route | `CanViewCase`. |
| Tenant isolation | `X-Tenant-ID` and `X-User-ID` are still the gateway request context. The path does not select the tenant. |
| Read or write | The method stays GET. The selected handler reads the case. This reproduction does not call a mutation and does not introduce a second tenant. |

A `#` in the same position drops `/timeline` as a fragment and also selects `GetCase`. The fragment is not transmitted.

This is a confirmed change of route. It is not a demonstrated privilege escalation: both handlers use `CanViewCase`. It is not a demonstrated cross-tenant leak. `CONFIRMED_CROSS_ROUTE_ACCESS=NO` is withdrawn.

## 5. Query allowlist

Current readers:

- Timeline (`GetTimeline`) reads `page` and `limit` through `parseIntDefault`. Empty, a non-integer, and any integer below 1 return the fallback (page 1, limit 50). The response status stays 200.
- Duplicates (`FindDuplicates`) reads `itemType`, `itemId`, and `shipmentId`. `shipmentId` is applied only when `uuid.Parse` succeeds. An invalid value leaves the filter unset. Status stays 200.
- Handoffs (`ListHandoffs`) reads `limit` through `parseIntDefault` (fallback 20), and `fromUser` and `toUser` through `uuid.Parse`. An invalid user id leaves that filter unset. Status stays 200.
- `url.Values.Get` and `URL.Query().Get` return the first value. `ListHandoffs` on the gateway encodes the entire `r.URL.Query()`, so a duplicate key is present on the wire and the read model still uses the first. Timeline and duplicates append `r.URL.RawQuery` as text.
- Unknown keys are ignored by these three readers. They are still forwarded today.
- SQL for these filters uses placeholders. This is not SQL injection.

The proposed gateway behavior does not change those filter results. A stricter 400 for an unknown key, a bad number, or a bad filter UUID would be a product change and needs a controller decision. It is not part of CT-L2.

| Case | Current behavior | Proposed behavior | Read-model compatibility | HTTP status | Required test |
| --- | --- | --- | --- | --- | --- |
| Repeated key | `Get` uses the first value. Handoff `Encode` still writes every value. | Copy the first value once with `url.Values.Set`. | The reader already uses `Get`, so the filter result is unchanged. | The status of the same request with one value. | Downstream query contains the key once. Record the URL. |
| Empty value | `Get` returns `""`. Numbers use the fallback. UUID filters stay unset. | Omit an empty allowlisted value. That is the same as an absent key. | Yes. `Get` of a missing key is also `""`. | 200 when the remaining input is valid. | Empty `page`, `limit`, `fromUser`, `toUser`, `shipmentId`. Record the URL. |
| Unknown key | Ignored by the reader and still forwarded by `RawQuery` or full `Encode`. | Drop it. Do not answer 400. | The reader never applied it. | Same status as the request without that key. | `extra=1` is absent from the downstream query. |
| Invalid UUID (`fromUser`, `toUser`, `shipmentId`) | Parse fails, filter stays unset, status 200. | Forward the first value unchanged so the read model keeps that result. | Yes. This does not turn the filter into a 400. | 200. | Invalid `fromUser` still reaches the read model and does not replace `X-Tenant-ID`. |
| Invalid number (`page`, `limit`) | `parseIntDefault` returns the fallback. Status 200. | Forward the value. Do not answer 400. | Yes. The fallback stays in the read model. | 200, with the fallback page or limit. | `page=abc` and `limit=0` keep today’s fallback. Record the URL. |
| Several values of one key | First value wins at `Get`. The wire may contain the rest. | Forward only the first. | Yes. | 200 when that first value is otherwise valid. | `limit=1&limit=99` produces one `limit`. |
| Encoded characters in a value | `Query()` decodes once. `Encode` writes the canonical form. `RawQuery` forwards the original text. | Parse, keep the allowlisted first value, and `Encode`. | The read model decodes `Query()` again. One decode of a normal value is preserved. | 200 for an allowlisted value. A `%` here is query text, not an identifier, so the identifier reject does not apply. | A value containing `%3F` stays one query value. Record the URL. |

`tenant` and `user` in the query are unknown keys. They are dropped. They must not be copied into `X-Tenant-ID` or `X-User-ID`.

Allowlists, unchanged from the previous proposal:

- Timeline: `page`, `limit`.
- Duplicates: `itemType`, `itemId`, `shipmentId`.
- Handoffs: `limit`, `fromUser`, `toUser`.

## 6. Dynamic identifier policy

Validation happens before `PathEscape` and before `http.NewRequest`. A rejected identifier does not call the read model. The proposed gateway status is 400. Today the read model returns 400 for a UUID it cannot parse (`VALIDATION_ERROR`). Moving that reject to the gateway changes where the 400 is produced. It does not change the path of a value that is already valid.

`PathEscape` of a canonical UUID, and of the `itemType` token, does not change the characters. The original trimmed text is escaped after validation. The value is not reformatted, so an uppercase UUID that `uuid.Parse` accepts keeps the text the gateway sends today.

The route parameter for a participant is `userId`. There is no `participantId` parameter.

| Identifier | Type in the handler | UUID required | Allowed | Empty | Forbidden before `PathEscape` | Validation | Encoding | Malformed | Negative tests | Old clients |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `itemType` | String. `GetWorkItem` passes it through. Constants `exception` and `risk`. Stored literals `driver_event` and `tracking`. | No | One token `[A-Za-z0-9_-]+` | Reject | `% / \ ? #`, a token that is `.` or `..`, controls, any other character | Token rule, not a frozen enum | `PathEscape` after the token check | 400, no downstream call | `%`, `/`, `.`, `..`, empty, Unicode | The four observed values stay valid |
| `itemId` | String source id. `GetWorkItem` compares `SourceID` as a string and does not call `uuid.Parse`. | No. A UUID-shaped value is still a string. | Any text that survives the forbidden-character reject. No positive alphabet was proven for every stored source id. | Reject | `% / \ ? # \`, the whole value `.` or `..`, CR, LF, other controls | Reject that set, then escape | `PathEscape` of the remaining text | 400, no downstream call | The forbidden set, including `%252F` | A source id without those characters keeps one segment. This rule does not start requiring a UUID. |
| `viewId` | `uuid.Parse` in `UpdateView`, `DeleteView`, `SetDefaultView` | Yes | Text `uuid.Parse` accepts | Reject | `% / \ ? #`, `.`, `..`, controls, and any text `uuid.Parse` rejects | Forbidden-character reject, then `uuid.Parse` | `PathEscape` of the original trimmed text | 400, no downstream call | Non-UUID, `%2F`, `%252F`, `.`, `..` | A UUID that is valid today keeps its path text |
| `handoffId` | `uuid.Parse` in `GetHandoff` | Yes | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | Same as `viewId` | 400 | Same as `viewId` | Same as `viewId` |
| `caseId` | `parseCaseID` calls `uuid.Parse` | Yes | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | Same as `viewId` | 400 | Same as `viewId`, plus the timeline `%3F` case | Same as `viewId` |
| `noteId` | `uuid.Parse` in `UpdateNote` | Yes | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | Same as `viewId` | 400 | Same as `viewId` | Same as `viewId` |
| `actionId` | `uuid.Parse` in `UpdateActionItem` and `CompleteActionItem` | Yes | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | Same as `viewId` | 400 | Same as `viewId` | Same as `viewId` |
| `userId` (participant) | `uuid.Parse` in `UpdateParticipant` and `RemoveParticipant` | Yes | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | Same as `viewId` | 400 | Same as `viewId` | Same as `viewId` |
| `stepId` | `parseUUIDParam(r, "stepId")` in `mutateExecutionStep` | Yes. This is handler evidence, not a separate domain declaration. | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | `PathEscape` before the id is placed in the `steps/...` suffix | 400 | Same as `viewId` | Same as `viewId` |
| `ruleId`, `playbookId`, `recommendationId`, `executionId` | `parseUUIDParam` on each of those names | Yes | Same as `viewId` | Reject | Same as `viewId` | Same as `viewId` | `PathEscape` of the primary id. The suffix stays a constant. | 400 | One non-UUID and one `%` case per id | A valid id keeps today’s path text |

Future CT-L2 file scope is sufficient for this contract and is not sufficient for the adjacent event and risk builders:

- `services/api-gateway/internal/controltower/case_handler.go` — raw `caseId`, `linkId`, `userId`, timeline, and duplicates query.
- `services/api-gateway/internal/controltowerreadmodel/client_case.go` — `CasePath` suffixes for `noteId`, `actionId`, and participant `userId`.
- `services/api-gateway/internal/controltowerreadmodel/client_workspace.go` — the `fmt.Sprintf` paths for `itemType`, `itemId`, `viewId`, and `handoffId`, and the handoff query copy. Not `resolveWorkspaceEndpoint` and not `CheckRedirect`.
- `services/api-gateway/internal/controltower/automation_handler.go` — `stepId` before it enters the suffix. The other automation ids already go through `PathEscape` and gain the UUID pre-check at that call site only if the implementation touches it; the current gap in this file is `stepId`.
- One new `_test.go` next to those handlers or the read-model client.

No read-model file, no public route file, and no event or risk client is in that list.

## 7. Dependency on CT-L1

```text
J_CT09C_001=OPEN
CT_L2_IMPLEMENTATION_GATE=BLOCKED_UNTIL_CT_L1_SECURITY_REVIEW_AND_INTEGRATION
```

PR #398 is the unmerged CT-L1 change. On `a9ef8f40`, `resolveWorkspaceEndpoint` still returns an absolute URL unchanged when the string contains `://`. The #398 diff binds the workspace client to the configured origin and rejects a cross-origin redirect on a per-request client copy.

`J-CT09C-001` stays open. The risk is that a caller-supplied `CheckRedirect` can still need protection against a URL that changes after the origin check. This document does not patch that. PR #398 is not edited from this branch.

CT-L2, when it is eventually authorized, stacks on the reviewed CT-L1 tree. It does not:

- Weaken the origin check.
- Weaken the redirect guard.
- Copy `X-Tenant-ID` or `X-User-ID` onto a foreign origin.
- Start from the unprotected `client_workspace.go` on old main.
- Merge PR #398 by itself.

Query truncation and a path whose shape changes inside a segment are not problems CT-L1 already solved. They are the subject of sections 3 and 4.

## 8. Adversarial test matrix

Future tests use `httptest` only. Each row records the downstream URL the test server received, and the gateway status. A status with no recorded URL does not pass the row. No external network call is part of the matrix.

| Test | Required observation |
| --- | --- |
| Timeline `%3F` case from section 4 | Downstream path is `/internal/v1/control-tower/cases/{uuid}` and the handler selected by that path is `GetCase`. Status recorded. Same `X-Tenant-ID` as the caller. |
| `%2F` in `itemId`, `caseId`, and `stepId` | Rejected before `NewRequest`, or the recorded path has no extra slash. The temporary probe showed chi still matching while `URL.Path` contains a slash, so the assertion is the path, not only `URLParam`. |
| `%252F` in the same identifiers | Rejected. The row fails if the recorded `URL.Path` contains a slash that was not in the ordinary route. |
| `.` and `..` | Rejected. `PathEscape` left both unchanged in the probe. |
| Invalid UUID for `viewId`, `handoffId`, `caseId`, `noteId`, `actionId`, `userId`, `stepId`, `ruleId`, `playbookId`, `recommendationId`, `executionId` | 400 and zero requests at the test server. |
| `itemType` token and non-UUID `itemId` | `exception`, `risk`, `driver_event`, and `tracking` keep one segment. `itemId` is not required to be a UUID. `%`, `.`, and `..` are rejected for both. |
| Repeated query key | One copy of the key on the recorded query. |
| Empty query value | The empty key is absent. Status matches an absent key. |
| Unknown query key | Absent from the recorded query. Status matches the request without it. |
| `tenant` or `user` in the query | Absent from the downstream query. `X-Tenant-ID` and `X-User-ID` equal the gateway context. |
| Downstream headers | The test server records `X-Tenant-ID` and `X-User-ID` on the ordinary route and on the rejected-identifier route (no request). |
| Ordinary GET, POST, PATCH, and DELETE | One route of each method from section 2 keeps today’s path and status. The recorded URL matches the current builder for a valid id. |

## 9. Findings

Severity and the reviewer’s own evidence text are `DETAILS_NOT_AVAILABLE` for every id. Nothing in this table is `CLOSED`.

| Finding ID | Severity | Evidence | Remediation | Status |
| --- | --- | --- | --- | --- |
| `J-CT09D-001` | `DETAILS_NOT_AVAILABLE` | `DETAILS_NOT_AVAILABLE` | Section 1 and section 2 replace the earlier inventory. The verified counts are 27 groups, 64 public registrations on the CT-L2 surface, and 46 dynamic registrations. The full block is 75. The claim of 33 groups is not repeated. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-002` | `DETAILS_NOT_AVAILABLE` | `DETAILS_NOT_AVAILABLE` | Section 3 withdraws the guarantee that `%252F` stays one segment, records `URL.Path` versus `URLParam`, and sets the CT-L2 reject of `%` before the downstream URL is built. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-003` | `DETAILS_NOT_AVAILABLE` | `DETAILS_NOT_AVAILABLE` | Section 4 sets `CONFIRMED_CROSS_ROUTE_ACCESS=YES`, documents the reproduced timeline transition, and does not call it a privilege escalation or a cross-tenant leak. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-004` | `DETAILS_NOT_AVAILABLE` | `DETAILS_NOT_AVAILABLE` | Section 5 defines duplicate, empty, unknown, invalid UUID, invalid number, multi-value, and encoded query behavior without changing the read-model filter result. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-005` | `DETAILS_NOT_AVAILABLE` | `DETAILS_NOT_AVAILABLE` | Section 6 states `itemId` is a string and `stepId` is a UUID only because `parseUUIDParam` parses it. The file list covers that contract. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-006` | `DETAILS_NOT_AVAILABLE` | `DETAILS_NOT_AVAILABLE` | Section 7 leaves `J-CT09C-001` open and sets `CT_L2_IMPLEMENTATION_GATE=BLOCKED_UNTIL_CT_L1_SECURITY_REVIEW_AND_INTEGRATION`. | `DOCUMENT_REMEDIATED_PENDING_J` |

The controller described three MEDIUM findings and three LOW findings without naming which id has which severity. All six are addressed in this document. None is closed, because Agent J has not reconfirmed them. No LOW item is left open inside the docs-only scope: the inventory, the decode model, the route transition, the query table, the identifier contracts, and the CT-L1 gate are all written down. A product change for unknown-query 400, or for invalid filter-UUID 400, is explicitly deferred to the controller and is not required to accept this architecture text.

## 10. Implementation gate

CT-L2 implementation does not start from this revision.

```text
GO_FOR_J_R2_ARCHITECTURE_REVIEW=YES
CT_L2_IMPLEMENTATION_READY=NO
CT_L2_IMPLEMENTATION_GATE=BLOCKED_UNTIL_CT_L1_SECURITY_REVIEW_AND_INTEGRATION
CONFIRMED_CROSS_ROUTE_ACCESS=YES
CONFIRMED_PRIVILEGE_ESCALATION=NO
CONFIRMED_CROSS_TENANT_LEAK=NO
J_CT09C_001=OPEN
FINAL_VERDICT=READY_FOR_J_R2_ARCHITECTURE_REVIEW
```

Issue #334 stays open. Issue #344 stays open. Production consumer readiness stays blocked.

# Control Tower read safety 0.9D — path boundary discovery

Status: documentation only. Revision R4 after independent review. No product code, migration, staging change, or merge. CT-L2 is not authorized by this revision.

```text
AGENT=C
STAGE=CONTROL-TOWER-READ-SAFETY-0.9D-R4
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
PREVIOUS_HEAD=d54af4e1d94cbe8c5bbe7889a08b9809739490aa
RELATED_PR=398
RELATED_HEAD=b1f8e4749f2b8160e8a0d57061eaebe0361ad51f
PR398_CHANGED=NO
J_CT09C_001=OPEN
CT_L1_INTEGRATION=NOT_COMPLETE
CT_L2_IMPLEMENTATION_READY=NO
PUBLIC_STATUS_CHANGE_APPROVED=NO
ISSUE_334=OPEN
ISSUE_344=OPEN
PRODUCT_CODE_CHANGED=NO
```

`origin/main` for this revision is `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c`. Draft PR #398 remains at `b1f8e4749f2b8160e8a0d57061eaebe0361ad51f`. This document does not edit that branch. The route and parser facts below are from that main tree. The unmerged CT-L1 diff is only a dependency.

The previous revision of this file is withdrawn where it conflicts with the sections below. In particular, this revision does not claim 33 endpoint groups, does not claim that `%252F` stays one segment for the whole decode chain, and does not claim `CONFIRMED_CROSS_ROUTE_ACCESS=NO`.

Section 11 uses Agent J's finding numbers, including the R2 mapping for `J-CT09D-007` through `J-CT09D-010`. This revision does not add a finding id.

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

Checked with `net/http`, `net/url`, and chi v5.2.1 against `httptest`, including requests whose `URL.RawPath` was set explicitly. The temporary probe was deleted and is not part of this change. No external host was called. These observations are on `a9ef8f40`. Section 8 says where PR #398 changes the result. A URL parse error is not a substitute for an explicit reject.

One decode and a second decode are different steps.

- `%252F` decoded once is the three characters `%2F`. It is not yet `/`.
- Parsing that `%2F` text as a URL decodes it again. `URL.Path` then contains `/`. `EscapedPath` can still show `%2F`.
- `URL.Path` and `URL.RawPath` are different fields. With `RawPath` set to `/api/v1/control-tower/work-items/exception/abc%252Fdef` and `Path` containing a slash, chi `URLParam` returned `abc%252Fdef`. With `RawPath` empty and `Path` containing the characters `%2F`, `URLParam` returned `abc%2Fdef`. `URLParam` follows the raw segment when `RawPath` is set. It does not guarantee that `Path` has one segment.

| Input | What was observed |
| --- | --- |
| `%2F` | Chi can still match `{param}` and return the characters `%2F`. `URL.Path` contains a real `/`. |
| `%252F` | First decode yields the text `%2F`. The second decode, inside `http.NewRequest`, puts `/` in `URL.Path`. This is not one segment for the whole chain. |
| `%25zz` | First decode of the public segment `a%25zz` yields `a%zz`. The next step depends on the builder, as the paragraphs after this table record. |
| `%3F` | The parameter contains `?`. `http.NewRequest` starts the query there and shortens the path. The rows in section 4 are this case. |
| `%23` | The parameter contains `#`. On this main tree the fragment is not sent and the path stops before it. On the PR #398 client the fragment rejects the call before `Do`. |
| `%5C` | The parameter contains `\`. On this main tree `EscapedPath` emits `%5C` and the segment stays one. On the PR #398 client a backslash rejects the call before `Do`. |
| `%0A` | The parameter contains a line feed. `http.NewRequest` returns `invalid control character in URL` when the builder concatenates that character. That error is not the validation rule. |
| Unicode, including U+FF0F and U+2215 | One segment. These are not an ASCII `/`. UUID parse rejects them. `itemType` would currently forward them. |
| Empty segment | `GET /api/v1/control-tower/cases//timeline` matched `{caseId}` with an empty parameter on chi v5.2.1. CT-L2 still rejects an empty identifier before the downstream URL is built. |
| `.` and `..` | `url.PathEscape(".")` is `.` and `url.PathEscape("..")` is `..`. Chi matches both as `{param}`. `PathEscape` is not the control. |

`http.NewRequest` does not treat `%25zz` the same way on every route. The public segment `a%25zz` is one chi parameter whose value is `a%zz`.

Concat and `fmt.Sprintf` builders place that value in the URL string unchanged. Timeline (`GetCaseTimeline`), link removal (`RemoveCaseLink`), note update (the raw `noteId` suffix), and the work-item sprintf paths all do this. `http.NewRequest` then returns `invalid URL escape "%zz"`. `Do` is not called. The gateway maps that error through `mapWorkspaceDependencyError` to HTTP 503. Downstream request count is 0.

`GetCase` is the other builder. It calls `url.PathEscape` on `caseId` before `http.NewRequest`. `PathEscape("a%zz")` is `a%25zz`, so `NewRequest` succeeds and the request is sent. `URL.Path` contains `/internal/v1/control-tower/cases/a%zz`. The recorded probe response for that delivered request was HTTP 200. That 200 is the probe observation for this builder. It is not a claim that every route returns 200, and it is not a change to the public status policy in section 7.

A parse error on the concat builders is not the validation rule. CT-L2 still rejects `%` before either builder runs, so `GetCase` cannot remain a probe that sends the request while the concat builders fail closed.

CT-L2 rejects `%` in every dynamic identifier before the downstream URL is built. UUID text and the `itemType` token contain no `%`. The same pre-check rejects `.`, `..`, an empty string, `/`, `\`, `?`, `#`, and control characters. `PathEscape` runs only after that check.

## 4. CONFIRMED_CROSS_ROUTE_ACCESS

```text
CONFIRMED_CROSS_ROUTE_ACCESS=YES
CONFIRMED_PRIVILEGE_ESCALATION=NO
CONFIRMED_CROSS_TENANT_LEAK=NO
PRIVILEGE_ESCALATION_IMPOSSIBLE=NOT_CLAIMED
```

Four groups were reproduced on `a9ef8f40` with chi v5.2.1. `PRIVILEGE_ESCALATION_PROVEN=NO` means this input did not show a higher role or a second tenant. It does not mean a route change cannot become one under a different role split or a different body.

The gateway check named below is the check that runs on the public route. The target check is the check registered for the downstream route. The target check does not run, because the gateway already authorized the source route.

### A. Timeline to GetCase

```text
PUBLIC_ROUTE=GET /api/v1/control-tower/cases/{caseId}/timeline
INPUT=caseId segment 11111111-1111-1111-1111-111111111111%3F
DOWNSTREAM_METHOD=GET
DOWNSTREAM_PATH=/internal/v1/control-tower/cases/11111111-1111-1111-1111-111111111111
DOWNSTREAM_QUERY=/timeline?
TARGET_HANDLER=GetCase
SOURCE_AUTHORIZATION=CanViewCase
TARGET_AUTHORIZATION=CanViewCase
TENANT_HEADER_PRESERVED=YES
PRIVILEGE_ESCALATION_PROVEN=NO
CROSS_TENANT_ACCESS_PROVEN=NO
```

`GetCaseTimeline` concatenates `"/internal/v1/control-tower/cases/" + caseID + "/timeline?" + r.URL.RawQuery`. Chi's parameter is `11111111-1111-1111-1111-111111111111?`. The first `?` ends the path. The read-model route `GET /cases/{caseId}` selects `GetCase`. The query `/timeline?` is on that request. `GetCase` does not read it. `X-Tenant-ID` is unchanged. Both named checks are `CanViewCase`.

### B. Claim to bulk action

```text
PUBLIC_ROUTE=POST /api/v1/control-tower/work-items/{itemType}/{itemId}/claim
INPUT=itemType segment bulk-action%3F, itemId segment x
DOWNSTREAM_METHOD=POST
DOWNSTREAM_PATH=/internal/v1/control-tower/work-items/bulk-action
DOWNSTREAM_QUERY=/x/claim
TARGET_HANDLER=BulkAction
SOURCE_AUTHORIZATION=CanClaimWork
TARGET_AUTHORIZATION=CanBulkManageWork
TENANT_HEADER_PRESERVED=YES
PRIVILEGE_ESCALATION_PROVEN=NO
CROSS_TENANT_ACCESS_PROVEN=NO
```

Chi's parameters are `itemType=bulk-action?` and `itemId=x`. `ClaimWorkItem` formats `"/internal/v1/control-tower/work-items/%s/%s/claim"`. `http.NewRequest` sends `POST /internal/v1/control-tower/work-items/bulk-action` with query `/x/claim`. The read-model route is `BulkAction`.

`ClaimWorkItem` sends a nil body. `BulkAction` requires JSON `items`. An empty body fails that decode with the read model's validation error. This input selects the bulk handler and does not apply a bulk item list. `CanClaimWork` and `CanBulkManageWork` both reduce to `CanAccessControlTower` today. That is the checked case. It is not a proof that a different body, or a later split of those two role functions, stays inside the caller's rights.

On the PR #398 client this `?` URL is still accepted: it is same-origin, it has no fragment, and it has no backslash. The route switch remains after CT-L1.

### C. KPI route

```text
PUBLIC_ROUTE=GET /api/v1/control-tower/cases/{caseId}/timeline
INPUT=caseId segment kpi%3F
DOWNSTREAM_METHOD=GET
DOWNSTREAM_PATH=/internal/v1/control-tower/cases/kpi
DOWNSTREAM_QUERY=/timeline?
TARGET_HANDLER=GetKPI
SOURCE_AUTHORIZATION=CanViewCase
TARGET_AUTHORIZATION=CanViewCase
TENANT_HEADER_PRESERVED=YES
PRIVILEGE_ESCALATION_PROVEN=NO
CROSS_TENANT_ACCESS_PROVEN=NO
```

The same timeline concatenation with parameter `kpi?` sends `GET /internal/v1/control-tower/cases/kpi`. That route is `GetKPI`, not `GetTimeline` and not `GetCase`. The query `/timeline?` is on the KPI request. `GetKPI` does not read it. Both named checks are `CanViewCase`. The PR #398 client accepts this relative URL for the same reason as group B.

### D. Query injection

`?`, and a `%3F` that the public parser has already turned into `?`, moves the remainder into `RawQuery`. `#`, and a decoded `%23`, starts a fragment. On this main tree the fragment is not transmitted. On the PR #398 client a fragment rejects the call before `Do`. A later `&` splits query pairs only after a `?` is already present. The read-model handlers below do not apply those query keys. The keys are still on the downstream request.

UpdateNote:

```text
PUBLIC_ROUTE=PATCH /api/v1/control-tower/cases/{caseId}/notes/{noteId}
INPUT=noteId segment aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa%3Fextra=1
DOWNSTREAM_METHOD=PATCH
DOWNSTREAM_PATH=/internal/v1/control-tower/cases/11111111-1111-1111-1111-111111111111/notes/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
DOWNSTREAM_QUERY=extra=1
TARGET_HANDLER=UpdateNote
SOURCE_AUTHORIZATION=CanAddCaseNote
TARGET_AUTHORIZATION=CanAddCaseNote
TENANT_HEADER_PRESERVED=YES
PRIVILEGE_ESCALATION_PROVEN=NO
CROSS_TENANT_ACCESS_PROVEN=NO
```

`CasePath` escapes `caseId` only. The suffix is `notes/` plus the raw `noteId`. The note id that arrives is the UUID before `?`. `UpdateNote` reads that path id and the body. It does not read `extra`. The query `extra=1` is on the request.

RemoveCaseLink, which is the DELETE route:

```text
PUBLIC_ROUTE=DELETE /api/v1/control-tower/cases/{caseId}/links/{linkId}
INPUT=linkId segment bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb%3Ftenant=other
DOWNSTREAM_METHOD=DELETE
DOWNSTREAM_PATH=/internal/v1/control-tower/cases/11111111-1111-1111-1111-111111111111/links/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
DOWNSTREAM_QUERY=tenant=other
TARGET_HANDLER=RemoveLink
SOURCE_AUTHORIZATION=CanManageCase
TARGET_AUTHORIZATION=CanManageCase
TENANT_HEADER_PRESERVED=YES
PRIVILEGE_ESCALATION_PROVEN=NO
CROSS_TENANT_ACCESS_PROVEN=NO
```

`RemoveCaseLink` concatenates `caseId` and `linkId`. The link id that `RemoveLink` parses is the UUID before `?`. The query `tenant=other` arrives on the DELETE request. `RemoveLink` takes the tenant from `X-Tenant-ID`, not from that query. This case does not show a second tenant. It does show a caller-controlled query on a DELETE.

## 5. Query allowlist

Order for timeline, duplicates, and handoffs:

1. Parse the public query with `url.ParseQuery`.
2. For each allowlisted key, take the first value only.
3. If that first value is empty, omit the key. Do not fall through to a later value.
4. Ignore every later value of the same key.
5. Drop keys that are not on the allowlist.
6. Encode the result with `url.Values.Encode`.

`limit=&limit=5` parses as first value empty and second value `5`. The key is omitted. The read model applies its fallback. The downstream query is not `limit=5`.

`limit=5&limit=` keeps `limit=5`, because the first value is `5`.

`page=&page=2` omits `page`. The fallback page is 1.

`pa%67e=2` parses as key `page` and value `2`. After the standard parse it is the allowlisted key, so it is forwarded as `page=2`.

`itemId=%3F` parses as value `?`. Duplicates forwards that one value. `Encode` writes `itemId=%3F`. The read model receives `?` as the item id string. This does not change the duplicate filter rule.

An invalid `fromUser`, `toUser`, or `shipmentId` is forwarded unchanged. The read model leaves that filter unset and answers 200. CT-L2 does not turn that into a 400.

An unknown key is dropped. The status matches the same request without that key.

`tenant` and `user` are unknown keys. They are dropped. They are not copied into `X-Tenant-ID` or `X-User-ID`.

Allowlists: timeline `page` and `limit`; duplicates `itemType`, `itemId`, and `shipmentId`; handoffs `limit`, `fromUser`, and `toUser`.

Current readers stay as they are. Timeline and handoffs use `parseIntDefault`: empty, a non-integer, and an integer below 1 use the fallback, and the status stays 200. Duplicate and handoff UUID filters apply only when `uuid.Parse` succeeds. `Get` already returns the first value. Unknown keys are ignored by these readers and are still forwarded today. The gateway change stops forwarding them. It does not change the filter result.

## 6. Dynamic identifier policy

Validation happens before `PathEscape` and before `http.NewRequest`. A rejected identifier does not call the read model. The public status of that local reject is section 7. It is not the read model's 400, and it is not a 400 today.

`PathEscape` of a canonical UUID, and of the `itemType` token, does not change the characters. The original trimmed text is escaped after validation. The participant parameter is `userId`. There is no `participantId` parameter.

| Identifier | Type in the handler | UUID required | Forbidden before `PathEscape` | Malformed |
| --- | --- | --- | --- | --- |
| `itemType` | String. Constants `exception` and `risk`. Stored literals `driver_event` and `tracking`. | No. One token `[A-Za-z0-9_-]+`, not a frozen enum. | `% / \ ? #`, a token that is `.` or `..`, controls, any other character | No downstream call. The four observed values stay valid. |
| `itemId` | String source id. `GetWorkItem` compares `SourceID` and does not call `uuid.Parse`. | No | `% / \ ? #`, the whole value `.` or `..`, controls | No downstream call. A source id without those characters keeps one segment. |
| `viewId` | `uuid.Parse` in the view handlers | Yes | `% / \ ? #`, `.`, `..`, controls, plus any text `uuid.Parse` rejects | No downstream call |
| `handoffId` | `uuid.Parse` in `GetHandoff` | Yes | Same as `viewId` | No downstream call |
| `caseId` | `parseCaseID` calls `uuid.Parse` | Yes | Same as `viewId` | No downstream call |
| `linkId` | `uuid.Parse` in read-model `RemoveLink`. The gateway source is `RemoveCaseLink`, which concatenates the raw parameter. | Yes | `% / \ ? #`, `.`, `..`, controls, plus any text `uuid.Parse` rejects | No downstream call. The query-injection row in section 4 is this DELETE route. |
| `noteId` | `uuid.Parse` in `UpdateNote` | Yes | Same as `viewId` | No downstream call |
| `actionId` | `uuid.Parse` in `UpdateActionItem` and `CompleteActionItem` | Yes | Same as `viewId` | No downstream call |
| `userId` | `uuid.Parse` in `UpdateParticipant` and `RemoveParticipant` | Yes | Same as `viewId` | No downstream call |
| `stepId` | `parseUUIDParam(r, "stepId")` in `mutateExecutionStep` | Yes. This is handler evidence. | Same as `viewId` | No downstream call. Escape the id before it enters the `steps/` suffix. |
| `ruleId`, `playbookId`, `recommendationId`, `executionId` | `parseUUIDParam` on each name | Yes | Same as `viewId` | No downstream call. The suffix stays a constant. |

## 7. Public HTTP status

```text
CURRENT_PUBLIC_STATUS=503
PROPOSED_PUBLIC_STATUS=400_FOR_LOCAL_REJECT
PUBLIC_STATUS_CHANGE_APPROVED=NO
```

A malformed identifier that reaches the read model is a read-model validation error. `mapWorkspaceDependencyError` keeps 409 as conflict and 404 as not found. Every other read-model status, including 400 and 403, becomes `ControlTowerReadModelUnavailable`. `respond.Error` maps that code to HTTP 503. The checked public status for this class is 503. The R2 sentence that described the current public result as 400 is withdrawn.

`apperrors.Validation` is already mapped to 400, but the identifier paths do not return it today. They call the read model and then hit the 503 mapper.

CT-L2 rejects the bad identifier before `http.Client.Do`. Two ways to surface that:

Option A. Return the error the current mapper already turns into 503, and do not call the read model. Existing clients that treat this class as 503 stay compatible. The downstream count is 0, which is the security property. Files: `case_handler.go`, `client_case.go`, `client_workspace.go`, `automation_handler.go`, and the new test file. Negative tests expect public 503 and `DOWNSTREAM_REQUEST_COUNT=0`. 404 and 409 from a real read-model response stay on their current branches. The risk appears only if the default branch of `mapWorkspaceDependencyError` is edited; that edit is not part of option A.

Option B. Return `apperrors.Validation` from the gateway handler before the client call. `respond.Error` already maps that code to 400, so `workspace.go` is not required. The additional file is `workspace_handler.go`, because work-item, view, and handoff parameters are read there. A reject inside `client_workspace.go` alone still returns a `DependencyError` and becomes 503 through `mapWorkspaceDependencyError`. Negative tests expect public 400 and `DOWNSTREAM_REQUEST_COUNT=0`. Clients that branch on 503 for a bad id see a new 400. 403 from the read model stays 503 unless `workspace.go` is changed. Changing that default branch could remap 403 together with 400. That is why `workspace.go` is not the minimal file.

Recommendation: use 400 for a parameter the gateway rejects locally. That public change is not approved.

```text
PUBLIC_STATUS_CHANGE_APPROVED=NO
STATUS_FILE_SCOPE=PENDING_CONTROLLER_APPROVAL
```

Until that approval, CT-L2 does not switch the public status. `workspace_handler.go` and `workspace.go` are not added to the authorized list. `workspace_handler.go` is the minimal file if option B is approved. `workspace.go` stays out of both options.

## 8. Dependency on CT-L1

```text
J_CT09C_001=OPEN
CT_L1_INTEGRATION=NOT_COMPLETE
CT_L2_IMPLEMENTATION_READY=NO
```

The notes below are from `resolveWorkspaceEndpoint` and `workspaceHTTPClient` at `b1f8e4749f2b8160e8a0d57061eaebe0361ad51f`. They are not the behavior of `a9ef8f40`, and the main-tree probe is not reused as if the stack behaved the same way.

- `#`. `url.Parse` sets `Fragment`. The resolver returns `errWorkspaceEndpointRejected` before `NewRequest`. On main, the same string is parsed by `NewRequest`, the fragment is dropped, and the shortened path is sent.
- `\`. `strings.ContainsAny` rejects `\`, space, tab, CR, LF, and NUL before `url.Parse`. On main, `\` is encoded as `%5C` and remains one segment.
- `?` in a relative endpoint. The resolver joins it to `baseURL` and does not reject it. `http.NewRequest` then splits the path. Groups B and C still select `BulkAction` and `GetKPI` on this stack.
- Origin. An absolute URL must be `http` or `https`, with no userinfo and no opaque part, and scheme, host, and port must match the configured base. A foreign host is rejected before `Do`.
- Redirect. The per-request client checks the redirect URL's origin before the inherited `CheckRedirect`. A foreign origin, userinfo, fragment, or opaque redirect returns `errWorkspaceRedirectRejected`.
- `J-CT09C-001`. After that origin check, the inherited `CheckRedirect` still runs. A caller-supplied function that changes the URL after the check is not blocked by the code at `b1f8e474`. This revision does not patch it, and it does not edit PR #398.

Query truncation and a path shortened by `?` are not problems CT-L1 already solved.

Future CT-L2 does not weaken the origin check, does not weaken the redirect guard, does not copy tenant or user headers onto a foreign origin, does not start from the unprotected `client_workspace.go` on old main, and does not merge PR #398 by itself.

## 9. Future file scope

Path and query construction, with the public status left at 503 until the controller decides:

- `services/api-gateway/internal/controltower/case_handler.go`
- `services/api-gateway/internal/controltowerreadmodel/client_case.go`
- `services/api-gateway/internal/controltowerreadmodel/client_workspace.go`, limited to the sprintf paths and the handoff query copy. Not `resolveWorkspaceEndpoint` and not `CheckRedirect`.
- `services/api-gateway/internal/controltower/automation_handler.go`, limited to `stepId` before the suffix
- one or more `_test.go` files beside those handlers or the client

`workspace_handler.go` is `PENDING_CONTROLLER_APPROVAL`. It is required only for option B, so a local reject returns 400. `workspace.go` is not included. The read model, the public router, and the event and risk clients stay out.

## 10. Adversarial test matrix

Future tests use `httptest` only. Each row records the gateway status, the downstream URL, the handler the path selects, and the number of downstream requests. A forbidden identifier passes only when `REQUEST_REJECTED=YES` and `DOWNSTREAM_REQUEST_COUNT=0`. A row that allows either a reject or a preserved route does not pass. No external network call is part of the matrix. Tests that stack on PR #398 use the resolver at `b1f8e474`, not the main-tree client.

| Test | Required observation |
| --- | --- |
| Section 4 group A, timeline `%3F` | Downstream path is `/cases/{uuid}`, handler `GetCase`, query begins with `/timeline`. |
| Section 4 group B, `bulk-action%3F` | Downstream path is `/work-items/bulk-action`, method POST, handler `BulkAction`. |
| Section 4 group C, `kpi%3F` | Downstream path is `/cases/kpi`, handler `GetKPI`. |
| Section 4 UpdateNote `%3F` | Query `extra=1` is on the recorded URL. Handler is `UpdateNote`. |
| Section 4 RemoveCaseLink `%3F` | Method DELETE. Query `tenant=other` is on the recorded URL. Handler is `RemoveLink`. `X-Tenant-ID` is the caller tenant. |
| Forbidden `linkId`, including `%` | `REQUEST_REJECTED=YES`, `DOWNSTREAM_REQUEST_COUNT=0`. |
| `itemId` as an opaque string | `exception` and a non-UUID source id keep one segment. `%` is rejected with count 0. |
| `stepId` as a UUID | A valid UUID keeps the step path. A non-UUID has count 0. |
| `%252F` with `RawPath` set | The assertion reads both `URL.Path` and `URLParam`. The forbidden form is rejected with count 0. |
| `%25zz` on `GetCase` | Current probe: the request is sent, `URL.Path` contains `a%zz`, and the probe response was HTTP 200. Required CT-L2 behavior: `REQUEST_REJECTED=YES` and `DOWNSTREAM_REQUEST_COUNT=0`, so this route is not an oracle. |
| `%25zz` on timeline, link, note, and work-item builders | Current behavior: `http.NewRequest` returns `invalid URL escape "%zz"`, public status 503, downstream count 0. Required CT-L2 behavior is the same count with a local reject before `NewRequest`, not an accidental parse error. |
| Empty segment, as in `/cases//timeline` | Rejected with count 0. |
| `.` and `..` | Rejected with count 0. |
| `limit=&limit=5` | The recorded query has no `limit`. It is not `limit=5`. |
| `limit=5&limit=` | The recorded query has one `limit=5`. |
| Invalid filter UUID | Forwarded. The read model leaves the filter unset. Header tenant is unchanged. |
| `tenant` and `user` in the query | Absent from the downstream query. Headers stay the gateway context. |
| Option A status | Public 503 and count 0. |
| Option B status | Public 400 and count 0. This row is not a pass condition until `PUBLIC_STATUS_CHANGE_APPROVED=YES`. |
| Same `%3F` groups on the CT-L1 client | `?` still selects `BulkAction` and `GetKPI`. `#` and `\` do not call `Do`. |

## 11. Findings

J's previous status is the status the controller supplied. This revision does not mark a finding closed.

| Finding | Previous status | Correction | Status awaiting J |
| --- | --- | --- | --- |
| `J-CT09D-001` | `CLOSED_WITH_NIT` | Inventory in sections 1 and 2 is unchanged. No new evidence reopens it. | `CLOSED_WITH_NIT` |
| `J-CT09D-002` | `CLOSED` | Not reopened. The extra decode rows in section 3 are the new LOW items, not a new claim against this id. | `CLOSED` |
| `J-CT09D-003` | `PARTIAL` | Section 4 now has the timeline, claim-to-bulk, KPI, UpdateNote, and RemoveCaseLink cases, with method, path, query, both authorizations, and the tenant result. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-004` | `PARTIAL` | Section 5 takes the first value and drops the key when that first value is empty. `limit=&limit=5` does not become `limit=5`. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-005` | `PARTIAL` | Section 6 adds `linkId` as a UUID from `RemoveLink`. Section 7 replaces the public-400 claim with the checked 503 path. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-006` | `PARTIAL` | Section 8 separates main from the `b1f8e474` resolver for `#`, `\`, `?`, origin, and redirect, and leaves `J-CT09C-001` open. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-007` | `NEW_LOW` | `linkId` policy. Section 6: UUID from `RemoveLink`, rejected before `PathEscape`, and the DELETE query row in section 4. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-008` | `NEW_LOW` | Current public HTTP 503. Section 7: a read-model validation error is mapped to 503. The proposed local 400 stays unapproved. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-009` | `NEW_LOW` | File list and downstream oracle. Section 9 lists the CT-L2 files. Section 10 requires `REQUEST_REJECTED=YES` and `DOWNSTREAM_REQUEST_COUNT=0`. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-010` | `NEW_LOW` | `RawPath`, `%252F`, and `%25zz`. Section 3 records `RawPath` against `URLParam`, the two-step `%252F` decode, and the builder split for `%25zz`. Section 10 repeats that split. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-011` | `LOW` | R4 corrects this table so 007 is `linkId`, 008 is public 503, 009 is the file list and downstream oracle, and 010 is `RawPath`, `%252F`, and `%25zz`. | `DOCUMENT_REMEDIATED_PENDING_J` |
| `J-CT09D-012` | `LOW` | R4 corrects `%25zz` in sections 3 and 10. Concat builders fail `http.NewRequest` with public 503 and no downstream call. `GetCase` sends the request, `URL.Path` contains `a%zz`, and the probe response was 200. The local-reject rule is unchanged. | `DOCUMENT_REMEDIATED_PENDING_J` |

`J-CT09D-007` through `J-CT09D-010` keep the requirements already written for those subjects. R4 changes only which finding id points at which requirement, and the `%25zz` builder split. It does not close a finding.

## 12. Implementation gate

CT-L2 implementation does not start from this revision.

```text
GO_FOR_J_R4_VERIFICATION=YES
CT_L2_IMPLEMENTATION_READY=NO
CT_L1_INTEGRATION=NOT_COMPLETE
J_CT09C_001=OPEN
PUBLIC_STATUS_CHANGE_APPROVED=NO
CONFIRMED_CROSS_ROUTE_ACCESS=YES
CONFIRMED_PRIVILEGE_ESCALATION=NO
PRIVILEGE_ESCALATION_IMPOSSIBLE=NOT_CLAIMED
CONFIRMED_CROSS_TENANT_LEAK=NO
FINAL_VERDICT=READY_FOR_J_R4_VERIFICATION
```

Issue #334 stays open. Issue #344 stays open. Production consumer readiness stays blocked.

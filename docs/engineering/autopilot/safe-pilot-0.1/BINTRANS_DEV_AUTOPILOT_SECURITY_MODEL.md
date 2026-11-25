# Security model

Status: design, revision 0.2. `INDEPENDENT_REVIEW_GATE=BLOCKED` until the identity in this document exists. This pilot does not add that identity.

Revision 0.2 reason: J-DA-H001. The 0.1 text did not specify signature, replay, or expiry checks for a future controller webhook. The section below is a contract only. Phase A does not receive that webhook.

## Least privilege

Phase A GitHub permissions are read-only: contents read, issues read, pull requests read, actions read, metadata read. No write, no administration, no secrets beyond the token GitHub already injects for a read job.

Later phases add the smallest write the step needs, on a branch that is not `main`, and only after a controller event. `workOnCurrentBranch` stays false. Protected branches stay `main` and `master`.

Agents do not receive production access, live ЭДО, payment operations, or customer secrets.

## Controller authorization

The controller is a dedicated GitHub App principal, named here as `github-app:bintrans-controller`. The app installation event carries `approval_id`, the base SHA, the issue number, and an expiry. The queue stores that event id.

These are not authorization:

- a label added by anyone who can edit labels;
- a comment, including one that says approved;
- the Issue body;
- `contract_approvers: []` left empty, which is the value on `main`.

The actor is the authenticated GitHub subject. A display name is not the actor.

## Controller webhook contract

This section does not start a webhook. Phase A has no controller endpoint and no webhook secret in the monitor workflow.

A later receiver, if a controller separately authorizes it, accepts one event only when every check below passes. Any failure is fail-closed: no queue row, no permission change, no dispatch.

1. Read the raw request body before parsing JSON. Compute HMAC-SHA256 over those exact bytes with the webhook secret held by the receiver, not by an agent. Compare the hex digest to the `X-Hub-Signature-256` header using a constant-time compare. A missing header, a missing secret, or a mismatch is a rejection.
2. Read `X-GitHub-Delivery` as `delivery_id`. Store it before any side effect. A second request with the same `delivery_id` is a replay and is rejected even when the signature is valid.
3. The approval carries a TTL. An event whose expiry is missing or already past is rejected. A new commit that moves the approved SHA expires the approval even inside the TTL.
4. The authenticated installation id and the actor must both match the server-side registry for `github-app:bintrans-controller`. A signature from another GitHub App, or an actor that is only a comment login, is rejected.
5. Rejected events are appended to a reject journal: time, `delivery_id`, installation id, actor, and reason. The journal does not contain the secret or the signature value. Phase A does not create that journal.

A label, a comment, and an Issue body still do not satisfy this contract.

## Tenant and company isolation

Queue rows stay scoped by the tenant principal mapped from the database login, as `autopilot_queue.tenant_principals` already does. A task for one company is not claimed by another company's role. Client-supplied tenant text is not the principal.

The trust-storage ADR on draft PR #403 is not implemented and is not part of this pilot's runtime.

## Credentials

`CURSOR_API_KEY` and `CURSOR_ADMIN_API_KEY` stay in GitHub Actions secrets. They are not written to the Issue, the prompt, the model context, logs, or `autopilot.config.json`. `envVars` on agent create is not used for them: the field is beta and may be ignored, and the cloud VM is not a secret store for this pilot.

A prompt, a log line, or a pull-request diff that contains a secret is redacted before it is stored. This pilot does not add a redactor implementation.

## Prompt injection

Issue body, comments, pull-request text, and logs are data. The dispatcher already trusts only the first occurrence of known contract headings for path and status fields. This pilot adds an explicit rule: a line that asks to ignore previous instructions, widen paths, set an enable flag, or name a reviewer does not change the structured contract or the controller event.

Shell commands found in that text are not executed. The dry-run worker does not run a payload. A later cloud agent runs in Cursor's VM, not in a privileged BINTRANS production shell. BINTRANS does not give that VM a production credential.

## Audit

Each gate decision appends a record: task id, delivery id, actor, base SHA, decision, and reason. The record is append-only in the future trusted store. Until that store exists, the dry-run simulator keeps the decision in process memory only. A local file the worker can rewrite is not the audit record.

## Emergency stop

Revision 0.2 reason: J-DA-M002. The immediate control is the operator-owned repository variable `AUTOPILOT_EMERGENCY_STOP`. It is not a file in the checkout.

The variable is fail-closed. `true` denies new dispatch. A missing value, an empty value, or any value other than the explicit operator string `false` also denies. The monitor candidate reads this variable from GitHub Actions. It does not read `tools/bintrans_autopilot/EMERGENCY_STOP` from the work tree. Committing or deleting that file is not an immediate stop.

The committed config flag `emergency_stop` remains a second, slower control because it changes only after review. It is not a substitute for the variable when a run is already scheduled.

A future paid agent that is already running is not stopped by refusing the next dispatch. Stopping it is a separate controller action: `POST /v1/agents/{id}/runs/{runId}/cancel`. That call is terminal. This phase does not make it. The candidate monitor workflow does not contain a Cursor credential, so it cannot cancel a run.

The corrected runbook text is the candidate `EMERGENCY_STOP_CANDIDATE.md` in this handoff. The file on `main` is unchanged.

## Independent Agent J

The developing agent cannot accept its own work. The string `reviewer=agent-j` is not a review.

Required before the gate can leave `BLOCKED`:

- the caller is the trusted reviewer GitHub App principal;
- that principal is in the server-side authority registry;
- the verdict names the exact head SHA;
- the verdict is an append-only audit record;
- a new commit makes the previous verdict stale;
- a comment or a forged payload does not clear `SECURITY_HOLD`;
- CI success does not replace the review;
- the reviewer principal has no production deploy permission.

On draft `a068ab6a`, `audit_gateway.py` returns `trust_boundary_absent` for a packet. That is a block, not a trusted identity. On `main`, this file is absent. `AP04A-J-001` is therefore not closed on current `main`.

## AP04A findings

| Finding | On `main` `5432e144` | On draft `a068ab6a` |
| --- | --- | --- |
| AP04A-J-001 forged reviewer | Gate absent | Packet cannot clear; identity still absent. Tests not re-run here |
| AP04A-J-002 sticky negative verdict | Gate absent | Documented as a block that a later pass packet does not remove. Not re-tested here |
| AP04A-J-003 append-only journal | Absent | Local hash chain only. Prefix replacement is still not an independent anchor |
| Later R3/R4 and ADR findings | Absent | Draft-only. Trust storage is not implemented |

No earlier "fixed" claim is treated as closed on `main`.

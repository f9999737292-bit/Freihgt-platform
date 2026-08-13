# Quick Start — Parallel Engineering for Project Owners

Simple guide for non-developers orchestrating multiple Cursor Agents on freight-platform (Windows).

## Scenario

You want to implement **Feature X** (for example, Control Tower alert acknowledgement) without agents overwriting each other's work.

---

## Step 1 — Describe the feature to the Orchestrator

Open a Cursor Agent chat with the **orchestrator** subagent (or paste `ORCHESTRATOR_PROMPT_TEMPLATE.md`).

Example message:

```text
Реализуй Control Tower — Alert Acknowledgement v0.1
```

The orchestrator inspects the repo and returns Task Contracts, branch names, worktree paths, and agent prompts.

---

## Step 2 — Review Task Contracts

Each workstream gets a filled **Task Contract** (`TASK_CONTRACT_TEMPLATE.md`) with:

- who owns it (backend, frontend, security, etc.)
- which folders they may edit
- dependencies and order

Approve or adjust before coding starts.

---

## Step 3 — Create worktrees (PowerShell)

For each approved workstream, run commands from `WORKTREE_PROCEDURE.md`. Example:

```powershell
cd D:\Projects\freight-platform
git fetch origin
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-api -b feat/control-tower-alert-ack-api-v0.1 origin/main
```

Repeat for frontend, docs, etc., with **different paths and branches**.

---

## Step 4 — Open each worktree in a separate Cursor window

- Window 1 → `D:\Projects\freight-platform-wt\ct-alert-ack-api`
- Window 2 → `D:\Projects\freight-platform-wt\ct-alert-ack-ui`
- Window 3 → security review (readonly) on handoff diffs

One agent — one window — one branch — one task.

---

## Step 5 — Give each agent its prompt

Paste the filled **Agent Prompt** (`AGENT_PROMPT_TEMPLATE.md`) into each window's agent chat.

Agents will check Git state, read the Task Contract, implement, validate, and hand off.

---

## Step 6 — Collect handoffs

Each implementer completes `HANDOFF_TEMPLATE.md` with:

- branch and SHAs
- files changed
- what was tested (**PASS** / **NOT_RUN**)
- risks

---

## Step 7 — Review and integrate

1. **security-auditor** — if auth/tenant touched
2. **reviewer** — diff vs Task Contract
3. **qa-verification** — acceptance criteria evidence
4. **integrator** — merge in dependency order per `INTEGRATION_PROTOCOL.md`

You (or release owner) decide when to merge to `main`. Agents do not push unless you ask.

---

## What you should not do

- Do not point two agents at the same folder/worktree for implementation.
- Do not let agents edit `packages/openapi` in parallel without a contract-owner task first.
- Do not force-push or delete existing pilot/staging worktrees.

---

## Where to read more

| Topic | Document |
|-------|----------|
| Full system overview | `PARALLEL_ENGINEERING_SYSTEM_V1.md` |
| Owner playbook | `AGENTS.md` |
| Orchestrator prompt | `ORCHESTRATOR_PROMPT_TEMPLATE.md` |
| Agent prompt | `AGENT_PROMPT_TEMPLATE.md` |
| Worktree commands | `WORKTREE_PROCEDURE.md` |
| Collision rules | `COLLISION_POLICY.md` |

---

## Bootstrap worktree for engineering changes

Parallel Engineering System itself is maintained on:

- Branch: `chore/parallel-engineering-system-v1`
- Worktree: `D:\Projects\freight-platform-parallel-bootstrap`

Do not use the dirty main checkout (`D:\Projects\freight-platform`) for engineering-system edits unless you intentionally work there.

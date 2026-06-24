# Low-code Pilot Week-2 BILLING_REGISTER Operator Quick Guide v0.1

**Audience:** pilot operator, platform admin  
**Scope:** BILLING_REGISTER custom fields on **approved demo/pilot entities only**

---

## Purpose

This guide explains how to **safely view and edit** custom fields on a Billing Register (реестр биллинга) during the low-code pilot. It does not cover template design, imports, migrations, or payment/billing status changes.

## Who Can Use This

| Role | Can edit BILLING_REGISTER custom fields? |
|------|---------------------------------------------|
| Platform admin | Yes — on approved test entities |
| Finance operator (pilot user) | Only when explicitly enabled by pilot lead |
| Other roles | No — unless product approves |

**Today:** internal demo/testing only. Do not edit random production billing registers.

## Before You Start

1. Confirm you are on the **pilot tenant** (not another company).
2. Confirm the billing register is an **approved test entity** (ask pilot lead if unsure).
3. Check that the page loaded without a red error banner.
4. Do not edit if the system health check failed today.

**Dev demo example:** DEMO-BR-001

## What You May Edit

Only these custom fields on `billing_register_default` template:

- **cost_allocation_code** — код распределения затрат (текст)
- **approval_group** — группа согласования (выбор из списка)
- **payment_priority** — приоритет оплаты (выбор из списка)

**Valid values for approval_group:** LOGISTICS_FINANCE, FINANCE, OPS, MANAGEMENT

**Valid values for payment_priority:** LOW, NORMAL, HIGH

Do **not** try to change billing register status, payment status, totals, or UPD documents here — use the main Billing Register screen and billing workflows.

## What You Must Not Do

- Edit billing registers outside the approved pilot list
- Change billing/payment status through custom fields
- Run migration or batch operations
- Import or publish templates
- Click **Save** many times quickly — wait for the button to finish
- Change data in the database manually
- Share login credentials

## Step-by-Step Flow

1. **Log in** to web-admin.
2. **Open the billing register** — from Billing Registers list or use link from pilot lead.
3. **Find the custom fields panel** — below register details (status, totals).
4. **Check current values** — read before changing.
5. **Edit only allowed fields** — use valid SELECT options only.
6. **Confirm** you are not changing status or amounts on the main card.
7. **Click Save once** — wait until saving finishes.
8. **Look for success or error message** — read the text; take a screenshot if error.
9. **Refresh the page** — confirm your change is still there; **status and totals unchanged**.
10. **Tell pilot lead** if something looks wrong.

**Alternative:** open **Low-code → Custom field values**, choose BILLING_REGISTER, pick the entity, edit, save — same rules apply.

## How To Check Audit

After saving, pilot lead may verify in **Low-code → Audit**:

- Look for **CUSTOM_FIELD_VALUES_UPDATED**
- Check the billing register ID matches
- Check time matches your save

Operators: if audit is empty after save, **stop and report** — do not save again blindly.

## Financial Safety Rules

- Custom fields are **extra metadata** — they do **not** change payment or billing status.
- **Totals and status** on the main page are controlled by billing services, not custom fields.
- If status or totals changed after your save → **stop immediately** and report (P0).

## What To Do On Error

| Situation | Action |
|-----------|--------|
| Red error message on save (e.g. invalid option) | Stop. Screenshot. Report via feedback form. |
| Page blank / spinning forever | Refresh once. If still broken — report. |
| Values disappeared after save | Stop. Do not re-save. Report immediately. |
| Wrong billing register shows | Stop. Do not save. Report. |
| Status or totals changed after save | **P0** — stop. Report immediately. |

Form: `LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`

## Stop Conditions

**Stop working and call pilot lead** if:

- You edited the wrong billing register or company
- Save succeeded but values look wrong on refresh
- **Billing status or totals changed** after custom field save
- You see another company's data
- Audit has no record after your save
- The same error happens twice

## Contact / Escalation

1. **First:** pilot lead / operator on duty
2. **P0 (urgent):** platform ops — wrong tenant, financial side effect, data loss, security issue
3. **Document:** daily report + feedback form

**Next pack for team:** Limited Write Enablement — follow pilot lead instructions before expanding who can edit BILLING_REGISTER fields.

---

Full technical review: `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_FLOW_REVIEW_V0.1.md`

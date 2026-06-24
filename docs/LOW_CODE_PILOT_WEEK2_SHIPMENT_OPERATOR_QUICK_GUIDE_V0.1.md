# Low-code Pilot Week-2 SHIPMENT Operator Quick Guide v0.1

**Audience:** pilot operator, platform admin  
**Scope:** SHIPMENT custom fields on **approved demo/pilot entities only**

---

## Purpose

This guide explains how to **safely view and edit** custom fields on a Shipment (перевозка) during the low-code pilot. It does not cover template design, imports, or migrations.

## Who Can Use This

| Role | Can edit SHIPMENT custom fields? |
|------|----------------------------------|
| Platform admin | Yes — on approved test entities |
| Shipper logist (pilot user) | Only when explicitly enabled by pilot lead |
| Other roles | No — unless product approves |

**Today:** internal demo/testing only. Do not edit random production shipments.

## Before You Start

1. Confirm you are on the **pilot tenant** (not another company).
2. Confirm the shipment is an **approved test entity** (ask pilot lead if unsure).
3. Check that the page loaded without a red error banner.
4. Do not edit if the system health check failed today.

**Dev demo example:** DEMO-SH-PLANNED

## What You May Edit

Only these custom fields on `shipment_default` template:

- **temperature_mode** — режим температуры
- **loading_contact_phone** — телефон контакта на погрузке
- **driver_comment** — комментарий для водителя
- **planned_pickup_date** — плановая дата погрузки
- **declared_value** — объявленная стоимость
- **handling_flags** — флаги обращения (хрупкое и т.д.)

Do **not** try to change shipment status, carrier, or route here — use the main Shipment screen.

## What You Must Not Do

- Edit shipments outside the approved pilot list
- Run migration or batch operations
- Import or publish templates
- Click **Save** many times quickly — wait for the button to finish
- Change data in the database manually
- Share login credentials

## Step-by-Step Flow

1. **Log in** to web-admin.
2. **Open the shipment** — from Shipments list or use link from pilot lead.
3. **Find the custom fields panel** — below shipment details.
4. **Check current values** — read before changing.
5. **Edit only allowed fields** — one logical change at a time when learning.
6. **Click Save once** — wait until saving finishes.
7. **Look for success or error message** — read the text; take a screenshot if error.
8. **Refresh the page** — confirm your change is still there.
9. **Tell pilot lead** if something looks wrong.

**Alternative:** open **Low-code → Custom field values**, choose SHIPMENT, pick the entity, edit, save — same rules apply.

## How To Check Audit

After saving, pilot lead may verify in **Low-code → Audit**:

- Look for **CUSTOM_FIELD_VALUES_UPDATED**
- Check the shipment ID matches
- Check time matches your save

Operators: if audit is empty after save, **stop and report** — do not save again blindly.

## What To Do On Error

| Situation | Action |
|-----------|--------|
| Red error message on save | Stop. Screenshot. Report via feedback form. |
| Page blank / spinning forever | Refresh once. If still broken — report. |
| Values disappeared after save | Stop. Do not re-save. Report immediately. |
| Wrong shipment shows | Stop. Do not save. Report. |

Form: `LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`

## Stop Conditions

**Stop working and call pilot lead** if:

- You edited the wrong shipment or company
- Save succeeded but values look wrong on refresh
- You see another company's data
- Audit has no record after your save
- The same error happens twice

## Contact / Escalation

1. **First:** pilot lead / operator on duty
2. **P0 (urgent):** platform ops — wrong tenant, data loss, security issue
3. **Document:** daily report + feedback form

**Next pack for team:** Limited Write Enablement — follow pilot lead instructions before expanding who can edit SHIPMENT fields.

---

Full technical review: `LOW_CODE_PILOT_WEEK2_SHIPMENT_OPERATOR_FLOW_REVIEW_V0.1.md`

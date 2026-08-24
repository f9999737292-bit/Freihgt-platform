# UAT Plan — Freight Platform v1

Human acceptance by persona. **Status:** PLANNED (blocked until staging/UAT env ready).

Sign-off owner placeholders: `{BUSINESS_OWNER}`.

---

## SHIPPER UAT

**Objective:** Procure transport, award, track order, see cost/variance.

| ID | Steps | Expected | Pass/Fail |
|----|-------|----------|-----------|
| FP-UAT-SHP-001 | Login → create tender → publish | Carriers invited | |
| FP-UAT-SHP-002 | Evaluate bids → award | Winner selected, audit visible | |
| FP-UAT-SHP-003 | Create order from award | Rate snapshot on order | |
| FP-UAT-SHP-004 | Track shipment on map/timeline | Status matches driver | |
| FP-UAT-SHP-005 | View freight cost variance | Matches finance register | |

---

## CARRIER UAT

| ID | Steps | Expected |
|----|-------|----------|
| FP-UAT-CAR-001 | View open tender → submit bid | Bid accepted |
| FP-UAT-CAR-002 | Win award → accept shipment | Execution unlocked |
| FP-UAT-CAR-003 | Assign driver → monitor | CT shows progress |
| FP-UAT-CAR-004 | View settlement | Amount matches execution |

---

## DRIVER UAT

| ID | Steps | Expected |
|----|-------|----------|
| FP-UAT-DRV-001 | Login mobile → see assigned job | Correct shipment only |
| FP-UAT-DRV-002 | Pickup → transit → delivery | Milestones accepted |
| FP-UAT-DRV-003 | Upload POD | Delivery confirmed |

---

## CONTROL TOWER UAT

| ID | Steps | Expected |
|----|-------|----------|
| FP-UAT-CT-001 | See delay/problem on dashboard | Event within acceptable lag |
| FP-UAT-CT-002 | Acknowledge critical event | State updated |
| FP-UAT-CT-003 | Create/resolve case | Timeline complete |

---

## FINANCE UAT

| ID | Steps | Expected |
|----|-------|----------|
| FP-UAT-FIN-001 | Settlement → approve | Totals match rate |
| FP-UAT-FIN-002 | Billing register → UPD | Documents correct RU labels |
| FP-UAT-FIN-003 | Freight cost analytics | No manual recalc needed |

---

## ADMIN UAT

| ID | Steps | Expected |
|----|-------|----------|
| FP-UAT-ADM-001 | Manage users/roles | RBAC enforced |
| FP-UAT-ADM-002 | Tenant company setup | Isolation verified |

---

## Business Acceptance Checklist (one page)

- [ ] Can shipper procure transport?
- [ ] Can carrier bid?
- [ ] Can award turn into execution?
- [ ] Can driver execute?
- [ ] Can operator monitor (Control Tower)?
- [ ] Can delivery be proven (POD)?
- [ ] Can finance settle?
- [ ] Can finance bill?
- [ ] Can platform calculate freight cost?
- [ ] Can analytics explain the cost?
- [ ] All without cross-tenant leakage?

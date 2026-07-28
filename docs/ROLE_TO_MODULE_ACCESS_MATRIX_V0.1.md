# Role-to-Module Access Matrix v0.1

## Summary

Initial role-to-module matrix for next product iteration.

## Roles

| Role | Russian Name | Cabinet Strategy |
| --- | --- | --- |
| admin | Администратор | web-admin full access |
| shipper | Грузовладелец | web-admin role nav first, web-shipper later |
| carrier | Перевозчик | web-admin role nav first, web-carrier later |
| forwarder | Экспедитор | web-admin role nav first, no dedicated app yet |
| consignee | Грузополучатель | web-admin role nav first, web-consignee later |
| finance | Финансы | web-admin role nav first, web-finance later |
| procurement | Закупки | web-admin role nav first, web-procurement later |

## Identity Role Mapping (from pilot docs)

| Product Role | Example Identity Role | Example Email (staging seed) |
| --- | --- | --- |
| admin | PLATFORM_ADMIN | admin@bintrans.local |
| shipper | SHIPPER_LOGIST | shipper@bintrans.local |
| carrier | CARRIER_DISPATCHER / CARRIER_MANAGER | carrier@bintrans.local |
| forwarder | PROCUREMENT_MANAGER / FORWARDER_MANAGER | forwarder@bintrans.local |
| consignee | CONSIGNEE_OPERATOR / CONSIGNEE_VIEWER | consignee@bintrans.local |
| finance | (TBD — finance role codes) | TBD |
| procurement | PROCUREMENT_MANAGER | overlaps with forwarder in seed docs |

## Module Access Draft

| Module | Admin | Shipper | Carrier | Forwarder | Consignee | Finance | Procurement |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Dashboard | full | own summary | own summary | own summary | receive summary | finance view | procurement view |
| Control tower | full | own shipments | assigned shipments | assigned flows | receiving view | financial view | procurement view |
| Transport orders | full | create/manage | view/accept assigned | manage delegated | view related | view financial status | procurement view |
| Freight requests | full | create/manage | view/respond | manage/respond | no/limited | no/limited | manage |
| RFx/tenders | full | create/manage | participate/respond | manage/respond | no/limited | view costs | manage |
| Shipments | full | view/manage own | update assigned | manage assigned | receive/confirm | view | view |
| Documents | full | create/view/sign | upload/view/sign | manage docs | receive docs | view/approve | view |
| Billing registers | full | view/approve | view own settlements | manage settlements | no/limited | full finance | view |
| Companies | full | own company | own company | own company | own company | view | view |
| Users | full | own company users | own company users | own company users | limited | finance users | procurement users |
| Low-code | full | no/limited | no/limited | no/limited | no | no/limited | limited |
| Settings | full | own | own | own | own | own | own |
| Health | full | no | no | no | no | limited | limited |

## web-admin Sidebar Mapping (current vs target)

| Sidebar Item | Current (all users) | Target: non-admin |
| --- | --- | --- |
| /dashboard | visible | visible (scoped data) |
| /control-tower | visible | role-dependent |
| /companies | visible | own company only |
| /users | visible | admin / company admin only |
| /transport-orders | visible | shipper, forwarder |
| /freight-requests | visible | shipper, forwarder, procurement |
| /rfx | visible | shipper, forwarder, procurement |
| /shipments | visible | all operational roles |
| /documents | visible | all operational roles |
| /billing-registers | visible | admin, finance, shipper (view) |
| /low-code | visible | admin only |
| /health | visible | admin only |
| /settings | visible | all |

## Notes

```text
This is a planning matrix only.
It does not change RBAC, frontend navigation, backend permissions, or production behavior.
Forwarder and procurement overlap in current seed documentation — needs explicit role split in RBAC design pack.
Finance role codes not yet fully defined in identity seed docs.
```

## Next

```text
RBAC_AND_ROLE_NAVIGATION_DESIGN_PACK v0.1
```

# RFI dispatch readiness 0.1X

Date: 2026-11-18. This is a draft decision for the BINTRANS controller. It does not authorize sending.

Questions Q1–Q10 stay in these unchanged 0.1W documents:

- `docs/architecture/tracking/0_1w/PROVIDER_RFI_MICROSOFT.md`
- `docs/architecture/tracking/0_1w/PROVIDER_RFI_AWS.md`
- `docs/architecture/tracking/0_1w/PROVIDER_RFI_GOOGLE.md`
- `docs/architecture/tracking/0_1w/PROVIDER_RFI_REKOR.md`

Those files are on Draft PR #393 at `a6fcfc1f290282e19bcfb4a804463e6183f194a2`. This qualification does not edit them. Their contact sections already say `CONTACT_NOT_VERIFIED` and `NOT_SENT`. The 0.1X reading confirms that no published channel is an authorized recipient of those questions.

## Decision

```text
RFI_SEND_READY=NO
RFI_SEND_AUTHORIZED=NO
PROVIDER_MESSAGES_SENT=NO
SUPPORT_TICKETS_CREATED=NO
PROVIDER_REPLIES_RECEIVED=NO
MANAGED_ANCHOR=NOT_SELECTED
PILOT_RECOMMENDATION=DO_NOT_START
```

`SEND_READY=YES` would mean a controller could later authorize one identified channel without first obtaining an account, a paid support offer, or a new official recipient. None of the four providers is in that position.

## Per provider

```text
PROVIDER=MICROSOFT
SEND_READY=NO
SEND_AUTHORIZED=NO
BLOCKER=ONE_TO_ONE_TECHNICAL_SUPPORT_REQUIRES_A_PAID_AZURE_SUPPORT_PLAN_AND_THE_PLAN_IS_NOT_DOCUMENTED_AS_A_WRITTEN_GUARANTEE_CHANNEL
```

```text
PROVIDER=AWS
SEND_READY=NO
SEND_AUTHORIZED=NO
BLOCKER=A_TECHNICAL_CASE_IS_UNAVAILABLE_ON_BASIC_SUPPORT_AND_THE_PRICING_FORM_IS_A_SALES_QUOTE_CHANNEL
```

```text
PROVIDER=GOOGLE_CLOUD
SEND_READY=NO
SEND_AUTHORIZED=NO
BLOCKER=BASIC_CUSTOMER_CARE_CANNOT_OPEN_A_TECHNICAL_CASE_AND_THE_SALES_FORM_IS_NOT_DOCUMENTED_FOR_THIS_QUALIFICATION
```

```text
PROVIDER=SIGSTORE
SEND_READY=NO
SEND_AUTHORIZED=NO
BLOCKER=THE_PUBLISHED_NON_SECURITY_ROUTE_IS_PUBLIC_COMMUNITY_HELP_AND_THE_VULNERABILITY_MAILBOX_IS_EXCLUDED
```

## What a later authorization would still require

A separate controller decision is required before any send. That decision is not this document. Even after such a decision, the sender would still need, for Microsoft, AWS, or Google, an account in the controller's name and a paid technical-support offer. Buying that offer is not authorized here. For Sigstore, a later decision would first have to name a recipient that these public pages do not name.

The outbound text, if one is ever authorized, starts from `RFI_DISCLOSURE_CHECKLIST.md`. It does not attach repository evidence, and it does not use a vulnerability-reporting address.

## Gates that remain closed

```text
IMPLEMENTATION_GATE=CLOSED
ISSUER_ISSUANCE_GATE=CLOSED
DECISION_CONSUMPTION_GATE=CLOSED
PRODUCTION_CONSUMER_READINESS=BLOCKED
FOURTH_SERVER_PURCHASE=NOT_AUTHORIZED
P3B_ALL_STORES_ROLLBACK=NOT_FIXED
ANCHOR_FAILURE_DOMAIN=NOT_PROVEN
OLD_PRIMARY_FENCING=NOT_PROVEN
J_DML_001=NOT_CLOSED
```

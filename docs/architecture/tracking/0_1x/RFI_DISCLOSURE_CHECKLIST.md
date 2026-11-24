# RFI disclosure checklist 0.1X

Date: 2026-11-18. This checklist prepares a minimum description. It is not an outbound message. `SEND_AUTHORIZED=NO`.

The description below is the only technical content that this qualification allows a later controller decision to consider. Q1–Q10 remain in the 0.1W documents and are not copied into an outbound message by this checklist.

## Allowed description

Russian, for the controller:

BINTRANS квалифицирует независимый журнал подтверждений для B2B SaaS. Журнал должен хранить монотонную историю подтверждённых записей и разделять полномочия оператора сервиса и администратора журнала. Нужны письменные технические условия такого использования и его стоимость. Это квалификационный вопрос. Это не сообщение об уязвимости и не просьба создать ресурсы.

English, the same statement:

BINTRANS is qualifying an independent audit and recovery anchor for a B2B SaaS service. The anchor must keep a monotonic history of acknowledged records and separate the authority of the service operator from the authority of the anchor administrator. The request is for the written technical terms of that use and for its price. This is a qualification question. It is not a vulnerability report and it is not a request to create resources.

## Identity the provider forms already ask for

A sales form or a support case asks for the sender's own business identity. That identity is supplied by the controller at send time, from the controller's own records. This repository does not store it. The Google sales form, read on 2026-11-18, asks for name, job title, business email, business phone, company, industry, country, and a problem description of up to 1000 characters. Other forms mark their own required fields on the page. Those fields are not filled in here.

## Allowed in a future authorized message

- The neutral description in this file.
- The product name under qualification: Azure Confidential Ledger, DynamoDB, S3 Object Lock, Cloud Storage, or Rekor.
- A request for written terms and for the price of the described use.
- Public documentation URLs that the provider itself publishes.

## Forbidden in any outbound message

- Server addresses.
- SSH key names, key material, and account credentials.
- Secrets, tokens, and passwords.
- Internal vulnerability details and privileged-access paths.
- Customer data and tenant identifiers.
- Private links, pull-request numbers, commit identifiers, and repository evidence.
- A request to open, change, or delete a provider resource.
- A vulnerability report, including any use of `security@sigstore.dev`.

## Before a future send

```text
NEUTRAL_DESCRIPTION_ONLY=YES
Q1_TO_Q10_ATTACHED=NO
REPOSITORY_EVIDENCE_ATTACHED=NO
VULNERABILITY_CHANNEL_USED=NO
SEND_AUTHORIZED=NO
```

If a later controller decision attaches the existing Q1–Q10 text, that decision is a new authorization. The text is then reviewed again against the forbidden list above before it leaves BINTRANS. This document does not perform that attachment.

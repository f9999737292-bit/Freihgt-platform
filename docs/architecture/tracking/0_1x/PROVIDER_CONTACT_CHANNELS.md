# Provider contact channels 0.1X

Date: 2026-11-18. Read-only public research. No message was sent. No support ticket was opened. No account was created. No paid service was purchased.

The questions remain Q1–Q10 in the 0.1W documents on Draft PR #393, head `a6fcfc1f290282e19bcfb4a804463e6183f194a2`. Those documents are unchanged. A published product-team address, a community forum, or a sales form is not by itself an authorized recipient of that RFI.

`CONTACT_AUTHORIZED_FOR_RFI=NO` for every provider below. `SEND_AUTHORIZED=NO`.

## Microsoft — Azure Confidential Ledger

```text
PROVIDER=MICROSOFT
PRODUCT=AZURE_CONFIDENTIAL_LEDGER
OFFICIAL_CONTACT_CHANNEL=AZURE_PORTAL_SUPPORT_REQUEST
OFFICIAL_SOURCE_URL=https://learn.microsoft.com/en-us/azure/azure-portal/supportability/how-to-create-azure-support-request
SOURCE_CHECK_DATE=2026-11-18
CHANNEL_PURPOSE=SUBSCRIPTION_MANAGEMENT_FOR_ALL_CUSTOMERS_AND_TECHNICAL_SUPPORT_WITH_A_PAID_PLAN
ACCOUNT_REQUIRED=YES
PAID_SUPPORT_REQUIRED=YES_FOR_ONE_TO_ONE_TECHNICAL_SUPPORT
CONTACT_AUTHORIZED_FOR_RFI=NO
REQUIRED_USER_ACTION=AZURE_SUBSCRIPTION_ROLE_AND_PAID_SUPPORT_PLAN_THEN_PORTAL_CASE
VERIFICATION_STATUS=VERIFIED_RESTRICTED_CHANNEL
```

Rules read on 2026-11-18:

- Billing, quota, and account-transfer support is included for Azure customers. One-to-one technical support requires a paid support plan. Source: [Azure support plans FAQ](https://azure.microsoft.com/en-us/support/legal/faq).
- A support request is created in the Azure portal, by the Azure support ticket REST API, or by Azure CLI. The commercial portal is `https://portal.azure.com`. The creator needs Owner, Contributor, Support Request Contributor, or a custom role with `Microsoft.Support/*` on the subscription. Source: [How to create an Azure support request](https://learn.microsoft.com/en-us/azure/azure-portal/supportability/how-to-create-azure-support-request).
- Basic has technical support marked not included. Developer is listed at $29 per month and includes business-hours access for minimal business impact. Standard is listed at $100 per month. Architecture support on Developer and Standard is general guidance. Advisory support begins at Professional Direct. Source: [Azure Support Plans Comparison](https://azure.microsoft.com/en-us/support/plans).
- The same FAQ limits technical support of public preview to break-fix scenarios. It defines advisory support as general how-to help. Professional Direct advisory support does not include verification of specifications or detailed architectural guidance.
- The sales directory at [Contact Azure sales](https://azure.microsoft.com/en-us/contact) is a commercial channel. It does not say that a sales conversation answers a durability and deletion qualification.
- Without a paid plan, the documented technical route is Azure on Q&A, which is a community channel.

The portal case can carry a technical question and, on a paid plan, a root-cause request. The pages do not say that the case produces written technical guarantees for Q1–Q10. Cost of the support plan is published. Cost of Confidential Ledger itself is not established by these contact pages.

Excluded: `accledgerdevs@microsoft.com` on [the Power Automate connector page](https://learn.microsoft.com/en-us/connectors/acl/). That page names the Azure Confidential Ledger team. It does not say the mailbox answers durability, deletion, or price questions. The 0.1W RFI already excludes it. This qualification does not authorize it.

`SEND_READY=NO`.

## AWS — DynamoDB and S3 Object Lock

```text
PROVIDER=AWS
PRODUCT=DYNAMODB_AND_S3_OBJECT_LOCK
OFFICIAL_CONTACT_CHANNEL=AWS_SUPPORT_CENTER_TECHNICAL_CASE
OFFICIAL_SOURCE_URL=https://aws.amazon.com/contact-us/
SOURCE_CHECK_DATE=2026-11-18
CHANNEL_PURPOSE=TECHNICAL_CASES_ABOVE_BASIC_AND_ACCOUNT_OR_BILLING_FOR_ALL_CUSTOMERS
ACCOUNT_REQUIRED=YES_FOR_A_TECHNICAL_OR_BILLING_CASE
PAID_SUPPORT_REQUIRED=YES_FOR_A_TECHNICAL_CASE
CONTACT_AUTHORIZED_FOR_RFI=NO
REQUIRED_USER_ACTION=AWS_ACCOUNT_AND_PAID_TECHNICAL_PLAN_THEN_SUPPORT_CENTER_CASE
VERIFICATION_STATUS=VERIFIED_RESTRICTED_CHANNEL
```

Rules read on 2026-11-18:

- [Contact AWS](https://aws.amazon.com/contact-us/) lists technical support as unavailable under the Basic Support Plan. Account and billing support is a separate signed-in request. Compliance support is a separate request and is not this RFI.
- [Case management](https://docs.aws.amazon.com/awssupport/latest/user/case-management.html) states that a Basic Support customer cannot create a technical support case. Account and billing cases and service-quota requests are available to all AWS customers. The documented low severity is general guidance: a general development question or a feature request. The page does not say that such a case produces written technical guarantees.
- [AWS Support Plans](https://docs.aws.amazon.com/awssupport/latest/user/aws-support-plans.html) gives Basic customers account and billing responses, forums, service health, and documentation. Business Support+ adds phone, web, and chat access to Cloud Support Engineers. Developer Support is announced to end on 2027-01-01.
- [AWS Support pricing](https://aws.amazon.com/premiumsupport/pricing/) lists Business Support+ at a minimum of $29 per month per account, or a percentage of monthly AWS charges when that amount is greater. Enterprise Support is listed at a minimum of $5,000 per month.
- [AWS pricing specialists](https://aws.amazon.com/contact-us/sales-support-pricing/) publish a web form for questions and a personalized pricing quote, with a stated reply within one business day. The same page repeats that technical support is unavailable under Basic and points technical requests to a signed-in support case. The form is a commercial channel. It is not documented as the recipient of Q1–Q10.

A technical case requires an AWS account and a plan above Basic. The pricing form does not state that an AWS account is required. Neither channel is documented as authorized to give the written technical guarantees in the existing RFI.

`SEND_READY=NO`.

## Google Cloud — Cloud Storage

```text
PROVIDER=GOOGLE_CLOUD
PRODUCT=CLOUD_STORAGE
OFFICIAL_CONTACT_CHANNEL=GOOGLE_CLOUD_CONSOLE_SUPPORT_CASE
OFFICIAL_SOURCE_URL=https://docs.cloud.google.com/support/docs/customer-care-procedures
SOURCE_CHECK_DATE=2026-11-18
CHANNEL_PURPOSE=TECHNICAL_CASES_FOR_A_PAID_CUSTOMER_CARE_OFFER_AND_BILLING_ON_BASIC
ACCOUNT_REQUIRED=YES_FOR_A_CONSOLE_CASE
PAID_SUPPORT_REQUIRED=YES_FOR_A_TECHNICAL_CASE
CONTACT_AUTHORIZED_FOR_RFI=NO
REQUIRED_USER_ACTION=CLOUD_ORGANIZATION_OR_PROJECT_AND_PAID_CUSTOMER_CARE_THEN_CONSOLE_CASE
VERIFICATION_STATUS=VERIFIED_RESTRICTED_CHANNEL
```

Rules read on 2026-11-18:

- [Customer Care procedures](https://docs.cloud.google.com/support/docs/customer-care-procedures) create a case in the Google Cloud console for an organization or a project. The case asks for a product, a description, an observed error, a priority, a category, and either live chat or email. A paid support package is what opens the technical case path. Basic includes billing and payments questions in the console prompt.
- [Downgrading Customer Care](https://docs.cloud.google.com/support/docs/downgrade) states that users with Basic support cannot create technical support cases. They can use Cloud Billing support and self-help resources.
- [Customer Care](https://cloud.google.com/support) includes Basic for every Google Cloud customer, including a free trial: documentation, community forums, billing assistance, and Active Assist.
- [Standard Support](https://cloud.google.com/support/standard) lists unlimited technical support for workloads under development. The published charge is a minimum of $29.00 or 3 percent of monthly Cloud charges, whichever is higher.
- [The sales form](https://cloud.google.com/contact/form) invites a conversation with a sales specialist about a business problem and about product and solution pricing. The form collects a name, job title, business email, business phone, company, industry, country, and a problem description of up to 1000 characters, plus a privacy consent. It points technical support to the Cloud Support Hub. It does not say that the sales specialist answers a Bucket Lock qualification in writing.
- [Cloud Support Hub](https://cloud.google.com/support-hub) does not publish a mailbox for this qualification. The 0.1W RFI already records that fact.

No Cloud Storage mailbox for this RFI was found on these pages.

`SEND_READY=NO`.

## Sigstore — Rekor

```text
PROVIDER=SIGSTORE
PRODUCT=REKOR
OFFICIAL_CONTACT_CHANNEL=SIGSTORE_COMMUNITY_SLACK_OR_GITHUB_ISSUE
OFFICIAL_SOURCE_URL=https://docs.sigstore.dev/about/support/
SOURCE_CHECK_DATE=2026-11-18
CHANNEL_PURPOSE=COMMUNITY_HELP
ACCOUNT_REQUIRED=YES_FOR_SLACK_OR_GITHUB
PAID_SUPPORT_REQUIRED=NO_FEE_PUBLISHED
CONTACT_AUTHORIZED_FOR_RFI=NO
REQUIRED_USER_ACTION=NONE_THIS_CHANNEL_IS_NOT_AN_RFI_RECIPIENT
VERIFICATION_STATUS=VERIFIED_RESTRICTED_CHANNEL
```

Rules read on 2026-11-18:

- [Get Help](https://docs.sigstore.dev/about/support/) directs support requests to the Sigstore Slack `#general` channel. The invite link is on that page. The same page accepts questions in `#general` or as a GitHub issue in the relevant repository, and points maintainers to each repository's `CODEOWNERS`.
- Those routes are public community help. They are not a support contract, and the page does not offer written technical guarantees or a price for a private sequence.
- [The Sigstore security policy](https://github.com/sigstore/community/security/policy) reserves `security@sigstore.dev` for vulnerability disclosure. The Security Response Committee acknowledges a vulnerability report within 24 hours. That address is excluded from this RFI. Personal addresses named in that policy are not copied here and are not recipients.
- The community repository describes `sigstore-dev@googlegroups.com` as the list for infrequent community communications. It is not documented as an RFI recipient.

`SEND_READY=NO`.

# 0.1U: Infrastructure inventory for a recovery anchor

Stage: `TRACKING-BINDING-RECOVERY-ANCHOR-INFRASTRUCTURE-DECISION-0.1U`.
Mode: read-only. No host was contacted. No secret value was read. No cloud resource was created.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`.
Reference: PR #390 at `cd22a3789725cf49a32a8f5bfc3a58980a9ce7dd`, unchanged.
Also unchanged: PR #386–#389.

Anything not stated by a committed document on this base is `NOT_VERIFIED`. Two addresses are not two failure domains until provider, storage, administration, and backup are shown to be disjoint.

## 1. What this inventory is allowed to say

Sources are committed documents, local SSH host entries, and the confirmed inputs in `THREE_SERVER_INDEPENDENCE.md`. Documentary vendor names in the table are not the confirmed provider name. Confirmed: `PROVIDER_COUNT=1`, `PROVIDER_NAME=NOT_VERIFIED`. This stage did not SSH, did not open a provider panel, and did not read `/protected`, `.env`, or private keys.

## 2. Known servers and services

| ID | What the documents say | Provider | Region / site | Storage and backup | Administration | Authentication | Network | Status for an anchor |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| S1 | `161.104.53.221`. July 2026 read-only notes say production vhost `xn--80abvubqje.xn--p1ai` and staging vhost `staging.xn--80abvubqje.xn--p1ai` shared `/var/www/bintrans-web-admin` on this address. Selectel security-group text appears in the same pilot notes. | Named as Selectel in those notes. Account id `NOT_VERIFIED`. | `NOT_VERIFIED` | Web-root symlink recorded. Volume snapshots, PITR, and whether this disk is in the same snapshot policy as S2: `NOT_VERIFIED`. | Operator inspection. Shared root with S2: `NOT_VERIFIED`. | `NOT_VERIFIED` in this stage. No credential was opened. | DNS notes point the staging name at this address. Whether the address still answers: `NOT_VERIFIED`. | `NOT_SUITABLE`. It has held both production and staging web roots. It is inside the documented Selectel operating set. |
| S2 | `161.104.57.152`, hostname `bintrans-control-tower-staging`. Compose project `bintrans-ct-staging`. Checkout path `/opt/bintrans/control-tower-staging`. Runbooks show `ssh` as `root` with key name `bintrans_selectel_staging`. | Selectel is named by the key and by `cr.selcloud.ru`. Account id `NOT_VERIFIED`. | `NOT_VERIFIED`. The address is in the same `161.104.0.0/16` documentation set as S1. That is not proof of one machine or of two sites. | Deploy snapshot path recorded as `/protected/bintrans/deploy-snapshots/ops-blk005-20260903T083100Z`. That is an image/deploy snapshot record, not a proven database PITR domain, and not proven separate from S1. | Documented login is `root`. Whether the same person or panel also administers S1 and the registry: `NOT_VERIFIED`, and not assumed to be separate. | Public-key SSH is what the runbook shows. Key material was not read. | Operator SSH plus a local tunnel. No staging-to-anchor path exists, because no anchor exists. | `NOT_SUITABLE` as an anchor. It is the BINTRANS staging control plane. |
| S3 | `161.104.49.146`, hostname `bintrans-test-2-e-2`, local SSH alias `bintrans-e2e`. Prior operator specification: Ubuntu 24.04.4 LTS, created from the staging image. The Compose project `bintrans-e2e-stand-v01a` is a design for a stand on a dedicated host; it is not a separate fourth machine. | Same confirmed provider count as S1 and S2. Name `NOT_VERIFIED`. | `NOT_VERIFIED`. | Disk product `NOT_VERIFIED`. Initial contents come from the staging image. Backup schedule `NOT_VERIFIED`. | `User root` on the same workstation as S1 and S2. Key file name `bintrans_e2e_ed25519_v2`. Material was not read. | Public-key SSH is configured locally. No connection was made in this check. | Routes to S1 and S2 `NOT_VERIFIED`. | `NOT_SUITABLE`. Shared root custody and staging-image lineage. See `THREE_SERVER_INDEPENDENCE.md`. |
| R1 | Container registry `cr.selcloud.ru/bintrans-staging`. | Selectel (`selcloud.ru`), same brand as S2. Tenancy shared with the VMs: `NOT_VERIFIED`. | `NOT_VERIFIED` | Registry stores image manifests. It is not a monotonic sequence log for `(namespace, incarnation, seq, digest)`. | Same staging publish flow as S2. Separate registry admin: `NOT_VERIFIED`. | Registry credentials exist in protected env examples. Values were not read. | Staging hosts pull from it. | `NOT_SUITABLE`. Object storage here is an image registry, not an anchor. |
| O1 | Object storage / WORM. Workstream status: `S3/WORM not configured (F-004)`. EDO notes: Selectel bucket creation and object lock are `EXTERNAL_INFRA_VERIFICATION_REQUIRED` and not done by application work. | Selectel is the named candidate. No bucket was confirmed. | `NOT_VERIFIED` | No WORM policy is recorded as enabled. | Would be the same INFRA owner as S2 unless a different account is created. That account does not exist in the inventory. | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_SUITABLE` as currently documented. A future bucket in the BINTRANS Selectel account would share admin and backup control with S2. |
| G1 | GitHub-hosted Actions runners (`ubuntu-latest`). | GitHub / Azure. Not a BINTRANS account. | Changes per job. `NOT_VERIFIED` as a stable site. | Ephemeral disk. Job files are deleted when the job ends. | GitHub administers the runner. BINTRANS can start workflows. | `GITHUB_TOKEN` is workflow-scoped. Not an anchor key. | Outbound HTTPS from the runner. | `NOT_SUITABLE`. No durable monotonic log and no independent restore story. |
| L1 | The workstation that ran 0.1Q, 0.1R, and 0.1S. PR #388 and #389 state that the witness and PostgreSQL shared one machine. | Local. | `NOT_VERIFIED` | Local disk. | The operator who runs the tests. | Test passwords were per attempt and are not recorded here. | Local only. Those containers used `--network none`. | `NOT_SUITABLE`. This is the failure domain already shown to be insufficient. |

Confirmed server count is 3. The third address is `161.104.49.146` (`bintrans-test-2-e-2`). Placement, storage, backup, and restore independence of the three are in `THREE_SERVER_INDEPENDENCE.md`. `PROVIDER_COUNT=1`. `PROVIDER_NAME=NOT_VERIFIED`. Count of hosts available as an isolated anchor: **zero**.

## 3. Shared properties that are visible without a live probe

| Property | What is documented | What is not |
| --- | --- | --- |
| Provider | S2 and R1 are Selectel. S1's notes also say Selectel. | A written account id, project id, or region for any of them. |
| Address family | S1 and S2 are both `161.104.x.x` in committed docs. | Whether they are the same hypervisor, the same availability zone, or even both still allocated. |
| Administration | S2 runbooks log in as `root`. S1 was inspected by operators who could read the web root. E2E design uses a different SSH policy on paper. | A list of IAM users, who holds the Selectel panel, and whether one credential can snapshot every volume. |
| Backup | One deploy-snapshot path on the staging side. | A backup product, a snapshot schedule, and whether S1, S2, and a future disk would be restorable by one action. |
| Credentials | Separate *names* exist: staging SSH key name, ephemeral E2E database password, registry env keys. | Proof that those secrets are in different custody, or that no person holds more than one. |
| Connectivity | Staging SSH is operator-initiated. E2E design says no route to staging. 0.1R/0.1S had no network between containers and still shared a host. | Any measurement of routes, security groups, or shared VPC. |
| Prices | None in the recovery documents or in this inventory. | Every cost figure. `COST=NOT_VERIFIED`. |

## 4. Consequence

`AVAILABLE_ISOLATED_HOSTS=0`.
`INDEPENDENT_FAILURE_DOMAINS=NOT_PROVEN`.
`BACKUP_DOMAIN_INDEPENDENCE=NOT_VERIFIED`.
`ADMIN_DOMAIN_INDEPENDENCE=NOT_VERIFIED`.
`CREDENTIAL_ISOLATION=NOT_VERIFIED`.

Using S2 as the platform and S1, S3, R1, or O1 as the anchor would still leave Scenario B open: one administrative domain, or one restore action, can move every copy together. That is the same shape as P3B in PR #389, moved from one Windows host to one provider account.

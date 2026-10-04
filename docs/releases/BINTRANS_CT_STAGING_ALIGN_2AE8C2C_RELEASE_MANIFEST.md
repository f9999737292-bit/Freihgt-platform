# BINTRANS Control Tower staging release manifest

Immutable closeout for the aligned staging release. This file records observed image digests, database state, and backup evidence. It does not change staging runtime.

Observed: 2026-10-04T14:45:10Z.

## Release identity

| Field | Value |
|-------|-------|
| LOGICAL_RELEASE_SHA | `2ae8c2c69e87d9363dd4982633c853ac6d99b59a` |
| RELEASE_TARGET_SHA | `2ae8c2c69e87d9363dd4982633c853ac6d99b59a` |
| STAGING_RELEASE_STATUS | `ALIGNED` |
| NOT_ALL_RUNNING_IMAGES_HAVE_THIS_OCI_REVISION | `YES` |
| DB_VERSION | `95` |
| DB_DIRTY | `false` |
| RUNNING_SERVICES | `22` |
| UNHEALTHY_SERVICES | `0` |
| ROLLBACK_USED | `NO` |
| MIGRATION_RUN | `NO` |
| MIGRATION_RUN_DURING_ALIGN_3F | `NO` |

Unchanged application images remain on earlier immutable digests. That is intentional. Their build inputs were reviewed before this release, and they were not rebuilt.

## Updated images

| Service | Digest | OCI revision |
|---------|--------|--------------|
| network-optimizer-service | `sha256:2171d53091a80f23319dbe19aea59581013e3b536e3aa19ceb85d6404df12a7b` | `2ae8c2c69e87d9363dd4982633c853ac6d99b59a` |
| api-gateway | `sha256:a3bd58c74ce2dfbd8f11ad72dd32b725c49ed97d27cf931ee4021e569f0ceb4e` | `2ae8c2c69e87d9363dd4982633c853ac6d99b59a` |

Tags are `git-2ae8c2c`. Running digests match the pin file.

## Unchanged application images

| Service | Digest |
|---------|--------|
| identity-service | `sha256:337f90d6fcd2d96d8c035fb52b8ed297aa9c088231e8fe2deb99c9a25677f154` |
| company-service | `sha256:ee9e83e42e828d0b6d0ce0e7035966b92e4ddac6880e22e1257c10d70f1104ef` |
| transport-order-service | `sha256:83467abbb067d3aa10d090c91b900b7c025b17628851a6514c4dbfee40f1a570` |
| rfx-service | `sha256:2f25f14671fafb135e5ca436525b633ecb781672d7a55bde7946be244043c5e8` |
| shipment-service | `sha256:bc97d7b29fe4902b2f2a9e6710bfec847dea38da41f2197def3788071564f72f` |
| document-service | `sha256:9ac13d6a249159059e0a1a8728f7aef4c54bdc5b1a31d20d635275728eabd67a` |
| billing-register-service | `sha256:0e13e6600962641d2fa65f38a1b5466067107f1e4738705e7c6cf9feb348f7de` |
| low-code-service | `sha256:f3340a04025ba07044f178327e75b2eb19290ffb89d183f4633275bcebf5bafd` |
| payment-service | `sha256:26783fe0242544c490973409d1b47f0da032cac66b573c6de2602e89d2a123a1` |
| contract-rate-service | `sha256:d5fdfb2d0ba284f416f2a0d7bbd77ecef93d2760e33217f500888d5fd598be9a` |
| freight-cost-service | `sha256:f3e76109eb4ce0780c392e9a7f8b561bbadcc9b6066782972fb2b84d9a433c91` |
| control-tower-read-model-service | `sha256:18ff59ad22e68acd6ba2838947ffeebad5086a820be6cafab95b9b801377b3c7` |
| web-admin | `sha256:b75ce669dd1323ad6056ef4323cafaaa39f428916b044dca48110fab0ea8e8cd` |

`UNCHANGED_SERVICE_DIGESTS_VERIFIED=YES`

Historical OCI revisions still present on unchanged services include `4c8b22e8d447b7c80a3ca73a427a0a9630f916be` (shipment-service), `fec0100d21b8d89a95369b3ae8f75e759e322dbb` (document-service), and `9c6562d80f88da2bf41151605281e2d491f7ebb0` (control-tower-read-model-service).

## Database

`public.schema_migrations` was read as version `95`, dirty `false`. No migration was applied during alignment.

## Backup

| Field | Value |
|-------|-------|
| BACKUP_FILE | `/protected/bintrans/backups/freight_platform_20261004T142022Z.dump` |
| BACKUP_SHA256 | `49b5b9683a314d5ff700ed7b9dff2227c1001e5ab8fc9e825611713a67933801` |
| BACKUP_VALIDATED | `YES` |
| BACKUP_TIMER_ACTIVE | `active` |
| BACKUP_SERVICE_LAST_RESULT | `success` |
| LAST_SUCCESSFUL_RUN | `2026-10-04T14:20:22Z` to `2026-10-04T14:20:24Z` |
| NEXT_BACKUP_TRIGGER | `2026-10-05T00:00:00Z` |

The timer is enabled. Its last scheduled trigger was `2026-10-04T00:00:09Z`. The recorded successful result is the later manual oneshot, not that scheduled trigger.

## Health

| Check | Result |
|-------|--------|
| postgres | healthy |
| api-gateway | healthy; `/health` 200; `/ready` 200 |
| shipment-service | healthy |
| document-service | healthy |
| control-tower-read-model-service | healthy |
| network-optimizer-service | healthy; `/health` 200; `/ready` 200 |

`OPENAPI_EDO_ATTACHMENT_CONTRACT_PRESENT=YES`. The served document contract contains the six attachment paths, including content, finalize, and signatures.

## Pin evidence

| Field | Value |
|-------|-------|
| PIN_FILE_PATH | `/opt/bintrans/control-tower-staging/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-images.yml` |
| PIN_FILE_SHA | `f9dfcb22c5b784966df3b0278f1726aae7f69a76968e1103d67dd0b9be24723f` |
| PIN_ROLLBACK_FILE | `/opt/bintrans/control-tower-staging/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-images.yml.align3f-20261004T143416Z.bak` |
| PIN_ROLLBACK_FILE_SHA | `29ba9733da21dce0f50cb15e675b1817d49c5d8c5bde12bcc302f1ea0bdb1cae` |
| PIN_RUNTIME_ALIGNMENT | `PASS` |
| EFFECTIVE_CONFIG_VALID | `YES` |

Effective config was rendered with profiles `messaging`, `read-model`, and `observability`. Rendered application digests match the pin file.

## Host checkout

| Field | Value |
|-------|-------|
| LIVE_CHECKOUT_HEAD | `4c8b22e8d447b7c80a3ca73a427a0a9630f916be` |
| HOST_CHECKOUT_HEAD | `4c8b22e8d447b7c80a3ca73a427a0a9630f916be` |
| LIVE_CHECKOUT_DIRTY | `YES` |
| HOST_CHECKOUT_DIRTY | `YES` |

The live checkout was not reset, cleaned, or updated. Release truth is this manifest, the image digests, the database version, and the effective compose config.

## Residual debt

These items are recorded and were not changed by this closeout:

- D1: the live checkout is old and dirty.
- D2: alertmanager, postgres-exporter, and node-exporter are reported as compose orphans.
- D3: the host staging overlay differs from `main` on purpose.
- D4: unchanged services keep older OCI revisions.

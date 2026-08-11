# BINTRANS Staging Image Provenance

## Operator metadata (not embedded in images)

| Field | Expected value |
|-------|----------------|
| Runtime source SHA | `b75eb3d` |
| Publish tag | `git-b75eb3d` |
| Registry | `cr.selcloud.ru/bintrans-staging` |

## Repository facts

- Service `Dockerfile`s under `services/*/Dockerfile` contain **no** `LABEL` or `ARG` for GIT SHA / source revision.
- Local compose builds use project/service names (e.g. `freight-platform-identity-service`), not necessarily the BINTRANS registry tag.
- **Tag name alone is not cryptographic proof** that an image was built from SHA `b75eb3d`.

## What operator tooling can verify locally

`bintrans_ct_staging_image_provenance_check.sh`:

- Checks presence of local images tagged `${BINTRANS_REGISTRY}/<service>:git-b75eb3d` for all 10 runtime services.
- Reports image `Created` timestamp from `docker image inspect`.
- Does **not** assert git SHA from image metadata (unavailable).

## Recommended live publish flow

1. Build from reviewed commit `b75eb3d` (operator records checkout SHA).
2. Tag `cr.selcloud.ru/bintrans-staging/<service>:git-b75eb3d`.
3. Push tag; capture **registry digest** via `docker inspect --format='{{index .RepoDigests 0}}'`.
4. Pin runtime deploy to `@sha256:<digest>` in protected env.
5. Run `bintrans_ct_staging_runtime_images_validate.sh`.

Digest pinning is the deploy-time integrity gate; mutable tags are publish-only.

## Limitation statement

Until Dockerfiles add reproducible source labels (out of scope for staging-pack), provenance is:

**operator checkout SHA + build log + registry digest** — not image self-description.

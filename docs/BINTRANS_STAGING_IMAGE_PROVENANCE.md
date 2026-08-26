# BINTRANS Staging Image Provenance

## Contract (v0.5A+)

| Field | Value |
|-------|-------|
| Release SHA | `DEPLOYED_GIT_SHA` — full 40-char Git SHA in protected env |
| Publish tag | `BINTRANS_IMAGE_TAG=git-<short SHA>` — deterministically derived |
| OCI label | `org.opencontainers.image.revision=<full DEPLOYED_GIT_SHA>` on every application image |
| Optional labels | `org.opencontainers.image.version`, `org.opencontainers.image.source` |

Build with provenance labels:

```bash
git checkout <DEPLOYED_GIT_SHA>
make bintrans-staging-release-build
```

This builds all 13 canonical staging application services using the staging compose stack and passes `BINTRANS_GIT_SHA` + `BINTRANS_IMAGE_VERSION` to every build.

## Validation

`bintrans_ct_staging_image_provenance_check.sh` verifies:

- All 13 runtime images present locally (or digest refs from protected env)
- `org.opencontainers.image.revision` **exactly equals** `DEPLOYED_GIT_SHA`

**Target:** `FUTURE_RELEASE_REVISION_EVIDENCE=EXACT` — no LIKELY classification for newly built releases.

## Digest mode (preferred for runtime deploy)

Runtime start requires digest-pinned references:

`cr.selcloud.ru/bintrans-staging/<service>@sha256:<64-hex>`

Tag-only references are rejected by runtime preflight.

## Historical note

Prior releases relied on tag naming (`git-b75eb3d`) without OCI revision labels. That approach is superseded for new releases after v0.5A tooling merge.

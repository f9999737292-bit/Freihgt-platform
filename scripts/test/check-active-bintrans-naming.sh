#!/usr/bin/env bash
# Fail if active configuration/docs introduce erroneous 7rights associations.
# Historical audit evidence and identity migration tooling are allowlisted.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

echo "=== BINTRANS Active Naming Gate ==="

PATTERN='7rights\.ru|staging\.7rights|pilot\.7rights|api\.7rights|7rights[_-]ct[_-]|7rights[_-]control[_-]|7rights[_-]staging|7rights_ct_staging|7rights-ct-staging|@7rights\.local|dev-7rights|7Rights Dev|7RIGHT Control|7Rights Freight'

RG_EXCLUDES=(
  --glob '!**/node_modules/**'
  --glob '!**/.git/**'
  --glob '!**/dist/**'
  --glob '!**/.nuxt/**'
  --glob '!pnpm-lock.yaml'
  --glob '!scripts/ops/bintrans_identity_a2/**'
  --glob '!scripts/test/check-active-bintrans-naming.sh'
  --glob '!docs/*EVIDENCE*.md'
  --glob '!docs/*VERIFICATION*.md'
  --glob '!docs/*SIGNOFF*.md'
  --glob '!docs/LOCAL_SELECTEL_STAGING_MODIFIED_DOCS_REVIEW_V0.1.md'
  --glob '!docs/PROJECT_AUDIT_REPORT_V0.1.md'
  --glob '!docs/RUNTIME_VERIFICATION_REPORT_V0.1.md'
  --glob '!docs/UI_RUNTIME_VERIFICATION_V0.1.md'
  --glob '!docs/DEMO_UI_VERIFICATION_V0.1.md'
  --glob '!docs/DEMO_SCENARIO_FINAL_SIGNOFF_V0.1.md'
  --glob '!docs/DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_EVIDENCE_V0.2.md'
  --glob '!docs/DEMO_CREDENTIALS_AND_SEED_DATA_CLEANUP_TRACKING_V0.2.md'
  --glob '!docs/RBAC_ROLE_NAVIGATION_STAGING_POST_DEPLOY_REVIEW_EVIDENCE_V0.1.md'
  --glob '!docs/RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_RETRY_EVIDENCE_V0.1.md'
  --glob '!docs/PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_POST_DEPLOY_REVIEW_EVIDENCE_V0.1.md'
  --glob '!docs/PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_EXECUTION_EVIDENCE_V0.1.md'
  --glob '!docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md'
  --glob '!docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_EVIDENCE_V0.1.md'
  --glob '!docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_VERIFICATION_V0.1.md'
  --glob '!docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md'
  --glob '!docs/LOW_CODE_UI_TENANT_VERIFICATION_V0.1.md'
  --glob '!docs/LOW_CODE_RUNTIME_READINESS_REVIEW_V0.1.md'
)

SCAN_PATHS=(
  .github
  scripts/ops/bintrans_ct_staging
  scripts/dev
  scripts/test
  infrastructure
  services
  apps
  packages
  docs/AUTH_RBAC.md
  docs/QUICK_START.md
  docs/NEXT_COMMANDS.md
  docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md
  docs/BINTRANS_STAGING_AUTH_SMOKE.md
  docs/BINTRANS_STAGING_SHADOW_SMOKE.md
  docs/ops
  docs/engineering
  docs/LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md
  docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md
  docs/LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md
  docs/LOW_CODE_PILOT_LAUNCH_REHEARSAL_V0.1.md
  docs/LOW_CODE_PILOT_HANDOFF_NOTE_V0.1.md
  docs/LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md
  docs/LOW_CODE_PILOT_FIX_POLISH_SPRINT_V0.1.md
  docs/LOW_CODE_PILOT_MANUAL_UI_VERIFICATION_V0.1.md
  docs/LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md
  docs/LOW_CODE_PREVIEW_CONTEXT_V0.1.md
  docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md
  docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md
  docs/LOW_CODE_ADMIN_TEMPLATE_IMPORT_EXPORT_UI_V0.1.md
  docs/LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_SCHEDULING_NOTE_V0.1.md
  docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_V0.1.md
  docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_RETRY_V0.1.md
  docs/LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_ACTION_PLAN_V0.1.md
  Makefile
  infrastructure/docker-compose
)

existing_paths=()
for path in "${SCAN_PATHS[@]}"; do
  [[ -e "$path" ]] && existing_paths+=("$path")
done

if [[ ${#existing_paths[@]} -eq 0 ]]; then
  echo "WARN: no scan paths found"
  exit 0
fi

if ! command -v rg >/dev/null 2>&1; then
  echo "ERROR: ripgrep (rg) required for active naming gate"
  exit 1
fi

if rg -n -i -e "$PATTERN" "${RG_EXCLUDES[@]}" "${existing_paths[@]}"; then
  echo ""
  echo "ERROR: active 7rights false-association references detected (see above)."
  echo "7rights.ru is an unrelated external site. Use BINTRANS/bintrans/бинтранс.рф naming."
  exit 1
fi

echo "OK: no active 7rights false-association references in scanned paths"
echo "ACTIVE NAMING GATE PASSED"

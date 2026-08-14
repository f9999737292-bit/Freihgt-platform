#!/usr/bin/env bash
set -euo pipefail
source /protected/bintrans/control-tower-observation/staging.env
check() { if [[ -n "${!1:-}" ]]; then echo "${1}_SET=YES"; else echo "${1}_SET=NO"; fi; }
check JWT_TOKEN
check DEV_ADMIN_EMAIL
check DEV_ADMIN_PASSWORD
check ADMIN_EMAIL
check ADMIN_PASSWORD
check TENANT_ID
check BINTRANS_STAGING_AUTH_TEST_EMAIL
check BINTRANS_STAGING_AUTH_TEST_PASSWORD

#!/usr/bin/env bash
# Prints ephemeral stand database variables to stdout.
# Do not commit the output and do not reuse a platform or staging password.
set -euo pipefail

user="stand_$(openssl rand -hex 4)"
password="$(openssl rand -hex 24)"
database="bintrans_e2e_stand"

printf 'BINTRANS_E2E_STAND_POSTGRES_USER=%s\n' "$user"
printf 'BINTRANS_E2E_STAND_POSTGRES_PASSWORD=%s\n' "$password"
printf 'BINTRANS_E2E_STAND_POSTGRES_DB=%s\n' "$database"

#!/usr/bin/env bash
# Install BINTRANS pilot daily backup systemd service + timer.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
UNIT_DIR="${ROOT}/infrastructure/systemd"
SERVICE_SRC="${UNIT_DIR}/bintrans-pilot-backup.service"
TIMER_SRC="${UNIT_DIR}/bintrans-pilot-backup.timer"

[[ -f "${SERVICE_SRC}" && -f "${TIMER_SRC}" ]] \
  || { echo "missing systemd unit files under ${UNIT_DIR}" >&2; exit 1; }

install -m 0644 "${SERVICE_SRC}" /etc/systemd/system/bintrans-pilot-backup.service
install -m 0644 "${TIMER_SRC}" /etc/systemd/system/bintrans-pilot-backup.timer
systemctl daemon-reload
systemctl enable bintrans-pilot-backup.timer
systemctl start bintrans-pilot-backup.timer

echo "SERVICE_UNIT_LOADED=YES"
echo "TIMER_UNIT_LOADED=YES"
echo "TIMER_ENABLED=$(systemctl is-enabled bintrans-pilot-backup.timer)"
echo "TIMER_ACTIVE=$(systemctl is-active bintrans-pilot-backup.timer)"
systemctl list-timers bintrans-pilot-backup.timer --no-pager || true
echo "bintrans-pilot-backup-systemd-install: PASS"

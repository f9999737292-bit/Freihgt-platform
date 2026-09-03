#!/usr/bin/env bash
# Remove BINTRANS pilot daily backup systemd timer (does not delete backups).
set -euo pipefail

systemctl stop bintrans-pilot-backup.timer 2>/dev/null || true
systemctl disable bintrans-pilot-backup.timer 2>/dev/null || true
systemctl stop bintrans-pilot-backup.service 2>/dev/null || true
rm -f /etc/systemd/system/bintrans-pilot-backup.service
rm -f /etc/systemd/system/bintrans-pilot-backup.timer
systemctl daemon-reload
echo "bintrans-pilot-backup-systemd-remove: PASS"

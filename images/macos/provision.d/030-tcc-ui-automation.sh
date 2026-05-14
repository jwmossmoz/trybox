#!/usr/bin/env bash
set -Eeuo pipefail

update_tcc_db() {
  local db="$1"

  [[ -e "$db" ]] || return 0

  sudo sqlite3 "$db" <<'SQL'
INSERT OR REPLACE
INTO access (
  service,
  client_type,
  client,
  auth_value,
  auth_reason,
  auth_version,
  indirect_object_identifier_type,
  indirect_object_identifier
) VALUES
('kTCCServiceAccessibility', 1, '/usr/libexec/sshd-keygen-wrapper', 2, 0, 1, NULL, 'UNUSED'),
('kTCCServiceScreenCapture', 1, '/usr/libexec/sshd-keygen-wrapper', 2, 0, 1, NULL, 'UNUSED'),
('kTCCServicePostEvent', 1, '/usr/libexec/sshd-keygen-wrapper', 2, 0, 1, NULL, 'UNUSED'),
('kTCCServiceAppleEvents', 1, '/usr/libexec/sshd-keygen-wrapper', 2, 0, 1, 0, 'com.apple.systemevents'),
('kTCCServiceAppleEvents', 1, '/usr/libexec/sshd-keygen-wrapper', 2, 0, 1, 0, 'com.apple.Safari'),
('kTCCServiceAccessibility', 1, '/usr/bin/osascript', 2, 0, 1, NULL, 'UNUSED'),
('kTCCServiceScreenCapture', 1, '/usr/bin/osascript', 2, 0, 1, NULL, 'UNUSED'),
('kTCCServicePostEvent', 1, '/usr/bin/osascript', 2, 0, 1, NULL, 'UNUSED'),
('kTCCServiceAppleEvents', 1, '/usr/bin/osascript', 2, 0, 1, 0, 'com.apple.systemevents'),
('kTCCServiceAppleEvents', 1, '/usr/bin/osascript', 2, 0, 1, 0, 'com.apple.Safari');
SQL
}

sudo pkill -9 tccd >/dev/null 2>&1 || true
update_tcc_db "/Library/Application Support/com.apple.TCC/TCC.db"
update_tcc_db "$HOME/Library/Application Support/com.apple.TCC/TCC.db"
sudo pkill -9 tccd >/dev/null 2>&1 || true

#!/usr/bin/env bash
set -Eeuo pipefail

append_once() {
  local file="$1"
  local line="$2"

  touch "$file"
  grep -Fxq "$line" "$file" || printf '%s\n' "$line" >>"$file"
}

ZPROFILE="$HOME/.zprofile"

append_once "$ZPROFILE" 'export LANG=en_US.UTF-8'
append_once "$ZPROFILE" 'export HOMEBREW_NO_AUTO_UPDATE=1'
append_once "$ZPROFILE" 'export HOMEBREW_NO_INSTALL_CLEANUP=1'
append_once "$ZPROFILE" "if [[ -x /opt/homebrew/bin/brew ]]; then eval \"\$(/opt/homebrew/bin/brew shellenv)\"; fi"

if [[ ! -e "$HOME/.profile" ]]; then
  ln -s "$ZPROFILE" "$HOME/.profile"
fi

sudo mdutil -a -i off >/dev/null 2>&1 || true
sudo systemsetup -setremotelogin on >/dev/null 2>&1 || true
sudo safaridriver --enable >/dev/null 2>&1 || true

cat <<'PLIST' >/tmp/limit.maxfiles.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>limit.maxfiles</string>
  <key>ProgramArguments</key>
  <array>
    <string>launchctl</string>
    <string>limit</string>
    <string>maxfiles</string>
    <string>1048576</string>
    <string>1048576</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>ServiceIPC</key>
  <false/>
</dict>
</plist>
PLIST

sudo mv /tmp/limit.maxfiles.plist /Library/LaunchDaemons/limit.maxfiles.plist
sudo chown root:wheel /Library/LaunchDaemons/limit.maxfiles.plist
sudo chmod 0644 /Library/LaunchDaemons/limit.maxfiles.plist
sudo launchctl load -w /Library/LaunchDaemons/limit.maxfiles.plist >/dev/null 2>&1 || true

mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"
ssh-keygen -F github.com -f "$HOME/.ssh/known_hosts" >/dev/null 2>&1 || ssh-keyscan github.com >>"$HOME/.ssh/known_hosts"
chmod 600 "$HOME/.ssh/known_hosts"

if [[ "$(uname -m)" == "arm64" ]]; then
  /usr/sbin/softwareupdate --install-rosetta --agree-to-license >/dev/null 2>&1 || true
fi

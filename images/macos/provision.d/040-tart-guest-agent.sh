#!/usr/bin/env bash
set -Eeuo pipefail

if [[ -x /opt/homebrew/bin/brew ]]; then
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

brew list --versions tart-guest-agent >/dev/null 2>&1 || brew install cirruslabs/cli/tart-guest-agent

cat <<'PLIST' >/tmp/org.cirruslabs.tart-guest-daemon.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>org.cirruslabs.tart-guest-daemon</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/tart-guest-agent</string>
    <string>--run-daemon</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>/bin:/usr/bin:/usr/sbin:/usr/local/bin:/opt/homebrew/bin</string>
  </dict>
  <key>WorkingDirectory</key>
  <string>/var/empty</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/tart-guest-daemon.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/tart-guest-daemon.log</string>
</dict>
</plist>
PLIST

cat <<'PLIST' >/tmp/org.cirruslabs.tart-guest-agent.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>org.cirruslabs.tart-guest-agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/tart-guest-agent</string>
    <string>--run-agent</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>/bin:/usr/bin:/usr/sbin:/usr/local/bin:/opt/homebrew/bin</string>
    <key>TERM</key>
    <string>xterm-256color</string>
  </dict>
  <key>WorkingDirectory</key>
  <string>/Users/admin</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/tart-guest-agent.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/tart-guest-agent.log</string>
</dict>
</plist>
PLIST

sudo mv /tmp/org.cirruslabs.tart-guest-daemon.plist /Library/LaunchDaemons/org.cirruslabs.tart-guest-daemon.plist
sudo chown root:wheel /Library/LaunchDaemons/org.cirruslabs.tart-guest-daemon.plist
sudo chmod 0644 /Library/LaunchDaemons/org.cirruslabs.tart-guest-daemon.plist
sudo launchctl bootout system /Library/LaunchDaemons/org.cirruslabs.tart-guest-daemon.plist >/dev/null 2>&1 || true
sudo launchctl bootstrap system /Library/LaunchDaemons/org.cirruslabs.tart-guest-daemon.plist

sudo mv /tmp/org.cirruslabs.tart-guest-agent.plist /Library/LaunchAgents/org.cirruslabs.tart-guest-agent.plist
sudo chown root:wheel /Library/LaunchAgents/org.cirruslabs.tart-guest-agent.plist
sudo chmod 0644 /Library/LaunchAgents/org.cirruslabs.tart-guest-agent.plist
sudo launchctl bootout "gui/$(id -u)" /Library/LaunchAgents/org.cirruslabs.tart-guest-agent.plist >/dev/null 2>&1 || true
launchctl bootstrap "gui/$(id -u)" /Library/LaunchAgents/org.cirruslabs.tart-guest-agent.plist >/dev/null 2>&1 || true

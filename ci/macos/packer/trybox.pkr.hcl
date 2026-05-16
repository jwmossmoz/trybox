packer {
  required_plugins {
    tart = {
      version = ">= 1.20.0"
      source  = "github.com/cirruslabs/tart"
    }
  }
}

variable "vm_name" {
  type = string
}

variable "ipsw" {
  type = string
}

variable "cpu_count" {
  type    = number
  default = 8
}

variable "memory_gb" {
  type    = number
  default = 16
}

variable "disk_size_gb" {
  type    = number
  default = 200
}

variable "display" {
  type    = string
  default = "1920x1200"
}

variable "headless" {
  type    = bool
  default = false
}

variable "setup_flow" {
  type    = string
  default = "macos15"
}

source "tart-cli" "trybox" {
  from_ipsw          = var.ipsw
  vm_name            = var.vm_name
  cpu_count          = var.cpu_count
  memory_gb          = var.memory_gb
  disk_size_gb       = var.disk_size_gb
  display            = var.display
  headless           = var.headless
  ssh_username       = "admin"
  ssh_password       = "admin"
  ssh_timeout        = "30m"
  create_grace_time  = "30s"
  recovery_partition = "keep"

  # Setup Assistant flows are empirically verified against specific IPSWs.
  # Apple changes SA pages between point releases; if this regresses, walk a
  # fresh install with VNC/OCR and re-snapshot the new page order.
  boot_command = var.setup_flow == "macos26" ? [
    # Hello / Welcome
    "<wait60s><spacebar>",
    # Select Your Language
    "<wait30s>italiano<esc>english<enter>",
    # Select Your Country or Region. United States is selected by default after
    # choosing English on the 26.4 IPSW.
    "<wait 'Select Your Country or Region'>",
    "<wait2s><click '^Continue$'>",
    # Transfer Your Data to This Mac
    "<wait 'Transfer Your Data'>",
    "<wait2s><click 'Set up as new'>",
    "<wait2s><click '^Continue$'>",
    # Written and Spoken Languages
    "<wait 'Written and Spoken Languages'>",
    "<wait2s><click '^Continue$'>",
    # Accessibility
    "<wait 'Accessibility'>",
    "<wait2s><click 'Not Now'>",
    # Data & Privacy
    "<wait 'Data . Privacy'>",
    "<wait2s><click '^Continue$'>",
    # Create a Mac Account.
    "<wait 'Create a Mac Account'>",
    "<wait2s><click 'Full Name'>Managed via Tart<tab>admin<tab>admin<tab>admin<tab><tab><spacebar><tab><tab><spacebar>",
    # Sign In to Your Apple Account. macOS 26 exposes skip through a menu.
    "<wait120s><wait 'Sign In to Your Apple Account'>",
    "<wait2s><click 'Other Sign-In Options'>",
    "<wait2s><down><down><enter>",
    "<wait 'Are you sure you want to skip'>",
    "<wait2s><click '^Skip$'>",
    # Terms and Conditions.
    "<wait 'Terms and Conditions'>",
    "<wait2s><click '^Agree$'>",
    "<wait 'I have read and agree'>",
    "<wait2s><click '^Agree$'>",
    # Age Range is new in the 26.4 Setup Assistant flow.
    "<wait 'Age Range'>",
    "<wait2s><click 'Adult'>",
    # Location Services first exposes a checkbox/Continue page, then asks for
    # confirmation before the time zone page.
    "<wait 'Enable Location Services'>",
    "<wait2s><click '^Continue$'>",
    "<wait2s><click 'Don.t Use'>",
    "<wait 'Select Your Time Zone'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Analytics'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Screen Time'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Siri'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Select a Siri Voice'>",
    "<wait2s><click 'Choose For Me'>",
    "<wait 'Improve Siri . Dictation'>",
    "<wait2s><click 'Not Now'>",
    "<wait2s><click '^Continue$'>",
    # Keep FileVault off in reusable base images.
    "<wait 'Your Mac is Ready for FileVault'>",
    "<wait2s><click 'Not Now'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Choose Your Look'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Update Mac Automatically'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'welcome'>",
    "<wait2s><click 'Get Started'>",
    # Open Terminal and finish system configuration via CLI.
    "<wait15s><leftAltOn><spacebar><leftAltOff>Terminal<enter>",
    "<wait15s>defaults write NSGlobalDomain AppleKeyboardUIMode -int 3<enter>",
    "<wait5s>echo admin | sudo -S sh -c 'systemsetup -settimezone UTC; systemsetup -setremotelogin on; launchctl load -w /System/Library/LaunchDaemons/ssh.plist 2>/dev/null; launchctl load -w /System/Library/LaunchDaemons/com.apple.screensharing.plist 2>/dev/null; spctl --global-disable'<enter>",
    "<wait20s><leftAltOn>q<leftAltOff>",
    ] : [
    # Hello / Welcome
    "<wait60s><spacebar>",
    # Select Your Language
    "<wait30s>italiano<esc>english<enter>",
    # Select Your Country or Region
    "<wait30s>united states<leftShiftOn><tab><leftShiftOff><spacebar>",
    # Input Sources (added in macOS 15.6) - default US selection is fine
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # Dictation (added in macOS 15.6) - default English (US) is fine
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # Accessibility - "Not Now" sits where Continue normally does
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # Data & Privacy - Continue
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # Transfer Information - "Not now" is a link bottom-left, Continue is grayed.
    # From default focus, three Shift+Tab presses reach the "Not now" link.
    "<wait10s><leftShiftOn><tab><tab><tab><leftShiftOff><spacebar>",
    # Create a Mac Account: full name, account, pw, verify, skip hint, uncheck
    # "Allow Apple Account to reset password", Continue.
    "<wait10s>Managed via Tart<tab>admin<tab>admin<tab>admin<tab><tab><spacebar><tab><tab><spacebar>",
    # Account creation runs for ~90s. Enable VoiceOver afterwards: Apple's
    # Setup Assistant controls are much more predictable by keyboard with it on,
    # and this matches the current Cirrus Sequoia template.
    "<wait120s><leftAltOn><f5><leftAltOff>",
    # Sign In to Your Apple Account. macOS 15.6.1 still shows this page even
    # when the account reset checkbox is unchecked.
    "<wait 'Email or Phone Number'>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait 'Don.t Skip'>",
    "<wait2s><tab><spacebar>",
    # Terms and Conditions.
    "<wait 'Terms and Conditions'>",
    "<wait2s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # The OCR frame often truncates this modal heading after "AGRE".
    "<wait 'SOFTWARE LICENSE AGRE'>",
    "<wait2s><tab><spacebar>",
    "<wait 'I have read and agree'>",
    "<wait2s><tab><spacebar>",
    # Location Services. With VoiceOver enabled, the 15.6.1 flow advances to
    # Analytics without presenting the manual time zone page. The final timezone
    # is forced to UTC from Terminal after Setup Assistant exits.
    "<wait 'Enable Location Services'>",
    "<wait2s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # Analytics. OCR sees Continue, but clicking it lands on the analytics
    # checkbox/text region on 15.6.1. The VoiceOver keyboard path advances.
    "<wait 'Analytics'>",
    "<wait2s><leftShiftOn><tab><leftShiftOff><spacebar>",
    # Screen Time. VoiceOver must be off before the OCR click or the click only
    # focuses text. On this IPSW, the bottom-left Set Up Later text remains
    # greyed after the account path, while Continue is active and advances.
    "<wait 'Screen Time'>",
    "<wait2s><leftAltOn><f5><leftAltOff>",
    "<wait2s><click '^Continue$'>",
    # Siri defaults to enabled on the 15.6.1 IPSW. VoiceOver is useful for the
    # earlier keyboard path, but its focus overlay makes late OCR clicks land on
    # images/text instead of activating buttons, so keep it off for the
    # remaining mouse-driven pages.
    "<wait 'Siri'>",
    "<wait2s><click '^Continue$'>",
    "<wait 'Select a Siri Voice'>",
    "<wait2s><click 'Choose For Me'>",
    "<wait 'Improve Siri . Dictation'>",
    "<wait2s><click 'Not Now'>",
    "<wait2s><click '^Continue$'>",
    # Choose Your Look
    "<wait 'Choose Your Look'>",
    "<wait2s><click '^Continue$'>",
    # Update Mac Automatically
    "<wait 'Update Mac Automatically'>",
    "<click '^Continue$'>",
    # Welcome to Mac uses a large full-screen control that OCR does not
    # consistently expose as a button; Return activates the default continue
    # control.
    "<wait 'Welcome to Mac'>",
    "<wait2s><enter>",
    # Open Terminal and finish system configuration via CLI (more robust than
    # tabbing through System Settings, which Apple reshapes every release).
    "<wait15s><leftAltOn><spacebar><leftAltOff>Terminal<enter>",
    "<wait15s>defaults write NSGlobalDomain AppleKeyboardUIMode -int 3<enter>",
    "<wait5s>echo admin | sudo -S sh -c 'systemsetup -settimezone UTC; systemsetup -setremotelogin on; launchctl load -w /System/Library/LaunchDaemons/ssh.plist 2>/dev/null; launchctl load -w /System/Library/LaunchDaemons/com.apple.screensharing.plist 2>/dev/null; spctl --global-disable'<enter>",
    "<wait20s><leftAltOn>q<leftAltOff>",
  ]
}

build {
  sources = ["source.tart-cli.trybox"]

  provisioner "shell" {
    inline_shebang = "/bin/bash -e"
    inline = [
      "echo admin | sudo -S sh -c \"mkdir -p /etc/sudoers.d && echo 'admin ALL=(ALL) NOPASSWD: ALL' | EDITOR=tee visudo -f /etc/sudoers.d/admin-nopasswd\"",
      "echo '00000000: 1ced 3f4a bcbc ba2c caca 4e82' | sudo xxd -r - /etc/kcpassword",
      "sudo defaults write /Library/Preferences/com.apple.loginwindow autoLoginUser admin",
      "sudo defaults write /Library/Preferences/com.apple.screensaver loginWindowIdleTime 0",
      "defaults -currentHost write com.apple.screensaver idleTime 0",
      "sudo systemsetup -setsleep Off 2>/dev/null",
      "sysadminctl -screenLock off -password admin",
      "sudo safaridriver --enable",
    ]
  }

  provisioner "shell" {
    inline_shebang = "/bin/bash -e"
    inline = [
      "touch /tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress",
      "CLT_LABEL=$(softwareupdate --list | sed -n 's/.*Label: \\(Command Line Tools for Xcode-.*\\)/\\1/p' | tail -1)",
      "if [[ -n \"$CLT_LABEL\" ]]; then softwareupdate --install \"$CLT_LABEL\"; fi",
      "rm -f /tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress",
    ]
  }

  provisioner "shell" {
    script = "${path.root}/../provision.d/010-trybox-base.sh"
  }

  provisioner "shell" {
    script = "${path.root}/../provision.d/020-firefox-build-deps.sh"
  }

  provisioner "shell" {
    script = "${path.root}/../provision.d/040-tart-guest-agent.sh"
  }

  provisioner "shell" {
    inline_shebang = "/bin/bash -e"
    inline = [
      "source ~/.zprofile",
      "brew --version",
      "command -v tart-guest-agent",
      "sudo -n true",
      "test -f ~/.ssh/known_hosts",
    ]
  }
}

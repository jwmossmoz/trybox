packer {
  required_plugins {
    tart = {
      version = ">= 1.12.0"
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

  boot_command = [
    "<wait60s><spacebar>",
    "<wait30s>italiano<esc>english<enter>",
    "<wait30s>united states<leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><tab><tab><tab><spacebar><tab><tab><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s>Managed via Tart<tab>admin<tab>admin<tab>admin<tab><tab><spacebar><tab><tab><spacebar>",
    "<wait120s><leftAltOn><f5><leftAltOff>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><tab><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><tab><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><tab><spacebar>",
    "<wait10s><tab><tab>UTC<enter><leftShiftOn><tab><tab><leftShiftOff><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><tab><spacebar>",
    "<wait10s><tab><spacebar><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><spacebar>",
    "<wait10s><tab><spacebar>",
    "<wait10s><spacebar>",
    "<leftAltOn><f5><leftAltOff>",
    "<wait10s><leftAltOn><spacebar><leftAltOff>Terminal<enter>",
    "<wait10s>defaults write NSGlobalDomain AppleKeyboardUIMode -int 3<enter>",
    "<wait10s><leftAltOn>q<leftAltOff>",
    "<wait10s><leftAltOn><spacebar><leftAltOff>System Settings<enter>",
    "<wait10s><leftCtrlOn><f2><leftCtrlOff><right><right><right><down>Sharing<enter>",
    "<wait10s><tab><tab><tab><tab><tab><tab><tab><spacebar>",
    "<wait10s><tab><tab><tab><tab><tab><tab><tab><tab><tab><tab><tab><tab><spacebar>",
    "<wait10s><leftAltOn>q<leftAltOff>",
    "<wait10s><leftAltOn><spacebar><leftAltOff>Terminal<enter>",
    "<wait10s>sudo spctl --global-disable<enter>",
    "<wait10s>admin<enter>",
    "<wait10s><leftAltOn>q<leftAltOff>",
    "<wait10s><leftAltOn><spacebar><leftAltOff>System Settings<enter>",
    "<wait10s><leftCtrlOn><f2><leftCtrlOff><right><right><right><down>Privacy & Security<enter>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><leftShiftOn><tab><leftShiftOff><leftShiftOn><tab><leftShiftOff><leftShiftOn><tab><leftShiftOff><leftShiftOn><tab><leftShiftOff><leftShiftOn><tab><leftShiftOff><leftShiftOn><tab><leftShiftOff>",
    "<wait10s><down><wait1s><down><wait1s><enter>",
    "<wait10s>admin<enter>",
    "<wait10s><leftShiftOn><tab><leftShiftOff><wait1s><spacebar>",
    "<wait10s><leftAltOn>q<leftAltOff>",
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
    script = "${path.root}/../provision.d/030-tcc-ui-automation.sh"
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

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

source "tart-cli" "trybox" {
  vm_name            = var.vm_name
  recovery           = true
  recovery_partition = "keep"
  communicator       = "none"

  boot_command = [
    "<wait60s><right><right><enter>",
    "<wait30s><leftShiftOn><leftAltOn>T<leftAltOff><leftShiftOff>",
    "<wait10s>csrutil disable<enter>",
    "<wait10s>y<enter>",
    "<wait10s>admin<enter>",
    "<wait10s>admin<enter>",
    "<wait10s>halt<enter>",
  ]
}

build {
  sources = ["source.tart-cli.trybox"]
}

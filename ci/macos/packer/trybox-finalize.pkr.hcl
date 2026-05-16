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
  recovery_partition = "keep"
  ssh_username       = "admin"
  ssh_password       = "admin"
  ssh_timeout        = "10m"
}

build {
  sources = ["source.tart-cli.trybox"]

  provisioner "shell" {
    script = "${path.root}/../provision.d/030-tcc-ui-automation.sh"
  }

  provisioner "shell" {
    inline_shebang = "/bin/bash -e"
    inline = [
      "csrutil status",
    ]
  }
}

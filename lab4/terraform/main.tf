terraform {
  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
      version = "~> 0.9"
    }
  }
}

provider "libvirt" {
  uri = var.libvirt_uri
}

resource "libvirt_cloudinit_disk" "worker_cloudinit" {
  name = "worker-cloudinit.iso"
  user_data = templatefile("${path.module}/cloud-init/worker.yaml.tpl", {
    ansible_public_key = var.ansible_public_key
  })
  meta_data = ""
}

resource "libvirt_cloudinit_disk" "db_cloudinit" {
  name = "db-cloudinit.iso"
  user_data = templatefile("${path.module}/cloud-init/db.yaml.tpl", {
    ansible_public_key = var.ansible_public_key
  })
  meta_data = ""
}

resource "libvirt_volume" "worker" {
  name = "worker.qcow2"
  pool = var.storage_pool

  create = {
    content = {
      url = "file://${var.base_image_path}"
    }
  }
}

resource "libvirt_volume" "db" {
  name = "db.qcow2"
  pool = var.storage_pool

  create = {
    content = {
      url = "file://${var.base_image_path}"
    }
  }
}

resource "libvirt_domain" "worker" {
  name   = "worker"
  type   = "qemu"
  vcpu   = var.worker_vcpu
  memory = var.worker_memory_mb

  os = {
    type         = "hvm"
    type_arch    = var.arch
    type_machine = var.machine_type
    loader          = var.uefi_firmware_path
    loader_type     = "pflash"
    loader_readonly = "yes"
  }

  devices = {
    disks = [
      {
        device = "disk"
        driver = {
          name = "qemu"
          type = "qcow2"
        }
        source = {
          volume = {
            pool   = var.storage_pool
            volume = libvirt_volume.worker.name
          }
        }
        target = {
          dev = "vda"
          bus = "virtio"
        }
      },
      {
        device    = "cdrom"
        read_only = true
        source = {
          file = {
            file = libvirt_cloudinit_disk.worker_cloudinit.path
          }
        }
        target = {
          dev = "sda"
          bus = "sata"
        }
      }
    ]
    interfaces = [
      {
        source = {
          network = {
            network = var.network_name
          }
        }
        model = {
          type = "virtio"
        }
        wait_for_ip = {
          timeout = 120
        }
      }
    ]
  }
}

resource "libvirt_domain" "db" {
  name   = "db"
  type   = "qemu"
  vcpu   = var.db_vcpu
  memory = var.db_memory_mb

  os = {
    type         = "hvm"
    type_arch    = var.arch
    type_machine = var.machine_type
    loader          = var.uefi_firmware_path
    loader_type     = "pflash"
    loader_readonly = "yes"
  }

  devices = {
    disks = [
      {
        device = "disk"
        driver = {
          name = "qemu"
          type = "qcow2"
        }
        source = {
          volume = {
            pool   = var.storage_pool
            volume = libvirt_volume.db.name
          }
        }
        target = {
          dev = "vda"
          bus = "virtio"
        }
      },
      {
        device    = "cdrom"
        read_only = true
        source = {
          file = {
            file = libvirt_cloudinit_disk.db_cloudinit.path
          }
        }
        target = {
          dev = "sda"
          bus = "sata"
        }
      }
    ]
    interfaces = [
      {
        source = {
          network = {
            network = var.network_name
          }
        }
        model = {
          type = "virtio"
        }
        wait_for_ip = {
          timeout = 120
        }
      }
    ]
  }
}

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

# ---------- cloud-init disks ----------

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

# ---------- volumes via qemu-img (CoW on top of base image) ----------

resource "null_resource" "volumes" {
  triggers = {
    base_image = var.base_image_path
  }

  provisioner "local-exec" {
    command = <<-EOF
      set -e
      for vm in worker db; do
        dst="/var/lib/libvirt/images/$${vm}.qcow2"
        if [ ! -s "$dst" ]; then
          sudo qemu-img create -f qcow2 -b "${var.base_image_path}" -F qcow2 "$dst" ${var.disk_size_gb}G
        fi
        sudo chown libvirt-qemu:kvm "$dst"
        sudo chmod 640 "$dst"
      done
      sudo virsh pool-refresh ${var.storage_pool}
    EOF
  }
}

# ---------- worker VM ----------

resource "libvirt_domain" "worker" {
  depends_on = [null_resource.volumes]

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
    nv_ram = {
      nv_ram   = "/var/lib/libvirt/qemu/nvram/worker_VARS.fd"
      template = "/usr/share/AAVMF/AAVMF_VARS.fd"
    }
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
          file = {
            file = "/var/lib/libvirt/images/worker.qcow2"
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

# ---------- db VM ----------

resource "libvirt_domain" "db" {
  depends_on = [null_resource.volumes]

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
    nv_ram = {
      nv_ram   = "/var/lib/libvirt/qemu/nvram/db_VARS.fd"
      template = "/usr/share/AAVMF/AAVMF_VARS.fd"
    }
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
          file = {
            file = "/var/lib/libvirt/images/db.qcow2"
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

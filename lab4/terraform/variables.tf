variable "base_image_path" {
  description = "Path to the Ubuntu cloud image (qcow2)"
  type        = string
  default = "/Users/Ivan/libvirt-images/ubuntu-22.04-server-cloudimg-arm64.img"
}

variable "storage_pool" {
  description = "Libvirt storage pool name"
  type        = string
  default     = "default"
}

variable "network_name" {
  description = "Libvirt network name"
  type        = string
  default     = "default"
}

variable "arch" {
  description = "Guest CPU architecture"
  type        = string
  default     = "aarch64"
}

variable "machine_type" {
  description = "QEMU machine type (virt for ARM64, pc/q35 for x86_64)"
  type        = string
  default     = "virt"
}

variable "uefi_firmware_path" {
  description = "Path to UEFI firmware (required for ARM64 cloud images)"
  type        = string
  default     = "/opt/homebrew/share/qemu/edk2-aarch64-code.fd"
}

variable "worker_vcpu" {
  description = "vCPU count for worker VM"
  type        = number
  default     = 2
}

variable "worker_memory_mb" {
  description = "RAM in MB for worker VM"
  type        = number
  default     = 1024
}

variable "db_vcpu" {
  description = "vCPU count for db VM"
  type        = number
  default     = 2
}

variable "db_memory_mb" {
  description = "RAM in MB for db VM"
  type        = number
  default     = 1024
}

variable "libvirt_uri" {
  description = "Libvirt connection URI. On macOS: qemu+unix:///session?socket=$TMPDIR/libvirt/virtqemud-sock"
  type        = string
  default     = "qemu:///session"
}

variable "ansible_public_key" {
  description = "SSH public key for the ansible user (injected via cloud-init)"
  type        = string
}

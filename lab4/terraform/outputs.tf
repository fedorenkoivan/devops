output "worker_domain_id" {
  description = "Worker VM domain ID"
  value       = libvirt_domain.worker.id
}

output "worker_domain_uuid" {
  description = "Worker VM UUID"
  value       = libvirt_domain.worker.uuid
}

output "db_domain_id" {
  description = "DB VM domain ID"
  value       = libvirt_domain.db.id
}

output "db_domain_uuid" {
  description = "DB VM UUID"
  value       = libvirt_domain.db.uuid
}

output "get_ips_command" {
  description = "Commands to retrieve VM IPs after boot"
  value       = <<-EOT
    Run after terraform apply:
      virsh -c qemu:///session domifaddr worker
      virsh -c qemu:///session domifaddr db
    Then update ansible/inventory.ini with the IPs.
  EOT
}

users:
  - name: ansible
    groups: sudo
    shell: /bin/bash
    sudo: "ALL=(ALL) NOPASSWD:ALL"
    ssh_authorized_keys:
      - ${ansible_public_key}

package_update: true
package_upgrade: false
packages:
  - qemu-guest-agent

runcmd:
  - systemctl enable --now qemu-guest-agent

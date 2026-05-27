# Лабораторна робота №4 — IaC: Terraform + Ansible

## Варіант (N=26)

- **V2=1** — конфігурація через CLI args, БД: MariaDB
- **V3=3** — Simple Inventory
- **V5=2** — порт застосунку: **5200**

## Архітектура

```
client → VM1 (worker): nginx:80 → mywebapp:127.0.0.1:5200
                                        ↓
                              VM2 (db): MariaDB:<DB_IP>:3306
```

- **VM1 (worker)**: nginx reverse proxy + Go web application
- **VM2 (db)**: MariaDB, прослуховує лише свій внутрішній IP
- MariaDB доступна лише з worker та localhost; доступ ззовні заблокований через iptables

## Передумови

На хості (не у ВМ):
- [Terraform](https://developer.hashicorp.com/terraform/install) ≥ 1.6
- [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/) ≥ 2.14
- `ansible-galaxy collection install community.mysql`
- KVM/QEMU + libvirt (`sudo apt install qemu-kvm libvirt-daemon-system`)
- Ubuntu 22.04 cloud image: завантажити і покласти в libvirt storage pool

  ```bash
  wget https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
  sudo cp jammy-server-cloudimg-amd64.img /var/lib/libvirt/images/ubuntu-22.04-server-cloudimg-amd64.img
  ```

- SSH ключ для ansible user (згенерувати якщо немає):
  ```bash
  ssh-keygen -t ed25519 -f ~/.ssh/ansible_key -N ""
  ```

## Крок 1 — Provisioning (Terraform)

```bash
cd lab4/terraform

# Ініціалізація провайдера
terraform init

# Перегляд плану
terraform plan \
  -var="ansible_public_key=$(cat ~/.ssh/ansible_key.pub)"

# Застосування (підніме 2 ВМ)
terraform apply \
  -var="ansible_public_key=$(cat ~/.ssh/ansible_key.pub)"
```

Після завершення Terraform виведе IP-адреси ВМ та готовий snippet для inventory:

```
worker_ip = "192.168.122.X"
db_ip     = "192.168.122.Y"
```

## Крок 2 — Оновити inventory

Замінити `WORKER_IP` та `DB_IP` в `ansible/inventory.ini` на реальні IP:

```ini
[workers]
worker ansible_host=192.168.122.X

[db]
db ansible_host=192.168.122.Y
```

Або автоматично:

```bash
terraform output -raw ansible_inventory > ../ansible/inventory.ini
```

## Крок 3 — Configuration Management (Ansible)

```bash
cd lab4/ansible

# Перевірити з'єднання
ansible all -i inventory.ini -m ping \
  --private-key ~/.ssh/ansible_key

# Запустити playbook
ansible-playbook -i inventory.ini playbook.yml \
  --private-key ~/.ssh/ansible_key
```

Повторний запуск ідемпотентний — якщо конфігурація вже відповідає цільовому стану, змін не буде.

## Крок 4 — Перевірка

З хосту або будь-якого клієнта:

```bash
# Головна сторінка
curl -i http://<WORKER_IP>/

# API
curl -i -H 'Accept: application/json' http://<WORKER_IP>/items
curl -i -X POST http://<WORKER_IP>/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"Laptop","quantity":2}'

# Health (через nginx — мають бути 404)
curl -i http://<WORKER_IP>/health/alive
curl -i http://<WORKER_IP>/health/ready

# Health напряму до застосунку (мають бути 200)
ssh -i ~/.ssh/ansible_key ansible@<WORKER_IP> \
  "curl -s http://127.0.0.1:5200/health/alive"
ssh -i ~/.ssh/ansible_key ansible@<WORKER_IP> \
  "curl -s http://127.0.0.1:5200/health/ready"

# Перевірка, що MariaDB ззовні недоступна
nc -zv <DB_IP> 3306   # має не з'єднатись (connection refused / timeout)
```

### Перевірка готовності (ready endpoint)

Зупинити MariaDB на db VM:
```bash
ssh -i ~/.ssh/ansible_key ansible@<DB_IP> "sudo systemctl stop mariadb"
curl -i http://127.0.0.1:5200/health/ready   # → 500
sudo systemctl start mariadb
curl -i http://127.0.0.1:5200/health/ready   # → 200
```

## Знесення інфраструктури

```bash
cd lab4/terraform
terraform destroy \
  -var="ansible_public_key=$(cat ~/.ssh/ansible_key.pub)"
```

## Структура проєкту

```
lab4/
├── terraform/
│   ├── main.tf            # Libvirt provider, VMs, volumes, cloud-init
│   ├── variables.tf       # Input variables
│   ├── outputs.tf         # VM IPs + inventory snippet
│   └── cloud-init/
│       ├── worker.yaml.tpl  # cloud-init for worker VM
│       └── db.yaml.tpl      # cloud-init for db VM
└── ansible/
    ├── inventory.ini      # Static inventory (fill in IPs after terraform apply)
    ├── playbook.yml       # Main playbook
    ├── group_vars/
    │   ├── all.yml        # Shared variables
    │   ├── workers.yml    # Worker-specific variables
    │   └── db.yml         # DB-specific variables
    └── roles/
        ├── common/        # Users (teacher, student), gradebook — all VMs
        ├── worker/        # mywebapp binary, nginx, systemd, operator user
        └── db/            # MariaDB, bind address, DB user, firewall
```

## Користувачі в системі

| Користувач | ВМ           | Пароль    | Права                                              |
|------------|-------------|-----------|---------------------------------------------------|
| ansible    | всі         | —         | Sudo без пароля; SSH-ключ через cloud-init        |
| teacher    | всі         | 12345678  | Sudo з паролем                                    |
| student    | всі         | —         | Звичайний; `/home/student/gradebook` з N=26       |
| app        | worker      | —         | Системний, без логіну; запускає mywebapp          |
| operator   | worker      | 12345678  | Sudo лише: start/stop/restart/status mywebapp + reload nginx |

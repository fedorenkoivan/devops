#!/usr/bin/env bash
# setup-target.sh
# Prepares an Ubuntu 24.04 VM (target node) to run mywebapp in a Docker container.
# Run as root: sudo bash setup-target.sh
set -euo pipefail

APP_USER="mywebapp"
APP_CONFIG_DIR="/etc/mywebapp"
SYSTEMD_UNIT_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/systemd/mywebapp-container.service"

require_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    echo "must be run as root" >&2
    exit 1
  fi
}

install_packages() {
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    ca-certificates curl gnupg lsb-release nginx mariadb-server
}

install_docker() {
  if command -v docker &>/dev/null; then
    echo "docker already installed"
    return
  fi
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    docker-ce docker-ce-cli containerd.io docker-buildx-plugin
  systemctl enable --now docker
}

create_users() {
  for user in student teacher; do
    id -u "$user" &>/dev/null && continue
    useradd -m -s /bin/bash "$user"
    echo "${user}:12345678" | chpasswd
    chage -d 0 "$user" || true
  done
  id -u operator &>/dev/null || useradd -m -s /bin/bash operator
  echo "operator:12345678" | chpasswd
  usermod -aG sudo student
  usermod -aG sudo teacher
  usermod -aG docker operator
  id -u "$APP_USER" &>/dev/null || \
    useradd --system --no-create-home --shell /usr/sbin/nologin "$APP_USER"
}

configure_mariadb() {
  local cnf="/etc/mysql/mariadb.conf.d/50-server.cnf"
  if grep -q '^bind-address' "$cnf"; then
    sed -i 's/^bind-address\s*=\s*.*/bind-address = 127.0.0.1/' "$cnf"
  else
    echo 'bind-address = 127.0.0.1' >> "$cnf"
  fi
  systemctl enable --now mariadb
  mysql -u root <<'SQL'
CREATE DATABASE IF NOT EXISTS mywebapp CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'mywebapp'@'localhost' IDENTIFIED BY 'mywebapp';
GRANT ALL PRIVILEGES ON mywebapp.* TO 'mywebapp'@'localhost';
FLUSH PRIVILEGES;
SQL
}

install_app_config() {
  mkdir -p "$APP_CONFIG_DIR"
  if [[ ! -f "$APP_CONFIG_DIR/env" ]]; then
    cat > "$APP_CONFIG_DIR/env" <<'EOF'
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=mywebapp
DB_PASS=mywebapp
DB_NAME=mywebapp
APP_PORT=5200
EOF
    chmod 0600 "$APP_CONFIG_DIR/env"
  fi
}

install_systemd_unit() {
  local unit_dest="/etc/systemd/system/mywebapp-container.service"
  install -o root -g root -m 0644 "$SYSTEMD_UNIT_SRC" "$unit_dest"
  systemctl daemon-reload
  systemctl enable mywebapp-container
}

install_nginx() {
  local repo_lab3
  repo_lab3="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  install -o root -g root -m 0644 \
    "$repo_lab3/../deploy/nginx/mywebapp.conf" \
    /etc/nginx/sites-available/mywebapp.conf
  ln -sf /etc/nginx/sites-available/mywebapp.conf /etc/nginx/sites-enabled/mywebapp.conf
  rm -f /etc/nginx/sites-enabled/default || true
  nginx -t
  systemctl enable --now nginx
  systemctl reload nginx
}

install_sudoers() {
  local repo_lab3
  repo_lab3="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  install -o root -g root -m 0440 \
    "$repo_lab3/sudoers/operator" \
    /etc/sudoers.d/operator
}

write_gradebook() {
  install -o student -g student -m 0644 /dev/null /home/student/gradebook
  echo "26" > /home/student/gradebook
  chown student:student /home/student/gradebook
}

main() {
  require_root
  install_packages
  install_docker
  create_users
  write_gradebook
  configure_mariadb
  install_app_config
  install_systemd_unit
  install_sudoers
  install_nginx
  echo "Target node setup complete."
}

main "$@"

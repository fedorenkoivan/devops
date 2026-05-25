#!/usr/bin/env bash
set -euo pipefail

# Lab 1 deploy script for Ubuntu
# - Installs packages
# - Creates users
# - Configures MariaDB (local-only)
# - Installs mywebapp binary + migrations
# - Installs systemd socket/service
# - Installs nginx reverse-proxy config
# - Writes /home/student/gradebook with N=26
#
# Usage: sudo ./scripts/deploy.sh

N="26"
APP_USER="mywebapp"
APP_GROUP="mywebapp"
APP_HOME="/var/lib/mywebapp"
APP_CONFIG_DIR="/etc/mywebapp"
APP_ENV_FILE="$APP_CONFIG_DIR/env"
APP_BIN="/usr/local/bin/mywebapp"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    echo "must be run as root" >&2
    exit 1
  fi
}

install_packages() {
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    ca-certificates curl git nginx mariadb-server \
    golang-go build-essential
}

create_user() {
  local name="$1"
  id -u "$name" >/dev/null 2>&1 && return 0
  if getent group "$name" >/dev/null 2>&1; then
    useradd -m -s /bin/bash -g "$name" "$name"
  else
    useradd -m -s /bin/bash "$name"
  fi
}

create_users() {
  # student + teacher (sudo)
  create_user student
  create_user teacher
  create_user operator

  echo "student:12345678" | chpasswd
  echo "teacher:12345678" | chpasswd
  echo "operator:12345678" | chpasswd

  chage -d 0 student || true
  chage -d 0 teacher || true
  chage -d 0 operator || true

  usermod -aG sudo student
  usermod -aG sudo teacher

  # system user for app
  id -u "$APP_USER" >/dev/null 2>&1 || useradd --system --home "$APP_HOME" --create-home --shell /usr/sbin/nologin "$APP_USER"
}

write_gradebook() {
  install -o student -g student -m 0644 /dev/null /home/student/gradebook
  echo "$N" > /home/student/gradebook
  chown student:student /home/student/gradebook
}

configure_mariadb() {
  # Bind only to localhost
  local cnf="/etc/mysql/mariadb.conf.d/50-server.cnf"
  if grep -q '^bind-address' "$cnf"; then
    sed -i 's/^bind-address\s*=\s*.*/bind-address = 127.0.0.1/' "$cnf"
  else
    echo 'bind-address = 127.0.0.1' >> "$cnf"
  fi

  systemctl enable --now mariadb

  # Create DB + user limited to localhost
  mysql -u root <<SQL
CREATE DATABASE IF NOT EXISTS mywebapp CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'mywebapp'@'localhost' IDENTIFIED BY 'mywebapp';
GRANT ALL PRIVILEGES ON mywebapp.* TO 'mywebapp'@'localhost';
FLUSH PRIVILEGES;
SQL
}

build_and_install_app() {
  mkdir -p "$APP_HOME"
  chown -R "$APP_USER":"$APP_USER" "$APP_HOME"

  # build
  (cd "$REPO_ROOT" && /usr/bin/env GOPATH=/root/go go mod tidy)
  (cd "$REPO_ROOT" && /usr/bin/env GOPATH=/root/go go build -o /tmp/mywebapp ./cmd/mywebapp)

  install -o root -g root -m 0755 /tmp/mywebapp "$APP_BIN"

  # install migrations
  rm -rf "$APP_HOME/migrations"
  cp -a "$REPO_ROOT/migrations" "$APP_HOME/migrations"
  chown -R "$APP_USER":"$APP_USER" "$APP_HOME/migrations"
}

install_config() {
  mkdir -p "$APP_CONFIG_DIR"
  if [[ ! -f "$APP_ENV_FILE" ]]; then
    cat > "$APP_ENV_FILE" <<EOF
# root-owned secrets/config for systemd unit
DB_USER=mywebapp
DB_PASS=mywebapp
DB_NAME=mywebapp
EOF
    chmod 0600 "$APP_ENV_FILE"
  fi
}

install_systemd_units() {
  install -o root -g root -m 0644 "$REPO_ROOT/deploy/systemd/mywebapp.socket" /etc/systemd/system/mywebapp.socket
  install -o root -g root -m 0644 "$REPO_ROOT/deploy/systemd/mywebapp.service" /etc/systemd/system/mywebapp.service

  systemctl daemon-reload
  systemctl enable --now mywebapp.socket
  systemctl reset-failed mywebapp.service 2>/dev/null || true
  systemctl restart mywebapp.service || true
}

install_nginx() {
  install -o root -g root -m 0644 "$REPO_ROOT/deploy/nginx/mywebapp.conf" /etc/nginx/sites-available/mywebapp.conf
  ln -sf /etc/nginx/sites-available/mywebapp.conf /etc/nginx/sites-enabled/mywebapp.conf
  rm -f /etc/nginx/sites-enabled/default || true
  nginx -t
  systemctl enable --now nginx
  systemctl reload nginx
}

install_sudoers_operator() {
  install -o root -g root -m 0440 "$REPO_ROOT/deploy/sudoers/operator" /etc/sudoers.d/operator
}

disable_default_user() {
  # Best-effort: common cloud images use ubuntu user
  if id -u ubuntu >/dev/null 2>&1; then
    usermod -L ubuntu || true
  fi
}

main() {
  require_root
  install_packages
  create_users
  write_gradebook
  configure_mariadb
  build_and_install_app
  install_config
  install_sudoers_operator
  install_nginx
  install_systemd_units
  disable_default_user

  echo "deploy complete"
}

main "$@"

# Лабораторна робота №1 — mywebapp (Go + GORM + Goose)

## Варіант (N=26)

Формули:
- `V2 = (N % 2) + 1`
- `V3 = (N % 3) + 1`
- `V5 = (N % 5) + 1`

Розрахунок для `N=26`:
- `V2 = (26 % 2) + 1 = 1`
- `V3 = (26 % 3) + 1 = 3`
- `V5 = (26 % 5) + 1 = 2`

- **V3=3** = Simple Inventory
- **V2=1** = конфігурація через аргументи командного рядка, БД: MariaDB
- **V5=2** = порт застосунку: **5200**

## Як потикать?
Скачати Ubuntu Server звідси(https://ubuntu.com/download/server/arm)
Створити файл віртуального диску `qemu-img create -f qcow2 ubuntu-server.qcow2 20G`
Інсталювати ВМ-ку: `sudo ./scripts/install-vm.sh`
Запустити ВМ-ку: `sudo ./scripts/run-vm.sh`
Підключитися по ссш: `ssh -p 2222 username@localhost` (і ввести пароль)
Склонувати цю репку: `git clone https://github.com/fedorenkoivan/devops.git`
Перейти у папку, дати права на виконання файлику і задеплоїть: 
`cd devops`
`chmod +x ./scripts/deploy.sh`
`sudo ./scripts/deploy.sh`

`curl -i http://127.0.0.1:5200/health/alive`
`curl -i http://127.0.0.1:5200/health/ready`

`curl -i -H 'Accept: text/html' http://127.0.0.1:5200/`

`curl -i -H 'Accept: application/json' http://127.0.0.1:5200/items`

`curl -i -X POST http://127.0.0.1:5200/items \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d '{"name":"Laptop","quantity":2}'`

`curl -i -H 'Accept: text/html' http://127.0.0.1:5200/items`
`curl -i -H 'Accept: application/json' http://127.0.0.1:5200/items/1`
`curl -i -H 'Accept: text/html' http://127.0.0.1:5200/items/1` 

`curl -i http://127.0.0.1/` -> 200
`curl -i http://127.0.0.1/items` -> 200
`curl -i http://127.0.0.1/health/alive` -> 404 Not Found
`curl -i http://127.0.0.1/health/ready`. -> 404 Not Found

Зупинка бд
`sudo systemctl stop mariadb`

`curl -i http://127.0.0.1:5200/health/ready` -> 500 Internal Server Error

Піднімемо бд
`sudo systemctl start mariadb`

`curl -i http://127.0.0.1:5200/health/ready` -> 200 OK

Operator
`sudo -l` -> маєш всі права

## Архітектура

`client -> nginx (reverse proxy) -> mywebapp -> MariaDB`

Мережеві обмеження:
- nginx: `0.0.0.0:80`
- mywebapp: `127.0.0.1:5200`
- MariaDB: `127.0.0.1:3306`

## Web application

### Health endpoints
- `GET /health/alive` → завжди `200 OK` з тілом `OK`
- `GET /health/ready` → `200 OK` з тілом `OK`, якщо сервіс має підключення до БД, інакше `500`

> Примітка: nginx назовні **не проксить** `/health/*`.

### Business endpoints (Simple Inventory)
- `GET /items`
  - `Accept: application/json` → JSON список (id, name)
  - `Accept: text/html` → HTML-таблиця
- `POST /items` (підтримує JSON або form-data)
  - body JSON: `{ "name": "Laptop", "quantity": 2 }`
  - або form: `name=...&quantity=...`
- `GET /items/{id}`
  - JSON або HTML залежно від `Accept`

### Root endpoint
- `GET /` → тільки `text/html`, повертає список бізнес-ендпоінтів.

## Конфігурація (V2=1)

Застосунок конфігурується через CLI args.

Основні аргументи:
- `serve`: `-listen 127.0.0.1:5200` або `-socket-activation`
- DB: `-db-host`, `-db-port`, `-db-user`, `-db-pass`, `-db-name`, `-db-params`

## Міграції БД (Goose)

Міграції лежать у `migrations/`.

Запуск міграцій:
- `mywebapp migrate -migrations migrations -db-host ... -db-user ... -db-pass ... -db-name ...`

У systemd міграції виконуються перед стартом сервера через `ExecStartPre`.

## Розгортання на VM (Ubuntu)

### Автоматизація

Точка входу: `scripts/deploy.sh`.

Скрипт:
- встановлює пакети (nginx, mariadb, go)
- створює користувачів `student`, `teacher`, `operator`, `mywebapp`
- створює БД `mywebapp` і користувача MariaDB (localhost-only)
- збирає і ставить `/usr/local/bin/mywebapp`
- копіює міграції в `/var/lib/mywebapp/migrations`
- ставить systemd `mywebapp.service` + `mywebapp.socket` (socket activation)
- налаштовує nginx reverse proxy
- створює `/home/student/gradebook` з числом `26`
- блокує дефолтного користувача `ubuntu` (якщо існує)

### Systemd

Файли:
- `deploy/systemd/mywebapp.service` → `/etc/systemd/system/mywebapp.service`
- `deploy/systemd/mywebapp.socket` → `/etc/systemd/system/mywebapp.socket`

### Nginx

Файл: `deploy/nginx/mywebapp.conf` → `/etc/nginx/sites-available/mywebapp.conf`.

Назовні віддається:
- `/`
- `/items` і `/items/{id}`

## Як тестував (мінімальний сценарій)

1) Перевірка nginx/root:
- відкрити `http://<vm-ip>/` має показати HTML список ендпоінтів

2) JSON список:
- запит на `GET /items` з `Accept: application/json`

3) Створення item:
- `POST /items` (json або form)

4) HTML список:
- `GET /items` з `Accept: text/html` → таблиця

5) Readiness:
- зупинити mysql/mariadb → `/health/ready` має давати `500`
- підняти mysql/mariadb → `/health/ready` має давати `200 OK`

---

## Запуск через Docker Compose (Лабораторна №2)

### Передумови

- [Docker](https://docs.docker.com/get-docker/) та Docker Compose (входить у Docker Desktop)

### Швидкий старт

```bash
# 1. Скопіювати файл змінних середовища та встановити паролі
cp .env.example .env

docker compose up -d
```

Доступно для тикання`http://localhost`.

### Архітектура (Docker)

```
client → nginx:80 → app:5200 → db:3306 (MariaDB)
```

Всі три сервіси ізольовані у мережі `mywebapp-net`. Назовні відкритий тільки порт `80` nginx.

### Сервіси

| Сервіс | Image | Призначення |
|---|---|---|
| `db` | `mariadb:11` | База даних, дані у volume `db-data` |
| `app` | build з `Dockerfile` | Go-застосунок, запускає міграції при старті |
| `nginx` | `nginx:alpine` | Reverse proxy, приймає зовнішній трафік |

### Персистентність даних

Дані MariaDB зберігаються у named volume `db-data`. Volume переживає:
- перезапуск контейнерів (`docker compose restart`)
- видалення контейнерів (`docker compose down`)
- перезавантаження системи

Видалення даних — тільки явно: `docker compose down -v`

### Зупинка

```bash
# Зупинити, зберігши дані
docker compose down

# Зупинити і видалити дані БД
docker compose down -v
```

### Перевірка

```bash
# Список items (JSON)
curl -i http://localhost/items -H 'Accept: application/json'

# Створення item
curl -i -X POST http://localhost/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"Laptop","quantity":2}'

# Health endpoints заблоковані nginx (повертають 404)
curl -i http://localhost/health/alive
curl -i http://localhost/health/ready
```
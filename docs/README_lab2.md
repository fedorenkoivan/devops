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
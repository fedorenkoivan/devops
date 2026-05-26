## Dockerfile-и

### Go (FizzBuzz)

- [Dockerfile](./dockerfiles/golang/Dockerfile) — базовий однорівневий образ на `golang:1.21-bookworm`. Збирає бінарник та запускає його безпосередньо; простий у налагодженні, але фінальний образ містить весь Go-тулчейн.

- [Dockerfile.distroless](./dockerfiles/golang/Dockerfile.distroless) — multi-stage build: збірка у `golang:1.21-bookworm`, фінальний образ `gcr.io/distroless/static-debian12`. Містить мінімальний набір системних бібліотек без шелу, що зменшує розмір і поверхню атаки.

- [Dockerfile.scratch](./dockerfiles/golang/Dockerfile.scratch) — multi-stage build із фінальним образом `scratch`. Абсолютно порожній образ — лише скомпільований статичний бінарник, найменший можливий розмір.

### Python (Spaceship)

- [Dockerfile](./dockerfiles/python/Dockerfile) — базовий образ `python:3.13-bookworm`, копіює весь проєкт і встановлює залежності. Найпростіший варіант, великий за розміром.

- [Dockerfile.optimized](./dockerfiles/python/Dockerfile.optimized) — теж `python:3.13-bookworm`, але копіює лише необхідні директорії (`build/`, `spaceship/`) замість усього контексту, що прискорює збірку і зменшує образ.

- [Dockerfile.alpine](./dockerfiles/python/Dockerfile.alpine) — базується на `python:3.13-alpine`; встановлює `gcc`, `musl-dev`, `libffi-dev` для компіляції C-залежностей. Менший базовий образ ніж Debian, але потребує build-інструментів.

- [Dockerfile.alpine-numpy](./dockerfiles/python/Dockerfile.alpine-numpy) — аналогічний alpine-варіант, але додатково встановлює `numpy` з `openblas-dev` та `g++` для компіляції числових бібліотек.

- [Dockerfile.debian-numpy](./dockerfiles/python/Dockerfile.debian-numpy) — `python:3.13-bookworm` з `numpy`; використовує Debian замість Alpine, щоб уникнути проблем із сумісністю бінарних колес numpy.

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
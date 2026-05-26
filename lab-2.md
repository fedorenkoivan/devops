# Лабораторна робота №2 — Контейнеризація

---

## 1. Python Application — Dockerfile без оптимізації

Є `Dockerfile` (naive), де весь код копіюється до встановлення залежностей:

```dockerfile
FROM python:3.13-bookworm
WORKDIR /app
COPY . .
RUN pip install --no-cache-dir fastapi "pydantic>=2.0" pydantic-settings starlette "uvicorn[standard]"
EXPOSE 8000
CMD ["uvicorn", "spaceship.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

### Команди

```bash
# Перша збірка (без кешу)
time docker build --no-cache -f Dockerfile -t spaceship:naive .

# Зміна коду (додав ім'я у build/index.html)
# Повторна збірка
time docker build -f Dockerfile -t spaceship:naive-rebuild .
```

### Результати

| Збірка | Час | Розмір |
|---|---|---|
| Перша (без кешу) | ~7s | 1.54 GB |
| Після зміни коду | ~7s | 1.54 GB |

Себто при кожній зміні коду `COPY . .` інвалідує кеш → `pip install` запускається заново щоразу.

---

## 2. Python Application — Оптимізований Dockerfile (шари)

Переписав `Dockerfile.optimized` — спочатку копіюємо тільки файл залежностей, встановлюємо їх, потім копіюємо код:

```dockerfile
FROM python:3.13-bookworm
WORKDIR /app
COPY requirements/backend.in requirements/backend.in
RUN pip install --no-cache-dir fastapi "pydantic>=2.0" pydantic-settings starlette "uvicorn[standard]"
COPY build/ build/
COPY spaceship/ spaceship/
EXPOSE 8000
CMD ["uvicorn", "spaceship.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

### Команди

```bash
# Перша збірка
time docker build --no-cache -f Dockerfile.optimized -t spaceship:optimized .

# Зміна коду в spaceship/app.py, повторна збірка
time docker build -f Dockerfile.optimized -t spaceship:optimized-rebuild .
```

### Результати

| Збірка | Час | Розмір |
|---|---|---|
| Перша (без кешу) | ~7s | 1.54 GB |
| Після зміни коду | **~0.8s** | 1.54 GB |

Завдяки правильному порядку шарів `pip install` береться з кешу при зміні коду - rebuild прискорюється з 7s до 0.8s. Різниця стає ще суттєвішою з важкими залежностями (numpy, torch тощо). Правило: змінюється рідко - вгору, змінюється часто - вниз.

---

## 3. Python Application — Alpine базовий образ

`Dockerfile.alpine` на основі `python:3.13-alpine`. Alpine потребує системних пакетів для компіляції деяких Python-бібліотек:

```dockerfile
FROM python:3.13-alpine
WORKDIR /app
RUN apk add --no-cache gcc musl-dev libffi-dev
COPY requirements/backend.in requirements/backend.in
RUN pip install --no-cache-dir fastapi "pydantic>=2.0" pydantic-settings starlette "uvicorn[standard]"
COPY build/ build/
COPY spaceship/ spaceship/
EXPOSE 8000
CMD ["uvicorn", "spaceship.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

### Команди

```bash
time docker build --no-cache -f Dockerfile.alpine -t spaceship:alpine .
docker images --format "{{.Repository}}:{{.Tag}}\t{{.Size}}" | grep spaceship
```

### Результати

| Базовий образ | Час збірки | Розмір |
|---|---|---|
| `python:3.13-bookworm` (Debian) | ~7s | 1.54 GB |
| `python:3.13-alpine` | ~16s | **391 MB** |

Alpine майже в 4 рази менший (391 MB проти 1.54 GB). Але збірка довша — потрібно скачати системні залежності та скомпілювати деякі пакети. Для production з фіксованими залежностями це виправдано.

---

## 4. Python Application — numpy + matrix endpoint

Додав `numpy` до `requirements/backend.in` та новий endpoint в `spaceship/routers/api.py`:

```python
@router.get('/matrix')
def matrix_multiply() -> dict:
    matrix_a = np.random.randint(0, 100, (10, 10))
    matrix_b = np.random.randint(0, 100, (10, 10))
    product = matrix_a @ matrix_b
    return {
        'matrix_a': matrix_a.tolist(),
        'matrix_b': matrix_b.tolist(),
        'product': product.tolist(),
    }
```

Побудував два образи — `Dockerfile.debian-numpy` та `Dockerfile.alpine-numpy`.

### Команди

```bash
time docker build --no-cache -f Dockerfile.debian-numpy -t spaceship:debian-numpy .
time docker build --no-cache -f Dockerfile.alpine-numpy -t spaceship:alpine-numpy .
```

### Результати

| Образ | Час збірки | Розмір |
|---|---|---|
| `spaceship:debian-numpy` | ~10s | 1.63 GB |
| `spaceship:alpine-numpy` | ~19s | 670 MB |

**Спостереження:** numpy на Alpine займає майже вдвічі більше часу (~19s проти ~10s) — немає prebuilt wheel для musl libc, тому pip компілює numpy з вихідників (потрібні gcc, openblas-dev, g++). Зате фінальний образ менший: 670 MB проти 1.63 GB.

**Висновок:** Alpine з numpy — розумний компроміс для production-образів, де важливий розмір. Для dev-середовища де збірка відбувається часто — Debian швидший.

---

## 5. Musl (Alpine) vs glibc (Debian/Ubuntu) — DNS

Перевірив поведінку DNS-резолвінгу з кастомним доменом та `--dns-search` на Alpine (musl) та Ubuntu (glibc).

### Команди

```bash
docker network create dns-lab

docker run --rm -d --name dns-server --network dns-lab \
  alpine sh -c "apk add dnsmasq && \
  echo 'address=/myservice.internal.corp/10.0.0.50' > /etc/dnsmasq.conf && \
  dnsmasq -k --log-queries --log-facility=-"

DNS_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' dns-server)

docker run --rm --network dns-lab \
  --dns=$DNS_IP \
  --dns-search="corp" \
  ubuntu:latest getent hosts myservice.internal

docker run --rm --network dns-lab \
  --dns=$DNS_IP \
  --dns-search="corp" \
  alpine:latest getent hosts myservice.internal

docker logs dns-server

docker stop dns-server && docker network rm dns-lab
```

### Результати

**Ubuntu (glibc):**
```
10.0.0.50       myservice.internal.corp
```

**Alpine (musl):**
```
(порожній вивід, exit code 2)
```

**Логи DNS-сервера:**
```
# Ubuntu зробив ДВА запити:
query[A] myservice.internal → NXDOMAIN
query[A] myservice.internal.corp → 10.0.0.50  ← search domain застосовано

# Alpine зробив ОДИН запит:
query[A] myservice.internal → NXDOMAIN
# search domain не застосовано взагалі
```

### Аналіз

Різниця пояснюється різним трактуванням `ndots` та search-доменів:

- **glibc** (Ubuntu): значення `ndots=5` за замовчуванням. `myservice.internal` має 1 крапку < 5 → спочатку пробує додати search-домени. `myservice.internal` + `.corp` = `myservice.internal.corp` → знаходить відповідь.
- **musl** (Alpine): будь-яка назва, що містить крапку, трактується як абсолютний FQDN. `myservice.internal` має крапку → DNS-запит йде без модифікацій → NXDOMAIN.

### Висновки та ризики

Така поведінка може призводити до:

1. **Зламаного service discovery** в Kubernetes або Docker Compose, якщо сервіси розраховані на пошук через search-домени. Alpine-контейнер не знайде `db` як `db.default.svc.cluster.local`, хоча Ubuntu-контейнер знайде.
2. **Непослідовної поведінки** — той самий код у Debian-контейнері та Alpine-контейнері дає різні результати DNS-резолвінгу.
3. **Складного дебагу** — різниця виявляється лише при специфічних конфігураціях DNS і може бути непомітна в більшості сценаріїв.

**Рекомендація:** якщо в проєкті використовується Alpine та custom DNS з search-доменами — варто звертатися до сервісів по повному FQDN (з крапкою в кінці) або явно додавати `options ndots:5` в `/etc/resolv.conf` образу.

---

## 6. Golang Application — базова збірка

Додав `Dockerfile` для Go-проєкту. Весь Go toolchain залишається в фінальному образі:

```dockerfile
FROM golang:1.21-bookworm
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o build/fizzbuzz
EXPOSE 8080
CMD ["./build/fizzbuzz", "serve"]
```

### Команди

```bash
time docker build --no-cache -f Dockerfile -t fizzbuzz:basic .
docker images --format "{{.Repository}}:{{.Tag}}\t{{.Size}}" | grep fizzbuzz
docker run --rm fizzbuzz:basic find / -not -path '/proc/*' -type f 2>/dev/null | wc -l
```

### Результати

| Метрика | Значення |
|---|---|
| Час збірки | ~22s |
| Розмір образу | 1.32 GB |
| Кількість файлів | ~45,000 |

**Аналіз вмісту:** 45,000 файлів у образі, хоча для запуску Go-бінарника потрібно лише сам бінарник та директорія `templates/`. Весь Go toolchain (`go`, `gofmt`, компілятор, stdlib вихідники) залишається в образі — це зайві ~1.3 GB.

---

## 7. Golang — Multi-stage build з FROM scratch

Двоетапна збірка: перший етап — компіляція, другий — абсолютно порожній образ (`FROM scratch`):

```dockerfile
FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o fizzbuzz .

FROM scratch
WORKDIR /app
COPY --from=builder /app/fizzbuzz .
COPY --from=builder /app/templates/ templates/
EXPOSE 8080
ENTRYPOINT ["./fizzbuzz", "serve"]
```

`CGO_ENABLED=0` — статичне компонування, не потребує системних `.so` бібліотек.

### Команди

```bash
time docker build --no-cache -f Dockerfile.scratch -t fizzbuzz:scratch .
docker run --rm --entrypoint sh fizzbuzz:scratch
docker run --rm fizzbuzz:scratch /app/fizzbuzz --help
```

### Результати

| Метрика | Значення |
|---|---|
| Час збірки | ~7s |
| Розмір образу | **15.4 MB** |

**Аналіз вмісту:** лише 2 елементи — бінарний файл `fizzbuzz` та директорія `templates/`. Немає shell, немає пакетного менеджера, немає нічого зайвого.

**Незручності scratch:**
- `docker exec -it container sh` — неможливий (нема shell)
- Нема `ls`, `cat`, будь-яких утиліт для дебагу всередині
- Нема CA-сертифікатів - HTTPS-запити назовні зламаються
- Нема timezone data - час може бути некоректним

---

## 8. Golang — Multi-stage build з distroless

Замінив `FROM scratch` на `gcr.io/distroless/static-debian12` — образ від Google без shell, але з базовими бібліотеками:

```dockerfile
FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o fizzbuzz .

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/fizzbuzz .
COPY --from=builder /app/templates/ templates/
EXPOSE 8080
ENTRYPOINT ["./fizzbuzz", "serve"]
```

### Команди

```bash
time docker build --no-cache -f Dockerfile.distroless -t fizzbuzz:distroless .
docker run --rm --entrypoint sh fizzbuzz:distroless
```

### Результати

| Образ | Розмір | Shell | CA-certs | Timezone |
|---|---|---|---|---|
| `fizzbuzz:basic` | 1.32 GB | + | + | + |
| `fizzbuzz:scratch` | 15.4 MB | - | - | - |
| `fizzbuzz:distroless` | **21.6 MB** | - | + | + |

**Висновок:** distroless додає лише 6 MB відносно scratch, але дає CA-сертифікати та timezone data — мінімально необхідне для більшості production-сервісів. Shell відсутній (не можна `docker exec`), але це плюс з точки зору безпеки — зловмисник, що потрапив у контейнер, не має інструментів для подальших дій.

---

## Загальні висновки та рекомендації

### Порядок шарів у Dockerfile

Варто завжди писати інструкції від «рідко змінюється» до «часто змінюється»:
1. Системні залежності (`apt install`, `apk add`)
2. Залежності проєкту (`pip install`, `npm install`, `go mod download`)
3. Код застосунку (`COPY . .`)

Неправильний порядок може збільшити час CI/CD pipeline в рази.

### Alpine vs Debian

| | Alpine | Debian |
|---|---|---|
| Розмір | 4–10x менший | Більший |
| Час збірки з native-залежностями | Довший (компіляція) | Коротший (prebuilt wheels) |
| DNS з search-доменами | Ризик несумісності | Стандартна поведінка |
| Рекомендація | Прості застосунки без нативних залежностей | Застосунки з C-розширеннями (numpy, cryptography) |

### Multi-stage builds для Go (та інших компільованих мов)

Multi-stage build обов'язковий для production Go-образів:
- Базова збірка: 1.32 GB → distroless: 21.6 MB (зменшення у **61 разів**)
- Менший образ = швидший pull в Kubernetes, менша атакова поверхня
- `distroless` краще за `scratch` для більшості випадків: CA-certs і timezone без shell

### Musl vs glibc — критично для service discovery

Якщо інфраструктура використовує custom DNS domains та search-домени — **не використовуйте Alpine** без явного налаштування `ndots` або переходу на FQDN. Це особливо критично в Kubernetes-середовищах.

---

## Практична частина — Docker Compose

### Архітектура

```
client → nginx:80 → app:5200 → db:3306 (MariaDB)
```

Три сервіси в ізольованій мережі `mywebapp-net`. Назовні відкритий тільки порт `80`.

### Що зробив

**`Dockerfile`** — multi-stage build для Go-застосунку:
- Stage 1: `golang:1.22-bookworm` — компіляція з `CGO_ENABLED=0` (статичний бінарник)
- Stage 2: `alpine:3.20` — мінімальний образ із shell для entrypoint-скрипту

**`entrypoint.sh`** — запускає міграції (`mywebapp migrate`) перед стартом сервера (`mywebapp serve`). Пароль та хост БД прокидуються через змінні середовища.

**`deploy/nginx/mywebapp.docker.conf`** — nginx-конфіг адаптований під Docker: `proxy_pass http://app:5200` замість `127.0.0.1:5200`.

**`docker-compose.yml`** — оркестрація всіх трьох сервісів:
- `db` — MariaDB із healthcheck; `app` стартує тільки після `service_healthy`
- `app` — збирається з локального Dockerfile, env vars для підключення до БД
- `nginx` — монтує конфіг read-only, залежить від `app`
- `db-data` — named volume для персистентності даних
- `mywebapp-net` — ізольована bridge-мережа

### Команди

```bash
# Підготовка
cp .env.example .env
# відредагувати .env — встановити DB_ROOT_PASSWORD та DB_PASS

# Запуск
docker compose up -d

# Перевірка статусу
docker compose ps
docker compose logs app

# Тест
curl -i http://localhost/
curl -i http://localhost/items -H 'Accept: application/json'
curl -i -X POST http://localhost/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"Laptop","quantity":2}'

# Health через nginx — мають бути 404
curl -i http://localhost/health/alive
curl -i http://localhost/health/ready

# Зупинка (дані зберігаються)
docker compose down

# Зупинка з видаленням даних
docker compose down -v
```

### Результати

| Перевірка | Результат |
|---|---|
| `GET /` через nginx | 200 OK |
| `GET /items` (JSON) | 200 OK, повертає список |
| `POST /items` | 201 Created |
| `GET /health/alive` через nginx | 404 (заблоковано) |
| `GET /health/ready` через nginx | 404 (заблоковано) |
| Restart db-контейнера, дані після | збережені |
| `docker compose down` + `up` | дані збережені |

### Розмір образу застосунку

| Stage | Розмір |
|---|---|
| `golang:1.22-bookworm` (builder) | ~1.1 GB |
| `mywebapp:latest` (фінальний, alpine) | ~20 MB |

### Особливості та труднощі

**Міграції при старті:** застосунок не підтримує `depends_on` логіку сам по собі — вирішено через `entrypoint.sh`, який спочатку виконує `migrate`, потім `serve`. Альтернатива — окремий `migrate`-сервіс у compose, але entrypoint простіший.

**`depends_on` з healthcheck:** без `condition: service_healthy` застосунок намагався підключитися до БД до того, як MariaDB завершила ініціалізацію — отримували `dial tcp: connection refused`. Healthcheck вирішив проблему.

**listen address:** за замовчуванням застосунок слухає `127.0.0.1:5200` — в Docker це означає, що nginx-контейнер не може до нього достукатися. Вирішено через `entrypoint.sh`: `-listen 0.0.0.0:5200`.

**Secrets:** паролі не захардкоджені в `docker-compose.yml` — передаються через `.env` файл (не комітиться, є `.env.example`).

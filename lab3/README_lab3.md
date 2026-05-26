# Лабораторна робота №3 — CI/CD

## Зміст

1. [Огляд архітектури](#1-огляд-архітектури)
2. [Структура файлів](#2-структура-файлів)
3. [GitHub Actions pipeline](#3-github-actions-pipeline)
4. [Аналіз коду (linting)](#4-аналіз-коду-linting)
5. [Автоматичні тести](#5-автоматичні-тести)
6. [Збірка Docker-образу](#6-збірка-docker-образу)
7. [Підготовка self-hosted runner](#7-підготовка-self-hosted-runner)
8. [Підготовка target node](#8-підготовка-target-node)
9. [Розгортання](#9-розгортання)
10. [Верифікація](#10-верифікація)
11. [Branch protection rules](#11-branch-protection-rules)
12. [GitHub Secrets](#12-github-secrets)
13. [Workflow розгортання (end-to-end)](#13-workflow-розгортання-end-to-end)

---

## 1. Огляд архітектури

```
 Developer
     │  git push / PR / annotated tag
     ▼
 GitHub Actions (ubuntu-latest)
   ├── lint     — golangci-lint, hadolint, shellcheck, yamllint
   ├── test     — go test, coverage ≥ 40%, artifact upload
   └── build    — docker build + push to GHCR (on push only, not PRs)
                        │
                        │ (тільки annotated tag + build passed)
                        ▼
            self-hosted runner (Ubuntu 24.04)
                 │  SSH
                 ▼
            target node (Ubuntu 24.04)
              docker pull ghcr.io/.../devops:stable
              systemctl restart mywebapp-container
                 │
                 ├── Docker container (--network=host :5200)
                 ├── nginx (proxy → 127.0.0.1:5200)
                 └── MariaDB (localhost:3306)
```

Після розгортання runner виконує `verify.sh`, який перевіряє HTTP-доступність сервісу.

---

## 2. Структура файлів

```
.github/workflows/
  ci-cd.yml               # єдиний pipeline: lint → test → build → deploy

cmd/mywebapp/
  main_test.go            # unit-тести (59 %+ coverage)

lab3/
  scripts/
    setup-runner.sh       # встановлення залежностей на runner VM
    setup-target.sh       # підготовка target node (Docker, nginx, MariaDB, systemd)
    deploy-app.sh         # розгортання образу на target node (запускається via SSH)
    verify.sh             # перевірка після розгортання
  systemd/
    mywebapp-container.service   # systemd unit для Docker-контейнера
  sudoers/
    operator              # NOPASSWD для systemctl mywebapp-container
  README_lab3.md

.golangci.yml             # конфіг golangci-lint
.yamllint.yml             # конфіг yamllint
```

Nginx-конфіг для lab3 — той самий `deploy/nginx/mywebapp.conf`, що й у lab1  
(проксі на `127.0.0.1:5200`). Контейнер стартує з `--network=host`.

---

## 3. GitHub Actions pipeline

Файл: `.github/workflows/ci-cd.yml`

| Job | Тригер | Залежить від |
|-----|--------|--------------|
| `lint` | push main, tags, PR → main | — |
| `test` | push main, tags, PR → main | — |
| `build` | push main + annotated tags | lint, test |
| `deploy` | annotated tags тільки | build |

### Теги образів

| Подія | Теги |
|-------|------|
| push до `main` | `latest`, `sha-<full-commit-sha>` |
| annotated tag | `stable`, `<tag>` |

---

## 4. Аналіз коду (linting)

| Інструмент | Що перевіряє |
|-----------|--------------|
| **golangci-lint** (`v1.59`) | Go-код (`govet`, `staticcheck`, `errcheck`, `ineffassign`, `gofmt`, `misspell`, `unused`) |
| **hadolint** | `Dockerfile` |
| **shellcheck** | всі `*.sh`-файли в репозиторії |
| **yamllint** | `.github/workflows/*.yml` |

Конфіги: `.golangci.yml`, `.yamllint.yml` у корені репозиторію.

---

## 5. Автоматичні тести

Файл: `cmd/mywebapp/main_test.go`

Тести написані для пакету `main`. Для тестування HTTP-хендлерів, що потребують БД,
використовується **in-memory SQLite** через `github.com/glebarez/sqlite` (pure-Go, без CGO).

Що покривається:
- `wantsHTML`, `htmlEscape`, `mysqlDSN` — чисті функції, всі гілки
- `parseServeFlags`, `parseMigrateFlags` — включно з error-кейсами
- `renderRootHTML` — HTTP-відповідь
- `writeJSON`, `decodeJSON` — включно з unknown-field rejection
- `listItems`, `createItem`, `getItem` — JSON та HTML-відповіді, всі error-шляхи

Поточне покриття: **~59.6 %** (поріг: 40 %).

Якщо покриття падає нижче 40 % — `test` job завершується з помилкою і `build`/`deploy` не запускаються.

Артефакт `coverage-report` (`coverage.out`) завантажується при кожному push до `main`.

---

## 6. Збірка Docker-образу

Образ будується з існуючого `Dockerfile` у корені репозиторію.  
Публікується в **GitHub Container Registry** (`ghcr.io`).

Автентифікація в GHCR: `GITHUB_TOKEN` (автоматично доступний в Actions).

Після першого push пакет потрібно зробити **публічним** в налаштуваннях репозиторію  
(`Packages → mywebapp → Package visibility → Public`), щоб target node міг його  
завантажити без автентифікації.

---

## 7. Підготовка self-hosted runner

### 7.1 Запуск VM

Підніміть нову Ubuntu 24.04 Server VM (окремо від target node).

### 7.2 Встановлення залежностей

```bash
sudo bash lab3/scripts/setup-runner.sh
```

Скрипт встановлює: Docker, Git, curl, ssh-client та завантажує бінарник runner.

### 7.3 Реєстрація runner (вручну, токен НЕ зберігається в репо)

```bash
su - github-runner
cd ~/actions-runner
./config.sh \
  --url https://github.com/<owner>/<repo> \
  --token <REGISTRATION_TOKEN> \
  --labels deploy \
  --unattended
sudo ./svc.sh install
sudo ./svc.sh start
```

Токен реєстрації — разовий, отримується в GitHub → Settings → Actions → Runners → **New self-hosted runner**.

### 7.4 SSH-доступ до target node

На runner VM згенеруйте SSH-ключ і додайте публічну частину до `~operator/.ssh/authorized_keys` на target node:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/deploy_key -N ""
# Скопіюйте вміст ~/.ssh/deploy_key.pub на target node
```

Приватну частину (`deploy_key`) додайте як `TARGET_SSH_KEY` у GitHub Secrets (див. [розділ 12](#12-github-secrets)).

### 7.5 Після завершення демонстрації

Зупиніть або видаліть runner VM, щоб уникнути компрометації:

```bash
# На runner VM:
cd ~/actions-runner && sudo ./svc.sh stop
# Або видаліть VM повністю
```

---

## 8. Підготовка target node

### 8.1 Запуск VM

Ubuntu 24.04 Server. Це та сама VM, що й у lab1 (або нова з тим самим образом).

### 8.2 Клонування репо та запуск скрипта

```bash
git clone https://github.com/fedorenkoivan/devops.git
cd devops
sudo bash lab3/scripts/setup-target.sh
```

Що робить скрипт:
- Встановлює Docker, nginx, MariaDB
- Створює користувачів (`student`, `teacher`, `operator`, `mywebapp`)
- Конфігурує MariaDB (bind 127.0.0.1, БД `mywebapp`, юзер `mywebapp`)
- Створює `/etc/mywebapp/env` з конфігурацією застосунку
- Встановлює systemd unit `mywebapp-container.service`
- Встановлює nginx-конфіг та sudoers для `operator`

### 8.3 Env-файл `/etc/mywebapp/env`

```ini
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=mywebapp
DB_PASS=mywebapp
DB_NAME=mywebapp
APP_PORT=5200
```

> Пароль БД у файлі — тільки для локального розгортання на ізольованій VM.  
> У реальному середовищі використовуйте GitHub Secrets і передавайте через pipeline.

---

## 9. Розгортання

Розгортання запускається автоматично при push **анотованого тегу** після успішних lint, test і build.

### Як створити annotated tag

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### Що відбувається в pipeline

1. Runner отримує завдання `deploy`
2. SSH на target node: `ssh operator@<TARGET_HOST>`
3. На target node виконується `lab3/scripts/deploy-app.sh`:
   - `docker pull ghcr.io/fedorenkoivan/devops:<tag>`
   - `docker pull ghcr.io/fedorenkoivan/devops:stable`
   - `sudo systemctl restart mywebapp-container`
4. Runner виконує `lab3/scripts/verify.sh http://<TARGET_HOST>`

### systemd unit

`lab3/systemd/mywebapp-container.service`:

- Запускає контейнер з `--network=host` (доступ до MariaDB на localhost)
- `EnvironmentFile=/etc/mywebapp/env`
- Автоматично видаляє старий контейнер перед стартом (`ExecStartPre=-docker stop/rm`)
- `Restart=on-failure`
- **Без socket activation** (не потрібно в цій роботі)

---

## 10. Верифікація

Скрипт `lab3/scripts/verify.sh <base_url>` перевіряє:

| Перевірка | Очікуваний результат |
|-----------|---------------------|
| `GET /` | HTTP 200 |
| `GET /items` | HTTP 200 |
| `GET /health/alive` | HTTP 404 (nginx блокує) |
| `GET /health/ready` | HTTP 404 (nginx блокує) |
| Content-Type `/items` | `application/json` |

При будь-якій помилці скрипт завершується з кодом 1 → job `deploy` падає.

---

## 11. Branch protection rules

Налаштовуються в GitHub → Settings → Branches → Add rule для гілки `main`:

- [x] **Require status checks to pass before merging**
  - Required checks: `Lint`, `Test`
- [x] **Require branches to be up to date before merging**
- [x] **Do not allow bypassing the above settings**

Це гарантує, що PR не можна злити, якщо lint або тести не пройшли.

---

## 12. GitHub Secrets

Налаштовуються в Settings → Secrets and variables → Actions:

| Secret | Що містить |
|--------|-----------|
| `TARGET_HOST` | IP-адреса або hostname target node |
| `TARGET_SSH_KEY` | Приватний SSH-ключ для підключення runner → target (`operator@TARGET_HOST`) |

`GITHUB_TOKEN` — вбудований, не потребує налаштування вручну.

---

## 13. Workflow розгортання (end-to-end)

```
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
         │
         ▼
[GitHub Actions]
  lint  ──────────────────────────────► OK
  test  ──► coverage.out artifact ────► OK
  build ──► ghcr.io/.../devops:stable
         ──► ghcr.io/.../devops:v1.0.0 ► OK
  deploy (self-hosted runner):
    ssh operator@<TARGET_HOST>
      docker pull ghcr.io/.../devops:stable
      systemctl restart mywebapp-container
    verify.sh http://<TARGET_HOST>
      GET /       → 200 ✓
      GET /items  → 200 ✓
      GET /health/alive → 404 ✓
      GET /health/ready → 404 ✓
      Content-Type /items → application/json ✓
    ► DEPLOY SUCCESS
```

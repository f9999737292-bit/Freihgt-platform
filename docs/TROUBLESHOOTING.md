# Troubleshooting

Типичные проблемы при работе с Freight Platform.

## Не могу найти файлы после закрытия Cursor

**Решение:**

1. Откройте Cursor.
2. **File → Open Folder**.
3. Выберите корневую папку **`freight-platform`** (не подпапку вроде `apps/web-admin`).
4. Убедитесь, что видны каталоги: `apps`, `services`, `infrastructure`, `docs`.

Быстрая навигация:

```bash
make project-map
make tree-project
make find-service NAME=company
```

Документы: [PROJECT_MAP.md](./PROJECT_MAP.md), [FILE_INDEX.md](./FILE_INDEX.md).

## Не запускается frontend

**Проверить:**

```bash
node -v
npm -v
```

Если Node.js не установлен — установите **Node.js 20 LTS** или **22 LTS**, либо на Windows:

```bash
make setup-node
```

Затем:

```bash
make install-web-admin
make run-web-admin
```

## Backend не отвечает

**Проверить:**

```bash
make platform-ps
make health-check
docker compose -f infrastructure/docker-compose/docker-compose.yml logs -f
```

Перезапуск:

```bash
make platform-restart
make migrate-up
make health-check
```

## `make platform-up` падает при скачивании Prometheus/Grafana

Prometheus и Grafana вынесены в Docker Compose profile `observability`. Обычный `make platform-up` **не** должен их тянуть и запускать.

```bash
make platform-up      # только backend + postgres
make migrate-up
make health-check
```

Мониторинг отдельно (когда сеть доступна):

```bash
make observability-up
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001

Подробнее: [OBSERVABILITY.md](./OBSERVABILITY.md).

## Проверка DB metrics на Windows

`make db-metrics-check` и `make generate-db-metrics-traffic` используют Python-скрипты (`scripts/dev/`), без `grep` и `curl`.

```bash
make generate-db-metrics-traffic
make db-metrics-check
```

Нужен Python в `PATH` как `python`. При 401 от API Gateway см. предупреждение в выводе скрипта или временно `AUTH_ENABLED=false` в docker-compose.

Если `make health-check` или `make db-metrics-check` падает с ошибкой «python не найден» (код 9009 на Windows), установите **Python 3** (python.org или `winget install Python.Python.3.12`) и убедитесь, что `python --version` работает в том же терминале, где запускаете `make`. На Linux/macOS: `make PYTHON=python3 health-check`. Подробнее: [WINDOWS_ENVIRONMENT.md](./WINDOWS_ENVIRONMENT.md).

## Python is not in PATH

**Симптом:**

- Makefile scripts fail with code 9009
- `python` command not found

**Решение:**

- установить Python 3.12 (python.org или `winget install Python.Python.3.12`)
- при установке отметить **Add Python to PATH**
- или использовать Windows launcher:

```bash
py -3 --version
make python-check-win
```

**Команды с override:**

```bash
make PYTHON="py -3" health-check
make PYTHON="py -3" db-metrics-check
make PYTHON="py -3" db-pool-metrics-check
make PYTHON="py -3" docker-readiness
make PYTHON="py -3" ports-check
```

## Windows port reservation

**Симптом:**

- Docker container cannot bind 8080/8081/etc.
- port is excluded/reserved by Windows (Hyper-V / winnat)

**Диагностика:**

```bash
make ports-check
```

или:

```bash
make PYTHON="py -3" ports-check
```

**Возможное решение** (от имени администратора, не во время активных сетевых задач):

```bat
net stop winnat
net start winnat
```

Затем перезапустите Docker Desktop и снова `make ports-check`. Подробнее: [WINDOWS_ENVIRONMENT.md](./WINDOWS_ENVIRONMENT.md).

## Компания не видна в Companies

Проверьте tenant в Settings:

```
74519f22-ff9b-4a8b-8fff-a958c689682f
```

Заголовок `X-Tenant-ID` должен совпадать с tenant компании в базе.

## API Gateway возвращает 401

Проверьте:

- выполнен ли login в UI;
- есть ли JWT token в session / localStorage;
- переменная `AUTH_ENABLED` в docker-compose (по умолчанию может быть `false`);
- создан ли dev admin: `make seed-dev-admin` (см. [AUTH_RBAC.md](./AUTH_RBAC.md))

Если `make seed-dev-admin` завершается с `API Gateway unavailable` — сначала `make platform-up`. Скрипт использует прямые вызовы identity/company service (как integration smoke test); назначение роли может вывести TODO, если endpoint требует JWT.

При mock auth (`NUXT_PUBLIC_MOCK_AUTH=true`) gateway может работать без реального JWT.

## Shipment не добавляется в Billing Register

Проверьте статус shipment:

- должен быть `READY_FOR_BILLING`;
- или `DOCUMENTS_COMPLETED`, если backend это разрешает.

Поиск статусов в коде:

```bash
make find-text TEXT=READY_FOR_BILLING
```

## mark-paid не работает

Последовательность для УПД:

1. create UPD
2. mark-sent-to-edo
3. mark-signed
4. mark-paid

Каждый шаг меняет статус; пропуск шага блокирует следующий.

## Smoke test падает

```bash
make platform-health
make integration-smoke-test
```

Убедитесь, что все сервисы `:8081`–`:8087` доступны. Подробности: `tests/integration/README.md`.

`make full-flow-smoke-test` — alias на `smoke-test.sh` (см. `tests/integration/full-flow-smoke-test.sh`). Полноценный отдельный full-flow тест можно расширить позже.

## Команды поиска не работают на Windows

Makefile использует bash (`find`, `grep`). Запускайте через **Git Bash**, **WSL** или терминал Cursor с bash. Альтернатива — поиск в IDE (**Ctrl+Shift+F**).

Targets: `make project-map`, `make tree-project`, `make find-service NAME=...`, `make find-text TEXT=...`.

## Windows path with Cyrillic characters

Некоторые Makefile/npm команды могут падать, если путь проекта содержит кириллицу (например, имя пользователя Windows `Пользователь`).

**Рекомендуемый путь проекта:**

- `C:\Projects\freight-platform`
- или `C:\dev\freight-platform`

Не используйте путь с русскими буквами для Node/Docker/Make.

**Проверка frontend без Make:**

```bash
cd apps/web-admin
npm run build
```

Если `npm run build` проходит, а `make build-web-admin` падает — перенесите репозиторий в ASCII-путь.

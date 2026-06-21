# Справочник разработчика

## 1. Как открыть проект в Cursor

1. Запустите Cursor.
2. **File → Open Folder** (или **Open Workspace from File** → `freight-platform.code-workspace`).
3. Выберите корневую папку **`freight-platform`**.
4. **Не открывайте** отдельную папку вроде `services/company-service` как самостоятельный проект — всегда работайте из корня монорепозитория.

## 2. Как понять, что вы в правильной папке

В корне должны быть видны:

- `Makefile`
- `README.md`
- `apps/`
- `services/`
- `infrastructure/`
- `packages/`
- `docs/`

Проверка в терминале:

```bash
pwd
ls Makefile README.md apps services infrastructure packages docs
```

## 3. Как запустить backend

```bash
make platform-up
make migrate-up
make platform-health
```

Дополнительно:

```bash
make platform-ps       # статус контейнеров
make platform-logs     # логи
make platform-down     # остановить
```

## 4. Как запустить frontend

```bash
make install-web-admin
make run-web-admin
```

На Windows без глобального Node.js:

```bash
make setup-node
make install-web-admin
make run-web-admin
```

## 5. Как открыть интерфейс

- Admin UI: http://127.0.0.1:3000

## 6. Как открыть API docs

- Swagger UI: http://localhost:8080/docs
- OpenAPI YAML: http://localhost:8080/openapi.yaml

```bash
make platform-up
make api-docs-open
```

## 7. Как найти нужный сервис

| Задача | Путь |
|--------|------|
| Компании | `services/company-service` |
| Пользователи, login, JWT | `services/identity-service` |
| Транспортные заявки | `services/transport-order-service` |
| Тендеры, bids | `services/rfx-service` |
| Перевозки | `services/shipment-service` |
| Документы, подписание | `services/document-service` |
| Реестры, УПД | `services/billing-register-service` |
| Frontend admin | `apps/web-admin` |
| Docker | `infrastructure/docker-compose` |
| Миграции | `infrastructure/migrations` |

Makefile-поиск:

```bash
make find-service NAME=company
make find-service NAME=shipment
make find-service NAME=billing
```

## 8. Как искать файлы в терминале

Через Makefile (рекомендуется):

```bash
make find-go
make find-vue
make find-service NAME=company
make tree-project
```

Напрямую (Git Bash / Linux / macOS):

```bash
find . -name "*company*"
find . -name "*.go"
find . -name "*.vue"
find . -name "docker-compose.yml"
```

В Cursor: **Ctrl+P** — быстрый переход к файлу по имени.

## 9. Как искать текст в проекте

Через Makefile:

```bash
make find-text TEXT=TransportOrder
make find-text TEXT=BillingRegister
make find-text TEXT=READY_FOR_BILLING
make find-text TEXT=api/v1/companies
```

Напрямую:

```bash
grep -R "TransportOrder" . --exclude-dir=node_modules --exclude-dir=.git
grep -R "BillingRegister" .
grep -R "READY_FOR_BILLING" .
grep -R "api/v1/companies" .
```

## 10. Полезные команды Makefile

```bash
make help
make project-map
make go-build
make go-test
make integration-smoke-test
make openapi-check
```

## 11. Документация

| Документ | Ссылка |
|----------|--------|
| Карта проекта | [PROJECT_MAP.md](./PROJECT_MAP.md) |
| Индекс файлов | [FILE_INDEX.md](./FILE_INDEX.md) |
| Быстрый старт | [QUICK_START.md](./QUICK_START.md) |
| Troubleshooting | [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) |

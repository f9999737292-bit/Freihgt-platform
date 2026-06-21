# Индекс ключевых файлов

Быстрый указатель на важные файлы проекта. Полный список — через `make tree-project` или поиск в IDE.

## Backend services

### api-gateway

| Файл | Назначение |
|------|------------|
| `services/api-gateway/cmd/server/main.go` | Точка входа |
| `services/api-gateway/internal/config/config.go` | Конфигурация |
| `services/api-gateway/internal/http/router.go` | Маршрутизация |
| `services/api-gateway/internal/http/proxy.go` | Прокси к микросервисам |
| `services/api-gateway/internal/http/middleware/auth.go` | JWT-аутентификация |
| `services/api-gateway/README.md` | Документация сервиса |
| `services/api-gateway/Dockerfile` | Docker-образ |

### identity-service

| Файл | Назначение |
|------|------------|
| `services/identity-service/cmd/server/main.go` | Точка входа |
| `services/identity-service/internal/http/handlers/auth_handler.go` | Login, `/auth/me` |
| `services/identity-service/internal/http/handlers/user_handler.go` | CRUD пользователей |
| `services/identity-service/internal/service/auth_service.go` | Бизнес-логика auth |
| `services/identity-service/internal/repository/user_repository.go` | Доступ к БД |
| `services/identity-service/README.md` | Документация сервиса |

### company-service

| Файл | Назначение |
|------|------------|
| `services/company-service/cmd/server/main.go` | Точка входа |
| `services/company-service/internal/http/handlers/company_handler.go` | API компаний |
| `services/company-service/internal/http/handlers/membership_handler.go` | Участники компании |
| `services/company-service/internal/service/company_service.go` | Бизнес-логика |
| `services/company-service/internal/repository/company_repository.go` | Доступ к БД |
| `services/company-service/README.md` | Документация сервиса |

### transport-order-service

| Файл | Назначение |
|------|------------|
| `services/transport-order-service/cmd/server/main.go` | Точка входа |
| `services/transport-order-service/internal/http/handlers/transport_order_handler.go` | API заявок |
| `services/transport-order-service/internal/service/transport_order_service.go` | Бизнес-логика |
| `services/transport-order-service/README.md` | Документация сервиса |

### rfx-service

| Файл | Назначение |
|------|------------|
| `services/rfx-service/cmd/server/main.go` | Точка входа |
| `services/rfx-service/internal/http/handlers/rfx_handler.go` | RFx / тендеры |
| `services/rfx-service/internal/http/handlers/freight_request_handler.go` | Freight requests |
| `services/rfx-service/internal/http/handlers/bid_handler.go` | Ставки (bids) |
| `services/rfx-service/README.md` | Документация сервиса |

### shipment-service

| Файл | Назначение |
|------|------------|
| `services/shipment-service/cmd/server/main.go` | Точка входа |
| `services/shipment-service/internal/http/handlers/shipment_handler.go` | API перевозок |
| `services/shipment-service/internal/service/shipment_service.go` | Бизнес-логика |
| `services/shipment-service/README.md` | Документация сервиса |

### document-service

| Файл | Назначение |
|------|------------|
| `services/document-service/cmd/server/main.go` | Точка входа |
| `services/document-service/internal/http/handlers/document_handler.go` | Документы, версии, файлы |
| `services/document-service/internal/http/handlers/signing_handler.go` | Mock signing |
| `services/document-service/README.md` | Документация сервиса |

### billing-register-service

| Файл | Назначение |
|------|------------|
| `services/billing-register-service/cmd/server/main.go` | Точка входа |
| `services/billing-register-service/internal/http/handlers/billing_register_handler.go` | Реестры, УПД |
| `services/billing-register-service/internal/service/billing_register_service.go` | Бизнес-логика |
| `services/billing-register-service/README.md` | Документация сервиса |

## Frontend (web-admin)

### Страницы

| Файл | Назначение |
|------|------------|
| `apps/web-admin/pages/login.vue` | Вход |
| `apps/web-admin/pages/dashboard/index.vue` | Дашборд |
| `apps/web-admin/pages/companies/index.vue` | Список компаний |
| `apps/web-admin/pages/companies/[id].vue` | Карточка компании |
| `apps/web-admin/pages/users/index.vue` | Пользователи |
| `apps/web-admin/pages/transport-orders/index.vue` | Транспортные заявки |
| `apps/web-admin/pages/rfx/index.vue` | RFx / тендеры |
| `apps/web-admin/pages/freight-requests/index.vue` | Freight requests |
| `apps/web-admin/pages/shipments/index.vue` | Перевозки |
| `apps/web-admin/pages/documents/index.vue` | Документы |
| `apps/web-admin/pages/billing-registers/index.vue` | Реестры (stub UI) |

### Инфраструктура UI

| Файл | Назначение |
|------|------------|
| `apps/web-admin/composables/useApi.ts` | HTTP-клиент, заголовки |
| `apps/web-admin/composables/useAuth.ts` | Login / logout |
| `apps/web-admin/stores/auth.ts` | Сессия, токен |
| `apps/web-admin/stores/tenant.ts` | Tenant ID |
| `apps/web-admin/components/layout/AppSidebar.vue` | Навигация |
| `apps/web-admin/README.md` | Документация frontend |

### Модули по доменам

| Папка | Содержимое |
|-------|------------|
| `apps/web-admin/components/rfx/` | RFx UI |
| `apps/web-admin/components/freight-requests/` | Freight requests, bids |
| `apps/web-admin/components/shipments/` | Shipments UI |
| `apps/web-admin/components/documents/` | Documents UI |
| `apps/web-admin/components/signing/` | Signing UI |
| `apps/web-admin/types/` | TypeScript-типы по доменам |

## Infrastructure

| Файл / папка | Назначение |
|--------------|------------|
| `infrastructure/docker-compose/docker-compose.yml` | Docker Compose |
| `infrastructure/migrations/` | SQL-миграции PostgreSQL |
| `.env.example` | Пример переменных окружения |
| `Makefile` | Команды разработки |

## Tests

| Файл | Назначение |
|------|------------|
| `tests/integration/smoke-test.sh` | End-to-end smoke test (через прямые URL сервисов) |
| `tests/integration/README.md` | Описание интеграционных тестов |
| `tests/integration/env.example` | Переменные для smoke test |
| `tests/integration/payloads/` | JSON-payloads для тестов |

> **Примечание:** целевой скрипт `tests/integration/full-flow-smoke-test.sh` (через gateway `:8080`) пока не добавлен. Используйте `make integration-smoke-test`.

## OpenAPI

| Файл | Назначение |
|------|------------|
| `packages/openapi/openapi.yaml` | Сводная OpenAPI-спецификация |
| `packages/openapi/schemas/` | Схемы по доменам |
| `scripts/openapi/generate_openapi.py` | Генерация |

# Карта проекта Freight Platform

Верхнеуровневая структура монорепозитория `freight-platform`.

```
freight-platform/
├── apps/                    # Frontend-приложения (Nuxt 3)
├── services/                # Backend-микросервисы (Go)
├── packages/                # Общие пакеты (OpenAPI, UI, i18n, shared)
├── infrastructure/          # Docker Compose, SQL-миграции
├── docs/                    # Документация
├── tests/                   # Интеграционные тесты
├── scripts/                 # Вспомогательные скрипты
├── Makefile                 # Команды разработки
└── README.md                # Точка входа в проект
```

## apps/

| Путь | Назначение |
|------|------------|
| `apps/web-admin` | Админ-панель на Nuxt 3 (порт 3000) — основной UI для операторов |
| `apps/web-shipper` | Интерфейс грузоотправителя (порт 3001) |
| `apps/web-carrier` | Интерфейс перевозчика (порт 3002) |
| `apps/web-consignee` | Интерфейс грузополучателя (порт 3003) |
| `apps/web-finance` | Финансовый интерфейс (порт 3004) |
| `apps/web-procurement` | Закупки (порт 3005) |

## services/

| Путь | Порт | Назначение |
|------|------|------------|
| `services/api-gateway` | 8080 | Единый входной API, прокси, Swagger UI |
| `services/identity-service` | 8081 | Пользователи, login, JWT, роли |
| `services/company-service` | 8082 | Компании и сотрудники |
| `services/transport-order-service` | 8083 | Транспортные заявки, locations, cargo |
| `services/rfx-service` | 8084 | RFx, mini tender, freight requests, bids |
| `services/shipment-service` | 8085 | Перевозки, водители, автомобили, статусы |
| `services/document-service` | 8086 | Документы, версии, файлы, mock signing |
| `services/billing-register-service` | 8087 | Реестры, УПД, закрытие |
| `services/localization-service` | — | Локализация (вспомогательный сервис) |

## packages/

| Путь | Назначение |
|------|------------|
| `packages/openapi` | OpenAPI-спецификации и сгенерированные артефакты |
| `packages/shared-go` | Общий Go-код: config, logging, health |
| `packages/shared-ts` | Общие TypeScript-типы |
| `packages/ui` | Общие Vue-компоненты |
| `packages/i18n` | Общие локали (ru-RU, en-US, zh-CN) |
| `packages/proto` | Protobuf-определения |

## infrastructure/

| Путь | Назначение |
|------|------------|
| `infrastructure/docker-compose` | Docker Compose для локальной разработки |
| `infrastructure/migrations` | SQL-миграции PostgreSQL |

## docs/

Архитектура, API, база данных, биллинг и навигационная документация для разработчиков.

| Файл | Описание |
|------|----------|
| `docs/PROJECT_MAP.md` | Эта карта проекта |
| `docs/FILE_INDEX.md` | Индекс ключевых файлов |
| `docs/DEVELOPER_HANDBOOK.md` | Справочник разработчика |
| `docs/QUICK_START.md` | Быстрый старт |
| `docs/TROUBLESHOOTING.md` | Решение типичных проблем |

## tests/

| Путь | Назначение |
|------|------------|
| `tests/integration` | Интеграционные smoke-тесты (`smoke-test.sh`) |

## scripts/

| Путь | Назначение |
|------|------------|
| `scripts/dev` | Dev seed-скрипты (планируется: `seed_dev_admin.sh`) |
| `scripts/openapi` | Генерация и валидация OpenAPI |
| `scripts/setup-node.ps1` | Установка portable Node.js (Windows) |

## Главная бизнес-цепочка

```
Company
  → User
  → Transport Order
  → Mini Tender
  → Bid
  → Shipment
  → Document
  → Billing Register
  → UPD
  → Closed
```

| Этап | Backend | Frontend (web-admin) |
|------|---------|----------------------|
| Company | `company-service` | `pages/companies/` |
| User | `identity-service` | `pages/users/` |
| Transport Order | `transport-order-service` | `pages/transport-orders/` |
| Mini Tender / RFx | `rfx-service` | `pages/rfx/`, `pages/freight-requests/` |
| Bid | `rfx-service` | `components/freight-requests/` |
| Shipment | `shipment-service` | `pages/shipments/` |
| Document | `document-service` | `pages/documents/` |
| Billing Register / UPD | `billing-register-service` | `pages/billing-registers/` |

## Быстрый поиск

```bash
make project-map          # ссылки на документацию
make tree-project         # дерево каталогов (3 уровня)
make find-service NAME=company
make find-vue
make find-text TEXT=READY_FOR_BILLING
```

# Быстрый старт

Минимальная инструкция для локального запуска Freight Platform.

## 1. Открыть проект в Cursor

**File → Open Folder** → выберите корень `freight-platform`.

## 2. Проверить Docker Desktop

Docker Desktop должен быть запущен (PostgreSQL и backend-сервисы поднимаются через Compose).

## 3. Запустить backend

```bash
make platform-up
make migrate-up
make platform-health
```

Ожидаемый результат: все health-эндпоинты `:8080`–`:8087` отвечают.

## 4. Запустить frontend

```bash
make install-web-admin
make run-web-admin
```

## 5. Открыть интерфейс

http://127.0.0.1:3000

## 6. Dev login

| Поле | Значение |
|------|----------|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Email | `admin@7rights.local` |
| Password | `Admin123456!` |

Tenant ID также можно задать в **Settings** приложения.

> По умолчанию включён mock auth (`NUXT_PUBLIC_MOCK_AUTH=true` в `apps/web-admin/.env.example`).

## 7. Если dev admin не создан

```bash
make seed-dev-admin
```

> Команда `seed-dev-admin` будет доступна после добавления скрипта `scripts/dev/seed_dev_admin.sh`. Пока используйте mock auth или создайте пользователя через API identity-service.

## 8. Проверить полный flow

```bash
make integration-smoke-test
```

Или (когда будет добавлен):

```bash
make full-flow-smoke-test
```

## Полезные ссылки

- [Карта проекта](./PROJECT_MAP.md)
- [Индекс файлов](./FILE_INDEX.md)
- [Справочник разработчика](./DEVELOPER_HANDBOOK.md)
- [Troubleshooting](./TROUBLESHOOTING.md)
- API docs: http://localhost:8080/docs

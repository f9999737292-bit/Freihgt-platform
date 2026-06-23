# Auth & RBAC (web-admin)

Краткий справочник по аутентификации и проверкам ролей/прав в административном frontend.

## Login

- **Страница:** `apps/web-admin/pages/login.vue`
- **Store:** `apps/web-admin/stores/auth.ts` — метод `login()`, хранение JWT и user в session/localStorage (`freight_admin_session`)
- **Mock auth:** при `NUXT_PUBLIC_MOCK_AUTH=true` (по умолчанию в `.env.example`) login не вызывает backend и создаёт demo-сессию

## Dev admin seed

Для реального backend (без mock):

```bash
make platform-up
make migrate-up
make seed-dev-admin
```

Скрипт: `scripts/dev/seed_dev_admin.sh`

- Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`
- Email: `admin@7rights.local`
- Password: `Admin123456!`
- Company: «ООО 7Rights Dev» (`PLATFORM_OPERATOR`)

Скрипт идемпотентен: повторный запуск не создаёт дубликаты. Требует доступный API Gateway (`http://localhost:8080/health`). Если gateway недоступен — см. [TROUBLESHOOTING.md](./TROUBLESHOOTING.md).

> **TODO:** endpoints создания пользователя/назначения роли могут требовать auth. Скрипт выводит предупреждения, но не падает без понятного сообщения, если endpoint ещё не реализован.

## usePermissions composable

**Файл:** `apps/web-admin/composables/usePermissions.ts`

```ts
const { hasRole, hasAnyRole, hasPermission, hasAnyPermission, isPlatformAdmin } = usePermissions()
```

| Функция | Описание |
|---------|----------|
| `hasRole(role)` | Есть ли роль у текущего пользователя |
| `hasAnyRole(roles)` | Есть ли хотя бы одна из ролей |
| `hasPermission(permission)` | Есть ли permission |
| `hasAnyPermission(permissions)` | Есть ли хотя бы одно permission |
| `isPlatformAdmin()` | Роль `PLATFORM_ADMIN` или dev fallback |

Low-code permission matrix: `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` — используйте `useLowCodePermissions()` для admin/migration/runtime edit в low-code UI.

### Источник данных

- Роли и permissions читаются из `authStore.user` (`roles[]`, `permissions[]`)
- Login и `/auth/me` возвращают `roles[]` для tenant (identity-service)

### Dev fallback

В mock auth mode, если `user.email === admin@7rights.local`, `isPlatformAdmin()` и остальные проверки возвращают `true` — только для локальной разработки UI.

## Рекомендации

- Не встраивать жёсткую бизнес-логику RBAC в компоненты — используйте `usePermissions()`
- Не менять token storage и login flow без необходимости
- После появления RBAC в `/auth/me` — расширить `AuthUser` и убрать dev fallback

## См. также

- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) — API Gateway 401, seed-dev-admin
- [QUICK_START.md](./QUICK_START.md) — быстрый старт платформы

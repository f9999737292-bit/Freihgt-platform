# Windows Environment Guide

Руководство по подготовке окружения Freight Platform на Windows (Git Bash, PowerShell, WSL).

## Recommended project path

**Рекомендуемый путь:**

```
C:\Projects\freight-platform
```

или

```
C:\dev\freight-platform
```

**Не рекомендуется:**

- путь с кириллицей (например `C:\Users\Пользователь\...`)
- путь в OneDrive (синхронизация ломает `node_modules` и Docker volumes)
- очень длинный путь (ограничения Windows MAX_PATH)

Подробнее: [TROUBLESHOOTING.md](./TROUBLESHOOTING.md#windows-path-with-cyrillic-characters).

## Python

Проверить:

```bash
python --version
```

Если не работает (код 9009):

```bash
py -3 --version
make python-check-win
```

Для Makefile-скриптов:

```bash
make PYTHON="py -3" health-check
make PYTHON="py -3" db-metrics-check
make PYTHON="py -3" db-pool-metrics-check
make PYTHON="py -3" generate-db-metrics-traffic
```

Быстрая проверка:

```bash
make python-check
```

На Linux/macOS, если `python` отсутствует:

```bash
make PYTHON=python3 health-check
```

## Docker Desktop

Проверить готовность:

```bash
make docker-readiness
```

или на Windows без `python` в PATH:

```bash
make PYTHON="py -3" docker-readiness
```

Если Docker disk разросся:

```bash
make docker-disk-usage
make docker-clean-safe
```

Затем перезапустите **Docker Desktop**. Скрипт **не** выполняет `docker volume prune`.

Подробнее: [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md).

## Windows ports

Диагностика портов платформы:

```bash
make ports-check
```

или:

```bash
make PYTHON="py -3" ports-check
```

Если Windows зарезервировал порт (ошибка bind при `make platform-up`):

1. Перезапустите Docker Desktop
2. От имени **администратора** (PowerShell или cmd):

```bat
net stop winnat
net start winnat
```

**Предупреждение:**

- выполняйте `net stop/start winnat` только от администратора
- не делайте это во время активных сетевых задач (VPN, удалённый рабочий стол, важные соединения)
- после перезапуска NAT проверьте: `make ports-check`

## Environment checks (quick)

```bash
make python-check
make docker-readiness
make ports-check
```

Windows alternative:

```bash
make PYTHON="py -3" docker-readiness
make PYTHON="py -3" ports-check
```

## Runtime verification

После восстановления окружения:

```bash
make platform-up
make migrate-up
make health-check
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
```

Если Python не в PATH:

```bash
make PYTHON="py -3" health-check
make PYTHON="py -3" generate-db-metrics-traffic
make PYTHON="py -3" db-metrics-check
make PYTHON="py -3" db-pool-metrics-check
```

## See also

- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
- [QUICK_START.md](./QUICK_START.md)
- [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md)

# Runtime Verification Report v0.1

Дата: 2026-06-19  
Путь проекта: `C:\Projects\freight-platform` (ASCII, рекомендуемый)  
Python: `$env:LOCALAPPDATA\Programs\Python\Python312\python.exe` (3.12.10, не в PATH)  
Make: `C:\Program Files (x86)\GnuWin32\bin\make.exe`

## Summary

Runtime-цепочка **не пройдена**: Docker Desktop перестал запускаться после падения `platform-up` на сборке `billing-register-service`. Backend health, metrics, seed и smoke test заблокированы. Окружение (python-check, ports-check) частично OK до краха Docker.

## Results

| Check | Status | Notes |
| ----- | ------ | ----- |
| python-check | OK | Python 3.12.10 через `PYTHON=$PY`; `python` в PATH — Windows Store stub |
| docker-readiness | OK → FAILED | До `platform-up`: OK; после сборки: Docker Desktop unable to start (WSL `0xc00000fd`) |
| ports-check | OK | Все 12 портов FREE до запуска платформы |
| platform-up | FAILED | `billing-register-service`: `error reading from server: EOF` при `go build`; затем WSL stack overflow |
| migrate-up | FAILED | Docker Desktop unable to start |
| health-check | FAILED | Все сервисы `:8080`–`:8087` UNAVAILABLE |
| db-metrics-check | SKIPPED | Backend не запущен |
| db-pool-metrics-check | SKIPPED | Backend не запущен |
| seed-dev-admin | SKIPPED | API Gateway недоступен |
| web-admin build | SKIPPED | `npm` не в PATH; `.tools/node` частично скопирован (сломан `npm-cli.js`) |
| web-admin runtime | SKIPPED | Node/npm недоступны |
| full-flow-smoke-test | FAILED | `bash` недоступен: `execvpe(/bin/bash) failed`; alias на `smoke-test.sh` |
| performance-smoke | SKIPPED | k6 not installed |

## Detailed errors

### platform-up

```
target billing-register-service: failed to receive status: rpc error: code = Unavailable desc = error reading from server: EOF
make: *** [platform-up] Error 1
```

**Причина:** Docker disk / WSL — на диске `C:` ~0.49 GB свободно; параллельная сборка 8 Go-сервисов исчерпала ресурсы WSL.

### Docker Desktop после сборки

```
Error response from daemon: Docker Desktop is unable to start
exit status 0xc00000fd
wsl: Конфигурация прокси-сервера localhost обнаружена, но не отражена в WSL
```

**Причина:** WSL engine crash (stack overflow `0xc00000fd`) — см. [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md).

### health-check

```
api-gateway: UNAVAILABLE http://localhost:8080/health
...
billing-register-service: UNAVAILABLE http://localhost:8087/health
Health check completed: FAILED
```

### full-flow-smoke-test

```
Running full-flow smoke test...
Note: current full-flow-smoke-test is an alias to smoke-test.sh and should be expanded later.
execvpe(/bin/bash) failed: No such file or directory
```

Требуется Git Bash или WSL с bash.

### web-admin build

```
Error: Cannot find module 'C:\Projects\freight-platform\.tools\node\node_modules\npm\bin\npm-cli.js'
```

Portable Node в `.tools/node` неполный после robocopy; системный `npm` отсутствует.

## Blockers

| Blocker | Impact |
| ------- | ------ |
| **Docker disk / WSL** | `platform-up` падает на build; Docker Desktop не стартует (`0xc00000fd`) |
| **Low disk space (~0.5 GB on C:)** | Недостаточно места для Docker build cache и WSL |
| **Python not in PATH** | Обход: `make PYTHON="$env:LOCALAPPDATA\Programs\Python\Python312\python.exe" ...` |
| **Node/npm unavailable** | `.tools/node` повреждён; `make setup-node` или установка Node 20+ |
| **bash not available** | `integration-smoke-test`, `full-flow-smoke-test`, `seed-dev-admin` требуют Git Bash/WSL |
| **k6 not installed** | `performance-smoke` пропущен |
| **Duplicate project path** | Старая копия `C:\Users\Пользователь\freight-platform` занимает место на диске |

## Next fixes

Минимальные действия (без `docker volume prune`, без destructive cleanup):

1. **Восстановить Docker Desktop**
   - Quit Docker Desktop полностью
   - Освободить место на `C:` (удалить старую копию `C:\Users\Пользователь\freight-platform` после проверки `C:\Projects\...`)
   - Перезапустить Docker Desktop
   - Если WSL не стартует: `wsl --shutdown`, затем снова Docker Desktop

2. **Освободить диск** (без volume prune)
   ```powershell
   make docker-clean-safe   # когда Docker снова работает
   ```

3. **Исправить Python PATH**
   - Отключить Windows Store python aliases
   - Или всегда: `$PY = "$env:LOCALAPPDATA\Programs\Python\Python312\python.exe"`

4. **Восстановить Node.js**
   ```powershell
   cd C:\Projects\freight-platform
   make setup-node          # переустановит .tools/node если node.exe отсутствует/битый
   make install-web-admin
   npm run build            # в apps/web-admin
   ```

5. **Повторить runtime chain**
   ```powershell
   $PY = "$env:LOCALAPPDATA\Programs\Python\Python312\python.exe"
   $MAKE = "C:\Program Files (x86)\GnuWin32\bin\make.exe"
   cd C:\Projects\freight-platform

   & $MAKE PYTHON="$PY" python-check
   & $MAKE PYTHON="$PY" docker-readiness
   & $MAKE PYTHON="$PY" ports-check
   & $MAKE platform-up
   & $MAKE migrate-up
   & $MAKE db-check
   & $MAKE PYTHON="$PY" health-check
   & $MAKE PYTHON="$PY" generate-db-metrics-traffic
   & $MAKE PYTHON="$PY" db-metrics-check
   & $MAKE PYTHON="$PY" db-pool-metrics-check
   ```

6. **Smoke / seed** — из Git Bash:
   ```bash
   make seed-dev-admin
   make full-flow-smoke-test
   ```

7. **Windows ports** — если bind падает:
   ```bat
   net stop winnat
   net start winnat
   ```
   (от администратора)

## Environment notes

- Проект перенесён в `C:\Projects\freight-platform` (ASCII path) — OK
- `make build-web-admin` с кириллицей в пути — не тестировался в этом прогоне (использован `C:\Projects`)
- Prometheus/Grafana не требовались (optional profile)

## References

- [WINDOWS_ENVIRONMENT.md](./WINDOWS_ENVIRONMENT.md)
- [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md)
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
- [PROJECT_AUDIT_REPORT_V0.1.md](./PROJECT_AUDIT_REPORT_V0.1.md)
- [FRONTEND_BACKEND_STATUS.md](./FRONTEND_BACKEND_STATUS.md)

---

## Project Recovery Check

Дата: 2026-06-19 (Project Recovery & Continue Pack v0.1)  
Путь: `C:\Projects\freight-platform`

| Check                  | Status  | Notes |
| ---------------------- | ------- | ----- |
| Project root found     | OK      | Makefile, README.md, services/, apps/, infrastructure/, docs/ присутствуют |
| Key docs found         | OK      | Все 8 ключевых документов найдены |
| Makefile targets found | OK      | Все 16 required targets присутствуют |
| Python check           | OK      | Python 3.12.10; `make python-check` и `PYTHON=$PY` — OK |
| Docker readiness       | FAILED  | Daemon unavailable: `npipe:////./pipe/dockerDesktopLinuxEngine` not found; disk ~2.87 GB free (low) |
| Ports check            | OK      | 12 free, 0 in use |
| Platform up            | SKIPPED | Docker daemon недоступен — не запускали |
| Migrations             | SKIPPED | Docker daemon недоступен |
| Health check           | SKIPPED | Backend не запущен |
| DB query metrics       | SKIPPED | Backend не запущен |
| DB pool metrics        | SKIPPED | Backend не запущен |
| Web-admin build        | FAILED  | `npm` не в PATH; `.tools/node` — node v22.14.0 OK, но `npm-cli.js` / `npm-prefix.js` отсутствуют |

### Recovery blockers (this run)

1. **Docker Desktop not running** — `failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine`
2. **Low disk space** — ~2.87 GB free on project drive; warning from `docker-readiness`
3. **Broken portable Node** — `.tools/node/node.exe` работает, но npm-модули неполные; нужен `make setup-node`

### Next minimal action

1. Запустить Docker Desktop и дождаться статуса Running
2. `make docker-clean-safe` (когда Docker работает; без volume prune)
3. `make setup-node` → `make install-web-admin` → `make build-web-admin`
4. Повторить runtime chain (platform-up → migrate-up → health-check → metrics)

---

## Frontend Backend Status UX Pack v0.1

Дата: 2026-06-19  
Scope: web-admin UX — mock mode vs backend availability (no backend business logic changes).

| Item                            | Status      | Notes                             |
| ------------------------------- | ----------- | --------------------------------- |
| useBackendStatus                | OK          | GET /health with 3s timeout       |
| BackendStatusBanner             | OK          | online/offline + mock mode badges |
| AppShell integration            | OK          | banner under header               |
| Login backend status            | OK          | status + refresh                  |
| Dashboard offline UX            | OK          | no crash, explicit message        |
| Control Tower offline UX        | OK          | no crash, explicit message        |
| useApi network errors           | OK          | BACKEND_UNAVAILABLE               |
| i18n ru/en/zh                   | OK          | backendStatus.* keys              |
| docs/FRONTEND_BACKEND_STATUS.md | OK          | created                           |
| npm run build                   | OK          | frontend build passed             |
| Backend business logic          | NOT CHANGED | confirmed                         |

---

*Generated by Runtime Verification Pack v0.1. Re-run after Docker/WSL recovery.*

---

## Runtime Continue Check v0.2

Дата: 2026-06-20  
Путь проекта: `D:\Projects\freight-platform` (правильный корень; `C:\Users\Пользователь\freight-platform` не использовался)

| Check                                     | Status  | Notes |
| ----------------------------------------- | ------- | ----- |
| Project root D:\Projects\freight-platform | OK      | Makefile, README.md, services/, apps/, infrastructure/, docs/ присутствуют |
| project-map                               | OK      | docs/PROJECT_MAP.md и связанные документы перечислены |
| python-check                              | OK      | Python 3.12.10; `make python-check` exit 0 |
| docker-readiness                          | FAILED  | Docker CLI 29.5.3 установлен; daemon недоступен: `failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine: The system cannot find the file specified` — Docker Desktop is not running or unavailable |
| ports-check                               | OK      | 11 free, 1 in use (3000 web-admin); backend-порты 5432, 8080–8087, 9090 — FREE |
| platform-up                               | SKIPPED | Docker daemon недоступен — не запускали |
| platform-ps                               | SKIPPED | Docker daemon недоступен |
| migrate-up                                | SKIPPED | Docker daemon недоступен |
| health-check                              | SKIPPED | Backend не запущен |
| seed-dev-admin                            | SKIPPED | Backend не запущен |
| db-metrics-check                          | SKIPPED | Backend не запущен |
| db-pool-metrics-check                     | SKIPPED | Backend не запущен |
| web-admin build                           | FAILED  | `npm` не в PATH; `.tools/node/node.exe` отсутствует на `D:\Projects\freight-platform` |

### Docker error (this run)

```
failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine
The system cannot find the file specified.
Docker Desktop is not running or unavailable
```

Disk free (project drive): 326.08 GB — OK.

### Frontend build error (this run)

```
npm : The term 'npm' is not recognized ...
```

Portable Node не установлен в `.tools/node` на диске `D:`.

### Next minimal action (v0.2)

1. Запустить **Docker Desktop** и дождаться статуса Running (при проблемах WSL: `wsl --shutdown`, затем снова Docker Desktop).
2. Повторить: `make docker-readiness` → `make ports-check`.
3. Если OK: `make platform-up` → `make platform-ps` → `make migrate-up` → `make health-check` → `make seed-dev-admin` → metrics chain → `make setup-node` → `npm run build` в `apps/web-admin`.
4. Destructive cleanup и `docker volume prune` **не выполнять**.

---

## Docker Serial Build Pack v0.1

Дата: 2026-06-20  
Путь: `D:\Projects\freight-platform`

| Item                         | Status | Notes |
| ---------------------------- | ------ | ----- |
| platform-build-serial        | OK     | Makefile target added; builds `BACKEND_SERVICES` one at a time with `COMPOSE_PARALLEL_LIMIT=1` |
| platform-up-no-build         | OK     | Makefile target added; `docker compose up -d --no-build` |
| platform-up-safe             | OK     | Makefile target added; `platform-build-serial` + `platform-up-no-build` |
| platform-build-service       | OK     | Makefile target added; `make platform-build-service SERVICE=name` |
| docs/DOCKER_WSL_STABILITY.md | OK     | Created with EOF/WSL workflow and runtime chain |
| README link                  | OK     | Windows safe startup section + docs table link |

### Context (unchanged business code)

- Docker/WSL crashed during parallel `make platform-up` (`EOF`, `0xc00000fd`).
- Sequential build is recommended on Windows via `make platform-up-safe`.
- Backend business logic unchanged; only Makefile/dev-experience updates.

### Build verification (this pack)

| Service | Status | Notes |
| ------- | ------ | ----- |
| document-service | FAILED | `make platform-build-service SERVICE=document-service` — Docker Desktop unable to start (`0xc00000fd`) |
| billing-register-service | SKIPPED | Docker daemon down after document-service attempt |
| api-gateway | SKIPPED | Docker daemon down |
| platform-up-no-build | SKIPPED | Docker daemon down |

Makefile targets verified from PowerShell + GnuWin32 make (recipe runs; Docker blocked builds).

### Recommended next runtime chain

```powershell
cd D:\Projects\freight-platform
make docker-readiness
make platform-build-service SERVICE=document-service
make platform-build-service SERVICE=billing-register-service
make platform-build-service SERVICE=api-gateway
make platform-up-no-build
make platform-ps
make migrate-up
make health-check
make seed-dev-admin
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
```

---

## Docker Recovery Checkpoint (2026-06-20)

Снимок состояния после Serial Build Pack v0.1 и попыток восстановления Docker Desktop.

### Текущий блокер

| Проверка | Status | Notes |
| -------- | ------ | ----- |
| `docker version` → Server | FAILED | `Docker Desktop is unable to start` |
| WSL error | FAILED | `exit status 0xc00000fd` (stack overflow) |
| WSL proxy warning | ACTIVE | `localhost proxy detected but not mirrored in WSL (NAT mode)` |
| `make docker-readiness` | FAILED | Daemon unavailable |
| `make platform-build-service SERVICE=document-service` | FAILED | Blocked by Docker daemon |

### Что уже сделано (код бизнес-логики не менялся)

- **Serial Build Pack v0.1:** Makefile targets + `docs/DOCKER_WSL_STABILITY.md` + README
- **Корень проекта:** `D:\Projects\freight-platform` (не `C:\Users\Пользователь\freight-platform`)
- **Python:** OK (3.12.10)
- **Disk:** OK (~326 GB free)
- **Ports:** backend-порты свободны; 3000 (web-admin) in use — некритично

### Образы (partial build до падения WSL)

| Service | Build status |
| ------- | ------------- |
| company-service | OK (built sequentially) |
| identity-service | OK |
| transport-order-service | OK |
| rfx-service | OK |
| shipment-service | OK |
| document-service | NOT BUILT |
| billing-register-service | NOT BUILT |
| api-gateway | NOT BUILT |

### Попытки восстановления Docker (manual)

1. Quit Docker Desktop
2. `wsl --shutdown`
3. `Restart-Service com.docker.service` (Admin)
4. Очистка `HTTP_PROXY` / `HTTPS_PROXY` в сессии
5. Отключение proxy в Windows Settings и Docker Desktop Settings
6. Повторный запуск Docker Desktop

**Результат:** Server block в `docker version` пока не появляется.

### Следующий шаг (когда Server появится)

```powershell
cd D:\Projects\freight-platform
docker version                    # должен быть блок Server
make docker-readiness
make platform-build-service SERVICE=document-service
make platform-build-service SERVICE=billing-register-service
make platform-build-service SERVICE=api-gateway
make platform-up-no-build
make platform-ps
make migrate-up
make health-check
make seed-dev-admin
make generate-db-metrics-traffic
make db-metrics-check
make db-pool-metrics-check
```

**Не выполнять:** `docker volume prune`, destructive cleanup.

### Связанные документы

- [DOCKER_WSL_STABILITY.md](./DOCKER_WSL_STABILITY.md)
- [DOCKER_DISK_TROUBLESHOOTING.md](./DOCKER_DISK_TROUBLESHOOTING.md)
- [WINDOWS_ENVIRONMENT.md](./WINDOWS_ENVIRONMENT.md)

---

## Runtime End-to-End Check v0.3

Дата: 2026-06-21  
Путь: `D:\Projects\freight-platform`

### 1. Grafana UI

| Check | Status | Notes |
| ----- | ------ | ----- |
| Grafana `/api/health` | OK | Grafana 11.2.2, database ok |
| Grafana login (admin/admin) | OK | HTTP 200, session cookie (см. docs/OBSERVABILITY.md) |
| Prometheus `/-/healthy` | OK | HTTP 200 |
| `make observability-up` | OK | Prometheus + Grafana running |

URLs:
- Grafana: http://localhost:3001 (admin / admin)
- Prometheus: http://localhost:9090

### 2. Windows smoke test fixes (`tests/integration/smoke-test.sh`)

| Fix | Status | Notes |
| --- | ------ | ----- |
| TENANT_ID psql parsing | OK | `psql -q` + `grep -Eo UUID` (Windows `INSERT 0 1` no longer corrupts UUID) |
| Git Bash docker wrapper | OK | `docker_cmd()` uses `docker.exe` on Windows |
| TENANT_ID from env | OK | Optional override via `TENANT_ID=` |

### 3. Integration smoke test (after fixes)

| Check | Status | Notes |
| ----- | ------ | ----- |
| Service health (8081–8087) | OK | All 7 services |
| Tenant / companies / user | OK | Valid UUID tenant |
| Full business flow (TO → RFX → shipment → document → billing) | OK | Through billing register approved |
| UPD creation | FAILED | `POST .../upd` HTTP 400 — API/business validation, not Windows/env |
| `make integration-smoke-test` | PARTIAL | Fails at UPD step |
| `make full-flow-smoke-test` | PARTIAL | Same script alias |

**Windows env blocker resolved.** Remaining smoke failure is at billing UPD endpoint (HTTP 400), not Docker/WSL.

### 4. Full runtime stack status

| Component | Status | Notes |
| --------- | ------ | ----- |
| Backend platform (9 containers) | OK | Serial build + `platform-up-no-build` |
| `make health-check` | OK | All 8 services |
| DB metrics / pool metrics | OK | Verified earlier in session |
| Dev admin login | OK | `admin@7rights.local` (created via API when seed duplicate) |
| web-admin `npm run build` | OK | Node 24.17.0 |
| web-admin dev server | OK | http://localhost:3000 |
| Observability | OK | Prometheus + Grafana |
| Docker Serial Build Pack v0.1 | OK | Makefile targets + docs |

### 5. Known issues (non-blocking for local dev)

1. **`make seed-dev-admin`** — fails if tenant row exists with different `code` (PK conflict on `core.tenants.id`); use Git Bash or API seed workaround.
2. **`make metrics-check`** — requires `curl` in Makefile shell on Windows (GnuWin32); endpoints OK via PowerShell.
3. **Integration smoke UPD step** — HTTP 400 at billing-register UPD creation (needs separate API investigation; business code unchanged in this session).
4. **Docker Desktop stability** — may need restart after long sessions; use `make platform-up-safe` on Windows.

### Recommended next actions

1. Open Grafana dashboards at http://localhost:3001
2. Investigate billing UPD HTTP 400 in smoke test (optional; not env fix)
3. Fix `ensure_tenant` in `seed_dev_admin.sh` for existing tenant PK (optional dev script fix)

---

## Runtime End-to-End Check v0.4

Дата: 2026-06-21  
Путь: `D:\Projects\freight-platform`

### 1. Integration smoke test — **PASSED**

```powershell
& "C:\Program Files\Git\bin\bash.exe" -lc "cd /d/Projects/freight-platform && make integration-smoke-test"
# SMOKE TEST PASSED
```

| Fix (test infra only) | Notes |
| --------------------- | ----- |
| Unique vehicle plate / driver license per run | Avoids HTTP 409 on repeated runs (`SMK-${SMOKE_RUN_ID}`, `LIC-${SMOKE_RUN_ID}`) |
| `api_request` POST body via stdin | `printf \| curl --data-binary @-` — Windows cmdline mangled UTF-8 in `-d "$data"` |
| UPD `function_code` via `jq` unicode escapes | `\u0421\u0427\u0424\u0414\u041e\u041f` (= `СЧФДОП`) — script file encoding safe |

**Root cause UPD HTTP 400:** not API/business logic — Git Bash + `curl -d` on Windows corrupts Cyrillic (and multiline JSON args). Billing API accepts UPD when body is sent as UTF-8 (verified via PowerShell and piped curl).

### 2. Stack status

| Component | Status |
| --------- | ------ |
| Backend (9 containers) | OK |
| `make health-check` | OK |
| `make integration-smoke-test` | **OK** (full flow through billing close) |
| Grafana / Prometheus | OK |
| web-admin dev | OK — http://localhost:3000 (`npm run dev`) |

### 3. Remaining optional items

1. **`make seed-dev-admin`** — tenant PK conflict when row exists with different `code`
2. **`make metrics-check`** — needs `curl` in GnuWin32 make shell
3. **Docker Desktop** — use `make platform-up-safe` after long sessions

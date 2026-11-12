# BINTRANS — отказ инфраструктуры CI 11 ноября 2026

STAGE: `BINTRANS-CI-INFRA-FAILURE-0.1A`  
AGENT: `A`  
BASE: `3f306fc407aeda4bdc652d825954292f4a54fad6`  
MODE: `READ_ONLY_DIAGNOSIS`  
CI_WORKFLOW_CHANGED: `NO`

Этот отчёт фиксирует причину одинакового отказа GitHub Actions на PR #349–#354. Workflow не изменён. PR #352 не менялся. Локальная сборка Docker-образа E2E этим отчётом не подтверждается.

## 1. Вывод

`ROOT_CAUSE=CANONICAL_DNS_AZURE_ARCHIVE_UBUNTU_COM`

На hosted runners имя `azure.archive.ubuntu.com` не резолвилось. `apt` поэтому не скачал индексы и пакеты Ubuntu и завершился с кодом 100. Двенадцать jobs с `pnpm exec playwright install --with-deps chromium` упали на том же DNS внутри установки системных шрифтов и завершились с кодом 1. Это не ошибка продукта, не ошибка checksum и не локальный TLS timeout Docker.

`packages.microsoft.com` на тех же runners в ту же минуту открывался по HTTPS и отдавал `InRelease`. Сеть раннера в целом работала. Не работало разрешение имени зеркала Ubuntu в Azure.

Публичная запись того же окна: [actions/runner-images#15296](https://github.com/actions/runner-images/issues/15296). Сбой начался около 13:48 UTC 11 ноября 2026 и затронул hosted Ubuntu 22.04, 24.04 и 26.04. Комментарий GitHub в 18:14 UTC сообщил, что Azure mirror снова в норме. Наши шесть прогонов стартовали в 14:06–14:37 UTC, внутри этого окна.

## 2. Проверенные прогоны

У каждого PR база `3f306fc407aeda4bdc652d825954292f4a54fad6`. У текущего head ровно одна попытка CI. Во всех шести попытках 75 jobs, 62 success, 13 failure.

| PR | Head | Run | Старт UTC | URL |
|---|---|---|---|---|
| #349 | `07a1fb0f06b93ac34514f465d13ad8f28a1f02e1` | `43036487315` | 2026-11-11T14:13:59Z | https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/43036487315 |
| #350 | `cf4ecad0998a39bdd13e8bb18443c6b91dacbe48` | `43035316447` | 2026-11-11T14:06:40Z | https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/43035316447 |
| #351 | `14f9e53150ef724c756c0000baa35506af5b15e2` | `43036396166` | 2026-11-11T14:13:26Z | https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/43036396166 |
| #352 | `844fe235e771365b17d9dcd908a539a3be661b51` | `43036698967` | 2026-11-11T14:15:18Z | https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/43036698967 |
| #353 | `0918fee4e833cb71eb41dabee166b7b92ab83ff4` | `43037223049` | 2026-11-11T14:18:35Z | https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/43037223049 |
| #354 | `0762cfacefd1b08291ecbc8b94dbb93ed68df274` | `43040257256` | 2026-11-11T14:37:26Z | https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/43040257256 |

У PR #349 есть более ранний прогон `43033917703` на предыдущем head `d8ab9c67f6b08963b70f2b13b60106eb1a86c02d`, старт 13:57 UTC. У него тоже 75 jobs, 62 success, 13 failure, и упавшие шаги называются так же. Полный текст логов снят для шести текущих head, не для этого предыдущего SHA.

Одинаковый набор из 13 jobs на каждом текущем head:

- `repository-safety`, шаг `Install ripgrep for naming gate`
- `frontend-web-carrier-check`
- `frontend-web-shipper-check`
- `freight-cost-intelligence-browser-e2e`
- `rfx-buyer-xlsx-browser-e2e`
- `rfx-buyer-xlsx-create-browser-e2e`
- `rfx-carrier-response-browser-e2e`
- `rfx-carrier-xlsx-browser-e2e`
- `rfx-e7-browser-acceptance`
- `rfx-human-provenance-browser-e2e`
- `rfx-late-submission-browser-e2e`
- `rfx-scoring-v3-browser-e2e`
- `rfx-studio-browser-e2e`

Последние двенадцать падают на шаге `Install Playwright Chromium`.

В `.github/workflows/ci.yml` на этом base ровно 13 обращений к apt: одна строка `sudo apt-get update && sudo apt-get install -y ripgrep` и двенадцать строк `pnpm exec playwright install --with-deps chromium`. Других `apt-get` в workflow нет. Упали ровно эти 13 jobs. Остальные 62 jobs apt не вызывают и завершились успехом.

## 3. Что написано в логах

Скачаны и прочитаны логи всех 78 упавших jobs шести текущих head. В каждом логе единственное нерезолвящееся имя — `azure.archive.ubuntu.com`. Других имён в строках `Temporary failure resolving` нет.

Образ во всех 78 логах: label `ubuntu-latest`, фактический image `ubuntu-24.04`, release tag `ubuntu24/20261101.360`, строка версии образа `20261101.360.1`. Runner version `2.338.2`. Hosted Compute Agent `20261028.624`. Регионы разные: `centralus`, `eastus`, `eastus2`, `eastus3`, `northcentralus`, `southcentralus`, `westcentralus`, `westus`, `westus2`, `westus3`. Это не один сломанный регион.

Источники apt на раннере:

- метод зеркала `file:/etc/apt/apt-mirrors.txt`
- первое зеркало `http://azure.archive.ubuntu.com/ubuntu` для `noble`, `noble-updates`, `noble-backports`, `noble-security`
- последующие попытки apt показывает как `archive.ubuntu.com` и `security.ubuntu.com`, но текст ошибки резолвера остаётся `Temporary failure resolving 'azure.archive.ubuntu.com'`
- `https://packages.microsoft.com/ubuntu/24.04/prod` на каждом из 78 jobs успел отдать `InRelease`

`repository-safety` на PR #349, job `130576557749`, регион `eastus2`:

```text
Get:1 file:/etc/apt/apt-mirrors.txt Mirrorlist [144 B]
Get:6 https://packages.microsoft.com/ubuntu/24.04/prod noble InRelease [3600 B]
Ign:2 http://azure.archive.ubuntu.com/ubuntu noble InRelease
Err:2 https://security.ubuntu.com/ubuntu noble InRelease
  Temporary failure resolving 'azure.archive.ubuntu.com'
W: Failed to fetch mirror+file:/etc/apt/apt-mirrors.txt/dists/noble/InRelease  Temporary failure resolving 'azure.archive.ubuntu.com'
E: Failed to fetch https://security.ubuntu.com/ubuntu/pool/universe/r/rust-ripgrep/ripgrep_14.1.0-1_amd64.deb  Temporary failure resolving 'azure.archive.ubuntu.com'
E: Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?
##[error]Process completed with exit code 100.
```

`apt-get update` при этой ошибке только предупреждает и продолжает работу по старому индексу. Падает уже `apt-get install`. Код 100 — код apt, не код теста.

`rfx-carrier-response-browser-e2e` на том же PR доходит до `pnpm exec playwright install --with-deps chromium`. `libnss3` на образе уже стоит. Падение происходит на скачивании шрифтов:

```text
libnss3 is already the newest version (2:3.98-1ubuntu0.3).
E: Failed to fetch https://security.ubuntu.com/ubuntu/pool/universe/f/fonts-ipafont/fonts-ipafont-gothic_00303-21ubuntu1_all.deb  Temporary failure resolving 'azure.archive.ubuntu.com'
E: Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?
##[error]Process completed with exit code 1.
```

Код 1 здесь — обёртка Playwright вокруг неудачного `apt`. До скачивания бинарника Chromium шаг не доходит. Та же DNS-строка есть в остальных одиннадцати Playwright jobs каждого из шести PR.

## 4. Отдельно от локального Docker TLS

Локальный отказ `docker build` на машине Cursor — другой контур.

| | GitHub Actions 11 ноября | Локальный Docker на машине Cursor |
|---|---|---|
| Где | Hosted runner, Azure | Docker Desktop на рабочей станции |
| Симптом | `Temporary failure resolving 'azure.archive.ubuntu.com'` | `net/http: TLS handshake timeout` к `registry-1.docker.io`, затем к `proxy.golang.org` и `sum.golang.org` |
| Что при этом работало | HTTPS к `packages.microsoft.com` | С хоста `proxy.golang.org` и `sum.golang.org` отвечали HTTP 200 |
| Дополнительный локальный факт | Нет | Запрос хоста к `https://registry-1.docker.io/v2/` получил `CRYPT_E_REVOCATION_OFFLINE (0x80092013)` |
| Прокси | Не используется workflow | Демон ранее сообщал `http.docker.internal:3128` |

12 ноября демон Docker на этой машине не запущен, повторная сборка образа не выполнялась. `DOCKER_BUILD` для PR #352 остаётся `BLOCKED` и не считается доказанным. Проверка TLS, `GOSUMDB` и checksum не отключались.

## 5. Штатный rerun без пустого коммита

Механизм есть и пустой коммит не требует:

```text
gh run rerun 43035316447 --repo f9999737292-bit/Freihgt-platform --failed
gh run rerun 43036396166 --repo f9999737292-bit/Freihgt-platform --failed
gh run rerun 43036487315 --repo f9999737292-bit/Freihgt-platform --failed
gh run rerun 43036698967 --repo f9999737292-bit/Freihgt-platform --failed
gh run rerun 43037223049 --repo f9999737292-bit/Freihgt-platform --failed
gh run rerun 43040257256 --repo f9999737292-bit/Freihgt-platform --failed
```

`--failed` перезапускает только упавшие jobs и их зависимости на том же head SHA. Текущая учётная запись `gh` имеет scope `repo`, этого достаточно для rerun. В этой стадии команды не выполнялись: диагноз read-only, а PR #352 должен остаться без нового прогона и без нового коммита.

Повторять rerun стоит после того, как диагностический пробник ниже резолвит `azure.archive.ubuntu.com`. Повтор во время того же DNS-окна снова даст код 100.

## 6. Диагностический тест без продуктового кода

Тест не добавлен в репозиторий. Его место — отдельный `workflow_dispatch`, не правка `ci.yml`, и только после решения контроллера. На `ubuntu-latest` достаточно одного шага:

```bash
set +e
. /etc/os-release
echo "image=$VERSION"
echo "--- mirrors ---"
cat /etc/apt/apt-mirrors.txt
echo "--- sources ---"
cat /etc/apt/sources.list
ls -l /etc/apt/sources.list.d
for host in azure.archive.ubuntu.com archive.ubuntu.com security.ubuntu.com packages.microsoft.com; do
  if getent hosts "$host"; then
    echo "DNS_OK $host"
  else
    echo "DNS_FAIL $host"
  fi
done
sudo apt-get update
echo "APT_UPDATE_EXIT=$?"
```

Пробник зелёный, когда `azure.archive.ubuntu.com` резолвится и `apt-get update` заканчивается нулём. Он красный с `DNS_FAIL azure.archive.ubuntu.com`, когда повторяется инцидент 11 ноября. Продуктовые тесты, Playwright и Go-модули для этого не нужны.

## 7. План минимального исправления

Сейчас workflow менять не нужно. Инцидент по записи GitHub в 18:14 UTC 11 ноября снят. Ближайшее действие — rerun упавших jobs из раздела 5, после зелёного пробника из раздела 6.

Постоянная правка `ci.yml` требует отдельного решения контроллера. Если такое решение будет, минимальная правка, которая не ослабляет проверки:

1. Один общий шаг перед существующими 13 apt-вызовами. Сначала `getent hosts azure.archive.ubuntu.com`.
2. Если имя резолвится, источники apt не трогать.
3. Если не резолвится, временно добавить HTTPS-зеркало вне зоны `ubuntu.com`. Индексы и пакеты по-прежнему проверять ключом Ubuntu `/usr/share/keyrings/ubuntu-archive-keyring.gpg`. Поле `Signed-By` оставить. `Acquire::https::Verify-Peer` не выключать. HTTP без подписи не использовать.
4. `GOSUMDB`, checksum Go и TLS verification не отключать. Эти проверки к отказу apt не относятся.
5. Не переносить jobs на другую версию Ubuntu ради этого сбоя. Тот же DNS ломал `ubuntu-24.04` с образом `20261101.360`, а именно он и выполнял эти jobs.
6. Не считать `Acquire::Retries` достаточным исправлением. Во время окна одно и то же имя не резолвилось несколько минут, и смена label на `ubuntu-24.04` у других репозиториев дала ту же ошибку.
7. Не убирать `--with-deps` в этой стадии. На образе уже был `libnss3`, а apt падал на пакетах шрифтов. Отказ от `--with-deps` уберёт apt, но изменит состав браузерного окружения. Это отдельное решение, не обход DNS.

## 8. Что не делалось

- `.github/workflows/ci.yml` не менялся.
- PR #352 остаётся Draft. Head по-прежнему `844fe235e771365b17d9dcd908a539a3be661b51`, один коммит, merge не выполнялся.
- `gh run rerun` не запускался.
- `run-session.sh --execute`, deploy и повторный `docker build` не запускались.
- Личный токен для расширения прав GitHub не использовался.

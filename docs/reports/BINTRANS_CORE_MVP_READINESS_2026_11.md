# BINTRANS Core MVP Readiness — 2026-11-10

STAGE: `BINTRANS-CORE-VS-FULL-READINESS-0.2A`  
BASE: `9d577fe1459a715a076ba065568637300b2e40da`  
SOURCE: `docs/reports/BINTRANS_FULL_PROJECT_READINESS_2026_11.md`  
MODE: read-only. Этот файл не меняет баллы 14 модулей принятого аудита.

## 1. Что оценивается

Core MVP — путь одного грузовладельца и ограниченного круга перевозчиков:

1. TMS.
2. RFx / Procurement.
3. Tracking.
4. Shipper Portal.
5. Carrier Portal.

Shipper Portal и Carrier Portal посчитаны отдельно. Показатель Customer Portals из матрицы 14 модулей (Score 27) сюда не копируется. Смешанные 34% полной платформы сюда не переносятся.

Официальная готовность Core MVP: `NOT_ESTIMABLE`. Все числа ниже — `EXPLICIT_EXPERT_ESTIMATE`.

## 2. Граница доказательств

Проверено на этом SHA:

- Принятый отчёт PR #347 находится в `main`.
- От `f58f13b5` до этого SHA изменился только тот отчёт. Продуктовый код не менялся, поэтому новые PASS не появляются из факта merge.
- Post-merge CI run `42701751576`, event `push`, head `9d577fe1`, conclusion `success`, 75 jobs success. Отдельные логи jobs в этой оценке заново не читались.
- Открытые PR на 2026-11-10: 17. Открытые Issues: 6. PR #347 merged и в открытый список не входит.
- Draft PR #340, #341, #346 в готовность `main` не входят.
- Живой staging, production, backup и restore не опрашивались. Локальные тесты этой оценкой не запускались.

## 3. Шкала одной ячейки

| Балл | Критерий |
|---|---|
| 0 | Кода нет |
| 20 | Каркас или документ |
| 40 | Частичный production-код, крупный разрыв |
| 60 | Существенный код и часть CI, известный разрыв остаётся |
| 80 | Код и интеграционный CI закрывают ядро среза, остаток операционный |
| 100 | Не ставится: полный live E2E и staging не подтверждены |

Промежуточные значения используются только там, где ячейка принятой матрицы уже была между ступенями либо срез уже, чем модуль. Каждая такая ячейка названа ниже.

Итог направления:

`DirectionScore = 0.25F + 0.15UI + 0.20I + 0.15T + 0.15S + 0.10O`

Формула аналитическая, та же, что в принятом аудите. Она не утверждена как официальная метрика.

## 4. Пять направлений

| Направление | F | UI | I | T | S | O | DirectionScore |
|---|---:|---:|---:|---:|---:|---:|---:|
| TMS | 55 | 30 | 50 | 45 | 25 | 10 | 39.75 |
| RFx / Procurement | 70 | 65 | 55 | 70 | 40 | 20 | 56.75 |
| Tracking | 45 | 15 | 35 | 30 | 20 | 10 | 29.00 |
| Shipper Portal | 40 | 45 | 25 | 25 | 25 | 10 | 30.25 |
| Carrier Portal | 45 | 50 | 25 | 25 | 25 | 10 | 32.25 |

Проверка сумм:

- TMS: `0.25*55 + 0.15*30 + 0.20*50 + 0.15*45 + 0.15*25 + 0.10*10 = 39.75`.
- RFx: `0.25*70 + 0.15*65 + 0.20*55 + 0.15*70 + 0.15*40 + 0.10*20 = 56.75`.
- Tracking: `0.25*45 + 0.15*15 + 0.20*35 + 0.15*30 + 0.15*20 + 0.10*10 = 29.00`.
- Shipper: `0.25*40 + 0.15*45 + 0.20*25 + 0.15*25 + 0.15*25 + 0.10*10 = 30.25`.
- Carrier: `0.25*45 + 0.15*50 + 0.20*25 + 0.15*25 + 0.15*25 + 0.10*10 = 32.25`.

Почему ячейки не равны принятой матрице:

- TMS F 55, не 60: отдельный CI job назначения перевозчика не найден. UI 30, не 35: порталы вынесены из TMS. T 45, не 55: контракт stop-event не покрывает назначение. S 25, не 40: `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` лежит внутри этого среза и остаётся OPEN. O 10, не 25: стенд не проверялся.
- RFx F/UI/I/T совпадают с модулем C (70/65/55/70). S 40, не 45: авторизация внешней компании перевозчика не доказана. O 20, не 30: живой тендер не проверялся.
- Tracking ниже модуля G: Issue #344 блокирует именно consumer, а `apps/driver-mobile` без spec-файлов. UI 15.
- Shipper и Carrier не наследуют K=27.

Основания:

- TMS: `shipment-service`, `transport-order-service`, миграции `000088`–`000093`, job `order-execution-integration` входит в успешный граф 75.
- RFx: 181 тестовый файл `rfx-service` по принятому подсчёту, browser jobs `web-procurement`, job `rfx-award-to-transport-order-integration`.
- Tracking: `execution_event_consumer.go`, Issue #344 OPEN, PR #346 Draft и не является исправлением.
- Shipper: страницы `login`, `shipments`, `tracking`, `documents`. Страницы RFx нет.
- Carrier: страницы `login`, `tenders`, `transport-orders`, `fleet`, `shipments/.../tracking`, `shipments/.../documents`, компонент `CompanyGate.vue`. Форма `tenders/[id].vue` вызывает `createResponse` и `submitResponse`.

## 5. Два сводных расчёта

Равные веса пяти направлений, `ASSUMED_EQUAL_DIRECTION_WEIGHTS`:

`(39.75 + 56.75 + 29.00 + 30.25 + 32.25) / 5 = 188.00 / 5 = 37.6`

`CORE_BLENDED_EQUAL_WEIGHTS=37.6_PERCENT`. Это не 34% полной платформы и не официальная готовность.

Предлагаемые веса первого пилота, `PROPOSED_WEIGHTS_NOT_APPROVED`:

| Направление | Вес | Почему такой вес |
|---|---:|---|
| TMS | 0.30 | Заказ, перевозка и завершение — тело пилота |
| RFx / Procurement | 0.25 | Без тендера и award путь не начинается |
| Tracking | 0.20 | Pickup, delivery и видимость входят в обязательный путь |
| Shipper Portal | 0.15 | Вход грузовладельца и чтение статуса |
| Carrier Portal | 0.10 | Ставка и назначение; часть действий может идти через API, если UI доказан отдельно |

`0.30*39.75 + 0.25*56.75 + 0.20*29.00 + 0.15*30.25 + 0.10*32.25 = 39.675`

`CORE_BLENDED_PROPOSED_WEIGHTS=39.675_PERCENT`. Веса не утверждены. Число не является разрешением на пилот.

## 6. Сквозные показатели

Среднее одной колонки по пяти направлениям, равные веса:

| Показатель | Расчёт | Значение |
|---|---|---|
| `CORE_FUNCTIONAL_IMPLEMENTATION` | `(55+70+45+40+45)/5 = 51` | `51_PERCENT` |
| `CORE_UI_COMPLETENESS` | `(30+65+15+45+50)/5 = 41` | `41_PERCENT` |
| `CORE_INTEGRATION_READINESS` | `(50+55+35+25+25)/5 = 38` | `38_PERCENT` |
| `CORE_TEST_COVERAGE` | `(45+70+30+25+25)/5 = 39` | `39_PERCENT` |
| `CORE_SECURITY_READINESS` | `(25+40+20+25+25)/5 = 27` | `27_PERCENT` |
| `CORE_OPERATIONAL_READINESS` | `(10+20+10+10+10)/5 = 12` | `12_PERCENT` |
| `CORE_PILOT_READINESS` | Решение, не процент | `NO` для полноценного пилота. Pilot A ниже — только `CONDITIONAL` |

`CORE_COMMERCIAL_READY=NO`. Деньги, акты и юридический ЭДО в Core MVP не входят и на `main` не закрыты.

Успешный unit test и зелёный CI не считаются завершённым E2E. `CORE_E2E_VERIFIED=0` из 14 шагов как один живой путь.

## 7. Обязательный путь

`LIVE_STAGING=NOT_VERIFIED` для всех 14 строк. Browser E2E shipper и carrier в репозитории подменяет API через `route.fulfill` (`apps/web-shipper/e2e/shipper-office.spec.ts`, `apps/web-carrier/e2e/carrier-office.spec.ts`). Это не backend-to-backend и не staging.

| Шаг | Production-код | Настоящий UI | API | Integration test | Browser E2E | Переход B2B | Security blocker | Для первого пилота |
|---|---|---|---|---|---|---|---|---|
| Shipper login | Да, `pages/login.vue` вызывает `office.login` | Да | Клиент есть. Identity `CreateAccessToken` есть | Не как сквозной UI→identity | Mock | Не доказан | Полнота привязки пользователя к tenant не перепроверялась. Форма принимает `tenantId` | Вход на изолированном стенде против identity |
| Company selection | UI выбора есть в shipper e2e (`company-select`) и в carrier `CompanyGate.vue` | Да | Company-service есть | Полнота авторизации компании `NOT_VERIFIED` | Mock | Не доказан | Company authorization не доказана | Доказать, что компания A не читает данные компании B |
| RFx | Да, `rfx-service` | Да, `web-procurement`. В `web-shipper` страницы тендера нет | Да | Плотный CI RFx | Playwright procurement в CI, не shipper | Частично | Внешняя сеть перевозчиков не доказана | Автор тендера в этой поставке — procurement UI, не shipper |
| Carrier bid | Страница `tenders/[id].vue` вызывает `createResponse` и `submitResponse` | Да | Клиент `useCarrierApi` | RFx-тесты есть. Этот экран против живого API не доказан | Mock | Не доказан | Права перевозчика не доказаны | Одна ставка на стенде |
| Award | Сервис и job есть | Procurement UI, полнота экрана award в этом срезе не перечитывалась | Job в графе CI | `rfx-award-to-transport-order-integration` входит в 75 success. Лог job не перечитывался | Не как живой UI | Job утверждает переход | Нет отдельного нового blocker сверх tenant и прав | Сохранить этот job зелёным на SHA пилота |
| Transport order | `transport-order-service`, страницы carrier | Да, список и карточка | Да | `order-execution-integration` в графе 75. Лог не перечитывался | Mock на портале | Job есть, лог не перечитан | Tenant header, если заказ создаёт shipment | Переход виден на стенде |
| Shipment | `shipment-service` | Shipper list/detail | Да, с `X-Tenant-ID` | Часть execution CI | Mock | Не весь путь | `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` OPEN | Отдельный review границы до merge пилота |
| Carrier assignment | Код и миграции частичные. Отдельный обязательный job не найден | Carrier fleet и orders есть | `UNKNOWN` как обязательный контракт | `UNKNOWN` | Не доказан | `UNKNOWN` | Тот же tenant boundary | Либо тест с именем перехода, либо явное исключение |
| Driver / vehicle | `driver-mobile` partial, spec-файлов нет. PR #13 не merged | Fleet UI carrier есть. Native app не готова | Не доказан E2E | Нет связанного E2E | Нет | Нет | Персональные данные водителя | Для Pilot A допустим диспетчер в carrier portal без native app |
| Pickup | Контракт stop-event | Нет отдельного подтверждённого экрана водителя | Consumer есть | `TestStopEventTrackingContract09B` PASS в принятом аудите | Нет live | Контракт, не рейс | Issue #344 на боевом consumer | Событие на стенде либо честное исключение tracking |
| Delivery | Тот же контракт, включая `FAIL_DELIVERY_ACTION` | Не доказан как UI рейса | То же | То же | Нет live | То же | Issue #344 | То же |
| Tracking | Consumer и publisher есть. Провайдер `NOT_VERIFIED` | Страницы shipper и carrier | Да | Контракт Apply. Живой поток `NOT_RUN` | Mock | Publisher→consumer не прогонялся здесь | Issue #344 OPEN | Классификация постоянных ошибок до приёмки tracking |
| POD | Миграция `000099`, страницы documents | Список и вложение в тестах UI | Write path под Issue #343 | Не сквозной upload | Mock показывает `pod.pdf` | Не доказан | Issue #343, `POTENTIAL_IMPACT=HIGH`, exposure `NOT_ESTABLISHED` | Запись только после review границы |
| Completion | `000093` delivery disposition | Не доказан как экран завершения | Частично | Не полный рейс | Нет | Нет | Зависит от shipment и POD | Критерий приёмки назвать явно: статус, не устное «доехало» |

## 8. Security gates

Исключение функции из Core MVP не снимает зависимость, если путь её читает или пишет.

| Gate | Состояние на `main` | Блокер |
|---|---|---|
| Identity / JWT | `AuthService.CreateAccessToken` есть. Это выпуск токена, не доказательство на каждом порту | Условный: нужен стендовый вход |
| Tenant isolation | Платформенный `ENABLE ROW LEVEL SECURITY` в `infrastructure/migrations` не найден. Фильтры в сервисах есть, полный обзор `NOT_VERIFIED` | Не обходится ради скорости |
| Company authorization | UI выбора компании есть. Межкомпанийская проверка `NOT_VERIFIED` | P1 до реального пилота |
| Service-to-service | PR #336 закрыл analytics caller. Полнота остальных internal routes `NOT_VERIFIED` | P1 для сервисов пути |
| `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` | OPEN. `resolveVerifiedTenant` берёт `X-Tenant-ID`. Эксплуатация не воспроизводилась | P0. Не смешивать с RLS и с Issue #343 |
| Issue #343 | OPEN. Прямые write routes document-service | P0 для POD и любого внешнего document write |
| Issue #344 | OPEN. Постоянная ошибка inbox sequence ретраится | P0, если приёмка включает живой tracking |
| Права Shipper / Carrier | Middleware уводит на `/login`. Матрица ролей не доказана | P1 |
| POD | Код и миграция есть. Запись блокируется границей #343 | P0 вместе с #343 |
| Backup / restore | `NOT_VERIFIED` | P0 до загрузки данных, которые нельзя выбросить |

`CT_MUTATION_EQUALS_VIEW` остаётся OPEN. В обязательный путь грузовладельца Control Tower не входит. Если в пилоте оператор подтверждает события из роли просмотра, этот пункт становится блокером того пилота. Issue #345 test-only dependency пилот не блокирует. Issue #334 не закрывает #343.

24/7 Autopilot в Core MVP не входит. PR #320 на `main` — dry-run foundation. Issue #313 OPEN. `AUTOPILOT_24_7_READY=NO`. Техническая возможность не является разрешением на эксплуатацию.

## 9. Что сознательно вне Core MVP

Finance, юридический ЭДО, slot booking, consignee, forwarder, network optimizer, analytics KPI, Autopilot. Они остаются в полной платформе. Finance и ЭДО возвращаются как обязательные только в Commercial MVP.

## 10. Решения по запуску

Подробные сценарии — в `docs/reports/BINTRANS_CORE_VS_FULL_DECISION_2026_11.md`.

| Сценарий | Решение |
|---|---|
| Pilot A, контролируемый технический | `CONDITIONAL` |
| Pilot B, реальный заказчик | `NO-GO` |
| Commercial MVP | `NO-GO` |

Сегодня нет основания поставить GO даже для Pilot A: стенд не проверен, три границы (tenant header, #343, #344) открыты, backup не подтверждён.

## 11. Evidence register

| Утверждение | Источник | Дата |
|---|---|---|
| main SHA | `git rev-parse origin/main` | 2026-11-10 |
| Только отчёт между `f58f13b5` и HEAD | `git diff --name-only` | 2026-11-10 |
| Post-merge CI 75 success | run `42701751576` | 2026-11-10 |
| Открытые PR 17, Issues 6 | `gh pr list`, `gh issue list` | 2026-11-10 |
| 14-модульная матрица и 34% | принятый отчёт, раздел 5 | 2026-11-09, файл в этом SHA |
| Shipper login и tenant field | `apps/web-shipper/pages/login.vue` | 2026-11-10 |
| Shipper pages без RFx | `apps/web-shipper/pages` | 2026-11-10 |
| Carrier bid UI | `apps/web-carrier/pages/tenders/[id].vue` | 2026-11-10 |
| Mock browser | `e2e/shipper-office.spec.ts`, `e2e/carrier-office.spec.ts` | 2026-11-10 |
| Award и execution jobs | `.github/workflows/ci.yml` имена jobs | 2026-11-10 |
| Tenant header, #343, #344, RLS, Autopilot | принятый отчёт, разделы 11, 18, 23 | этот SHA |
| Draft #340 #341 #346 | `gh pr list` | 2026-11-10 |
| Живые серверы | не проверялись | `NOT_VERIFIED` |

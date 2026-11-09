# BINTRANS Full Platform Readiness — 2026-11-10

STAGE: `BINTRANS-CORE-VS-FULL-READINESS-0.2A`  
BASE: `9d577fe1459a715a076ba065568637300b2e40da`  
SOURCE: `docs/reports/BINTRANS_FULL_PROJECT_READINESS_2026_11.md`

Эта оценка не заменяет 14-модульную матрицу. Она проверяет, можно ли её сдвинуть на актуальном `main`.

## 1. Изменение с прошлого аудита

Между `f58f13b5b4b539895673a77ff404ab47669dc96d` и `9d577fe1459a715a076ba065568637300b2e40da` в git один файл: `docs/reports/BINTRANS_FULL_PROJECT_READINESS_2026_11.md`.

Post-merge CI run `42701751576` завершился success, 75/75, на merge SHA. Это повтор уже известного графа после docs-only merge. Нового продуктового доказательства он не добавляет.

Готовность полной платформы из-за этого merge не повышается. 84 экспертных балла не пересматривались.

## 2. Четырнадцать направлений

Классы модуля, без изменений: `IMPLEMENTED=0`, `PARTIAL=13`, `DESIGNED_ONLY=1` (slot booking).

| Модуль | F | UI | I | T | S | O | Score | Класс |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| A Foundation | 65 | 45 | 55 | 50 | 35 | 30 | 50 | PARTIAL |
| B TMS | 60 | 35 | 50 | 55 | 40 | 25 | 47 | PARTIAL |
| C RFx | 70 | 65 | 55 | 70 | 45 | 30 | 59 | PARTIAL |
| D Rates and freight cost | 60 | 40 | 50 | 55 | 40 | 25 | 48 | PARTIAL |
| E Finance | 45 | 10 | 40 | 40 | 30 | 15 | 33 | PARTIAL |
| F EDO / documents | 40 | 25 | 30 | 40 | 25 | 15 | 31 | PARTIAL |
| G Tracking / driver | 50 | 20 | 40 | 35 | 30 | 15 | 35 | PARTIAL |
| H Control Tower | 50 | 40 | 45 | 50 | 30 | 25 | 42 | PARTIAL |
| I Network optimizer | 45 | 20 | 25 | 55 | 35 | 15 | 34 | PARTIAL |
| J Analytics | 25 | 15 | 20 | 15 | 25 | 10 | 20 | PARTIAL |
| K Customer portals | 30 | 35 | 25 | 30 | 25 | 10 | 27 | PARTIAL |
| L Slot booking | 15 | 10 | 10 | 5 | 10 | 5 | 10 | DESIGNED_ONLY |
| M Integrations | 25 | 15 | 20 | 25 | 20 | 10 | 20 | PARTIAL |
| N Autopilot | 30 | 5 | 15 | 25 | 20 | 5 | 19 | PARTIAL |

Сумма Score = 475. `475/14 = 33.928...`, принятое округление `34_PERCENT`.  
`BLENDED_READINESS_METHOD=ASSUMED_EQUAL_WEIGHTS`.  
`FULL_BLENDED_SCORE=34_PERCENT`.  
Это не функциональная готовность и не готовность Core MVP.

Сумма F = 610. `610/14 = 43.571...`, принятое `43.6_PERCENT`.  
`FULL_FUNCTIONAL_IMPLEMENTATION=43.6_PERCENT`.  
`FUNCTIONAL_F_EXPERT_AVERAGE_STATUS=ILLUSTRATIVE_ONLY`.

## 3. Средние колонок

Тоже `EXPLICIT_EXPERT_ESTIMATE` и `ILLUSTRATIVE_ONLY`. Они не являются новыми измерениями и не поднимают матрицу.

| Колонка | Сумма | Среднее |
|---|---:|---:|
| F | 610 | 43.6 |
| UI | 380 | 27.1 (`380/14 = 27.142...`) |
| I | 480 | 34.3 (`480/14 = 34.285...`) |
| T | 550 | 39.3 (`550/14 = 39.285...`) |
| S | 410 | 29.3 (`410/14 = 29.285...`) |
| O | 235 | 16.8 (`235/14 = 16.785...`) |

Проверка UI: `45+35+65+40+10+25+20+40+20+15+35+10+15+5 = 380`.  
I: `55+50+55+50+40+30+40+45+25+20+25+10+20+15 = 480`.  
T: `50+55+70+55+40+40+35+50+55+15+30+5+25+25 = 550`.  
S: `35+40+45+40+30+25+30+30+35+25+25+10+20+20 = 410`.  
O: `30+25+30+25+15+15+15+25+15+10+10+5+10+5 = 235`.

## 4. Раздельные показатели

| Показатель | Статус |
|---|---|
| `FULL_FUNCTIONAL_IMPLEMENTATION` | `43.6_PERCENT`, `ILLUSTRATIVE_ONLY` |
| `FULL_INTEGRATION_READINESS` | `NOT_ESTIMABLE`. Колонка I в среднем 34.3 является экспертной иллюстрацией, не подтверждением сквозных связей |
| `FULL_SECURITY_READINESS` | `NOT_ESTIMABLE`. Колонка S в среднем 29.3 не закрывает открытые границы |
| `FULL_OPERATIONAL_READINESS` | `NOT_ESTIMABLE`. Staging и production `NOT_VERIFIED` |
| `FULL_COMMERCIAL_READINESS` | `NOT_ESTIMABLE`. `COMMERCIAL_LAUNCH_READY=NO` |
| `OFFICIAL_OVERALL_READINESS` | `NOT_ESTIMABLE` |
| `FULL_PILOT_READY` | `NO` |
| `FULL_COMMERCIAL_READY` | `NO` |

Утверждённой методики весов 14 модулей нет. Пока её нет, официальный процент не ставится.

## 5. Пять уровней подтверждения

| Уровень | Что подтверждено | Что не подтверждено |
|---|---|---|
| Готово в коде | 17 Go-сервисов с `cmd/server`, 7 frontend-приложений, миграции `000001`–`000105` | Slot booking как продукт, forwarder app, юридический оператор ЭДО, native driver store build |
| Подтверждено CI | Run `42701751576`, 75/75 success на этом SHA. Предыдущий продуктовый attempt 2 на `f58f13b5` тоже был success | Полнота графа с первой попытки не гарантирована: attempt 1 на прежнем SHA регистрировал 20 из 75 |
| Подтверждено staging | Нет в этой оценке | Хост, migration head базы, DNS, TLS, secrets, restore |
| Подтверждено пилотом | Нет | Живые компании и пользователи |
| Готово к production | Нет | Те же пробелы плюс backup, провайдеры, деньги, ЭДО, consumer #344 |

`FULL_E2E_VERIFIED=0` из 16 сценариев принятого раздела 21. Частичные CI jobs сохранены там и здесь не отменяются. `BLOCKED` и `NOT_RUN` по-прежнему пересекаются: 4 и 16 не складываются.

## 6. Особые объекты

| Объект | Состояние 2026-11-10 | В готовности `main` |
|---|---|---|
| PR #340 | OPEN Draft, NLO production adapter proof | Нет |
| PR #341 | OPEN Draft, snapshot-to-closing money identity | Нет |
| PR #346 | OPEN Draft, контракт permanent-error. Не исправление consumer | Нет |
| Issue #343 | OPEN, document write routes | Блокер внешнего document write и production |
| Issue #344 | OPEN. Узкая классификация `CORE-P0-003A` = `3–8 PERSON_DAYS`, `CONFIDENCE=LOW`. `CORE-P0-003B` и `CORE-P0-003C` = `NOT_ESTIMABLE`. `REBALANCE_DESIGN=BLOCKED`. PR #346 не чинит consumer | `PRODUCTION_CONSUMER_READINESS=BLOCKED`. 3–8 дней не закрывают issue |
| Issue #345 | OPEN LOW, test-only shipment→tracking import | Не блокер runtime по тексту принятого аудита |
| `CT_MUTATION_EQUALS_VIEW` | OPEN в контракте 0.9B | Блокер операторского пилота Control Tower |
| `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` | OPEN в принятом отчёте. Эксплуатация не воспроизводилась | Блокер external pilot и production для shipment-service. Не является #343 и не является RLS |
| Autopilot 24/7 | PR #320 merged как dry-run. Issue #313 OPEN. Draft #329 и #335 не на `main` | `AUTOPILOT_24_7_READY=NO`. В Core MVP не входит |

## 7. Autopilot отдельно

Подтверждено: dry-run foundation в истории `main`, SQL очереди в `tools/bintrans_autopilot`.  
Не подтверждено и этим заданием не разрешается: paid activation, auto-merge, auto-deploy, круглосуточный автономный режим.

Наличие worker foundation не является разрешением на эксплуатацию.

## 8. P3, развитие полной платформы

Приоритеты P0 и P1 пути грузовладельца живут в Core-отчёте и в решении. Здесь только то, что не требуется для первого Core-пилота.

`PARTIAL_ESTIMATED_SUBTOTAL=47_TO_112_PERSON_DAYS` в файле решения — сумма уже оценённых строк. `TOTAL_REMAINING_EFFORT_TO_PILOT=NOT_ESTIMABLE`. Из subtotal исключены: внешние tracking providers, интеграции ERP и WMS, routing providers, стоимость сторонних услуг, разработки дополнительного security audit, юридические работы вне инженерной описи, неоценённая часть Issue #344 (`CORE-P0-003B` и `CORE-P0-003C`). Этот редакционный split не повышает готовность платформы.

Трудозатраты — человеко-дни, не календарь. `NOT_ESTIMABLE` стоит там, где размер внешнего контура не измерялся.

| TASK_ID | MODULE | DESCRIPTION | CURRENT_EVIDENCE | DEPENDENCIES | ACCEPTANCE_CRITERIA | RECOMMENDED_AGENT | PR_OR_ISSUE | PRIORITY | EFFORT_RANGE | CONFIDENCE |
|---|---|---|---|---|---|---|---|---|---|---|
| FULL-P3-001 | Network optimizer | Довести production adapter proof до `main` | PR #340 Draft | Расчёт NLO уже на `main`. Исполнение перевозки — другой контур | Proof merged и CI job зелёный. Это не запуск маршрутизации в production | optimizer | #340 | P3 | `NOT_ESTIMABLE`. Diff Draft здесь не измерялся | LOW |
| FULL-P3-002 | Slot booking | Продукт записи слота | Модуль L, Score 10, DESIGNED_ONLY. UI вне скоупа документа slot intelligence | Исполнение и складской контур | Booking workflow существует и не сводится к `SlotFacts` | product + TMS | нет | P3 | `NOT_ESTIMABLE` | LOW |
| FULL-P3-003 | Customer portals | Consignee, finance UI, forwarder | Скелеты `index.vue` и отсутствие `apps/web-forwarder` | Модуль K PARTIAL | Три роли либо реализованы, либо явно исключены из обещания платформы | portal | нет | P3 | `NOT_ESTIMABLE` | LOW |
| FULL-P3-004 | Control Tower | Закрыть мутации роли просмотра | `CT_MUTATION_EQUALS_VIEW` OPEN | Не заменяет #343 | View role не может acknowledge, assign, resolve | security + CT | контракт 0.9B, #334 | P3 для Core-пути. P1, если пилот включает оператора CT | 3–8 человеко-дней на три названные мутации. Большая модель ролей этот диапазон превысит | MEDIUM на факт, LOW на потолок |
| FULL-P3-005 | Tracking tests | Убрать test-only импорт | Issue #345. Бинарники сервера пакет не импортируют | Нет | Тест живёт без импорта tracking из shipment server | tracking | #345 | P3 | 1–3 | MEDIUM |
| FULL-P3-006 | Control Tower | Три LOW read safety | Issue #334 | Не закрывает #343 | Follow-up закрыты и в тексте не числятся заменой #343 | CT | #334 | P3 | 1–3 | MEDIUM |
| FULL-P3-007 | Analytics | Источники OTIF и POD KPI | GAP-C-001, GAP-C-002, GAP-B-001 в принятом разделе 14 | Документы и tracking | KPI либо считаются из названного источника, либо сняты с витрины | analytics | нет | P3 | `NOT_ESTIMABLE` | LOW |
| FULL-P3-008 | Integrations | Боевые ERP, WMS, tracking provider, routing | Контракты есть, провайдеры `NOT_VERIFIED` | Внешние стороны | Подключение на стенде с именем провайдера и датой | integrations | нет | P3 | `NOT_ESTIMABLE` | LOW |
| FULL-P3-009 | Autopilot | 24/7 только после cost gates | Issue #313. Dry-run merged | Запрет активации этим заданием | Отдельное разрешение. Зелёный CI его не заменяет | autopilot | #313, #320 merged, #329 Draft, #335 Draft | P3 | `NOT_ESTIMABLE` | LOW |
| FULL-P3-010 | Finance and EDO | Коммерческий контур платформы | PR #341 Draft. GAP-B-001 open. `web-finance` скелет | Core path может идти без счетов | Деньги и юридические документы на `main` либо письменное исключение из оферты | finance + documents | #341 | P2 для Commercial, P3 для технической платформы | `NOT_ESTIMABLE` | LOW |

`FULL_P0_BLOCKERS` для запуска всей платформы шире Core: все Core P0 плюс отсутствие утверждённой коммерческой модели, юридического ЭДО и проверенной эксплуатации. Пока цель — не вся платформа, эти пункты не называются минимальным путём первого пилота.

## 9. Evidence register

| Утверждение | Источник | Дата |
|---|---|---|
| Матрица и суммы 475 и 610 | принятый отчёт, раздел 5, этот SHA | 2026-11-10 прочитан повторно |
| Diff только отчёта | `git diff --name-only f58f13b5 HEAD` | 2026-11-10 |
| CI | run `42701751576` | 2026-11-10 |
| PR #340 #341 #346 Draft | `gh pr list` | 2026-11-10 |
| Issues повторно | `gh issue list` 2026-11-10: #345, #344, #343, #337, #334, #313. Шесть. Не перенесено без проверки | 2026-11-10 |
| Снимок PR | `OPEN_PRS_BEFORE_PR348=17`. `OPEN_PRS_AFTER_PR348_CREATION=18`. Повторная длина списка 18, включает #348 | 2026-11-10 |
| Autopilot dry-run | принятый раздел 23, PR #320 | этот SHA |
| Колоночные суммы | сложение ячеек раздела 2 этого файла | 2026-11-10 |

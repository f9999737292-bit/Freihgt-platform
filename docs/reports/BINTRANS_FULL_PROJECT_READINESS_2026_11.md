# BINTRANS — состояние проекта на дату аудита

Дата проверки: **2026-11-09** (UTC).  
Аудируемый коммит: `f58f13b5b4b539895673a77ff404ab47669dc96d` (`origin/main`).  
Репозиторий: `f9999737292-bit/Freihgt-platform`.  
Режим: чтение исходников и метаданных GitHub. Серверы, SSH, Docker и миграции не запускались.

## 1. Executive Summary

BINTRANS сегодня — работающий в репозитории каркас многоарендной TMS/B2B-платформы: 17 Go-сервисов с `cmd/server`, семь frontend-приложений, миграции `000001`–`000105` без пропусков, и зелёный полный CI на этом SHA (run `42671916835`, attempt 2, 75/75 success). Это не доказательство коммерческой эксплуатации.

**Можно ли сегодня запускать BINTRANS с реальными клиентами?** Полноценный коммерческий запуск — нет. Ограниченный закрытый технический пилот отдельных уже покрытых CI контуров (RFx, часть TMS execution, freight settlement) возможен только после отдельного решения по staging, секретам, document-service write boundary (Issue #343) и tracking consumer (Issue #344). Этот аудит такое решение не даёт и живой staging не проверял.

| Вопрос | Ответ |
|---|---|
| Смешанная модульная оценка | `BLENDED_MODULE_READINESS=34_PERCENT`. `BLENDED_READINESS_METHOD=ASSUMED_EQUAL_WEIGHTS`. Это не функциональная готовность |
| Среднее экспертных F | `FUNCTIONAL_F_EXPERT_AVERAGE=43.6_PERCENT`. `FUNCTIONAL_F_EXPERT_AVERAGE_STATUS=ILLUSTRATIVE_ONLY`. Это не подтверждённая функциональная готовность |
| Официальная функциональная готовность | `OFFICIAL_FUNCTIONAL_READINESS=NOT_ESTIMABLE` |
| Официальная общая готовность | `OFFICIAL_OVERALL_READINESS=NOT_ESTIMABLE` |
| Коммерческая готовность | `COMMERCIAL_READINESS_PERCENT=NOT_ESTIMABLE`. `COMMERCIAL_LAUNCH_READY=NO` |
| Технический пилот | `CONDITIONAL`. Живой стенд в этом аудите `NOT_VERIFIED` |
| Коммерческий пилот | `NO` |
| Production | `NO` |
| Модули матрицы из 14 | 0 полностью готовых, 13 `PARTIAL`, 1 `DESIGNED_ONLY` (slot booking) |
| Каркасы внутри частичных модулей | consignee UI, finance UI, forwarder app, юридический оператор ЭДО, режим Autopilot 24/7 |
| Не начато после поиска | 0 пустых доменов. `apps/web-forwarder` отсутствует. Это function-level `DESIGNED_ONLY` внутри модуля K, не пятнадцатый модуль |
| Открытые критические проблемы запуска | 5 |
| Autopilot 24/7 | `NO`. PR #320 merged как dry-run foundation, не как автономный режим |

34% и 43,6% ниже — иллюстрации экспертной таблицы. Ни одно из этих чисел не является подтверждённой функциональной готовностью. Пометка confidence `MEDIUM` в матрице относится к качеству собранных технических свидетельств, а не к статистической точности этих процентов.

| Срез | Значение | Статус |
|---|---|---|
| Blended module mean | `34_PERCENT` | `ASSUMED_EQUAL_WEIGHTS`. Среднее шести показателей по 14 модулям. Не functional readiness |
| Expert F average | `43.6_PERCENT` | `ILLUSTRATIVE_ONLY`. Среднее только колонки F: 610/14. Исходные F не менялись |
| Official functional | `NOT_ESTIMABLE` | веса и обязательный набор функций не утверждены |
| Official overall | `NOT_ESTIMABLE` | то же |
| Integration | `NOT_ESTIMABLE` | отдельные CI jobs есть; полный путь из 16 сценариев не запускался |
| Security | `NOT_ESTIMABLE` | открытые границы есть, pentest не выполнялся |
| Operational | `NOT_ESTIMABLE` | живые серверы не опрашивались |
| Pilot | `NOT_ESTIMABLE` | документ walkthrough есть, живые пользователи не подтверждены |
| Commercial | `NOT_ESTIMABLE` | `COMMERCIAL_LAUNCH_READY=NO` |

Пять главных блокеров:

1. Issue #343: прямые write-маршруты document-service не закрыты как доверенная граница. Production rollout в тексте issue запрещён до оценки.
2. Issue #344: tracking consumer повторяет постоянную ошибку inbox sequence. `PRODUCTION_CONSUMER_READINESS=BLOCKED`.
3. Деньги: PR #341 (snapshot-to-closing money identity) всё ещё Draft и не в `main`.
4. Юридически значимый ЭДО-оператор и GAP-B-001 runtime не реализованы. Внутренняя подпись это не заменяет.
5. Живые staging/production и backup/restore в этом аудите не подтверждались. CI attempt 1 на том же SHA зарегистрировал только 20 из 75 jobs и получил `failure` при 20 success; attempt 2 восстановил граф.

Следующие десять действий, без запуска новых этапов из этого аудита:

1. Закрыть review Issue #343 до любого внешнего доступа к document-service.
2. Исправить классификацию постоянных ошибок Kafka (Issue #344) отдельным изменением, не через docs-only PR #346.
3. Довести и смержить доказательство money identity (сейчас Draft #341) либо явно сузить пилот без актов и платежей.
4. Не считать PR #340 (Draft) production-адаптером NLO.
5. Включить `low-code-service` в CI или зафиксировать, что сервис вне обязательного графа.
6. Повторить один живой staging smoke только отдельным разрешением: identity, gateway, shipment, tracking consumer, document write.
7. Зафиксировать migration head живой базы. В git head — `000105`. Совпадение с историческим стендом не проверено на сервере.
8. Не открывать коммерческий пилот, пока consignee/finance/forwarder остаются каркасом, а slot booking не является продуктом.
9. Оставить Autopilot в dry-run. PR #320 merged. 24/7 и auto-deploy не включать.
10. Отдельно решить NET/GROSS, VAT и округление. В payment-service есть `WITH_VAT`, это не закрытая коммерческая модель.

## 2. Evidence and Audit Methodology

Проверено 2026-11-09:

- `git fetch origin main`; `origin/main` = `f58f13b5b4b539895673a77ff404ab47669dc96d`. Совпадает с EXPECTED_MAIN. Отдельный worktree `D:\Projects\freight-platform-audit-2026-11`, ветка `docs/bintrans-full-project-readiness-2026-11`. Исходное дерево агентов не переключалось.
- `gh pr list` / `gh issue list` / `gh pr view 320`.
- CI run `42671916835` attempt 2: `conclusion=success`, `headSha` тот же, 75 jobs success. Attempt 1 на том же SHA: `conclusion=failure`, jobs API `total_count=20`, все success. Источник: предыдущая диагностика этого репозитория в тот же день и повторное чтение `gh run view`.
- Подсчёт `*_test.go`, миграций, `go.mod`, frontend-файлов скриптом по worktree.
- Поиск `CREATE SCHEMA`, `ENABLE ROW LEVEL SECURITY`, slot booking, Kafka (`twmb/franz-go`), JWT.

Формула модуля, если данных хватает для аналитической оценки:

`ModuleReadiness = 0.25F + 0.15UI + 0.20I + 0.15T + 0.15S + 0.10O`

Каждый показатель 0–100 и помечен `EXPLICIT_EXPERT_ESTIMATE`. Все 84 балла F/UI/I/T/S/O — экспертные оценки по прочитанным свидетельствам, не измерения. Формула не менялась. Исходные баллы в R1 не менялись.

`BLENDED_MODULE_READINESS=34_PERCENT` при `BLENDED_READINESS_METHOD=ASSUMED_EQUAL_WEIGHTS` — среднее итоговых Score. `FUNCTIONAL_F_EXPERT_AVERAGE=43.6_PERCENT` — среднее только колонки F, статус `ILLUSTRATIVE_ONLY`. `OFFICIAL_FUNCTIONAL_READINESS=NOT_ESTIMABLE`. `OFFICIAL_OVERALL_READINESS=NOT_ESTIMABLE`. `COMMERCIAL_READINESS_PERCENT=NOT_ESTIMABLE`. `COMMERCIAL_LAUNCH_READY=NO`.

Уверенность MEDIUM означает качество собранных технических свидетельств: код, тесты и CI на `main` прочитаны, живой стенд нет. MEDIUM не означает статистическую точность процентов. LOW означает: в основном документы или каркас. 100% не ставится из-за зелёных unit tests.

Если доказательства нет, статус `NOT_VERIFIED`, не PASS.

Локальные тесты этого аудита: `NOT_RUN`. Изоляция от чужих баз не требовалась, потому что прогон не запускался. Результаты тестов ниже — из CI и из имён тестов в репозитории.

## 3. Repository Inventory

| Класс | Факт | Источник |
|---|---|---|
| Backend Go services | 17 каталогов в `services/`, у каждого есть `cmd/server/main.go` | worktree listing |
| Frontend | `apps/driver-mobile`, `web-admin`, `web-carrier`, `web-consignee`, `web-finance`, `web-procurement`, `web-shipper` | тот же listing |
| Shared packages | `packages/events`, `i18n`, `openapi`, `portal-client`, `proto`, `shared-go`, `shared-ts`, `statussnapshot`, `ui` | listing |
| Go modules | 22 файла `go.mod` | glob |
| API gateway | `services/api-gateway` | path |
| Workers / extra mains | `shipment-outbox-replay`, `shipment-status-snapshot-export`, `control-tower-status-snapshot-import` | glob `main.go` |
| Kafka | franz-go в shipment outbox publisher и tracking `execution_event_consumer.go` | grep |
| Migrations | 105 файлов `*.up.sql`, номера 1–105 без пропусков, head `000105_platform_postgres_role_foundation_v0_2_i1.up.sql` | подсчёт |
| Schemas, созданные миграциями | `core`, `transport`, `rfx`, `documents`, `billing`, `lowcode`, `control_tower`, `tracking`, `contract_rate`, `freight_cost`, `network_optimizer`, `company` | `CREATE SCHEMA` |
| Compose | 9 файлов `docker-compose*.yml` под `infrastructure/docker-compose` | glob |
| CI workflows | `ci.yml`, `ci-manual.yml`, `bintrans-e2e-stand-opt-in.yml`, `bintrans-autopilot-dispatcher.yml` | `.github/workflows` |
| Monitoring | каталог `infrastructure/monitoring` существует | listing. Живой scrape `NOT_VERIFIED` |
| Autopilot | SQL RLS в `tools/bintrans_autopilot/sql/autopilot_queue`. PR #320 merged | grep + `gh pr view 320` |
| Mobile | `apps/driver-mobile`, 101 файл, 0 `*.spec`. Native qualification PR #13 открыт и не в `main` | подсчёт + `gh pr list` |

`low-code-service` имеет сервер и 34 `*_test.go`, но его нет в матрице `backend-go-check` в `.github/workflows/ci.yml` (матрица перечислена в job `backend-go-check`, строки module около 111–128 на этом SHA). Наличие каталога не считается работающим production-сервисом. Deployment каждого сервиса на живом хосте: `NOT_VERIFIED`.

### Service inventory

Readiness ниже — аналитическая, confidence MEDIUM, если не указано иное. Missing — главный пробел, не полный backlog.

| Service | Responsibility | Stack | Path | DB/schema | Tests (`*_test.go`) | CI | Deployment | Readiness | Missing |
|---|---|---|---|---|---|---|---|---|---|
| api-gateway | единая внешняя граница | Go | `services/api-gateway` | не владеет схемой | 97 | `backend-go-check` в полном CI | `NOT_VERIFIED` | partial | полный обход прямых портов сервисов не закрыт как доказанный |
| identity-service | пользователи, JWT | Go | `services/identity-service` | `NOT_VERIFIED` отдельная schema | 10 | в матрице CI | `NOT_VERIFIED` | partial | живой SSO/MFA `NOT_VERIFIED` |
| company-service | компании и связи | Go | `services/company-service` | `company` с `000100` | 20 | в матрице | `NOT_VERIFIED` | partial | cross-tenant пилот не подтверждён |
| shipment-service | перевозки, outbox | Go | `services/shipment-service` | `transport` | 133 | в матрице + execution tracking job | `NOT_VERIFIED` | partial | production tracking consumer blocked, Issue #344 |
| transport-order-service | транспортные заказы | Go | `services/transport-order-service` | `transport` | 20 | в матрице | `NOT_VERIFIED` | partial | полный carrier/driver assignment E2E `NOT_VERIFIED` |
| tracking-service | ingestion и execution consumer | Go, franz-go | `services/tracking-service` | `tracking` | 10 | в матрице полного CI | `NOT_VERIFIED` | partial | permanent retry, Issue #344 |
| rfx-service | тендеры, award | Go | `services/rfx-service` | `rfx` | 181 | много integration/browser jobs | `NOT_VERIFIED` | strongest partial | живой buyer pilot `NOT_VERIFIED` |
| contract-rate-service | договоры и ставки | Go | `services/contract-rate-service` | `contract_rate` | 11 | в матрице + public e2e job имени | `NOT_VERIFIED` | partial | коммерческий тарифный контур не закрыт |
| freight-cost-service | стоимость и ledger | Go | `services/freight-cost-service` | `freight_cost` | 57 | CI jobs freight-cost-* | `NOT_VERIFIED` | partial | intelligence rollout PR #61 всё ещё open |
| billing-register-service | реестры | Go | `services/billing-register-service` | `billing` | 21 | в матрице | `NOT_VERIFIED` | partial | акт/счёт как юридический документ не доказан |
| payment-service | платежи, VAT basis | Go | `services/payment-service` | uses billing snapshots | 24 | в матрице | `NOT_VERIFIED` | partial | money identity Draft #341 |
| document-service | документы и подпись | Go | `services/document-service` | `documents` | 46 | в матрице | `NOT_VERIFIED` | partial | Issue #343, нет оператора ЭДО |
| control-tower-read-model-service | операционная проекция | Go | `services/control-tower-read-model-service` | `control_tower` | 49 | в матрице | `NOT_VERIFIED` | partial | `CT_MUTATION_EQUALS_VIEW` OPEN |
| network-optimizer-service | планы и поиск цепочек | Go | `services/network-optimizer-service` | `network_optimizer` | 113 | NLO jobs входят в полный граф CI | `NOT_VERIFIED` | partial | production adapter Draft #340; исполнение плана не равно расчёту |
| analytics-service | KPI reads | Go | `services/analytics-service` | `NOT_VERIFIED` | 4 | в матрице; caller-identity merged #336 | `NOT_VERIFIED` | low partial | опубликованные OTIF/POD completeness остаются gap |
| localization-service | локали | Go | `services/localization-service` | `NOT_VERIFIED` | 0 | в матрице (`go test ./...`) | `NOT_VERIFIED` | thin | нет тестовых файлов |
| low-code-service | custom fields | Go | `services/low-code-service` | `lowcode` | 34 | нет в матрице `backend-go-check` | `NOT_VERIFIED` | unknown in CI | CI graph не гоняет этот модуль |

Public/internal route tables по каждому сервису в этом аудите не переписывались построчно. Подтверждённые группы: document write routes в тексте Issue #343; JWT создаётся в `identity-service/internal/service/auth_service.go` (`CreateAccessToken`). Остальные маршруты: код есть, полный каталог `NOT_VERIFIED` в этом отчёте.

## 4. Architecture Overview

Платформа собрана как набор Go-сервисов за `api-gateway`, с PostgreSQL-схемами из `infrastructure/migrations` и Redpanda/Kafka-контрактами на границе shipment outbox → tracking consumer. Control Tower читает проекцию, а не является единственным writer исполнения. RFx, contract/rate, freight cost, billing и payment — отдельные сервисы. Frontend разделён по ролям; часть приложений — скелет.

Доказательство связности — CI workflow `.github/workflows/ci.yml` на `f58f13b5` и успешный attempt 2 run `42671916835`. Доказательство живой топологии серверов в этом аудите отсутствует.

## 5. Module Readiness Matrix

Все 84 значения F, UI, I, T, S и O — `EXPLICIT_EXPERT_ESTIMATE`. Числа в R1 не пересматривались. Confidence в таблице говорит о полноте свидетельств по модулю, не о точности процента. MEDIUM: код, тесты и CI прочитаны. LOW: в основном документы или каркас.

| Модуль | F | UI | I | T | S | O | Score | Confidence | Класс |
|---|---:|---:|---:|---:|---:|---:|---:|---|---|
| A Foundation | 65 | 45 | 55 | 50 | 35 | 30 | 50 | MEDIUM | PARTIAL |
| B TMS | 60 | 35 | 50 | 55 | 40 | 25 | 47 | MEDIUM | PARTIAL |
| C RFx | 70 | 65 | 55 | 70 | 45 | 30 | 59 | MEDIUM | PARTIAL |
| D Rates and freight cost | 60 | 40 | 50 | 55 | 40 | 25 | 48 | MEDIUM | PARTIAL |
| E Finance | 45 | 10 | 40 | 40 | 30 | 15 | 33 | MEDIUM | PARTIAL |
| F EDO / documents | 40 | 25 | 30 | 40 | 25 | 15 | 31 | MEDIUM | PARTIAL |
| G Tracking / driver | 50 | 20 | 40 | 35 | 30 | 15 | 35 | MEDIUM | PARTIAL |
| H Control Tower | 50 | 40 | 45 | 50 | 30 | 25 | 42 | MEDIUM | PARTIAL |
| I Network optimizer | 45 | 20 | 25 | 55 | 35 | 15 | 34 | MEDIUM | PARTIAL |
| J Analytics | 25 | 15 | 20 | 15 | 25 | 10 | 20 | LOW | PARTIAL |
| K Customer portals | 30 | 35 | 25 | 30 | 25 | 10 | 27 | MEDIUM | PARTIAL |
| L Slot booking | 15 | 10 | 10 | 5 | 10 | 5 | 10 | LOW | DESIGNED_ONLY |
| M Integrations | 25 | 15 | 20 | 25 | 20 | 10 | 20 | LOW | PARTIAL |
| N Autopilot | 30 | 5 | 15 | 25 | 20 | 5 | 19 | MEDIUM | PARTIAL |

Сумма экспертных Score = 475. `BLENDED_MODULE_READINESS=34_PERCENT` (`33.9` до округления, `ASSUMED_EQUAL_WEIGHTS`). Сумма экспертных F = 610. `FUNCTIONAL_F_EXPERT_AVERAGE=43.6_PERCENT` (`ILLUSTRATIVE_ONLY`). Оба числа не являются `OFFICIAL_FUNCTIONAL_READINESS`. Она остаётся `NOT_ESTIMABLE`.

Почему не 100% при зелёном CI: CI доказывает, что зарегистрированные jobs этого workflow прошли на этом SHA. Он не доказывает пилот, юридический ЭДО, деньги, slot booking и production consumer.

## 6. Overall Readiness

`OFFICIAL_OVERALL_READINESS=NOT_ESTIMABLE`. `OFFICIAL_FUNCTIONAL_READINESS=NOT_ESTIMABLE`. `COMMERCIAL_READINESS_PERCENT=NOT_ESTIMABLE`. `COMMERCIAL_LAUNCH_READY=NO`.

`BLENDED_MODULE_READINESS=34_PERCENT` при равных весах 14 модулей — иллюстрация таблицы раздела 5, не функциональная готовность. `FUNCTIONAL_F_EXPERT_AVERAGE=43.6_PERCENT` — иллюстрация только колонки F. Ни 34%, ни 43,6% не отменяют блокеры безопасности, ЭДО, финансов и непроверенной инфраструктуры.

Непройденные launch gates:

- Document write trust boundary, Issue #343.
- Tracking permanent-error handling, Issue #344.
- Money identity на `main`, PR #341 Draft.
- Юридический оператор ЭДО и GAP-B-001 runtime.
- Подтверждённый backup/restore живой базы: `NOT_VERIFIED`.
- Production tracking provider и routing provider как подключённые системы: `NOT_VERIFIED`.
- Customer pilot с реальными компаниями: `NOT_VERIFIED`.
- `CT_MUTATION_EQUALS_VIEW` остаётся OPEN (`docs/architecture/tms/TMS_STOP_EVENT_TRACKING_CONTRACT_0_9B.md`).

## 7. Core TMS Readiness

На `main` есть фундамент исполнения: миграции `000088`–`000093` (execution foundation, commands, driver stop tasks, stop tracking, control-tower projection, delivery disposition), shipment-service и transport-order-service. Контракт stop-event проверен тестом `TestStopEventTrackingContract09B` в CI attempt 2 job `execution-tracking-pod-integration` (job id `129357503754`): PASS, включая `PICKUP_DELIVERY_SEQUENCE`, `OUT_OF_ORDER_PREDECESSOR`, `FAIL_DELIVERY_ACTION`, `COMMAND_OUTBOX_ATOMICITY`.

Это контрактный integration test, не полный операционный E2E от заказа до вручения с живым водителем. Carrier assignment, returns, partial delivery и exceptions как коммерческие процессы: код и миграции частично есть (`000093` delivery disposition). Полнота `NOT_VERIFIED` как пользовательский сценарий.

Цепочка заказ → перевозка → назначение → multi-stop → POD не объявляется E2E PASS.

## 8. RFx and Procurement

Самый плотный контур репозитория: 181 `*_test.go` в `rfx-service`, browser Playwright в `apps/web-procurement/e2e/*`, CI jobs questionnaire, lane ownership, late submission, human provenance, award-to-transport-order. На attempt 2 весь граф success, значит эти jobs, входящие в 75, прошли. На attempt 1 часть из них тоже была среди 20 success, но это не полный граф.

Award → transport order покрыт job-именем `rfx-award-to-transport-order-integration` (присутствовал и в attempt 1 как success). Версионирование и audit отражены миграциями `000065`–`000074`. Живой тендер с внешними перевозчиками: `NOT_VERIFIED`.

## 9. Finance, Contracts and Rates

Схемы `contract_rate` (`000048`) и `freight_cost` (`000054`) существуют. Тесты: contract-rate 11, freight-cost 57, billing 21, payment 24. CI содержит freight settlement, pricing snapshot, cost analytics jobs; они входят в полный граф attempt 2.

Payment code использует `TaxBasisWithVAT` и `TotalWithVAT` (`services/payment-service/internal/domain/outbox.go`, repository lookup). Это след модели с НДС, не закрытое решение NET/GROSS, округления и платёжного контура. PR #341 Draft: «Prove snapshot-to-closing money identity for one shipment». Пока он Draft, доказательство на `main` отсутствует.

Коммерческая готовность финансов: не готова. UI `apps/web-finance` — скелет (см. раздел 15).

## 10. EDO and Documents

`document-service` и схема `documents` есть. Миграции подписи и вложений: `000087`, `000095`, `000096`, POD binding `000099`. PR #339 merged (`6e81da05`): signing recovery. Issue #343 прямо говорит, что signing recovery уже fail-closed 403, и что это не доказательство защиты остальных write routes.

GAP-B-001 остаётся открытым контрактом полноты документов, не runtime. Источники: `docs/architecture/edo-document-completeness-source-v0.4b.md` (`SOURCE_GAP=GAP-B-001`), `docs/architecture/edo-signing-recovery-v0.1.md` («GAP-B-001 stays open», «No document write-authority contract»).

Внешний оператор ЭДО: поиск по репозиторию в рамках архитектурных заметок не показывает подключённого оператора. Юридически значимый документооборот: `NOT_STARTED` как внешний юридический контур. Внутренний механизм подписи: `PARTIAL`.

## 11. Tracking and Driver Application

Consumer: `services/tracking-service/internal/service/execution_event_consumer.go` (franz-go). Publisher: `services/shipment-service/internal/outbox/kafka_publisher.go`. Issue #344: любая ошибка `Apply`, включая unique `uq_execution_event_inbox_sequence` (`23505`), ретраится без классификации. Offset не коммитится, партиция блокируется. Статус issue: OPEN. PR #346 — Draft «Define the tracking Kafka permanent-error contract». Это документация контракта, не исправление consumer. В `main` дефект остаётся.

Issue #345 OPEN, LOW: test-only зависимость shipment → tracking module. Бинарники сервера пакет не импортируют, по тексту issue.

`apps/driver-mobile` есть (101 файл) и не имеет spec-файлов в подсчёте. PR #13 (native Android/iOS qualification) открыт, не merged. Driver app на `main`: `PARTIAL`. Store-ready native: не в `main`.

ETA и внешний tracking provider как боевое подключение: `NOT_VERIFIED`.

## 12. Control Tower

Read model и схема `control_tower` есть (`000015`, `000092`, `000093`). 49 тестовых файлов. `web-admin` — 353 файла, это не скелет из двух страниц, но полнота operator workspace в этом аудите не кликалась: `NOT_VERIFIED` как UI walkthrough.

`CT_MUTATION_EQUALS_VIEW` = OPEN в `TMS_STOP_EVENT_TRACKING_CONTRACT_0_9B.md`: роли просмотра всё ещё могут acknowledge, assign и resolve. RBAC в том этапе не менялся. Issue #334 открыт как три LOW follow-up read safety. Это не закрытый операторский контур.

## 13. Network Logistics Optimizer

113 тестовых файлов. Миграции поиска и accept/activate до `000097`, плюс изоляция коммерческих данных merged PR #333 (`3d2975a2`). Документы NLO прямо выносят slot booking, assignment и исполнение за скобки расчёта (`docs/architecture/network-optimizer/NLO_0_3_API_DATA_CONTRACTS.md`).

Расчёт плана и реальное выполнение — разные вещи. PR #340 Draft: production adapter proof. На `main` этого proof нет. Внешний routing provider (исторически 2GIS на клоне staging) в этом аудите по сети не проверялся: `CURRENT_STATE_NOT_VERIFIED`.

## 14. Analytics

`analytics-service`: 4 тестовых файла. PR #336 merged: service-to-service caller identity. Архитектурные заметки держат `CAR_OTIF`, `OPS_POD_COMPLETENESS`, `CAR_POD_COMPLETENESS`, `CAR_DOCUMENT_COMPLETENESS` как нереализованные или blocked source (GAP-C-001, GAP-C-002, GAP-B-001). Свежесть данных и staging deployment аналитики: `NOT_VERIFIED`.

## 15. Customer Portals

| Портал | Что на `main` | Тесты | Пилот |
|---|---|---|---|
| Shipper `apps/web-shipper` | страницы login, shipments, tracking, documents; компоненты ETA/slot facts | `playwright.config.ts`, specish count 2 | walkthrough docs PR #338; живой пилот `NOT_VERIFIED` |
| Carrier `apps/web-carrier` | приложение есть, 44 файла | playwright config, specish 2 | `NOT_VERIFIED` как реальные пользователи |
| Procurement `apps/web-procurement` | плотный RFx UI, 329 файлов | много e2e playwright, они в CI | browser CI ≠ пилот с backend заказчика |
| Admin `apps/web-admin` | 353 файла | specish 0 в этом подсчёте | `NOT_VERIFIED` |
| Consignee | `pages/index.vue`: «skeleton UI, no business logic yet» | 0 | нет |
| Finance portal | `pages/index.vue`: «skeleton UI, no business logic yet» | 0 | нет |
| Forwarder | каталога `apps/web-forwarder` нет | нет | `NOT_IMPLEMENTED` по `CUSTOMER_PORTALS_MAIN_BASELINE_0_2.md` |

Mock Playwright не приравнивается к пилоту. PR #338 добавил walkthrough и smoke E2E как документацию/тест в репозитории (`96f6f979`). Этот аудит сценарий не переигрывал и пользователей не опрашивал.

## 16. Slot Booking

Отдельного продукта записи слотов склада нет. Документ `docs/engineering/parallel/pilots/control-tower-slot-intelligence-v0.7.2/ARCHITECTURE.md` явно выносит slot booking UI за скоуп. Shipper UI показывает `SlotFacts` / `SlotHistoryTable` как факты трекинга, не как назначение дока. Интеграция слота с исполнением перевозки: `NOT_STARTED` как booking workflow. Класс: `DESIGNED_ONLY`.

## 17. Integration Inventory

| Направление | На `main` | Реальный провайдер |
|---|---|---|
| ERP | миграции RFx ERP principals `000074`, excel exchange `000073` | подключённый ERP заказчика `NOT_VERIFIED` |
| WMS | отдельного WMS-адаптера в инвентаре сервисов нет | `NOT_VERIFIED` / вероятно отсутствует |
| Tracking providers | consumer и контракт событий есть | боевой провайдер `NOT_VERIFIED` |
| Routing/maps | optimizer читает контракты; внешний вызов в этом аудите не делался | `NOT_VERIFIED` |
| EDO operator | внутренние подписи | оператор `NOT_STARTED` |
| Customer API | gateway + portal clients | внешние клиенты `NOT_VERIFIED` |
| Carrier API | RFx/carrier portal и interfaces в коде | живая сеть перевозчиков `NOT_VERIFIED` |
| Webhooks | `NOT_VERIFIED` полным поиском контракта | `UNKNOWN` |
| Excel | RFx excel migrations и browser e2e jobs | файловый обмен в CI, не пилот |
| EDI | `NOT_VERIFIED` как подключённый контур | `UNKNOWN` |
| Object storage | document attachments в миграциях `000095` | боевой bucket `NOT_VERIFIED` |

Контракт или адаптер в репозитории не равен подключённому провайдеру.

## 18. Security Review

Активные атаки не выполнялись.

| RISK_ID | SEVERITY | IMPACT | EVIDENCE | MITIGATION | OWNER | BLOCKS_PILOT | BLOCKS_PRODUCTION |
|---|---|---|---|---|---|---|---|
| SEC-DOC-343 | HIGH (по тексту issue: POTENTIAL_IMPACT=HIGH, exposure NOT_ESTABLISHED) | запись документов в обход ожидаемой границы | Issue #343 OPEN; routes listed there | отдельный review и, если нужно, отдельное API-изменение. Этот аудит API не меняет | не назначен в issue | YES для внешнего доступа к document writes | YES |
| SEC-TRACK-344 | MEDIUM | остановка партиции Kafka на постоянной ошибке | Issue #344, `execution_event_consumer.go` | классификация ошибок и карантин. PR #346 это не чинит | не назначен | YES для боевого tracking | YES |
| SEC-CT-VIEW | MEDIUM | роль просмотра мутирует кейсы | `CT_MUTATION_EQUALS_VIEW` OPEN в контракте 0.9B | отдельный RBAC этап | не назначен | YES для операторского пилота CT | YES |
| SEC-RLS | MEDIUM | платформенные схемы без найденного RLS | `ENABLE ROW LEVEL SECURITY` в `infrastructure/migrations` не найден. RLS есть в autopilot SQL | не считать отсутствие строки доказательством дыры в каждом запросе; изоляция может быть в WHERE. Для production RLS или эквивалент нужно доказать отдельно | platform | CONDITIONAL | YES, пока не доказано |
| SEC-S2S | MEDIUM, частично закрыт | подмена internal caller | PR #336 merged, analytics caller tokens | распространить тот же приём на остальные internal routes. Полнота `NOT_VERIFIED` | analytics уже в main | PARTIAL | PARTIAL |
| SEC-345 | LOW | тестовая зависимость модулей | Issue #345 | перенос контрактного теста. Не дефект runtime по тексту issue | tracking | NO | NO |
| SEC-SECRETS | UNKNOWN | утечка конфигурации | секреты не читались | не публиковать env. Управление секретами на сервере `NOT_VERIFIED` | ops | UNKNOWN | YES, пока не проверено |
| SEC-334 | LOW | follow-up read safety CT | Issue #334 OPEN | три LOW, не закрывают #343 | CT | NO как единственный блокер | NO |
| SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER | MEDIUM, статус OPEN. Эксплуатация не воспроизводилась | смена tenant context, если заголовок доходит до сервиса без доверенного вызывающего | `handlers/tenant.go` `resolveVerifiedTenant`; комментарий в `successor_revision_handler.go`; `userauth/jwt.go` заголовок не читает | отдельный security review границ маршрутов. Код shipment-service этим аудитом не менялся | не назначен | YES для external pilot, пока review не закрыт | YES, пока review не закрыт |

`SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` не является Issue #343 и не является вопросом database RLS.

Что известно. `resolveVerifiedTenant` в `services/shipment-service/internal/http/handlers/tenant.go` берёт tenant только из `X-Tenant-ID` и проверяет, что значение непустое и является UUID. Комментарий функции говорит, что заголовок выставляет API Gateway. `successor_revision_handler.go` повторяет: operating tenant comes from the verified gateway header, never from the body. `RequireJWT` в `services/shipment-service/internal/http/userauth/jwt.go` проверяет bearer и прямо не читает `X-Tenant-ID` и `X-User-ID`.

Граница доверия. Для маршрутов, которые вызывают `resolveVerifiedTenant`, сервис считает заголовок уже доверенным. Этот аудит не составил карту, какие маршруты стоят за gateway, какие за `RequireJWT`, и какие доступны напрямую.

Потенциальное влияние. Если прямой вызов shipment-service принимает клиентский `X-Tenant-ID` без проверки вызывающего, tenant context можно подменить. Это описание границы, не подтверждённая эксплуатация. Воспроизведения не было.

Что не проверялось. Сетевая достижимость сервиса, полнота middleware на каждом маршруте, соответствие tenant в JWT и в заголовке, staging и production.

Нужен отдельный security review. Статус риска: OPEN. До него external pilot и production, которые ходят в shipment-service с этим заголовком, не считаются закрытыми по tenant boundary.

JWT: `AuthService.CreateAccessToken` в identity-service. Это реализация выпуска токена, не доказательство, что каждый прямой порт сервиса её требует.

## 19. Database and Migration State

- Head в git: `000105_platform_postgres_role_foundation_v0_2_i1.up.sql`.
- 105 up-миграций, номера 1…105, пропусков нет.
- Down-файлы у миграций в этом аудите не сверялись попарно: полнота rollback `NOT_VERIFIED`.
- Историческое ограничение стенда «head 000105» совпадает с текущим git head. Живая база могла отставать или быть впереди локальных файлов вне `main`. `CURRENT_DB_HEAD=NOT_VERIFIED`.
- Миграции не применялись.
- Referential integrity и retention: точечно не аудировались. `NOT_VERIFIED` как свойство всей базы.
- Tenant isolation в SQL: RLS платформы не найден (раздел 18). Фильтры `tenant_id` в запросах существуют в сервисах, полный обзор `NOT_VERIFIED`.

## 20. Infrastructure and Deployment

Подтверждено как код и CI, не как живой хост:

- Compose-файлы для локального/staging-описания есть, включая `docker-compose.bintrans-ct-staging.yml` и e2e stand opt-in workflow.
- CI `main` на этом SHA: attempt 2 success. Attempt 1 — неполный граф и failure. Это риск планировщика GitHub Actions, уже воспроизведённый фактом 20 против 75 jobs, и не продуктный регресс тестов.
- Backup timer и clone shutdown runbook из параллельной работы не являются частью этого SHA, пока файлы не в `main`. В этом worktree они не считаются production backup.

`CURRENT_STATE_NOT_VERIFIED` для production, staging, dedicated E2E host, DNS/TLS, S3, secrets, логов, restore и health на сервере. Дата последней живой проверки в этом отчёте не ставится, потому что проверки не было.

E2E на реальном сервере из этого аудита проводить нельзя: нет разрешения и нет свежего подтверждения изоляции. Условия для staging pilot: отдельная авторизация, Issue #343 и #344 хотя бы с явным компенсирующим контролем, зафиксированный migration head, запрет на production, запрет удаления томов.

Нужные ресурсы (из состава compose и сервисов, не из замера сервера): PostgreSQL, Redpanda/Kafka, набор Go-сервисов, gateway, как минимум web-procurement и web-admin/web-shipper для портального среза, объектное хранилище для документов. Точные CPU/RAM: `NOT_VERIFIED`.

## 21. E2E Test Coverage

`E2E_FULL_PATHS_TOTAL=16`. `E2E_FULL_PATHS_VERIFIED=0`. `E2E_BLOCKED_ATTRIBUTE=4`. `E2E_LIVE_NOT_RUN_ATTRIBUTE=16`.

`BLOCKED` и `NOT_RUN` — пересекающиеся признаки одной и той же таблицы, не две доли, которые можно сложить. Четыре сценария со статусом BLOCKED (7, 9, 10, 13) тоже не проходили полный live E2E, поэтому они входят в 16 `NOT_RUN`. Сумма 4+16 не равна 20 и не используется.

`CI TESTED` в колонке статуса означает конкретный частичный тест или job. Успех интеграционного теста не подтверждает полный пилотный сценарий. Частичные доказательства ниже сохранены.

| # | Сценарий | Статус | Почему |
|---|---|---|---|
| 1 | Shipper → RFx → Award | TESTED частично | RFx CI есть. Shipper UI не создаёт тендер. Сквозной UI `NOT_RUN` |
| 2 | Award → Transport Order | TESTED | job `rfx-award-to-transport-order-integration` в графе CI, attempt 2 success |
| 3 | Transport Order → Shipment | TESTED | job `order-execution-integration` входит в полный граф 75, attempt 2 success. Отдельный лог этого job в аудите не перечитывался |
| 4 | Shipment → Carrier assignment | UNKNOWN | отдельный обязательный job с таким именем не выделялся |
| 5 | Carrier → Driver/Vehicle | UNKNOWN | driver-mobile не связан доказанным E2E |
| 6 | Pickup → multi-stop → delivery | TESTED как контракт | `TestStopEventTrackingContract09B` PASS. Полный операционный рейс `NOT_RUN` |
| 7 | Driver event → Tracking | BLOCKED для production | Issue #344. Контрактный Apply тестировался в #342 |
| 8 | Tracking → Control Tower | TESTED частично | проекция и CI control-tower job. Живой поток `NOT_RUN` |
| 9 | Shipment → POD | BLOCKED для внешнего запуска | код и миграции есть; Issue #343 на write path |
| 10 | POD → Documents | BLOCKED | тот же #343. Signing recovery 403 не закрывает остальные writes |
| 11 | Shipment → Billing | TESTED частично | billing service и settlement CI job. Сквозной платёж `NOT_RUN` |
| 12 | Billing → Settlement | TESTED частично | `freight-settlement-integration` в CI |
| 13 | Settlement → Act/Invoice | BLOCKED / не на main | PR #341 Draft |
| 14 | Optimizer → Accept/Activate | TESTED частично | миграция `000086`, NLO CI в полном графе. Production adapter не на main |
| 15 | Portal → real backend | TESTED в CI browser jobs | пилот `NOT_VERIFIED`. Playwright ≠ пилот |
| 16 | Slot booking → execution | NOT_STARTED | booking UI вне скоупа документов и не найден как продукт |

`E2E_FULL_PATHS_VERIFIED` остаётся 0. Строки со словом TESTED не повышают это число.

## 22. CI/CD Health

Workflow: `.github/workflows/ci.yml`. Push в `main` и pull_request в `main`.

На `f58f13b5`:

- Run `42671916835` attempt 1: completed/failure, 20 success, 55 jobs не зарегистрированы, `--log-failed` пустой.
- Attempt 2: completed/success, 75 success, 0 failed, 0 cancelled. `execution-tracking-pod-integration` success.
- Предыдущий main run `42665604564` на `6e81da05`: success, 75 jobs.
- Принятый PR CI #342 run `42668402805` на `7dc6325a`: success, 75 jobs.

Вывод: обязательный граф этого workflow — 75 jobs. Attempt 1 был сбоем регистрации графа, не падением теста. Продуктовый CI на том же SHA затем прошёл. Это не гарантия, что следующий push снова получит полный граф с первой попытки.

`low-code-service` в этот граф не входит.

E2E stand workflow `bintrans-e2e-stand-opt-in.yml` — opt-in. Его свежий success на живом стенде: `NOT_VERIFIED`.

Autopilot dispatcher workflow существует. Включение 24/7 им не подтверждено.

## 23. Autopilot and 24/7 Readiness

PR #320: **MERGED** 2026-10-31, merge `e031d90c6306b7e6b1aa0a7621e3be4484e0d8dc`, title «feat(autopilot): add a dry-run worker foundation», не Draft. Старые рассказы «PR #320 ещё Draft» на 2026-11-09 неверны.

Открытые Draft, не функциональность `main`:

- #329 mock-only orchestration.
- #335 refuse unknown Cursor charge.

Issue #313 OPEN: model-cost and provider gates before paid activation.

Реально подтверждено: foundation dry-run попала в историю `main`, очередь описана SQL с RLS в `tools/bintrans_autopilot`. Не подтверждено и этим заданием запрещено: paid activation, auto-merge, auto-deploy, 24/7 autonomous mode, Cloud Pilot как включённый режим. `AUTOPILOT_24_7_READY=NO`.

## 24. Open PRs and Issues

Снимок автора отчёта, `gh pr list` 2026-11-09, до того как этот PR стал предметом ревью: `AUTHOR_SNAPSHOT_OPEN_PRS=17`. Открытые Issues в том же снимке: 6.

Независимый ревью-снимок: `INDEPENDENT_REVIEW_SNAPSHOT_OPEN_PRS=18`. Разница — появление PR #347.

Повторная проверка R1, `gh pr list --state open` 2026-11-09: открытых PR 18, среди них #347. Исторические 17 не подменяются этим числом и не выдаются за полный текущий реестр без этой пометки. Состав остальных открытых PR ниже относится к авторскому снимку и в R1 заново не переписывался.

Особые PR:

| PR | Состояние | HEAD | CI | Review | Блокер | Модуль |
|---|---|---|---|---|---|---|
| #340 NLO production adapter proof | OPEN, Draft | ветка `test/nlo-production-adapter-proof-v1.0a` | полный rollup в этом аудите не дочитан из-за TLS timeout на одном `gh pr view` | нет reviewDecision в списке | Draft нельзя считать адаптером на `main` | NLO |
| #341 Finance money identity | OPEN, Draft | `5444800f2b9e1df8a2bdedf33ca28c15eebeedb8` | в rollup есть SUCCESS по крайней мере у autopilot unit; полный вывод CI не сводился | нет | Draft, деньги не на `main` | Finance |
| #346 Kafka permanent-error discovery | OPEN, Draft | `9b480f17118985c8b4791c66039b2a0525afdb98` | CI rollup содержит SUCCESS jobs; это docs contract | нет | не исправляет consumer | Tracking |
| #320 | MERGED | merge `e031d90c` | исторический, не переоткрыт | merged | не блокер сам по себе | Autopilot dry-run |

Остальные открытые PR (не функциональность `main`, пока не merged): #335, #329, #315 (не Draft, cloud agent dev env docs), #298, #282 (предыдущий readiness report Draft), #272, #163 и #161 с пометкой DO_NOT_MERGE, #65, #61, #51, #13, #11, #10.

Issues:

| Issue | Состояние | Severity по тексту | Что блокирует |
|---|---|---|---|
| #343 document write routes | OPEN | review required, potential HIGH | внешний запуск документов |
| #344 Kafka permanent retry | OPEN | MEDIUM, production consumer BLOCKED | боевой tracking |
| #345 test-only dependency | OPEN | LOW | не runtime |
| #337 NLO balanced hidden-rate test | OPEN | LOW по заголовку | качество регресса NLO, не единственный launch gate |
| #334 CT read safety follow-ups | OPEN | LOW | не заменяет `CT_MUTATION_EQUALS_VIEW` |
| #313 autopilot cost gates | OPEN | до paid activation | включение платного autopilot |

Существенные merged PR по `git log` этого SHA, не полный исторический список: #342 stop-event contract, #339 signing recovery, #338 portal walkthrough docs, #336 analytics caller identity, #333 NLO commercial isolation, #332 document completeness fix, #330 shipper e2e tamper hardening. Номера взяты из сообщений merge-коммитов, не из пересчёта всех PR репозитория.

## 25. Unimplemented Functionality

`FUNCTION_LEVEL_CLASSIFICATION`. Таблица ниже классифицирует функции, не модули.

`MODULE_LEVEL`: `IMPLEMENTED=0`, `PARTIAL=13`, `DESIGNED_ONLY=1` (slot booking как модуль L). Эта матрица из 14 модулей в R1 не менялась. Function-level `DESIGNED_ONLY` внутри частичного модуля не добавляет модуль. Consignee UI, Finance UI и forwarder portal лежат внутри модуля K. Юридический ЭДО лежит внутри модуля F. Режим 24/7 Autopilot лежит внутри модуля N.

| Функция | Класс | Основание |
|---|---|---|
| Юридический ЭДО-оператор | NOT_STARTED | нет подключённого оператора в проверенных контрактах; GAP-B-001 runtime не авторизован документами |
| Forwarder portal | DESIGNED_ONLY | `apps/web-forwarder` отсутствует, так и записано в baseline 0.2 |
| Consignee portal | DESIGNED_ONLY | скелет без бизнес-логики |
| Finance portal UI | DESIGNED_ONLY | скелет. Backend billing/payment при этом PARTIAL |
| Slot booking product | DESIGNED_ONLY | UI booking вне скоупа, продукта нет |
| 24/7 Autopilot | DESIGNED_ONLY / запрещён | dry-run foundation merged, режим не включён |
| Production NLO adapter | BLOCKED | PR #340 Draft |
| Money identity closing | BLOCKED | PR #341 Draft |
| Tracking permanent-error fix | BLOCKED | Issue #344, PR #346 не код фикса |
| Document write closure | BLOCKED | Issue #343 |
| Полный E2E 16 шагов | NOT_RUN | раздел 21 |
| Commercial pilot | NOT_VERIFIED | нет подтверждённых платящих пользователей в этом аудите |
| Backup/restore живой базы | NOT_VERIFIED | процедуры на сервере не выполнялись |
| Monitoring и incident response на бою | NOT_VERIFIED | каталог monitoring есть |
| WMS, EDI как подключённые контуры | UNKNOWN | поиск не доказал ни реализацию, ни полное отсутствие упоминаний |
| Native store driver apps | NOT на main | PR #13 open. Каталог `driver-mobile` при этом есть |

## 26. Load and Scalability

Цели 1 400 грузовладельцев, 40 000 перевозчиков, 20 000 экспедиторов и 30 000 перевозок в месяц — требования, не измерения.

Performance suite, замер Kafka throughput, concurrent users и latency на этих объёмах в этом аудите не найдены как выполненный отчёт. `freight-cost-analytics-100k-gate` входит в CI-граф и на attempt 2 граф зелёный. Это шлюз одного аналитического контура, не сертификация платформы на целевой масштаб.

`TARGET_SCALE_VERIFIED=NO`.

Вывод о производительности по размеру сервера не делается: сервер не измерялся.

## 27. Competitor Feature Gap

Сравнение с Transporeon и Oracle OTM не является launch gate BINTRANS.

`EXTERNAL_COMPARISON_EVIDENCE=EXTERNAL_COMPARISON_NOT_VERIFIED`. В R1 колонки конкурентов не используются как доказанные преимущества или недостатки BINTRANS. Предыдущие формулировки «страница заявляет» сняты: точные URL, версии страниц и дословные выдержки в этом исправлении заново не проверялись и по памяти не восстанавливаются.

Колонка BINTRANS остаётся выводом по репозиторию `f58f13b5`.

| FUNCTION | BINTRANS | TRANSPOREON | ORACLE_OTM | GAP |
|---|---|---|---|---|
| Transportation management | PARTIAL, сервисы shipment и transport-order на `main` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | коммерческий контур BINTRANS не доказан |
| Procurement | PARTIAL, самый зрелый CI в репозитории | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | живая сеть перевозчиков BINTRANS `NOT_VERIFIED` |
| Carrier network | портал и RFx есть, сеть `NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | сеть не подключена доказанно |
| Execution | контракт stop-event PASS, production consumer BLOCKED | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | Issue #344 |
| Tracking | consumer есть, провайдер `NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | боевой провайдер не подтверждён |
| Visibility | Control Tower partial | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `CT_MUTATION_EQUALS_VIEW` OPEN |
| Slot booking | DESIGNED_ONLY как продукт | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | booking workflow не начат |
| Optimization | расчётный NLO partial | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | production adapter не на `main` |
| Tender management | RFx partial | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | пилот не подтверждён |
| Analytics | 4 теста, часть KPI не опубликована | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | источники OTIF/POD не закрыты |
| Finance | billing/payment partial, identity Draft | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | деньги и акты не закрыты |
| Integrations | контракты ERP/Excel, провайдеры `NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | боевые ERP/WMS не подтверждены |
| Enterprise security | JWT есть, Issue #343 и tenant header shipment OPEN, RLS платформы не доказан | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | паритет не заявляется |
| Customer portals | shipper/carrier/procurement partial; consignee, finance UI и forwarder — каркас внутри модуля K | `EXTERNAL_COMPARISON_NOT_VERIFIED` | `EXTERNAL_COMPARISON_NOT_VERIFIED` | три роли не являются отдельными модулями |

## 28. Minimal Pilot Plan

Scope: один грузовладелец и ограниченный набор перевозчиков на уже зелёных CI-контурах RFx award → transport order и stop-event contract, без юридических счетов, без внешнего ЭДО, без боевого Kafka, без slot booking, без Autopilot.

Dependencies: решение по Issue #343 (документы не писать снаружи, пока review открыт), Issue #344 (tracking не объявлять production), зафиксированный staging и migration head.

Critical path: изоляция staging → повторный smoke identity/gateway/shipment на разрешённом стенде → письменный запрет document writes и consumer-production → только затем приглашение людей.

Security gates: #343, #344, `CT_MUTATION_EQUALS_VIEW` если в пилот входит оператор CT.

Integration gates: не подключать ERP, WMS, ЭДО-оператора и routing provider в этом плане.

Infrastructure: отдельный staging, не production. Backup перед смоуком. Restore в этом аудите не доказан, поэтому пилот с ценными данными нельзя начинать, пока restore не проверен отдельной работой.

Tests: обязательный полный CI 75 jobs success на SHA пилота. Локальный прогон этого аудита `NOT_RUN`.

Manual operator actions: человек смотрит award и создаёт исполнение. Autopilot не нажимает merge и не деплоит.

Приёмка: сценарии 2 и 6 (контракт) повторены на staging и записаны. Сценарии 13 и 16 не обещаются.

Риски: attempt 1 CI показал, что зелёные jobs могут сосуществовать с failure всего run. Пилотный SHA нужно принимать только после полного графа.

Оценка трудозатрат: не даётся. Нет согласованной команды и оценки задач по людям. Диапазон без ресурсов был бы выдумкой.

## 29. Commercial MVP Plan

Scope: Minimal Pilot плюс закрытые деньги одного shipment (то, что обещает Draft #341), предсказуемый VAT/rounding, document writes с доверенной границей, tracking consumer без вечной блокировки партиции, shipper и carrier на реальном backend, backup/restore.

Не входит: forwarder portal, slot booking, юридический ЭДО-оператор, 30 000 перевозок в месяц, 24/7 Autopilot.

Critical path: #343 → #344 → money identity на `main` → staging с реальными двумя компаниями → только потом счёт.

Security и integration gates: все блокеры раздела 6. ERP остаётся вне MVP, пока нет живого адаптера.

Infrastructure: staging, затем отдельный production с секретами, мониторингом и restore drill.

Tests: CI 75 плюс один записанный staging сценарий 11–12 без утверждения, что акт юридически годен.

Приёмка: один грузовладелец и один перевозчик проводят перевозку и видят согласованную сумму. ЭДО-оператор не имитируется внутренней подписью.

Риски: внутренний signing recovery создаёт ложное ощущение юридического ЭДО.

Трудозатраты: `NOT_ESTIMABLE`.

## 30. Enterprise Roadmap

Scope поверх MVP: forwarder portal, slot booking, подключённые ERP/WMS/tracking/routing, юридический ЭДО, NLO production adapter после отдельного proof, аналитика OTIF/POD, RLS или доказанный эквивалент, нагрузочное испытание на целевые объёмы, incident response.

Последовательность: безопасность границ → деньги и документы → tracking production → порталы ролей → слоты → внешние провайдеры → оптимизатор в исполнении → масштаб и Autopilot только после cost gates Issue #313.

24/7 Autopilot не является ранним этапом.

Трудозатраты: `NOT_ESTIMABLE`.

## 31. Top Risks and Dependencies

1. Неполный граф CI может снова пометить `main` красным без падения тестов. Зависимость от GitHub Actions, не от продукта.
2. Document writes и tracking consumer блокируют внешний запуск независимо от процента модулей.
3. Финансы и ЭДО выглядят «почти готовыми» из-за сервисов и подписей, но юридический и денежный контур не на `main`.
4. NLO и slot facts легко перепутать с исполнением перевозки и записью на склад.
5. Отсутствие платформенного RLS в миграциях не равно доказанной дыре и не равно доказанной изоляции.
6. Draft PR не являются функциями `main`. Особенно #340, #341, #346, #329, #335.
7. `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` остаётся OPEN. Это отдельный review границы `X-Tenant-ID` в shipment-service. Эксплуатация не воспроизводилась. Риск не складывается с Issue #343 и не заменяет вопрос RLS. До отдельного review он держит external pilot и production открытыми по этой границе.

## 32. Final Controller Recommendations

1. Не запускать коммерческих клиентов на текущем `main`.
2. Не считать attempt 2 CI разрешением на production.
3. Не мержить этот отчёт автоматически. Это docs-only Draft.
4. Следующий инженерный этап выбирать отдельно: либо #343, либо #344, либо money identity. Не все сразу и не Autopilot.
5. Полевой staging smoke — только новым разрешением, без SSH из этого аудита.
6. `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` вынести в отдельный security review. Это не подтверждённая эксплуатация и не замена Issue #343 или проверки RLS.

## Evidence register

| Утверждение | Источник | Дата |
|---|---|---|
| main SHA | `git rev-parse origin/main` = `f58f13b5b4b539895673a77ff404ab47669dc96d` | 2026-11-09 |
| PR #342 merged | `gh pr view 342` в предыдущей сессии того же SHA; merge commit message на HEAD | 2026-11-09 |
| CI attempt 2 success, 75 jobs | run `42671916835`, head SHA тот же | 2026-11-09 |
| Contract test PASS | job `129357503754`, лог `--- PASS: TestStopEventTrackingContract09B` | 2026-11-09 |
| 17 services, 7 apps, 22 go.mod, 105 migrations | worktree inventory | 2026-11-09 |
| No platform RLS statement | grep `ENABLE ROW LEVEL SECURITY` в `infrastructure/migrations` — нет совпадений | 2026-11-09 |
| Consignee and finance skeletons | `apps/web-consignee/pages/index.vue`, `apps/web-finance/pages/index.vue` | 2026-11-09 |
| Forwarder absent | `docs/architecture/customer-portals/CUSTOMER_PORTALS_MAIN_BASELINE_0_2.md` и отсутствие каталога | 2026-11-09 |
| Issues #343 #344 #345 | `gh issue view` | 2026-11-09 |
| PR #320 merged dry-run | `gh pr view 320` | 2026-11-09 |
| PR #340 #341 #346 draft | `gh pr list` | 2026-11-09 |
| Open PR author snapshot 17 | `gh pr list` до ревью PR #347 | 2026-11-09 |
| Open PR independent snapshot 18 | ревью зафиксировало 18 из-за PR #347 | 2026-11-09 |
| Open PR R1 recheck 18 | `gh pr list --state open`, длина 18, включает #347 | 2026-11-09 |
| Shipment tenant header | `services/shipment-service/internal/http/handlers/tenant.go` `resolveVerifiedTenant`; `successor_revision_handler.go`; `internal/http/userauth/jwt.go` | 2026-11-09 |
| R1 terminology | экспертные F не менялись; 610/14 = 43.6 иллюстративно; blended 475/14 = 34 | 2026-11-09 |
| JWT issuance | `services/identity-service/internal/service/auth_service.go` | 2026-11-09 |
| VAT basis constant | `services/payment-service/internal/domain/outbox.go` | 2026-11-09 |
| CT view equals mutation | `docs/architecture/tms/TMS_STOP_EVENT_TRACKING_CONTRACT_0_9B.md` | 2026-11-09 |
| GAP-B-001 open | `docs/architecture/edo-document-completeness-source-v0.4b.md` | 2026-11-09 |
| Slot booking out of scope | `docs/engineering/parallel/pilots/control-tower-slot-intelligence-v0.7.2/ARCHITECTURE.md` | 2026-11-09 |
| Competitor columns | `EXTERNAL_COMPARISON_NOT_VERIFIED`. URL в R1 не восстанавливались | 2026-11-09 |
| Live servers | не проверялись | `NOT_VERIFIED` |
| Local tests this audit | не запускались | `NOT_RUN` |

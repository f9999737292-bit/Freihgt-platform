# BINTRANS Core MVP и Full Platform — решение 2026-11-10

STAGE: `BINTRANS-CORE-VS-FULL-READINESS-0.2A`  
BASE: `9d577fe1459a715a076ba065568637300b2e40da`  
CORE: `docs/reports/BINTRANS_CORE_MVP_READINESS_2026_11.md`  
FULL: `docs/reports/BINTRANS_FULL_PLATFORM_READINESS_2026_11.md`

Проценты — `EXPLICIT_EXPERT_ESTIMATE`, кроме клеток `NOT_ESTIMABLE`. 34% полной платформы не являются оценкой Core MVP. 37.6% Core MVP не являются оценкой полной платформы.

## 1. Сравнение

| Показатель | Core MVP | Full Platform |
|---|---|---|
| Functional implementation | `51_PERCENT` среднее F пяти направлений | `43.6_PERCENT` среднее F четырнадцати, `ILLUSTRATIVE_ONLY` |
| UI completeness | `41_PERCENT` | `27.1_PERCENT` среднее колонки UI, `ILLUSTRATIVE_ONLY` |
| Integration readiness | `38_PERCENT` | `NOT_ESTIMABLE` (иллюстрация колонки I: 34.3) |
| Security readiness | `27_PERCENT` | `NOT_ESTIMABLE` (иллюстрация колонки S: 29.3) |
| Operational readiness | `12_PERCENT` | `NOT_ESTIMABLE` (иллюстрация колонки O: 16.8) |
| Live E2E verified | `0` из 14 шагов пути | `0` из 16 сценариев |
| Pilot readiness | `NO` для полноценного пилота. Pilot A только `CONDITIONAL` | `NO` |
| Commercial readiness | `NO` | `NOT_ESTIMABLE` и `COMMERCIAL_LAUNCH_READY=NO` |
| Main blockers | Tenant header OPEN, Issue #343, Issue #344, backup `NOT_VERIFIED`, стенд `NOT_VERIFIED` | Те же границы плюс деньги не на `main`, юридический ЭДО не начат, провайдеры не подключены, 24/7 запрещён |
| Remaining work | P0 и P1 из раздела 4 | P3 полной платформы. Коммерческий объём `NOT_ESTIMABLE` |
| Estimated effort | P0: 23–56 человеко-дней. P1: 24–56 человеко-дней. Это не календарь | `NOT_ESTIMABLE` |

Сводка Core при равных весах направлений: `37.6_PERCENT`. При `PROPOSED_WEIGHTS_NOT_APPROVED`: `39.675_PERCENT`. Сводка Full при равных весах 14 модулей: `34_PERCENT`, без изменения после docs-only merge.

`OFFICIAL_READINESS_STATUS=NOT_ESTIMABLE` для обеих рамок.

## 2. Рекомендация

Минимальный объём, который нужно закончить, чтобы безопасно провести первый полноценный пилот, — это Pilot B, не процент и не вся платформа.

Нужно закрыть до приёмки:

1. Review границы `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` с картой маршрутов. Обход tenant isolation не предлагается.
2. Границу записи document-service, Issue #343, до любой загрузки POD.
3. Классификацию постоянных ошибок Kafka, Issue #344, до приёмки живого tracking.
4. Доказательство, что компания грузовладельца и компания перевозчика не читают чужие данные.
5. Backup и restore стенда, на котором лежат данные пилота, и зафиксированный migration head.
6. Один живой прогон пути: вход, выбор компании, тендер в procurement UI, ставка перевозчика, award, transport order, shipment, назначение, pickup, delivery, чтение tracking, POD, завершение. Mock Playwright этот пункт не закрывает.

Пока пункты 1–6 не выполнены, решение по полноценному пилоту — `NO-GO`.

Контролируемый технический пилот на изолированном стенде с синтетическими данными можно готовить параллельно. Сегодня и он `CONDITIONAL`, не GO: стенд этой оценкой не проверялся.

В минимальный объём не входят slot booking, Autopilot 24/7, network optimizer, consignee, forwarder, finance UI и юридический оператор ЭДО. Они не нужны, чтобы проверить основной процесс. Они нужны для обещания полной платформы и для коммерческого счёта.

## 3. Три сценария

### Pilot A — Controlled Technical Pilot

Один грузовладелец на тестовой учётной записи, ограниченный список перевозчиков, синтетические или письменно разрешённые тестовые данные. Цель — проверить процесс. Коммерческая готовность не утверждается.

Обязательные функции: login, выбор компании, создание RFx в `web-procurement` (shipper UI тендер не создаёт), ставка в carrier portal, award, transport order, shipment, назначение перевозчика, событие pickup и delivery, чтение tracking, запись POD через уже проверенную границу, статус завершения.

Допустимые исключения: native `driver-mobile` и PR #13; consignee; forwarder; slot booking; optimizer; акты и платежи; юридический ЭДО; Autopilot; боевой tracking provider, если событие пишет сам стенд и это написано в протоколе.

Запрещённые компромиссы: отключить проверку tenant; принять клиентский `X-Tenant-ID` как доверенный без review; открыть document write в интернет до Issue #343; считать mock Playwright приёмкой; считать unit test сквозным путём; удалять тома как способ «починить» стенд.

Launch blockers: три открытые границы, отсутствие backup, непроверенный стенд.

Минимальная инфраструктура: PostgreSQL, Kafka или Redpanda, identity, gateway, rfx, transport-order, shipment, tracking, document, `web-procurement`, `web-shipper`, `web-carrier`. Объектное хранилище, если POD пишет файл. CPU и RAM `NOT_VERIFIED`.

Обязательные тесты: существующие jobs award-to-transport-order, order-execution и контракт stop-event на SHA стенда; плюс один живой прогон 14 шагов. Зелёный CI без живого прогона недостаточен.

Приёмка: протокол с SHA, migration head, именами компаний-фикстур и результатом каждого шага. Секреты в протокол не класть.

Решение: `CONDITIONAL`.

### Pilot B — Real Customer Pilot

Настоящие участники и настоящие перевозки. Это и есть первый полноценный пилот из рекомендации.

Обязательные функции: всё из Pilot A без исключения tracking и POD, плюс авторизация компаний, аудит действий award и POD, контролируемая инфраструктура, обработка персональных данных водителя и контактов.

Документы: внутренний POD допустим только после закрытия Issue #343. Юридически значимый ЭДО можно исключить письменным решением заказчика. Это исключение не разрешает открытые write routes.

Tracking: Issue #344 закрыт, провайдер либо подключён, либо события идут из согласованного приложения, и это сказано в договоре пилота.

Персональные данные: список полей, срок хранения, кто имеет доступ. Оценка юридического текста здесь `NOT_ESTIMABLE`.

Допустимые исключения: слоты, оптимизатор, Autopilot, finance UI, второй грузовладелец.

Запрещённые компромиссы: те же, что в Pilot A, плюс синтетическая подмена реальной перевозки в отчёте приёмки и общий логин на несколько компаний.

Launch blockers: все P0 и P1. `CT_MUTATION_EQUALS_VIEW`, если оператор платформы работает в пилоте.

Инфраструктура: как Pilot A, плюс подтверждённый backup, отдельные секреты, запрет production-кластера, доступ только приглашённых компаний.

Тесты: набор Pilot A и проверка межкомпанийского отказа доступа.

Приёмка: реальная перевозка доходит до согласованного статуса, POD читается той компанией, которой он принадлежит, чужая компания получает отказ, журнал backup существует.

Решение: `NO-GO`.

### Commercial MVP

Первый платящий клиент.

Обязательные функции: Pilot B, плюс идентичность денег на `main` (сейчас Draft PR #341), явное решение NET/GROSS и VAT, документ, который можно отдать клиенту, и дежурство.

Допустимые исключения: Autopilot, slot booking, forwarder, если договор их не обещает.

Запрещённые компромиссы: выставить счёт из скелета `web-finance`; считать `TaxBasisWithVAT` закрытой коммерческой моделью; включить 24/7, чтобы заменить поддержку.

Launch blockers: всё из Pilot B, PR #341 не на `main`, GAP-B-001, backup, непроверенные секреты.

Инфраструктура: как Pilot B, плюс хранение финансовых документов и контур поддержки. Размер контура `NOT_ESTIMABLE`.

Тесты: Pilot B и один денежный сценарий на `main`, не на ветке Draft.

Приёмка: клиент платит по сумме, которую можно проследить от перевозки до закрытия, и документ соответствует обещанию договора.

Решение: `NO-GO`.

## 4. Критический путь

```text
P0-001 tenant header review
P0-002 Issue #343
P0-003 Issue #344
        \        |        /
         \       |       /
          P0-004 backup
          P0-005 isolated stand and migration head
                    |
                 P0-006 one live 14-step run
                    |
                 P1 company auth, assignment, POD upload, personal data, portal permissions, s2s
                    |
                 Pilot B acceptance
                    |
                 P2 money identity, VAT decision, EDO or written exclusion, support
                    |
                 Commercial MVP
```

P0-001, P0-002 и P0-003 можно вести параллельно. P0-006 стоит после них и после стенда. P1-004 (живая запись POD) зависит от P0-002. Календарные даты из этой схемы не выводятся.

`CRITICAL_PATH_DEFINED=YES`.

## 5. Backlog P0 и P1

Предпосылка диапазона: исполнитель уже знает репозиторий; review читает gateway и middleware, это не pentest; исправление предполагается локальным. Если review обнаружит сквозную смену контракта, диапазон недействителен. Человеко-дни не складываются в срок.

| TASK_ID | MODULE | DESCRIPTION | CURRENT_EVIDENCE | DEPENDENCIES | ACCEPTANCE_CRITERIA | RECOMMENDED_AGENT | PR_OR_ISSUE | PRIORITY | EFFORT_RANGE | CONFIDENCE |
|---|---|---|---|---|---|---|---|---|---|---|
| CORE-P0-001 | TMS | Карта маршрутов shipment и review `X-Tenant-ID` | `tenant.go`, комментарий successor handler, `jwt.go` не читает заголовок | Gateway | Для каждого write/detail маршрута назван доверенный вызывающий. Прямой доступ либо закрыт, либо признан | security | риск в отчёте #347, отдельного issue нет | P0 | 5–12 | LOW |
| CORE-P0-002 | EDO / documents | Закрыть прямые write routes | Issue #343 OPEN. Signing recovery 403 не закрывает остальные writes | Нет | Внешний вызов write без доверенной границы получает отказ. POD пилота ходит только в принятый путь | security + documents | #343 | P0 | 5–15 | MEDIUM на факт, LOW на размер |
| CORE-P0-003 | Tracking | Классифицировать постоянную ошибку inbox sequence | Issue #344. PR #346 только контракт | Consumer на `main` | Постоянная ошибка не крутит партицию бесконечно. Контрактный тест остаётся зелёным | tracking | #344, #346 Draft | P0 | 3–8 | MEDIUM |
| CORE-P0-004 | Operations | Backup и restore стенда пилота | В принятом аудите `NOT_VERIFIED` | Выделенный стенд, не production | Есть свежий backup и проверенное восстановление до записи пилотных данных | ops | нет | P0 | 3–6 | LOW |
| CORE-P0-005 | Operations | Зафиксировать изолированный стенд и migration head | Git head `000105`. Живая база `NOT_VERIFIED` | Разрешение на стенд | Head записан, production не используется, тома не удаляются | ops | нет | P0 | 2–5 | LOW |
| CORE-P0-006 | Path | Один живой прогон 14 шагов | Сейчас `CORE_E2E_VERIFIED=0`. CI jobs частичные | P0-001…005 | Протокол по каждому шагу на одном SHA | QA + TMS | jobs award, order-execution, stop-event | P0 | 5–10 | LOW |
| CORE-P1-001 | Portals | Межкомпанийский отказ | UI выбора есть, проверка `NOT_VERIFIED` | Identity и company | Компания A получает отказ на объект компании B | security | нет | P1 | 3–8 | LOW |
| CORE-P1-002 | TMS | Явный переход назначения перевозчика | Отдельный job не найден | Shipment и carrier | Тест или протокол называет assignment | TMS | нет | P1 | 4–10 | LOW |
| CORE-P1-003 | Tracking | Водитель и ТС без обязательного store release | `driver-mobile` без spec. PR #13 OPEN | Можно заменить fleet UI | Либо назначение из carrier portal доказано, либо native app исключена письменно | carrier portal | #13 не обязателен | P1 | 3–8 | LOW |
| CORE-P1-004 | Documents | Живая запись POD | Миграция `000099`, UI показывает список | P0-002 | Файл виден владельцу и не виден чужой компании | documents | #343 | P1 | 4–8 | LOW |
| CORE-P1-005 | Privacy | Учёт персональных полей водителя | Поля есть в домене перевозки. Юридический текст не оценивался | Pilot B | Список полей и доступ. Юридическое заключение отдельно | privacy + TMS | нет | P1 | 2–4 на инженерную опись. Юридический текст `NOT_ESTIMABLE` | LOW |
| CORE-P1-006 | Portals | Матрица shipper, carrier, procurement | Три приложения есть | P1-001 | Роль не открывает чужой контур | portal QA | нет | P1 | 3–6 | LOW |
| CORE-P1-007 | Platform | S2S для сервисов пути | PR #336 закрыл analytics | Gateway, rfx, shipment, tracking, document | Вызов без caller identity отвергается на этих сервисах | security | #336 merged, полнота нет | P1 | 5–12 | LOW |

Сумма нижних границ P0: `5+5+3+3+2+5 = 23`. Верхних: `12+15+8+6+5+10 = 56`.  
Сумма нижних границ P1: `3+4+3+4+2+3+5 = 24`. Верхних: `8+10+8+8+4+6+12 = 56`. Инженерная опись P1-005 взята как 2–4; юридический текст в сумму не входит.

`ESTIMATED_REMAINING_EFFORT` до безопасного полноценного пилота: P0+P1, 47–112 человеко-дней при последовательном сложении, confidence LOW. До коммерческого MVP и до полной платформы: `NOT_ESTIMABLE`.

## 6. Evidence register

| Утверждение | Источник | Дата |
|---|---|---|
| SHA и docs-only diff | worktree на `9d577fe1` | 2026-11-10 |
| CI | run `42701751576` | 2026-11-10 |
| Открытые PR 17 и Issues 6 | `gh pr list`, `gh issue list` | 2026-11-10 |
| Баллы Core | `BINTRANS_CORE_MVP_READINESS_2026_11.md`, арифметика в файле | 2026-11-10 |
| Баллы Full | принятая матрица, пересчитанные суммы колонок | 2026-11-10 |
| Mock E2E | spec-файлы shipper и carrier | 2026-11-10 |
| Draft не на main | #340, #341, #346 | 2026-11-10 |
| Локальные тесты и серверы | не запускались и не опрашивались | `NOT_RUN` / `NOT_VERIFIED` |

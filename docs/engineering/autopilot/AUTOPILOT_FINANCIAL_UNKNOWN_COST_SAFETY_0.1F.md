# BINTRANS Autopilot — неизвестная стоимость не равна нулю 0.1F

AGENT=G
STAGE=AUTOPILOT-FINANCIAL-UNKNOWN-COST-SAFETY-0.1F
MODE=READ_ONLY_DISCOVERY
BASE_SHA=`db5e77e2c86af4dfdbe498a85f68c54e9518a916`
BASE=`origin/main` (merge PR #322)
BT_01B_BUSINESS_APPROVAL=NOT_VERIFIED
CODE_OR_CONFIG_CHANGED=NO
PAID_LAUNCHES=EXCLUDED
MERGE_OR_DEPLOY=NO

Техническое задание на устранение ложного нуля. Суммы лимитов не назначаются. BT-01b не утверждается: состояние `OVERRUN` в коде остаётся правилом учёта, не согласием владельца платить превышение.

---

## 1. Где отсутствующий `chargedCents` становится 0

Подтверждены два места. Остальные читатели либо сохраняют отсутствие, либо заранее закрывают запуск по нулевому лимиту.

| ID | Место | Поведение | Ложный ноль |
| --- | --- | --- | --- |
| F-1 | `tools/bintrans_autopilot/cursor_api.py`, `CursorClient.cloud_charged_cents` | `total += float(event.get("chargedCents") or 0)` | Да. Нет ключа, `null` и пустое значение дают вклад 0. Настоящий `0` тоже ложен в Python (`0 or 0`), поэтому сумма не отличает «счёта нет» от «счёт 0» |
| F-2 | `autopilot_queue._budget_charged_cents` в миграции `0005`, функция позже не заменена | `NULL` или пустой текст возвращает `0`, затем `CEIL` для непустого числа | Да, если помощник вызван с пустым текстом. Живой `budget_reconciler_apply_usage` миграции `0015` до вызова помощника поднимает `unknown_usage` и откатывает транзакцию. Дыра остаётся в самом помощнике и в `_budget_commit_from_verified`: тот проверяет `charged_known`, но пустой текст при `charged_known=true` прошёл бы в помощник и стал бы проведённым нулём |

Не являются превращением отсутствующего расхода в ноль:

- `parse_usage_payload` / `_optional_amount`: нет ключа, `None` и пустая строка → `charged_known=false`. Число `0` → текст `"0"` и `charged_known=true`. Выражение `text or "0"` срабатывает только после успешного разбора числа, когда хвост нулей стёрт до пустой строки. Это запись настоящего нуля, не подстановка вместо отсутствия.
- `usage_journal._amount`: нет ключа или `null` → `None`. Повтор той же пары agent/run с другим payload → `conflict`, не перезапись нулём. Журнал не является живым stop.
- `assess_budget`: дневной или месячный потолок `<= 0` возвращает `budget_not_authorized` до вызова суммы. На `origin/main` оба потолка равны 0, а `load_config()` держит `test_budget_harness=false`, поэтому следом всё равно будет `provider_spend_cap_unconfirmed`. Это закрытый запуск, а не доказательство, что F-1 безопасен. Как только наблюдение суммы начнут считать, F-1 занизит её.
- `scheduler._paid_block`: `observed_charged_cents is None` при ненулевом бюджете даёт `unknown_spend`, не ноль. Живой scheduler передаёт `observed_charged_cents=None`.
- Диспетчер, `budget.py` и scheduler не читают `usage_unknown`.

---

## 2. Настоящий ноль и отсутствие данных

| Ситуация | Ledger | Сумма диспетчера |
| --- | --- | --- |
| Ключ `chargedCents` отсутствует, `null` или пустая строка | `charged_known=false`, состояние остаётся `RESERVED`, флаг `usage_unknown` | Вклад 0 в сумму (F-1) |
| Токены `GET /v1/agents/{id}/usage` без `chargedCents`, в том числе нулевые токены «ещё не записано» | То же, неизвестно. RT-39 | Эти токены в сумму не входят; официальный ответ usage не содержит `chargedCents` |
| Число `0` в payload, записанное reconciler | `charged_known=true`, текст `"0"`, состояние `COMMITTED`, если 0 не выше резерва. Это не `RELEASED` | Вклад 0, неотличим от отсутствия |
| `charged_known=false` | Проводка запрещена: `_budget_commit_from_verified` поднимает `unknown_usage` | Не используется |

`TRUE_ZERO_DISTINGUISHED=LEDGER_YES_SUM_NO`.

Настоящий ноль в ledger — только явная десятичная величина и `charged_known=true`, с строкой в `budget_verified_usage`. `RELEASED` таким нулём не является.

---

## 3. Соответствие PostgreSQL ledger

Ledger не подставляет ноль вместо неизвестной суммы на пути `budget_reconciler_apply_usage` (миграция `0015`):

- нет подтверждённого запуска `LAUNCH_CONFIRMED` или `EXTERNAL_AGENT_CANCELLED` — проводки нет;
- `charged_known=false` — строка остаётся `RESERVED`, `usage_unknown=true`, окна не получают committed;
- пустой текст при `charged_known=true` — `unknown_usage`, откат;
- число проводится через `CEIL` в счётчики, точный текст сохраняется отдельно.

Расхождение с ledger — F-1: stop диспетчера считает другую книгу и обнуляет дыры. F-2 не совпадает с контрактом ledger «пустой текст — не ноль», хотя текущий вызов из `0015` до помощника не доходит.

---

## 4. Может ли неизвестная стоимость освободить резерв

`RESERVATION_RELEASE_RISK=YES_UNPROVEN_RELEASE_BOOKS_ZERO_COMMITTED`.

Что удержано:

- Runtime `budget_protocol_release` идёт через `_budget_assert_release_allowed`: `usage_unknown`, любой launch intent, связанный `agent_id`/`run_id` или строка `budget_verified_usage` запрещают release.
- Пока флаг `usage_unknown` установлен, `budget_global_safety_block_reason()` возвращает `unknown_usage_open`.

Что освобождает без доказательства нулевого счёта:

`_budget_release_unused_reserve` (миграция `0010`, позже не заменена) не смотрит на `usage_unknown`. Для состояния `RESERVED` оно ставит `RELEASED` и вызывает `_budget_window_apply` с committed-дельтой `0, 0`. Это возврат резерва, но в окне появляется нулевое начисление.

Вызовы, которые доходят до этого помощника:

- отмена до отправки, только из `INTENT`;
- `launch_protocol_reject_definite` из `SEND_STARTED` (миграция `0012`);
- ручной reject из `MANUAL_RECONCILIATION_REQUIRED` (миграция `0010`).

Флаг `usage_unknown` ставится только из подтверждённого или внешне отменённого запуска, поэтому эти три перехода в нормальной матрице с флагом не пересекаются. Помощник всё равно не защищён: прямой вызов на `RESERVED` + `usage_unknown` освободит резерв. Ручной reject и definite reject освобождают резерв без строки verified usage. Для них ноль в окне — недоказанный ноль. Поздний счёт после `RELEASED` в ledger не возвращается (см. ниже). ADR прямо говорит, что `RELEASED` не доказывает расход.

---

## 5. Новый запуск при незакрытой неизвестной стоимости

`NEW_LAUNCH_BLOCKING=LEDGER_BLOCKS_DISPATCHER_DOES_NOT_READ_LEDGER`.

- Новый `budget_protocol_reserve` блокируется, пока есть любой `usage_unknown`, любой `OVERRUN` или launch в `LAUNCH_UNKNOWN` / `MANUAL_RECONCILIATION_REQUIRED`. Python kill switch повторяет эти причины и пишет `BLOCKED`.
- `dispatcher.py`, `budget.py` и scheduler это состояние не читают. Их отказ на `origin/main` держится на других гейтах: `autopilot_enabled=false`, нулевые потолки, `provider_spend_cap_unconfirmed`, пустая цена модели. Это не связь с открытым `usage_unknown`.
- Платный запуск этим заданием не открывается. Требование на будущее: диспетчер не имеет права считать молчание своей суммы разрешением, если ledger ему недоступен.

---

## 6. Запоздавшие billing events

`LATE_BILLING_EVENTS=HELD_OR_ORPHAN_NOT_FOLDED_INTO_LEDGER`.

- Почасовая агрегация Admin API описана в клиенте. Живой `CursorClient` нигде не ставит `spend_events_lagged`. Атрибут читается только в `assess_budget`, и только тесты включают его. Реальное наблюдение поэтому не помечается как неполное. Решение о запуске может увидеть ещё не доехавший час. Сейчас следом срабатывает `provider_spend_cap_unconfirmed`, потому что harness выключен в загрузчике.
- Первая запись `usage_unknown`, затем payload с уже известной суммой: строка `budget_verified_usage` уже есть, поля расходятся → `idempotency_conflict`. Резерв не отпускается и не становится нулём. Сумма в ledger тоже не попадает. Новые резервы остаются закрыты флагом.
- Уже `COMMITTED` или `OVERRUN`, затем другая сумма: тот же `idempotency_conflict`. Повторной прибавки нет. Описанный в ADR поздний пересчёт BT-02 / BT-03 этим конфликтом не выполняется. Это наблюдение кода, не утверждение BT-01b.
- После `RELEASED` поздняя сумма не проводится. `budget_orphan_spend_signal` только пишет сигнал. Окно по-прежнему содержит committed 0 от release.

---

## 7. Повтор одного billing event

`DUPLICATE_BILLING_EVENTS=LEDGER_IDEMPOTENT_SUM_NOT_DEDUPED`.

- Тот же `verification_id`, дайджест и та же сумма: вторая запись не вставляется; если состояние уже `COMMITTED`/`OVERRUN` и текст совпал, `_budget_commit_from_verified` выходит без новой дельты окна.
- Тот же agent/run в журнале с тем же payload: `unchanged`. С другим payload: `conflict`.
- `cloud_charged_cents` не имеет ключа события. Повтор объекта на следующей странице прибавит `chargedCents` ещё раз. Одинаковые настоящие события на одной странице тоже нельзя схлопывать по величине: у ответа нет отдельного id, и два равных начисления различить нечем. Исправление не должно вычитать дубликаты только по сумме.

---

## 8. Уже запущенный Cloud Agent после emergency stop

`ESTOP_EXISTING_RUN=CONTINUES_CANCEL_NOT_CALLED`.

- Новый вызов отсекается в `runtime_enabled` (`config.py`) и в `_kill_switch_for_cursor`: env `AUTOPILOT_EMERGENCY_STOP`, флаг конфига или файл `tools/bintrans_autopilot/EMERGENCY_STOP`.
- `Dispatcher.run` при выключенном runtime возвращается сразу. Уже записанные `agent_id` / `run_id` не отменяются.
- `CursorClient.cancel_run` — единственное определение `POST /v1/agents/{id}/runs/{runId}/cancel`. Вызовов нет ни в диспетчере, ни в workflow. `EMERGENCY_STOP.md` это фиксирует: stop не отменяет run, отмена терминальна.
- Начисление уже идущего run может прийти следующим часом и попасть под F-1, если поля ещё нет, либо остаться вне ledger, если резерв уже `RELEASED`.

---

## 9. Подтверждённые находки

```text
CONFIRMED_FINDINGS=F-1,F-2,F-3,F-4,F-5,F-6,F-7
```

| ID | Находка |
| --- | --- |
| F-1 | Сумма `chargedCents` подставляет 0 вместо отсутствия и не отличает его от настоящего 0 |
| F-2 | `_budget_charged_cents` возвращает 0 на `NULL` и пустую строку |
| F-3 | Release неиспользованного резерва пишет committed 0 и не проверяет `usage_unknown` |
| F-4 | Поздняя известная сумма после `usage_unknown` или после уже закрытой проводки конфликтует и в ledger не входит; после `RELEASED` остаётся только сигнал |
| F-5 | Сумма диспетчера не дедуплицирует события; ledger и журнал дедуплицируют одинаковый payload |
| F-6 | Диспетчер не читает `unknown_usage_open`. Текущий запрет запуска держится на других гейтах |
| F-7 | Emergency stop не вызывает `cancel_run`. Уже созданный run продолжается |

---

## 10. Минимальные исправления

Лимиты не задаются и не повышаются. Флаги включения, секреты, workflow и платные запуски не меняются. BT-01b не объявляется принятым.

| ID | Правка | Запрещено этой правкой |
| --- | --- | --- |
| FIX-1 | В `cloud_charged_cents` отсутствие ключа, `null` и пустая строка — ошибка `spend_amount_absent`, вся сумма отбрасывается. Число, включая 0, складывается через `Decimal`. `or 0` удаляется. `assess_budget` уже переводит `CursorApiError` в `spend_api_error` с сохранением состояния задач | Не подставлять 0 и не пропускать отдельное событие молча |
| FIX-2 | `_budget_charged_cents` на `NULL` и пустой текст поднимает `invalid_amount`. Текст `0` по-прежнему даёт bigint 0 | Не проводить пустой текст как `COMMITTED` |
| FIX-3 | `_budget_release_unused_reserve` при `usage_unknown` поднимает `unknown_usage_open` и не меняет состояние и окна | Не считать `RELEASED` подтверждённым нулевым счётом |
| FIX-4 | Пока нет отдельного преемника payload, повтор с другой суммой остаётся `idempotency_conflict`: без второй дельты окна, без перевода в 0 и без release. Преемник, если его будут делать позже, обязан в одной транзакции заменить неизвестное на явную сумму и не имеет права выбрать 0 из пустого поля. Это задание его не реализует и не утверждает оплату превышения | Не лечить конфликт записью нуля |
| FIX-5 | До любого будущего платного dispatch диспетчер обязан закрываться, если не может прочитать `budget_global_safety_block_reason()`. Этот патч соединение не включает | Не считать текущие нулевые лимиты заменой этой связи |
| FIX-6 | Обработчик emergency stop для уже известной пары agent/run вызывает `cancel_run` и не считает невызванную отмену остановкой счёта. Создание агентов не добавляется | Не включать autopilot ради проверки отмены |
| FIX-7 | Живой клиент не должен оставлять `spend_events_lagged` ложным по умолчанию, пока ответ не доказывает закрытый час. Без доказательства `assess_budget` остаётся на `spend_observation_stale` и сохраняет задачи | Не выдумывать свежесть часа |

Порядок минимального патча против ложного нуля: FIX-1, FIX-2, FIX-3 и тесты к ним. FIX-4 фиксирует текущий безопасный отказ и запрещает ложное лечение. FIX-5, FIX-6 и FIX-7 обязательны до финансового допуска и не входят в патч, который только убирает подстановку нуля.

---

## 11. Негативные тесты

Существующие проверки настоящего нуля и токенов без суммы сохраняются. Добавить следующие. Все на подставных payload и изолированной тестовой базе. Сеть Cursor и платный запуск не используются.

| ID | Вход | Ожидание |
| --- | --- | --- |
| T-1 | Событие без ключа `chargedCents` | `spend_amount_absent`, сумма не возвращена, задачи не переведены в `BUDGET_HOLD` |
| T-2 | `chargedCents: null` | То же, что T-1 |
| T-3 | `chargedCents: ""` | То же, что T-1 |
| T-4 | Страница из числа 0 и события без поля | Ошибка на всё наблюдение, а не сумма одних нулей |
| T-5 | Единственное событие `chargedCents: 0` | Сумма 0 как наблюдение; в ledger отдельно: `charged_known=true`, состояние `COMMITTED`, не `RELEASED`, `usage_unknown=false` |
| T-6 | Payload только с токенами | `usage_unknown`, резерв `RESERVED`, committed-окно не выросло |
| T-7 | `usage_unknown`, затем runtime release | `unknown_usage_open`, состояние не `RELEASED` |
| T-8 | `usage_unknown`, затем `_budget_release_unused_reserve` | После FIX-3 тот же отказ; до патча тест фиксирует текущую дыру и должен сменить ожидание вместе с патчем |
| T-9 | `usage_unknown`, затем второй `request_reserve` другого launch | Отказ `unknown_usage_open`, новая строка не `RESERVED` |
| T-10 | Два одинаковых verification payload | Одна дельта окна |
| T-11 | Сначала неизвестно, затем другой `chargedCents` | `idempotency_conflict`; не 0, не `RELEASED`, флаг неизвестности сохраняется |
| T-12 | `COMMITTED`, затем большая сумма тем же launch | `idempotency_conflict`, окно не увеличено повторно |
| T-13 | `RELEASED`, затем ненулевая сумма | Сигнал сироты либо отказ проводки; committed-окно не увеличивается этой суммой |
| T-14 | Две страницы с одним и тем же объектом без отдельного id | Тест не требует схлопывания по величине. Он обязан показать, что отсутствие поля на второй странице отменяет всю сумму |
| T-15 | Emergency stop при уже записанных `agent_id` и `run_id` | Сейчас: `cancel_run` не вызван, новый `create_agent` не вызван. После FIX-6: отмена вызвана для этой пары, новый create по-прежнему не вызван |
| T-16 | `_budget_charged_cents(NULL)` и пустая строка | `invalid_amount`, не bigint 0 |
| T-17 | Диспетчер без чтения ledger при открытом `usage_unknown` | Характеризующий тест: текущий диспетчер ledger не спрашивает. До FIX-5 это не считается финансовым допуском |

---

## 12. Вердикт

```text
AGENT=G
STAGE=AUTOPILOT-FINANCIAL-UNKNOWN-COST-SAFETY-0.1F
BASE_SHA=db5e77e2c86af4dfdbe498a85f68c54e9518a916
UNKNOWN_COST_ZERO_FALLBACK=YES_F1_SUM_AND_F2_SQL_HELPER
TRUE_ZERO_DISTINGUISHED=LEDGER_YES_SUM_NO
RESERVATION_RELEASE_RISK=YES_UNPROVEN_RELEASE_BOOKS_ZERO_COMMITTED
LATE_BILLING_EVENTS=HELD_OR_ORPHAN_NOT_FOLDED_INTO_LEDGER
DUPLICATE_BILLING_EVENTS=LEDGER_IDEMPOTENT_SUM_NOT_DEDUPED
NEW_LAUNCH_BLOCKING=LEDGER_BLOCKS_DISPATCHER_DOES_NOT_READ_LEDGER
ESTOP_EXISTING_RUN=CONTINUES_CANCEL_NOT_CALLED
CONFIRMED_FINDINGS=F-1,F-2,F-3,F-4,F-5,F-6,F-7
REQUIRED_FIXES=FIX-1,FIX-2,FIX-3,FIX-4,FIX-5,FIX-6,FIX-7
REQUIRED_TESTS=T-1..T-17
BT_01B_BUSINESS_APPROVAL=NOT_VERIFIED
FINANCIAL_READINESS=NOT_READY
FINAL_VERDICT=SPEC_ONLY_UNKNOWN_COST_NOT_CLOSED
```

Код, конфигурация, GitHub и платёжные настройки не изменялись. Платные запуски не включались. Merge и deployment не выполнялись.

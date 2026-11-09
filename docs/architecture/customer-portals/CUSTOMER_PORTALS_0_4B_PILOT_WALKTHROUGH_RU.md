# Customer Portals 0.4B — операторский сценарий пилота (RU)

Локальная демонстрация двух кабинетов BINTRANS с mock API (Playwright) или с тем же потоком на staging **только после** отдельного разрешения оператора. Пароли и токены в этом документе — **плейсхолдеры**; не использовать production.

**Предпосылки:** `pnpm --filter @freight-platform/web-shipper build` и `web-carrier build`; Playwright с `127.0.0.1:3001` / `3002`; шлюз — mock в E2E или согласованный demo API.

**Синтетические идентификаторы:** `tenant-1`, `co-shipper`, `co-carrier`, `co-consignee` (только в данных перевозки), `shp-1` / `shipment-A`, `TO-1`.

---

## A. Кабинет грузоотправителя (Shipper, :3001)

| Шаг | Действие | Ожидаемый результат | Проверка |
| --- | --- | --- | --- |
| A.1 | Открыть `/login`, ввести tenant `tenant-1`, email `shipper@example.com`, пароль (demo) | Форма на русском; переход после входа | Видны поля `tenant-id`, `email`, `password`, кнопка **Войти** |
| A.2 | Войти | Экран **Выберите компанию** | `data-testid=company-gate`, подпись **Кабинет грузоотправителя** |
| A.3 | Выбрать компанию `co-shipper`, **Продолжить** | Главная кабинета | `data-testid=home-shell`, навигация **Отправления** |
| A.4 | **Отправления** | Список с `SHP-1`, статус **BOOKED** | `shipment-inbox`, колонка статуса; в данных видны id перевозчика/получателя (`co-carrier`, `co-consignee`) без отдельного кабинета consignee |
| A.5 | Открыть отправление | Деталь: режим **ROAD**, плановые даты, статус | `shipment-detail`, `shipment-status` содержит BOOKED |
| A.6 | **Отслеживание** | Tracking **ACTIVE**, ETA pickup/delivery, слоты **Europe/Moscow** | `tracking-status`, `eta-pickup`, `slot-pickup`; нет сырых provider id в UI |
| A.7 | Документы по отправлению | Строка **DOC-1**, тип POD | Ссылки только `/api/v1/shipper/...` с `shipper_company_id=co-shipper` (в E2E — журнал mock) |
| A.8 | Переключение компании (если 2 shipper membership) | Список документов/отправлений сбрасывается под новую компанию | `company-switch`; прежние DOC-1 не видны для `co-shipper-b` |
| A.9 | Запрет чужой компании | Tamper / неверная компания → снова **company-gate**; нет вызовов с `co-spoof` | Playwright: `pilot-walkthrough` + полный `shipper-office` tamper |

**Автоматизация пилота:** `apps/web-shipper/e2e/pilot-walkthrough.spec.ts` — сжатый happy path (A.1–A.7).

---

## B. Кабинет перевозчика (Carrier, :3002)

| Шаг | Действие | Ожидаемый результат | Проверка |
| --- | --- | --- | --- |
| B.1 | `/login`, `tenant-1`, `carrier@example.com`, пароль (demo) | RU-форма входа | testid sign-in |
| B.2 | Войти, выбрать `co-1`, **Продолжить** | Главная, навигация **Заявки**, **Тендеры**, **Автопарк** | `home-shell` |
| B.3 | **Заявки** | Строка **TO-1**, статус **CONFIRMED** | `transport-orders`, ссылка на деталь |
| B.4 | Открыть заявку | Статус заказа, при наличии — shipment ref и ссылка tracking | `order-status`, `open-tracking` href `/shipments/.../tracking` |
| B.5 | **Отслеживание** (по shipment ref) | Tracking **ACTIVE**, ETA, окна | только `/api/v1/carrier/shipments/...?carrier_company_id=co-1` |
| B.6 | **Документы** по shipment (из списка заявок) | На экране документов по shipment отображается номер **DOC-1** | `data-testid=document-number` содержит `DOC-1` |
| B.7 | Deep link чужой перевозки | `/shipments/shipment-b/tracking` при выбранной `co-1` | Сообщение **Отслеживание недоступно для выбранной компании**; запросы с `carrier_company_id=co-1` |
| B.8 | Запрет чужой компании | Нет business API до выбора компании; tamper очищается после reload | `reloadAndExpectCompanyGate` в полном suite |

**Автоматизация Carrier**

- **PILOT SMOKE** — `apps/web-carrier/e2e/pilot-walkthrough.spec.ts`: happy path **B.1–B.6**, где **B.6 ограничен** переходом к документам по shipment и проверкой `document-number` = `DOC-1`; отдельный сценарий в том же файле покрывает **B.7** (реакция UI на mock 404 cross-shipment tracking).
- **OFFICE REGRESSION** — `apps/web-carrier/e2e/carrier-office.spec.ts`, тест `documents open from the server shipment reference`: `open-documents` href `/shipments/shipment-A/documents`; список и деталь **DOC-1**; строка вложения `pod.pdf`; клик **Скачать** инициирует mock-запрос `.../attachments/att-1/content` (проверка URL в журнале вызовов, не проверка бинарного скачивания файла); отсутствие `secret-bucket` / `secret-object-key` в UI.

В pilot smoke шаг **B.6** не проверяет вложение `pod.pdf` и не выполняет скачивание. Эти шаги относятся к office regression выше. Оба набора используют **mock API** и не подтверждают реальное object storage, backend tenant isolation или staging pilot.

---

## C. Согласованность пилота Shipper ↔ Carrier

| Тема | Ожидание |
| --- | --- |
| Consignee | Отображается как `co-consignee` в карточке отправления у shipper; **отдельного UI нет** |
| Tenant | Клиент не шлёт `X-Tenant-ID` / `X-User-ID`; tenant только в login body и query memberships |
| Локаль | UI по умолчанию **ru-RU** (`LocaleSwitcher` при необходимости) |
| Изоляция company | Все business GET/POST с выбранным membership company id в query + `X-Company-ID` |

---

## D. Ручной прогон vs CI

| Режим | Команда |
| --- | --- |
| Shipper pilot smoke | `pnpm --filter @freight-platform/web-shipper exec playwright test e2e/pilot-walkthrough.spec.ts` |
| Carrier pilot smoke | `pnpm --filter @freight-platform/web-carrier exec playwright test e2e/pilot-walkthrough.spec.ts` |
| Полный office regression | `pnpm --filter @freight-platform/web-shipper test:e2e` (и carrier) |

**Запрещено без отдельного approval:** staging writes, POST/PUT/DELETE на demo data (см. `LOW_CODE_PILOT_WEEK3_CONTROLLED_PILOT_TEST_PLAN_V0.1.md`).

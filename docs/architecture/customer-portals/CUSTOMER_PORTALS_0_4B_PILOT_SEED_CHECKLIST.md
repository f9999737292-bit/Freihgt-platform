# Customer Portals 0.4B — Pilot seed checklist (synthetic only)

Использовать **только синтетические** идентификаторы в mock E2E и в будущем demo seed. **Не коммитить** пароли, JWT, `DATABASE_URL`, staging `.env`.

## Tenant

| Поле | Значение (mock / план demo) | Статус E2E |
| --- | --- | --- |
| `tenant_id` | `tenant-1` | USED |
| Отображаемое имя | «ООО Bintrans Dev Tenant» (demo plan) | PLANNED |

## Companies

| Company id | Тип | Роль в пилоте | UI |
| --- | --- | --- | --- |
| `co-shipper` | SHIPPER | Основной shipper office | Shipper gate |
| `co-shipper-b` | SHIPPER | Второй shipper (switch) | Shipper gate (optional) |
| `co-carrier` | CARRIER | Указан на отправлении shipper | Данные на карточке |
| `co-consignee` | CONSIGNEE | Получатель на отправлении | **Нет отдельного UI** |
| `co-forwarder` | FORWARDER | Поле на отправлении | **Нет forwarder office** |
| `co-1` | CARRIER (office) | Основной carrier office | Carrier gate |

## Users (плейсхолдеры email)

| Email | Роли (JWT / membership) | Портал |
| --- | --- | --- |
| `shipper@example.com` | `SHIPPER_ADMIN` или `SHIPPER_LOGIST` | web-shipper |
| `carrier@example.com` | `CARRIER_DISPATCHER` или `CARRIER_ADMIN` | web-carrier |

Пароль в E2E: литерал `secret` вводится в форму; **не** сохранять в репозитории как production secret — только в локальном mock login handler.

## Shipment / order graph (mock)

| Сущность | Id | Связи |
| --- | --- | --- |
| Shipment | `shp-1` / `SHP-1` | `shipper_company_id=co-shipper`, `carrier_company_id=co-carrier`, `consignee_company_id=co-consignee`, status `BOOKED` |
| Transport order (carrier) | `order-1` / `TO-1` | `shipment_id` → `shipment-A` (carrier tracking path), status `CONFIRMED` |
| Cross-company probe | `shipment-b` | 404/forbidden для `co-1` — проверка изоляции |

## Documents

| Id | Shipment | Тип | Вложение |
| --- | --- | --- | --- |
| `doc-1` | `shp-1` / `shipment-A` | POD | `pod.pdf` (mock buffer) |

## Tracking / ETA / slots (mock payloads)

| Слой | Ключевые поля |
| --- | --- |
| Tracking | `trackingStatus=ACTIVE`, position 55.75 / 37.62, freshness `fresh` |
| ETA | pickup `2026-12-01T11:00:00Z`, delivery `2026-12-02T16:00:00Z` |
| Slots | timezone `Europe/Moscow`, pickup/delivery windows |

Provider-side ids (`provider-device-secret` и т.д.) **не должны** отображаться в UI.

## Memberships (server refresh)

| User | Memberships в mock login refresh |
| --- | --- |
| Shipper default | `m-1` → `co-shipper`, ACTIVE, `SHIPPER_ADMIN` |
| Shipper switch demo | + `co-shipper-b` |
| Carrier default | `m-1` → `co-1`, ACTIVE, `CARRIER_DISPATCHER` |

`selectedCompanyId` в storage после tamper должна сбрасываться после `GET .../users/.../companies`.

## Checklist перед демо оператору

- [ ] Сборки `web-shipper` и `web-carrier` актуальны на целевом SHA
- [ ] Playwright pilot smoke PASS
- [ ] RU: навигация **Отправления** / **Заявки** видна без en-US fallback на основных экранах
- [ ] Consignee **не** ожидается как отдельный вход
- [ ] Staging seed — **NOT_RUN** без operator approval (этот pack — локальный/mock)

## Запрещено в рамках 0.4B

- Создание данных в staging/production
- Реальные customer tenant id / email
- Новые публичные API или миграции БД

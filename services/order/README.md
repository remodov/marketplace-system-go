# order

Order Service из сквозного маркетплейс-кейса сайта [vikulin-va.ru](https://vikulin-va.ru/use-case-pattern/case/order-service/):
оформление заказов. Сервис ходит за ценами в соседний `catalog` и с восьмого шага практикума умеет
переживать его медленные ответы, сорванные соединения и полное отсутствие.

**Уровень 3** методологии Use Case Pattern: агрегат `Order` с позициями и правилами внутри, команда и
обработчик сценария с явными портами, выходной адаптер к каталогу с таймаутами, повтором и размыкателем.
С девятого шага создание заказа идемпотентно по заголовку `Idempotency-Key`, с десятого события уезжают соседям через outbox и Kafka по внешнему контракту из [`contracts/`](../../contracts/), с одиннадцатого заказ живёт по статусной модели, а отмена оплаченного заказа идёт сагой с возвратом денег через `payment`.

Спецификация в [`docs/spec/`](docs/spec/), контракт REST в [`docs/order.openapi.yaml`](docs/order.openapi.yaml).

## Как устроен сервис

```
cmd/order/main.go                       точка входа: конфигурация, миграции, сервер, остановка
internal/
  apperr/                               ошибки с видом и кодом, общие для ядра и адаптеров
  core/
    security/                           Principal из токена, роли
    order/
      aggregate/                        Order и Item: поля закрыты, переходы статусов в методах, события
      port/out/                         интерфейсы: репозиторий, шлюз каталога, часы, идентификаторы, единица работы
      usecase/                          CreateOrder, переходы статусов (LifecycleHandler), relay outbox, просрочка оплаты
      query/                            чтение заказа с проверкой владения
  adapter/
    in/http/                            chi, Problem Details, роли в middleware, DTO
    in/kafka/                           потребитель PaymentCompleted: processed_events и перевод в PAID одной транзакцией
    out/catalog/                        HTTP-клиент каталога: таймауты, повтор, размыкатель gobreaker
    out/persistence/                    pgx, миграции goose, транзакция в контексте, ключи идемпотентности, outbox
    out/kafka/                          издатель событий на kafka-go: ключ, заголовки, acks=all
    out/payment/                        HTTP-клиент возврата в сервис платежей с Idempotency-Key
    out/system/                         системные часы и uuid
  bootstrap/                            composition root, настройки клиента каталога, тесты
```

Ядро не знает ни про chi, ни про pgx, ни про gobreaker: это стережёт `internal/bootstrap/architecture_test.go`.
Каталог для ядра - интерфейс `CatalogGateway`, который отдаёт цены или ошибку с кодом; как именно клиент
добывает цены и когда сдаётся, ядро не видит.

## Запуск

```bash
docker compose -f ../../infra/compose.yaml up -d
go run ../catalog/cmd/catalog &
go run ./cmd/order
```

Переменные: `HTTP_ADDR` (`:8084`), `DATABASE_URL` (`postgres://catalog:catalog@localhost:5440/orders`),
`CATALOG_BASE_URL` (`http://localhost:8083`), `PAYMENT_BASE_URL` (`http://localhost:8086`), `KAFKA_BROKERS` (`localhost:9094`,
пусто - события только в лог), `KAFKA_GROUP` (`order`), `OUTBOX_INTERVAL` (`1s`), `EXPIRE_UNPAID_AFTER` (`15m`),
`EXPIRE_INTERVAL` (`1m`), `AUTH_MODE` (`local` или `jwt`), для `jwt` ещё `JWKS_URL`,
`JWT_ISSUER`, `JWT_AUDIENCE`.

В режиме `local` токен это строка `role.uuid`, роли `customer`, `seller`, `admin`:

```bash
CUSTOMER=$(uuidgen | tr A-Z a-z)
curl -s -X POST localhost:8084/api/v1/orders -H "Authorization: Bearer customer.$CUSTOMER" \
  -H "Idempotency-Key: $(uuidgen)" -H 'Content-Type: application/json' \
  -d '{"items":[{"productId":"<id опубликованного товара>","sellerId":"<id продавца>","quantity":2}],
       "shippingAddress":{"country":"RU","city":"Москва","street":"Тверская, 1","postalCode":"125009"}}'
```

Товар должен быть опубликован в каталоге: черновики каталог отдаёт только владельцу, а заказ ходит без токена.

## Что происходит, когда каталог не отвечает

Клиент каталога (`internal/adapter/out/catalog/client.go`) и его настройки (`CatalogSettings` в
`internal/bootstrap/wire.go`):

| что | значение | зачем |
|---|---|---|
| таймаут соединения | 500 мс | не висеть на `connect`, если сосед не слушает |
| таймаут запроса | 1 с | медленный сосед не должен держать обработчик заказа |
| попытки | 2, пауза 50 мс | одна сорванная попытка не роняет оформление |
| размыкатель | половина отказов из последних десяти вызовов, открыт 60 с | лежащему соседу не копить очередь запросов |

Повторяются только сетевые ошибки и ответы 5xx. Ответ `404` каталога это не отказ, а ответ: он без повтора
становится `PRODUCT_NOT_FOUND`. Исчерпанные попытки и открытый размыкатель уходят наружу как
`503 SERVICE_DEGRADED`, заказ при этом не создаётся. Худшее время ответа при этих числах: две попытки по
секунде и пауза, около 2,05 с.

## Один запрос - один заказ

Заголовок `Idempotency-Key` обязателен. Сценарий сначала ищет ключ: тот же ключ с тем же хешем тела отдаёт
прежний заказ ответом 200, тот же ключ с другим телом - `409 IDEMPOTENCY_KEY_CONFLICT`. Если ключа нет, заказ и
ключ пишутся в одной транзакции, ключ занимается вставкой с `ON CONFLICT DO NOTHING`: при гонке второй `INSERT`
дожидается первой транзакции, получает ноль строк, откатывает свой заказ и читает чужой. Тест
`TestIdempotency_sameKeyAtOnce_createsOneOrder` шлёт восемь одинаковых запросов разом и ждёт один заказ.
Хеш тела считается в HTTP-адаптере по каноническому JSON разобранного запроса, в ядро доезжает уже строкой.

## Событие уезжает через outbox

Агрегат при создании регистрирует `OrderCreated`; обработчик сценария кладёт события в таблицу `outbox` в той же
транзакции, что заказ и ключ идемпотентности. Фоновая горутина relay (`usecase.OutboxRelay.Run`) раз в
`OUTBOX_INTERVAL` берёт пачку строк с `published_at IS NULL` под `FOR UPDATE SKIP LOCKED`, публикует каждую через
порт `ExternalEventPublisher` и помечает отправленной в той же транзакции; упал брокер - транзакция откатилась,
строки остались, следующий круг повторит. Остановка сервиса ждёт конца текущей пачки, а не рвёт её.

Payload строки это внешний контракт, а не дамп внутреннего типа: `persistence.payloadOf` собирает
`ordersv1.OrderCreatedPayload` из пакета [`contracts/orders/v1`](../../contracts/orders/v1/events.go) - `customerId`
строкой, сумма десятичной строкой, ничего лишнего. Издатель `adapter/out/kafka` пишет в топик `marketplace.orders.v1`
с ключом `aggregateId` и заголовками `event-id`, `event-type`, `event-version`, `aggregate-type`, `aggregate-id`,
`occurred-at`; по `event-id` потребитель отбрасывает повторную доставку.

## Статусы и сага отмены

Переходы живут в агрегате и перечисляют разрешённое: `Confirm` DRAFT -> PENDING_PAYMENT (не меньше 100 рублей,
иначе `ORDER_BELOW_MINIMUM`), `MarkPaid` -> PAID, `MarkShipped` -> SHIPPED, `ConfirmDelivery` -> DELIVERED, `Cancel`
из DRAFT и PENDING_PAYMENT, `CancelAfterPayment` из PAID, `Expire` из PENDING_PAYMENT. Всё остальное отвечает
`409 ORDER_INVALID_STATE` и данные не трогает. Каждый переход - одна транзакция: строка под `FOR UPDATE`, метод
агрегата, `UPDATE`, события в outbox.

Ручки: `POST /api/v1/orders/{id}/confirm`, `/cancel` (покупатель или администратор), `/ship` (продавец заказа),
`/deliver` (покупатель), `/pay` (только администратор, для стенда; в бою оплату приносит событие `PaymentCompleted`
из топика `marketplace.payments.v1`, потребитель отбрасывает повтор по `event-id` через `processed_events`).

Отмена оплаченного заказа - сага с компенсацией: обработчик внутри транзакции зовёт сервис платежей
`POST /api/v1/payments/{paymentId}/refund` с `Idempotency-Key: refund-<orderId>`, и только после успешного возврата
переводит заказ в CANCELLED с `refundId` в событии `OrderCancelled`. Платежи легли - `503 SERVICE_DEGRADED`,
транзакция откатилась, заказ остался PAID, повтор отмены безопасен: платежи повторный возврат не считают вторым.
Неоплаченный заказ закрывает фоновая горутина `ExpireUnpaid`: раз в `EXPIRE_INTERVAL` переводит в EXPIRED те,
что висят в PENDING_PAYMENT дольше `EXPIRE_UNPAID_AFTER`.

## Тесты

```bash
go test ./...
```

Интеграционные тесты идут на настоящей PostgreSQL (`orders_test` из compose); `TestKafkaPublisher_*` ждёт Kafka со стенда
и пропускается, если брокер не поднят. Каталог в тестах подменяется
`httptest.Server`, который умеет держать ответ, рвать соединение и отвечать 404: четыре проверки
`TestCatalog_*` закрывают повтор, лежащий каталог, таймаут и размыкатель; `TestLifecycle_*` проходят заказ от черновика
до получения, отмену с возвратом через подменный сервис платежей и просрочку оплаты на подкручиваемых часах.

## Коды ошибок

`VALIDATION_ERROR`, `MALFORMED_REQUEST`, `EMPTY_ORDER`, `MULTI_SELLER_NOT_SUPPORTED` (400), `TOKEN_MISSING`,
`TOKEN_INVALID` (401), `ACCESS_DENIED` (403), `PRODUCT_NOT_FOUND`, `ORDER_NOT_FOUND` (404),
`IDEMPOTENCY_KEY_CONFLICT`, `ORDER_INVALID_STATE`, `REFUND_REJECTED`, `PAYMENT_NOT_FOUND` (409), `ORDER_BELOW_MINIMUM` (400),
`SERVICE_DEGRADED` (503). Тело ошибки в формате Problem Details, `type` вида `urn:problem:order:<CODE>`.

## Что почитать

- [Order Service в кейсе](https://vikulin-va.ru/use-case-pattern/case/order-service/) и [Use Case Pattern](https://vikulin-va.ru/use-case-pattern/).
- [Паттерны отказоустойчивости на Go](https://vikulin-va.ru/patterns/go/resilience/): повтор, таймаут, размыкатель.
- [Монолит и микросервисы](https://vikulin-va.ru/architecture-choice/monolith-vs-microservices/): цена сетевого вызова к соседу.
- [Гексагональная архитектура на Go](https://vikulin-va.ru/patterns/hexagonal/go/core-layer/): почему каталог для ядра - интерфейс.
- [HTTP-заголовки и Idempotency-Key на Go](https://vikulin-va.ru/rest-api/go/headers/): ключ занимается до операции.
- [Распределённые паттерны на Go](https://vikulin-va.ru/patterns/go/distributed-patterns/): outbox и идемпотентный потребитель.
- [Фоновые горутины и outbox-relay при остановке](https://vikulin-va.ru/graceful-shutdown/go/scheduled-async-outbox/).

# notification

Notification Service из сквозного маркетплейс-кейса сайта [vikulin-va.ru](https://vikulin-va.ru/use-case-pattern/case/notification-service/):
читает события заказа из Kafka и заводит уведомления адресату. В практикуме на Go появляется на десятом шаге как
потребитель outbox сервиса заказов; доставка писем и push тут не реализована, уведомление остаётся в статусе `PENDING`.

## Как устроен сервис

```
cmd/notification/main.go      точка входа: миграции, консьюмер в горутине, HTTP, остановка
internal/
  config/                     переменные окружения
  migrations/                 goose: processed_events и notifications
  inbox/                      идемпотентная обработка: processed_events и уведомление в одной транзакции
  consumer/                   kafka-go Reader: заголовки event-id и event-type, commit после обработки
  httpapi/                    GET /api/v1/notifications?userId= для администратора, health
```

Контракт событий общий с продюсером: пакет [`contracts/orders/v1`](../../contracts/orders/v1/events.go). Адресат
берётся из `customerId`, у `DisputeOpened` - из `sellerId`. Payload не по контракту (например, `customerId`
вложенным объектом) это ошибка обработки: offset не сдвигается, в `processed_events` записи нет.

## Повторная доставка

Kafka доставляет как минимум один раз: перебалансировка группы, повтор relay после сбоя пометки. Поэтому перед
работой консьюмер вставляет `event-id` в `processed_events` с `ON CONFLICT DO NOTHING` в той же транзакции, что и
уведомление. Второй раз вставка даёт ноль строк, второе письмо не рождается.

## Запуск

```bash
docker compose -f ../../infra/compose.yaml up -d
go run ./cmd/notification
curl -s 'localhost:8085/api/v1/notifications?userId=<uuid покупателя>' -H 'Authorization: Bearer admin'
```

Переменные: `HTTP_ADDR` (`:8085`), `DATABASE_URL` (`postgres://catalog:catalog@localhost:5440/notifications`),
`KAFKA_BROKERS` (`localhost:9094`), `KAFKA_GROUP` (`notification`), `KAFKA_TOPIC` (`marketplace.orders.v1`),
`ADMIN_TOKEN` (`admin`).

## Тесты

```bash
go test ./...
```

Тесты `internal/inbox` идут на настоящей PostgreSQL (`notifications_test` из compose): повторная доставка даёт одно
уведомление, спор адресуется продавцу, payload не по контракту отклоняется без пометки.

## Что почитать

- [Kafka на Go в production](https://vikulin-va.ru/kafka/go/production-essentials/): Reader, заголовки, commit offset.
- [Распределённые паттерны на Go](https://vikulin-va.ru/patterns/go/distributed-patterns/): идемпотентный потребитель.

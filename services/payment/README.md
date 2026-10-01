# payment

Payment Service из сквозного маркетплейс-кейса сайта [vikulin-va.ru](https://vikulin-va.ru/use-case-pattern/case/):
авторизация, списание и возврат платежа по заказу. В практикуме на Go появляется на одиннадцатом шаге как сервис,
в который ходит сага отмены заказа; это задание ученика.

Нарочно самый простой сервис из всех: один пакет, `database/sql` с SQL и маппингом руками, схема из
`schema.sql` при старте вместо миграций. Сравни с соседями: `catalog-starter` и `catalog` на pgx, `order` на pgx
с портами и goose. Что каждый из них скрывает и что даёт?

## Автомат статусов

У платежа четыре статуса: `AUTHORIZED`, `CAPTURED`, `REFUNDED`, `FAILED`. `Status.CanMoveTo` перечисляет
**разрешённое**: `AUTHORIZED -> CAPTURED | REFUNDED | FAILED`, `CAPTURED -> REFUNDED`; конечные статусы никуда не
ведут, переход в себя же не переход. Всё остальное `Payment.MoveTo` отвергает ошибкой `InvalidTransitionError`,
наружу это `409 INVALID_PAYMENT_TRANSITION`.

Повторы саги обрабатываются в сервисе, а не в автомате: повторная авторизация того же заказа возвращает уже
созданный платёж (`order_id` уникален), повторный возврат отдаёт тот же ответ, а деньги возвращаются один раз.

## Ручки

```
POST /api/v1/payments               {"orderId","amount","currency"} -> 201, AUTHORIZED
GET  /api/v1/payments/{id}
POST /api/v1/payments/{id}/capture  -> CAPTURED
POST /api/v1/payments/{id}/refund   -> REFUNDED, повтор безопасен
```

## Запуск и тесты

```bash
docker compose -f ../../infra/compose.yaml up -d
go run ./cmd/payment
go test ./...
```

Переменные: `HTTP_ADDR` (`:8086`), `DATABASE_URL` (`postgres://catalog:catalog@localhost:5440/payments`).
Тесты автомата идут без базы, тесты API - на настоящей PostgreSQL (`payments_test` из compose).

## Что почитать

- [Распределённые паттерны на Go](https://vikulin-va.ru/patterns/go/distributed-patterns/): сага и компенсации.
- [Паттерны отказоустойчивости на Go](https://vikulin-va.ru/patterns/go/resilience/): как сосед переживает отказ платежей.

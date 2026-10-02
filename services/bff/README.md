# BFF: экран собирается одним запросом

Backend for frontend это тонкий слой на границе. Клиент просит **экран**, а не
три ресурса из трёх сервисов: мобильному приложению три круговые задержки
дороже, чем одна.

Здесь же живёт то, что положено границе: лимит частоты и внятный ответ, когда
сосед не отвечает.

## Запустить

```bash
docker compose -f ../../infra/compose.yaml up -d redis
go run ./cmd/bff
```

Порт 8090. Переменные: `REDIS_ADDR` (`localhost:6381`), `ORDER_URL` (`http://localhost:8084`),
`CATALOG_URL` (`http://localhost:8083`), `PAYMENT_URL` (`http://localhost:8086`),
`RATE_LIMIT_PER_MINUTE` (`60`).

```bash
curl -s -H 'X-Client-Id: demo' -H "Authorization: Bearer customer.$CUSTOMER" \
  localhost:8090/api/v1/screens/order/<orderId>
```

Токен клиента BFF пересылает соседям как есть: заказ отдаёт только своему покупателю,
и решает это сервис заказов, а не граница.

## Что внутри

| файл | зачем |
|---|---|
| `internal/screen/screen.go` | сборка экрана: заказ, затем карточки товаров и статус платежа параллельно |
| `internal/screen/client.go` | походы к соседям с таймаутом и ошибкой `DownstreamError` |
| `internal/ratelimit/limiter.go` | счётчик запросов в Redis, общий для всех экземпляров |
| `internal/ratelimit/middleware.go` | 429 и `Retry-After` при превышении, `X-RateLimit-Remaining` всегда |
| `internal/httpapi/router.go` | одна ручка на весь экран; недоступный сосед даёт 502, а не пятисотку без объяснений |

Карточки товаров и статус платежа запрашиваются через `errgroup`: экран ждёт самый
медленный ответ, а не сумму всех. Платежа может не быть вовсе: заказ ещё не
оплачивали, и это обычное состояние экрана (`paymentStatus: NONE`), а не ошибка.

## Тесты

```bash
docker compose -f ../../infra/compose.yaml up -d redis
go test ./services/bff/...
```

Соседи подменены `httptest.Server`, Redis нужен настоящий: счётчик и должен быть
общим, а не в памяти процесса. Если Redis недоступен в работе, лимит пропускает
запрос и пишет предупреждение: граница без счётчика лучше границы, которая
не отвечает никому.

## Что почитать

- [Структурные паттерны микросервисов на Go](https://vikulin-va.ru/patterns/go/microservices-structural/): API Gateway, BFF и почему не «универсальный» ресурс.
- [Стили API](https://vikulin-va.ru/api-styles/): откуда берётся проблема трёх запросов.
- [Redis](https://vikulin-va.ru/redis/): счётчики с протуханием.

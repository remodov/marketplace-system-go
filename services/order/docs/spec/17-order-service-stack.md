---
type: context-section
context: order-service
parent: "[[order-service]]"
section: stack
tier: C
ucp-level: 3
tags:
  - stack
  - tech/go
  - tech/chi
  - tech/pgx
  - tech/postgres
  - tech/kafka
  - tech/redis
  - tech/ddd
  - bc/order
---

## 17. Стек технологий

### Платформа

- **Go 1.26** - стандартная библиотека, `log/slog`, `context`, дженерики.
- **chi v5** - маршрутизация и middleware HTTP.

### Use Case Pattern

- Команда - структура с полями, обработчик - тип с методом `Handle(ctx, cmd)`; порты - интерфейсы в `core/order/port/out`.
- Запросы отделены от команд пакетом `core/order/query`.

### DDD

- Агрегат `Order` с закрытыми полями и правилами в методах, позиции `Item` внутри агрегата, `Money` и `Address` значениями.

### Хранилище

- **PostgreSQL 16+** - основное хранилище (write-side, Outbox, идемпотентные ключи).
- **pgx v5** - драйвер и пул, SQL руками в адаптере.
- **goose** - миграции, встроенные в бинарник через `embed`.

### События

- **Apache Kafka 3.x** - транспорт между сервисами.
- **kafka-go** - продюсер и консьюмер.
- **Outbox-relay** - своя горутина с `SELECT ... FOR UPDATE SKIP LOCKED`.

### Устойчивость

- **gobreaker v2** - размыкатель на вызовах Catalog и Payment.
- Повтор с паузой и таймауты - на `context` и `net/http`, без сторонних библиотек.

### Безопасность

- **golang-jwt v5** + **keyfunc** - проверка JWT по JWKS Keycloak; роли из `realm_access.roles`.
- Локальный режим `AUTH_MODE=local` - токен вида `role.uuid` для тестов и стенда.

### Наблюдаемость

- **log/slog** - структурные логи JSON.
- **OpenTelemetry** - метрики и трассировка (добавляются на шаге про наблюдаемость).

### Тесты

- **testing** и **net/http/httptest** - интеграционные тесты на настоящей PostgreSQL, соседние сервисы подменяются `httptest.Server`.
- **go/packages** - архитектурные тесты на направление импортов.

### Инфраструктура

- **Docker Compose** - стенд в `infra/compose.yaml`.
- **Kubernetes** - деплой на шаге про инфраструктуру.

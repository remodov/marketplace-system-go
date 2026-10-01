# contracts

Внешний контракт событий заказа: один на продюсера (`services/order`) и потребителей
(`services/notification`, позже `services/payment`).

- [`asyncapi/marketplace-orders-v1.yaml`](asyncapi/marketplace-orders-v1.yaml) - канал, заголовки, сообщения.
- [`schemas/order-events.yaml`](schemas/order-events.yaml) - поля событий, один источник правды.
- [`orders/v1`](orders/v1/events.go) - Go-пакет `ordersv1` с теми же полями: продюсер пишет payload из него,
  потребитель читает в него, компилируются оба против одних типов.

Контракт намеренно плоский: во внешнее событие не протекают внутренние типы сервиса. `customerId` - строка
с UUID, а не вложенный объект; сумма - десятичная строка, а не число с плавающей точкой.

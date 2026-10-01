# Шаг 7. Тот же каталог, но по-взрослому

Рядом с учебным `catalog-starter` лежит `services/catalog` - тот же сервис, собранный так,
как его собирают в большой системе: ядро без импортов chi и pgx, порты интерфейсами,
спецификация в `docs/spec/`, роли и владение, журнал действий администратора.

## Часть первая: сравнить

Откройте оба сервиса и ответьте письменно (хватит десяти строк в `docs/COMPARISON.md`):

1. Что в `services/catalog` стало возможным, чего в `catalog-starter` не было?
   Подсказка: тест ядра без базы, второй вход (Kafka), смена хранилища, проверка границ.
2. Чего это стоило: сколько файлов нужно открыть, чтобы добавить поле в карточку, здесь и там.
3. При каком размере команды и сервиса вы бы остались на простой раскладке.

## Часть вторая: перенести смену цены

Смена цены из шага 3 в этом сервисе вынута. Шесть тестов `TestChangePrice_*` в
`internal/bootstrap/change_price_test.go` красные. Верните команду по слоям:

- `internal/core/product/usecase/change_product_price.go` - команда `ChangeProductPrice`
  и обработчик `ChangeProductPriceHandler`; смотрите на `change_status.go` как на образец:
  единица работы, строка под `FOR UPDATE`, владение через `requireOwnership`, журнал для администратора.
- `internal/core/product/aggregate/product.go` - метод `ChangePrice`: правило BR-P01, цена больше нуля,
  округление до копеек, `updatedAt`.
- `internal/adapter/in/http/product_handler.go` - маршрут `PATCH /api/v1/products/{productId}/price`
  и обработчик: разбор тела, проверка контракта (ноль - `VALIDATION_ERROR` до вызова ядра), ответ DTO.
- `internal/bootstrap/wire.go` - собрать обработчик сценария и передать его в `NewProductHandler`.

Места отмечены `TODO шаг 7`.

## Проверка

```bash
go test ./services/catalog/...
```

Зелёными должны стать шесть `TestChangePrice_*` и остаться зелёными архитектурные тесты:
если смена цены потянула в ядро `pgx` или `chi`, `TestCoreDependsOnlyOnCoreAndStdlib` упадёт первым.

## Куда смотреть

- [Use Case Pattern](https://vikulin-va.ru/use-case-pattern/) - почему команда и обработчик, а не сервис с методами.
- [Core слой на Go](https://vikulin-va.ru/patterns/hexagonal/go/core-layer/) и [порты](https://vikulin-va.ru/patterns/hexagonal/go/ports/).
- [ABAC и владение ресурсом в Go](https://vikulin-va.ru/patterns/auth-patterns/go/abac-resource-ownership/) - почему чужой товар это 404.
- [Журнал действий администратора в Go](https://vikulin-va.ru/patterns/auth-patterns/go/audit-admin/).

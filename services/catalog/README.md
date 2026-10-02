# catalog

Catalog Service из сквозного маркетплейс-кейса сайта [vikulin-va.ru](https://vikulin-va.ru/case/catalog-service/),
взрослая версия учебного `catalog-starter`: те же карточки товаров, но с границами слоёв, спецификацией,
ролями, владением и журналом действий администратора.

**Уровень 2** методологии Use Case Pattern: команда и обработчик сценария с явными портами, без агрегатов
с событиями и саг. Простой автомат статусов `DRAFT -> PUBLISHED <-> HIDDEN`, владение проверяется в
обработчике сценария, хранение через pgx с SQL в адаптере.

Спецификация в [`docs/spec/`](docs/spec/), контракт REST в [`docs/catalog.openapi.yaml`](docs/catalog.openapi.yaml).

## Как устроен сервис

```
cmd/catalog/main.go                     точка входа: конфигурация, миграции, сервер, остановка
internal/
  apperr/                               ошибки с видом и кодом, общие для ядра и адаптеров
  core/
    security/                           Principal из токена, роли
    product/
      aggregate/                        Product: поля закрыты, правила в методах
      port/out/                         интерфейсы: репозиторий, журнал, часы, идентификаторы, единица работы
      usecase/                          команды: создать, сменить цену, опубликовать, скрыть
      query/                            чтение: карточка, мои товары
  adapter/
    in/http/                            chi, Problem Details, роли в middleware, DTO
    out/persistence/                    pgx, миграции goose, транзакция в контексте, журнал
    out/system/                         системные часы и uuid
  bootstrap/                            composition root и архитектурные тесты
```

Правило одно: `core/` импортирует только стандартную библиотеку и свои пакеты. Его стережёт
`internal/bootstrap/architecture_test.go` на `go/packages`, а `var _ out.ProductRepository = (*PgProductRepository)(nil)`
в адаптере ловит расхождение порта и реализации на этапе компиляции.

## Запуск

```bash
docker compose -f ../../infra/compose.yaml up -d
go run ./cmd/catalog
```

Переменные: `HTTP_ADDR` (`:8083`), `DATABASE_URL` (`postgres://catalog:catalog@localhost:5440/catalog`),
`AUTH_MODE` (`local` или `jwt`), для `jwt` ещё `JWKS_URL`, `JWT_ISSUER`, `JWT_AUDIENCE`.
Хранилище картинок: `IMAGES_ENDPOINT` (`http://localhost:9000`), `IMAGES_BUCKET` (`marketplace-images`),
`IMAGES_ACCESS_KEY` и `IMAGES_SECRET_KEY` (`marketplace`), `IMAGES_REGION` (`us-east-1`), `IMAGES_UPLOAD_TTL` (`10m`).

В режиме `local` токен это строка `role.uuid`, роли `seller`, `admin`, `customer`:

```bash
SELLER=$(uuidgen | tr A-Z a-z)
curl -s -X POST localhost:8083/api/v1/products -H "Authorization: Bearer seller.$SELLER" \
  -H 'Content-Type: application/json' -d '{"title":"Кофемолка","price":2490.5,"currency":"RUB"}'
```

Карточку в статусе `DRAFT` видят только владелец и администратор; опубликованную видят все без токена.

Фото грузится мимо сервиса: владелец просит временную ссылку, а файл кладёт браузер.

```bash
curl -s -X POST localhost:8083/api/v1/products/$PRODUCT/image-upload-url -H "Authorization: Bearer seller.$SELLER" \
  -H 'Content-Type: application/json' -d '{"contentType":"image/jpeg"}'
curl -X PUT "$URL_ИЗ_ОТВЕТА" -H 'Content-Type: image/jpeg' --data-binary @photo.jpg
```

## Тесты

```bash
go test ./...
```

Интеграционные тесты идут на настоящей PostgreSQL (`catalog_test` из compose): `TestMain` накатывает миграции,
каждый тест чистит таблицы. Архитектурные тесты проверяют направление импортов.

## Коды ошибок

`VALIDATION_ERROR`, `MALFORMED_REQUEST`, `INVALID_PRICE`, `INVALID_CURRENCY`, `PRODUCT_NOT_FOUND`,
`OWN_PRODUCT_REQUIRED` (чужой товар, 404, а не 403), `INVALID_STATE_TRANSITION` (409), `TOKEN_MISSING` (401),
`TOKEN_INVALID` (401), `ACCESS_DENIED` (403). Тело ошибки в формате Problem Details, `type` вида `urn:problem:catalog:<CODE>`.

## Что почитать

- [Catalog Service в кейсе](https://vikulin-va.ru/case/catalog-service/) и [Use Case Pattern](https://vikulin-va.ru/use-case-pattern/).
- [Гексагональная архитектура на Go](https://vikulin-va.ru/patterns/hexagonal/go/core-layer/): почему ядро не знает про chi и pgx.
- [Архитектурные тесты на Go](https://vikulin-va.ru/patterns/hexagonal/go/architecture-tests/).
- [ABAC и владение ресурсом в Go](https://vikulin-va.ru/patterns/auth-patterns/go/abac-resource-ownership/).
- Учебная версия того же сервиса для первых шагов практикума: `../catalog-starter`.

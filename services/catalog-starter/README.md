# Каталог: учебная версия на Go

Каталог маркетплейса, написанный так, как пишут обычный Go-сервис: обработчик ->
сервис -> репозиторий, `net/http` с роутером chi, pgx и SQL руками, миграции goose.
С этого начинается практикум.

## Запустить

```bash
docker compose -f ../../infra/compose.yaml up -d postgres-catalog-starter
go run ./cmd/catalog-starter
```

Сервис поднимется на 8082, схему накатят миграции при старте.

```bash
curl -s localhost:8082/products
curl -s -X POST localhost:8082/products -H 'Content-Type: application/json' \
  -d '{"title":"Беспроводная мышь","price":1990.00,"stock":7}'
curl -s -X POST localhost:8082/products/<id>/reserve -H 'Content-Type: application/json' \
  -d '{"quantity":2}'
```

Настройки - переменные окружения: `HTTP_ADDR` (`:8082`), `DATABASE_URL`
(база из compose на 5440).

## Прогнать тесты

```bash
go test ./...
```

Тесты идут на настоящем PostgreSQL - на второй базе того же контейнера
(`catalog_starter_test`, `TEST_DATABASE_URL`). Так они проверяют и SQL,
и блокировки, которых в памяти не увидеть.

## Что внутри

| файл | зачем |
|---|---|
| `internal/product/product.go` | тип `Product` и единственное бизнес-правило: нельзя зарезервировать больше, чем есть |
| `internal/product/repository.go` | SQL руками через pgx: выборки, вставка, обновление с проверкой версии |
| `internal/product/service.go` | сценарии: найти, создать, зарезервировать |
| `internal/product/handler.go` | REST на chi: `GET /products`, `GET /products/{id}`, `POST /products`, `POST /products/{id}/reserve` |
| `internal/httpx/problem.go` | тело ошибки в формате Problem Details, коды 400, 404 и 409 |
| `internal/migrations` | миграции goose, вшиты в бинарник |

Правило, вокруг которого всё крутится, живёт в типе, а не в сервисе:

```go
func (p *Product) Reserve(quantity int) error {
	if quantity > p.stock {
		return &OutOfStockError{ID: p.id, Requested: quantity, Available: p.stock}
	}
	p.stock -= quantity
	return nil
}
```

Поля `Product` не экспортированы: менять остаток снаружи нечем, правило нельзя обойти.
Это первый шаг к тому, что дальше в программе называется доменной моделью.

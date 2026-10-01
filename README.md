# Маркетплейс на Go: сквозная система для практики

Та же система, что в [практикуме на Java](https://github.com/remodov/marketplace-system):
маркетплейс из разбора [«Как разбить систему на сервисы»](https://vikulin-va.ru/case/services-map/),
только написанный на Go. Репозиторий - практическая часть программы
[«Backend · Go»](https://vikulin-va.ru/programs/backend-go/) с
[vikulin-va.ru](https://vikulin-va.ru/): каждый шаг практикума привязан к статьям,
которые закрывают его тему.

## Что внутри

| сервис | отвечает за | стек |
|---|---|---|
| `services/catalog-starter` | карточки товаров, остатки, резерв, поиск | `net/http` + chi, pgx, миграции goose, Redis |
| `services/catalog` | те же карточки по-взрослому: слои, спецификация, роли, владение, журнал администратора | chi, pgx, goose, golang-jwt, архитектурные тесты |
| `services/order` | заказы: черновик с ценами из каталога, таймауты, повтор и размыкатель на соседе, идемпотентность, outbox | chi, pgx, goose, gobreaker, kafka-go |
| `services/notification` | уведомления: потребитель событий заказа с защитой от повторной доставки | kafka-go, pgx, goose |
| `contracts` | внешний контракт событий заказа: AsyncAPI, схемы и Go-пакет для продюсера и потребителей | AsyncAPI 3 |

Первая часть практикума - учебный каталог и шесть шагов на нём, вторая начинается
с каталога по-взрослому ([план](docs/practicum/PLAN.md)), дальше идут сервис заказов и уведомления.
Платежи появятся следом, по образцу Java-версии.

## С чего начинать

[`services/catalog-starter`](services/catalog-starter/README.md): обработчик -> сервис ->
репозиторий, одна таблица, SQL руками. Клонировал, поднял базу, запустил, увидел товар.

## Поднять стенд

```bash
docker compose -f infra/compose.yaml up -d
docker compose -f infra/compose.yaml ps
```

| что | порт | зачем |
|---|---|---|
| PostgreSQL | 5440 | базы `catalog_starter`, `catalog`, `orders` и `notifications` плюс тестовые `*_test` |
| Redis | 6381 | кэш карточек, шаг 6 |
| Kafka | 9094 | события заказа из outbox, шаг 10 |

## Как устроен шаг

Ветка `step-NN-<тема>` - задание: каркас на месте, реализация вынута, тест красный,
условие в `TASK.md` внутри сервиса. Ветка `step-NN-<тема>-solution` - эталон.
`main` - накопленный эталон всех шагов.

```bash
git switch step-02-read-endpoint
go test ./services/catalog-starter/...
```

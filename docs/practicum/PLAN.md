# Практикум на Go: маркетплейс по шагам

Сквозная практика к программе «Backend · Go». Вход - знаешь Go, писал обработчики,
про архитектуру пока читал. Система та же, что в Java-версии практикума:
маркетплейс из разбора [«Как разбить систему на сервисы»](https://vikulin-va.ru/case/services-map/).

Ученик не пишет систему с нуля: каркас, конфигурация и тесты даются. Он реализует
то, ради чего шаг придуман, и проверяет себя зелёным тестом, а не кнопкой
«показать решение».

## Как устроен шаг

Ветка `step-NN-<тема>` - задание: каркас на месте, реализация вынута, тест красный.
`TASK.md` в корне сервиса - условие: что сделать, где стоят `TODO`, чем проверяется,
куда смотреть по дороге. Ветка `step-NN-<тема>-solution` - эталон.

# Часть первая: обычный сервис

## Шаг 1. Запустить каталог и разобрать по частям

**Материал:** [/go/project-structure-and-wiring/](https://vikulin-va.ru/go/project-structure-and-wiring/) · [/go/routing-with-chi/](https://vikulin-va.ru/go/routing-with-chi/) · [/go/handlers-and-json/](https://vikulin-va.ru/go/handlers-and-json/)

**Даётся:** рабочий `catalog-starter`, база в compose, пять зелёных тестов.

**Ученик:** поднимает базу, запускает сервис, дёргает четыре ручки; отвечает, что
делает каждый слой и почему тесты идут на настоящей базе.

**Проверка:** `go test ./...` зелёный, `curl` возвращает созданный товар.

## Шаг 2. Новая ручка на чтение

**Материал:** [/rest-api/go/query-params/](https://vikulin-va.ru/rest-api/go/query-params/) · [/go/persistence-sqlc/](https://vikulin-va.ru/go/persistence-sqlc/) · [/go/handlers-and-json/](https://vikulin-va.ru/go/handlers-and-json/)

**Даётся:** красный тест на `GET /products?maxPrice=…`.

**Ученик:** метод репозитория с SQL, метод сервиса, параметр в обработчике.

**Проверка:** тест зеленеет; в логе видно, какой SQL ушёл в базу.

## Шаг 3. Команда, валидация и коды ошибок

**Материал:** [/go/validation/](https://vikulin-va.ru/go/validation/) · [/go/errors-and-http/](https://vikulin-va.ru/go/errors-and-http/) · [/rest-api/go/errors/](https://vikulin-va.ru/rest-api/go/errors/)

**Даётся:** тесты на 400, 404 и 409; разбор ошибок в обработчике как образец.

**Ученик:** изменение цены и остатка: проверки входа, доменные ошибки, тело ответа
в формате Problem Details, правильные коды.

**Проверка:** каждый сценарий отказа отвечает своим кодом, а не пятисоткой.

## Шаг 4. Правило внутри модели

**Материал:** [/domain-driven-design/01-what-is-ddd/](https://vikulin-va.ru/domain-driven-design/01-what-is-ddd/) · [/go-basics/structs-and-methods/](https://vikulin-va.ru/go-basics/structs-and-methods/) · [/go-basics/errors/](https://vikulin-va.ru/go-basics/errors/)

**Даётся:** тесты домена, которым не нужна база.

**Ученик:** переносит правила в тип - скидка не больше половины, цена округляется
до копеек; наружу торчат методы, а не поля.

**Проверка:** доменные тесты зелёные и работают за миллисекунды; правило нельзя
обойти из сервиса.

## Шаг 5. База: миграции, транзакции, одновременный резерв

**Материал:** [/postgres/acid-and-isolation/](https://vikulin-va.ru/postgres/acid-and-isolation/) · [/postgres/locks/](https://vikulin-va.ru/postgres/locks/) · [/go-basics/goroutines-and-channels/](https://vikulin-va.ru/go-basics/goroutines-and-channels/) · [/concurrency/race-conditions/](https://vikulin-va.ru/concurrency/race-conditions/)

**Даётся:** тест на сто одновременных покупателей и тест, который сверяет схему
после миграций с полями типа.

**Ученик:** отделяет резерв от остатка - новая колонка `reserved` миграцией,
`available = stock - reserved`; резерв удерживает товар, а не списывает его.
Дальше разбирается, почему при обычном чтении продаётся больше, чем есть, и берёт
строку под блокировку в транзакции.

**Проверка:** сто параллельных резервов на десять единиц продают ровно десять,
остаток на складе при этом не меняется.

## Шаг 6. Поиск: LIKE, индекс, кэш

**Материал:** [/postgres/indexes-types/](https://vikulin-va.ru/postgres/indexes-types/) · [/redis/caching-patterns/](https://vikulin-va.ru/redis/caching-patterns/) · [/algorithms/](https://vikulin-va.ru/algorithms/)

**Даётся:** генератор на сто тысяч товаров, скрипт замера и тест, который считает
обращения к базе.

**Ученик:** снимает время ответа на ста тысячах товаров, видит `Seq Scan` в плане,
заводит триграммный индекс миграцией, потом кладёт карточку товара в кэш и
сбрасывает запись при любом изменении.

**Проверка:** время ответа до и после - числом; повторный запрос отвечает из кэша,
правка товара кэш сбрасывает.

# Часть вторая: взрослая система

## Шаг 7. Тот же каталог, но по-взрослому

**Материал:** [/use-case-pattern/](https://vikulin-va.ru/use-case-pattern/) · [/patterns/hexagonal/go/core-layer/](https://vikulin-va.ru/patterns/hexagonal/go/core-layer/) · [/patterns/hexagonal/go/architecture-tests/](https://vikulin-va.ru/patterns/hexagonal/go/architecture-tests/) · [/case/catalog-service/](https://vikulin-va.ru/case/catalog-service/)

**Даётся:** `services/catalog` - ядро без единого импорта chi и pgx, порты интерфейсами,
спецификация, роли и владение, журнал действий администратора; архитектурный тест на
направление импортов и интеграционные тесты смены цены на настоящей PostgreSQL.

**Ученик:** сравнивает две версии одного сервиса и письменно отвечает, что дала
сложность и чего стоила; потом переносит смену цены из третьего шага сюда - команда,
обработчик сценария, метод порта, SQL в адаптере, обработчик chi.

**Проверка:** шесть проверок `TestChangePrice_*` зелёные - цена меняется и доезжает до
базы, ноль не проходит, чужой товар отдаёт 404, админское изменение оставляет запись в
журнале, без токена 401; архитектурный тест по-прежнему зелёный.

## Шаг 8. Заказ: сосед отвечает медленно, срывается и лежит

**Материал:** [/patterns/go/resilience/](https://vikulin-va.ru/patterns/go/resilience/) · [/architecture-choice/monolith-vs-microservices/](https://vikulin-va.ru/architecture-choice/monolith-vs-microservices/) · [/use-case-pattern/case/order-service/](https://vikulin-va.ru/use-case-pattern/case/order-service/)

**Даётся:** `services/order` - агрегат заказа, сценарий создания черновика, который ходит в
`catalog` за ценами, хранение на pgx и тесты, где каталог подменён `httptest.Server`: он умеет
держать ответ, рвать соединение и отвечать 404.

**Ученик:** в клиенте каталога ставит таймауты на соединение и на запрос, повтор с паузой только
для сетевых ошибок и 5xx, размыкатель на `gobreaker`; исчерпанные попытки и открытый размыкатель
превращает в доменное `SERVICE_DEGRADED`, а 404 каталога оставляет `PRODUCT_NOT_FOUND` без повтора.
Подбирает числа и считает худшее время ответа.

**Проверка:** четыре проверки `TestCatalog_*` зелёные - зависший первый ответ переживается
повтором и к каталогу ушло ровно два запроса, лежащий каталог даёт 503 и ноль заказов в базе,
медленный каталог отбивается таймаутом раньше, чем ответит, после серии отказов размыкатель
перестаёт ходить к каталогу.

## Шаг 9. Идемпотентность

**Материал:** [/rest-api/go/headers/](https://vikulin-va.ru/rest-api/go/headers/) · [/graceful-shutdown/go/idempotency-in-flight/](https://vikulin-va.ru/graceful-shutdown/go/idempotency-in-flight/)

**Даётся:** заголовок `Idempotency-Key` и хеш тела уже доезжают до сценария, таблица
`idempotency_keys` в миграциях, порт `IdempotencyKeys`, тесты, которые шлют один и тот же
запрос дважды и восемь раз разом.

**Ученик:** проверка ключа до работы, занятие ключа вставкой с `ON CONFLICT DO NOTHING` в одной
транзакции с заказом, ответ проигравшему гонку прежним заказом, конфликт хеша тела кодом
`IDEMPOTENCY_KEY_CONFLICT`.

**Проверка:** повтор не создаёт второй заказ и возвращает тот же ответ, другой текст под тем
же ключом даёт 409, восемь одновременных запросов дают один заказ.

## Шаг 10. События, outbox и контракт

**Материал:** [/kafka/go/fundamentals/](https://vikulin-va.ru/kafka/go/fundamentals/) · [/patterns/go/distributed-patterns/](https://vikulin-va.ru/patterns/go/distributed-patterns/) · [/graceful-shutdown/go/scheduled-async-outbox/](https://vikulin-va.ru/graceful-shutdown/go/scheduled-async-outbox/) · [/kafka/go/production-essentials/](https://vikulin-va.ru/kafka/go/production-essentials/)

**Даётся:** таблица `outbox`, агрегат, который регистрирует `OrderCreated`, издатель на kafka-go, Kafka в
стенде, контракт событий в `contracts/` (AsyncAPI плюс Go-пакет `ordersv1`) и сервис `notification`
с консьюмером и журналом `processed_events`.

**Ученик:** пишет событие в outbox в одной транзакции с заказом, собирает payload по внешнему
контракту, а не из внутреннего типа, и делает relay: пачка под `FOR UPDATE SKIP LOCKED`,
публикация, пометка отправленного в той же транзакции. Разбирается, почему `customerId`
вложенным объектом ломает потребителя.

**Проверка:** строка outbox рождается вместе с заказом и не рождается при откате; поля payload
ровно те, что в контракте; relay публикует и помечает, при лежащем брокере строка остаётся;
повторная доставка в `notification` не создаёт второе уведомление.

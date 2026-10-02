# Шаг 15. Доставка и наблюдаемость

## Что нужно сделать

Сервис написан и работает на ноутбуке. Между этим и «работает в проде» лежит
шаг, который обычно делают в последний день и потому плохо.

**1. Образ.** `services/catalog-starter/Dockerfile`: сборка отдельно от запуска,
чтобы в образ не уезжали go toolchain, исходники и кэш модулей; бинарник
статический (`CGO_ENABLED=0`), иначе `distroless/static` его не запустит;
контейнер не должен бежать от root. Контекст сборки это корень репозитория:
`docker build -f services/catalog-starter/Dockerfile .`

**2. Манифесты.** `deploy/k8s/catalog-starter.yaml`:

- пробы готовности и живости: без них кластер считает под готовым сразу и шлёт
  запросы в ещё не поднявшееся приложение;
- запросы и лимиты ресурсов: под без лимитов утягивает узел за собой;
- корректное завершение: под должен успеть уйти из балансировщика раньше, чем
  перестанет отвечать;
- образ с версией, а не `latest`: откатываться на «latest» некуда.

Эталон рядом: `deploy/k8s/bff.yaml`.

**3. Наблюдаемость.** `internal/observability/observability.go` стартового
каталога: `Mount` должен поднять пробы `/health/live` и `/health/ready` (готовность
проверяет базу) и отдать `/metrics` в формате Prometheus; гистограмма времени
ответа обязана нести метку `service`, иначе в общем Prometheus не отличить,
чьё это время; `Sampler` берёт долю трасс из настроек, а не роняет все.

## Где править

`// TODO шаг 15`:

- `services/catalog-starter/Dockerfile`;
- `deploy/k8s/catalog-starter.yaml`;
- `services/catalog-starter/internal/observability/observability.go`.

## Как проверить себя

```bash
python3 tools/check-deploy.py
go test ./services/catalog-starter/internal/observability/
```

Скрипт сейчас находит девять замечаний, четыре проверки
`observability_test.go` красные: обе пробы, метрики с меткой сервиса и
сэмплер.

## На что посмотреть по дороге

- Разница между пробой готовности и живости: первая отвечает «слать ли мне
  трафик», вторая «не пора ли меня перезапустить». Если перепутать, кластер
  начнёт перезапускать поды, которые просто ещё прогреваются.
- Метрика без метки сервиса бесполезна: в общем Prometheus не отличить, чьё это
  время ответа. А метка `route` обязана быть шаблоном маршрута, не сырым URL:
  иначе каждый идентификатор товара станет отдельным рядом.
- `distroless/static` не содержит ни оболочки, ни libc: `sh -c "sleep 5"` в
  `preStop` там не выполнится. Подумай, чем заменить, и зачем тогда `preStop`
  вообще нужен (ответ в статье про graceful shutdown).
- Посмотри пайплайн `.github/workflows/ci.yml`: он гоняет тесты сервисов, тесты
  клиента и эту же проверку выката. Заказы и уведомления в нём не гоняются:
  им нужна Kafka, и они живут на стенде.

## Материал

- Dockerfile для сервиса на Go: https://vikulin-va.ru/docker/go/dockerizing/
- Рантайм в контейнере: https://vikulin-va.ru/docker/go/runtime/
- Kubernetes: https://vikulin-va.ru/kubernetes/
- Пробы: https://vikulin-va.ru/observability/go/health-checks/
- Метрики: https://vikulin-va.ru/observability/go/metrics/
- CI/CD: https://vikulin-va.ru/cicd/

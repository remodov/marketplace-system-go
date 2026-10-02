# TODO шаг 15: образ сервиса.
# Сейчас всё в один слой и от root: в образ уедут go toolchain, исходники и кэш модулей,
# а контейнер побежит с лишними правами. Контекст сборки это корень репозитория.
FROM golang:1.26-alpine
WORKDIR /src
COPY . .
RUN go build -o /src/catalog-starter ./services/catalog-starter/cmd/catalog-starter
EXPOSE 8082
ENTRYPOINT ["/src/catalog-starter"]

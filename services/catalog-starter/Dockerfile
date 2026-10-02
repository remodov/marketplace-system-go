FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/catalog-starter ./services/catalog-starter/cmd/catalog-starter

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/catalog-starter /app/catalog-starter
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app/catalog-starter"]

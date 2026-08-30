# syntax=docker/dockerfile:1.7
# ==========================================================
# Образ сервиса задач RTM-Task.
#
# Сборка:
#   docker build -t rtm-task .
# ==========================================================
ARG GO_VERSION=1.26.1
ARG ALPINE_VERSION=3.22

# ==========================================================
# Stage 1: deps — слой зависимостей
#
# Отдельный слой: инвалидируется только при правке go.mod/go.sum,
# а не при любом изменении кода.
# ==========================================================
FROM golang:${GO_VERSION}-alpine AS deps
WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# ==========================================================
# Stage 2: builder — компиляция
# ==========================================================
FROM deps AS builder
ARG TARGETARCH

# Ограничение нагрузки на CPU при сборке: на общем раннере полная
# загрузка ядер мешает соседям. Пусто — без ограничений.
ARG BUILD_JOBS=2
ARG GOMAXPROCS=2

COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Кэш-маунты переживают пересборку образа:
#   /go/pkg/mod           — скачанные модули
#   /root/.cache/go-build — кэш компиляции
# -trimpath убирает пути сборочной машины из бинарника.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    env CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    ${GOMAXPROCS:+GOMAXPROCS=${GOMAXPROCS}} \
    go build -trimpath -ldflags='-w -s' \
    ${BUILD_JOBS:+-p ${BUILD_JOBS}} \
    -o /out/service \
    ./cmd/rtm-task

# ==========================================================
# Stage 3: runtime
# ==========================================================
FROM alpine:${ALPINE_VERSION} AS runtime

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup && \
    mkdir -p /app/config && \
    chown -R appuser:appgroup /app

WORKDIR /app

COPY --from=builder --chown=appuser:appgroup /out/service /app/service

# config.yaml намеренно НЕ копируется в образ: там пароль базы и ключ
# smtp-сервиса. Файл лежит на сервере и монтируется через volume
# (см. docker-compose.yml). Пустой /app/config — точка монтирования.

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/service"]

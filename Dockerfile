# syntax=docker/dockerfile:1
# Образ сервера: статический бинарник в distroless — ни оболочки, ни пакетов,
# только сервер. Поэтому healthcheck — команда самого сервера, а не wget.

# Версию Go меняем вместе с go.mod и .github/workflows/ci.yml.
FROM golang:1.27-alpine AS build
WORKDIR /src
# Сначала зависимости: слой кешируется, пока не меняются go.mod и go.sum.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /usr/local/bin/api
ENV APP_ENV=production
ARG GIT_SHA=dev
ENV GIT_SHA=$GIT_SHA
# nonroot (uid 65532) уже задан базовым образом: в контейнере нечего делать под root.
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD ["/usr/local/bin/api", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/api"]

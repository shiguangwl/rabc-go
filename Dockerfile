# syntax=docker/dockerfile:1

FROM node:22-alpine AS web-builder

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
ENV HUSKY=0

WORKDIR /src/web

RUN corepack enable && corepack prepare pnpm@10.20.0 --activate

COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm install --frozen-lockfile

COPY web ./
RUN pnpm build

FROM golang:1.25-alpine AS go-builder

WORKDIR /src

ENV CGO_ENABLED=0
ENV GOOS=linux

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . ./
COPY --from=web-builder /src/web/dist ./web/dist

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags embed_frontend -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

ENV TZ=Asia/Shanghai
ENV APP_CONF=/app/config/prod.yml

COPY --from=go-builder /out/server /app/server
COPY config/prod.yml /app/config/prod.yml

RUN mkdir -p /app/storage/logs \
    && chown -R app:app /app

USER app

EXPOSE 8000

CMD ["/app/server"]

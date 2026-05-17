# syntax=docker/dockerfile:1.7

FROM node:22-bookworm-slim AS web-builder
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN pnpm install --frozen-lockfile
COPY web ./web
RUN pnpm --dir web build

FROM golang:1.25-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN rm -rf ./internal/frontend/dist && mkdir -p ./internal/frontend/dist
COPY --from=web-builder /app/web/dist ./internal/frontend/dist
RUN CGO_ENABLED=0 go build -o /out/docker-proxy-hub ./cmd/server

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 --shell /usr/sbin/nologin appuser \
    && mkdir -p /data \
    && chown -R appuser:appuser /data /home/appuser
WORKDIR /home/appuser
COPY --from=go-builder /out/docker-proxy-hub /usr/local/bin/docker-proxy-hub
ENV DPH_ADDR=:8080 \
    DPH_DB_PATH=/data/docker-proxy-hub.db
EXPOSE 8080
VOLUME ["/data"]
USER appuser
ENTRYPOINT ["/usr/local/bin/docker-proxy-hub"]

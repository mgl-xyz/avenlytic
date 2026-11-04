# syntax=docker/dockerfile:1
FROM node:26.13.0-alpine AS frontend
WORKDIR /src/web-ui
COPY web-ui/package.json ./package.json
RUN npm install --no-audit --no-fund
COPY web-ui/ ./
RUN mkdir -p /src/cmd/server/web && npm run build

FROM golang:1.27.3-alpine AS backend
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod ./
RUN go mod download
COPY . .
COPY --from=frontend /src/cmd/server/web/ ./cmd/server/web/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/avenlytic-server ./cmd/server

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata curl && addgroup -S avenlytic && adduser -S -G avenlytic -u 10001 avenlytic
WORKDIR /app
COPY --from=backend /out/avenlytic-server /usr/local/bin/avenlytic-server
RUN mkdir -p /data && chown -R avenlytic:avenlytic /data
ENV LISTEN_ADDR=:9080 DATA_FILE=/data/avenlytic.db
VOLUME ["/data"]
EXPOSE 9080
USER avenlytic
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD curl -fsS http://127.0.0.1:9080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/avenlytic-server"]
CMD ["--listen", ":9080", "--data", "/data/avenlytic.db"]

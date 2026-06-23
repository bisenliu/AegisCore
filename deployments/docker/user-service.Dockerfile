# Build from the repository root:
#   docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .

FROM golang:1.26-alpine3.22 AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY common ./common
COPY user-service ./user-service

WORKDIR /src/user-service
RUN go build -o /out/user-services ./cmd

FROM arigaio/atlas:latest AS atlas

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache tzdata && addgroup -S aegiscore && adduser -S aegiscore -G aegiscore

COPY --from=builder /out/user-services /app/user-service/bin/user-services
COPY --from=atlas /atlas /usr/local/bin/atlas
COPY user-service/configs /app/user-service/configs
COPY user-service/migrations /app/user-service/migrations
COPY user-service/scripts /app/user-service/scripts

RUN chmod +x /app/user-service/scripts/*.sh /app/user-service/bin/user-services

USER aegiscore
ENTRYPOINT ["/app/user-service/scripts/entrypoint.sh"]
CMD ["/app/user-service/bin/user-services", "serve", "--config", "/app/user-service/configs/config.yaml"]

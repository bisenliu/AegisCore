# 从仓库根目录构建：
#   docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .

FROM golang:1.26-alpine3.22 AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY common ./common
COPY user-service ./user-service

WORKDIR /src/user-service
RUN go build -o /out/user-services ./cmd

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache tzdata \
  && addgroup -g 10001 -S aegiscore \
  && adduser -u 10001 -S aegiscore -G aegiscore

COPY --from=builder --chmod=0755 /out/user-services /app/user-service/bin/user-services
COPY user-service/configs /app/user-service/configs

USER aegiscore
ENTRYPOINT ["/app/user-service/bin/user-services"]
CMD ["serve", "--config", "/app/user-service/configs/config.yaml"]

# Build from the repository root:
#   docker build -f user-services/Dockerfile -t aegiscore-user-services .

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY common ./common
COPY user-services ./user-services

WORKDIR /src/user-services
RUN go build -o /out/user-services ./cmd

FROM arigaio/atlas:latest AS atlas

FROM alpine:3.20
WORKDIR /app

RUN addgroup -S aegiscore && adduser -S aegiscore -G aegiscore

COPY --from=builder /out/user-services /app/user-services/bin/user-services
COPY --from=atlas /atlas /usr/local/bin/atlas
COPY user-services/atlas.hcl /app/user-services/atlas.hcl
COPY user-services/configs /app/user-services/configs
COPY user-services/migrations /app/user-services/migrations
COPY user-services/scripts /app/user-services/scripts

RUN chmod +x /app/user-services/scripts/*.sh /app/user-services/bin/user-services

USER aegiscore
ENTRYPOINT ["/app/user-services/scripts/entrypoint.sh"]
CMD ["/app/user-services/bin/user-services", "serve", "--config", "/app/user-services/configs/config.yaml"]

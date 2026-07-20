# syntax=docker/dockerfile:1.25
# 从仓库根目录构建：
#   docker buildx build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-service .
#
# 基础镜像同时保留可读 tag 与多架构 manifest digest，便于 Renovate 审查 tag/digest 更新。

FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY common/go.mod common/go.sum ./common/
COPY tools/openapi-convert/go.mod ./tools/openapi-convert/
COPY user-service/go.mod user-service/go.sum ./user-service/

RUN --mount=type=cache,target=/go/pkg/mod \
  go list -m -mod=readonly all >/dev/null

COPY common ./common
COPY tools ./tools
COPY user-service ./user-service

WORKDIR /src/user-service
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build \
    -mod=readonly \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/user-service ./cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
WORKDIR /app

COPY --from=builder --chmod=0755 /out/user-service /app/user-service/bin/user-service
COPY user-service/configs /app/user-service/configs

USER 65532:65532
ENTRYPOINT ["/app/user-service/bin/user-service"]
CMD ["serve", "--config", "/app/user-service/configs/config.yaml"]

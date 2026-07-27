# syntax=docker/dockerfile:1.25

FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder
WORKDIR /src/tools/nacos-config-seed

COPY tools/nacos-config-seed/go.mod ./
COPY tools/nacos-config-seed/*.go ./

RUN --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build \
    -mod=readonly \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/nacos-config-seed .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
WORKDIR /app

COPY --from=builder --chmod=0755 /out/nacos-config-seed /app/nacos-config-seed

USER 65532:65532
ENTRYPOINT ["/app/nacos-config-seed"]

# syntax=docker/dockerfile:1.26

# Jaeger 1.76 all-in-one 不包含 IANA zoneinfo；复用仓库固定的 Distroless runtime 只补上海时区数据。
FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS timezone

FROM jaegertracing/all-in-one:latest

COPY --from=timezone /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

ENV TZ=Asia/Shanghai

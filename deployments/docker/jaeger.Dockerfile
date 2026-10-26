# syntax=docker/dockerfile:1.25

# Jaeger 1.76 all-in-one 不包含 IANA zoneinfo；复用仓库固定的 Distroless runtime 只补上海时区数据。
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS timezone

FROM jaegertracing/all-in-one:latest

COPY --from=timezone /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

ENV TZ=Asia/Shanghai

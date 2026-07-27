## ADDED Requirements

### Requirement: 本地 Compose 时区一致性

本地 Compose 的常驻服务与一次性任务 MUST 显式使用 `TZ=Asia/Shanghai`，不得依赖宿主机时区或镜像浮动默认值。缺少 IANA zoneinfo 的基础镜像 MUST 通过可审查的最小镜像层补齐所需时区数据，不得保留设置了 `TZ` 但进程仍以 UTC 运行的无效配置。

#### Scenario: 全部 Compose 服务声明时区

- **WHEN** 渲染 `deployments/compose/docker-compose.yml`
- **THEN** PostgreSQL、Redis、Nacos、Nacos 初始化任务、Jaeger、RBAC seed、user-service、Prometheus 和 Grafana MUST 都获得 `TZ=Asia/Shanghai`
- **AND** 配置 MUST NOT 挂载宿主机 `/etc/localtime` 或把宿主机时区作为正确性前提

#### Scenario: Jaeger 基础镜像缺少 zoneinfo

- **WHEN** 当前 Jaeger 基础镜像不包含 `/usr/share/zoneinfo/Asia/Shanghai`
- **THEN** 本地 Jaeger 镜像 MUST 从固定且可审查的构建阶段复制该 IANA zoneinfo，并在进程启动后解析为 UTC+8
- **AND** 薄镜像 MUST 保留基础 Jaeger 的 entrypoint、端口与功能，MUST NOT 为时区修复引入 Jaeger 版本迁移

#### Scenario: PostgreSQL 已有数据卷

- **WHEN** Compose 使用已经初始化且配置为 UTC 的 PostgreSQL data volume 重建容器
- **THEN** PostgreSQL session `timezone` 与 `log_timezone` MUST 在不删除 volume、不运行 migration 的情况下变为 `Asia/Shanghai`
- **AND** 官方 `docker-entrypoint.sh` 与健康检查 MUST 继续正常工作

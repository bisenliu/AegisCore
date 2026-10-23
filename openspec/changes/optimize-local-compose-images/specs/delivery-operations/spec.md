## MODIFIED Requirements

### Requirement: Compose 本地编排与时区一致性

系统 MUST 提供可从仓库根目录运行的本地 Compose 编排，默认启动 PostgreSQL、Redis、Nacos、Jaeger 和 user-service，并通过 `observability` profile 按需启动 Prometheus 和 Grafana；缺少必填 secret 时 MUST 提前失败。常驻服务与一次性任务 MUST 显式使用 `TZ=Asia/Shanghai`；端口 MUST 只暴露当前真实入口。

#### Scenario: 全部 Compose 服务声明时区

- **WHEN** 渲染 `deployments/compose/docker-compose.yml`
- **THEN** PostgreSQL、Redis、Nacos、Nacos 初始化任务、Jaeger、RBAC seed、user-service、Prometheus 和 Grafana MUST 都获得 `TZ=Asia/Shanghai`
- **AND** 配置 MUST NOT 挂载宿主机 `/etc/localtime` 或把宿主机时区作为正确性前提

#### Scenario: 默认与按需观测服务

- **WHEN** 不启用 profile 启动本地 Compose
- **THEN** Compose MUST 启动核心依赖、Jaeger 和 user-service，MUST NOT 启动 Prometheus 或 Grafana
- **WHEN** 启用 `observability` profile 启动本地 Compose
- **THEN** Compose MUST 同时提供 Prometheus 和 Grafana，并沿用各自原有的依赖、配置、数据卷和本地端口

#### Scenario: RBAC seed 复用服务镜像

- **WHEN** 已构建 user-service 镜像后运行 `rbac-seed`
- **THEN** RBAC seed MUST 使用与 user-service 相同的本地镜像，MUST NOT 单独声明同一镜像的构建

#### Scenario: Jaeger 基础镜像缺少 zoneinfo

- **WHEN** 当前 Jaeger 基础镜像不包含 `/usr/share/zoneinfo/Asia/Shanghai`
- **THEN** 本地 Jaeger 镜像 MUST 从固定且可审查的构建阶段复制该 IANA zoneinfo，并在进程启动后解析为 UTC+8
- **AND** 薄镜像 MUST 保留基础 Jaeger 的 entrypoint、端口与功能，MUST NOT 为时区修复引入 Jaeger 版本迁移

#### Scenario: PostgreSQL 已有数据卷

- **WHEN** Compose 使用已经初始化且配置为 UTC 的 PostgreSQL data volume 重建容器
- **THEN** PostgreSQL session `timezone` 与 `log_timezone` MUST 在不删除 volume、不运行 migration 的情况下变为 `Asia/Shanghai`
- **AND** 官方 `docker-entrypoint.sh` 与健康检查 MUST 继续正常工作

#### Scenario: 默认本地入口端口

- **WHEN** 调用方使用必填本地环境变量渲染 `deployments/compose/docker-compose.yml`
- **THEN** Compose MUST 发布 user-service HTTP `8080:8080`，以及 PostgreSQL、Redis 和 Jaeger 的本地入口端口
- **AND** 启用 `observability` profile 时 Compose MUST 发布 Prometheus 和 Grafana 的本地入口端口
- **AND** Compose MUST NOT 发布 `19090:9090`
- **AND** Compose MUST NOT 发布 `6060:6060`

#### Scenario: 当前无真实入站 gRPC API

- **WHEN** user-service 尚未提供真实入站 gRPC API
- **THEN** Compose 默认 MUST 设置 `AEGISCORE_SERVER_GRPC_ENABLED=false`
- **AND** Compose MUST NOT 为 user-service 发布宿主 gRPC 端口

## MODIFIED Requirements

### Requirement: Compose 本地编排

系统 MUST 提供可从仓库根目录运行的本地 Compose 编排，启动 PostgreSQL、Redis、Jaeger OTLP、本地用户服务、Prometheus 和 Grafana。Compose MUST 使用本地必填环境变量注入数据库、JWT 和 Grafana 密码，MUST 在缺少必填值时提前失败。Compose 默认 MUST 只发布当前真实本地入口端口，MUST NOT 默认发布没有真实入站 API 的 gRPC 端口或默认关闭的诊断端口。

#### Scenario: 默认本地入口端口

- **WHEN** 调用方使用必填本地环境变量渲染 `deployments/compose/docker-compose.yml`
- **THEN** Compose MUST 发布 user-service HTTP `8080:8080`
- **AND** Compose MUST 发布 PostgreSQL、Redis、Jaeger、Prometheus 和 Grafana 的本地入口端口
- **AND** Compose MUST NOT 发布 `19090:9090`
- **AND** Compose MUST NOT 发布 `6060:6060`

#### Scenario: 当前无真实入站 gRPC API

- **WHEN** user-service 尚未提供真实入站 gRPC API
- **THEN** Compose 默认 MUST 设置 `AEGISCORE_SERVER_GRPC_ENABLED=false`
- **AND** Compose MUST NOT 为 user-service 发布宿主 gRPC 端口

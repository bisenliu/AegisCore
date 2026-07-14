## ADDED Requirements

### Requirement: 运行配置交付一致性

仓库中的本地配置、测试 fixture、Compose、Docker、Kubernetes、Helm、脚本和文档 MUST 使用当前有效的 runtime config 契约，MUST NOT 把旧字段描述为有效配置。

#### Scenario: 交付 user-service 配置

- **WHEN** user-service 通过本地或部署资产启动
- **THEN** server MUST 使用 `server.http` 和默认禁用的 `server.grpc`
- **AND** Redis/PostgreSQL MUST 使用 `resources.redis` 和 `resources.postgres`
- **AND** password MUST 使用环境变量或 Secret 注入而不是明文示例

#### Scenario: 使用配置环境变量

- **WHEN** 部署覆盖嵌套配置字段
- **THEN** 环境变量 MUST 使用新路径，例如 `AEGISCORE_SERVER_HTTP_PORT`
- **AND** Redis MUST 使用 `AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_*` 和统一 `TIMEOUT`
- **AND** PostgreSQL MUST 使用 `AEGISCORE_RESOURCES_POSTGRES_USER_DB_*` 和 `POOL_*`
- **AND** 进程时区 MUST 使用平台 `TZ`
- **AND** MUST NOT 继续使用旧顶层 HTTP、Redis 或 PostgreSQL 路径

#### Scenario: 交付观测与入口边界

- **WHEN** 生成 Compose、Kubernetes 或 Helm 运行时配置
- **THEN** 日志 MUST 只输出 stdout/stderr
- **AND** tracing enabled 时 MUST 通过 OTLP endpoint 导出
- **AND** pprof MUST 默认不配置或暴露，临时诊断 MUST 使用 loopback 和受控端口转发
- **AND** trusted proxy MUST 由 Ingress、gateway 或 service mesh 入口策略治理

#### Scenario: 扫描旧配置契约

- **WHEN** 执行架构和仓库验证
- **THEN** 有效配置与文档示例 MUST NOT 包含 system、顶层 http、顶层 redis、顶层 postgres、local_cache、http.pprof、http.trusted_proxies、文件日志或 tracing exporter

## MODIFIED Requirements

### Requirement: Docker-backed 集成测试 CI 门禁

GitHub Actions MUST 在阻塞式测试 job 中启用真实 PostgreSQL/Redis 测试，并使用唯一规范开关 `AEGISCORE_TEST_CONTAINERS=1`。该门禁 MUST 覆盖共享容器基础设施、依赖 PostgreSQL 特有语义的 store 测试和完整 user-service HTTP E2E。

#### Scenario: CI 启用规范容器测试开关

- **WHEN** PR 或主线 push 执行阻塞式 test job
- **THEN** job MUST 设置 `AEGISCORE_TEST_CONTAINERS=1` 并运行 `make test`
- **AND** 系统 MUST NOT 增加或读取 `TEST_CONTAINERS` 兼容别名
- **AND** 仅设置 `AEGISCORE_TEST_E2E` MUST NOT 作为完整容器测试门禁的替代方案

#### Scenario: 真实依赖测试不得静默跳过

- **WHEN** CI test job 已设置 `AEGISCORE_TEST_CONTAINERS=1`
- **THEN** PostgreSQL/Redis smoke test、role PostgreSQL 集成测试和 user-service HTTP E2E MUST 实际执行
- **AND** Docker daemon、镜像拉取、容器启动、migration 应用或测试前置条件失败 MUST 使 job 失败，而不是转为 skip

#### Scenario: E2E harness 使用完整有效配置

- **WHEN** user-service HTTP E2E 启动 Fx runtime
- **THEN** harness 生成的配置 MUST 满足当前严格配置契约，包括 `server.http`、具名 resources、feature cache、metrics 和 tracing
- **AND** E2E MUST 启动真实 PostgreSQL 与 Redis、应用已提交 SQL migration 并执行认证和用户 HTTP flow

#### Scenario: 轻量测试 job 不重复真实容器负载

- **WHEN** CI 执行 verify、race 或 coverage job
- **THEN** 这些 job MAY 保持默认容器测试关闭状态
- **AND** 阻塞式 Docker-backed test job MUST 成为真实依赖通过状态的唯一权威门禁

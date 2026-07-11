## ADDED Requirements

### Requirement: user-service 最小运行时镜像

系统 MUST 使用固定 digest 的 Distroless static nonroot 基础镜像交付 user-service 运行时，并 MUST 只包含服务运行和显式 user-service CLI 命令所需文件。运行时镜像 MUST NOT 包含 shell、包管理器、下载工具、通用文本处理工具、Atlas CLI 或 Distroless debug 工具。

#### Scenario: 构建静态 Distroless 运行时镜像

- **WHEN** CI 或协作者从仓库根目录构建 `deployments/docker/user-service.Dockerfile`
- **THEN** builder MUST 显式使用 `CGO_ENABLED=0` 生成 Linux 静态二进制
- **AND** runtime stage MUST 使用固定多架构 digest 的 `gcr.io/distroless/static-debian12:nonroot`
- **AND** 最终镜像 MUST 能执行 `serve`、`rbac`、`fxgraph`、`healthcheck` 和根命令 help

#### Scenario: 运行时攻击面断言

- **WHEN** CI 检查最终 user-service 镜像内容
- **THEN** `/bin/sh`、`/busybox/sh`、`apk`、`wget`、`curl`、`grep` 和 `/usr/local/bin/atlas` MUST 不存在
- **AND** 镜像 MUST 通过配置的 HIGH/CRITICAL 漏洞门禁并生成 image SBOM

#### Scenario: 运行时基础数据可用

- **WHEN** user-service 在 Distroless 镜像中加载生产或本地配置
- **THEN** CA certificates MUST 可用于 TLS 连接
- **AND** `time.LoadLocation` MUST 能加载配置的 IANA timezone，包括 `Asia/Shanghai`
- **AND** `/tmp` MUST 可供数值 nonroot 用户写入临时日志或运行时文件

#### Scenario: 数值运行身份一致

- **WHEN** user-service 由 Docker、Compose、Kubernetes 或 Helm 启动
- **THEN** 镜像和部署资产 MUST 统一使用 UID/GID `65532`
- **AND** Kubernetes 与 Helm 的 `runAsUser`、`runAsGroup` 和 `fsGroup` MUST 与该身份一致
- **AND** 系统 MUST NOT 保留命名用户 `aegiscore`、UID/GID `10001` 或双身份兼容逻辑

#### Scenario: 禁止调试运行时 fallback

- **WHEN** 协作者需要诊断运行中的 user-service 容器
- **THEN** 生产运行时镜像 MUST 继续使用无 shell 的 Distroless static nonroot 变体
- **AND** 部署资产 MUST NOT 切换到 Alpine、Distroless debug 或动态下载调试工具作为长期诊断方案

### Requirement: user-service 原生容器健康检查

user-service CLI MUST 提供无外部命令依赖的 `healthcheck` 子命令，用于容器内部检查 HTTP 健康端点。该命令 MUST 使用有限超时，并通过进程退出码表达健康结果。

#### Scenario: 就绪检查成功

- **WHEN** `healthcheck` 请求目标 `/readyz` 并收到 HTTP 2xx 与有效 ready 响应
- **THEN** 命令 MUST 以退出码 0 结束
- **AND** 命令 MUST NOT 依赖 shell、wget、curl、grep 或其他镜像内工具

#### Scenario: 就绪检查失败

- **WHEN** 目标连接失败、请求超时、返回非 2xx、返回无效响应或报告非 ready 状态
- **THEN** `healthcheck` MUST 以非 0 退出码结束
- **AND** 错误输出 MUST 提供可诊断但不包含凭据的失败原因

#### Scenario: Compose 使用 exec-form 健康检查

- **WHEN** Compose 启动 user-service
- **THEN** user-service healthcheck MUST 以 `CMD` exec-form 调用当前镜像内的 `healthcheck` 子命令
- **AND** Compose MUST NOT 使用 `CMD-SHELL`、pipe 或镜像外部下载的探针工具

#### Scenario: Kubernetes 探针保持 HTTP 语义

- **WHEN** Kubernetes 或 Helm 启动 user-service Deployment
- **THEN** liveness、readiness 和 startup probe MUST 继续分别使用 kubelet `httpGet` 请求 `/livez`、`/readyz` 和 `/startupz`
- **AND** Kubernetes probe MUST NOT 依赖容器内 `healthcheck` 命令或 shell

### Requirement: user-service 可验证且缓存友好的 Docker 构建

user-service Docker 构建 MUST 使用 BuildKit、不可变基础镜像输入、只读 Go module 解析和分离的依赖/源码缓存边界。缓存 MUST 只用于加速构建，不得替代 checksum、digest 或安全扫描。

#### Scenario: workspace manifest 独立缓存层

- **WHEN** Docker 构建准备 Go 依赖
- **THEN** Dockerfile MUST 在复制源码前复制 `go.work`、`go.work.sum` 和所有 workspace module 的 `go.mod`
- **AND** 对存在 `go.sum` 的 module MUST 同时复制对应文件
- **AND** `tools/openapi-convert/go.mod` MUST 作为 workspace manifest 参与依赖层 cache key

#### Scenario: BuildKit module 与编译缓存

- **WHEN** Docker 构建下载依赖或编译 user-service
- **THEN** 构建步骤 MUST 挂载持久化 `/go/pkg/mod` cache
- **AND** 编译步骤 MUST 挂载 Go build cache
- **AND** Dockerfile MUST NOT 提供 legacy builder 的无 cache 兼容分支

#### Scenario: 只读且可重复的 Go 构建

- **WHEN** builder 编译 `user-service/cmd`
- **THEN** workspace checksum MUST 已规范化并提交
- **AND** 构建 MUST 使用 `-mod=readonly`、`-trimpath` 和显式 VCS metadata 策略
- **AND** 构建过程 MUST NOT 修改 `go.work.sum`、module `go.mod` 或 module `go.sum`

#### Scenario: 基础镜像输入不可变

- **WHEN** Dockerfile 声明 builder 或 runtime 基础镜像
- **THEN** 每个基础镜像 MUST 同时保留可读版本 tag 并固定审核后的 digest
- **AND** digest 更新 MUST 通过依赖升级流程接受 review 和完整镜像验证

#### Scenario: CI 复用同一镜像工件

- **WHEN** CI 对同一提交执行镜像构建、内容断言、漏洞扫描和 SBOM 生成
- **THEN** 所有步骤 MUST 使用同一 image ID 或不可变 digest
- **AND** CI MUST NOT 在独立 runner 无共享 cache 地重复冷构建同一提交
- **AND** BuildKit cache MUST 使用 GitHub Actions cache backend 或等价 external cache 持久化

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
- **THEN** harness 生成的配置 MUST 满足当前 config validation，包括 metrics path 和 tracing exporter
- **AND** E2E MUST 启动真实 PostgreSQL 与 Redis、应用已提交 SQL migration 并执行认证和用户 HTTP flow

#### Scenario: 轻量测试 job 不重复真实容器负载

- **WHEN** CI 执行 verify、race 或 coverage job
- **THEN** 这些 job MAY 保持默认容器测试关闭状态
- **AND** 阻塞式 Docker-backed test job MUST 成为真实依赖通过状态的唯一权威门禁

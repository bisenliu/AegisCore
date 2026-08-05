## MODIFIED Requirements

### Requirement: 质量门禁、架构诊断与可复现生成

系统 MUST 提供模块级和仓库级测试、lint、架构检查、生成和完整 verify 入口。测试与生成 MUST 可诊断、可复现且不得扩张正式生产 API。`common`、`user-service`、`tools/openapi-convert` 和 `tools/nacos-config-seed` 四个 Go module MUST 进入仓库级 lint、`govulncheck` 和 `gosec` 门禁，并在完整仓库 checkout 中保持可独立校验的 module metadata。`user-service-architecture-lint` MUST 保护正式架构来源声明的边界；Fx 依赖图诊断 MUST 无外部及运行时激活副作用。OpenAPI 转换库、CLI 和服务脚本 MUST 分别由 `common/http/openapi`、`tools/openapi-convert` 和 user-service 拥有其通用逻辑、可执行入口与服务参数。GitHub Actions MUST 由主 CI workflow 唯一拥有 PR 与主线 push 的标准质量触发，并通过仅支持 `workflow_call` 的复用 workflow 为同一 commit 各执行一次 lint 和普通单测。

#### Scenario: 统一质量与生成门禁

- **WHEN** 执行 `make test`、`make lint` 或 `make verify`
- **THEN** 系统 MUST 分别运行四个 Go module 的测试、各 module 的 `golangci-lint`，或覆盖 lint、架构检查、测试、必要生成并以 `git diff --exit-code` 检测 drift
- **WHEN** PR 或主线 push 触发 GitHub Actions 标准质量门禁
- **THEN** 主 CI MUST 仅调用一次复用质量 workflow，且同一 commit MUST 只产生一组稳定命名的 `quality / lint` 与 `quality / unit` 检查
- **AND** 复用质量 workflow MUST 仅接受 `workflow_call`，MUST NOT 同时直接监听与主 CI 重叠的 `pull_request` 或主线 `push`
- **AND** CI 的架构/生成检查和 Docker-backed 测试 MUST 使用独立 job，MUST NOT 再次执行 `make lint` 或普通 `make test`
- **WHEN** CI 执行 Go 漏洞和静态安全扫描
- **THEN** `govulncheck` 与 `gosec` matrix MUST 分别覆盖四个 Go module，并使用不含 module 路径分隔符的稳定 job、SARIF 和 artifact 名称
- **WHEN** CI 检查 user-service 架构、OpenAPI 或 migration
- **THEN** MUST 使用 `make user-service-architecture-lint`、`make user-service-openapi-generate` 和 `make user-service-migrate-validate`，MUST NOT 调用不存在或缺少服务前缀的私有目标
- **WHEN** package 需要 mock、metrics no-op 或其他 Go 生成物
- **THEN** module MUST 显式声明工具依赖，生成入口 MUST 归消费 package 所有且排除正常构建，生成物 MUST 位于测试边界或所属 feature；`make common-generate`、`make user-service-generate` 和 verify MUST 可重建并检测 drift
- **WHEN** 非测试 Go 文件引入仅测试消费、暴露内部状态、全局可变替身或等价测试专用 API
- **THEN** 架构检查 MUST 拒绝，测试 MUST 使用现有实现、局部依赖注入、消费侧最小接口或 `common/testing`，MUST NOT 驱动冗余生产分支和适配层
- **WHEN** 测试构造 Fx app、feature module、启动失败或 rollback，或直接调用可能阻塞的停止函数
- **THEN** 测试 MUST 保留可定位诊断信息，并使用带 timeout 的 context 和测试级 guard；实现不尊重 context 时 MUST 在 guard 内失败，MUST NOT 退化为等待全局 `go test -timeout`

#### Scenario: Module metadata 独立维护

- **WHEN** 在完整仓库 checkout 中分别进入任一 Go module 执行 `GOWORK=off go mod tidy -diff`
- **THEN** 命令 MUST 成功且不得产生 `go.mod` 或 `go.sum` drift
- **AND** `tools/openapi-convert` MUST 通过仓库内受控依赖策略解析相邻 `common` 源码，MUST NOT 尝试下载不可用的 `github.com/aegiscore/common@v0.0.0`
- **AND** 该契约 MUST NOT 被解释为 `tools/openapi-convert` 可脱离完整仓库 checkout 单独分发

#### Scenario: 架构边界与 Fx graph

- **WHEN** 业务代码违反 feature-first、分层、共享边界、生成配置或部署契约
- **THEN** `user-service-architecture-lint` MUST 失败并指向正式架构来源；当前不存在的 gRPC、MQ、eventbus 或 outbox MUST NOT 以空壳或推测性实现进入正式边界
- **WHEN** 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`
- **THEN** 系统 MUST 基于正式配置投影和无运行时激活的 wiring graph 或专用 graph root 生成非空 DOT
- **AND** 生成过程 MUST NOT 执行生产 `fx.Invoke`、连接 PostgreSQL/Redis/OTLP、启动 listener、创建 workerpool/localcache/tracing exporter 后台资源、注册真实 route 或 runtime metrics，也 MUST NOT 修改 `TZ`、`time.Local` 或 Gin mode
- **WHEN** `serve` 构建正式 Fx App
- **THEN** 系统 MUST 使用同时包含 wiring 与 runtime module 的正式 App module，并保持 HTTP、pprof、route、dependency metrics、timezone、RBAC lifecycle 和 hooks 的运行时语义；graph root MUST NOT 取代该激活链路

#### Scenario: OpenAPI 转换、生成与 drift

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** user-service 脚本 MUST 调用仓库级 CLI 将 Swagger 转换为 OpenAPI 3，并更新 `openapi.go`、`openapi.json` 和 `openapi.yaml`
- **WHEN** 生成需要 server、探活路径、security scheme 或输出路径
- **THEN** user-service 脚本 MUST 显式传参，通用 CLI MUST NOT 写死 `/api/v1`、健康路由或 `BearerAuth` 等服务语义
- **WHEN** 转换工具或依赖变化
- **THEN** 测试 MUST 验证结构化输出、输出 writer 错误和其他错误路径，完整验证 MUST 检测生成物及依赖 drift

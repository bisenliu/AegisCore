## Purpose

定义 AegisCore 的交付运维能力，覆盖构建、运行、测试、lint、架构检查、代码生成、数据库迁移、部署资产和发布顺序。
## Requirements
### Requirement: 构建与本地运行

系统 MUST 提供统一 Makefile 入口构建和运行 user-service，并支持通过配置文件启动服务。

#### Scenario: 构建全部服务

- **WHEN** 协作者执行 `make build`
- **THEN** 系统 MUST 构建 user-service 二进制到配置的 `USER_SERVICE_BIN`

#### Scenario: 运行 user-service

- **WHEN** 协作者执行 `make user-service-run`
- **THEN** 系统 MUST 使用 `USER_SERVICE_CONFIG` 指定的 YAML 配置运行 `aegiscore-user-services serve`

#### Scenario: 查看命令帮助

- **WHEN** 协作者执行 `make help` 或 `make -C user-service help`
- **THEN** 系统 MUST 输出可用命令及中文说明

### Requirement: user-service 进程生命周期超时配置

系统 MUST 支持通过 YAML 配置文件声明 `aegiscore-user-services serve` 的 Fx app 进程级启动和关闭超时。`runtime.lifecycle.start_timeout` MUST 控制 `app.Start` 的最长等待时间，`runtime.lifecycle.stop_timeout` MUST 控制收到 `SIGINT` 或 `SIGTERM` 后 `app.Stop` 的最长等待时间。未声明这些字段时，系统 MUST 使用默认正数超时；显式配置非正数超时时，系统 MUST 拒绝启动并返回配置校验错误。

`user-service/cmd/serve.go` MUST NOT 定义 Fx app lifecycle timeout 默认常量；默认值 MUST 由共享配置层统一提供，serve 命令只消费已加载并校验过的配置值。

#### Scenario: 使用默认生命周期超时启动服务

- **WHEN** 协作者使用未声明 `runtime.lifecycle.start_timeout` 和 `runtime.lifecycle.stop_timeout` 的配置文件执行 `aegiscore-user-services serve`
- **THEN** 系统 MUST 使用默认 Fx app 启动和关闭超时
- **AND** 默认 `runtime.lifecycle.stop_timeout` MUST 大于或等于默认 `server.http.shutdown_timeout` 和默认 `server.grpc.shutdown_timeout`

#### Scenario: 使用配置化生命周期超时启动服务

- **WHEN** 配置文件声明 `runtime.lifecycle.start_timeout: 60s` 和 `runtime.lifecycle.stop_timeout: 120s`
- **THEN** `aegiscore-user-services serve` MUST 使用 `60s` 作为 Fx app 启动 context timeout
- **AND** 收到 `SIGINT` 或 `SIGTERM` 后 MUST 使用 `120s` 作为 Fx app 停止 context timeout
- **AND** 停止阶段 MUST 保持未被信号取消的上游 context value 传递语义

#### Scenario: 拒绝无效生命周期超时

- **WHEN** 配置文件声明非正数 `runtime.lifecycle.start_timeout` 或 `runtime.lifecycle.stop_timeout`
- **THEN** 系统 MUST 拒绝加载配置并返回可定位的配置校验错误

#### Scenario: 总关闭预算覆盖协议关闭预算

- **WHEN** 配置文件声明 `runtime.lifecycle.stop_timeout` 小于 `server.http.shutdown_timeout` 或 `server.grpc.shutdown_timeout`
- **THEN** 系统 MUST 拒绝加载配置并返回可定位的配置校验错误

#### Scenario: 不改变协议和业务行为

- **WHEN** 仅新增或调整 `runtime.lifecycle.start_timeout` 与 `runtime.lifecycle.stop_timeout`
- **THEN** 系统 MUST NOT 改变 HTTP API、OpenAPI、数据库 schema、RBAC、认证会话、metrics 指标契约或 HTTP/gRPC server shutdown timeout 的语义

#### Scenario: CLI 层不保留默认常量

- **WHEN** 系统实现 runtime lifecycle timeout 配置
- **THEN** `user-service/cmd/serve.go` MUST NOT 保留 `fxAppStartTimeout` 或 `fxAppStopTimeout` 默认常量
- **AND** serve 命令 MUST 使用配置 loader 返回的 lifecycle timeout 构造 `app.Start` 和 `app.Stop` context

### Requirement: 测试、lint 和完整验证

系统 MUST 提供统一测试、lint、架构边界检查和完整 verify 入口。OpenSpec change 的最终 `make lint` 和 `make verify` MUST 在全部实现、规格和文档任务完成后执行，且执行前 MUST 先暂存本次预期变更。

#### Scenario: 运行全部测试

- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 运行 `common` 和 `user-service` 的 Go 测试

#### Scenario: 运行 lint

- **WHEN** 协作者执行 `make lint`
- **THEN** 系统 MUST 运行各 Go 模块的 `golangci-lint`

#### Scenario: OpenSpec 最终 lint 顺序

- **WHEN** 协作者准备完成 OpenSpec change 并执行最终 `make lint`
- **THEN** 协作者 MUST 已完成该 change 的实现、规格和文档任务
- **AND** 协作者 MUST 先将本次预期代码和文档变更加到暂存区

#### Scenario: 运行完整验证

- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 依次执行 lint、user-service 架构边界检查、测试、OpenAPI 生成，并通过 `git diff --exit-code` 暴露生成物 drift

#### Scenario: OpenSpec 最终 verify 顺序

- **WHEN** 协作者准备完成 OpenSpec change 并执行最终 `make verify`
- **THEN** 协作者 MUST 已完成该 change 的实现、规格和文档任务
- **AND** 协作者 MUST 先将本次预期代码和文档变更加到暂存区
- **AND** `make verify` 的最终 `git diff --exit-code` MUST 用于暴露生成物 drift 或未纳入暂存区的意外变更

### Requirement: Go 测试断言与失败处理

Go 测试 MUST 优先使用 `testify/require` 表达错误、对象、数值、集合、字符串、专属类型和前置条件等语义化断言，并通过立即失败机制减少后续空指针、错误状态级联和手写判断样板。当 `testify` 已提供更具体的语义化断言时，测试 MUST 使用对应断言，而不是通过 `True`、`False`、手写 `if` 或组合多个基础断言来表达同一语义。

#### Scenario: 使用语义化 require 断言

- **WHEN** Go 测试断言错误返回值、对象和值、数值范围、集合、字符串或专属类型行为
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.ErrorContains`、`require.Equal`、`require.NotEqual`、`require.Nil`、`require.NotNil`、`require.Greater`、`require.Less`、`require.GreaterOrEqual`、`require.Len`、`require.Empty`、`require.Contains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 或等价语义化断言
- **AND** 测试 MUST NOT 使用 `require.True`、`require.False`、手写 `if` 或多个基础断言拼凑上述已有语义化断言可以清晰覆盖的检查

#### Scenario: 布尔状态断言例外

- **WHEN** 测试断言对象自身暴露的布尔状态、channel 是否关闭等本质上就是布尔值的结果，且 `testify` 没有更具体的语义化断言
- **THEN** 测试 MAY 使用 `require.True` 或 `require.False`

#### Scenario: 多个独立失败收集

- **WHEN** 单个测试需要在一次执行中收集多个相互独立的断言失败，且后续检查不依赖前置检查成功
- **THEN** 测试 MAY 使用 `testify/assert` 表达这些独立检查
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果时，测试 MUST 使用 `require` 立即终止当前测试

#### Scenario: 禁止机械 Fail 替换

- **WHEN** 测试迁移或新增失败处理
- **THEN** 测试 MUST NOT 将手写失败判断机械替换成 `require.FailNow`、`require.FailNowf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **AND** 存在明确语义化断言时，测试 MUST 优先使用对应的 `require` 或 `assert` 方法

#### Scenario: 保留 testing.T 失败方法例外

- **WHEN** 测试仍直接使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf`
- **THEN** 该用法 MUST 属于无法通过现有语义化断言清晰表达的自定义测试控制流、特殊诊断输出，或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 保留原因 MUST 在代码上下文中保持清晰

### Requirement: Go 生成物 drift 校验

系统 MUST 将 mock 生成物和 metrics no-op 生成物纳入 Go 生成与交付验证流程。完整验证 MUST 能在生成物过期、缺失或未提交时通过 drift 检查失败。认证 HTTP controller 使用的 use case mock MUST 由 auth HTTP transport 本地 `mock_generate.go` 声明，并由仓库约定生成命令维护。

#### Scenario: 生成 mock 和 metrics no-op

- **WHEN** 协作者执行仓库约定的 Go 生成命令
- **THEN** 系统 MUST 生成 `go.uber.org/mock/mockgen` mock 文件和 metrics no-op 文件
- **AND** 生成命令 MUST 覆盖 `common` 与 `user-service` 中声明的相关 `go:generate` 入口

#### Scenario: 完整验证发现生成物 drift

- **WHEN** mock 或 metrics no-op 源 interface 变化但生成物未同步
- **THEN** `make verify` 或等价完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift
- **AND** drift 未解决前 change MUST NOT 被视为完成

#### Scenario: 工具依赖可复现

- **WHEN** 新增或更新 mock 生成入口
- **THEN** 对应 Go module MUST 显式声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口
- **AND** 生成流程 MUST NOT 依赖开发者机器上的隐式全局 `mockgen` 二进制

#### Scenario: auth HTTP controller mockgen 入口

- **WHEN** 认证 HTTP controller 测试需要模拟 `LoginUseCase`、`RefreshTokenUseCase`、`ChangePasswordUseCase`、`LogoutCurrentSessionUseCase` 或 `LogoutAllSessionsUseCase`
- **THEN** mockgen 入口 MUST 位于 `user-service/internal/features/auth/transport/http/mock_generate.go`
- **AND** 生成 mock MUST 位于 `auth/transport/http` 测试包内
- **AND** 生成物 MUST NOT 进入全局 `mocks/` 目录或跨 feature mock 包

### Requirement: 测试 mock 生成入口不参与正式构建

仓库中仅承载 `go:generate mockgen` 的 `mock_generate.go` MUST 归消费 mock 的 package 所有，并 MUST 使用 `generate` build tag 从正常 Go 构建排除。`make common-generate`、`make user-service-generate` 和完整 verify MUST 继续生成、校验并检测 mock 生成物 drift。

#### Scenario: generate-only 源文件保留本地生成入口

- **WHEN** package 通过 `mock_generate.go` 声明一个或多个 `go:generate mockgen` 指令
- **THEN** 文件 MUST 包含 `//go:build generate`
- **AND** 指令 MUST 继续输出到该消费 package 的 `_test.go` mock 生成物

#### Scenario: 正常构建排除测试生成入口

- **WHEN** 系统对 `common` 或 `user-service` package 执行普通 `go list`、`go build` 或 `go test`
- **THEN** `mock_generate.go` MUST NOT 出现在正常 package `GoFiles` 中
- **AND** 正式二进制 MUST NOT 因 mock 生成入口增加可编译生产源文件

#### Scenario: 生成命令仍可发现全部指令

- **WHEN** 开发者执行 `make common-generate` 或 `make user-service-generate`
- **THEN** Go generate MUST 使用 `generate` build tag 发现对应 `mock_generate.go`
- **AND** 所有已登记 mock MUST 能在删除或过期后被重新生成

#### Scenario: 生成物 drift 阻塞交付

- **WHEN** mock 接口、生成指令或生成物内容不一致
- **THEN** 模块 generate/verify 或仓库 `make verify` MUST 通过 `git diff --exit-code` 暴露 drift
- **AND** 系统 MUST NOT 通过跳过生成、保留旧 mock 或复制兼容文件绕过失败

### Requirement: 测试适配生产代码受交付门禁约束

仓库的架构或交付检查 MUST 拒绝明确带有测试语义且进入正式构建的 API、hook、分支或生成入口。自动检查 MUST 聚焦可确定的结构特征，其他仅测试引用的导出符号 MUST 结合主规格、生产调用图和运行时职责人工复核。

#### Scenario: 拒绝显式 test-only 正式 API

- **WHEN** 人工维护的非 `_test.go` Go 文件新增 `ForTest`、`set*ForTest`、`testHook` 或等价测试语义 symbol
- **THEN** 架构检查 MUST 失败并定位文件
- **AND** 实现 MUST 将该逻辑移入测试边界或改为具有真实运行时职责的依赖设计

#### Scenario: 拒绝未隔离的 mock 生成文件

- **WHEN** 新增或修改的 `mock_generate.go` 缺少 `generate` build tag
- **THEN** 架构或交付检查 MUST 失败
- **AND** 检查 MUST 覆盖 `common/` 与 `user-service/` 的全部人工维护 package

#### Scenario: 人工复核低调用量公开 API

- **WHEN** 扫描发现导出 symbol 只有测试消费者或注释包含测试用途
- **THEN** 实施者 MUST 检查主规格、生产调用图、跨模块职责和可替代测试手段后再决定删除或保留
- **AND** 扫描结果 MUST NOT 直接作为删除共享 API、协议兼容或安全边界的唯一依据

### Requirement: CI 工作流使用存在的服务前缀根目标

系统 MUST 确保 GitHub Actions 中的交付校验步骤调用仓库根 `Makefile` 中存在的目标；当步骤执行 user-service 私有交付能力时，目标名称 MUST 使用 `user-service-` 前缀。

#### Scenario: PR 门禁运行 user-service 架构 lint

- **WHEN** GitHub Actions PR 或 push verify job 需要执行 user-service 架构边界检查
- **THEN** 工作流 MUST 调用 `make user-service-architecture-lint`

#### Scenario: PR 门禁运行 user-service OpenAPI 生成

- **WHEN** GitHub Actions PR 或 push verify job 需要检查 user-service OpenAPI 生成物是否存在 drift
- **THEN** 工作流 MUST 调用 `make user-service-openapi-generate`
- **THEN** 工作流 MUST 继续通过 `git diff --exit-code` 暴露生成物 drift

#### Scenario: migration 工作流校验 user-service migrations

- **WHEN** GitHub Actions migration validation job 需要校验 user-service Atlas migrations
- **THEN** 工作流 MUST 调用 `make user-service-migrate-validate`

#### Scenario: 禁止无服务上下文目标

- **WHEN** GitHub Actions workflow 需要调用 user-service 私有 lint、生成或 migration 目标
- **THEN** 工作流 MUST NOT 调用根 `Makefile` 中不存在的 `architecture-lint`、`openapi-generate` 或 `migrate-validate` 目标

### Requirement: user-service Fx 依赖图生成

系统 MUST 为 user-service 提供可执行的 Fx 依赖图生成入口，并通过带 `user-service-` 前缀的交付命令暴露给协作者。

#### Scenario: 生成 user-service 依赖图

- **WHEN** 协作者执行 user-service Fx 依赖图生成命令
- **THEN** 系统 MUST 基于 user-service 当前顶层 Fx module 生成依赖图文件
- **AND** 生成过程 MUST 复用 `common/` 中的业务中立 Fx 依赖图 helper

#### Scenario: 根 Makefile 使用服务前缀

- **WHEN** 仓库根 `Makefile` 暴露 user-service Fx 依赖图生成能力
- **THEN** 目标名称 MUST 使用 `user-service-` 前缀
- **AND** 根 `Makefile` MUST NOT 新增无服务上下文的 `fxgraph-generate`、`dependency-graph` 或等价目标

#### Scenario: 依赖图 drift 可检查

- **WHEN** user-service provider、module 或 invoke 关系变化后重新生成依赖图
- **THEN** 系统 MUST 能通过提交的生成物 diff 或专用 check 命令暴露依赖图 drift

### Requirement: 架构边界检查

系统 MUST 提供 user-service 架构 lint，用于检查 feature 分层依赖、禁止跨层违规引用，并校验 OpenSpec/OPSX Markdown 语言约束。架构 lint MUST 明确依赖 `ripgrep` 提供的 `rg` 命令，并在缺少该命令时 fail-fast 输出可诊断错误；CI 完整验证环境 MUST 在运行架构 lint 前安装该依赖。

#### Scenario: 分层引用合法

- **WHEN** feature 内代码遵循 domain、application、infrastructure、transport 的依赖方向
- **THEN** `make user-service-architecture-lint` MUST 通过

#### Scenario: 分层引用违规

- **WHEN** 代码出现违反架构边界的 import 或跨 feature 非法依赖
- **THEN** 架构 lint MUST 失败并输出违规位置

#### Scenario: 缺少 ripgrep 前置依赖

- **WHEN** 协作者在缺少 `rg` 命令的环境执行 `make user-service-architecture-lint`
- **THEN** 架构 lint MUST fail-fast
- **AND** 输出 MUST 明确提示需要安装 `ripgrep`

#### Scenario: CI 安装架构 lint 依赖

- **WHEN** GitHub Actions 执行完整验证流程
- **THEN** workflow MUST 在运行 `make lint`、`make user-service-architecture-lint` 或 `make verify` 相关步骤前安装 `ripgrep`

#### Scenario: OPSX 文档残留英文模板

- **WHEN** `openspec/specs/`、`openspec/changes/` 或 `docs/opsx/` 下 Markdown 保留默认英文模板标题或说明
- **THEN** 架构 lint MUST 失败并要求改为简体中文正文

#### Scenario: feature-first 组织违规

- **WHEN** 服务内业务代码新增到横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包
- **THEN** 架构 lint 或 review MUST 阻止该变更，并要求代码放入所属 `internal/features/<feature>/`

#### Scenario: HTTP controller 边界

- **WHEN** HTTP controller 处理请求输入
- **THEN** controller MUST 先调用 `binding.BindOrAbort`，再调用一个 feature-local input preparer，且 MUST NOT 直接导入 Ent、Redis client、SQL package 或基础设施 adapter

#### Scenario: domain 依赖保护

- **WHEN** feature domain 层新增 import
- **THEN** domain MUST NOT 导入 Gin、Ent、Redis、config、logger、response envelope、application ports 或 infrastructure adapter

#### Scenario: 生产代码测试专用 API

- **WHEN** 测试需要 fake、stub、fixture、时间控制或特殊断言入口
- **THEN** 这些能力 SHOULD 位于 `_test.go`、`common/testing` 或对应测试基础设施；正式代码 MUST NOT 暴露 `NewXForTest`、`testHook`、`setNowForTest` 等仅为测试服务的 API，除非它们具有清晰运行时职责

#### Scenario: 避免测试驱动的冗余生产代码

- **WHEN** 新增或调整单元测试
- **THEN** 测试 MUST 基于现有实现和合理的可测试性设计；正式代码 MUST NOT 仅为了单元测试而引入与业务无关的额外逻辑、分支、接口或适配层

### Requirement: 代码生成与数据库迁移

系统 MUST 提供 Ent 代码生成、Atlas migration diff、migration validate 和 migration hash 校验入口，并要求 schema 相关变更同步生成物。系统 MUST NOT 提供通过仓库 Makefile、脚本或部署资产直接连接数据库并执行 `atlas migrate apply` 的入口。user-service Ent 生成配置 MUST 启用支持 RBAC bulk insert 唯一冲突忽略所需的生成特性。user-service Atlas migration 生成链路 MUST 全局禁用数据库真实外键，数据库 SQL migration MUST NOT 创建 `FOREIGN KEY` 或 `REFERENCES` 约束。

#### Scenario: 生成 Ent 代码

- **WHEN** Ent schema 或 Ent 生成特性变化后执行 `make user-service-generate`
- **THEN** 系统 MUST 运行 `go generate ./ent` 并更新 Ent 生成代码
- **AND** 生成代码 MUST 支持 RBAC 批量写入路径使用 bulk create 的唯一冲突忽略能力

#### Scenario: 生成 migration

- **WHEN** 数据库 schema 变化需要生成 migration
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>` 生成 Ent 代码与 Atlas migration，并审查 SQL 与 `atlas.sum`
- **AND** Atlas MUST 通过 user-service 的 external schema loader 读取 Ent 目标 schema
- **AND** external schema loader MUST 输出不含真实数据库外键的 Ent 目标 schema
- **AND** `user-service/scripts/migrate-diff.sh` MUST 在 diff 前强制 build 本地 Atlas pg_trgm dev database image，确保 dev image 与仓库 Dockerfile 当前状态一致

#### Scenario: 本地 Atlas dev image tag 一致

- **WHEN** 维护 user-service Atlas dev database、migration diff 脚本或本地 Compose PostgreSQL 配置
- **THEN** `deployments/docker/atlas-postgres-pgtrgm.Dockerfile`、`user-service/scripts/migrate-diff.sh`、`user-service/migrations/atlas.hcl` 和 `deployments/compose/docker-compose.yml` 中的本地 PostgreSQL image tag MUST 保持一致
- **AND** 当前本地约定 tag MUST 使用 `latest`
- **AND** architecture lint MUST 校验这些 tag 一致性并在 drift 时失败
- **AND** 该约束只适用于本地交付配置，正式或受控环境 SHOULD 固定版本或 digest

#### Scenario: 校验 migration

- **WHEN** migration 准备进入环境或发布流程
- **THEN** 系统 MUST 支持 `make user-service-migrate-validate` 校验已提交 SQL migration 和 `atlas.sum`
- **AND** 系统 MUST NOT 支持通过 `DATABASE_URL` 执行 `make user-service-migrate-apply` 或等价仓库命令连接数据库自动应用 migration

#### Scenario: 手动调整 migration SQL

- **WHEN** 生成的 SQL migration 被手动调整
- **THEN** 协作者 MUST 刷新并提交 `atlas.sum`，且 MUST 确保 `make user-service-migrate-validate` 通过

#### Scenario: 受控执行 SQL migration

- **WHEN** SQL migration 已通过 validate 并准备进入目标数据库
- **THEN** 协作者 MUST 将 SQL migration 和权限要求提交到 Git，并通过 DBA 工单或受控发布平台人工或受控执行
- **AND** 仓库文档 MUST 将标准流程描述为 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行

#### Scenario: pg_trgm 扩展前置

- **WHEN** SQL migration 使用 `gin_trgm_ops` 或其他 `pg_trgm` 能力
- **THEN** 首个 SQL migration 文件 MUST 在相关索引创建前包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`
- **AND** 文档或任务 MUST 提醒生产库执行该语句可能需要 DBA 权限

#### Scenario: 运行时不修改 schema

- **WHEN** user-service 正常启动或 E2E 初始化数据库 schema
- **THEN** schema MUST 来自已提交 Atlas SQL migration，运行时服务代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更

#### Scenario: 禁止数据库真实外键

- **WHEN** 协作者生成或审查 user-service SQL migration
- **THEN** SQL migration MUST NOT 包含 `FOREIGN KEY` 或 `REFERENCES` 约束
- **AND** Ent schema MUST 保留 edge、绑定表关联字段和必要唯一索引，用于代码层关联查询和重复绑定约束
- **AND** 协作者 MUST NOT 通过删除 Ent edge、删除关联字段或绕过 Ent 关联定义来规避数据库外键生成

#### Scenario: Ent 生成特性 drift 检查

- **WHEN** user-service Ent 生成特性发生变化但生成物未同步
- **THEN** `make verify` 或等价完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift

### Requirement: 仓库级 OpenAPI 转换工具

系统 MUST 将跨服务复用的 OpenAPI 转换 CLI 维护在仓库级 `tools/openapi-convert/`，并通过服务脚本传入服务专属生成参数。OpenAPI 转换核心 MUST 保持在 `common/http/openapi`，服务 `internal/` 目录 MUST NOT 承载该通用转换 CLI。

#### Scenario: user-service 生成 OpenAPI

- **WHEN** 协作者执行 `make user-service-openapi-generate`
- **THEN** user-service 生成脚本 MUST 调用 `tools/openapi-convert` 完成 Swagger 2 到 OpenAPI 3 的转换
- **AND** 系统 MUST 更新 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`

#### Scenario: 服务专属生成参数

- **WHEN** 服务生成 OpenAPI 文档时需要配置业务 server、root server、探活路径、security scheme 或输出路径
- **THEN** 对应服务脚本 MUST 显式传入这些参数
- **AND** `tools/openapi-convert` MUST NOT 写死 user-service 的 `/api/v1`、`/livez`、`/readyz`、`/startupz` 或 `BearerAuth` 作为服务语义默认值

#### Scenario: 工具归属边界

- **WHEN** 仓库维护 Swagger/OpenAPI 转换能力
- **THEN** 可复用转换库 MUST 位于 `common/http/openapi`
- **AND** 可执行转换 CLI MUST 位于 `tools/openapi-convert`
- **AND** `user-service/internal/tools/openapi-convert` MUST 不存在

### Requirement: 发布和部署资产

系统 MUST 维护 Docker、Compose、Kubernetes、Helm 和观测部署资产，并明确生产发布中数据库 SQL 执行与 RBAC seed 的顺序。user-service 普通运行时镜像 MUST NOT 包含 Atlas 二进制；仓库提供的部署资产 MUST NOT 自动执行 Atlas migration apply，数据库 SQL migration MUST 由 DBA 工单或受控发布平台在 HTTP rollout 前完成。

#### Scenario: 构建 Docker 镜像

- **WHEN** 协作者执行 Docker build 命令并指定 `deployments/docker/user-service.Dockerfile`
- **THEN** 系统 MUST 能从仓库根目录构建 user-service 运行时镜像
- **AND** 运行时镜像 MUST NOT 包含 `/usr/local/bin/atlas` 或其他 Atlas CLI 二进制

#### Scenario: Dockerfile 路径约束

- **WHEN** 调整 user-service Dockerfile、migration 相关 Dockerfile 或 COPY 规则
- **THEN** 路径 MUST 继续以仓库根 build context 为基准
- **AND** user-service 运行时镜像 MUST NOT 为了执行 migration 而复制 `user-service/migrations/` 或 Atlas 二进制
- **AND** 仓库 Docker 资产 MUST NOT 暴露默认执行 `atlas migrate apply` 的入口

#### Scenario: 本地 Compose 启动

- **WHEN** 协作者使用 `deployments/compose` 运行本地环境
- **THEN** 系统 MUST 提供 user-service 所需的数据库、缓存和观测服务配置
- **AND** Compose 资产 MUST NOT 自动执行 `atlas migrate apply`

#### Scenario: Compose 启动顺序

- **WHEN** 使用本地 Compose 启动包含 RBAC seed 和 user-service 的环境
- **THEN** RBAC seed MUST 只在目标数据库已完成对应 SQL migration 后执行，RBAC seed MUST 先于 user-service app 启动

#### Scenario: 生产发布顺序

- **WHEN** user-service 发布到生产环境
- **THEN** 运维 MUST 先确认 user-service `user_db` 的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行，再执行 RBAC seed Job，按需显式创建或分配超级管理员，最后启动或滚动更新 HTTP 副本

#### Scenario: 普通容器启动不执行 migration

- **WHEN** user-service 普通运行时容器启动
- **THEN** 容器 MUST 直接启动服务或执行显式传入的 user-service CLI 命令，不得应用 migration
- **AND** `RUN_MIGRATIONS=true` MUST NOT 使普通运行时镜像尝试执行 Atlas migration

#### Scenario: 禁止显式自动 apply 入口

- **WHEN** 简单部署、Compose、Kubernetes、Helm 或发布文档描述 HTTP 服务启动前的数据库准备
- **THEN** 部署流程 MUST 指向已提交 SQL migration 的 DBA 工单或受控发布平台执行结果
- **AND** 仓库资产 MUST NOT 提供可直接运行 `atlas migrate apply` 的 migration Job、service、Helm 默认 command 或 Makefile 目标

### Requirement: 用户服务 Kubernetes 生产清单

系统 MUST 为 user-service 提供云厂商无关的 Kubernetes 生产清单，并覆盖运行副本、服务发现、配置引用、Secret 引用、安全上下文、资源配额、探针、滚动更新、PDB、HPA 和 NetworkPolicy。user-service NetworkPolicy MUST 默认使用显式来源与目的地约束，且准入标签 MUST 由 admission policy 或等价集群准入控制限制使用范围。

#### Scenario: 渲染生产清单

- **WHEN** 协作者查看或应用 `deployments/k8s/user-services/` 下的清单
- **THEN** 清单 MUST 包含 user-service 的 `Deployment`、`Service`、配置资源、Secret 引用、`ServiceAccount`、必要 RBAC、`PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`

#### Scenario: HTTP 副本探针

- **WHEN** user-service Pod 由 Kubernetes Deployment 启动
- **THEN** liveness probe MUST 指向 `GET /livez`，readiness probe MUST 指向 `GET /readyz`，startup probe MUST 指向 `GET /startupz`

#### Scenario: 运行副本不执行 migration

- **WHEN** user-service Deployment 启动普通 HTTP 副本
- **THEN** Pod 环境变量 MUST NOT 设置 `RUN_MIGRATIONS=true`，副本 MUST 只启动 HTTP 服务

#### Scenario: Pod 安全和资源边界

- **WHEN** user-service Pod 被调度
- **THEN** Pod 和容器 securityContext MUST 默认使用非 root、只读根文件系统、禁止特权升级和收敛 Linux capabilities，并且容器 MUST 设置 CPU 与内存 requests/limits

#### Scenario: NetworkPolicy 入站来源约束

- **WHEN** user-service 原生 Kubernetes 生产清单声明 NetworkPolicy ingress
- **THEN** ingress 来源 MUST 使用明确的 `namespaceSelector` 与 `podSelector` 组合约束允许访问 user-service HTTP 端口的上游
- **AND** ingress MUST NOT 使用 `namespaceSelector: {}` 配合单个 Pod 标签作为生产默认来源约束

#### Scenario: NetworkPolicy 出站目的地约束

- **WHEN** user-service 原生 Kubernetes 生产清单声明访问 PostgreSQL、Redis 或 OTLP Collector 的 NetworkPolicy egress
- **THEN** 每类业务依赖 egress 规则 MUST 包含 `to` 目的地约束
- **AND** 目的地 MUST 使用目标 namespace、目标 Pod 标签或精确 `ipBlock` 约束实际 PostgreSQL、Redis 或 OTLP Collector
- **AND** egress MUST NOT 仅按 `5432`、`6379`、`4317` 或 `4318` 端口对任意目的地址放行

#### Scenario: 准入标签治理

- **WHEN** 集群使用标签表达允许访问 user-service 的上游身份
- **THEN** 部署资产 MUST 提供 admission policy 或等价准入治理说明，限制未授权 namespace 或 workload 自行设置该准入标签
- **AND** 未授权租户 MUST NOT 能通过自行添加准入标签获得访问 user-service 的 NetworkPolicy 入站许可

### Requirement: Kubernetes 发布前置作业

系统 MUST 为 user-service Kubernetes 发布提供 RBAC seed Job，并明确数据库 SQL migration 已由 DBA 工单或受控发布平台完成后，才允许执行 RBAC seed 和 HTTP Deployment rollout。仓库 Kubernetes 资产 MUST NOT 提供自动执行 `atlas migrate apply` 的 migration Job。

#### Scenario: 数据库迁移前置确认

- **WHEN** 发布流水线准备发布 user-service 到 Kubernetes 环境
- **THEN** 发布流程 MUST 先确认目标环境已执行本 release 对应的已提交 SQL migration
- **AND** 确认来源 MUST 是 DBA 工单、发布平台记录或等价受控执行记录

#### Scenario: RBAC seed Job

- **WHEN** 数据库 SQL migration 已确认完成后
- **THEN** RBAC seed Job MUST 使用当前发布镜像执行 `rbac seed`，并 MUST 支持 `--reactivate-system` 与 `--sync-system-bindings` 选项

#### Scenario: 发布顺序

- **WHEN** user-service 发布到 Kubernetes 环境
- **THEN** 发布流程 MUST 等待数据库 SQL migration 完成确认，再等待 RBAC seed Job 成功，最后创建或滚动更新 HTTP Deployment

#### Scenario: 前置确认或作业失败

- **WHEN** 数据库 SQL migration 未确认完成或 RBAC seed Job 失败
- **THEN** 发布流程 MUST 停止 HTTP Deployment rollout，并保留可诊断记录

#### Scenario: 镜像版本一致性

- **WHEN** 发布系统设置 user-service 镜像
- **THEN** RBAC seed Job 和 HTTP Deployment MUST 使用同一 release 工件集合或同一 release tag
- **AND** 发布说明 MUST 禁止混用新版 user-service 运行时镜像和旧版 RBAC seed Job 模板

### Requirement: 用户服务 Helm chart

系统 MUST 为 `aegiscore-user-services` 提供 Helm chart，用 values 模板化 Kubernetes 交付能力，并保持默认值不包含真实生产 Secret。chart MUST 支持 RBAC seed Job 和 HTTP Deployment 使用 user-service 镜像，且 MUST NOT 渲染或默认配置自动执行 `atlas migrate apply` 的 migration Job。chart 的默认 NetworkPolicy values MUST 表达显式来源与目的地安全基线。

#### Scenario: Helm chart 元数据和 values

- **WHEN** 协作者查看 `deployments/helm/aegiscore-user-services/`
- **THEN** chart MUST 包含 `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例，并 MUST 暴露 image、service、config、Secret 引用、resources、probes、autoscaling、PDB、NetworkPolicy、RBAC seed Job 和 rollout 配置
- **AND** chart MUST NOT 暴露默认执行 `atlas migrate apply` 的 migration Job 配置

#### Scenario: Helm 渲染 Secret 引用

- **WHEN** 协作者执行 `helm template` 渲染 chart
- **THEN** 模板 MUST 通过 `existingSecret` 或等价 values 引用外部 Secret，不得默认渲染真实敏感值

#### Scenario: Helm 渲染发布作业

- **WHEN** Helm values 启用 RBAC seed Job
- **THEN** chart MUST 渲染 RBAC seed Job
- **AND** RBAC seed Job MUST 使用 user-service 发布镜像执行 `rbac seed`
- **AND** chart MUST NOT 渲染自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: Helm Deployment 默认行为

- **WHEN** Helm chart 渲染 user-service Deployment
- **THEN** Deployment MUST 默认不设置 `RUN_MIGRATIONS=true`，并 MUST 渲染 `/livez`、`/readyz`、`/startupz` 探针和资源 requests/limits
- **AND** Deployment 使用的 user-service 镜像 MUST NOT 依赖 Atlas 二进制或 migration SQL 文件启动 HTTP 服务

#### Scenario: Helm 默认 NetworkPolicy 入站来源

- **WHEN** 协作者使用默认 values 渲染 `aegiscore-user-services` chart
- **THEN** 渲染出的 NetworkPolicy ingress MUST 使用明确的 namespace 与 Pod 选择器约束允许访问 user-service HTTP 端口的来源
- **AND** 默认 values MUST NOT 使用 `namespaceSelector: {}` 配合单个 Pod 标签作为入站来源约束

#### Scenario: Helm 默认 NetworkPolicy 出站目的地

- **WHEN** 协作者使用默认 values 渲染 `aegiscore-user-services` chart
- **THEN** 渲染出的 PostgreSQL、Redis 和 OTLP Collector egress 规则 MUST 分别包含 `to` 目的地约束
- **AND** 默认 values MUST NOT 对任意目的地址开放 `5432`、`6379`、`4317` 或 `4318`

#### Scenario: Helm 环境覆盖外部依赖

- **WHEN** 目标环境使用集群外 PostgreSQL、Redis 或 OTLP Collector
- **THEN** 环境 values MUST 使用精确 `ipBlock` 或等价明确目的地覆盖默认 egress 目的地
- **AND** 环境 values MUST NOT 通过删除 `to` 字段恢复任意目的端口放行

### Requirement: Kubernetes 和 Helm 验证说明

系统 MUST 为 user-service Kubernetes 与 Helm 资产提供可执行的验证说明，覆盖模板渲染、YAML/schema 检查和发布顺序检查。

#### Scenario: 验证原生清单

- **WHEN** 协作者修改 `deployments/k8s/user-services/`
- **THEN** tasks 或 README MUST 指明用于校验 YAML、Kubernetes schema 或 server-side dry-run 的命令
- **AND** 验证说明 MUST 包含检查仓库清单不再提供自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: 验证 Helm chart

- **WHEN** 协作者修改 `deployments/helm/aegiscore-user-services/`
- **THEN** tasks 或 README MUST 指明 `helm lint` 和 `helm template` 的验证命令，并说明如何检查 RBAC seed Job 和 Deployment 的关键字段
- **AND** 验证说明 MUST 包含检查 Helm 渲染结果不再包含自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: 架构文档验证

- **WHEN** 部署规格或 OPSX artifacts 变更完成
- **THEN** 协作者 MUST 执行 `make user-service-architecture-lint`，确保中文文档和 OpenSpec 结构约束通过

### Requirement: common mockgen 交付验证

系统 MUST 为 `common` 模块提供可复现的 mockgen 工具入口、生成命令和 drift 校验。仓库完整验证 MUST 覆盖 `common` 中声明的 mock 生成物，生成物过期、缺失或未提交时 MUST 通过 `git diff --exit-code` 或等价 drift 检查失败。

#### Scenario: common 模块声明 mockgen 工具依赖

- **WHEN** `common` 新增或更新 mock 生成入口
- **THEN** `common` Go module MUST 显式声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口
- **AND** 生成流程 MUST NOT 依赖开发者机器上的隐式全局 `mockgen` 二进制

#### Scenario: common 生成命令覆盖 go generate

- **WHEN** 协作者执行 `make -C common generate` 或根 `make common-generate`
- **THEN** 系统 MUST 执行 `common` 模块内的 `go generate ./...`
- **AND** 该命令 MUST 覆盖 `common/security/casbin` 和 `common/http/middleware` 中声明的 mockgen 入口

#### Scenario: common verify 暴露生成物 drift

- **WHEN** 协作者执行 `make common-verify` 或 `make -C common verify`
- **THEN** 系统 MUST 运行 common lint、common 生成和 common 测试
- **AND** 系统 MUST 通过 drift 检查暴露 common 生成物缺失、过期或未提交

#### Scenario: 完整 verify 覆盖 common 生成物

- **WHEN** 协作者执行根 `make verify`
- **THEN** 系统 MUST 在完整验证链路中执行 common 生成命令
- **AND** 最终 `git diff --exit-code` MUST 能暴露 common mock 生成物 drift 或未纳入暂存区的意外变更

### Requirement: user-service 运行时装配测试断言迁移

`user-service/internal/bootstrap` 与 `user-service/internal/providers` 中覆盖 Fx provider、bootstrap validation、PostgreSQL/Redis/Ent provider、Gin engine、routes provider 和 HTTP server lifecycle 的测试 MUST 使用 `docs/TESTING.md` 规定的语义化断言。断言迁移 MUST 保持 Fx 依赖图、provider 输出、生命周期 hook、server start/stop、graceful shutdown、forced close、drain tracker 和配置默认值语义不变。

#### Scenario: Fx provider 和 bootstrap validation 断言

- **WHEN** provider 或 bootstrap 测试验证 `fx.ValidateApp`、named resource、provider 输出、配置默认值、lifecycle hook 数量、启动日志或关闭顺序
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotNil`、`require.Len`、`require.Equal`、`require.ElementsMatch` 或等价语义化断言
- **AND** 多个互相独立的 provider 输出或日志字段 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 改变 Fx module、provider、invoke、named resource 或 bootstrap validation 生产行为

#### Scenario: HTTP server lifecycle 断言

- **WHEN** bootstrap 测试验证 listener bind 失败、Serve 错误、Shutdown 错误、lifecycle context cancellation、active handler drain、forced close、default shutdown timeout 或 drain tracker wait 行为
- **THEN** 测试 MUST 使用语义化断言表达错误、错误包含关系、调用次数、耗时边界、日志字段和 server timeout 配置
- **AND** channel handoff、blocked handler、goroutine 退出等待或跨 goroutine 错误传递等测试控制流 MAY 保留符合 `docs/TESTING.md` 例外规则的直接 `testing.T` 失败调用
- **AND** 迁移 MUST NOT 改变 HTTP server start/stop、graceful shutdown、forced close、drain tracker 或 Fx lifecycle 语义

#### Scenario: 残留失败调用受扫描约束

- **WHEN** 目标范围 `_test.go` 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于并发协调、特殊诊断输出、测试辅助工具边界或无法通过现有语义化断言清晰表达的控制流
- **AND** change tasks MUST 列明剩余例外及原因

### Requirement: 运行时装配断言迁移不扩大交付范围

断言迁移 MUST 只覆盖 issue 指定的 router、providers 和 bootstrap 测试路径。系统 MUST NOT 将本 change 扩展为 feature、cmd、Ent schema、e2e、common、部署资产或 OpenAPI 生成物迁移。

#### Scenario: 实施范围受限

- **WHEN** 实施本 change
- **THEN** 代码修改 MUST 限定在 `user-service/internal/router/**/*_test.go`、`user-service/internal/providers/**/*_test.go`、`user-service/internal/bootstrap/**/*_test.go` 和本 change 的 OpenSpec artifacts
- **AND** change MUST NOT 修改生产 Go 文件、Ent schema、Atlas migration、OpenAPI 生成物、部署清单或 `common` 测试

### Requirement: user-service CLI 与 Ent schema 测试断言迁移

`user-service/cmd` 与 `user-service/ent/schema` 中覆盖 CLI command、flag/env normalization、cleanup error、Ent schema field、edge、index、annotation、default、validator 和 mixin 的测试 MUST 使用 `docs/TESTING.md` 规定的语义化断言。断言迁移 MUST 保持服务前缀 Make target、CLI command graph、命令帮助输出约束、Ent schema 定义、Atlas migration 和生成物交付流程不变。

#### Scenario: CLI command 断言

- **WHEN** cmd 测试验证 root command、serve command、RBAC command、flag 绑定、env normalization、command output、usage 文本、cleanup error 或执行错误
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotNil`、`require.Len`、`require.Equal`、`require.Contains`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 command property MAY 使用 `assert`
- **AND** 迁移 MUST NOT 新增旧 root command alias、旧 flag/env 名、旧 usage 文本或无服务前缀 Make target 兼容断言

#### Scenario: Ent schema 断言

- **WHEN** Ent schema 测试验证 field 数量、field 名称、类型、唯一性、可选性、默认值、validator、edge、index、annotation、mixin 或 schema comment
- **THEN** 测试 MUST 使用 `require.Len`、`require.Equal`、`require.NotNil`、`require.Empty`、`require.NotEmpty`、`require.ElementsMatch`、`require.Contains`、`require.Greater`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 field、edge、index 或 annotation 检查 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 修改 Ent schema、Ent 生成代码、Atlas migration 或 schema 运行时行为

#### Scenario: 残留失败调用受扫描约束

- **WHEN** 目标范围 `_test.go` 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于特殊测试控制流、特殊诊断输出、测试辅助工具边界或无法通过现有语义化断言清晰表达的控制流
- **AND** change tasks MUST 列明剩余例外及原因

### Requirement: cmd 与 Ent schema 断言迁移不扩大交付范围

断言迁移 MUST 只覆盖 issue 指定的 cmd 与 Ent schema 测试路径。系统 MUST NOT 将本 change 扩展为 router/provider/bootstrap、feature、e2e、common、部署资产、OpenAPI 生成物或数据库结构变更。

#### Scenario: 实施范围受限

- **WHEN** 实施本 change
- **THEN** 代码修改 MUST 限定在 `user-service/cmd/**/*_test.go`、`user-service/ent/schema/**/*_test.go` 和本 change 的 OpenSpec artifacts
- **AND** change MUST NOT 修改生产 Go 文件、Ent schema、Ent 生成代码、Atlas migration、OpenAPI 生成物、部署清单或 `common` 测试

### Requirement: 聚合路由测试断言与覆盖率验收
系统 MUST 为 user-service 聚合路由注册补充符合交付断言规范的 Go 测试，并通过覆盖率验证确保 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均被执行。

#### Scenario: 语义化断言
- **WHEN** 新增或修改 `user-service/internal/router` 路由注册测试
- **THEN** 测试 MUST 优先使用 `require` 的语义化断言表达错误、集合、字符串、长度和包含关系
- **AND** 存在 `Len`、`Contains`、`ElementsMatch`、`ErrorContains`、`Regexp` 或等价更具体断言时，测试 MUST NOT 使用 `True` 或 `False` 包装布尔表达式
- **AND** 只有多个互相独立的 route 条目需要一次性收集失败且后续检查不依赖前置结果时，测试 MAY 使用 `assert`

#### Scenario: router 覆盖率验收
- **WHEN** 本 change 实施完成
- **THEN** `go test -cover ./user-service/internal/router` MUST 通过
- **AND** `go tool cover -func` MUST 显示 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均有覆盖
- **AND** `openspec validate cover-user-service-route-registration-no-compat` MUST 通过

### Requirement: CLI 命令测试语义化断言

`delivery-operations` 的 user-service 命令测试 MUST 使用语义化断言表达 CLI 错误、参数缺失、依赖初始化、cleanup 合并和命令属性检查。测试 MUST 优先使用 `require` fail-fast 断言；只有互相独立且不影响后续测试前置条件的命令 property 检查 MAY 使用 `assert`。

#### Scenario: 命令错误使用 fail-fast 断言

- **WHEN** CLI 测试验证参数缺失、配置错误、依赖初始化错误、命令执行错误或 cleanup 错误
- **THEN** 测试 MUST 使用 `require.Error`、`require.ErrorContains`、`require.ErrorIs`、`require.ErrorAs` 或等价 fail-fast 断言
- **AND** 测试 MUST NOT 使用 `require.True`、`assert.True` 或手写 `if` 拼装错误断言替代更具体的错误断言

#### Scenario: 后续检查依赖当前命令结果

- **WHEN** 后续断言需要依赖命令执行成功、初始化成功、返回对象非空或 error 类型匹配
- **THEN** 测试 MUST 使用 `require` 断言建立前置条件
- **AND** 失败后 MUST 停止当前测试，避免继续读取无效结果

#### Scenario: 独立命令属性允许 assert

- **WHEN** 多个命令 flag 默认值、短描述、Use 字符串或互相独立的布尔属性彼此不构成前置依赖
- **THEN** 测试 MAY 使用 `assert` 聚合这些独立属性检查
- **AND** 若存在 `Len`、`Contains`、`ElementsMatch`、`Regexp`、`Greater`、`LessOrEqual` 等更具体断言，测试 MUST 优先使用具体断言

#### Scenario: 禁止机械 failure helper 替换

- **WHEN** 新增或修改 user-service 命令测试
- **THEN** 测试 MUST NOT 使用机械 `Fail`、`Failf`、`FailNow`、`FailNowf` 或旧手写断言兼容 helper 表达常见断言
- **AND** `t.Fatal`、`t.Fatalf`、`t.Error` 和 `t.Errorf` 只允许出现在 `docs/TESTING.md` 明确允许的边界内

### Requirement: user-service E2E 测试断言迁移
系统 MUST 将 `user-service/tests` 下的 E2E HTTP flow、migration validation 和测试 harness Go 测试迁移到 `docs/TESTING.md` 固化的统一断言规范。断言迁移 MUST 保持 E2E 流程、测试数据构造、Testcontainers 前置条件、migration 应用顺序和生产行为不变，并 MUST NOT 引入旧断言兼容 helper、旧 API 响应兼容断言或机械 `Fail*` 替换。

#### Scenario: 语义化断言优先
- **WHEN** E2E 测试断言 HTTP status、错误、响应 envelope、集合长度、无序集合、JSON 响应、时间相关结果、文件读取、SQL 执行或对象字段
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotEmpty`、`require.Empty`、`require.Len`、`require.Greater`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration` 或等价语义化断言
- **AND** 存在更具体语义化断言时，测试 MUST NOT 使用 `True`、`False`、手写 if 或多个基础断言拼凑同一检查

#### Scenario: 完整 HTTP flow 独立字段收集
- **WHEN** 完整 HTTP flow 的单个响应包含多个互相独立的字段检查，且后续检查不依赖这些字段全部成功
- **THEN** 测试 MAY 使用 `testify/assert` 收集独立失败
- **AND** 初始化失败、容器或配置前置条件、JSON 解码、数据库连接、migration 应用和后续流程依赖的结果 MUST 使用 `require` 立即终止当前测试

#### Scenario: migration validation 断言迁移
- **WHEN** migration harness 枚举 SQL migration、读取文件、拆分 SQL statement、定位 user-service 根目录或逐条执行 migration
- **THEN** 测试 MUST 使用语义化断言表达错误、空集合、执行失败和路径定位失败
- **AND** 迁移 MUST NOT 改变 SQL parser 对注释、单引号、双引号、dollar quote、statement 分隔和错误返回的处理语义

#### Scenario: 残留失败调用受扫描约束
- **WHEN** 实施完成后扫描 `user-service/tests/**/*_test.go` 中的 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的特殊测试控制流、特殊诊断输出、测试辅助工具边界，或验收正则对 `fmt.Errorf` 的 false positive
- **AND** change tasks MUST 列明每个剩余命中及原因

#### Scenario: E2E 断言迁移验证
- **WHEN** 本 change 实施完成
- **THEN** `rg "github.com/stretchr/testify/(require|assert)" user-service/tests --glob "*_test.go"` MUST 定位到迁移后的实际使用点
- **AND** `go test ./user-service/tests/...` MUST 在具备 E2E 容器前置条件时通过
- **AND** 若容器前置条件不可用，tasks MUST 明确记录 `AEGISCORE_TEST_E2E=1` 或通用容器测试开关、Docker 或兼容容器运行时等可运行前置条件和已完成替代验证
- **AND** `openspec validate standardize-e2e-test-assertions-no-compat` MUST 通过

### Requirement: 仓库级工具测试断言验证
仓库级工具测试 MUST 遵循统一 Go 测试断言规范。OpenAPI 转换、CLI 工具输入输出、文件生成和交付验证相关工具测试 MUST 优先使用 `testify/require` 的语义化断言；存在更具体的 `Len`、`ErrorContains`、`Contains`、`ElementsMatch`、`JSONEq`、`Regexp` 等断言时，测试 MUST NOT 使用 `True` / `False` 或手写 `if` 拼装同等检查。

#### Scenario: 迁移工具测试断言
- **WHEN** `tools/**/*_test.go` 或仓库级工具测试断言错误、文件内容、JSON/YAML 输出、集合长度、字符串匹配或生成物路径
- **THEN** 测试 MUST 使用 `require.NoError`、`require.ErrorContains`、`require.Len`、`require.Contains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp` 或等价语义化断言表达检查
- **AND** 测试 MUST NOT 使用手写 `t.Fatalf` / `t.Errorf` 或 `require.True` / `require.False` 包装可由专属断言表达的检查

#### Scenario: 工具测试包为空
- **WHEN** 当前仓库级 `tools` 范围没有 Go 测试包或没有 `_test.go` 文件
- **THEN** change tasks MUST 记录实际包列表、扫描结果和替代验证命令
- **AND** 系统 MUST NOT 为了满足迁移任务而新增旧工具输出格式、旧 CLI flag 或旧文件路径兼容断言

#### Scenario: 多个独立工具输出差异
- **WHEN** 单个工具测试需要在一次执行中检查多个独立输出字段、文件内容差异或生成路径差异，且后续检查不依赖前置检查成功
- **THEN** 测试 MAY 使用 `testify/assert` 收集这些独立断言失败
- **AND** 初始化、命令执行、文件读取或解析失败 MUST 使用 `require` 立即终止当前测试

### Requirement: Go 测试按维护主题组织

认证与 provider 相关 Go 测试 MUST 按可独立维护的行为主题组织文件，避免单个测试文件长期承载多个不相关子主题。测试拆分 MUST 保持原有业务断言覆盖，不得通过删除关键场景降低覆盖范围。

#### Scenario: auth Redis session store 测试拆分

- **WHEN** 协作者维护 `user-service/internal/features/auth/infrastructure/redis` 的 session store 测试
- **THEN** token version cache、token version validator、refresh session 创建查询删除、refresh session rotation、全量 session 删除、purge pool/Fx lifecycle 和 Redis key schema 测试 MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 旧的跨主题大型 `session_store_test.go` MUST NOT 继续承载这些全部场景

#### Scenario: auth command use case 测试拆分

- **WHEN** 协作者维护 `user-service/internal/features/auth/application/command` 的 command use case 测试
- **THEN** login、change-password、refresh、logout current、logout all 和共享构造 helper MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 旧的跨 use case 大型 `service_test.go` MUST NOT 继续承载这些全部场景

#### Scenario: provider routes 与 Gin engine 测试拆分

- **WHEN** 协作者维护 `user-service/internal/providers` 的 routes 或 Gin engine 测试
- **THEN** auth middleware、route 注册冲突、tracing、request ID、HTTP metrics、panic recovery 和 runtime endpoint skip 测试 MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 单个 provider 测试文件 MUST NOT 同时承载所有 route、metrics、tracing 和 panic 场景

#### Scenario: 拆分后保持测试集合完整

- **WHEN** 大型测试文件被拆分
- **THEN** 协作者 MUST 对比拆分前后的 `Test` 函数集合或等价测试清单
- **AND** 目标包 `go test` MUST 通过

### Requirement: 复杂测试替身使用生成 mock

Go 测试中表示外部 collaborator port 调用契约的复杂 fake、stub 或 spy MUST 使用包内 `mockgen` 生成物替代。仅用于构造领域值、提供无行为分支统计快照、真实 miniredis/localcache 夹具或简单不可变配置的测试 helper MAY 保留在 `_test.go` 文件内。

#### Scenario: collaborator 调用契约使用 mockgen

- **WHEN** 测试需要断言 credential store、token issuer、refresh session store、token version store/cache、RBAC seed service、authorizer、watcher 或 metrics collaborator 的调用、参数、顺序或失败路径
- **THEN** 测试 MUST 使用同包或同 feature 测试包内的 `go.uber.org/mock/mockgen` 生成 mock 设置 expectation
- **AND** 测试 MUST NOT 通过复杂手写 fake/stub/spy 字段隐藏这些 collaborator 调用契约

#### Scenario: mock 生成入口归属

- **WHEN** 新增或替换测试 collaborator mock
- **THEN** `mock_generate.go` MUST 位于消费该 mock 的包或 feature-local 测试边界内
- **AND** 生成 mock MUST NOT 放入全局 `mocks/` 包或跨 feature 共享 mock 包
- **AND** 生成入口 MUST 使用可复现的 `go tool mockgen` 或仓库约定等价入口

#### Scenario: 允许轻量测试 helper

- **WHEN** 测试 helper 只构造领域对象、返回固定 stats、运行真实 workerpool task、包装 miniredis、包装真实 localcache 或提供无外部调用契约的配置值
- **THEN** helper MAY 保留为 `_test.go` 内部类型或函数
- **AND** helper MUST NOT 替代 mockgen 记录外部 port 调用、失败注入或调用顺序

### Requirement: Metrics no-op 生成约定一致

feature-local 业务 metrics interface 的 no-op 实现 MUST 继续通过业务中立生成器或统一生成约定维护。`common/runtime/observability/metrics` MUST 只承载生成器和通用 runtime metrics 能力，不得承载 user-service feature 的业务 metrics 方法。

#### Scenario: feature-local no-op 生成

- **WHEN** auth、permission 或其他 feature 定义 `Metrics` interface 且需要默认空实现
- **THEN** feature MUST 通过统一的 `nopgen` 生成约定生成 `metrics_nop_gen.go` 或等价 no-op 生成物
- **AND** no-op 生成物 MUST 与 feature-local `Metrics` interface 编译匹配

#### Scenario: common 不承载业务指标方法

- **WHEN** 统一 metrics no-op 生成约定被调整
- **THEN** `common/runtime/observability/metrics` MUST NOT 定义 auth 登录、refresh、logout、session purge、RBAC policy reload、watcher 或 route diff 等 user-service 业务指标方法
- **AND** auth/permission 业务指标方法 MUST 保留在所属 feature 边界内

#### Scenario: 生成物 drift 可验证

- **WHEN** metrics interface 或 mock 源 interface 变化
- **THEN** 仓库生成与完整验证流程 MUST 能更新对应生成物
- **AND** 未同步生成物 MUST 通过 `git diff --exit-code` 或等价 drift 检查暴露

### Requirement: 仓库级 OpenAPI 转换工具测试验证

系统 MUST 为 `tools/openapi-convert` 提供默认可执行的 Go 测试，覆盖 CLI 参数解析、错误路径和文件生成结果。根 `make test` 和 `make verify` MUST 执行该工具模块测试，工具模块测试失败时完整验证 MUST 失败。

#### Scenario: 根测试覆盖 OpenAPI 转换工具
- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 执行 `tools/openapi-convert` 模块的 Go 测试
- **AND** 系统 MUST 同时保持 `common` 和 `user-service` 模块测试执行

#### Scenario: 完整验证覆盖 OpenAPI 转换工具
- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 通过测试阶段执行 `tools/openapi-convert` 模块测试
- **AND** 工具模块测试失败 MUST 阻止 `make verify` 成功完成

#### Scenario: CLI 参数错误回归测试
- **WHEN** `tools/openapi-convert` 测试覆盖缺少必填 `input`、`json`、`yaml` 或 `go` 输出路径的调用
- **THEN** 测试 MUST 断言 CLI 返回失败结果并输出明确错误

#### Scenario: root path 参数约束回归测试
- **WHEN** `tools/openapi-convert` 调用设置 `root-path` 但未设置 `root-server`
- **THEN** CLI MUST 返回失败结果
- **AND** 测试 MUST 断言该约束错误被保留

#### Scenario: 文件生成回归测试
- **WHEN** `tools/openapi-convert` 使用合法 Swagger 2 输入和输出路径执行
- **THEN** 测试 MUST 断言 JSON、YAML 和 Go embed 输出文件被创建
- **AND** 测试 MUST 断言生成内容包含 OpenAPI 版本、路径或 Go package 等关键结构

#### Scenario: 输入输出错误回归测试
- **WHEN** `tools/openapi-convert` 收到不存在的输入文件或不可写输出目标
- **THEN** CLI MUST 返回失败结果
- **AND** 测试 MUST 断言错误信息能定位输入转换或输出写入阶段

### Requirement: user-service Swagger UI 依赖升级验证

系统 MUST 将 user-service 的 Swagger UI 静态资源依赖维护在 `github.com/swaggo/files/v2`，并通过交付验证确认 v2 模块路径、embedded `fs.FS` 运行时路由和 OpenAPI 生成链路一致。升级 MUST NOT 保留 `github.com/swaggo/files` v1 依赖、`github.com/swaggo/gin-swagger` wrapper、旧 import、旧 handler fallback 或双版本兼容代码。

#### Scenario: 依赖使用 v2 模块路径

- **WHEN** 协作者审查 `user-service/go.mod`
- **THEN** `user-service` MUST 显式依赖 `github.com/swaggo/files/v2`
- **AND** `user-service` MUST NOT 继续依赖 `github.com/swaggo/files` v1 模块路径

#### Scenario: 编译和测试覆盖升级

- **WHEN** 协作者完成 Swagger UI v2 升级实现
- **THEN** `go test ./user-service/internal/router` MUST 通过
- **AND** 测试 MUST 验证 OpenAPI JSON、OpenAPI UI 或 docs redirect 的当前稳定行为

#### Scenario: OpenAPI 生成链路保持可验证

- **WHEN** 协作者执行 `make user-service-openapi-generate`
- **THEN** 系统 MUST 继续生成 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`
- **AND** 生成链路 MUST NOT 因 Swagger UI v2 依赖升级改变 `tools/openapi-convert` CLI 参数、服务脚本传入参数或输出文件集合

#### Scenario: 完整验证暴露依赖或生成物 drift

- **WHEN** 协作者准备完成本 change
- **THEN** 协作者 MUST 先暂存本次预期代码、OpenSpec artifacts 和必要生成物变更
- **AND** `make lint` 和 `make verify` MUST 通过
- **AND** `make verify` 的最终 `git diff --exit-code` MUST 能暴露未纳入暂存区的依赖、代码或生成物 drift

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
- **THEN** harness 生成的配置 MUST 满足当前严格配置契约，包括 `server.http`、具名 resources、feature cache、metrics 和 tracing
- **AND** E2E MUST 启动真实 PostgreSQL 与 Redis、应用已提交 SQL migration 并执行认证和用户 HTTP flow

#### Scenario: 轻量测试 job 不重复真实容器负载

- **WHEN** CI 执行 verify、race 或 coverage job
- **THEN** 这些 job MAY 保持默认容器测试关闭状态
- **AND** 阻塞式 Docker-backed test job MUST 成为真实依赖通过状态的唯一权威门禁

### Requirement: user-service 根命令测试局部依赖注入

`delivery-operations` 的 user-service 根命令构造 MUST 通过命令实例本地依赖注入支持测试替身，正式代码 MUST NOT 为 `serve` lifecycle app factory 或 RBAC runner 暴露 package-level 可变函数变量。该约束 MUST 保持现有 CLI command graph、flag 默认值、配置路径传递和 `serve` lifecycle shutdown 语义不变。

#### Scenario: serve 生命周期测试使用局部 factory

- **WHEN** `user-service/cmd` 测试覆盖 `serve` 启动和停止 lifecycle
- **THEN** 测试 MUST 在当前命令或函数调用范围内传入 lifecycle app factory 替身
- **AND** 测试 MUST NOT 通过赋值 package-level `newLifecycleApp` 或等价可变全局函数变量注入替身

#### Scenario: 根命令 surface 测试不共享全局替身

- **WHEN** `user-service/cmd` 测试构造 root command 并检查 `serve`、`rbac` 或 `fxgraph` command surface
- **THEN** 每个测试 MUST 使用本地构造的命令依赖或默认依赖
- **AND** 测试 MUST NOT 依赖保存、覆盖和恢复 package-level runner 变量来避免执行真实命令

#### Scenario: CLI 外部行为保持不变

- **WHEN** 运维执行 `aegiscore-user-services serve` 或查看 root command 帮助
- **THEN** 系统 MUST 保持现有 command 名称、flag 名称、flag 默认值、配置路径默认值、输出语义和退出码
- **AND** 本次依赖注入重构 MUST NOT 修改 Makefile 目标、OpenAPI 生成物、Ent schema、Atlas migration 或部署资产

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

### Requirement: user-service CLI 统一协调外部与内部退出

`aegiscore-user-services serve` MUST 在 Fx App 成功启动后同时消费外部 context 取消和 `App.Wait()` 返回的 `fx.ShutdownSignal`。任一退出来源就绪后，命令 MUST 使用配置化进程停止预算调用且仅调用一次 `App.Stop()`，并把内部失败或停止失败转换为 Cobra error；命令内部 MUST NOT 调用 `os.Exit`。

#### Scenario: 外部终止信号正常退出

- **WHEN** `SIGINT`、`SIGTERM` 或上游 context 取消触发 serve 命令退出，且 `App.Stop()` 成功
- **THEN** 命令 MUST 使用未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 预算调用一次 `App.Stop()`
- **AND** 命令 MUST 正常返回且不产生非零 Cobra error

#### Scenario: 内部零 exit code 请求关闭

- **WHEN** `App.Wait()` 返回 exit code 为 `0` 的内部 `fx.ShutdownSignal`，且 `App.Stop()` 成功
- **THEN** 命令 MUST 立即调用一次带预算的 `App.Stop()`
- **AND** 命令 MUST 正常返回

#### Scenario: 内部非零 exit code 请求关闭

- **WHEN** `App.Wait()` 返回非零 exit code 的 `fx.ShutdownSignal`
- **THEN** 命令 MUST 在一次带预算的 `App.Stop()` 完成后返回包含该 exit code 的 Cobra error
- **AND** 现有 main 入口 MUST 将该 error 转换为非零进程退出码

#### Scenario: App Stop 失败

- **WHEN** 任一退出来源触发停止且 `App.Stop()` 返回错误
- **THEN** 命令 MUST 返回可诊断的 Cobra error
- **AND** 若内部 shutdown signal 同时携带非零 exit code，返回错误 MUST 同时保留内部 exit code 与 Stop error，不能以其中一项覆盖另一项

#### Scenario: 多个退出来源并发竞争

- **WHEN** 外部 context 取消与一个或多个内部 shutdown signal 并发到达
- **THEN** 命令 MUST 只调用一次 `App.Stop()`
- **AND** 命令 MUST 在停止预算内完成或返回停止错误，不得重复 Stop 或死锁

#### Scenario: 保持手动生命周期可测试性

- **WHEN** 命令层测试构造 serve App 替身
- **THEN** 最小 App 接口 MUST 只暴露 `Start`、`Wait` 和 `Stop` 所需生命周期能力
- **AND** 测试 MUST 继续通过命令实例或函数调用范围内的局部 factory 注入替身，不得引入 package-level 可变测试 hook

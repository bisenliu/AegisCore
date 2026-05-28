## Context

仓库是 Go workspace，包含 `common` 和 `user-services` 两个模块。当前版本基线不一致：根目录 `go.work` 声明 `go 1.24` 和 `toolchain go1.24.1`，`user-services/go.mod` 声明 `go 1.24` 和 `toolchain go1.24.1`，`common/go.mod` 声明 `go 1.23`。这会让不同开发者或自动化环境在依赖解析、语言版本语义和测试时使用不同 Go 版本。

本变更影响 workspace 与模块元数据，以及开发文档中的工具链说明。它不改变 controller/service/repository 分层，不触碰 `user-services/ent/` 生成代码，不改变 Redis/PostgreSQL 初始化、HTTP 响应契约、错误映射或 API 路由。

## Goals / Non-Goals

**Goals:**
- 将 `go.work`、`common/go.mod` 和 `user-services/go.mod` 的 `go` 版本统一为 `1.26`。
- 将显式 `toolchain` 指令更新为 Go 1.26 最新可用补丁版本，并避免 workspace 与模块之间出现冲突声明。
- 更新 `docs/DEVELOPMENT.md` 的前置条件，使开发说明与实际工具链一致。
- 在升级后分别验证 `common` 与 `user-services` 模块的 `go test ./...`。

**Non-Goals:**
- 不引入新的业务 API、认证、支付、健康检查聚合或用户资料写操作能力。
- 不升级业务依赖版本，除非 Go 1.26 兼容性验证明确要求。
- 不修改 Ent schema，不重新生成 `user-services/ent/` 代码。
- 不改变 YAML 配置结构、`AEGISCORE_` 环境变量覆盖规则或响应信封格式。

## Decisions

- 以 `go.work` 作为 workspace 级工具链事实来源，同时同步两个模块的 `go` 版本。替代方案是只修改 `go.work`，但模块在脱离 workspace 时仍会暴露旧版本要求，不能解决版本基线不一致问题。
- 保留显式 `toolchain` 指令并升级到 Go 1.26 最新补丁版本。替代方案是移除 `toolchain`，但这会让本地环境更依赖开发者已安装版本，降低可重复性。
- 将工具链基线建模为新的 `go-toolchain-baseline` capability。替代方案是把该要求并入 `shared-infrastructure` 或 `http-service-runtime`，但 Go 版本基线横跨模块和开发流程，不属于运行时基础设施或 HTTP 服务行为。
- 验证范围限定为两个 Go 模块的测试。替代方案是启动完整 HTTP 服务，但本地运行依赖 Redis 和 PostgreSQL；工具链升级不需要通过外部依赖启动验证来证明 API 行为未变。

## Risks / Trade-offs

- Go 1.26 未安装或补丁版本不可用 → 在实现时先确认本机可用的 Go 1.26 最新补丁版本，再写入 `toolchain` 并运行测试。
- 间接依赖与 Go 1.26 编译器存在兼容性问题 → 先运行 `go test ./...` 定位失败；仅在失败与工具链兼容性直接相关时升级最小必要依赖。
- CI 或部署环境仍固定旧 Go 版本 → 需要同步更新相关环境配置；当前仓库未发现 `.github/workflows`，实现时仍应搜索其他 CI 配置文件。
- `toolchain` 同时出现在 workspace 和模块时可能产生维护成本 → 保持声明一致，并在文档中明确 Go 1.26 为统一基线。

## Migration Plan

1. 确认 Go 1.26 最新可用补丁版本，确定 `toolchain go1.26.x` 的精确值。
2. 更新 `go.work`、`common/go.mod`、`user-services/go.mod` 的 Go 版本声明和必要的 toolchain 声明。
3. 更新 `docs/DEVELOPMENT.md` 的 Go 前置条件。
4. 分别在 `common/` 和 `user-services/` 运行 `go test ./...`。
5. 如测试失败，优先修复与 Go 1.26 兼容性直接相关的问题；避免顺带重构业务代码。

回滚策略是将 Go 版本和 toolchain 声明恢复到变更前版本，并恢复开发文档中的旧基线说明。

## Open Questions

- 实现时需要以可用安装源确认 Go 1.26 的最新补丁版本，例如 `go1.26.0`、`go1.26.1` 或更高补丁版本。

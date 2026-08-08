## MODIFIED Requirements

### Requirement: 质量门禁、架构诊断与可复现生成

系统 MUST 提供模块级和仓库级测试、lint、架构检查、生成和完整 verify 入口。测试与生成 MUST 可诊断、可复现且不得扩张正式生产 API。`common`、`user-service`、`tools/openapi-convert` 和 `tools/nacos-config-seed` 四个 Go module MUST 进入仓库级 lint、`govulncheck` 和 `gosec` 门禁，并在完整仓库 checkout 中保持可独立校验的 module metadata。`user-service-architecture-lint` MUST 保护正式架构来源声明的边界；Fx 依赖图诊断 MUST 无外部及运行时激活副作用。OpenAPI 转换库、CLI 和服务脚本 MUST 分别由 `common/http/openapi`、`tools/openapi-convert` 和 user-service 拥有其通用逻辑、可执行入口与服务参数。GitHub Actions MUST 由主 CI workflow 唯一拥有 PR 与主线 push 的标准质量触发，并通过仅支持 `workflow_call` 的复用 workflow 为同一 commit 各执行一次 lint 和普通单测。

#### Scenario: Nacos source adapter 测试与示例门禁

- **WHEN** `common/runtime/config/nacos` package 的 adapter 行为或文件组织变化
- **THEN** 验证 MUST 覆盖 Nacos package 普通测试、race 测试、`go vet` 和 examples
- **AND** 测试 MUST 使用本地 fake loader 或 `httptest.Server` 固化多 dataId 加载、server failover、总 timeout 分配、认证 token 复用、endpoint 拼接、响应体上限和 safe error message 行为
- **AND** 这些验证 MUST NOT 连接真实 Nacos、启动 Compose、读取部署配置目录、要求 Nacos SDK 或新增动态刷新后台任务

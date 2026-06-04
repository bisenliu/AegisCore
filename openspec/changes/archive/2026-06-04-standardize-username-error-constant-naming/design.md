## Context

`user-services/internal/errmsg/messages.go` 定义用户服务公开错误文案常量，其中用户名为空的错误常量当前命名为 `MsgInvalidUserName`。项目其他用户领域代码普遍使用 `Username` 作为单词边界，包括 DTO 字段、domain 字段、repository 方法、service 参数、Ent 生成方法和测试变量。该变更属于 `project-naming-consistency` 范围内的内部 Go 符号命名修正。

当前可见引用点位于用户服务校验逻辑和测试中，公开文案仍是 `用户名不能为空`。因此实现应只改变 Go identifier 与引用，不改变外部 API 行为。

## Goals / Non-Goals

**Goals:**

- 将 `MsgInvalidUserName` 统一为 `MsgInvalidUsername`。
- 全仓库搜索 `UserName`、`userName` 等类似拆词式内部 Go 符号，确认是否存在其他可低风险同步修改的候选。
- 同步更新所有引用点，并运行相关 Go 测试验证重命名没有破坏编译或行为。
- 保持错误消息文本、响应码、HTTP 状态码和 JSON 字段完全兼容。

**Non-Goals:**

- 不修改 `username` JSON 字段、查询参数、数据库字段、Ent schema 或 migration。
- 不重命名 `GetByUsername`、`Username`、`gotUsername` 等已符合 `Username` 风格的符号。
- 不引入新的错误消息包、常量治理结构或跨模块公共 API。
- 不修改 `user-services/ent/` 下生成代码。

## Decisions

- 采用直接内部符号重命名，而不是保留旧常量别名。该常量位于 `user-services/internal` 包内，不是外部 Go API；直接重命名可以避免项目继续出现两套命名。
- 使用仓库级文本搜索确认候选，而不是只修改已知文件。这样能覆盖测试、文档或其他服务内符号，并符合命名一致性审查要求。
- 只修改低风险内部 Go identifier，不修改字符串值。错误文案是外部可观察响应内容，当前需求只要求统一领域概念命名，保持文案不变可避免行为回归。
- 对生成代码和外部契约保持只读。Ent 生成方法 `SetUsername`、`OldUsername` 等已经符合命名风格，无需改动；`username` 字段名属于外部和持久化契约，应保持小写字符串。

## Risks / Trade-offs

- 误改外部契约字符串 -> 通过限定修改范围为 Go identifier 并运行测试缓解。
- 遗漏跨包引用导致编译失败 -> 通过全仓库搜索和 `go test ./...` 在相关模块验证缓解。
- 搜索结果包含生成代码或符合规范的 `Username` 符号 -> 只处理 `UserName`、`userName` 拆词式候选，不因搜索数量多而扩大重构范围。
- 直接移除旧常量别名可能影响未提交的并行改动 -> 该符号是 internal 包内实现细节；如编译发现新引用，按同一命名规则同步更新。

## Migration Plan

1. 在仓库内搜索 `MsgInvalidUserName`、`UserName` 和 `userName`。
2. 将确认的低风险内部 Go 符号改为 `Username` 风格，并同步引用。
3. 运行 `gofmt` 处理被修改的 Go 文件。
4. 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，或至少运行受影响的用户服务测试。

回滚策略：如发现不可接受风险，恢复本次内部符号重命名即可；无需数据库、配置或迁移回滚。

## Open Questions

无。

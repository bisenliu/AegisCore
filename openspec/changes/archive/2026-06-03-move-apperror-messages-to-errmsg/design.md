## Context

`user-services/internal/apperror/messages.go` 当前仅定义用户服务内复用的中文错误消息常量。该包名会暗示其中包含应用错误类型、错误码或响应映射逻辑，但这些职责实际由 `common/response` 及 controller/service 调用点承担。

本变更只调整 user-services 内部包命名与引用关系，不改变 controller/service/repository 分层，不触碰 `common/response` 响应信封，不涉及 Ent schema、Atlas migration、Redis/PostgreSQL 配置或 HTTP server 启动依赖。

## Goals / Non-Goals

**Goals:**

- 将消息常量迁移到 `user-services/internal/errmsg/messages.go`，让包名表达“错误消息文本”职责。
- 删除 `user-services/internal/apperror` 包，避免与应用错误模型混淆。
- 更新所有 user-services 内引用，保证编译通过且对外响应内容保持一致。

**Non-Goals:**

- 不新增或修改 HTTP API、路由、中间件或响应信封字段。
- 不修改业务错误码、HTTP 状态码或现有中文错误文案。
- 不调整 `common/response` 的应用错误模型。
- 不修改数据库 schema、生成 Ent 代码或 Atlas migration。

## Decisions

- 使用 `errmsg` 作为新包名，而不是继续扩展 `apperror`。
  理由：当前文件只包含 `Msg*` 文本常量，`errmsg` 比 `apperror` 更准确，且不会暗示包内存在错误类型或错误码构造逻辑。备选方案是保留 `apperror` 并只移动文件位置，但这无法解决命名混淆。

- 保持常量名称和值不变。
  理由：调用点只需要修改 import/package qualifier，避免无意义的大范围重命名，并降低响应消息文本变化风险。备选方案是重命名为更短的常量名，但会扩大 diff 且没有行为收益。

- 仅在 user-services 模块内迁移，不把消息常量放入 common。
  理由：这些消息是用户服务业务文案，不属于跨服务共享基础能力。放入 common 会扩大共享模块职责并引入不必要耦合。

## Risks / Trade-offs

- 引用遗漏导致编译失败 → 通过搜索 `internal/apperror` 和 `apperror.` 并运行 user-services 测试验证。
- 错误消息文本被意外修改 → 迁移时保留常量名称和值，必要时用测试或 diff 检查。
- 删除旧包影响未搜索到的生成文件或测试文件 → 全仓库搜索旧 import，并避免手写修改 `user-services/ent/` 生成代码。

## Migration Plan

1. 新增 `user-services/internal/errmsg/messages.go`，内容从旧 messages 文件迁移并将 package 改为 `errmsg`。
2. 将 user-services 内所有 `internal/apperror` import 更新为 `internal/errmsg`，并将 `apperror.Msg*` 引用更新为 `errmsg.Msg*`。
3. 删除 `user-services/internal/apperror/messages.go` 和空目录。
4. 运行 `gofmt` 与 `go test ./...`（user-services 模块）验证。
5. 回滚策略：如出现问题，可恢复 `apperror/messages.go` 并将 import/references 改回旧包。

## Open Questions

- 无。

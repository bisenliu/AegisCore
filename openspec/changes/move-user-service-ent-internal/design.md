## Context

`user-service` 当前把 Ent schema 与生成代码放在模块根级 `user-service/ent/`，对应 import path 为 `github.com/aegiscore/user-service/ent`。该路径不是 Go `internal` 包，其他 workspace module 或未来服务只要解析到 `github.com/aegiscore/user-service`，就可以直接导入 Ent client、entity、predicate、`enttest` 和 `migrate` 包，绕过 user-service 的 feature-first、application/domain 和 infrastructure adapter 边界。

Ent schema 仍是数据库结构来源，Atlas SQL migration 仍是可审查交付工件。本 change 只调整 Ent 代码归属和生成路径，不改变 HTTP API、OpenAPI、数据库 schema 语义、migration SQL 策略、部署发布顺序或观测资产。

## Goals / Non-Goals

**Goals:**

- 将 Ent schema、生成代码、`enttest` 和 `migrate` 统一迁入 `user-service/internal/persistence/ent/`。
- 移除模块根级 `user-service/ent/` 公开包路径，不保留兼容包、alias、shim 或双路径支持。
- 调整 Ent generate、Atlas schema helper、user-service 内部 import 和测试，使所有 Ent 使用收敛到 `github.com/aegiscore/user-service/internal/persistence/ent`。
- 通过 `user-service-architecture-lint` 或等价架构检查禁止重新引入 `github.com/aegiscore/user-service/ent` import。
- 保持现有数据库 schema、migration SQL 语义、运行时行为和公开 HTTP/OpenAPI 契约不变。

**Non-Goals:**

- 不拆分 user-service 数据库或引入跨服务数据访问 API。
- 不为其他服务提供 Ent client、repository SDK 或数据库共享契约。
- 不调整 feature application/domain 的业务行为、DTO、HTTP route 或 RBAC policy 语义。
- 不新增 Atlas apply、自动 migration Job 或运行时 schema create 入口。
- 不保留 `github.com/aegiscore/user-service/ent` 的任何兼容导出。

## Decisions

1. Ent 目标路径使用 `user-service/internal/persistence/ent/`。

   选择原因：`internal` 由 Go 编译器强制限制导入范围，`persistence/ent` 清晰表达这是 user-service 私有持久化实现。备选 `user-service/internal/ent/` 虽然也能保护访问，但目录语义不如 `persistence/ent` 明确；备选保留 `user-service/ent/` 并依赖 lint 或约定无法阻止外部 module import。

2. 不保留根级兼容包。

   选择原因：兼容包会继续暴露 Ent 类型，无法消除架构边界风险，并会让未来使用方误认为根级 Ent 是稳定公共 API。备选 re-export、type alias 或 deprecation window 均与本次安全边界收敛目标冲突。

3. Ent schema 与生成物整体迁移。

   选择原因：Ent 生成代码内部存在大量相对 schema 的自引用 import，schema 与生成物分离会增加 generate 配置复杂度和 drift 风险。整体迁移后可继续在新目录内使用 `go generate`，让 Ent 自然生成新的 `internal/persistence/ent` import path。

4. Atlas schema helper 只导入新的 `internal/persistence/ent/migrate`。

   选择原因：migration diff 和 validate 仍由 user-service 交付流程拥有，但不应通过根级 Ent 包暴露 migration 实现。该脚本位于 `user-service/scripts/atlas-schema/`，属于 `github.com/aegiscore/user-service` 模块目录树内，可以合法导入 user-service 的 internal 包。

5. 架构检查必须禁止旧 import path。

   选择原因：生成目录迁移后，旧路径不应再出现在手写代码、生成入口或测试中。仅靠人工 review 容易回归，架构 lint 需要把该边界固化为可执行门禁。

## Risks / Trade-offs

- [Risk] Ent 生成代码迁移会造成大量 import path diff，review 噪声较大。→ Mitigation：优先通过 Ent generate 重建生成物，手写代码只做 import 路径收敛，并用 `git diff` 区分生成物与手写变更。
- [Risk] Atlas migration helper 或 Ent generate 路径未同步会重新生成根级 `user-service/ent/`。→ Mitigation：更新生成入口后执行 `make user-service-generate`、`make user-service-migrate-validate` 和 drift 检查，并用架构 lint 禁止旧路径。
- [Risk] 测试中的 `enttest` import 改动可能遗漏。→ Mitigation：全仓搜索 `github.com/aegiscore/user-service/ent`，并运行 `make user-service-test`。
- [Risk] 外部未登记模块如果已经依赖根级 Ent，会在升级后编译失败。→ Mitigation：这是预期 breaking change，不提供兼容分支；外部模块必须停止直接访问 user-service 数据库模型，改走正式服务 API 或明确的新 change 设计。
- [Risk] 回滚会重新暴露架构风险。→ Mitigation：如实现阶段出现阻断，只回滚本 change 的目录迁移和 import 更新，不发布部分完成状态；不引入临时双路径兼容。

## Migration Plan

- 移动 `user-service/ent/schema` 和生成入口到 `user-service/internal/persistence/ent/`，删除旧 `user-service/ent/` 目录。
- 在新目录执行 Ent generate，使生成物 import path 全部变为 `github.com/aegiscore/user-service/internal/persistence/ent/...`。
- 更新 user-service 内部 provider、feature infrastructure、RBAC CLI、Atlas schema helper 和测试 import。
- 更新架构 lint，拒绝 `github.com/aegiscore/user-service/ent` import 和根级 `user-service/ent` 目录回归。
- 运行 `make user-service-generate`、`make user-service-migrate-validate`、`make user-service-architecture-lint`、`make user-service-test`，暂存本次预期变更后运行 `make lint` 和 `make verify`。

Rollback strategy：实现前未发布时，使用 Git 回滚本 change 的文件移动、生成物和 import 更新即可；不得通过新增根级兼容包回滚风险。发布后如发现问题，应回退到上一个完整版本，而不是在当前版本恢复公开 Ent 包路径。

## Open Questions

无。

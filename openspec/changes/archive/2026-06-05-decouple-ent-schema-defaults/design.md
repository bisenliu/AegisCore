## Context

用户服务的 `User` Ent schema 当前位于 `user-services/ent/schema/user/schema.go`，并通过 `github.com/aegiscore/user-services/internal/domain` 获取 `status` 字段默认值。该默认值本身是数据库契约值 `100`，但 import 方向让 Ent schema/codegen/Atlas schema source 依赖业务 domain 包。

现有分层要求是 Service 使用领域实体和领域状态规则，Repository 负责 Ent 模型与 domain 类型转换，Ent 模型保持 PostgreSQL repository 和 schema 生成链路的实现细节。本次变更只修正 schema 层默认值来源，不改变用户表字段、索引、字段注释、API 或业务状态判断。

## Goals / Non-Goals

**Goals:**

- 移除 `user-services/ent/schema/user/schema.go` 对 `internal/domain` 的 import。
- 在 Ent schema 层保留 `status` 数据库默认值 `100`，确保 Atlas 读取到的 schema 语义不变。
- 保持 repository 到 domain 的映射边界不变，业务状态规则继续由 `domain.UserStatus` 及其方法表达。
- 通过 `go generate ./ent` 和测试验证 Ent codegen、编译与现有行为不受影响。

**Non-Goals:**

- 不迁移 `domain.UserStatus` 到 `common` 或新的共享包；用户状态仍是用户服务业务常量。
- 不修改用户表结构、字段类型、索引、注释、默认值数字或 migration 历史。
- 不修改 controller、service、repository 的公开接口、HTTP API、响应信封或错误码。
- 不手写 `user-services/ent/` 下的生成代码。

## Decisions

### Decision: Ent schema 使用本地数据库默认值常量

在 `user-services/ent/schema/user/schema.go` 或同一 schema 分类包内定义只服务于 schema 声明的本地常量，例如 `const defaultUserStatus = 100`，并用于 `field.Int64("status").Default(defaultUserStatus)`。

选择该方案是因为默认值是数据库 schema 契约，不需要依赖业务 domain 类型即可表达。该常量名称应体现数据库默认值用途，避免被 service 或 repository 当作业务状态规则复用入口。

备选方案：继续使用 `int64(domain.UserStatusNormal)`。该方案保持数字一致但维持反向依赖，不能解决 codegen 链路依赖业务 domain 的问题。

备选方案：将用户状态常量抽到 `common`。该方案会把用户业务语义提升为共享契约，但当前没有跨服务复用需求，违反服务业务常量保留在 `user-services` 内的边界。

### Decision: Repository/domain 映射保持不变

Repository 继续把 Ent `status` 数值映射为 `domain.UserStatus`，Service 继续通过 domain 表达状态规则。本次变更不改变 repository 抽象、领域实体、认证或查询流程。

选择该方案是因为问题仅在 schema 默认值来源，业务层边界已经由现有 `user-domain-boundary` 能力约束。扩大到 repository 或 service 重构会增加无关风险。

### Decision: 不生成数据库 migration

本次变更保持 `status` 字段默认值仍为 `100`，字段类型、注释和索引不变。因此 Atlas migration 目录和 `atlas.sum` 不应变化。

实现后可以运行 Ent 代码生成确认 schema source 可编译；如生成输出没有语义变化，不应新增 SQL migration。若工具产生了仅格式或无语义变化的生成文件，应审查并只保留必要变更。

## Risks / Trade-offs

- [Risk] schema 本地常量与 `domain.UserStatusNormal` 数字未来可能漂移。→ Mitigation：在 spec 和实现说明中明确数据库默认值必须保持与正常用户状态契约一致，并通过测试或 code review 校验 `100` 兼容性。
- [Risk] 本地常量可能被误用为业务状态规则。→ Mitigation：将常量限制在 Ent schema 分类包内，命名为 schema/default 用途，不从 domain、service 或 repository 引用。
- [Risk] 运行 `go generate ./ent` 可能更新生成代码。→ Mitigation：不手写生成代码，只保留 Ent 生成工具基于 schema 产生且经审查的变更；确认数据库 schema 语义未变化。

## Migration Plan

1. 修改 `user-services/ent/schema/user/schema.go`，移除 `internal/domain` import，并用 schema 本地默认值表达 `status=100`。
2. 在 `user-services` 模块运行 `go generate ./ent`，确保 Ent codegen 可以在不依赖 domain import 的情况下完成。
3. 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，确认编译和测试通过。
4. 审查 `user-services/migrations/` 和 `atlas.sum` 不发生变化；本次无需运行迁移 apply，也无需部署前数据回填。

Rollback 策略：如发现生成链路或测试失败，可回退 schema 本地常量改动并恢复 domain import；该回退不涉及数据库 migration 或外部 API 回滚。

## Open Questions

无。

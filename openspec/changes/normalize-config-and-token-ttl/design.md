## Context

`shared-infrastructure` 通过 `common/config.Config` 表达运行时配置。当前 `Redis` 字段与 `PostgresConfigs` 字段同样承载命名实例集合，但命名风格不一致；外部配置契约已经稳定为 `redis.<name>` 与 `postgres.<name>`。`user-session-control` 在认证 service 和 Redis session store 中使用默认 TTL 兜底，但默认值直接写在方法体内，审查和统一调整成本较高。

本变更横跨 `common` 和 `user-services` 两个模块，但不涉及 API、数据库 schema、Ent 生成代码、Atlas migration 或运行时依赖图调整。

## Goals / Non-Goals

**Goals:**

- 将 `common/config.Config` 的 PostgreSQL 命名实例字段统一为 `Postgres`，并同步仓库内所有 Go 引用。
- 保持 `mapstructure:"postgres"`、YAML 配置路径和 `AEGISCORE_POSTGRES_*` 环境变量覆盖行为不变。
- 在 `user-services/internal/service` 中为认证默认 TTL 提供集中常量，替代方法体中的魔法时间值。
- 通过现有或新增测试验证配置加载和默认 TTL 行为没有变化。

**Non-Goals:**

- 不新增配置字段，不改变 YAML、环境变量、Redis key、JWT claims、HTTP API 或错误响应。
- 不引入外部兼容别名或迁移层，除非实现阶段发现仓库内存在无法同步更新的消费者。
- 不调整认证 TTL 的默认数值和业务策略。
- 不修改 Ent schema、生成代码或数据库 migration。

## Decisions

- 决策：直接将 `Config.PostgresConfigs` 重命名为 `Config.Postgres`，保留 struct tag `mapstructure:"postgres"`。理由是当前需求是命名一致性，外部配置契约由 tag 和 Viper 映射决定，字段名仅影响 Go API。备选方案是保留旧字段并添加新字段或 helper，但会留下重复来源并增加配置状态歧义。
- 决策：同步更新仓库内所有 `PostgresConfigs` 引用，并将与新字段冲突的 PostgreSQL 数据库配置 helper 重命名为 `Config.PostgresDatabase(name)`。理由是 Go 不允许同一类型同时存在名为 `Postgres` 的字段和方法，编译可完整捕获遗漏引用，避免半迁移状态。备选方案是保留旧字段并添加新字段，但会留下重复来源并增加配置状态歧义。
- 决策：默认 TTL 常量放在 `user-services/internal/service` 包内，供 `auth_service.go` 和 `session_store.go` 使用。理由是这些默认值属于认证会话服务实现策略，不需要上移到 `common`；配置加载仍只负责反序列化显式配置。备选方案是放入 `common/config` 默认值，但会把 service 兜底策略混入配置 loader 职责。
- 决策：保持当前默认值不变：Access Token 为 15 分钟，Refresh Token 为 7 天，token version cache 为 5 分钟；如 session store 的通用会话 TTL 兜底仍存在，应一并常量化但不改变数值。理由是本变更目标是可维护性清理，不改变安全或会话生命周期策略。

## Risks / Trade-offs

- [Risk] `Config.PostgresConfigs` 是导出的 Go 字段，仓库外消费者如果直接引用会编译失败。→ Mitigation：当前仓库为单 workspace 内部服务，先同步仓库内引用；如未来确认存在外部模块依赖，再单独提出兼容策略。
- [Risk] struct 字段重命名可能误改外部配置 key。→ Mitigation：保留 `mapstructure:"postgres"` 并运行 `common` 配置加载测试覆盖 YAML 和环境变量路径。
- [Risk] 提取 TTL 常量时可能改变零值配置的兜底行为。→ Mitigation：只替换字面量，不改变 `<= 0` 判断；用用户服务测试覆盖默认 TTL 和显式配置 TTL。
- [Risk] 过度抽象默认值会让配置职责不清。→ Mitigation：常量限制在 `user-services/internal/service` 包内，不放入 `common/config.Load` 或共享基础设施 provider。

## Context

`shared-infrastructure` 通过 `common/config.Load` 读取 YAML 配置并支持 `AEGISCORE_` 环境变量覆盖。当前实现先调用 `setDefaults(v)` 注册 Viper 默认值，再在 `applyDefaults` 中为 PostgreSQL driver、端口、连接池和超时等字段补默认值。这样可以降低本地启动成本，但也会让缺少主要配置的部署环境以隐式值启动，增加连接到错误依赖、暴露错误 HTTP 监听地址或使用非预期日志/Redis 参数的风险。

近期 PostgreSQL 配置已调整为共享连接参数结构，配置校验已经集中在 `common/config`。本变更继续收紧配置策略：主要运行时配置必须显式来自配置文件或环境变量，缺失时直接返回错误，让 Fx app 在启动前失败。

## Goals / Non-Goals

**Goals:**

- 移除 Viper `SetDefault` 默认注册，不再由代码向配置对象注入主要运行时默认值。
- 将主要配置字段改为显式必填，包括 app、HTTP、log、Redis 和 PostgreSQL 关键字段。
- 保持 `AEGISCORE_` 环境变量覆盖能力，允许部署通过环境变量补齐或覆盖配置值。
- 让 `common/config.Load` 在缺少主要配置时返回清晰错误，从而阻止服务启动。
- 更新测试以覆盖完整配置成功、缺失配置失败和环境变量覆盖仍可用。

**Non-Goals:**

- 不改变配置文件查找路径或 `--config` CLI 参数行为。
- 不新增远程配置中心、密钥管理或动态配置热加载。
- 不修改 HTTP API、controller/service/repository 分层、响应信封或 Ent schema。
- 不手写 `user-services/ent/` 生成代码，也不需要运行 `go generate ./ent`。

## Decisions

1. 删除 Viper 默认值注册，而不是仅缩减默认值范围。

   用户目标是“移除默认配置，主要配置没有配置就直接报错”。保留部分 `SetDefault` 容易让哪些字段可以默认、哪些必须显式变得不一致。因此实现应移除 `setDefaults` 或让其不再注册运行时默认配置，并通过校验明确要求主要字段存在。

   备选方案是只移除数据库默认值，但 HTTP、Redis、日志仍可能隐式默认，不能满足缺少主要配置即失败的目标。

2. 使用显式验证函数输出清晰错误。

   Go 零值无法区分“未配置”和“配置为零值”时，应通过字段语义验证主要字段：例如端口必须在有效范围，超时必须大于零，字符串字段不能为空。相比单纯依赖 validator tag，显式验证更容易输出与当前代码风格一致的错误，并覆盖 `time.Duration` 等字段。

   备选方案是给结构体加大量 `validate` tag，但当前代码已有手写 PostgreSQL 校验，混合 tag 和手写逻辑会增加维护成本。

3. 环境变量覆盖继续在 Viper 读取后统一生效。

   `AEGISCORE_` 前缀和 key replacer 是现有能力的一部分。移除默认值不应影响配置文件值被环境变量覆盖；测试应保留或新增覆盖用例，确保显式配置策略不破坏部署覆盖能力。

4. 示例 YAML 保持完整声明。

   `user-services/configs/config.yaml` 已包含主要配置字段，实施时应确认它在无默认值条件下仍可加载。若测试暴露缺失字段，应补齐示例配置而不是恢复代码默认值。

## Risks / Trade-offs

- [Risk] 依赖隐式默认值的本地或部署环境会启动失败。→ Mitigation：在 proposal 和 tasks 中标记 BREAKING，并确保示例配置完整。
- [Risk] 验证规则过严可能拒绝合法但少见的配置。→ Mitigation：仅要求主要运行时字段显式存在并满足基本有效性，不引入业务无关限制。
- [Risk] 移除默认值后现有测试需要更完整的配置 fixture。→ Mitigation：集中创建测试 helper 提供最小完整配置，并用定向缺失字段用例验证错误。
- [Risk] 环境变量覆盖在没有默认值时可能因 Viper 绑定不足而无法填充。→ Mitigation：实现阶段验证现有 `AutomaticEnv` 行为，必要时显式绑定关键配置 key。

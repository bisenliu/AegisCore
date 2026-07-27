## Context

当前 `common/runtime/config.DecodeStrict[T]` 先通过反射把 `commonconfig.DefaultConfig()` 注册到 Viper，再通过未导出的 `defaultsProvider` 自动发现 user-service 的 `ConfigDefaults()`，decode 后又通过 `defaultsApplier` 自动调用 `ApplyDefaults()`。user-service 的 resources、Ent 和两个 feature cache 分别依赖不同的默认值时机，完整顺序只能结合 `common/runtime/config/loader.go` 与 `user-service/internal/config/config.go` 推断。

Nacos 的 env、client、认证、failover 和文档读取也位于 config 根包，并由 `user-service/internal/config.Load` 直接调用 `LoadNacosMergedSettings`。auth 和 permission/RBAC 则分别从完整根配置读取 feature cache，并重复转换为 `localcache.Config`。本 change 横跨共享 runtime primitive、user-service 配置边界和两个安全 feature，因此分七阶段实施，先锁定行为再逐步替换调用链。

## Goals / Non-Goals

**Goals:**

- 固定 `source -> documents -> deep merge -> raw digest -> strict decode -> normalize -> validate` 的显式加载顺序。
- 将 Nacos 实现隔离为 document source adapter，让测试来源和未来文件、ConfigMap 来源复用同一加载入口。
- 用 `user-service/internal/config.DefaultConfig()` 集中 user-service 完整默认值。
- 用公开、显式 decode options 替代 `DecodeStrict[T]` 的隐式接口发现。
- 简化 feature cache 配置，集中默认值、校验和 localcache 映射，同时保留 enabled/disabled 安全语义。
- 让 auth、RBAC、Ent 和 resources provider 只消费职责所需的窄 settings。
- 保持 YAML 配置字段、Nacos 分层顺序、严格未知键、raw digest、effective render 脱敏和现有业务安全语义。

**Non-Goals:**

- 不实现配置 watch、热更新、Kubernetes ConfigMap source 或新的远程配置中心。
- 不改变 HTTP API、OpenAPI、Ent schema、Atlas migration、Redis key schema、Casbin policy 或业务错误契约。
- 不把 user-service 的 feature cache、资源名、JWT、RBAC 或 Ent 配置下沉到 `common`。
- 不保留旧 Go API、指针访问器或隐式 defaults hook 的兼容 wrapper。
- 不要求禁用的 feature cache 把容量、TTL 或回源超时清零；这些字段在禁用时只是不参与运行时构造和校验。

## Decisions

### Decision: 先锁定现有行为，再替换配置模型

阶段一复用并确认现有测试覆盖 Nacos 缺省 dataId、deep merge、unknown key、raw digest、effective render 脱敏、feature cache enabled/disabled、auth/permission localcache 创建。后续每个阶段必须保持这些测试的业务断言，只有“禁用时默认字段必须为零”这类实现细节断言可以随非指针配置迁移而删除。

备选方案是一次性重写后再补测试。该方案无法区分有意 breaking change 与无意行为漂移，尤其会放大安全配置和缓存回退风险。

### Decision: feature cache 使用完整值对象

`FeatureCacheConfig` 使用 `Enabled bool`、`Size int64`、`TTL time.Duration` 和 `LoadTimeout time.Duration`。`DefaultFeatureCacheConfig` 构造完整默认值，`Validate(path)` 只在启用时要求正数，`Localcache(name)` 集中转换为 `localcache.Config`。保留 `int64 Size` 可以在校验和构造阶段继续拒绝负数，再安全转换为 `uint64 Capacity`。

备选方案是继续使用指针区分缺省、显式 false 和零值。该方案延续访问器与后置补值样板，不能达到简化配置模型的目标。

### Decision: defaults 构造后由外部配置覆盖

新增 `user-service/internal/config.DefaultConfig()`，从 `commonconfig.DefaultConfig()` 开始组装 resources、Ent、auth 和 RBAC 默认值。严格 decode 以该完整值作为目标初值，外部配置覆盖对应字段；normalize 只处理需要结合输入完成的归一化，不再承担隐式默认值发现。

备选方案是保留 Viper defaults 和后置 `ApplyDefaults()`。该方案仍要求调用方理解不同字段的默认时机，并让显式 false 的正确性依赖隐藏规则。

### Decision: `DecodeStrict` 接收显式泛型 options

通用 config 包提供 `DecodeOptions[T]`，至少包含 `Defaults func() T`、`Normalize func(*T)` 和 `Validate func(T) error`。`DecodeStrict` 不再探测 `ConfigDefaults()` 或 `ApplyDefaults()`；user-service 的 `DecodeSettings` 在调用点明确组合三个步骤。unknown key 检查继续基于 raw merged settings 和 mapstructure metadata，在返回配置前失败并报告完整叶子路径。

备选方案是为隐式接口改名或导出。该方案只是让魔法可见，仍把服务扩展约定嵌入共享 loader。

### Decision: Nacos 是 `DocumentSource` adapter

通用根包拥有业务中立的 `ConfigDocument`、`SourceMetadata`、`DocumentSource` 和 source-to-decode 管线原语；`common/runtime/config/nacos` 拥有环境变量、HTTP client、认证、failover 和 Nacos 文档读取。`user-service/internal/config.Load` 显式从环境创建 Nacos source，再调用可注入 source 的加载入口。

根 config 包不能 import Nacos 子包，避免反向依赖；Nacos adapter可以 import 根包的 document contract。未来新来源通过新增 adapter 接入，不修改 deep merge、digest 或 strict decode 原语。

备选方案是在根包保留 `LoadNacosMergedSettings` facade。该方案继续让共享加载入口固定为 Nacos，也会形成需要长期维护的旧 API。

### Decision: composition root 派生窄 settings

user-service 配置包提供 `AuthSettings`、`RBACSettings`、`EntSettings` 和 `ResourceSettings` 及相应 provider。auth、permission/RBAC、Ent 和 resource provider 只接收自身 settings；根 `*Config` 只在配置加载和 composition root 派生阶段存在。settings 是普通 Go value，不嵌入 Fx 类型或 named tag。

备选方案是继续把完整配置传给 feature，再用代码约定限制访问。该方案无法形成编译期边界，测试仍需构造无关配置。

### Decision: raw 与 effective 配置保持双重语义

`SourceMetadata.Digest` 始终基于 deep merge 后、defaults 和 normalize 前的 raw settings。`EffectiveSettings()` 继续从最终 typed config 编码，供 `config render` 在调用 `RedactSettings` 后输出；默认值变化不得改变同一 raw 输入的 source digest，render 不得泄漏 JWT、Redis 或 PostgreSQL secret。

备选方案是对 effective settings 计算 digest。该方案会使默认值或编码格式变化看起来像配置来源变化，不利于定位 Nacos 原始配置版本。

## Risks / Trade-offs

- [风险] Viper 对预填默认 struct 的覆盖行为与 `SetDefault` 不完全相同。→ 缓解：每阶段运行配置单元测试，重点覆盖显式 false、零值、duration、具名资源和 unknown key。
- [风险] Nacos 包迁移产生 import cycle 或意外改变认证/failover。→ 缓解：根包只定义 document contract，adapter 单向依赖根包；迁移现有 Nacos 测试并保持请求、认证、超时和 failover 断言。
- [风险] 禁用 cache 后保留默认数值会让 render 看似仍配置容量。→ 缓解：规格明确 disabled 时这些值被忽略；不在 runtime config 中制造特殊清零语义。
- [风险] 窄 settings 改造 Fx graph 时遗漏 named dependency 或 lifecycle owner。→ 缓解：按 resources、Ent、auth、RBAC 分步迁移，并运行各 module/Fx graph 测试和 architecture lint。
- [风险] breaking Go API 影响仓库内未发现调用方。→ 缓解：全仓 `rg` 搜索旧 symbol，删除后运行 workspace 测试、`make lint` 和 `make verify`，不添加兼容 adapter。
- [风险] raw digest 或 render 路径在重构中改为对错误状态编码。→ 缓解：保留阶段一 digest 与脱敏测试，并在最终验证中运行 `user-service/cmd` 配置命令测试。

## Migration Plan

1. 确认阶段一行为测试并建立本 change，不修改生产加载链。
2. 将 feature cache 迁移为完整值对象，更新 auth/permission 构造与测试。
3. 引入 user-service 完整默认配置，显式完成 defaults 与 normalize。
4. 引入 `DecodeOptions[T]`，切换 user-service 后删除隐式 hook。
5. 把 Nacos 迁入子包并以 `DocumentSource` 接入 user-service。
6. 提供窄 settings 并收窄 resources、Ent、auth、RBAC provider 依赖。
7. 删除旧接口和实现细节测试，更新开发文档与规格，执行全量验证后归档。

回滚按阶段恢复最近一次调用链，YAML 和部署配置无需回滚。任一阶段未通过行为基线、相关包测试或 architecture lint 时不得进入下一阶段；最终归档前必须先暂存预期变更并通过 `make lint` 与 `make verify`。

## Validation

- 阶段测试：`go test ./runtime/config`、`go test ./internal/config`、`go test ./internal/features/auth`、`go test ./internal/features/permission/infrastructure/casbin`、`go test ./cmd`。
- 结构检查：`make user-service-architecture-lint`。
- OpenSpec：`openspec validate refactor-config-loading-pipeline`、`openspec validate --specs`。
- 最终门禁：暂存预期变更后运行 `make lint` 与 `make verify`。

## Open Questions

无。

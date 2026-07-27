## 1. 阶段一：梳理并锁定行为

- [x] 1.1 创建 `refactor-config-loading-pipeline` 的 proposal、design、`shared-platform-primitives`、`auth-session-management`、`rbac-access-control` spec delta 和覆盖七阶段的 tasks。
- [x] 1.2 确认 `common/runtime/config/nacos_test.go` 覆盖 Nacos env 缺省 dataId 顺序和 YAML deep merge 的 map 递归合并、slice 替换及 null 覆盖。
- [x] 1.3 确认 common 与 user-service config 测试覆盖 unknown key 完整路径拒绝、raw merged settings digest 与 effective settings defaults 解耦。
- [x] 1.4 确认 `user-service/internal/config/config_test.go` 和 `user-service/cmd/config_command_test.go` 覆盖 effective settings 编码、render 脱敏及 secret 不泄漏。
- [x] 1.5 确认 user-service config 测试覆盖 feature cache enabled/disabled 的默认、显式配置和校验行为。
- [x] 1.6 确认 auth 与 permission/RBAC 测试覆盖 localcache 启用时创建、容量映射、关闭，以及禁用时 direct fallback 和安全语义。
- [x] 1.7 运行 `openspec validate refactor-config-loading-pipeline`、相关配置/cache 单元测试和 `make user-service-architecture-lint`。

## 2. 阶段二：重构 feature cache 配置

- [x] 2.1 将 `FeatureCacheConfig` 改为非指针 `Enabled`、`Size`、`TTL`、`LoadTimeout` 字段，新增 `DefaultFeatureCacheConfig`、`Validate(path)` 和 `Localcache(name)`。
- [x] 2.2 更新 auth token-version cache 和 permission user-role cache 构造路径，统一使用 `Localcache(name)`，删除重复 `localcache.Config` 映射。
- [x] 2.3 更新配置、auth 和 permission/RBAC 测试，保留 enabled/disabled 安全行为，删除禁用时默认字段必须为零的实现细节断言。
- [x] 2.4 删除 `SizeValue`、`CapacityValue`、`TTLValue`、`LoadTimeoutValue`、`applyDefaults` 和 `validateFeatureCache` 样板，并运行相关包测试与 architecture lint。

## 3. 阶段三：统一 user-service 默认值

- [x] 3.1 新增 `user-service/internal/config.DefaultConfig()`，集中组装 common、resources、Ent、auth token-version cache 和 RBAC user-role cache 默认值。
- [x] 3.2 让 user-service 加载入口以完整默认配置为初值并由 raw settings 覆盖，保留显式 false、具名资源和 duration 解码行为。
- [x] 3.3 删除 `ConfigDefaults()` 和 `ApplyDefaults()` 的默认值职责；如仍需 normalize，保留为由加载入口显式调用的普通函数。
- [x] 3.4 更新默认配置、effective settings、raw digest 和资源默认值测试，并运行相关包测试与 architecture lint。

## 4. 阶段四：替换 `DecodeStrict[T]` 扩展 hook

- [x] 4.1 在 `common/runtime/config` 定义包含 `Defaults`、`Normalize` 和 `Validate` 的 `DecodeOptions[T]`，固定 strict decode 调用顺序和错误包装。
- [x] 4.2 修改 `DecodeStrict[T]` 与 user-service `DecodeSettings`，在调用点显式传入默认值、归一化和校验函数。
- [x] 4.3 删除 `defaultsProvider`、`defaultsApplier` 自动发现逻辑和相关反射默认注册路径，不新增旧签名兼容 wrapper。
- [x] 4.4 更新 common/user-service strict decode 测试，覆盖 defaults 覆盖、显式 false、normalize、validate、unknown key 完整路径和 raw digest，并运行相关包测试与 architecture lint。

## 5. 阶段五：拆分 Nacos source

- [x] 5.1 在 config 根包定义业务中立的 `DocumentSource` contract 和 source loading pipeline，使测试来源可复用同一入口。
- [x] 5.2 将 Nacos env、HTTP client、认证、timeout、failover 和 document source 实现迁入 `common/runtime/config/nacos`，保持缺省/显式 dataId 顺序与错误上下文。
- [x] 5.3 修改 `user-service/internal/config.Load`，显式从环境创建 Nacos source 并调用通用 source loading 入口；删除根包 `LoadNacosMergedSettings` 等旧 Nacos facade。
- [x] 5.4 迁移并补齐 Nacos source 测试，覆盖 env、认证、单次登录、failover、超时、空文档、deep merge 和 source metadata，并运行相关包测试与 architecture lint。

## 6. 阶段六：收窄使用方配置依赖

- [x] 6.1 在 user-service config 边界定义并提供 `AuthSettings`、`RBACSettings`、`EntSettings` 和 `ResourceSettings` 普通 Go value。
- [x] 6.2 修改 auth provider 和 token-version localcache 构造器只接收 `AuthSettings`，移除对完整 `*serviceconfig.Config` 的依赖。
- [x] 6.3 修改 permission/RBAC provider 和 user-role resolver 只接收 `RBACSettings`，移除对完整 `*serviceconfig.Config` 的依赖。
- [x] 6.4 修改 Ent 和具名 Redis/PostgreSQL provider 只接收对应窄 settings，保持资源选择、生命周期和错误语义。
- [x] 6.5 更新 Fx graph、module 和 constructor 测试，确认 feature 不能读取无关配置段，并运行相关包测试与 architecture lint。

## 7. 阶段七：清理、文档与最终验证

- [x] 7.1 全仓搜索并删除 `ConfigDefaults`、隐式 `ApplyDefaults` hook、`defaultsProvider`、`defaultsApplier`、旧 feature cache 指针访问器、根包 Nacos facade 和完整根配置 feature 依赖。
- [x] 7.2 更新 `docs/DEVELOPMENT.md`，明确 source、documents、deep merge、raw digest、strict decode、defaults、normalize、validate、effective render 和 redact 顺序及包职责。
- [x] 7.3 检查配置 examples、Compose、Kubernetes、Helm 和测试 fixture，确认 YAML 字段与 Nacos dataId 顺序无 drift；确认无需更新 OpenAPI、Ent 生成物、migration 和 dashboard。
- [x] 7.4 运行 `openspec validate refactor-config-loading-pipeline`、`openspec list --specs`、`openspec validate --specs`、相关包测试和 `make user-service-architecture-lint`。
- [x] 7.5 检查 `git diff`，确认只包含本 change 预期代码、测试、文档和 OpenSpec artifacts，并将全部预期变更加入暂存区。
- [x] 7.6 在预期变更保持暂存的状态下运行 `make lint`，修复后重跑直至通过。
- [x] 7.7 在预期变更保持暂存的状态下运行 `make verify`，确认生成物、测试、lint 和最终 `git diff --exit-code` 全部通过。
- [x] 7.8 所有实现、规格、文档和验证任务完成后，将 checkbox 更新为 `- [x]`，执行归档前规格一致性检查，再运行 `/opsx:archive refactor-config-loading-pipeline`。

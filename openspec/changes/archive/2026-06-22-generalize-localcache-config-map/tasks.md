## 1. OpenSpec 与现状确认

- [x] 1.1 确认 `generalize-localcache-config-map` proposal、design 和 delta specs 通过 OpenSpec 状态检查。
- [x] 1.2 梳理 `common/runtime/config`、auth provider 和 permission/RBAC provider 中所有 `LocalCache.AuthTokenVersion`、`LocalCache.RBACUserRoles` 调用点。

## 2. common/runtime/config 泛化

- [x] 2.1 将 `common/runtime/config.LocalCacheConfig` 改为通用具名实例集合，并保留 `LocalCacheInstanceConfig` 字段语义。
- [x] 2.2 为 local cache 配置增加按名称读取的 helper 或等价访问方式，供服务侧 provider 明确处理缺失实例。
- [x] 2.3 修改 `common/runtime/config/validation.go`，按排序 key 遍历所有 `local_cache` entry，统一校验容量、TTL、load timeout、num counters 和 buffer items。
- [x] 2.4 从 common 中移除 `AuthTokenVersion`、`RBACUserRoles` 固定字段引用和两个业务缓存名的专用校验。

## 3. common 配置测试

- [x] 3.1 更新 loader 测试，确认 `local_cache.auth_token_version`、`local_cache.rbac_user_roles` 和额外自定义 entry 能解析到通用 map。
- [x] 3.2 更新 env override 测试，确认现有 local cache env 覆盖仍能作用到对应 map entry。
- [x] 3.3 更新 validation 测试，确认所有 entry 都执行通用校验，错误路径包含 `local_cache.<name>.<field>`，并覆盖空配置 key。

## 4. user-service provider 与调用点

- [x] 4.1 更新 auth feature/provider，使用 auth 本地常量读取 `local_cache.auth_token_version`，缺失时返回明确错误，并保持 localcache 构造参数和默认值不变。
- [x] 4.2 更新 permission/RBAC provider，使用 RBAC 本地常量读取 `local_cache.rbac_user_roles`，缺失时返回明确错误，并保持 localcache 构造参数和默认值不变。
- [x] 4.3 更新 user-service 中所有 `LocalCache.AuthTokenVersion` 和 `LocalCache.RBACUserRoles` 测试构造与断言。
- [x] 4.4 确认 `user-service/configs/config.yaml` 仍使用 `local_cache.auth_token_version` 和 `local_cache.rbac_user_roles`，且不改变默认配置值。

## 5. bootstrap/e2e 与回归测试

- [x] 5.1 更新 bootstrap 配置 validation 测试和 e2e harness 配置断言，适配通用 local cache map。
- [x] 5.2 新增或更新 auth provider 测试，覆盖缺少 `auth_token_version` 配置实例时明确报错。
- [x] 5.3 新增或更新 permission/RBAC provider 测试，覆盖缺少 `rbac_user_roles` 配置实例时明确报错。

## 6. 验证与收尾

- [x] 6.1 运行 common 配置相关测试，至少覆盖 `common/runtime/config`。
- [x] 6.2 运行 user-service 相关配置、bootstrap、auth 和 permission/RBAC 测试。
- [x] 6.3 运行 `make user-service-architecture-lint`。
- [x] 6.4 如格式化或生成物存在 drift，运行必要格式化/生成命令并检查 `git diff`。
- [x] 6.5 更新 tasks checkbox 状态并确认 `openspec status --change generalize-localcache-config-map` apply-ready。

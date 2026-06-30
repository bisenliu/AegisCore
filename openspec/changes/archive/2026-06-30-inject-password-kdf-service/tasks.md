## 1. common 密码 KDF 实例化

- [x] 1.1 在 `common/security/password` 新增实例化密码 KDF 服务公开类型和构造参数，要求并发上限和队列上限为正数且队列上限不小于并发上限。
- [x] 1.2 将 Argon2id 队列、执行槽位、`HashContext` 和 `VerifyContext` 迁移为实例方法，并删除包级 `HashContext`、`VerifyContext`、`argon2Gate` 和 `argon2Queue`。
- [x] 1.3 保持 Argon2id 算法参数、编码哈希格式、明文密码校验、哈希解析和常量时间比较语义不变。
- [x] 1.4 更新 `common/security/password` 单元测试，覆盖实例构造、无效资源预算、队列满、等待取消、哈希校验和多实例预算隔离。
- [x] 1.5 将 `common/security/password` 拆分为 constants、types、service、hash、kdf 和 parse 等包内文件，保持 `package password` 对外 API 和运行时行为不变。

## 2. user-service 装配和调用迁移

- [x] 2.1 在 user-service 配置结构、默认配置和配置校验中加入密码 KDF 并发上限与队列上限。
- [x] 2.2 在 `user-service/internal/providers` 的服务装配边界创建 `common/security/password` 服务实例，并通过 Fx 注入认证和用户 application。
- [x] 2.3 修改 auth credentials 组件，使登录校验和强制改密使用注入的密码 KDF 服务实例。
- [x] 2.4 修改 user create command，使用户创建密码哈希使用注入的密码 KDF 服务实例。
- [x] 2.5 修改 RBAC CLI 超级管理员创建或重置密码路径，使其在 CLI 装配边界显式使用密码 KDF 服务实例。

## 3. 测试和调用点清理

- [x] 3.1 更新 auth、user、RBAC CLI 和 e2e 测试中的密码哈希与校验调用，全部改为测试专用密码 KDF 服务实例。
- [x] 3.2 运行 `rg "password\\.(HashContext|VerifyContext)" common user-service`，确认没有包级密码 KDF 调用残留。
- [x] 3.3 运行 `go test ./common/security/password`，确认 common 密码 primitive 测试通过。
- [x] 3.4 运行相关 user-service package 测试，至少覆盖 auth credentials、auth command、user command、RBAC CLI 相关包。

## 4. 验证和规格收尾

- [x] 4.1 运行 `make user-service-architecture-lint`，确认架构边界和 OpenSpec 文档规则通过。
- [x] 4.2 运行 `make lint`，确认 Go lint 通过。
- [x] 4.3 运行 `make verify`，确认仓库级验证通过。
- [x] 4.4 运行 `git diff --exit-code -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml user-service/ent`，确认本 change 未产生 OpenAPI 或 Ent 生成物漂移。

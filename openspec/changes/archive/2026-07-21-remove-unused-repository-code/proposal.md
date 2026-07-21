## Why

仓库中存在一组已经没有生产或测试调用的 helper、只为测试便利而留在生产文件中的 panic 构造器，以及与当前配置和构建流程不一致的脚本、忽略规则和文档。这些残留扩大了共享 API 和生产代码表面积，部分内容还会触发无意义的登录副作用或指导运维人员使用实际不会生效的环境变量。

本次清理以全仓反向引用、正式二进制入口可达性、现有主规格和测试覆盖为共同依据，只删除具有替代入口或明确无消费者的实现。具有独立公共语义、外部契约风险、生成器入口或主规格要求的候选继续保留。

## What Changes

- **BREAKING** 删除 `common/http/binding.JSONBinderWithOptions`；仓库内调用继续使用语义明确的 `JSONBinder` 或 `StrictJSONBinder`。
- **BREAKING** 删除未使用的 `common/runtime/config.ValidatePositiveInt` 及其私有实现，并删除与单一配置来源组装约束冲突的旧 `common/runtime/config.ConfigPath` 和 `NewConfig` Fx loader。
- 从 auth 和 permission Redis adapter 的生产文件删除只被测试调用的 `MustKeyCatalog`，在 `_test.go` 中保留小写测试 helper。
- 删除真实指标压测脚本中未读取的 normal user refresh token 状态和由此产生的第二次登录。
- 清理 `.gitignore` 中重复规则和旧 `services/` 目录规则。
- 修复 pprof 环境变量、Go lint 规格链接和 gosec 固定版本文档。
- 删除 user-service Docker 编译层未消费的 `tools` 源码复制，并把 Docker 构建上下文收窄为所需的 `tools/openapi-convert/go.mod`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 收敛共享 binding 和 runtime config 的公开实现表面积，并明确共享配置不得通过 Fx loader 重复读取配置文件。

## Impact

- 共享 Go API：`common/http/binding/`、`common/runtime/config/`。仓库外消费者若存在，需要迁移到现有明确 API。
- user-service 测试：auth/permission Redis adapter 测试 helper 与调用点。
- 交付资产：真实指标压测脚本、Dockerfile、`.dockerignore`、`.gitignore`。
- 文档：根和 user-service README、架构/开发/lint 文档及部署 README。
- 不改变 HTTP API、OpenAPI、Ent schema、Atlas migration、数据库数据、RBAC 决策、metrics family 或运行时配置结构。

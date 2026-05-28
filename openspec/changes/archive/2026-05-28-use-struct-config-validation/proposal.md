## Why

`require-explicit-config` 当前实现用一大段手写 `validateConfig` 和 `requiredKeys` 同时表达必填、取值范围和环境变量绑定，逻辑重复且难维护。将必填/非必填和基础范围校验放到配置 struct tag 上，可以让配置契约靠近字段定义，同时减少 loader 中的手工 if 判断。

## What Changes

- 为 `common/config` 的配置结构体字段添加验证标签，明确哪些字段必填、哪些字段可选，以及端口、连接池大小、timeout 等基础范围约束。
- 恢复使用 `go-playground/validator` 校验配置结构，替代大部分手写字段校验。
- 保留必要的 Viper 环境变量绑定机制，确保无默认值时 `AEGISCORE_` 覆盖仍可工作。
- 对可选字段明确建模，例如 Redis username/password、PostgreSQL password、HTTP trusted proxies 可显式为空或省略时按约定处理；主要运行时字段仍必须显式提供。
- 更新测试以验证 struct tag 校验覆盖必填/非必填语义，并保持 `require-explicit-config` 的“缺少主要配置则失败”目标。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-infrastructure`：配置加载仍要求主要配置显式提供并校验失败阻止启动，但实现方式改为以配置结构体标签为主，减少 loader 中的手工校验逻辑。

## Impact

- 影响代码：`common/config/config.go`、`common/config/loader.go`、`common/config/loader_test.go`。
- 影响运行时：外部行为保持与 `require-explicit-config` 一致，缺少主要配置仍在启动前失败。
- 影响维护性：必填/可选字段规则集中在 struct 定义上，新增配置字段时更容易同时声明校验规则。
- 不影响 HTTP API、业务分层、响应信封或 Ent schema；不需要修改 `user-services/ent/` 生成代码。

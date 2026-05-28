## Why

当前 `common/config/loader.go` 同时维护显式 required key 列表和结构体 tag 校验，导致配置加载阶段承担了字段存在性与基础格式校验职责。新的目标是简化配置加载路径：配置只负责读取 YAML 与 `AEGISCORE_` 环境变量覆盖，服务启动时由 HTTP server、Redis、PostgreSQL、Ent 等实际初始化过程直接暴露错误并终止启动。

## What Changes

- 移除 `common/config.Load` 中所有配置字段校验逻辑，包括显式 required key 校验和 `go-playground/validator` 结构体验证。
- 移除配置结构体上的 `validate` tag，使 `common/config` 不再声明 required/optional 或基础范围校验规则。
- 保留 YAML 文件读取、环境变量覆盖、配置反序列化和 PostgreSQL DSN 构造能力。
- 将 fail-fast 边界移动到服务启动初始化：Redis ping、PostgreSQL open/ping、HTTP server 创建或监听失败等异常直接返回错误并使服务启动失败。
- 调整测试：不再断言缺失配置会在 `Load` 阶段失败，而是覆盖配置读取、环境覆盖、可省略字段和命名 PostgreSQL DSN 行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 修改配置加载契约，配置加载不再执行 required/optional/范围校验，启动失败由实际运行时依赖初始化负责。

## Impact

- 影响代码：`common/config/config.go`、`common/config/loader.go`、`common/config/loader_test.go`。
- 影响依赖：如果 `go-playground/validator` 仅由配置加载使用，可从 `common` 模块依赖中移除。
- 影响规格：`openspec/specs/shared-infrastructure/spec.md` 的配置加载要求将通过 delta spec 改为“读取与反序列化，不做字段校验”。
- 运行时行为：缺失或非法配置不再由 `common/config.Load` 报字段级校验错误；相关错误会在后续运行时初始化阶段以连接、监听、DSN、driver 或超时等具体错误形式暴露。

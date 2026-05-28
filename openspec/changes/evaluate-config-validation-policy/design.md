## Context

`common/config/loader.go` 当前使用 Viper 加载 YAML 与 `AEGISCORE_` 环境变量，然后执行两层校验：`explicitRequiredKeys` 检查字段是否显式设置，`go-playground/validator` 根据 `validate` tag 检查 required、min、max、gt 等规则。用户现在要求移除所有配置校验，服务启动异常时直接报错。

当前启动链路已经包含实际依赖初始化：`common/infrastructure/redis.go` 在 Fx `OnStart` 中 ping Redis；`common/infrastructure/postgres.go` 为 `user_db` 和 `common_db` 创建连接池并 ping；HTTP server 在启动监听时返回错误。这些启动错误将成为 fail-fast 的主要机制。

## Goals / Non-Goals

**Goals:**

- 让 `common/config.Load` 只负责读取配置文件、绑定环境变量、反序列化为 `config.Config`。
- 移除所有 required/optional/范围校验代码和配置结构体校验 tag。
- 保持 `AEGISCORE_` 环境变量覆盖能力，包括当前可选字段如 trusted proxies、Redis 凭据、PostgreSQL password 和 `pay_db_name`。
- 保持 PostgreSQL DSN 构造与命名数据库查询逻辑不变。
- 通过现有 Fx lifecycle 和服务启动流程暴露 Redis/PostgreSQL/HTTP 初始化失败。

**Non-Goals:**

- 不实现新的配置默认值体系。
- 不新增健康检查聚合、可选依赖框架或第三方 API client。
- 不改变 HTTP API、响应信封、controller/service/repository 分层。
- 不新增业务能力或 Ent schema 变更。

## Decisions

- 删除配置加载阶段字段校验。`Load` 只返回读取失败和反序列化失败，例如配置文件不存在、YAML 无法解析、类型无法解码等；字段缺失、空字符串、零值或范围异常不再由 `Load` 主动拒绝。
- 删除 `go-playground/validator` 使用点。配置包不再需要 validator 运行时依赖，也不再维护 `formatValidationError`、`configPath` 等仅服务于校验的辅助函数。
- 保留显式环境变量绑定清单，但重命名语义。原 `explicitRequiredKeys` 不再存在；保留的 key 列表仅用于让 Viper 正确绑定环境变量并反序列化嵌套字段，不能表达必填规则。
- 连接与启动错误不前置。Redis/PostgreSQL 可达性继续在 `common/infrastructure` 的 Fx lifecycle 中检查；HTTP 监听失败继续由 HTTP server 启动流程返回。
- 测试从“配置校验失败”转为“配置读取不校验”。删除缺失字段和非法范围断言，新增缺失主要字段仍可加载为零值的测试，以明确新契约。

## Risks / Trade-offs

- [Risk] 错误定位会从字段级校验错误变为运行时初始化错误，可能更接近底层但不一定直接指出缺失字段。→ Mitigation: 保留启动错误原始上下文，例如 `ping postgres user_db`、`ping redis`、HTTP listen 错误。
- [Risk] 某些配置缺失可能不会在启动时立即暴露，直到相关代码路径消费该字段。→ Mitigation: 本次按用户要求移除校验；后续如需更强保障，应重新引入特定初始化路径校验，而非配置加载全局校验。
- [Risk] 零值端口、timeout 或连接池参数可能导致标准库或依赖库使用默认行为。→ Mitigation: 接受该行为作为“无配置校验”策略的一部分，并依赖服务启动或运行时异常反馈。
- [Risk] 移除 validator 依赖可能影响其他包。→ Mitigation: 实现前搜索 `validator` 使用点，只在确认配置包是唯一使用方时移除依赖。

## Context

当前 `common/config.Load` 使用 Viper 读取 YAML、绑定 `AEGISCORE_` 环境变量、反序列化为 `config.Config`，随后调用 `cfg.Validate()`。校验逻辑位于 `common/config/validate.go`，覆盖 app、HTTP、log、Redis 命名实例、PostgreSQL 命名实例、端口、连接池和 timeout 等基础规则。

用户要求重新执行 `evaluate-config-validation-policy`，因为其他 change 已经把校验重新加入当前代码。新的目标是在保留命名 Redis/PostgreSQL 配置加载能力的前提下，移除配置加载阶段的字段校验。

## Goals / Non-Goals

**Goals:**

- 让 `common/config.Load` 只负责读取配置文件、绑定环境变量、反序列化为 `config.Config`。
- 移除 `Config.Validate()` 调用和当前 `common/config/validate.go` 校验实现。
- 保持命名 Redis/PostgreSQL map 结构和查询 helper 不变。
- 保持 `AEGISCORE_` 环境变量覆盖能力，包括命名实例字段覆盖。
- 通过现有 Fx lifecycle 和服务启动流程暴露 Redis/PostgreSQL/HTTP 初始化失败。

**Non-Goals:**

- 不恢复旧的 `database.postgres.*` 配置格式。
- 不新增默认值系统、配置修复逻辑或健康检查聚合。
- 不改变 HTTP API、响应信封、controller/service/repository 分层。
- 不新增业务能力或 Ent schema 变更。

## Decisions

- 删除 `Load` 中的 `cfg.Validate()` 调用，而不是引入开关或可选校验模式。配置包保持单一职责：读取、覆盖、反序列化。
- 删除 `common/config/validate.go`。当前校验没有外部依赖，但它会让配置加载重新承担字段级策略，和本 change 的目标冲突。
- 保留 `v.BindEnv(v.AllKeys())` 绑定流程。该流程服务于 Viper 对嵌套命名实例的环境变量覆盖，不表达必填规则。
- 测试从“校验失败”改为“加载成功并保留零值/非法原始值”。例如空 app name、零端口、负 Redis DB、零 timeout、零连接池大小均不在 `Load` 阶段拒绝。
- 运行时失败继续由依赖初始化负责。Redis ping、PostgreSQL ping、HTTP server 创建/监听失败仍应返回错误并阻止服务假装可用。

## Risks / Trade-offs

- [Risk] 错误定位从字段级校验错误变为运行时初始化错误，可能不直接指出缺失字段。→ Mitigation: 保留基础设施错误上下文，例如 `ping redis cache_redis`、`ping postgres user_db`。
- [Risk] 某些缺失字段不会在启动时立即暴露，直到相关路径消费该字段。→ Mitigation: 接受这是“无配置加载校验”策略的一部分；如后续需要更强保障，应在具体初始化路径增加校验。
- [Risk] 零值 timeout、端口或连接池参数可能触发依赖库默认行为。→ Mitigation: 测试明确记录 `Load` 不拦截这些值，运行时行为由依赖库和启动过程决定。

## Migration Plan

1. 移除 `common/config.Load` 中的 `cfg.Validate()` 调用。
2. 删除 `common/config/validate.go`。
3. 更新 `common/config/loader_test.go`，将校验失败断言改为加载成功与值保留断言。
4. 运行 `go test ./...` 于 `common/` 和 `user-services/`。

## Open Questions

- 无。

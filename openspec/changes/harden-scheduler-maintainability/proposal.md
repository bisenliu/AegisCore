## Why

`common/runtime/scheduler` 和 `common/testing/containers` 中仍保留多个内联时间默认值，`runJob()` 也继续串联较多执行、锁、续租和收尾流程，降低后续维护和审查效率。同时 `common` 模块存在 `go mod tidy` 可清理的残留依赖项，增加依赖面认知成本。

## What Changes

- 在 scheduler 内统一提取锁释放和锁续租默认超时命名常量，移除 `5 * time.Second` 内联默认值。
- 在 PostgreSQL 测试容器 helper 中提取 Docker mapped port 探测间隔命名常量，移除 `100 * time.Millisecond` 内联等待值。
- 将 scheduler `runJob()` 拆分为私有小函数，分别承载执行权获取、锁获取、上下文/续租准备、cleanup 和结果记录。
- 对 `common` 模块执行模块级依赖整理，移除 `go mod tidy` 判定不再需要的间接依赖残留。
- 不保留兼容方案：不新增公开配置项、不保留旧内联 fallback、不为依赖残留保留手工钉住项。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 强化 `common` runtime primitive、测试基础设施和模块依赖维护要求，不改变业务 API 契约。

## Impact

- 影响代码：`common/runtime/scheduler/executor.go`、`common/runtime/scheduler/renew.go`、`common/runtime/scheduler/validation.go`、`common/testing/containers/postgres.go`。
- 影响依赖：`common/go.mod`、`common/go.sum` 可能随 `GOWORK=off go mod tidy` 清理残留项。
- 不影响 HTTP API、OpenAPI、数据库 schema、部署资产和 user-service 运行时业务行为。
- 验证重点：scheduler 单元测试、common 模块测试、架构 lint，以及最终 `make lint`、`make verify`。

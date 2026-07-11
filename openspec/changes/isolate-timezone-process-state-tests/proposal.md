## Why

`common/runtime/timezone` 初始化会修改 `TZ`、`time.Local` 和包级 `state`，这些都是进程级全局状态。当前测试 helper 直接裸写 `state = timezoneState{}`，隔离语义分散且容易被后续并行测试误用，存在 race 和上下文漂移风险。

## What Changes

- 为 timezone 包引入包内受控的测试 reset 机制，使测试重置与 `state` 内部锁语义一致。
- 将测试中的 `TZ`、`time.Local` 和 timezone 初始化状态保存、恢复集中到单一 helper。
- 明确 timezone 初始化是进程级一次性行为，相关测试不得无隔离并行化。
- 不改变 `InitConfig` 的生产语义、默认时区 `Asia/Shanghai` 或 `system.timezone` 配置含义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧 `common/runtime/timezone` 测试隔离要求，要求通过受控 helper 重置进程级状态并显式避免并行误用。

## Impact

- 影响代码：`common/runtime/timezone/timezone.go`、`common/runtime/timezone/timezone_test.go`。
- 影响规格：`openspec/specs/shared-platform-primitives/spec.md` 的 runtime primitive 测试稳定性要求。
- 不影响对外 API、数据库 schema、OpenAPI、部署资产或生产运行时行为。
- 验证重点：`common` 模块下执行 `go test -race ./runtime/timezone`。

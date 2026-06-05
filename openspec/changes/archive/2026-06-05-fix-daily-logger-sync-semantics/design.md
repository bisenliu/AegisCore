## Context

`common/runtime/logger/daily_writer.go` 提供共享日志基础能力中的按日分割 lumberjack writer。当前 `dailyLumberjackWriteSyncer.Sync()` 在持锁后调用 `w.logger.Close()`，而 `Write()` 后续仍会复用 `w.logger.Write(p)`，这使运行期 `logger.Sync()` 具备关闭当前 writer 的副作用。

Zap 的 `Sync()` 语义通常用于 flush buffered log entries，不表示终止 writer 生命周期。lumberjack `Logger` 没有单独 flush API，`Close()` 更接近生命周期关闭操作，因此将 `Sync()` 映射为 `Close()` 会让共享基础设施的日志行为与调用方预期不一致。

受影响包和文件：

- `common/runtime/logger/daily_writer.go`
- `common/runtime/logger/*daily_writer*_test.go` 或同包现有日志测试文件

## Goals / Non-Goals

**Goals:**

- 使 `dailyLumberjackWriteSyncer.Sync()` 不关闭仍在使用的 `lumberjack.Logger`。
- 确保调用 `Sync()` 后继续写入同一天日志文件仍然成功。
- 保留日期变化时通过 `rotateLocked()` 关闭旧 `lumberjack.Logger` 并创建新 writer 的行为。
- 用单元测试覆盖 `Sync()` 后继续写入的回归场景。

**Non-Goals:**

- 不新增日志配置项，不改变 YAML 或 `AEGISCORE_` 环境变量覆盖规则。
- 不改变日志文件命名规则、按日分割格式或 lumberjack 的 size/backups/age 配置。
- 不修改 HTTP API、响应契约、数据库 schema、Ent 生成代码或 Atlas migration。
- 不引入新的日志依赖或替换 Zap/lumberjack。

## Decisions

- `Sync()` 对按日 lumberjack writer 返回 `nil`，不调用 `Close()`。

  理由：lumberjack writer 没有 flush-only API；`Close()` 会改变 writer 生命周期，和 Zap `Sync()` 的常见调用预期不一致。返回 `nil` 保持 Zap sync 调用安全，避免运行期同步后破坏后续写入。

  备选方案：在 `Sync()` 中先 `Close()` 再立即创建新的 `lumberjack.Logger`。该方案会在每次 sync 时打断当前文件句柄并增加不必要的生命周期变化，且不提供比 no-op 更明确的 flush 保证，因此不采用。

- 关闭旧 writer 只保留在真实轮转路径中。

  理由：日期变化时旧文件句柄不再被使用，关闭旧 writer 是明确的生命周期边界；这与运行期 flush 语义不同。

  备选方案：新增公开 `Close()` 方法供 Fx lifecycle 调用。当前 `newDailyLumberjackWriteSyncer` 返回 `zapcore.WriteSyncer`，没有现成 lifecycle owner；本次问题只要求修复 `Sync()` 语义，不扩大为日志生命周期重构。

- 测试直接针对 `dailyLumberjackWriteSyncer` 行为。

  理由：目标缺陷位于 writer 语义，单元测试可用临时目录、固定 clock 和同包访问覆盖，不需要启动 Fx、HTTP server、PostgreSQL 或 Redis。

## Risks / Trade-offs

- [Risk] `Sync()` 不调用 `Close()` 后，进程退出前不会通过该路径主动关闭文件句柄。→ Mitigation：Zap/lumberjack 没有 flush-only 能力，本变更保留轮转关闭；进程退出场景仍由 OS 回收文件句柄，后续如需要显式生命周期关闭应单独设计。
- [Risk] 部分调用方可能误以为 `logger.Sync()` 会释放日志文件。→ Mitigation：`Sync()` 的标准语义不是关闭，测试锁定运行期继续写入行为，避免共享基础设施依赖关闭副作用。
- [Risk] 未覆盖通过完整 Zap logger 调用链触发 `Sync()` 的场景。→ Mitigation：核心风险在 writer 实现；如现有测试结构允许，可补充使用 `zapcore.AddSync` 或 logger 实例的集成式单元测试。

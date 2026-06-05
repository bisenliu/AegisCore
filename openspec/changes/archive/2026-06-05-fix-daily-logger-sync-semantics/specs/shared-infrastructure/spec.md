## ADDED Requirements

### Requirement: Keep daily log writer sync non-closing
共享 Zap 日志的按日分割 lumberjack writer 在处理 `Sync()` 时 SHALL NOT 关闭仍在使用的当前 writer。`Sync()` MUST 保持运行期可安全调用；调用后，同一日期内后续日志 MUST 继续写入当前日期文件。日期变化触发轮转时，系统 MUST 继续关闭旧日期 writer 并创建新日期 writer。

#### Scenario: Continue writing after sync
- **Given** 按日分割日志 writer 已经为当前日期创建日志文件
- **When** 调用方执行 Zap logger `Sync()` 或底层 writer `Sync()` 后继续写入日志
- **Then** 系统 MUST NOT 因该 sync 调用关闭当前 writer
- **Then** 后续日志 MUST 继续成功写入当前日期对应的日志文件

#### Scenario: Daily rotation still closes old writer
- **Given** 按日分割日志 writer 已经写入旧日期日志文件
- **When** 本地日期变化后第一条新日志写入
- **Then** 系统 MUST 关闭旧日期 writer
- **Then** 系统 MUST 创建并写入新日期对应的日志文件

## ADDED Requirements

### Requirement: Scheduler 配置快照与严格校验

系统 MUST 在注册边界持有独立、归一化且通过校验的 job 配置快照，并 MUST 将零值默认与负数错误明确区分；调用方持有的可变配置不得在注册期间被修改，也不得在注册后改变已注册任务的执行策略。

#### Scenario: 注册嵌套锁与续租策略

- **WHEN** 调用方通过 `Add` 注册包含 `LockPolicy` 和 `RenewPolicy` 指针的 job
- **THEN** scheduler MUST 在填充默认值前复制全部嵌套策略，并只让 cron closure 与执行 pipeline 持有归一化副本
- **AND** `Add` MUST NOT 修改调用方传入的 job、lock 或 renew 对象，调用方在 `Add` 返回后修改这些对象 MUST NOT 改变已注册任务配置或造成共享读写竞态

#### Scenario: duration 使用零值默认

- **WHEN** default lock TTL、job lock TTL、renew interval、renew timeout 或 Redis retry interval 使用文档允许的零值
- **THEN** scheduler MUST 按对应配置层级填充既有默认值，并继续执行既有范围关系校验

#### Scenario: duration 使用负数

- **WHEN** default lock TTL、job lock TTL、renew interval、renew timeout 或 Redis retry interval 为负数
- **THEN** scheduler 或 Redis locker 构造与注册边界 MUST 返回可通过 `errors.Is(err, ErrInvalidLock)` 识别的错误
- **AND** 系统 MUST NOT 把负数静默转换为默认值

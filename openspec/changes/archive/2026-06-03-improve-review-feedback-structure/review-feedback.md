# 代码评审反馈整理

## 1. `user-services/internal/errmsg/` 包命名

### 问题说明

`user-services/internal/errmsg/` 这个包名如果目标是表达“错误消息”，整体方向是可以接受的，但需要明确避免改成 `errMsg`、`err_msg`、`errorMsg` 这类混合大小写或带下划线的形式。Go package name 通常应保持短小、全小写、无下划线，并且在调用处通过 `package.Identifier` 组合阅读时保持自然。

在 `message`、`errormessages`、`errmsg` 三个候选中，更推荐继续使用 `errmsg`。`message` 语义过宽，无法体现该包与错误消息相关；`errormessages` 虽然表达完整，但作为包名偏长，调用处会显得重复和笨重。

### 原因分析

Go 官方和社区惯例都倾向于使用简短、全小写、语义聚焦的 package name。包名不是完整业务描述，而是调用表达式的一部分；维护者通常会结合导出的标识符一起阅读，例如 `errmsg.InvalidParam` 或 `errmsg.UserNotFound`。如果包名过长，调用处会重复上下文；如果包名过宽，阅读者又需要额外打开包内容才能判断用途。

缩写词在 Go 包名中也应保持统一风格。包名本身不使用驼峰，因此不应写成 `errMsg`；Go 包名一般也不使用下划线，因此不应写成 `err_msg`。`errmsg` 虽然是缩写组合，但它短小、全小写，并且能清楚表达错误消息语义，符合 Go 包命名的实用取向。

### 建议改法

建议保持或统一为 `errmsg`，并避免引入 `message`、`errormessages`、`errMsg`、`err_msg`、`errorMsg` 等替代形式。

如果后续确认该包只维护错误消息常量、错误码到文案的映射或错误提示模板，可以继续使用：

```text
user-services/internal/errmsg
```

如果后续该包职责扩大到错误码、错误类型、错误构造器等更完整的错误模型，应重新评估是否拆分为更具体的包或迁移到统一错误模型中，但这应作为单独重构处理。真正执行包重命名时，需要同步更新 Go imports、相关测试和文档引用，并保持 HTTP API、错误码、响应信封、配置 key 和数据库 schema 不变。

## 2. `common/infrastructure/` 目录组织

### 问题说明

当前 `common/infrastructure/` 作为共享基础设施能力的集中目录，已经承载配置、日志、Redis、PostgreSQL 等 runtime dependency provider。随着后续可能继续增加 MongoDB、RabbitMQ、更多 datastore 或 messaging 组件，如果所有实现继续平铺在同一目录下，目录会逐步变成基础设施“大杂烩”，出现文件数量增长、职责边界变模糊、维护者定位成本升高的问题。

这种结构在组件较少时尚可接受，但当基础设施类型持续增加后，单目录平铺会让不同组件的生命周期管理、配置读取、连接初始化和 Fx wiring 混在一起，不利于长期维护。

### 原因分析

共享基础设施代码通常具有较强的横向复用属性，一旦目录缺少边界，某个组件的修改容易影响无关组件。例如维护 Redis provider 时需要在同一目录中过滤 PostgreSQL、logger 或未来 RabbitMQ 的文件；新增 MongoDB 时也容易复制既有模式但缺少清晰放置位置。长期来看，这会降低代码可发现性，增加跨组件误改风险，也会弱化 `common` 模块对共享能力的职责划分。

此外，Redis、PostgreSQL、MongoDB、RabbitMQ 的配置模型、连接生命周期、健康检查方式和关闭逻辑都不同。将它们全部放在一个平铺目录中，会让基础设施类型之间的差异被隐藏在文件命名中，而不是体现在目录边界中。

### 建议改法

建议后续按基础设施类型或职责进一步拆分 `common/infrastructure/`，并保持每个子目录聚焦单一组件或单一职责。

按基础设施类型拆分时，可以采用类似结构：

```text
common/infrastructure/
  redis/
  postgres/
  mongo/
  rabbitmq/
```

按职责分层拆分时，也可以采用类似结构：

```text
common/infrastructure/
  datastore/
  messaging/
  logging/
  config/
```

如果当前只包含 Redis 和 PostgreSQL，建议先保持小步重构：优先把 Redis provider、PostgreSQL provider 及其 helper 分别放到职责明确的位置，避免一次性引入过度复杂的目录层级。后续新增 MongoDB、RabbitMQ 等组件时，再按相同规则扩展目录。

无论采用哪种拆分方式，后续重构都必须保持外部契约不变，包括 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例和 Fx named injection 行为。也就是说，目录结构可以优化，但 `postgres.user_db`、`postgres.common_db`、`redis.cache_redis` 等运行时配置路径和依赖注入名称不应因为目录拆分而改变。

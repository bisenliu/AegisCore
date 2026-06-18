# 公共契约与运行时规格

## 需求

### 需求：公共契约保持业务中立
`common/contract/errors`、`common/contract/pagination` 和 `common/contract/response` 必须提供跨服务稳定契约，不得包含 user-service 业务语义。

#### 场景：新增错误码
Given 需要新增跨服务错误分类
When 添加到 common contract
Then 错误码必须保持业务中立，并可通过公共响应 helper 渲染。

### 需求：运行时基础能力不拥有功能语义
`common/runtime` 基础能力必须只提供配置、日志、datastore wiring、ID 生成、localcache、rediskey、workerpool、scheduler、resources、timezone、metrics 和 tracing 等无业务语义能力。

#### 场景：auth Redis key schema
Given 认证功能需要 refresh session Redis key
When 定义 key schema
Then 认证 infrastructure 必须拥有功能 key schema，只能复用 `common/runtime/rediskey` 的通用构造规则。

#### 场景：使用 workerpool
Given 功能需要受控后台清理
When 提交后台任务
Then workerpool 可以执行任务，但不得成为 MQ、eventbus、outbox、可靠投递框架或安全关键会话策略执行器。

#### 场景：workerpool 的业务边界
Given 功能代码使用 `common/runtime/workerpool`
When 提交后台任务
Then workerpool 只能提供并发控制、生命周期、日志和统计能力，不得承载 refresh session 上限裁剪、token version 撤销、可靠消息或业务一致性协议。

#### 场景：scheduler 分布式锁
Given 定时任务具有多实例副作用
When 注册任务
Then 任务必须声明锁策略，锁 TTL 必须为正值，长任务必须具备续租策略。

### 需求：公共安全与 HTTP helper 保持通用
公共 Casbin 与 Gin middleware helper 只能提供通用请求三元组、执行器包装和基于 hook 的中间件骨架。user-service subject schema、权限目录、路由差异诊断、策略加载器和超级管理员基线必须留在 user-service permission/shared 边界。

#### 场景：新增授权行为
Given 行为依赖 `user:<uuid>`、角色或权限目录
When 选择包位置
Then 必须实现于 user-service permission/shared 边界，不得放入 common。

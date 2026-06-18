# HTTP API 契约规格

## 需求

### 需求：统一响应信封

HTTP API 必须通过 `common/contract/response.Envelope` 和 `common/http/response` helper 输出响应。

#### 场景：成功响应

Given API 操作成功
When controller 写出响应
Then JSON 响应必须包含稳定的 `success`、`code`、`message` 和 `data` 字段。

#### 场景：校验失败

Given 请求绑定或结构校验失败
When `binding.BindOrAbort` 处理请求
Then 响应必须使用 common 错误信封和稳定错误码映射。

### 需求：分页契约

列表 API 必须使用 common pagination 契约表达 cursor/keyset 分页响应模型。

#### 场景：用户列表

Given 客户端请求 `GET /api/v1/users`
When user feature 返回分页结果
Then 分页元数据必须遵循 `common/contract/pagination`。

### 需求：controller 边界

HTTP 控制器不得直接导入 Ent、Redis client、SQL package 或基础设施适配器。

#### 场景：准备 application 输入

Given 请求包含 path、query 或 body 字段
When 控制器准备 application 输入
Then 字段裁剪、默认值归一化、UUID/cursor/token 解析和 command/query 构造必须委托给功能内输入准备器。

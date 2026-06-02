## 1. Common 分页响应与参数能力

- [x] 1.1 在 `common/response/response.go` 中新增 `Pagination` 结构，字段包含 `page`、`page_size`、`total`、`total_pages`。
- [x] 1.2 在 `common/response/response.go` 中新增泛型或等价分页 payload 结构，使响应 JSON 形如 `data.items` 与 `data.pagination`。
- [x] 1.3 在 common 中新增分页参数规范化公共方法，输入 `page`、`page_size`，输出规范化后的 `Page`、`PageSize`、`Offset`、`Limit`。
- [x] 1.4 实现 `page < 1` 默认 `1`、`page_size < 1` 默认 `10`、`total_pages` 向上取整，且 `total=0` 时 `total_pages=0`。
- [x] 1.5 为 common 分页 helper 增加单元测试，覆盖未传值、负数、0、正常值、`total=128/page_size=20` 和空列表分页元信息。

## 2. 用户列表 DTO 与过滤设计

- [x] 2.1 在 `user-services/internal/dto/user.go` 新增用户列表 query DTO，包含 `page`、`page_size`、`name`、`email`、`active`。
- [x] 2.2 设计 `active` 为可空布尔语义，确保未传时不过滤，传入非法布尔值时由绑定或校验返回统一 HTTP 400。
- [x] 2.3 复用 `dto.UserResponse` 作为列表 item 返回结构，确保响应不包含 `password`。
- [x] 2.4 如 Swagger 泛型展示不足，新增仅用于文档的用户列表分页响应 DTO，运行时仍复用 common 分页结构。

## 3. Repository 查询实现

- [x] 3.1 扩展 `UserRepository` 接口，新增用户列表查询方法，返回当前页用户集合、过滤后总数和错误。
- [x] 3.2 新增 repository 输入结构，包含分页 `Offset`、`Limit` 以及 `name`、`email`、`active` 过滤条件。
- [x] 3.3 使用 Ent predicate 实现 `name` 模糊匹配、`email` 精确匹配、`active` 布尔匹配。
- [x] 3.4 确保 count 查询与 items 查询使用同一组过滤条件，items 查询按稳定顺序排序并应用 offset/limit。
- [x] 3.5 将 repository 非预期错误包装为内部错误可识别的普通 error，避免暴露数据库细节。

## 4. Service 编排实现

- [x] 4.1 扩展 `UserService` 接口，新增用户列表方法，接收列表 query DTO 或 service input。
- [x] 4.2 在 service 中调用 common 分页规范化方法，清洗 `name` 空白字符、将 `email` trim 后转小写。
- [x] 4.3 调用 repository 获取 items 与 total，并将 `ent.User` 切片映射为 `[]dto.UserResponse` 或等价响应类型。
- [x] 4.4 使用 common 分页结构构造返回数据，确保空结果返回空数组与正确 pagination。
- [x] 4.5 对 repository 错误使用 `response.FromError` 或现有错误映射策略转换为统一内部错误。

## 5. Controller、路由与 Swagger

- [x] 5.1 在 `UserController` 新增 `List` 方法，使用共享 validator 绑定 query 参数。
- [x] 5.2 在 `user-services/internal/router/router.go` 注册 `GET /api/v1/users`，并确认不会与 `GET /api/v1/users/:id` 冲突。
- [x] 5.3 在 controller Swagger 注解中描述 `page`、`page_size`、`name`、`email`、`active` query 参数。
- [x] 5.4 在 Swagger 注解中描述 HTTP 200 分页用户资料响应，以及 HTTP 400、401、500 统一失败信封。
- [x] 5.5 重新生成或更新 Swagger 文档，并验证文档包含 `GET /users` 列表接口且不暴露 `password`。

## 6. 测试与验证

- [x] 6.1 为 controller 增加用户列表请求测试，覆盖默认分页、显式分页、无效 `active`、响应 envelope 和分页 JSON 结构。
- [x] 6.2 为 service 增加用户列表测试，覆盖分页默认值、过滤参数清洗、空列表和 repository 错误映射。
- [x] 6.3 为 repository 增加用户列表测试或可替代的查询构造测试，覆盖 name/email/active 过滤和 total 统计。
- [x] 6.4 运行 `gofmt` 格式化所有修改的 Go 文件。
- [x] 6.5 在 `common/` 运行 `go test ./...`，确保 common 分页能力测试通过。
- [x] 6.6 在 `user-services/` 运行 `go test ./...`，确保用户服务新增列表接口及既有接口测试通过。

## 7. 规格一致性检查

- [x] 7.1 确认 `common/response` 输出的分页响应示例与 `data.items`、`data.pagination.page/page_size/total/total_pages` 契约一致。
- [x] 7.2 确认 `GET /api/v1/users` 在未传分页参数或传入小于 1 的分页参数时分别使用 `page=1`、`page_size=10`。
- [x] 7.3 确认用户列表过滤字段只支持白名单参数 `name`、`email`、`active`，未定义参数不影响查询。
- [x] 7.4 确认本变更未修改 Ent schema、未手写 `user-services/ent/` 生成代码、未新增 Atlas migration。

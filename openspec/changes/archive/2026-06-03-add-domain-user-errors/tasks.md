## 1. Domain Error Boundary

- [x] 1.1 在 `user-services/internal/domain` 中新增用户领域 sentinel error 定义，包含 `ErrUserNotFound` 与 `ErrUserAlreadyExists`。
- [x] 1.2 确认领域错误仅表达用户业务概念，不依赖 `common/response`、Gin 或 Ent 类型。

## 2. Repository Error Conversion

- [x] 2.1 更新 `user-services/internal/repository/user_repository.go`，将创建用户时的 Ent 唯一约束错误转换为 `domain.ErrUserAlreadyExists`。
- [x] 2.2 更新用户查询相关 repository 路径，将 Ent not found 或未匹配到未删除用户转换为 `domain.ErrUserNotFound`。
- [x] 2.3 移除查询和创建路径中 repository 对 `common/response` 与用户错误消息常量的直接依赖，保留非预期数据库错误的上下文包装。

## 3. Service Application Error Mapping

- [x] 3.1 更新 `CreateUser`，在 repository 创建返回错误时使用 `errors.Is` 将 `domain.ErrUserAlreadyExists` 映射为 `response.ConflictError(errmsg.MsgUserAlreadyExists)`。
- [x] 3.2 更新 `GetUserByID`，在 repository 查询返回错误时使用 `errors.Is` 将 `domain.ErrUserNotFound` 映射为 `response.NotFoundError(errmsg.MsgUserNotFound)`。
- [x] 3.3 确认其他非预期错误仍通过 `response.FromError` 转换为内部错误，外部安全 message 保持不暴露数据库细节。

## 4. Tests And Verification

- [x] 4.1 更新或新增 repository 单元测试，断言用户不存在和唯一约束冲突路径返回可被 `errors.Is` 识别的领域错误。
- [x] 4.2 更新或新增 service 单元测试，断言领域错误映射为 HTTP 404 not found 与 HTTP 409 conflict 应用错误。
- [x] 4.3 确认用户查询和创建 controller/API 测试仍保持既有响应 envelope、业务 code 和中文 message。
- [x] 4.4 对修改的 Go 文件运行 `gofmt`。
- [x] 4.5 在 `user-services/` 运行 `go test ./...`。

## 1. Response Contract

- [x] 1.1 将 `common/response.Code` 从字符串类型改为 `int`，定义 `CodeOK`、`CodeBadRequest`、`CodeValidationFailed`、`CodeUnauthenticated`、`CodeForbidden`、`CodeConflict`、`CodeNotFound`、`CodeInternalError` 的数字值。
- [x] 1.2 更新 `common/response.Envelope`、`OK`、`Created`、`Fail` 和现有错误类型，使成功与失败响应输出数字 `code`。
- [x] 1.3 增加共享 message 格式化逻辑，确保无 `args` 时直接使用固定文案且不调用 `fmt.Sprintf`。
- [x] 1.4 增加 `BadRequestError`、`UnauthenticatedError`、`ForbiddenError`、`ConflictError`、`NotFoundError` 的可变参数版本，并补充对应 Gin helper。
- [x] 1.5 增加 `WrapInternal(err error, publicMessage string)`，并让普通 Go error 经 `FromError` 默认映射为 `CodeInternalError` 与 `internal server error`。

## 2. Validation And Service Messages

- [x] 2.1 调整 `common/validation.BindOrAbort` 或相关错误分类逻辑，使 validator 规则失败返回 `CodeValidationFailed`，解析类请求错误返回 `CodeBadRequest`。
- [x] 2.2 新增 `user-services/internal/apperror`，只维护业务错误文案常量和格式化模板，不封装响应 helper。
- [x] 2.3 将 `user-services` 现有用户不存在等业务错误文案迁移为 `apperror` 常量，并继续通过 `common/response` helper 返回应用错误。
- [x] 2.4 检查 controller/service/repository 调用点，保持 HTTP 解析在 controller、业务编排在 service、数据库访问在 repository。

## 3. Tests

- [x] 3.1 为 `common/response` 增加或更新测试，覆盖数字业务码、HTTP status、固定 `%` 文案、格式化文案、`WrapInternal` 和 `FromError`。
- [x] 3.2 更新 `common/validation` 测试，分别断言 validator 校验失败为 `10001`，类型解析或请求格式错误为 `10000`。
- [x] 3.3 更新 `user-services/internal/controller` 测试，断言成功、bad request、validation failed、not found 和 internal error 的数字 `code` 与消息。
- [x] 3.4 检查是否存在字符串错误码断言或旧 helper 签名调用，并全部迁移到新契约。

## 4. Verification

- [x] 4.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 4.2 在 `common/` 执行 `go test ./...` 并修复失败。
- [x] 4.3 在 `user-services/` 执行 `go test ./...` 并修复失败。
- [x] 4.4 复核 OpenSpec delta 与实现一致，确认未引入认证、授权、数据库 schema 或 migration 范围外变更。

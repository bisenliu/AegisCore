## 1. JWT Error Semantics

- [x] 1.1 在 `common/security/auth/jwt.go` 新增导出错误常量 `ErrInvalidUserID`，用于表示非空但格式非法的 JWT 用户 ID。
- [x] 1.2 调整 `JWTService.parse`：`claims.UserID == ""` 时继续返回 `ErrMissingUserID`，`uuid.Parse(claims.UserID)` 失败时返回 `ErrInvalidUserID`。
- [x] 1.3 调整 `JWTService.sign`：签发输入 `UserID == ""` 时继续返回 `ErrMissingUserID`，非空但非 UUID 时返回 `ErrInvalidUserID`。

## 2. Tests

- [x] 2.1 在 `common/security/auth/jwt_test.go` 添加非法 UUID `user_id` claim 的 ParseToken 用例，断言返回 `ErrInvalidUserID` 且不再匹配缺失用户 ID 语义。
- [x] 2.2 添加或更新签发输入非法 UUID 的测试，断言 access/refresh/password-change token 签发返回 `ErrInvalidUserID`。
- [x] 2.3 保留缺失 `user_id` 测试，确认空值仍返回 `ErrMissingUserID`。

## 3. Verification

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `common/` 模块运行 `go test ./...`。
- [x] 3.3 确认本变更不涉及 Ent schema、Atlas migration、Redis key、运行时配置或 `go generate`。

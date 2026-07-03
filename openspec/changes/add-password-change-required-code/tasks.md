## 1. 共享错误契约

- [x] 1.1 在 `common/contract/errors` 新增 `CodePasswordChangeRequired Code = 20006`，补充中文注释，并保持现有错误码数值不变。
- [x] 1.2 更新 `common/contract/errors` 单元测试，断言 `CodePasswordChangeRequired` 的数值为 `20006`，并确认无需在 `common/http/response` 增加 user-service 专用 writer。

## 2. 认证登录响应

- [x] 2.1 在 auth HTTP transport 中增加强制改密响应映射：当登录结果为受限改密 token 时，写出 HTTP `200 OK`、`success: false`、`code: CodePasswordChangeRequired`、公开消息和 token data，不再通过 `response.OK` 返回 `CodeOK`。
- [x] 2.2 调整 `TokenResponse`、mapper 或局部响应 DTO，使强制改密响应携带 `access_token`、`token_type`、`expires_in`，不得携带 `refresh_token`，且前端判定不再依赖 `password_change_required` 字段。
- [x] 2.3 更新登录接口 Swagger 注解，说明 `POST /api/v1/auth/login` 的 HTTP 200 响应可能包含 `CodeOK` 普通登录结果或 `CodePasswordChangeRequired` 强制改密结果。

## 3. 测试与生成物

- [x] 3.1 更新 auth controller 测试，覆盖强制改密登录返回 HTTP `200 OK`、`success: false`、`CodePasswordChangeRequired`、受限 access token、空 refresh token，并确认普通登录仍返回 `CodeOK`。
- [x] 3.2 更新 auth command 或 token 相关测试，确认强制改密分支仍只签发 subject 为 `password_change` 的受限 token，且不创建普通 refresh session。
- [x] 3.3 更新 e2e 登录改密流程，使测试通过 envelope code 识别强制改密分支，并确认正常登录不携带 `CodePasswordChangeRequired`。
- [x] 3.4 运行 `make user-service-openapi-generate` 更新 OpenAPI 生成物，并检查 `user-service/docs/openapi.go`、`user-service/docs/openapi.json`、`user-service/docs/openapi.yaml` 只包含本变更预期 diff。

## 4. 验证与收尾

- [x] 4.1 运行相关测试：`make common-test` 和 `make user-service-test`。
- [x] 4.2 运行 `make user-service-architecture-lint` 验证架构边界和 OpenSpec 相关规则。
- [x] 4.3 暂存本次预期代码、规格和生成物变更，并运行 `git diff --exit-code` 确认没有未暂存的生成物 drift。
- [x] 4.4 在暂存预期变更后运行 `make lint`。
- [x] 4.5 在暂存预期变更后运行 `make verify`，确认最终验证通过。

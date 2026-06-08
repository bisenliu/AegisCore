## 1. Shared Auth Parser

- [x] 1.1 在 `common/security/auth` 中新增 required Bearer authorization header 解析函数，返回 token 并区分格式错误与空 token。
- [x] 1.2 保持 `StripBearerPrefix` 现有可选前缀语义不变，并复用或对齐 Bearer 前缀大小写无关匹配逻辑。
- [x] 1.3 为 shared auth parser 添加单元测试，覆盖标准 `Bearer `、lowercase/uppercase Bearer、前后空白、缺少前缀、空 token 和 token 内容大小写保留。

## 2. Middleware Integration

- [x] 2.1 将 `common/http/middleware/auth.go` 中的 `strings.HasPrefix`/`strings.TrimPrefix` Bearer 提取逻辑替换为 `common/security/auth` 的 shared parser。
- [x] 2.2 保持缺失 header、格式错误、空 token、JWT 无效、JWT 过期和 token version 不匹配的现有日志分支、HTTP 401、业务码和公开 message 兼容。
- [x] 2.3 清理中间件中不再需要的 imports，确保 common 包分层仍为 middleware 处理 HTTP、shared auth 处理 credential transport parsing。

## 3. Tests And Verification

- [x] 3.1 补充或调整 Gin 认证中间件测试，验证 `Authorization: bearer <token>` 和 `Authorization: BEARER <token>` 可通过 shared parser 进入 JWT 校验并认证成功。
- [x] 3.2 补充或确认中间件测试覆盖无 Bearer 前缀与 `Bearer ` 空 token 仍返回 HTTP 401 且不执行后续 handler。
- [x] 3.3 对修改的 Go 文件执行 `gofmt`。
- [x] 3.4 在 `common/` 执行 `go test ./...`，确认 shared auth、middleware 和相关 common 测试通过。

## 4. Spec Compliance

- [x] 4.1 确认实现满足 `common-credentials` delta spec 中 required Bearer authorization parser 的格式错误、空 token 和大小写无关前缀语义。
- [x] 4.2 确认实现满足 `user-authentication` delta spec 中中间件复用 shared parser、错误响应兼容和认证上下文传播不变的要求。

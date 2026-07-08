## 1. OpenSpec 与范围确认

- [x] 1.1 确认 `migrate-auth-session-errors` 的 proposal、design、`auth-session-management` delta 和 `shared-platform-primitives` delta 已生成且可被 OpenSpec 识别。
- [x] 1.2 读取 apply 指令中的上下文文件，确认任务范围只覆盖 auth feature、`common/security/password` 的 KDF busy 错误表达、共享错误契约消费方式和本 change artifacts。

## 2. Password KDF busy 应用错误表达

- [x] 2.1 修改 `common/security/password`，将 `ErrPasswordKDFBusy` 表达为携带 `KindServiceUnavailable`、`Reason=password_kdf_busy`、`CodeServiceUnavailable` 和认证服务繁忙中文公开消息的应用错误，同时保持 `errors.Is(err, password.ErrPasswordKDFBusy)` 语义。
- [x] 2.2 更新 `common/security/password` 测试，覆盖 KDF busy 的 `errors.Is`、应用错误 `Kind`、`Reason`、`Code`、message 和 `response.Fail` 可直接渲染为 503，确认 Argon2id 参数、哈希编码、队列上限和并发上限不变。

## 3. 认证领域错误迁移

- [x] 3.1 修改 `user-service/internal/features/auth/domain/errors.go`，将无效凭据、用户状态拒绝、缺失会话、token 无效、refresh session 无效、强制改密 session 无效和撤销不完整错误定义为携带稳定 `Kind`、`Reason`、`Code`、中文公开 `Message` 的应用错误。
- [x] 3.2 调整 auth domain、credentials、sessions、validators 或 command 测试，使认证错误断言覆盖直接返回和包装后的 `errors.Is` 语义，以及必要的应用错误 `Reason` 分类。

## 4. Auth application 与 metrics 分类收敛

- [x] 4.1 调整 auth credentials、sessions、validators 和 command 中错误返回路径，确保 KDF busy、无效凭据、缺失 session/token、session mismatch、撤销不完整等失败直接返回可渲染应用错误。
- [x] 4.2 调整 login、refresh、logout metrics 分类逻辑和测试，确保仍可通过 `errors.Is` 或应用错误 `Reason` 识别 credential invalid、user status rejected、password KDF busy、refresh token invalid、refresh session invalid/mismatch、token version mismatch 和 revocation incomplete。

## 5. Auth HTTP 边界收敛

- [x] 5.1 修改 `user-service/internal/features/auth/transport/http/controller.go`，让登录、refresh、改密、退出当前会话和退出全部会话 use case 失败统一使用 `response.Fail(c, err)`，并保留强制改密成功分支现有 envelope。
- [x] 5.2 删除 `user-service/internal/features/auth/transport/http` 中仅用于认证错误翻译的 mapper 逻辑，确保不存在 `toAuthHTTPError` 或等价兼容函数。
- [x] 5.3 更新 auth HTTP transport 测试，覆盖无效凭据返回 401 unauthenticated、缺失或无效 session/token 返回 401 token invalid/unauthenticated、撤销不完整和 KDF busy 返回 503，且不依赖旧 mapper。

## 6. 验证与收尾

- [x] 6.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 6.2 运行 `rg "toAuthHTTPError" user-service/internal/features/auth` 并确认无命中。
- [x] 6.3 运行 `go test ./common/security/password/... ./user-service/internal/features/auth/...` 并确认通过。
- [x] 6.4 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构边界。
- [x] 6.5 将本次预期代码和 OpenSpec 变更加到暂存区，排除运行时文件 `AGENTS.md`、`CLAUDE.md`、`.multica/project/resources.json` 和 `.multica/**`。
- [x] 6.6 运行 `make lint` 和 `make verify`；如因本次范围外既有问题失败，记录失败命令和关键输出，不把未通过命令标记为完成。

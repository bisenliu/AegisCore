# Design

## Overview

本变更将 auth feature application 层调整为 command/use case 入口加稳定 component 包的结构：

```text
transport/http
  -> application/command
  -> application/{credentials,sessions,tokens,authctx,validators}
  -> application ports
  -> domain

infrastructure/postgres
  -> application ports
  -> domain

infrastructure/redis
  -> application ports
  -> domain
```

`application/command` 继续负责当前认证写侧流程：登录、刷新、强制改密、退出当前设备和退出全部设备。它不再承载凭证验证、token 签发解析和 session 生命周期的实现细节，而是调用同属 auth application 层的 component 包。

这不是把 auth 按单个 use case 拆成多个 Go package。Go package 是封装和依赖边界，不是文件夹分类标签。当前 use case 的复杂度仍适合保持扁平 command 包；真正增长的是 command 内部支撑组件，因此优先按稳定 component 拆分。

## Target Package Layout

```text
user-service/internal/features/auth/application/
  ports.go
  authctx/
    session.go
  command/
    dependencies.go
    login.go
    refresh_token.go
    change_password.go
    logout_current_session.go
    logout_all_sessions.go
    *_test.go
  credentials/
    verifier.go
    verifier_test.go
  sessions/
    lifecycle.go
    revocation.go
    lifecycle_test.go
  tokens/
    issuer.go
    result.go
    issuer_test.go
  validators/
    auth_validator.go
    session_policy.go
    token_version_validator.go
  query/
    README.md
```

`application/query` only keeps a README if there is no real query use case. Do not add empty handlers, empty services, empty DTOs, or placeholder interfaces for symmetry.

## Package Responsibilities

### application/command

Command owns use case entry points and command DTOs:

- `LoginUseCase` and `LoginCommand`
- `RefreshTokenUseCase` and `RefreshTokenCommand`
- `ChangePasswordUseCase` and `ChangePasswordCommand`
- `LogoutCurrentSessionUseCase`
- `LogoutAllSessionsUseCase`

Responsibilities:

- Validate transport-neutral command input through `application/validators`.
- Orchestrate credentials, tokens and sessions components.
- Preserve current error semantics and logging intent.
- Return transport-neutral application results.
- Expose narrow interfaces for HTTP controller and Fx wiring.

Command package may depend on:

- Standard library.
- `common/runtime/logger` if the use case itself logs business steps.
- `application/authctx`
- `application/credentials`
- `application/sessions`
- `application/tokens`
- `application/validators`
- `domain`
- `github.com/google/uuid` and `go.uber.org/zap` where needed.

Command package must not depend on Gin, HTTP binder, HTTP response, Ent, Redis client or SQL.

### application/credentials

Credentials component owns credential-oriented application behavior:

- Query user credentials through `application.UserCredentialStore`.
- Verify login passwords with `common/security/password`.
- Map missing login user and password mismatch to `authdomain.ErrInvalidCredentials`.
- Allow must-change-password users through login only to obtain a restricted token.
- Reject disabled or otherwise non-login statuses.
- For forced password change, load user by ID, check status, hash new password and call `UpdateCredentials`.
- Map user-not-found and unexpected repository errors as today.

This package can depend on application ports, auth domain, user domain, password helper and logger. It must not know about HTTP responses, Redis sessions, JWT signing, Gin, Ent or SQL.

### application/tokens

Tokens component owns JWT signing and parsing behavior used by auth application use cases:

- Preserve default access token TTL and refresh token TTL fallback.
- Sign access and refresh tokens with existing JWT service and claim values.
- Sign password-change tokens with existing subject and TTL behavior.
- Normalize optional Bearer input for refresh and password-change token parsing.
- Validate refresh token subject and user ID shape.
- Return `authdomain.ErrTokenInvalid` for invalid or unsupported tokens.
- Own `TokenResult` as a transport-neutral application DTO for login, refresh and password-change token responses.

This package can depend on `common/security/auth`, config, logger, auth domain and UUID. It must not persist sessions or update credentials.

### application/sessions

Sessions component owns authentication session lifecycle rules:

- Persist refresh sessions with expiry derived from refresh token TTL.
- Validate password-change token version against current token version.
- Validate refresh session existence, claim/session consistency and current token version.
- Rotate refresh sessions through the session store and map rejected rotation to `authdomain.ErrTokenInvalid`.
- Delete the current refresh session.
- Revoke all user sessions by incrementing token version, refreshing token version projection and deleting refresh sessions.
- Keep per-user active refresh session limit in application policy, passing it to the session store.

This package can depend on application ports, validators, auth domain, user domain, logger and UUID. It must not parse HTTP input, sign JWT tokens, verify passwords or import Redis client directly.

### application/authctx

Authctx owns small context helpers related to authenticated application session identity:

- Read user ID from `common/security/auth` context.
- Read session ID from `common/security/auth` context.
- Validate UUID shape where required by authenticated command use cases.
- Return `authdomain.ErrMissingSession` on missing or malformed context values.

This avoids introducing `application/common`, which would be too broad and likely to become a misc package.

### application/validators

Validators continue to own transport-neutral input and consistency validation:

- Login command input validation.
- Refresh token command input validation.
- Change-password command input validation.
- Token version match validation.
- Refresh session claim/session consistency validation.
- Session policy validation.

Validators must not import Gin, HTTP request/response DTO, HTTP response writer, Ent, Redis client or SQL.

### domain

Domain remains pure:

- `credential.go` owns `UserCredential`, credential status predicates and credential update domain input/result if they are pure data and rule methods.
- `session.go` owns `AuthSession`, `SessionRevocationResult` and pure session value semantics.
- `errors.go` owns auth domain errors such as invalid credentials, invalid token, missing session and session mismatch.

Domain must not import application packages, logger, config, Gin, Ent, Redis, password hash helper, JWT service or HTTP response envelope.

## Use Case Flows

### Login

`command.LoginUseCase` coordinates:

1. Validate `LoginCommand`.
2. Verify credentials through `credentials.Verifier`.
3. If password change is required, issue a password-change token through `tokens.Issuer` and do not create a refresh session.
4. Otherwise issue token pair through `tokens.Issuer`.
5. Create refresh session through `sessions.Lifecycle`.
6. Return `tokens.TokenResult`.

Existing behavior must be preserved:

- Invalid username/password maps to invalid credentials.
- Must-change-password users receive only a restricted access token.
- Normal login creates a refresh session using the same user ID, session ID and token version as the refresh token.
- TTL fallback behavior stays unchanged.

### Refresh Token

`command.RefreshTokenUseCase` coordinates:

1. Validate `RefreshTokenCommand`.
2. Parse refresh token through `tokens.Issuer`.
3. Validate refresh session through `sessions.Lifecycle`.
4. If rotation is disabled, issue a token pair for the existing session ID.
5. If rotation is enabled, issue a token pair for a new session ID and then rotate the session through `sessions.Lifecycle`.
6. Return `tokens.TokenResult`.

Existing behavior must be preserved:

- Non-rotation reuses the current session ID.
- Rotation failure does not return newly signed tokens.
- Signing failure does not mutate session state.
- Rejected rotation maps to invalid token.

### Change Password

`command.ChangePasswordUseCase` coordinates:

1. Validate `ChangePasswordCommand`.
2. Parse password-change token through `tokens.Issuer`.
3. Validate token version through `sessions.Lifecycle`.
4. Change credentials through `credentials.Verifier`.
5. Revoke user sessions at the new token version through `sessions.Lifecycle`.
6. Return `ChangePasswordResult{Changed: true}`.

Existing behavior must be preserved:

- Forced password change does not separately call `IncrementTokenVersion`.
- Credential update returns the new token version.
- Session revocation projection errors are logged but do not fail successful password change.

### Logout Current Session

`command.LogoutCurrentSessionUseCase` coordinates:

1. Read user ID and session ID through `authctx`.
2. Delete the current refresh session through `sessions.Lifecycle`.
3. Return `LogoutResult{LoggedOut: true}`.

Existing behavior must be preserved:

- Missing or malformed authenticated session maps to `authdomain.ErrMissingSession`.
- Token version is not incremented.
- Only the current refresh session is deleted.

### Logout All Sessions

`command.LogoutAllSessionsUseCase` coordinates:

1. Read user ID through `authctx`.
2. Revoke all user sessions through `sessions.Lifecycle`.
3. Return `LogoutResult{LoggedOut: true}`.

Existing behavior must be preserved:

- Missing or malformed user ID maps to `authdomain.ErrMissingSession`.
- User token version is incremented.
- Token version projection and refresh session deletion are attempted by the sessions component.

## Fx Wiring

Keep feature-level wiring in `user-service/internal/features/auth/fx.go` unless it becomes too large. Do not add an application-level Fx module only for symmetry.

Provider shape should remain explicit:

- Infrastructure providers implement `application.UserCredentialStore`, `application.UserTokenVersionStore` and `application.AuthSessionStore`.
- Component constructors provide credentials verifier, token issuer and session lifecycle.
- Command constructors provide narrow use case interfaces.
- HTTP controller depends on narrow command use case interfaces.

If a shared `UseCaseDeps` holder remains useful, keep it in `application/command/dependencies.go`, but it should hold component interfaces from `credentials`, `tokens` and `sessions`, not implement those components itself.

## Naming

Prefer business-semantic Go filenames:

- `login.go`
- `refresh_token.go`
- `change_password.go`
- `logout_current_session.go`
- `logout_all_sessions.go`
- `verifier.go`
- `issuer.go`
- `lifecycle.go`
- `revocation.go`

Avoid generic `command.go`, `handler.go`, `service.go`, `model.go` and `common.go` unless the file/package has a narrow and obvious local convention. In this feature, `handler` is especially easy to confuse with HTTP handler.

## Testing Strategy

Use focused tests by package:

- `application/credentials/verifier_test.go` for password verification, user state rejection and forced password update.
- `application/tokens/issuer_test.go` for token signing, parsing, Bearer normalization and TTL fallback.
- `application/sessions/lifecycle_test.go` for refresh session validation, rotation, token version fallback and revocation.
- `application/authctx/session_test.go` for context extraction and missing session errors.
- `application/command/*_test.go` for use case orchestration.
- `transport/http/controller_test.go` for HTTP DTO mapping and error rendering.

Use same-package tests when private helper coverage is important. Use external package tests only when testing exported API from a consumer viewpoint is more valuable.

## Documentation

Update:

- `AGENTS.md` auth feature structure and entry points.
- `docs/ARCHITECTURE.md` Feature-First Organization and dependency rules.
- `docs/DEVELOPMENT.md` if it describes adding auth application use cases.
- `docs/TESTING.md` if it references old auth application test locations.

Documentation must continue to state that this repository does not use OpenSpec/OPSX artifacts and must not reintroduce `openspec/` or `docs/opsx/`.

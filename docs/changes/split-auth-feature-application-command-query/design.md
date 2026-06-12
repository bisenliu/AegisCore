# Design

## Overview

本变更把 auth feature application 层从根部单体 service 调整为 command/use case 组织：

```text
transport/http
  -> application/command
  -> application
  -> domain

infrastructure/postgres
  -> application ports
  -> domain

infrastructure/redis
  -> application ports
  -> domain
```

`application/command` 负责所有当前认证写侧流程：登录、刷新、强制改密、退出当前设备和退出全部设备。`application/validators` 负责不依赖 HTTP 的输入前置校验。`application/tokenversion` 负责 access token 中间件和 command session lifecycle 共用的 token version 撤销校验策略。`application` 根包继续拥有稳定消费侧契约：credential、token version 和 session store ports。

当前 auth feature 没有只读 application 查询场景。本变更不创建空 `query` 包；未来如果新增查询当前会话、列出设备等真实读侧用例，再引入 `application/query`。

## Target Package Layout

```text
user-service/internal/features/auth/application/
  command/
    dependencies.go
    login.go
    refresh.go
    change_password.go
    logout_current_session.go
    logout_all_sessions.go
    credentials.go
    sessions.go
    tokens.go
    authenticated_session.go
    *_test.go
  validators/
    auth_validator.go
    auth_validator_test.go
  tokenversion/
    validator.go
  ports.go
```

Possible root package files after migration:

- `ports.go` keeps `UserCredentialStore`, `UserTokenVersionStore`, `AuthSessionStore` and any small component-facing interfaces that must be shared across command use cases.
- `tokenversion/validator.go` keeps the token version validator because it is consumed by service-level auth middleware and reused by command session lifecycle, but is not a command input validator.
- `service.go`, `commands.go` and `result.go` should be deleted or reduced to narrow compatibility only if implementation discovers a temporary compile bridge is unavoidable. The final shape should not keep a monolithic root `AuthService`.

Result types should live with their owning use cases. `TokenResult` can live in `command/tokens.go` or alongside login/refresh if shared by both; `ChangePasswordResult` should live near `change_password.go`; `LogoutResult` should live near logout use cases.

## Command Use Cases

The command package owns explicit use case interfaces. Names can follow local style, but the controller dependency surface should remain narrow:

```go
type LoginUseCase interface {
    Login(ctx context.Context, cmd LoginCommand) (*TokenResult, error)
}

type RefreshTokenUseCase interface {
    Refresh(ctx context.Context, cmd RefreshTokenCommand) (*TokenResult, error)
}

type ChangePasswordUseCase interface {
    ChangePassword(ctx context.Context, cmd ChangePasswordCommand) (*ChangePasswordResult, error)
}

type LogoutCurrentSessionUseCase interface {
    LogoutCurrentSession(ctx context.Context) (*LogoutResult, error)
}

type LogoutAllSessionsUseCase interface {
    LogoutAllSessions(ctx context.Context) (*LogoutResult, error)
}
```

Equivalent names are acceptable if they make controller wiring clear and avoid a catch-all auth service interface.

Each use case should own its input command type:

- `LoginCommand`
- `RefreshTokenCommand`
- `ChangePasswordCommand`
- `LogoutCurrentSessionCommand` only if a command struct is useful; otherwise `context.Context` is enough because authenticated user/session values already live in context.
- `LogoutAllSessionsCommand` only if a command struct is useful; otherwise `context.Context` is enough.

The command package may import:

- Standard library packages needed by the use case.
- `common/runtime/config`
- `common/runtime/logger`
- `common/security/auth`
- `common/security/password`
- `auth/application`
- `auth/application/validators`
- `auth/domain`
- `user/domain`
- `github.com/google/uuid`
- `go.uber.org/fx`
- `go.uber.org/zap`

The command package must not import Gin, HTTP binder, HTTP response, Ent, Redis client or SQL.

## Shared Command Dependencies

Most auth commands share credential, token and session collaborators. To avoid rebuilding a hidden monolith, introduce a small dependency holder in `application/command/dependencies.go`:

```go
type UseCaseDepsParams struct {
    fx.In

    Credentials   application.UserCredentialStore
    TokenVersions application.UserTokenVersionStore
    Sessions      application.AuthSessionStore
    JWT           *commonauth.JWTService
    Config        *config.Config
}

type UseCaseDeps struct {
    credentials          CredentialVerifier
    tokens               AuthTokenIssuer
    sessions             AuthSessionLifecycle
    refreshTokenRotation bool
}
```

`NewUseCaseDeps` can construct the three internal components:

- `CredentialVerifier` from `UserCredentialStore`
- `AuthTokenIssuer` from JWT/config
- `AuthSessionLifecycle` from token version and session stores

Individual use case constructors then accept `*UseCaseDeps` and expose only their own method. This keeps credential/token/session setup centralized without reintroducing an `AuthService` that owns every endpoint.

If implementation chooses not to use a shared holder, constructors must still avoid duplicated component logic and keep the use case interfaces narrow.

## Login Command

`login.go` owns username/password authentication.

Responsibilities:

- Validate transport-neutral login command input through `validators`.
- Log login attempts with existing field names.
- Verify password through the credential component.
- Preserve disabled-user and invalid-credential semantics.
- If the user must change password, issue a password-change token only:
  - no refresh token
  - `PasswordChangeRequired` set
  - no normal session persisted
- Otherwise issue access and refresh token pair.
- Create a refresh session with the same user ID, session ID and token version used in the refresh token.
- Preserve default and explicit TTL behavior.

## Refresh Command

`refresh.go` owns refresh token validation and token renewal.

Responsibilities:

- Validate transport-neutral refresh command input through `validators`.
- Parse refresh token through the token component.
- Validate session existence, session/claims consistency and current token version through the session component.
- Preserve non-rotation behavior:
  - reuse the current session ID
  - issue a new token pair
  - keep the existing refresh session record valid
- Preserve rotation behavior:
  - sign tokens for a new session ID before mutating Redis
  - atomically rotate old session to new session
  - do not return newly signed tokens if rotation fails
  - keep old session when signing fails or new session creation fails

## Change Password Command

`change_password.go` owns forced password-change flow.

Responsibilities:

- Validate transport-neutral change-password command input through `validators`.
- Parse and validate password-change token.
- Confirm token version is still current.
- Update credential password hash and restore user status to normal through the credential component.
- Do not call `IncrementTokenVersion` during forced password change; the credential update already returns the new token version.
- Refresh token version projection and delete all refresh sessions at the returned version.
- Preserve existing behavior where projection/session revocation errors are logged but do not fail a successful password change.
- Return `ChangePasswordResult{Changed: true}` on success.

## Logout Commands

`logout_current_session.go` owns current-device logout.

Responsibilities:

- Read authenticated user ID and session ID from `common/security/auth` context.
- Reject missing or malformed context values with `authdomain.ErrMissingSession`.
- Delete only the current refresh session.
- Return `LogoutResult{LoggedOut: true}` on success.

`logout_all_sessions.go` owns all-device logout.

Responsibilities:

- Read authenticated user ID from context.
- Reject missing or malformed user ID with `authdomain.ErrMissingSession`.
- Increment the user's token version through the session lifecycle.
- Refresh token version cache and delete all user refresh sessions.
- Preserve existing behavior where projection cleanup errors are logged by the session lifecycle but do not fail the revocation result.
- Return `LogoutResult{LoggedOut: true}` on success.

The helper that reads authenticated session context should live in `command/authenticated_session.go`, not in HTTP transport.

## Credential Component

`command/credentials.go` keeps credential rules used by auth commands.

Responsibilities:

- Query users through `application.UserCredentialStore`.
- Map missing login user to `authdomain.ErrInvalidCredentials`.
- Verify passwords with `common/security/password`.
- Allow must-change-password users through login only to obtain a restricted token.
- Reject disabled or otherwise non-login statuses as `authdomain.ErrInvalidCredentials`.
- For forced password change, load the user by ID, ensure status allows password change, hash the new password and call `UpdateCredentials`.
- Map user-not-found and unexpected repository errors as today.

The credential component must not know about HTTP responses, Redis sessions or JWT signing.

## Token Component

`command/tokens.go` keeps JWT signing and parsing behavior used by auth commands.

Responsibilities:

- Preserve `defaultAccessTokenTTL` and `defaultRefreshTokenTTL`.
- Sign access and refresh tokens with the existing JWT service and claim values.
- Sign password-change tokens with existing subject and TTL behavior.
- Normalize optional Bearer input for refresh and password-change token parsing.
- Validate refresh token subject and user ID shape.
- Return `authdomain.ErrTokenInvalid` for invalid or unsupported tokens.

Token result DTOs returned to controller should be command-layer result types, not root application `result.go` types.

## Session Component

`command/sessions.go` keeps authentication session lifecycle rules used by commands.

Responsibilities:

- Persist refresh sessions with expiry derived from refresh token TTL.
- Validate password-change token version against the current token version.
- Validate refresh session existence, claim/session consistency and current token version.
- Rotate refresh sessions through the Redis adapter and map rejected rotation to `authdomain.ErrTokenInvalid`.
- Delete a single session.
- Revoke all user sessions by incrementing token version, updating projection cache and deleting all refresh sessions.

`application/tokenversion/validator.go` should provide `NewValidator` for access-token middleware and `Current` for command session lifecycle. This package owns the shared Redis cache, PostgreSQL fallback and cache backfill strategy. Do not duplicate cache/database fallback semantics in `command/sessions.go`.

## Validators Layer

`application/validators` owns transport-neutral application input checks. HTTP DTO binding, JSON field labels and HTTP-specific error rendering stay in `transport/http`.

Initial validators should be small and preserve existing error semantics:

- `ValidateLoginCommand(username, password)` returns `authdomain.ErrInvalidCredentials` when trimmed values are blank.
- `ValidateRefreshToken(token)` returns `authdomain.ErrTokenInvalid` when the token is blank or effectively Bearer-only.
- `ValidateChangePassword(token, newPassword)` returns `authdomain.ErrTokenInvalid` for missing token and an application/domain-level invalid-password error for missing password if one already exists; otherwise leave password blank handling in HTTP validation and avoid inventing a new public error code.

Validators should not:

- Import Gin or HTTP DTO packages.
- Return response envelope errors.
- Duplicate all Gin validation tags.
- Call stores, hash passwords, sign tokens or mutate sessions.

## Root Application Contracts

`application/ports.go` remains the consumer-owned boundary:

```go
type UserCredentialStore interface {
    GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error)
    GetCredentialByUserID(ctx context.Context, userID uuid.UUID) (*authdomain.UserCredential, error)
    UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error)
}

type UserTokenVersionStore interface {
    GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
    IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
}

type AuthSessionStore interface {
    GetCachedTokenVersion(ctx context.Context, userID string) (int64, error)
    CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
    DeleteCachedTokenVersion(ctx context.Context, userID string) error
    CreateSession(ctx context.Context, session authdomain.AuthSession, ttl time.Duration) error
    RotateSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration) error
    GetSession(ctx context.Context, userID string, sessionID string) (authdomain.AuthSession, error)
    DeleteSession(ctx context.Context, userID string, sessionID string) error
    DeleteAllUserSessions(ctx context.Context, userID string) error
}
```

Infrastructure adapters continue to implement these interfaces. Do not move ports into infrastructure and do not define larger interfaces for adapter convenience.

## Controller Wiring

`transport/http.AuthController` should depend on explicit use cases:

```go
type AuthController struct {
    login          command.LoginUseCase
    refresh        command.RefreshTokenUseCase
    changePassword command.ChangePasswordUseCase
    logoutCurrent  command.LogoutCurrentSessionUseCase
    logoutAll      command.LogoutAllSessionsUseCase
    validator      *commonvalidation.Validator
}
```

The constructor may use an `fx.In` params struct if that keeps wiring readable.

Controller methods should continue to:

- Bind HTTP DTOs with `common/http/binding`.
- Normalize HTTP request values in `transport/http/validation.go`.
- Strip Bearer prefixes in HTTP normalization where that behavior already exists.
- Construct command-layer input types.
- Call the relevant use case.
- Map domain/application errors through `toAuthHTTPError`.
- Render unchanged HTTP response DTOs through `common/http/response`.

The controller must not pass HTTP request/response DTOs into command use cases.

## Mapper Updates

`transport/http/mapper.go` should import command result types instead of root application result types.

Expected mappings remain behaviorally identical:

- `TokenResult` to `TokenResponse`
- `ChangePasswordResult` to `ChangePasswordResponse`
- `LogoutResult` to `LogoutResponse`

Do not change JSON field names, omit-empty behavior, status codes or response envelope shape.

## Fx Wiring

`user-service/internal/features/auth/fx.go` should provide command use cases explicitly.

Expected composition:

```go
var Module = fx.Module("feature-auth",
    fx.Provide(
        fx.Annotate(
            authpostgres.NewCredentialStore,
            fx.As(new(authapplication.UserCredentialStore)),
        ),
        fx.Annotate(
            authpostgres.NewCredentialStore,
            fx.As(new(authapplication.UserTokenVersionStore)),
        ),
        authdomain.NewRedisKeyBuilder,
        fx.Annotate(
            authredis.NewSessionStore,
            fx.As(new(authapplication.AuthSessionStore)),
        ),
        authtokenversion.NewValidator,
        authcommand.NewUseCaseDeps,
        authcommand.NewLoginUseCase,
        authcommand.NewRefreshTokenUseCase,
        authcommand.NewChangePasswordUseCase,
        authcommand.NewLogoutCurrentSessionUseCase,
        authcommand.NewLogoutAllSessionsUseCase,
        authhttp.NewAuthController,
    ),
)
```

Use `fx.As` where constructors return concrete types but the controller asks for interfaces. Keep service-level provider code in `internal/providers`; feature-local business wiring stays in auth `fx.go`.

## Testing Strategy

Move and refocus current tests:

- Login tests move from `application/service_test.go` to `application/command/login_test.go`.
- Refresh tests move to `application/command/refresh_test.go`.
- Change-password tests move to `application/command/change_password_test.go`.
- Logout current/all tests move to `application/command/logout_*_test.go`.
- Credential, token and session component tests move or stay in command tests according to their package ownership.
- Token version validator behavior should be covered through `application/tokenversion` usage in command/session lifecycle tests and Redis adapter tests.
- Validator-specific tests live in `application/validators`.
- Controller tests should replace `stubAuthService` with small stubs for the explicit use case dependencies.

Controller tests should continue to assert:

- Login request normalization and command mapping.
- Change-password header/body normalization and command mapping.
- Refresh token normalization and command mapping.
- Logout endpoint success and error mapping.
- Domain/application error to HTTP error mapping.
- Response envelope and response DTO shape.

## Documentation Updates

Update long-lived docs:

- `AGENTS.md`
  - Repository Shape should mention auth `application/command`, `application/validators` and `application/tokenversion`.
  - Key Entry Points should point auth command services to the new files.
  - Repository Rules should describe auth ports staying in `application/ports.go`.
- `docs/ARCHITECTURE.md`
  - HTTP Request Flow should note auth command use cases in addition to user command/query use cases.
  - Feature-first organization should describe auth command/use case subdivision.
- `docs/DEVELOPMENT.md`
  - Guidance for new auth capabilities should point write/session use cases to `application/command`.
- `docs/TESTING.md`
  - If it names old auth application service tests, update paths to command/component tests.

Historical `docs/changes/*` records can remain as historical context unless they are part of this change.

## Behavior Preservation

The implementation must preserve:

- Public auth HTTP paths and methods.
- Request/response DTO fields.
- Response envelope shape.
- Status codes and error codes.
- Login username/password normalization behavior.
- Refresh token Bearer compatibility.
- Password-change token Bearer compatibility.
- JWT subject, claims, token type and TTL fallback behavior.
- Refresh token rotation semantics for enabled and disabled rotation.
- Session creation, rotation, deletion and revocation behavior.
- Token version cache/database fallback behavior.
- Redis key builder and Redis key semantics.

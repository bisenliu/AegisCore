## Implementation Tasks

- [x] Add permission application authorization boundary
   - Create `user-service/internal/features/permission/application/authorization/`.
   - Define `Authorizer` with `Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error)`.
   - Add a default service that parses `userID` as UUID and delegates to the existing Casbin engine through a small internal interface.
   - Ensure invalid or empty user IDs fail closed and do not trigger database access.

- [x] Wire authorization service in permission Fx module
   - Provide the authorization service from `permission/fx.go`.
   - Keep Casbin engine in `infrastructure/casbin`; do not make HTTP transport depend directly on infrastructure details.

- [x] Add Gin RBAC middleware in permission HTTP transport
   - Create middleware code under `user-service/internal/features/permission/transport/http/`.
   - Read user ID from `c.Get(auth.UserIDKey)` first, then `auth.UserIDFromContext(c.Request.Context())`.
   - Use `c.FullPath()` as path template and reject empty route templates with 403.
   - Use `c.Request.Method` as action.
   - Return unified response envelope for 401, 403, and internal errors.
   - Add explicit whitelist support using method + path template.
   - Bypass Casbin for `OPTIONS`.
   - Keep log messages in English and never log password, token, Authorization header, Cookie, or raw request body.

- [x] Wire RBAC into route registration
   - Add `Authorizer` to router route params and provider route params.
   - In `registerV1Routes`, mount RBAC on the `authorized` subgroup after JWT middleware.
   - Keep public auth routes outside JWT and RBAC.
   - Keep protected auth session routes after JWT but outside RBAC unless an explicit requirement later changes that.
   - Keep health and Swagger outside RBAC.

- [x] Add middleware and authorization tests
   - Cover missing/invalid user ID returning 401.
   - Cover denied authorization returning 403 envelope.
   - Cover authorizer error returning internal error envelope.
   - Cover allowed authorization entering the next handler.
   - Assert middleware passes `c.FullPath()` rather than raw URL path.
   - Assert whitelist and `OPTIONS` bypass do not call authorizer.

- [x] Add router/provider tests
   - Assert public auth routes do not enter RBAC.
   - Assert business protected routes execute JWT before RBAC.
   - Assert RBAC is mounted before user, role, and permission business controllers.

- [x] Verify no per-request database access
   - Keep authorization request path limited to middleware -> authorization service -> in-memory Casbin engine.
   - Do not call Ent/PostgreSQL/Redis from middleware or per-request authorization service logic.

- [x] Run validation
   - Run `gofmt -w` for changed Go files.
   - Run `go test ./...` from `user-service/`.

## Acceptance Checklist

- JWT authentication succeeds before RBAC authorization runs.
- Public auth routes do not enter RBAC authorization.
- Health, Swagger, and `OPTIONS` are not blocked by Casbin.
- Protected business routes return unified 403 response envelope when authorization denies access.
- RBAC middleware uses `c.FullPath()` as Casbin object.
- RBAC middleware uses `c.Request.Method` as Casbin action.
- Authorization service normalizes user subject through the existing `user:<user_uuid>` Casbin path.
- Role policy relations continue to use `role:<role_uuid>` and do not rely on `roles.code`.
- Each request avoids database access for authorization decisions.
- Logs remain English and contain no secrets.
- `user-service` `go test ./...` passes.

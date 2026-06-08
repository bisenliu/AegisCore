## 1. Controller Rename

- [x] 1.1 Rename `UserController.GetByID` to `UserController.GetByUserID` and update the godoc comment prefix to match the new method name.
- [x] 1.2 Update `user-services/internal/router/users.go` to register `GET /:user_id` with `userController.GetByUserID`.
- [x] 1.3 Search workspace references for `GetByID` and `UserController.GetByID`, then update only references related to this external `user_id` query handler.

## 2. Specification Updates

- [x] 2.1 Update `openspec/specs/project-naming-consistency/spec.md` with the archived requirement that user query controller handler naming must include `UserID`.
- [x] 2.2 Update `openspec/specs/user-profile-query/spec.md` so unauthenticated query requests are specified as not entering `UserController.GetByUserID`.
- [x] 2.3 Update `openspec/specs/http-service-runtime/spec.md` so user route bindings reference `UserController.GetByUserID`.

## 3. Verification

- [x] 3.1 Run `gofmt -w` on modified Go files.
- [x] 3.2 Run `go test ./...` in `user-services/` to verify the handler rename and route registration compile.
- [x] 3.3 Confirm the route path, Swagger annotations, response envelope, error codes, Ent schema and migrations are unchanged.

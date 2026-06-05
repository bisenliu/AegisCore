## 1. Schema Package Rename

- [x] 1.1 Rename `user-services/ent/schema/user/` to `user-services/ent/schema/userschema/`.
- [x] 1.2 Update the package declaration in the moved schema source file from `user` to `userschema`.
- [x] 1.3 Update `user-services/ent/schema/user.go` to import `github.com/aegiscore/user-services/ent/schema/userschema` and call `userschema.Fields()` / `userschema.Indexes()`.
- [x] 1.4 Confirm no application code imports `github.com/aegiscore/user-services/ent/schema/user` after the rename.

## 2. Generated Code Workflow

- [x] 2.1 Run `gofmt` on modified Go schema source files.
- [x] 2.2 In `user-services`, run `go generate ./ent` to verify Ent codegen reads the renamed schema subpackage.
- [x] 2.3 Review generated-code changes and confirm no manual edits were made under `user-services/ent/`.

## 3. Migration Safety

- [x] 3.1 Confirm `User` fields, indexes, comments, defaults and constraints are unchanged after the package rename.
- [x] 3.2 Confirm `user-services/migrations/` and `user-services/migrations/atlas.sum` remain unchanged because this is not a database schema change.

## 4. Verification

- [x] 4.1 Run `go test ./...` in `user-services`.
- [x] 4.2 Run `go test ./...` in `common` if workspace-level dependencies were touched or generated code affects shared compilation.
- [x] 4.3 Run `openspec status --change "clarify-ent-user-schema-package"` and confirm the change is apply-ready.

## 1. Response Contract

- [x] 1.1 Update `common/contract/response/pagination.go` to keep `DefaultPageSize=10`, add `MaxPageSize=100`, and redefine `Pagination` as `page_size`、`next_cursor`、`has_next`.
- [x] 1.2 Remove old `PaginationQuery`、`NormalizePagination(page, pageSize)`、`Page`、`Offset`、`Total`、`TotalPages` and any associated page/count calculations.
- [x] 1.3 Add `NormalizePageSize(pageSize int) int`, update `NewPagination(pageSize, nextCursor, hasNext)`, and preserve `NewPaginatedData` nil items as empty array behavior.
- [x] 1.4 Update common response tests to assert default page size 10, max page size 100, keyset pagination fields only, and absence of page/offset/total/total_pages semantics.

## 2. User API And HTTP Layer

- [x] 2.1 Update `ListUsersRequest` to remove `Page` and `Offset`, and keep `Cursor`、`PageSize`、`Limit`、`Nickname`、`Username`、`Status`.
- [x] 2.2 Update `NormalizeListUsers` to trim `Cursor`、`Nickname`、`Username`, normalize `PageSize` with `response.NormalizePageSize`, and set `Limit = PageSize`.
- [x] 2.3 Add `ParseListCursor` or equivalent feature-local validation to parse non-empty cursor as UUID and return HTTP 400 unified failure for invalid cursor.
- [x] 2.4 Update `UserController.ListUsers` to remove page handling, parse cursor, and pass `Cursor *uuid.UUID`、`PageSize`、`Limit`、filters into `userapp.ListUsersQuery`.
- [x] 2.5 Update user list HTTP/controller tests to remove page/total assertions and add invalid cursor 400, default page size, max page size, and response pagination field assertions.

## 3. User App Layer

- [x] 3.1 Update `ListUsersQuery` with `Cursor *uuid.UUID`、`PageSize`、`Limit`、`Nickname`、`Username`、`Status`.
- [x] 3.2 Update `ListUsersInput` with `AfterUserID *uuid.UUID`、`Limit`、`Nickname`、`Username`、`Status`.
- [x] 3.3 Change `UserProfileStore.ListUsers` to return `([]userdomain.User, bool, error)` where bool is `hasNext`.
- [x] 3.4 Update `ListUsersResult` to contain `Items`、`PageSize`、`NextCursor`、`HasNext`.
- [x] 3.5 Update service logic to call the store with `AfterUserID`, compute `nextCursor` from the last returned user only when `hasNext && len(users) > 0`, and wrap unexpected errors safely.
- [x] 3.6 Update app/service tests and mocks to use hasNext instead of total count and to verify next cursor generation behavior.

## 4. PostgreSQL Infra

- [x] 4.1 Update `infra/postgres/user_store.go` `ListUsers` to remove `Count(ctx)`, `Offset(...)`, and total return value.
- [x] 4.2 Add `entuser.UserIDGT(*input.AfterUserID)` when cursor is present, keep `deleted_at IS NULL` and existing filter predicates, order by `entuser.ByUserID()`, and query `Limit(input.Limit + 1)`.
- [x] 4.3 Trim extra row when `len(entUsers) > input.Limit`, return `hasNext=true`, and map remaining Ent users to domain models.
- [x] 4.4 Update infra tests to assert no Count, no Offset, `user_id > cursor`, `user_id ASC`, and `Limit + 1` hasNext behavior.

## 5. Mapping And Documentation

- [x] 5.1 Update user response mapper so `toUserListResponse` returns `response.NewPaginatedData(items, response.NewPagination(result.PageSize, result.NextCursor, result.HasNext))`.
- [x] 5.2 Update `user-services/internal/features/user/api/doc.go` so `UserListResponseDoc` keeps `response.Pagination` while documenting `page_size`、`next_cursor`、`has_next`.
- [x] 5.3 Update Swagger annotations for `GET /api/v1/users` to remove `page` and old total semantics, add `cursor`, and describe keyset pagination fields.
- [x] 5.4 Regenerate Swagger docs if the repository workflow requires generated `user-services/docs` artifacts to be committed.

## 6. Schema And Migration

- [x] 6.1 Update Ent `User` schema indexes with `index.Fields("deleted_at", "user_id")` and `index.Fields("status", "deleted_at", "user_id")`.
- [x] 6.2 Run `go generate ./ent` from `user-services/` and verify generated Ent code reflects index changes without manual edits under generated files.
- [x] 6.3 Run `./scripts/migrate-diff.sh user-list-keyset-pagination` from `user-services/` to generate Atlas SQL migration for the new indexes.
- [x] 6.4 Review generated SQL for index names and PostgreSQL suitability, run Atlas hash if SQL is manually adjusted, and keep `user-services/migrations/atlas.sum` synchronized.
- [x] 6.5 Run `./scripts/migrate-validate.sh` from `user-services/` to validate migration directory integrity.

## 7. Repository-Wide Cleanup And Verification

- [x] 7.1 Search the repository for old pagination symbols and JSON fields, then remove or update all `PaginationQuery`、`NormalizePagination`、`Page`、`Offset`、`Total`、`TotalPages` references.
- [x] 7.2 Update all user list tests to delete page/offset/total/total_pages assertions and add coverage for `data.pagination` containing only `page_size`、`next_cursor`、`has_next`.
- [x] 7.3 Run `gofmt` on changed Go files.
- [x] 7.4 Run `go test ./...` from `common/`.
- [x] 7.5 Run `go test ./...` from `user-services/`.
- [x] 7.6 Review final diff to confirm no generated code was hand-edited and no old page/offset/count behavior remains.

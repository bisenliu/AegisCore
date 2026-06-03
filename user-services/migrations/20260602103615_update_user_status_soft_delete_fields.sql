-- Modify "users" table while preserving existing user data.
ALTER TABLE "users" RENAME COLUMN "name" TO "nickname";
ALTER TABLE "users" RENAME COLUMN "password" TO "password_hash";
ALTER TABLE "users" ADD COLUMN "status" bigint, ADD COLUMN "deleted_at" bigint NULL;

UPDATE "users"
SET "status" = CASE WHEN "active" THEN 100 ELSE 200 END
WHERE "status" IS NULL;

ALTER TABLE "users" ALTER COLUMN "status" SET DEFAULT 100, ALTER COLUMN "status" SET NOT NULL, DROP COLUMN "active";

-- Create index "user_deleted_at" to table: "users"
CREATE INDEX "user_deleted_at" ON "users" ("deleted_at");
-- Create index "user_nickname" to table: "users"
CREATE INDEX "user_nickname" ON "users" ("nickname");
-- Create index "user_status" to table: "users"
CREATE INDEX "user_status" ON "users" ("status");

COMMENT ON COLUMN "users"."id" IS '用户ID';
COMMENT ON COLUMN "users"."user_id" IS '外部用户ID';
COMMENT ON COLUMN "users"."nickname" IS '用户昵称';
COMMENT ON COLUMN "users"."username" IS '用户名';
COMMENT ON COLUMN "users"."password_hash" IS '密码哈希';
COMMENT ON COLUMN "users"."token_version" IS '认证令牌版本';
COMMENT ON COLUMN "users"."status" IS '用户状态：100 正常，200 冻结/停用，300 必须修改密码';
COMMENT ON COLUMN "users"."deleted_at" IS '软删除时间戳毫秒，NULL 表示未删除';
COMMENT ON COLUMN "users"."created_at" IS '创建时间戳毫秒';
COMMENT ON COLUMN "users"."updated_at" IS '更新时间戳毫秒';

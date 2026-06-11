-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "user_id" uuid, ADD COLUMN "username" character varying;

-- Backfill existing rows before enforcing NOT NULL. New users receive UUIDv7 from the service.
UPDATE "users"
SET
  "user_id" = ('018f0000-0000-7000-8000-' || lpad("id"::text, 12, '0'))::uuid,
  "username" = CASE
    WHEN split_part("email", '@', 1) = '' THEN 'user_' || "id"::text
    ELSE lower(regexp_replace(split_part("email", '@', 1), '[^a-zA-Z0-9_.-]', '_', 'g')) || '_' || "id"::text
  END
WHERE "user_id" IS NULL OR "username" IS NULL;

ALTER TABLE "users" ALTER COLUMN "user_id" SET NOT NULL, ALTER COLUMN "username" SET NOT NULL, DROP COLUMN "email";

-- Create index "users_user_id_key" to table: "users"
CREATE UNIQUE INDEX "users_user_id_key" ON "users" ("user_id");
-- Create index "users_username_key" to table: "users"
CREATE UNIQUE INDEX "users_username_key" ON "users" ("username");

COMMENT ON COLUMN "users"."id" IS '用户ID';
COMMENT ON COLUMN "users"."user_id" IS '外部用户ID';
COMMENT ON COLUMN "users"."name" IS '用户昵称';
COMMENT ON COLUMN "users"."username" IS '用户名';
COMMENT ON COLUMN "users"."password" IS '密码哈希';
COMMENT ON COLUMN "users"."token_version" IS '认证令牌版本';
COMMENT ON COLUMN "users"."active" IS '是否启用';
COMMENT ON COLUMN "users"."created_at" IS '创建时间戳毫秒';
COMMENT ON COLUMN "users"."updated_at" IS '更新时间戳毫秒';

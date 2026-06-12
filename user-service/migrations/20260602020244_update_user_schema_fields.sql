-- Modify "users" table
ALTER TABLE "users"
  ADD COLUMN "password" character varying NOT NULL DEFAULT '__migration_required__',
  ALTER COLUMN "created_at" TYPE bigint USING (EXTRACT(EPOCH FROM "created_at") * 1000)::bigint,
  ALTER COLUMN "updated_at" TYPE bigint USING (EXTRACT(EPOCH FROM "updated_at") * 1000)::bigint;
ALTER TABLE "users" ALTER COLUMN "password" DROP DEFAULT;
COMMENT ON COLUMN "users"."id" IS '用户ID';
COMMENT ON COLUMN "users"."name" IS '用户名';
COMMENT ON COLUMN "users"."email" IS '邮箱';
COMMENT ON COLUMN "users"."password" IS '密码';
COMMENT ON COLUMN "users"."active" IS '是否启用';
COMMENT ON COLUMN "users"."created_at" IS '创建时间戳毫秒';
COMMENT ON COLUMN "users"."updated_at" IS '更新时间戳毫秒';

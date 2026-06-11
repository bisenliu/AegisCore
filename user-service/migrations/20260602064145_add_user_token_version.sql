-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "token_version" bigint NOT NULL DEFAULT 1;
COMMENT ON COLUMN "users"."token_version" IS '认证令牌版本';

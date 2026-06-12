-- Create index "user_deleted_at_user_id" to table: "users"
CREATE INDEX "user_deleted_at_user_id" ON "users" ("deleted_at", "user_id");
-- Create index "user_status_deleted_at_user_id" to table: "users"
CREATE INDEX "user_status_deleted_at_user_id" ON "users" ("status", "deleted_at", "user_id");

-- Create index "permission_active_permission_id" to table: "permissions"
CREATE INDEX "permission_active_permission_id" ON "permissions" ("active", "permission_id");
-- Create index "permission_http_method_permission_id" to table: "permissions"
CREATE INDEX "permission_http_method_permission_id" ON "permissions" ("http_method", "permission_id");
-- Create index "permission_is_system_permission_id" to table: "permissions"
CREATE INDEX "permission_is_system_permission_id" ON "permissions" ("is_system", "permission_id");
-- Create index "permission_module_permission_id" to table: "permissions"
CREATE INDEX "permission_module_permission_id" ON "permissions" ("module", "permission_id");
-- Create index "rolepermission_permission_id_role_id" to table: "role_permissions"
CREATE INDEX "rolepermission_permission_id_role_id" ON "role_permissions" ("permission_id", "role_id");
-- Create index "role_active_role_id" to table: "roles"
CREATE INDEX "role_active_role_id" ON "roles" ("active", "role_id");
-- Create index "role_is_system_role_id" to table: "roles"
CREATE INDEX "role_is_system_role_id" ON "roles" ("is_system", "role_id");
-- Create index "userrole_role_id_user_id" to table: "user_roles"
CREATE INDEX "userrole_role_id_user_id" ON "user_roles" ("role_id", "user_id");
-- Drop index "user_nickname" from table: "users"
DROP INDEX "user_nickname";
-- Create index "users_nickname_trgm" to table: "users"
CREATE INDEX "users_nickname_trgm" ON "users" USING GIN ("nickname" gin_trgm_ops);

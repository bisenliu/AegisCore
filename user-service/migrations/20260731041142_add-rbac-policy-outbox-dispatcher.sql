-- Modify "rbac_policy_outbox_events" table
ALTER TABLE "rbac_policy_outbox_events" ADD COLUMN "claim_token" uuid NULL, ADD COLUMN "claimed_until" bigint NULL;
-- Create index "rbacpolicyoutboxevent_status_claimed_until_revision" to table: "rbac_policy_outbox_events"
CREATE INDEX "rbacpolicyoutboxevent_status_claimed_until_revision" ON "rbac_policy_outbox_events" ("status", "claimed_until", "revision");
-- Set comment to column: "claim_token" on table: "rbac_policy_outbox_events"
COMMENT ON COLUMN "rbac_policy_outbox_events"."claim_token" IS '当前 dispatcher claim token';
-- Set comment to column: "claimed_until" on table: "rbac_policy_outbox_events"
COMMENT ON COLUMN "rbac_policy_outbox_events"."claimed_until" IS '当前 claim lease 截止时间戳毫秒';

-- Normalize existing usernames to the lowercase global-unique policy.
-- This migration intentionally fails before mutation if lowercase normalization would
-- make two existing users share the same username, including soft-deleted rows.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM (
      SELECT lower("username") AS normalized_username
      FROM "users"
      GROUP BY lower("username")
      HAVING count(*) > 1
    ) AS duplicate_usernames
  ) THEN
    RAISE EXCEPTION 'cannot normalize users.username to lowercase: duplicate usernames exist after lowercase normalization';
  END IF;
END $$;

UPDATE "users"
SET "username" = lower("username")
WHERE "username" <> lower("username");

-- users_username_key remains a full-table UNIQUE(username) index, so soft-deleted
-- users continue to reserve their usernames.

-- add the columns the ntfy payment approval uses to a database created before
-- that feature
--
-- without them every card tap fails, because a payment record cannot be
-- inserted
--
-- this is safe to run against a live database and safe to run more than once
--
-- $ psql card_db -f sql/migrate_ntfy.sql

\c card_db;

ALTER TABLE card_payments ADD COLUMN IF NOT EXISTS ntfy_flag CHAR(1) NOT NULL DEFAULT 'N';
ALTER TABLE card_payments ADD COLUMN IF NOT EXISTS ntfy_ts TIMESTAMPTZ;

SELECT column_name, data_type FROM information_schema.columns
WHERE table_name = 'card_payments' AND column_name IN ('ntfy_flag', 'ntfy_ts')
ORDER BY column_name;

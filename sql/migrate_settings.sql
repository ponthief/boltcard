-- add any settings rows that a database created by an earlier version does not
-- have yet
--
-- this is safe to run against a live database and safe to run more than once -
-- it adds missing rows and changes no existing value
--
-- $ psql card_db -f sql/migrate_settings.sql
--
-- an explanation for each setting can be found here
-- https://github.com/boltcard/boltcard/blob/main/docs/SETTINGS.md

\c card_db;

INSERT INTO settings (name, value)
SELECT new_settings.name, new_settings.value
FROM (VALUES
	('INTERNAL_API_KEY', ''),
	('INTERNAL_API_LISTEN', '127.0.0.1:9001'),
	('TRUSTED_PROXY_COUNT', ''),
	('EXTERNAL_RATE_LIMIT_PER_MIN', ''),
	('EXTERNAL_RATE_BURST', ''),
	('EXTERNAL_MAX_CONCURRENT', ''),
	('SETTING_CACHE_SEC', ''),
	('FUNCTION_NTFY', 'DISABLE'),
	('NTFY_URL', ''),
	('NTFY_TOPIC', ''),
	('NTFY_USER', ''),
	('NTFY_PASSWORD', ''),
	('LN_INVOICE_EXPIRY_SEC', '3600'),
	('DEFAULT_DESCRIPTION', 'bolt card service')
) AS new_settings(name, value)
WHERE NOT EXISTS (
	SELECT 1 FROM settings WHERE settings.name = new_settings.name
);

-- show the settings that hold no value, which may still need one
SELECT name FROM settings WHERE value = '' ORDER BY name;

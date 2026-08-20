\c card_db;

DELETE FROM settings;

-- an explanation for each of the bolt card server settings can be found here
-- https://github.com/boltcard/boltcard/blob/main/docs/SETTINGS.md

INSERT INTO settings (name, value) VALUES ('LOG_LEVEL', '');
INSERT INTO settings (name, value) VALUES ('AES_DECRYPT_KEY', '');
INSERT INTO settings (name, value) VALUES ('HOST_DOMAIN', '');
INSERT INTO settings (name, value) VALUES ('MIN_WITHDRAW_SATS', '');
INSERT INTO settings (name, value) VALUES ('MAX_WITHDRAW_SATS', '');
INSERT INTO settings (name, value) VALUES ('LN_HOST', '');
INSERT INTO settings (name, value) VALUES ('LN_PORT', '');
INSERT INTO settings (name, value) VALUES ('LN_TLS_FILE', '');
INSERT INTO settings (name, value) VALUES ('LN_MACAROON_FILE', '');
INSERT INTO settings (name, value) VALUES ('FEE_LIMIT_SAT', '');
INSERT INTO settings (name, value) VALUES ('FEE_LIMIT_PERCENT', '');
INSERT INTO settings (name, value) VALUES ('LN_TESTNODE', '');
INSERT INTO settings (name, value) VALUES ('FUNCTION_LNURLW', '');
INSERT INTO settings (name, value) VALUES ('FUNCTION_LNURLP', '');
INSERT INTO settings (name, value) VALUES ('FUNCTION_EMAIL', '');
INSERT INTO settings (name, value) VALUES ('DEFAULT_DESCRIPTION', 'bolt card service');
INSERT INTO settings (name, value) VALUES ('AWS_SES_ID', '');
INSERT INTO settings (name, value) VALUES ('AWS_SES_SECRET', '');
INSERT INTO settings (name, value) VALUES ('AWS_SES_EMAIL_FROM', '');
INSERT INTO settings (name, value) VALUES ('AWS_REGION', 'us-east-1');
INSERT INTO settings (name, value) VALUES ('EMAIL_MAX_TXS', '');
INSERT INTO settings (name, value) VALUES ('FUNCTION_LNDHUB', '');
INSERT INTO settings (name, value) VALUES ('LNDHUB_URL', '');
INSERT INTO settings (name, value) VALUES ('FUNCTION_INTERNAL_API', '');
INSERT INTO settings (name, value) VALUES ('INTERNAL_API_KEY', '');
INSERT INTO settings (name, value) VALUES ('INTERNAL_API_LISTEN', '127.0.0.1:9001');
INSERT INTO settings (name, value) VALUES ('TRUSTED_PROXY_COUNT', '');
INSERT INTO settings (name, value) VALUES ('EXTERNAL_RATE_LIMIT_PER_MIN', '');
INSERT INTO settings (name, value) VALUES ('EXTERNAL_RATE_BURST', '');
INSERT INTO settings (name, value) VALUES ('EXTERNAL_MAX_CONCURRENT', '');
INSERT INTO settings (name, value) VALUES ('SETTING_CACHE_SEC', '');
INSERT INTO settings (name, value) VALUES ('SENDGRID_API_KEY', '');
INSERT INTO settings (name, value) VALUES ('SENDGRID_EMAIL_SENDER', '');
INSERT INTO settings (name, value) VALUES ('LN_INVOICE_EXPIRY_SEC', '3600');

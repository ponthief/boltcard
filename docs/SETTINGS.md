# Settings

The database connection settings are in the system environment variables.  
Other settings are in the database in a `settings` table. 

## Upgrading an existing install

A database created by an earlier version does not hold the newer settings rows.
Every setting falls back to a working default when its row is missing, so the
service runs either way, but the rows have to exist before a value can be set.

To add the missing rows without touching any value already set:

```
$ psql card_db -f sql/migrate_settings.sql
```

It is safe to run against a live database and safe to run more than once. It
lists the settings still holding no value when it finishes.

After upgrading, review these in particular:

- `INTERNAL_API_KEY` - the internal API stays closed until it is set, see
  [the internal API](INTERNAL_API.md)
- `TRUSTED_PROXY_COUNT` - set it to `1` when a reverse proxy such as Caddy is in
  front of the service, so that rate limiting counts each caller rather than the
  proxy

Here are the descriptions of values available to use in the `settings` table:

| Name | Value | Description |
| --- | --- | --- |
| LOG_LEVEL | DEBUG | system logs are verbose to enable easier debug |
| | PRODUCTION | system logs are minimal |
| AES_DECRYPT_KEY | | hex encoded 128 bit AES key - see [FAQ](FAQ.md#how-do-i-generate-a-random-key-value-)|
| HOST_DOMAIN | yourdomain.com | the domain for hosting lnurlw & lnurlp services |
| MIN_WITHDRAW_SATS | 1 | minimum satoshis for lnurlw response |
| MAX_WITHDRAW_SATS | 1000000 | maximum satoshis for lnurlw response |
| LN_HOST | your_lnd_node.io | LND node gRPC domain |
| LN_PORT | 9001 | LND node gRPC port |
| LN_TLS_FILE | /home/ubuntu/boltcard/tls.cert | absolute path to your LND TLC certificate |
| LN_MACAROON_FILE | /home/ubuntu/boltcard/boltcard.macaroon | absolute path to your LND macaroon |
| FEE_LIMIT_SAT | 10 | the base fee limit amount for every invoice payment |
| FEE_LIMIT_PERCENT | 0.5 | the percentage fee limit amount added to the base fee limit amount |
| LN_TESTNODE | | lightning node pubkey for allowing only the defined test node |
| FUNCTION_LNURLW | ENABLE | system level switch for LNURLw (bolt card) services |
| FUNCTION_LNURLP | DISABLE | system level switch for LNURLp (lightning address) services |
| FUNCTION_EMAIL | DISABLE | system level switch for email updates on credits & debits |
| DEFAULT_DESCRIPTION | 'bolt card service' | default description of payment |
| AWS_SES_ID | | Amazon Web Services - Simple Email Service - access id |
| AWS_SES_SECRET | | Amazon Web Services - Simple Email Service - access secret |
| AWS_SES_EMAIL_FROM | | Amazon Web Services - Simple Email Service - email from field |
| AWS_REGION | us-east-1 | Amazon Web Services - Account region |
| EMAIL_MAX_TXS | | maximum number of transactions to include in the email body |
| FUNCTION_LNDHUB | DISABLE | system level switch for using LNDHUB in place of LND |
| LNDHUB_URL | | URL for the LNDHUB service |
| FUNCTION_INTERNAL_API | DISABLE | system level switch for activating the internal API |
| INTERNAL_API_KEY | | shared secret required by every internal API call - see [the internal API](INTERNAL_API.md) |
| INTERNAL_API_LISTEN | 127.0.0.1:9001 | address the internal API listens on - defaults to the loopback interface |
| TRUSTED_PROXY_COUNT | 1 | number of reverse proxies in front of the service, so that rate limiting counts each caller rather than the proxy - `1` for the supplied docker install, empty or `0` when nothing is in front |
| EXTERNAL_RATE_LIMIT_PER_MIN | 120 | requests per minute allowed per caller on the public endpoints |
| EXTERNAL_RATE_BURST | 40 | requests a caller may make at once on the public endpoints |
| EXTERNAL_MAX_CONCURRENT | 32 | public requests handled at once, which bounds the database connections in use |
| FUNCTION_NTFY | DISABLE | system level switch for asking the card holder to approve each payment by ntfy notification |
| NTFY_URL | http://localhost:2586 | the ntfy server to publish approval requests to |
| NTFY_TOPIC | | the ntfy topic to publish approval requests to - approval requests are not sent without it |
| NTFY_USER | | username for the ntfy server, where it needs one |
| NTFY_PASSWORD | | password for the ntfy server, where it needs one |
| NTFY_APPROVAL_SEC | 60 | seconds the card holder has to approve a payment before it is released - raise it where notifications reach the phone slowly |
| SETTING_CACHE_SEC | 10 | seconds a setting value is reused before it is read from the database again - a change to a setting takes up to this long to come into effect, `0` reads every time |
| SENDGRID_API_KEY      | | User API Key from SendGrid.com             |
| SENDGRID_EMAIL_SENDER | | Single Sender email address verified by SendGrid |
| LN_INVOICE_EXPIRY_SEC | 3600 | LN invoice's expiry time in seconds |

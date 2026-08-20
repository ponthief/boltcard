# Security Policy

Report issues you find by Telegram secret chat to `@peter_1337` .  
Please do not disclose any possible security vulnerabilities to third parties.

## Running the service safely

- the internal API creates cards and reads card keys, which amounts to control
  of the funds available to cards - it requires an `INTERNAL_API_KEY` and
  listens on the loopback interface by default, see
  [the internal API](docs/INTERNAL_API.md)
- only the external API (port 9000) is intended to be reverse proxied and
  reachable from the internet
- the one time link returned when a card is created hands over the card keys to
  whoever opens it first, so treat it as a secret and use it promptly
- keep `.env`, macaroons and TLS certificates out of version control

## How payments are protected

- a withdraw request may be used for one payment only - the transaction limit,
  the daily limit and the card balance are checked and the request is claimed in
  a single database transaction that locks the card, so concurrent callbacks for
  one card tap cannot each send a payment
- a payment still in flight counts against the card balance, and a payment that
  fails releases it again
- a wrong card PIN consumes the withdraw request, so a PIN can be tried once per
  card tap rather than being guessed in bulk
- payments and notifications run in their own goroutines, each of which recovers
  from a panic, so an unreachable lightning node cannot stop the service
- where payments are held for approval by notification, the approval link
  carries a single use token for that one payment rather than an API key, see
  [payment approval](docs/NTFY_APPROVAL.md)

## Rate limiting

The public endpoints are limited in two ways, both adjustable in the
[settings](docs/SETTINGS.md):

- each caller may make `EXTERNAL_RATE_LIMIT_PER_MIN` requests a minute, bursting
  to `EXTERNAL_RATE_BURST`, and is answered with HTTP 429 above that
- `EXTERNAL_MAX_CONCURRENT` requests are handled at once, and further requests
  are answered with HTTP 503 rather than queueing - this is the limit that keeps
  a flood from using up the connections the database allows, since a flood can
  come from many addresses

Set `TRUSTED_PROXY_COUNT` to the number of reverse proxies in front of the
service, so that callers are told apart by their own address rather than all
being counted as the proxy. It is `1` for the supplied docker install, where
Caddy is in front. Only that many entries at the end of `X-Forwarded-For` are
trusted, and the header is ignored entirely when the setting is empty or `0`, so
a caller cannot dodge the limiter by sending a header of its own.

`/ping` is not limited, so that health checks still answer while the service is
busy.

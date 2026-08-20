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

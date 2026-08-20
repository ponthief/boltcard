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

# Internal API

The internal API creates cards, reads and updates card settings, and wipes cards.

## Why it must stay private

A caller that can reach `/createboltcard` can

- create a card with spending limits of its own choosing, optionally allowing a
  negative balance
- read the returned one time link, and with it fetch the card keys from the
  public `/new` endpoint
- program a card (or compute card requests directly from the keys) and withdraw
  from the node by presenting an invoice of its own

A caller that can reach `/getboltcard` or `/wipeboltcard` can read card
settings, read card keys and disable cards in use.

The internal API therefore has to be treated as full control of the funds the
node makes available to cards.

## Protections

| Protection | Behaviour |
| --- | --- |
| API key | every function except `/ping` requires the `INTERNAL_API_KEY` to be presented; keys are compared in constant time |
| fail closed | the internal API port is not opened at all unless `FUNCTION_INTERNAL_API` is `ENABLE` **and** an `INTERNAL_API_KEY` of at least 16 characters is set |
| loopback by default | the listener binds to `INTERNAL_API_LISTEN`, which defaults to `127.0.0.1:9001`, so it is not reachable from the network |
| rate limiting | 60 requests per minute per caller, bursting to 20, answered with HTTP 429 when exceeded |
| no secrets in logs | one time codes, card keys and PINs are kept out of the log entries |

## Setting the API key

Generate a key, e.g.

```
$ openssl rand -hex 32
```

Supply it in either of these ways, with the environment taking precedence:

- the `INTERNAL_API_KEY` environment variable (`.env` for docker, the unit file
  for a systemd install)
- the `INTERNAL_API_KEY` row of the `settings` table

`docker_init.sh` generates a key into `.env` for you.

Keys shorter than 16 characters are rejected and leave the internal API closed.
32 characters or more is recommended.

## Calling the API

Present the key in an `Authorization: Bearer` header. It is deliberately not
accepted in the query string, because request URLs end up in logs.

```
$ curl -H "Authorization: Bearer $INTERNAL_API_KEY" \
    'localhost:9001/createboltcard?card_name=card_5&enable=false&tx_max=1000&day_max=10000&uid_privacy=true&allow_neg_bal=true'
```

An `X-Api-Key: <key>` header is accepted as an alternative for simple clients.

A missing or wrong key is answered with HTTP 401 and
`{"status":"ERROR","reason":"unauthorized"}`.

## Listening on a private network

Where the internal API has to be reached from another host or container, set
`INTERNAL_API_LISTEN`, for example `INTERNAL_API_LISTEN` = `0.0.0.0:9001`, and
restrict access to that address with a firewall or a docker network. The service
logs a warning at start up when the internal API is not on the loopback
interface. Do not publish the internal API port to the internet and do not
reverse proxy it alongside the external API.

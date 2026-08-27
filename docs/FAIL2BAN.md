# Banning abusive callers with fail2ban

The card service already turns away a caller that asks for too much: the public
endpoints are rate limited per caller and capped in how many are handled at
once, see [security](../SECURITY.md). fail2ban goes a step further and stops the
traffic reaching the service at all.

The files are in [`deploy/fail2ban`](../deploy/fail2ban).

## Read this first if you are behind Cloudflare

Banning at this machine's firewall does nothing while Cloudflare is in front of
the site. The packets arrive from Cloudflare, not from the caller, so a rule
against the caller's address never matches, and a rule against the address the
machine can see would block Cloudflare - that is, everyone.

So there are two parts to getting this right:

1. Caddy has to record the caller's address rather than Cloudflare's, which
   means telling it which proxies to trust
2. the ban has to be placed at Cloudflare, which fail2ban does through its
   `cloudflare-token` action

If your site is not behind Cloudflare, neither applies: drop the `action` line
from the jail and fail2ban's normal firewall action does the job.

## Two things that catch people out

Both showed up in a real log from a live install, so check for them before
trusting any of this.

**The address in the log is the proxy's, not the caller's.** A line like

```
104.22.56.205 - - [27/Aug/2026:15:37:27 +0000] "GET /admin HTTP/2.0" 200 1168
```

looks like an attacker at `104.22.56.205`, but that address is inside
`104.16.0.0/13`, which belongs to Cloudflare. Banning it takes the site off the
air for everybody. Fix it by trusting the proxy, below, and until then the jail
lists the Cloudflare ranges in `ignoreip` so a mistake cannot fire.

**A site that answers unknown paths with its own page returns 200 to a
scanner.** In the same log every probe for `/admin`, `/console`, `/panel` and
the rest was answered `200 1168` - the single page app's own index. A filter
that matches only 404 and 403 never fires on any of it. That is why this filter
also matches the paths themselves.

## 1. Caddy: log JSON, and trust the proxy

In the global options block, list the proxies in front of you. The current
Cloudflare ranges are published at <https://www.cloudflare.com/ips/>:

```caddy
{
	servers {
		trusted_proxies static 173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 \
			103.31.4.0/22 141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 \
			188.114.96.0/20 197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 \
			104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22 \
			2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 \
			2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32
	}
}
```

and turn on the access log for the site:

```caddy
thrilla.me {
	log {
		output file /var/log/caddy/access.log
		format json
	}

	@boltcard path /new /ln /cb /approve /ping /lnurlp/* /.well-known/lnurlp/*
	handle @boltcard {
		reverse_proxy 127.0.0.1:9000
	}

	handle {
		# your site
	}
}
```

With the proxies trusted, each log line carries `client_ip` holding the caller's
address alongside `remote_ip` holding Cloudflare's. The filter prefers
`client_ip` and falls back to `remote_ip` where it is absent, so it works either
way.

Where the log is in a common log format instead, from a `format transform`
template, use `{request>client_ip}` in place of `{request>remote_ip}` so the
address at the start of each line is the caller. The filter reads that format
too.

## 2. Install the filter and the jail

```
$ sudo cp deploy/fail2ban/filter.d/boltcard.conf /etc/fail2ban/filter.d/
$ sudo cp deploy/fail2ban/jail.d/boltcard.local /etc/fail2ban/jail.d/
$ sudo chmod 640 /etc/fail2ban/jail.d/boltcard.local
```

Edit the jail and set:

- `cfzone` - the zone id, on the Cloudflare dashboard overview page for the
  domain
- `cftoken` - an API token with `Zone` -> `Firewall Services` -> `Edit` on that
  zone only, from <https://developers.cloudflare.com/api/tokens/create/>. A
  scoped token, not the global API key.
- `logpath` and `backend`, if Caddy logs to a file rather than the journal

The jail file holds the token, which is why it is `chmod 640` and owned by root.

```
$ sudo systemctl restart fail2ban
$ sudo fail2ban-client status boltcard
```

## What it bans

| Seen in the Caddy log | Meaning |
| --- | --- |
| HTTP 429 | the card service's own rate limit turned the caller away |
| HTTP 503 | the service was already handling as many requests as it will take at once |
| HTTP 404 | a request for a path the site does not serve, which is what scanning looks like |
| a request for a path in `badpaths` | scanning, whatever status the site answered with |

Twenty of those from one address within ten minutes earns an hour's ban. Adjust
`maxretry`, `findtime` and `bantime` to taste.

`badpaths` is two lists in the filter. `scanpaths` holds paths nothing but a
scanner asks for - `/wp-login`, `/.env`, `/phpmyadmin` and so on. `sitepaths`
holds ones many sites do serve - `/admin`, `/dashboard`, `/portal`. **Remove
anything from `sitepaths` that your site serves**, or a real visitor asking for
it gets banned. `/account` and `/app` are deliberately not in the list, being
too likely to be real.

**What it cannot see.** The card service answers its own errors with HTTP 200
and `{"status":"ERROR"}` in the body, because that is what LNURL asks for. A bad
one time code on `/new`, a wrong CMAC on `/ln` or an unknown approval token all
look like a success to Caddy. Those are not left unprotected - they are bounded
by the per caller rate limit inside the service, which is what produces the 429s
this filter bans on. A caller hammering `/new` therefore gets rate limited
first, and banned once it keeps going.

## Checking the filter before you rely on it

`fail2ban-regex` will tell you what matches, and which address would be banned:

```
$ fail2ban-regex /var/log/caddy/access.log /etc/fail2ban/filter.d/boltcard.conf
$ fail2ban-regex -v /var/log/caddy/access.log /etc/fail2ban/filter.d/boltcard.conf
```

Confirm that the addresses listed are callers and not Cloudflare's. If every
match shows a Cloudflare address, `trusted_proxies` is not set and banning would
take out your own traffic.

To lift a ban:

```
$ sudo fail2ban-client set boltcard unbanip 203.0.113.7
```

## The simpler alternative

Cloudflare can do this itself, with a rate limiting rule on the card paths and
no moving parts on your machine. If all you want is to shed scanner noise, that
is less to maintain. fail2ban is the better fit when you also want to ban on
what your own logs see, or when the site is not behind a proxy that can do it
for you.

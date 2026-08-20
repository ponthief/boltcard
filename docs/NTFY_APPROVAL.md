# Payment approval by notification

With `FUNCTION_NTFY` set to `ENABLE`, every card payment is held until the card
holder approves it from an [ntfy](https://ntfy.sh) notification on their phone.

## How it works

1. a card is tapped and the point of sale sends the invoice to `/cb`
2. the payment rules are checked and the payment is claimed, so the card tap is
   already spent at this point and cannot be used for a second payment
3. the service answers `{"status":"OK"}` straight away, as LUD-03 requires, and
   does the rest on its own
4. a notification is published with Approve and Reject buttons
5. the payment is sent when the card holder approves within the approval period,
   which is one minute
6. a payment that is rejected, or not answered in time, is marked failed and its
   amount is cleared, so it counts towards neither the daily limit nor the card
   balance. The card tap stays spent either way.

Because the response is sent before the answer is known, the point of sale sees
`OK` for a payment that may still be rejected. That is what LUD-03 describes: a
service answers and then pays asynchronously.

## The approval link

Each notification carries a single use token for the one payment it belongs to,
and the buttons point at `/approve` on the public API:

```
https://yourdomain.com/approve?token=<32 hex characters>&approve=Y
```

The token stands in for authentication:

- it is 128 bits from a cryptographic random source, so it cannot be guessed
- it only ever answers the payment it was made for
- it is cleared once used, so a notification cannot be replayed
- it stops working as soon as its payment has been sent or released

This is why the internal API key never has to travel to the ntfy server or sit
in a notification on a phone, and why port 9001 does not have to be exposed.
`/approve` is rate limited with the rest of the public API.

There is also `/updateboltcardpayment` on the internal API, which answers by
`card_payment_id` and needs the API key. It is for use on the host itself, not
from a notification.

## Settings

| Name | Value | Description |
| --- | --- | --- |
| FUNCTION_NTFY | ENABLE | hold every payment for approval |
| NTFY_URL | http://localhost:2586 | the ntfy server to publish to |
| NTFY_TOPIC | | the topic to publish to, required |
| NTFY_USER | | username for the ntfy server, where it needs one |
| NTFY_PASSWORD | | password for the ntfy server, where it needs one |

Subscribe to the same topic on the phone. The topic name is all that is needed
to send notifications to it on a public ntfy server, so choose one that cannot
be guessed, or use a server that requires a login.

## Database

The approval flow needs three columns on `card_payments`, which
`sql/create_db.sql` creates. For a database made before this feature:

```
$ psql card_db -f sql/migrate_ntfy.sql
```

Without them no payment record can be inserted, so every card tap fails.

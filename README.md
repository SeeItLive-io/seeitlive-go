# seeitlive-go

Official SeeItLive SDK for Go — an idiomatic port of the reference
[`@seeitlive/sdk`](../client-node) Node client. Same behaviour, same wire
contract, Go conventions.

## Install

```bash
go get github.com/SeeItLive-io/seeitlive-go
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/SeeItLive-io/seeitlive-go"
)

func main() {
	client, err := seeitlive.New(seeitlive.Config{
		APIKey: os.Getenv("SEEITLIVE_API_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// SMS channel requires RecipientPhone + SMSConsent (the recipient
	// agreed to receive a text; carrier fees may apply).
	consent := true
	session, err := client.Sessions.Create(ctx, seeitlive.CreateSessionParams{
		Channel:        seeitlive.ChannelSMS,
		RecipientPhone: "+15551234567",
		SMSConsent:     &consent,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(session.InviteURL, session.InviteCode)

	// email channel requires RecipientEmail
	_, _ = client.Sessions.Create(ctx, seeitlive.CreateSessionParams{
		Channel:        seeitlive.ChannelEmail,
		RecipientEmail: "customer@example.com",
	})

	// link channel needs neither — just returns a shareable link
	linkSession, _ := client.Sessions.Create(ctx, seeitlive.CreateSessionParams{
		Channel: seeitlive.ChannelLink,
	})

	status, err := client.Sessions.Status(ctx, linkSession.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.Status, status.ProducerCount, status.HasVideo)
}
```

The client infers `Env` (`seeitlive.EnvLive` or `seeitlive.EnvTest`) from the
API key's prefix (`sil_test_...` → test, otherwise live). Override with
`Config.Env`, and override the API base URL with `Config.BaseURL` (default:
`https://api.seeitlive.io/v1`).

Provide **exactly one** of `Config.APIKey` or `Config.OAuthToken` —
supplying both, or neither, makes `New` return an error.

### Reading a session

`Sessions.Get` returns a stored session, including how its invitation was
delivered — useful when re-opening a session an integration created earlier:

```go
s, err := client.Sessions.Get(ctx, session.ID)
if err != nil {
    log.Fatal(err)
}
fmt.Println(s.Status, s.Metadata, s.Invitation.Channel, *s.Invitation.Recipient)
```

### Guest protection

A session can require the customer to enter a secret before their camera is
shared — a static password, or a rotating code. Read the current one to them:

```go
p, err := client.Sessions.GetProtection(ctx, session.ID)
if p.Type == "password" {
    fmt.Println(*p.Password)
}
if p.Type == "totp" {
    fmt.Println(*p.Code, *p.SecondsRemaining, "s left")
}
```

Change it after creation — including adding one to a session that had none.
Leave `ProtectionSecret` empty and one is generated for you:

```go
added, err := client.Sessions.SetProtection(ctx, session.ID, seeitlive.SetProtectionParams{
    Protection: "password",
})
fmt.Println(*added.Password) // read this to the customer

// Clearing also clears any lockout the customer triggered.
_, err = client.Sessions.SetProtection(ctx, session.ID, seeitlive.SetProtectionParams{
    Protection: "none",
})
```

Five wrong answers lock the guest out. Clear the lockout — rotation is opt-in,
since the usual cause is a mistyped secret:

```go
_, err = client.Sessions.ResetProtection(ctx, session.ID, seeitlive.ResetProtectionParams{})

rotate := true
fresh, err := client.Sessions.ResetProtection(ctx, session.ID, seeitlive.ResetProtectionParams{
    Rotate: &rotate, // issues a new secret as well
})
```

For `totp`, `GetProtection` returns the **current code**, never the seed — the
seed leaves the server only once, in the create response.

### Embedded viewer tokens

`Sessions.CreateAgentToken` mints a fresh agent-view grant so your own UI can
embed the live viewer. Tokens expire after 120 minutes — mint another to
re-open a viewer:

```go
viewer, err := client.Sessions.CreateAgentToken(ctx, session.ID, seeitlive.CreateAgentTokenParams{
    Label: "Odoo — crm.lead 42", // shown in the dashboard's viewer list
})
fmt.Println(viewer.AgentViewURL)
```

### Snapshots

Frames the agent captured during a session. Both calls require a plan with
snapshot history:

```go
snapshots, err := client.Snapshots.List(ctx, session.ID) // newest first
image, err := client.Snapshots.Get(ctx, snapshots[0].ID)

// Data is base64 PNG — the endpoint's raw-bytes mode is not used by the SDK.
raw, err := base64.StdEncoding.DecodeString(image.Data)
os.WriteFile("frame.png", raw, 0o600)
```

## OAuth usage

For OAuth-based (multi-tenant / marketplace) integrations, exchange an
authorization code for an access token using the standalone `ExchangeCode`
function (this call is unauthenticated — it uses your client credentials in
the request body, not a bearer token, and is a package-level function
because it runs *before* any token exists):

```go
token, err := seeitlive.ExchangeCode(ctx, seeitlive.ExchangeCodeParams{
	Code:         authorizationCodeFromRedirect,
	ClientID:     os.Getenv("SEEITLIVE_CLIENT_ID"),
	ClientSecret: os.Getenv("SEEITLIVE_CLIENT_SECRET"),
	RedirectURI:  "https://your-app.example.com/oauth/callback",
})
if err != nil {
	log.Fatal(err)
}

client, err := seeitlive.New(seeitlive.Config{OAuthToken: token.AccessToken})
if err != nil {
	log.Fatal(err)
}

info, err := client.OAuth.Verify(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println(info.ClientID, info.TeamID, info.Scopes, info.Plan)
```

## Request signing

Set `Config.SigningSecret` to have every outgoing request HMAC-signed
automatically. When set, each request carries:

- `X-SeeItLive-Signature: sha256=<hex>` —
  `HMAC_SHA256(SigningSecret, "<unix_ts>.<METHOD>.<path>.<sha256hex(raw_body)>")`,
  where `path` is the request path only (e.g. `/sessions`, with no base URL
  and no `/api/v1` prefix) and `raw_body` is the exact bytes sent (hashed as
  the empty string when there is no body).
- `X-SeeItLive-Timestamp: <unix_ts>` — unix seconds.

`path` is signed **including any query string** the SDK sends (e.g.
`/snapshots/<id>?format=base64`) — the server verifies path *and* query,
so the two must match exactly.

```go
client, err := seeitlive.New(seeitlive.Config{
	APIKey:        os.Getenv("SEEITLIVE_API_KEY"),
	SigningSecret: os.Getenv("SEEITLIVE_SIGNING_SECRET"),
})
if err != nil {
	log.Fatal(err)
}
```

A present-but-empty `SigningSecret` is treated the same as absent: requests
are unsigned by default, so this is fully backwards-compatible. The
standalone `ExchangeCode` function is never signed.

## Errors

Every SDK error embeds `seeitlive.APIError` (`StatusCode`, `Message`,
`Code`) and implements `Unwrap() error`, so `errors.As` reaches both the
concrete type and the base `*APIError`:

| Type | HTTP status |
| --- | --- |
| `*seeitlive.ValidationError` (also has `Field`) | 400 |
| `*seeitlive.AuthenticationError` | 401 |
| `*seeitlive.PermissionError` | 403 |
| `*seeitlive.NotFoundError` | 404 |
| `*seeitlive.RateLimitError` (also has `RetryAfter *int`, seconds) | 429 |
| `*seeitlive.APIError` | any other non-2xx status |

```go
import (
	"errors"

	"github.com/seeitlive/seeitlive-go"
)

_, err := client.Sessions.Status(ctx, "missing")
if err != nil {
	var rateLimit *seeitlive.RateLimitError
	var apiErr *seeitlive.APIError
	switch {
	case errors.As(err, &rateLimit):
		if rateLimit.RetryAfter != nil {
			log.Printf("retry after %d seconds", *rateLimit.RetryAfter)
		}
	case errors.As(err, &apiErr):
		log.Printf("%d %s %s", apiErr.StatusCode, apiErr.Message, apiErr.Code)
	default:
		log.Fatal(err) // network/transport error — not a SeeItLive API error
	}
}
```

`RateLimitError.RetryAfter` is a `*int`, not an `int`: the `Retry-After`
header can legitimately be `0` (retry immediately), which must be
distinguished from the header being absent (`nil`). Don't treat a `nil`
check as "no need to wait" — check for `nil` explicitly, then dereference.

Network/transport failures (DNS, connection refused, timeouts, context
cancellation) surface as Go's ordinary `*url.Error` / `context.Canceled` /
etc. from the standard library — they are not wrapped in a SeeItLive error
type.

## Security

- **Never log API keys, OAuth access tokens, or refresh tokens.**
- **Never ship `ClientSecret` to the browser or a mobile app.**
  `ExchangeCode` must only be called from a trusted server-side context —
  it is a confidential-client operation. Client-side code should redirect
  the user through your own backend, which performs the code exchange and
  hands the browser a short-lived session, never the raw OAuth token or
  client secret.
- The default `BaseURL` is HTTPS; the SDK never disables TLS verification.

## Development

```bash
go test ./...
go build ./...
```

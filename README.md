# squadron-gateway-sdk

Go SDK for building **Squadron gateways**: subprocess integrations
that bridge a running squadron to an external system (Discord, Slack,
Microsoft Teams, custom dashboards, …).

## Concept

Gateways are squadron's pluggable integration surface. Squadron
launches a gateway as a managed subprocess (same install/version
lifecycle as plugins) and connects to it over a bidirectional gRPC
channel:

- **Squadron → Gateway**: pushes events (`OnHumanInputRequested`,
  `OnHumanInputResolved`, `OnNotification`, …) so the gateway can mirror
  state to its external system. `OnNotification` is a one-way
  mission-lifecycle post (`mission_completed` / `mission_failed`) —
  informational, with nothing for the user to act on.
- **Gateway → Squadron**: pulls / mutates state (`ListHumanInputs`,
  `ResolveHumanInput`, …) so user actions in the external system flow
  back to squadron.

## Hello-world

```go
package main

import (
    "context"

    gateway "github.com/mlund01/squadron-gateway-sdk"
)

type myGateway struct{ api gateway.SquadronAPI }

func (g *myGateway) Configure(ctx context.Context, settings map[string]string, api gateway.SquadronAPI) error {
    g.api = api
    // catch up on anything that happened while we were down
    rows, _, err := api.ListHumanInputs(ctx, gateway.HumanInputFilter{OldestFirst: true})
    if err != nil { return err }
    for _, r := range rows { g.show(r) }
    return nil
}

func (g *myGateway) OnHumanInputRequested(ctx context.Context, rec gateway.HumanInputRecord) error {
    g.show(rec)
    return nil
}

func (g *myGateway) OnHumanInputResolved(ctx context.Context, rec gateway.HumanInputRecord) error {
    g.markAnswered(rec)
    return nil
}

func (g *myGateway) OnNotification(ctx context.Context, rec gateway.NotificationRecord) error {
    // one-way mission-lifecycle post; rec.Channel optionally overrides the
    // destination channel
    g.post(rec)
    return nil
}

func (g *myGateway) Shutdown(ctx context.Context) error { return nil }

func (g *myGateway) show(_ gateway.HumanInputRecord)         {}
func (g *myGateway) markAnswered(_ gateway.HumanInputRecord) {}
func (g *myGateway) post(_ gateway.NotificationRecord)       {}

func main() { gateway.Serve(&myGateway{}) }
```

## Catch-up after disconnects

Squadron pushes events live, but a gateway subprocess can crash or
restart. The pattern is:

1. Persist the latest event timestamp the gateway has processed.
2. On `Configure`, call `ListHumanInputs` with that timestamp as
   `Since` and replay anything you missed.
3. Live `OnHumanInput*` events resume after `Configure` returns.

`ResolveHumanInput` is idempotent — calling it on an already-resolved
request returns `AlreadyResolved=true` with the prior resolution
intact.

## Distribution

Squadron downloads a gateway's release archive from GitHub on first
load (same lifecycle as native plugins). The contract:

- Archive: `<repo>_<GOOS>_<GOARCH>.tar.gz` (or `.zip` on Windows).
- Inside: one `gateway` binary at the root.
- Sibling: `checksums.txt` with sha256 hashes.

For local development, set `version = "local"` in your squadron
config and place the binary at
`.squadron/gateways/<platform>/<name>/local/gateway`.

## See also

- [`squadron-gateway-discord`](https://github.com/mlund01/squadron-gateway-discord)
  — reference implementation that bridges to a Discord channel.

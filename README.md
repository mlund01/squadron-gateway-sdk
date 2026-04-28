# squadron-gateway-sdk

Go SDK for building **Squadron gateways**: subprocess integrations
that bridge a running squadron to an external system (Discord, Slack,
PagerDuty, custom dashboards, …).

## Concept

Gateways are squadron's pluggable integration surface. Squadron
launches a gateway as a managed subprocess (same install/version
lifecycle as plugins) and connects to it over a bidirectional gRPC
channel:

- **Squadron → Gateway**: pushes events (`OnHumanInputRequested`,
  `OnHumanInputResolved`, …) so the gateway can mirror state to its
  external system.
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

func (g *myGateway) Shutdown(ctx context.Context) error { return nil }

func (g *myGateway) show(_ gateway.HumanInputRecord)         {}
func (g *myGateway) markAnswered(_ gateway.HumanInputRecord) {}

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

## Releasing

Squadron pulls gateways from GitHub releases the same way it pulls
native plugins. Cut a release tag (e.g. `v0.1.0`), upload an
archive named `<repo>_<GOOS>_<GOARCH>.tar.gz` containing a single
`gateway` binary, and a `checksums.txt` with sha256 hashes. Squadron
will download the right archive for the host platform on first load.

For local development, set `version = "local"` in your squadron
config and place the binary at
`.squadron/gateways/<platform>/<name>/local/gateway`.

## See also

- [`squadron-gateway-discord`](https://github.com/mlund01/squadron-gateway-discord)
  — reference implementation that bridges to a Discord channel.

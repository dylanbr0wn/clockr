# ADR-0002: Connect and Protobuf API Boundary

- Status: Accepted
- Date: 2026-07-09
- Linear: DYL-94

## Context

shiet's React frontend currently calls the local Go process through generated
Wails bindings. That is a good native integration mechanism, but it is not a
network contract that a future browser client can reuse. The OAuth broker has a
separate hand-written JSON contract whose request and response structs have
also been repeated in the broker and desktop provider packages.

Native gRPC is not directly available to browser JavaScript. gRPC-Web adapts
the protocol for browsers, traditionally with a translating proxy, and does
not support every native streaming shape. Connect provides generated Go and
TypeScript APIs over ordinary HTTP while its Go handlers also accept Connect,
gRPC-Web, and gRPC clients.

A transport change alone does not create a hosted web product. User identity,
tenant ownership, hosted persistence, browser sessions, and the privacy impact
of moving local SQLite/keychain data remain separate architecture decisions.

## Decision

Use versioned Protobuf contracts and Connect for portable shiet APIs:

- `shiet.app.v1` contains application operations that can make sense on either
  a desktop-local or future hosted service. The first vertical slice is period
  listing and current-period creation.
- `shiet.broker.v1` contains the provider-neutral OAuth broker start, handoff,
  refresh, and revoke operations. Generated messages replace duplicated wire
  shapes in new desktop clients.
- Go Connect handlers are the RPC implementation. They accept Connect,
  gRPC-Web, and gRPC protocols; browser TypeScript uses the Connect protocol.
- Generated sources are checked in and generator versions are pinned with Buf.

The application RPC handler delegates to the existing `service.Service`; it
does not move business rules into the transport. Generated Protobuf messages
remain behind `frontend/src/lib/api/shietService.ts`, which maps `int64` IDs to
JavaScript numbers only after checking the safe-integer range.

The broker replaces its handwritten REST operations with the generated Connect
service. There are no released users requiring a compatibility period, so this
branch is a hard transport switch: desktop clients use the generated Connect-Go
client and the broker exposes no REST aliases or fallback behavior for start,
handoff, refresh, or revoke.

Google and GitHub callbacks remain ordinary HTTP GET routes because providers
navigate the user agent to those endpoints with `code` and `state`. Health,
readiness, and Prometheus metrics also remain ordinary HTTP endpoints.

## Security and compatibility

- The broker's no-persistent-token decision from ADR-0001 is unchanged.
  Protobuf token messages are transient and must never be logged or added to
  metrics.
- Broker Connect endpoints do not enable cross-origin browser access. A future
  browser client needs an explicit application-session/BFF decision before it
  may use token-bearing operations.
- Stable broker error identifiers are returned as Connect error details.
- Google is the only provider supporting refresh. GitHub refresh returns
  `Unimplemented`; revoke validates that Google supplies a refresh token and
  GitHub supplies an access token.
- Wails v2's AssetServer supports unary POST handlers, but its documentation
  warns about Vite 5 development-server routing and unsupported response
  streaming on Windows. This slice is unary, and its Connect POST was verified
  through both the packaged app and the Wails development server. If a future
  transport shape proves unreliable, use a loopback-only server with a
  per-process capability token and exact-origin CORS rather than exposing an
  unauthenticated local port.

## Alternatives considered

### Keep Wails bindings and handwritten broker JSON only

Smallest short-term change, but it leaves no reusable browser contract and
continues duplicating broker wire types.

### Native grpc-go everywhere

Appropriate for trusted server-to-server traffic, but browser clients cannot
use native gRPC directly. A separate gRPC-Web layer or proxy would still be
required.

### Replace every Wails method immediately

Rejected as an unsafe horizontal migration. Native-only capabilities such as
file dialogs, clipboard access, keychain storage, and desktop handoff belong
behind platform adapters. Portable operations can move incrementally through
the stable frontend API facade.

### Keep REST alongside Connect

Rejected because there are no released users to migrate. A dual transport
would add schema, tests, fallback behavior, and replay risk without providing
compatibility value.

## Sources

- [Connect overview](https://connectrpc.com/)
- [Connect protocol selection](https://connectrpc.com/docs/web/choosing-a-protocol/)
- [gRPC-Web protocol](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md)
- [Wails v2.12 AssetServer options](https://wails.io/docs/v2.12.0/reference/options/)
- [Buf code generation](https://buf.build/docs/generate/)
- [Google OAuth web-server flow](https://developers.google.com/identity/protocols/oauth2/web-server)
- [GitHub OAuth authorization flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps)

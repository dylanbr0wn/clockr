# ADR-0002: Connect and Protobuf API Boundary

- Status: Accepted (amended 2026-07-10)
- Date: 2026-07-09
- Linear: DYL-94

## Context

shiet's React frontend originally called the local Go process through generated
Wails bindings. That is a workable native integration mechanism, but it is not a
network contract a future browser client can reuse. The OAuth broker has a
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
See [ADR-0003](0003-platform-adapters.md) for how platform-specific behavior
stays behind handlers without splitting the frontend contract.

## Decision

Use versioned Protobuf contracts and Connect as the **sole application API**
between frontend and backend:

- `shiet.app.v1` contains every application operation that can make sense on
  either a desktop-local or future hosted service. Its versioned services cover
  periods, categories, calendars and sync, schedules and review decisions,
  settings, integration metadata and connect/disconnect, AI configuration, and
  export rendering/templates.
- `shiet.broker.v1` contains the provider-neutral OAuth broker start, handoff,
  refresh, and revoke operations. Generated messages replace duplicated wire
  shapes in new desktop clients.
- Go Connect handlers are the RPC implementation. They accept Connect,
  gRPC-Web, and gRPC protocols; browser TypeScript uses the Connect protocol.
- Generated sources are Git-ignored and reproduced locally and in CI from
  generator versions pinned with Buf.

The application RPC handler delegates to the existing `service.Service` and
integration providers; it does not move business rules into the transport.
Generated Protobuf messages remain behind `frontend/src/lib/api/shietService.ts`,
which maps `int64` IDs to JavaScript numbers only after checking the safe-integer
range.

### Wails role (desktop shell only)

Wails v2 remains the desktop **container**: webview, lifecycle, and AssetServer
mount for `/rpc`. It is **not** an application API surface.

- Do **not** add new Wails-bound business methods.
- Deprecate and remove existing `App` exports as each operation moves to Connect.
- Platform-specific behavior (keychain, system browser during OAuth, optional
  native save dialog) runs inside Connect handlers via adapters defined in
  [ADR-0003](0003-platform-adapters.md).

`service.Service` itself is not Wails-bound.

Low-level dependency and merge seams (`SetCalendarSync`, `SetEvidence`, and
`SyncEvents`) are internal service APIs, not frontend operations, and are
intentionally exposed through neither Connect nor Wails.

### HTTP endpoints (not Connect, not Wails)

These stay ordinary HTTP routes:

- OAuth provider and broker **callbacks** — providers navigate the user agent
  to fixed URLs with `code` and `state` (Google, GitHub, Slack).
- Broker health, readiness, and Prometheus metrics.
- (Future) hosted auth session endpoints once a web product exists.

### Desktop mounting

On desktop, Connect handlers mount on the Wails AssetServer at `/rpc` (same origin
as the embedded frontend). Standalone or hosted deployments mount the same
handler mux on their HTTP server. The frontend uses one generated Connect client;
only the base URL differs (`/rpc` vs remote origin).

## Security and compatibility

- The broker's no-persistent-token decision from ADR-0001 is unchanged.
  Protobuf token messages are transient and must never be logged or added to
  metrics.
- Broker Connect endpoints do not enable cross-origin browser access. A future
  browser client needs an explicit application-session/BFF decision before it
  may use token-bearing operations.
- Stable broker error identifiers are returned as Connect error details.
- Google is the only provider supporting refresh. GitHub and Slack refresh return
  `Unimplemented`; revoke validates that Google supplies a refresh token and
  GitHub and Slack supply access tokens.
- Wails v2's AssetServer supports unary POST handlers, but its documentation
  warns about Vite 5 development-server routing and unsupported response
  streaming on Windows. The application services are unary and share the same
  AssetServer handler path verified by the initial period slice. If a future
  transport shape proves unreliable, use a loopback-only server with a
  per-process capability token and exact-origin CORS rather than exposing an
  unauthenticated local port.

## Alternatives considered

### Keep Wails bindings and handwritten broker JSON only

Smallest short-term change, but it leaves no reusable browser contract and
continues duplicating broker wire types. **Rejected.**

### Native grpc-go everywhere

Appropriate for trusted server-to-server traffic, but browser clients cannot
use native gRPC directly. A separate gRPC-Web layer or proxy would still be
required. Connect already accepts gRPC clients on the server side.

### Permanent dual transport (Wails + Connect)

Maintain Wails bindings for "native" operations and Connect for portable reads.
**Rejected.** It forces two client paths, duplicate types, and drift (e.g.
integration connect on Wails while resource lists use Connect). One Connect
contract with platform adapters is strictly better for desktop + hosted.

### Route native capabilities through Connect (original rejection, now accepted)

Earlier draft rejected routing OAuth, keychain, and file dialogs through Connect
because the *implementations* are not portable. That conflated transport with
deployment. **Accepted (amended):** the RPC contract is portable; desktop and
hosted supply different adapters behind the same handlers. OAuth HTTP callbacks
remain HTTP; opening the system browser happens inside the Go handler, not as a
separate Wails export.

### Keep REST alongside Connect

Rejected because there are no released users to migrate. A dual transport
would add schema, tests, fallback behavior, and replay risk without providing
compatibility value.

## Consequences

- Frontend maintains one Connect client path per operation.
- Remaining Wails `App` methods are technical debt to remove, not a pattern to
  extend.
- Integration connect/disconnect/auth/catalog belong on `IntegrationService`
  (see [ADR-0004](0004-standardized-integrations-settings-surface.md)).

## Sources

- [Connect overview](https://connectrpc.com/)
- [Connect protocol selection](https://connectrpc.com/docs/web/choosing-a-protocol/)
- [gRPC-Web protocol](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md)
- [Wails v2.12 AssetServer options](https://wails.io/docs/v2.12.0/reference/options/)
- [Buf code generation](https://buf.build/docs/generate/)
- [Google OAuth web-server flow](https://developers.google.com/identity/protocols/oauth2/web-server)
- [GitHub OAuth authorization flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps)

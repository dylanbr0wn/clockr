# shiet Context

shiet is a Wails v2 desktop app for importing calendar events, categorizing
work, filling schedule gaps, and exporting timesheet reports. The app runs as a
single native binary with a Go backend, a React frontend, and a local SQLite
database.

## Terms

- **Desktop app**: the distributed shiet native binary running on the user's
  machine.
- **Local token store**: the OS keychain-backed storage used by the desktop app
  for provider refresh and access tokens.
- **OAuth broker**: the small multi-provider server-side component described in
  [ADR-0001](docs/adr/0001-secret-only-google-oauth-broker.md) that protects
  shiet's shared Google and GitHub OAuth client secrets without persistently
  storing provider tokens.
- **Handoff code**: a short-lived, one-time broker code that lets the desktop app
  retrieve token material after the broker completes a provider callback.
- **BYO credentials**: a developer or advanced-user mode where the desktop app
  is configured with provider OAuth credentials from local config or
  environment. GitHub PAT connect is also retained as an advanced-user path.

## Decisions

- Public shiet builds must not embed or ship a shared Google OAuth
  `client_secret`. Use the secret-only Google OAuth broker for public Google
  Calendar connections; keep BYO credentials as a development and advanced-user
  escape hatch. See [ADR-0001](docs/adr/0001-secret-only-google-oauth-broker.md).
- Public shiet builds use the same broker boundary for the shared GitHub OAuth
  App secret. GitHub OAuth App user tokens are handed to the desktop keychain,
  are not refreshed by the broker, and are revoked through the broker on
  disconnect. Local/BYO OAuth and PAT connect remain available.
- Portable frontend/backend operations use versioned Protobuf contracts and
  Connect, introduced incrementally behind the frontend API facade. Native-only
  desktop capabilities remain on Wails bindings. The OAuth broker serves start,
  handoff, refresh, and revoke only through Connect; provider callbacks and
  operational endpoints remain ordinary HTTP. See
  [ADR-0002](docs/adr/0002-connect-protobuf-api-boundary.md).

## Related docs

- [DESIGN.md](DESIGN.md) — product shape, core loop, schema intent, roadmap
- [docs/oauth-broker.md](docs/oauth-broker.md) — broker operator runbook
- [README.md](README.md) — setup, build, config

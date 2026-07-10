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
- **Integration**: a third-party service the desktop app connects to (e.g. Google
  Calendar, GitHub, Slack). Each integration has one or more connected accounts
  tracked in the connection registry.
- **Calendar source**: an integration kind that imports schedule events (today:
  Google Calendar). The user selects which calendars count toward the pay period.
- **Activity evidence provider**: an integration kind that supplies read-only
  activity context for gap-fill suggestions (e.g. GitHub repos, Slack channels).
  Evidence never auto-creates time entries.
- **Integrations settings**: the single Settings area where users connect accounts
  and configure integration resources. Providers appear in a catalog grouped by
  kind; adding a provider does not add a new top-level settings tab.

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
  Connect behind the frontend API facade. Wails bindings are restricted to
  native-only desktop capabilities. The OAuth broker serves start,
  handoff, refresh, and revoke only through Connect; provider callbacks and
  operational endpoints remain ordinary HTTP. See
  [ADR-0002](docs/adr/0002-connect-protobuf-api-boundary.md).

- Integrations settings use one catalog + detail surface for all providers. Adding
  a provider adds a catalog entry and kind config adapter — not a new top-level
  settings tab. See
  [ADR-0002](docs/adr/0002-standardized-integrations-settings-surface.md).
- Desktop and OAuth broker share one **zerolog** logging stack (`internal/log`)
  with ADR-0001 secret redaction. Desktop default log file:
  `<UserConfigDir>/shiet/shiet.log` (`log.path` / `SHIET_LOG_*`). Broker logs
  JSON to stdout. See [docs/logging.md](docs/logging.md).

## Related docs

- [DESIGN.md](DESIGN.md) — product shape, core loop, schema intent, roadmap
- [docs/adr/0002-standardized-integrations-settings-surface.md](docs/adr/0002-standardized-integrations-settings-surface.md) — Integrations settings IA and API contract
- [docs/logging.md](docs/logging.md) — desktop + broker logging (paths, config, redaction)
- [docs/oauth-broker.md](docs/oauth-broker.md) — broker operator runbook
- [README.md](README.md) — setup, build, config

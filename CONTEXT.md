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
- **Google OAuth broker**: the small server-side component described in
  [ADR-0001](docs/adr/0001-secret-only-google-oauth-broker.md) that protects
  shiet's shared Google OAuth client secret without storing Google tokens.
- **Handoff code**: a short-lived, one-time broker code that lets the desktop app
  retrieve token material after the broker completes Google's OAuth callback.
- **BYO credentials**: a developer or advanced-user mode where the desktop app
  is configured with Google OAuth credentials from local config or environment.

## Decisions

- Public shiet builds must not embed or ship a shared Google OAuth
  `client_secret`. Use the secret-only Google OAuth broker for public Google
  Calendar connections; keep BYO credentials as a development and advanced-user
  escape hatch. See [ADR-0001](docs/adr/0001-secret-only-google-oauth-broker.md).

## Related docs

- [DESIGN.md](DESIGN.md) — product shape, core loop, schema intent, roadmap
- [docs/oauth-broker.md](docs/oauth-broker.md) — broker operator runbook
- [README.md](README.md) — setup, build, config

# shiet

Local desktop app for summarizing time spent each pay period: import Google
Calendar, categorize events, fill schedule gaps, export a timesheet report.

Built with [Wails v2](https://wails.io/) (Go + React/TypeScript) and SQLite.
Calendar data and tokens stay on the machine. Public Google OAuth uses a
secret-only broker so the shared `client_secret` never ships in the binary —
see [ADR-0001](docs/adr/0001-secret-only-google-oauth-broker.md) and
[docs/oauth-broker.md](docs/oauth-broker.md).

## Development

Prerequisites: Go 1.26+, [pnpm](https://pnpm.io/), [Wails CLI](https://wails.io/docs/gettingstarted/installation), and [Buf](https://buf.build/docs/cli/installation/).

```bash
buf generate
pnpm -C frontend install
wails dev
```

Frontend HMR: http://localhost:5173.

On Ubuntu 24.04 (webkit2gtk-4.1), pass the build tag:

```bash
wails dev -tags webkit2_41
```

Frontend only (no Go / Wails):

```bash
pnpm --dir frontend dev
```

Regenerate the shared Connect/Protobuf APIs after editing `proto/`:

```bash
buf lint
buf generate
```

Generated Go and TypeScript API sources are ignored by Git. Run
`buf generate` after cloning and whenever a protobuf contract changes.

## Building

```bash
wails build
# or
./scripts/build.sh
```

Cross-platform helpers live under `scripts/` (`build-all.sh`,
`build-macos-arm.sh`, etc.). Output: `build/bin/`.

Linux/Ubuntu 24.04: `wails build -tags webkit2_41`.

## Tests

```bash
go test ./internal/...
pnpm -C frontend typecheck
pnpm -C frontend test
```

## Configuration

Layered: defaults → optional YAML → `SHIET_*` env. Search paths:

- `~/.config/shiet/config.yaml`
- `<UserConfigDir>/shiet/config.yaml`
- `./shiet.yaml` (cwd)

See [`config.example.yaml`](config.example.yaml). Dev DB override: `SHIET_DB`
(or `db.path`). Default DB: `<UserConfigDir>/shiet/shiet.db`.

Google auth: `google.auth_mode` = `broker` (public default) or `local` (BYO
credentials). Broker base URL: `google.broker_base_url`.

## Project structure

```
.
├── app.go / main.go / integrations.go   # Native Wails adapters and app wiring
├── cmd/
│   ├── db/                              # DB migrate/seed CLI
│   └── oauth-broker/                    # Deployable OAuth broker
├── internal/api/appapi/                 # Portable Connect application handlers
├── internal/                            # Go services, DB, AI, broker, config
├── proto/shiet/                         # Versioned app and broker contracts
├── frontend/                            # React + Vite + shadcn/ui (pnpm)
├── docs/
│   ├── adr/                             # Architecture decisions
│   └── oauth-broker.md                  # Broker operator runbook
├── design-mockups/                      # HTML UI mockups
├── scripts/                             # Build, DB, sqlc, broker smoke
├── CONTEXT.md                           # Domain glossary
├── DESIGN.md                            # Product / design intent
└── AGENTS.md                            # Agent / CI / toolchain notes
```

## Docs

| Doc | Purpose |
|-----|---------|
| [DESIGN.md](DESIGN.md) | Product shape, core loop, schema intent, roadmap |
| [CONTEXT.md](CONTEXT.md) | Domain terms and decisions |
| [docs/adr/](docs/adr/) | Accepted architecture decisions |
| [docs/oauth-broker.md](docs/oauth-broker.md) | Broker env, metrics, deploy |
| [AGENTS.md](AGENTS.md) | Linear tracking, CI, common commands |
| [design-mockups/](design-mockups/) | UI redesign mockups |

## DB tooling

```bash
./scripts/db.sh          # migrate / seed / reset a dev DB
./scripts/sqlc-gen.sh    # regenerate sqlc (never hand-edit internal/db/sqlc/**)
```

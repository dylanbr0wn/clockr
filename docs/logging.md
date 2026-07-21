# Logging

shiet uses one shared **zerolog** stack (`internal/log`) for the desktop app and
the OAuth broker. Logs are structured JSON with ADR-0001 secret redaction
(sensitive keys and token-shaped values become `[redacted]`). See
[ADR-0001](adr/0001-secret-only-google-oauth-broker.md) (observability /
redaction requirements).

## Desktop app

### Where is the log file?

Default path (sibling of the SQLite DB):

```text
<UserConfigDir>/shiet/shiet.log
```

Typical locations:

| OS | Default log path |
|----|------------------|
| macOS | `~/Library/Application Support/shiet/shiet.log` |
| Linux | `~/.config/shiet/shiet.log` |
| Windows | `%AppData%\shiet\shiet.log` |

Size-based rotation keeps ~8MB active files and up to 2 backups beside the
active log. In `wails dev`, the same events also mirror to stderr; production
builds write the file only.

### Config / env overrides

Layered like the rest of app config (defaults → YAML → `SHIET_*` env). See
[`config.example.yaml`](../config.example.yaml).

| Key | Env | Notes |
|-----|-----|-------|
| `log.path` | `SHIET_LOG_PATH` | Absolute or `~`-expanded path to the log file |
| `log.level` | `SHIET_LOG_LEVEL` | `trace` \| `debug` \| `info` \| `warn` \| `error` \| `fatal` \| `panic` \| `disabled` (default: `info`) |

Example:

```yaml
log:
  path: ~/.config/shiet/shiet.log
  level: info
```

```bash
SHIET_LOG_PATH=/tmp/shiet-debug.log SHIET_LOG_LEVEL=debug wails dev -tags webkit2_41
```

### Reveal from Settings

**Settings → General → Logs** shows the configured log file path and a button
to open the folder that contains it in the OS file manager:

- macOS: **Reveal in Finder**
- Linux / Windows: **Open log folder**

The displayed path matches `log.path` (or the default above). Opening the folder
works even if `shiet.log` does not exist yet (directory is created on first
write / reveal). Shipped in [DYL-122](https://linear.app/dylans-apps/issue/DYL-122/settings-reveal-log-folder).

## Failure logs

Desktop and shared failure emitters log `op` plus a fixed `reason` code — never
`err.Error()`, response body snippets, or URL query strings from provider/HTTP
paths. Reason codes:

`unauthorized` · `forbidden` · `not_found` · `rate_limited` · `network` ·
`invalid_config` · `unknown`

Key-based redaction and Google-shaped `LooksLikeSecret` remain a passive safety
net only; do not rely on expanding token prefixes.

## OAuth broker

The broker writes the same redacting JSON format to **stdout** (deploy-friendly;
no on-disk sink). Operator details, safe fields, and metrics live in
[oauth-broker.md](oauth-broker.md#observability).

```bash
go run ./cmd/oauth-broker
# → JSON lines on stdout via internal/log (ADR-0001 redaction)
```

## Out of scope

Explicitly deferred (not part of the shared-logging epic):

- In-app log viewer
- Cloud / remote log shipping
- Frontend console bridge / Error Boundary → Go logs

Frontend UI stays sparse: log on the Go side; surface only user-actionable
errors in the UI.

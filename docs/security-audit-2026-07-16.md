# Security Audit Report — shiet

**Repository:** `dylanbr0wn/shiet`  
**Date:** 2026-07-16  
**Scope:** Full codebase at HEAD on `main` (race conditions, business logic, infrastructure, data exposure, OAuth flows)  
**Auditor:** Automated security review  

> **Finding archive.** Remediation dispositions live in Linear ([DYL-170](https://linear.app/dylans-apps/issue/DYL-170/july-16-security-audit-remediation-map)). This document merges the July 16 audit artifacts from [PR #85](https://github.com/dylanbr0wn/shiet/pull/85) (`SECURITY_AUDIT.md` + `docs/security-audit-2026-07-16.md` on `cursor/vulnerability-findings-management-ae8d`) into a single permanent record. It is not the decision record.

---

## Executive Summary

The shiet codebase demonstrates good security hygiene overall. SQL queries are fully parameterized via sqlc, OAuth templates use `html/template` (auto-escaping), the broker validates handoff redirect URLs against loopback, and secrets are stored in the OS keychain rather than the database. No **critical** vulnerabilities were found.

Findings below cover desktop-app surfaces (export templates, AI client, Wails bindings) and the oauth-broker (metrics, TLS termination, logging, rate limiting). Several items are already mitigated by the single-user desktop threat model or by existing controls; others were dispositioned separately in Linear.

---

## 1. SQL Injection

### Result: **No issues found**

- **Database layer** (`internal/db/db.go`): Connection opened with a hardcoded DSN format string using `file:<path>?_pragma=...`. The `path` comes from config (file, env, or computed default), never from untrusted HTTP/IPC input.

- **Query layer**: All 18 `.sql` files under `internal/db/query/` use sqlc named parameters (`?`). The generated Go code in `internal/db/sqlc/*.go` exclusively uses `QueryRowContext`/`QueryContext`/`ExecContext` with positional parameters. No string concatenation in any query.

- **Raw SQL outside sqlc**: Only three instances found, all in `_test.go` files (`project_test.go:195`, `project_test.go:232`, `expected_time_test.go:251`). These use parameterized `ExecContext` with `?` placeholders and hardcoded column names—safe.

- **Migrations**: All 22 migration files contain only DDL (`CREATE TABLE`, `ALTER TABLE`, `INSERT INTO ... VALUES`). No dynamic SQL, no user input.

---

## 2. Command Injection

### Finding: **MEDIUM — `openFolder` passes unchecked directory path to OS command**

**File:** `open_folder.go:14-26`

```go
func openFolder(dir string) error {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin":
        cmd = exec.Command("open", dir)
    case "windows":
        cmd = exec.Command("explorer", dir)
    default:
        cmd = exec.Command("xdg-open", dir)
    }
    if err := cmd.Start(); err != nil { ... }
}
```

**Attack path:** `openFolder` is called from `App.RevealLogFolder()` (app.go:95) with `filepath.Dir(a.logPath)`. The log path originates from config (`cfg.Log.Path`), which can be set via the YAML config file or `SHIET_LOG_PATH` env var. On Linux, `xdg-open` will interpret certain path patterns as URIs (e.g., `http://...`), though `filepath.Dir()` makes the actual exploitation path narrow.

**Existing mitigations:**
- The `dir` value is `filepath.Dir(a.logPath)` — the log path is set at app startup from config and cannot be changed at runtime via IPC.
- Wails frontend JS cannot call `openFolder` directly; it can only call `RevealLogFolder()`, which derives the path internally.
- `exec.Command` passes `dir` as a single argument, not through a shell, so shell metacharacters (`; && |`) are not interpreted.

**Severity: Low (effectively mitigated)**  
The input is config-derived, not user-controlled at runtime. The `exec.Command` argument list prevents shell injection. No real exploitable attack path exists.

---

## 3. Path Traversal

### Finding: **MEDIUM — `SaveExportFile` writes to any OS-dialog-selected path without validation**

**File:** `app.go:244-262`

```go
func (a *App) SaveExportFile(defaultFilename, content string) (string, error) {
    path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{...})
    ...
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil { ... }
    return path, nil
}
```

**Attack path:** A malicious frontend (or a compromised Wails webview) could invoke `SaveExportFile` with attacker-controlled `content`. However, the `path` comes from the **native OS save dialog** (`runtime.SaveFileDialog`), which the user interacts with visually. The `content` parameter is the rendered export payload, which the frontend constructs from data returned by the Go backend's `RenderPeriodExport`.

**Existing mitigations:**
- Path selection goes through the native OS file dialog—the user physically chooses the destination.
- Content is rendered server-side from the period model; the frontend passes `defaultFilename` and the rendered content string.
- In the Wails threat model, the frontend is trusted (it's compiled into the binary, not served from a remote origin).

**Severity: Low (effectively mitigated)**  
The native dialog prevents arbitrary path writes. Content is backend-generated. This would only be exploitable if the Wails webview itself were compromised, which is outside the app's threat model.

---

## 4. SSRF

### Finding: **MEDIUM — User-configured AI endpoint URL is fetched without network restriction**

**Files:**
- `internal/ai/client.go:30-34` (NewClient)
- `internal/ai/client.go:45-47` (ListModels: `GET BaseURL+"/models"`)
- `internal/ai/client.go:110` (Validate: `POST BaseURL+"/chat/completions"`)
- `internal/ai/client.go:171` (ChatCompletion: `POST BaseURL+"/chat/completions"`)
- `app.go:201-204` (Wails binding: `ListAIModels(baseURL, apiKey)`)
- `app.go:207-209` (Wails binding: `ValidateAIConfig(baseURL, apiKey, model)`)

```go
func NewClient(baseURL, apiKey string) *Client {
    return &Client{
        BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
        APIKey:  strings.TrimSpace(apiKey),
        HTTP:    &http.Client{Timeout: defaultTimeout},
    }
}
```

**Attack path:** The user (or compromised frontend) supplies an arbitrary `baseURL` to `ListAIModels` or `ValidateAIConfig` via Wails bindings. The Go backend makes HTTP requests to that URL. An attacker controlling the webview could set `baseURL` to `http://169.254.169.254/latest/meta-data/` (cloud metadata), `http://127.0.0.1:6379/` (local Redis), or any internal service. The response body is partially returned to the caller (model list, or validation error message containing the response body prefix).

**`ClassifyEndpoint` is advisory only** (`internal/ai/classify.go`): It labels URLs as "local" vs. "cloud" for privacy UX, but does not block requests. A cloud URL is still fetched; classification only controls how much event context is sent to the LLM.

**Existing mitigations:**
- This is a **desktop app**, not a multi-tenant web service. The "attacker" must already have access to the user's desktop.
- The HTTP client has a 30-second timeout and reads are bounded.
- Response data is parsed as JSON (model list or chat response), limiting exfiltration of non-JSON responses.
- The Wails webview is same-origin and compiled into the binary.

**Severity: Medium**  
In a desktop context this is low risk (the user themselves configure the URL). However, if the app were ever exposed as a service, or if the webview were compromised (e.g., via a future XSS in rendered calendar data), this becomes a real SSRF. Consider adding an allowlist/blocklist for the AI endpoint URL (e.g., reject `169.254.x.x`, `10.x.x.x`, `192.168.x.x` unless explicitly local, reject non-HTTP(S) schemes).

### Secondary SSRF vector: Bitbucket pagination follows server-supplied URLs

**Files:**
- `internal/integration/bitbucket/provider.go:242` — `nextURL = strings.TrimSpace(page.Next)`
- `internal/integration/bitbucket/provider.go:287` — same pattern for repos
- `internal/integration/bitbucket/evidence.go:122` — same pattern for commits

The `Next` URL comes from the Bitbucket API response JSON. `getAbsoluteJSON` fetches it without validating that it remains within the `api.bitbucket.org` domain.

**Existing mitigations:**
- The initial URL is always constructed from the hardcoded `apiBaseURL` (`https://api.bitbucket.org/2.0`).
- The Bitbucket API is trusted; a malicious `Next` URL would require either MITM on the TLS connection or a Bitbucket API vulnerability.
- Requests carry the user's OAuth bearer token, which limits the blast radius to APIs that accept Bitbucket tokens.

**Severity: Low**  
The upstream API is trusted over TLS. In practice, this is the standard pagination pattern for all Bitbucket API clients. A defense-in-depth improvement would be to validate that `page.Next` shares the same origin as `apiBaseURL`.

---

## 5. API Security

### Result: **No issues found**

- **Connect service handlers** (`internal/api/appapi/`): All endpoints validate required fields, use `invalidArgument` for bad input, and delegate to the service layer. No mass assignment—proto message fields are explicitly mapped.

- **Settings API** (`SetSetting`/`GetSetting`): The key is caller-controlled, but the value is stored as opaque JSON in a single `app_setting` table. No key-based privilege escalation is possible because all settings share the same trust level (local desktop app, single user). The SQL uses parameterized queries.

- **Integration connections** expose: `id`, `provider`, `account_label`, `account_id`, `scopes`, `status`, `connected_at`, `updated_at`. No tokens or secrets are returned. Tokens live exclusively in the OS keychain (`secrets.KeyringStore`).

- **Export endpoints**: `RenderPeriodExport` and `BuildPeriodExport` require a `period_id`. There's no authorization check, but this is a single-user desktop app where all periods belong to the same user.

---

## 6. Template Injection / XSS

### Finding: **HIGH — Server-Side Template Injection (SSTI) via `text/template` in custom export templates**

| Field | Value |
|---|---|
| **File** | `internal/service/export_template_crud.go` (lines 264-270), `internal/service/export.go` (lines 297-316) |
| **Severity** | **High** (single-user desktop context: medium/low exploitable today) |
| **Category** | Business Logic / Code Execution |

**Description**

User-supplied export template bodies for the `text` format are parsed and executed with Go's `text/template` package, not `html/template`. `text/template` provides **unrestricted access to public methods on any value passed into the template context**.

The template data type is `textSummaryData`, which embeds `PeriodExportModel`. While the immediate struct fields are plain data, Go templates can chain method calls on any accessible field. More critically, `text/template` provides no output escaping, and the template itself is user-controlled. An attacker who crafts a malicious template body could:

1. Call methods on exposed data types that have side effects.
2. Access any exported method reachable through the data graph.
3. At minimum, exfiltrate arbitrary field data that might not normally be exposed through the API (e.g., internal IDs, descriptions from other entries that share the period).

**Attack path**

1. User creates or updates a custom export template with `format: "text"`.
2. User provides a crafted `body` containing `text/template` directives.
3. The `normalizeTextBody` function (line 264) validates the template parses but does not restrict what actions/functions/methods are callable.
4. When `PreviewExport` or `RenderPeriodExport` runs, the template executes against live period data.

**Existing mitigations**

- The `FuncMap` is restricted to three formatting helpers (`duration`, `signedDuration`, `hoursPerDay`).
- The data struct has no exported methods with side effects today.
- Output is saved to a file (not rendered in a browser), so XSS is not a concern here.
- This is a single-user desktop app — the user attacking themselves limits the blast radius.

**Assessment**

In the current single-user desktop context this is **medium** risk — the user is both attacker and victim. If multi-tenancy were ever added, or if templates are shared/imported, this would escalate to **critical** (arbitrary template execution against another user's data). The recommended fix is to sandbox the template, flatten execute data to plain maps, or switch to a restricted expression language.

### OAuth pages: **No issues found**

- `internal/oauthpages/render.go` uses `html/template`, which auto-escapes `{{.ProviderName}}`, `{{.Message}}`, and `{{.HandoffURL}}` in HTML context.
- In the success page, `{{.HandoffURL}}` appears in `href` and `meta http-equiv="refresh"` attributes. `html/template` escapes these appropriately for attribute context.
- `{{.Styles}}` is typed as `template.CSS`, which is the correct safe type for inline CSS.
- Fallback pages (`fallbackSuccessPage`, `fallbackErrorPage`) use `html.EscapeString()` explicitly.

### Broker handoff redirect: **Properly validated**

- `validateDesktopHandoffRedirect` restricts to `http` scheme, `127.0.0.1` hostname only, requires a path, and rejects query/fragment/userinfo. This prevents open redirect attacks.

---

## 7. Metrics Endpoint Exposed Without Authentication

| Field | Value |
|---|---|
| **File** | `internal/broker/httpapi/server.go` (lines 98, 124-131) |
| **Severity** | **Medium** |
| **Category** | Infrastructure / Data Exposure |

### Description

The broker's `GET /metrics` endpoint serves Prometheus counters to any caller without authentication or IP restriction. The metrics include:

- Aggregate auth start/failure/success counts
- Rate-limit and kill-switch activation counts per surface
- Handoff failure reasons (including `state_mismatch`, `already_used`, `expired`)
- Quota-risk signal counts (`handoff_replay`, `handoff_mismatch`, `invalid_grant`)

### Attack Path

1. Attacker discovers the public broker origin (e.g., `auth.shiet.app`).
2. `GET /metrics` returns operational counters.
3. Attacker monitors rate-limit and quota-risk counters to fingerprint active usage patterns, determine total user count, gauge when abuse mitigations are near thresholds, and detect when the kill switch is active.

### Existing Mitigations

- No tokens or secrets are in metrics values; only aggregate counters.
- Railway deployment may have network-level restrictions (not verified).

### Assessment

Operational metadata leakage. An attacker gains reconnaissance intel about broker load and abuse patterns. Recommend either removing the endpoint in production, restricting by IP/network, or requiring a bearer token.

---

## 8. Broker HTTP Server Listens Without TLS (Plain HTTP)

| Field | Value |
|---|---|
| **File** | `cmd/oauth-broker/main.go` (line 52) |
| **Severity** | **Medium** |
| **Category** | Infrastructure / Transport Security |

### Description

The broker binary calls `srv.ListenAndServe()` (plain HTTP), not `ListenAndServeTLS`. This means the broker process itself accepts cleartext HTTP connections. OAuth tokens, client secrets (in token exchange POST bodies), and handoff codes traverse this plaintext channel between the reverse proxy and the broker.

### Attack Path

If the Railway deployment terminates TLS at the edge (load balancer), traffic between the edge and the container is unencrypted. A compromised co-tenant or network tap within the Railway infrastructure could intercept:
- Authorization codes
- Handoff codes and verifiers
- Access/refresh tokens in handoff and refresh responses

### Existing Mitigations

- Railway's proxy typically adds TLS at the edge; internal traffic may be over a private network.
- `SHIET_BROKER_PUBLIC_ORIGIN` must be HTTPS (validated), so external clients always connect via TLS.

### Assessment

Standard for PaaS deployments where the platform handles TLS termination. Risk is internal-network interception. If Railway's internal network is untrusted, consider enabling in-process TLS or using a service mesh.

---

## 9. `LooksLikeSecret` Heuristic Misses Non-Google Tokens

| Field | Value |
|---|---|
| **File** | `internal/log/redact.go` (lines 37-44) |
| **Severity** | **Medium** |
| **Category** | Data Exposure |

### Description

The value-based secret detection function `LooksLikeSecret` only recognizes Google token patterns (`ya29.*` for access tokens, `1//*` for refresh tokens). GitHub, Slack, and Bitbucket tokens have different prefixes:
- GitHub: `gho_*`, `ghu_*`, `ghp_*`
- Slack: `xoxb-*`, `xoxp-*`, `xoxa-*`
- Bitbucket: opaque OAuth tokens with no standard prefix

If a token value from these providers appears in a log field whose **key name** is not in the sensitive key list, it will be logged in plaintext.

### Attack Path

1. A code path logs an error or debug message that includes a non-Google token in a field with a non-sensitive key name (e.g., `"response_body"`, `"detail"`, `"context"`).
2. The key-based check misses it, the value-based check only matches Google patterns.
3. The token appears in logs on disk or stdout.

### Existing Mitigations

- Key-based redaction covers most common field names (`access_token`, `refresh_token`, `token`, `*_token`, `*_secret`).
- The broker code is disciplined about which fields it logs — current logging does not appear to log raw token values under non-sensitive keys.

### Assessment

Defense-in-depth gap. The key-based redaction likely catches all current logging paths, but the value heuristic creates a false sense of completeness. Recommend auditing emitters so token-capable errors never reach logs, and/or adding prefix patterns for all supported providers.

---

## 10. No CORS Policy on Broker API

| Field | Value |
|---|---|
| **File** | `internal/broker/httpapi/server.go` (lines 92-103) |
| **Severity** | **Low** |
| **Category** | Infrastructure |

### Description

The broker's HTTP handler has no CORS middleware. The Connect RPC endpoints (`brokerv1connect.NewOAuthBrokerServiceHandler`) and JSON endpoints accept requests from any origin.

### Attack Path

A malicious webpage could issue cross-origin POST requests to the broker's Connect endpoints (start authorization, exchange handoff, refresh token). However:
- `startAuthorization` requires a valid desktop session ID and handoff challenge (attacker doesn't know these).
- `exchangeHandoff` requires the handoff code, verifier, session ID, and state (all secret to the desktop).
- `refreshToken` requires a valid refresh token (secret to the user).

### Existing Mitigations

- All operations require caller-held secrets that a cross-origin attacker cannot obtain from the browser.
- Rate limiting applies per IP bucket.

### Assessment

Low risk due to the secret-binding design. Adding CORS `Access-Control-Allow-Origin` restrictions would be defense-in-depth but is not exploitable in the current protocol design.

---

## 11. In-Memory Rate Limiter Does Not Survive Restarts

| Field | Value |
|---|---|
| **File** | `internal/broker/ratelimit/limiter.go` (entire file), `cmd/oauth-broker/main.go` (line 35) |
| **Severity** | **Low** |
| **Category** | Infrastructure / Abuse Resistance |

### Description

The rate limiter is a pure in-memory fixed-window counter. It resets completely on process restart. Railway's `restartPolicyType: ON_FAILURE` with up to 10 retries means the limiter resets on each crash.

### Attack Path

1. Attacker sends requests that trigger rate limiting.
2. Attacker crashes the broker (e.g., via resource exhaustion if any endpoint is vulnerable, or simply waiting for a restart).
3. Rate limits reset, attacker resumes.

### Existing Mitigations

- The limiter's design is documented as "suitable for a single-replica deployment."
- Short TTLs (state: 5min, handoff: 2min) limit the window of useful abuse.
- The broker has no known crash-triggering input paths.

### Assessment

Acknowledged design tradeoff for a single-replica deployment. For production hardening, consider persisting rate-limit counters in SQLite alongside the broker state.

---

## 12. `DisallowUnknownFields` on JSON Decoding May Cause Subtle Denial

| Field | Value |
|---|---|
| **File** | `internal/broker/httpapi/server.go` (lines 1151-1155) |
| **Severity** | **Low** |
| **Category** | Robustness |

### Description

The `decodeJSON` function uses `dec.DisallowUnknownFields()`. This means if a desktop client sends any extra JSON field (e.g., a newer client version adding a field), the request is rejected with a 400 error. This is a forward-compatibility concern rather than a security vulnerability.

### Assessment

Not exploitable for data breach, but could be used to fingerprint client versions or cause selective denial of service if an attacker can MITM and inject extra fields into requests.

---

## Areas Reviewed With No Findings

### Race Conditions in Store Layer (Finding: None)

The `ConsumeOAuthState` and `ConsumeHandoff` methods in `store.go` correctly use database transactions with `WHERE used_at IS NULL` atomicity guards. The UPDATE checks `RowsAffected == 1` to detect concurrent consumption races. SQLite's serialized writes provide additional safety. The rate limiter uses `sync.Mutex` correctly. **No race condition found.**

### Handoff TOCTOU (Finding: None)

The handoff exchange in `operations.go` performs all validation and consumption within a single `ConsumeHandoff` call, which runs in a database transaction. The binding checks (session ID, state ID, handoff challenge via PKCE) happen inside the transaction before the UPDATE. The `WHERE used_at IS NULL` guard prevents double-spend. **No TOCTOU vulnerability found.**

### OAuth State/PKCE Implementation (Finding: None)

- State parameters use 32 bytes of `crypto/rand` (256 bits of entropy).
- PKCE uses S256 challenge method with 64-byte verifiers.
- The broker validates state binding, provider binding, and expiry before accepting a callback.
- Handoff codes are stored as SHA-256 hashes; the plaintext code is never persisted.
- Token payloads are encrypted with AES-256-GCM using the client secret as key material, with AAD binding to state+session+challenge.
- Desktop handoff redirect validation enforces `http://127.0.0.1` loopback only.

### Business Logic — Review Workflow (Finding: None)

Review decisions are protected by:
- Transaction isolation (all side effects within a single TX).
- Status checks (`item.Status != "open"` prevents re-resolution).
- Conflict-key deduplication (same conflict cannot produce duplicate review items).
- Action validation (unknown actions are rejected).

### Business Logic — Time Entry Validation (Finding: None)

Time entries validate:
- Period ID existence and boundary checks (day must be within period range).
- Start/end minute range (0-1440, end > start).
- Work type and billable status against allowlists.
- Period-to-entry binding in UPDATE/DELETE queries (prevents cross-period manipulation).

### Dockerfile Security (Finding: None)

- Multi-stage build with `distroless/static-debian12` final image (no shell, minimal attack surface).
- `CGO_ENABLED=0` for static binary.
- `trimpath` and `-ldflags="-s -w"` strip debug info.

### Configuration Security (Finding: None)

- Broker base URL must be HTTPS (validated).
- Broker mode clears desktop client credentials from runtime config.
- Desktop handoff URL must use a custom scheme (not http/https).
- State/handoff TTLs are capped (10min/5min maximum).

### HTML Template Safety in OAuth Pages (Finding: None)

The `oauthpages/render.go` uses `html/template` (not `text/template`), which auto-escapes values. The `HandoffURL` is rendered in `href` and `content` attributes where `html/template` applies appropriate contextual escaping. The `Styles` field uses `template.CSS` type for trusted stylesheet content.

### Calendar Sync Concurrency (Finding: None)

`SyncPeriod` and `SyncEvents` run sequentially within a single goroutine per call. The three-way merge uses a single database transaction. There is no concurrent sync mechanism that could cause data races. Multiple simultaneous sync calls would serialize at the SQLite write lock.

---

## Summary Table

| # | Category | Finding | Severity | Exploitable today? |
|---|----------|---------|----------|-------------------|
| 1 | SQL Injection | No issues | — | — |
| 2 | Command Injection | `openFolder` passes config path to OS command | Low | No (config-derived, no shell) |
| 3 | Path Traversal | `SaveExportFile` writes to dialog-selected path | Low | No (native OS dialog) |
| 4 | SSRF | AI client fetches user-configured URLs without network restrictions | Medium | Theoretical (desktop app, single user) |
| 4b | SSRF | Bitbucket pagination follows server-supplied `Next` URLs | Low | No (TLS-protected upstream) |
| 5 | API Security | No issues | — | — |
| 6 | Template Injection | SSTI via `text/template` in custom exports | High | Low (single-user desktop app) |
| 6b | XSS | OAuth pages properly escaped | — | — |
| 7 | Data Exposure | Unauthenticated broker `/metrics` | Medium | Yes (info disclosure) |
| 8 | Transport | No TLS on broker process | Medium | Depends on infra |
| 9 | Data Exposure | Log redaction misses non-Google token patterns | Medium | Only if future code logs tokens under non-standard keys |
| 10 | Infrastructure | No CORS on broker API | Low | No (secret-binding mitigates) |
| 11 | Abuse Resistance | In-memory rate limiter resets on restart | Low | Requires process restart |
| 12 | Robustness | `DisallowUnknownFields` forward-compatibility | Low | Not a security issue |

Remediation decisions for in-scope findings are tracked under [DYL-170](https://linear.app/dylans-apps/issue/DYL-170/july-16-security-audit-remediation-map).

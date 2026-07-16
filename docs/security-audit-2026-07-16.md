# Security Audit Report — shiet

**Date:** 2026-07-16  
**Scope:** Full codebase at HEAD on `main`  
**Auditor:** Automated security review  

---

## Executive Summary

The shiet codebase demonstrates good security hygiene overall. SQL queries are fully parameterized via sqlc, OAuth templates use `html/template` (auto-escaping), the broker validates handoff redirect URLs against loopback, and secrets are stored in the OS keychain rather than the database. No **critical** vulnerabilities were found. Two **medium** findings and two **low** findings are documented below with full attack-path analysis.

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

### Finding: **MEDIUM — `text/template` executes user-defined export template bodies**

**Files:**
- `internal/service/export.go:304` — `template.New("text_summary").Funcs(exportTemplateFuncs()).Parse(body)`
- `internal/service/export_template_crud.go:268` — `normalizeTextBody` validates parse-ability

```go
func normalizeTextBody(body string) (string, error) {
    body = strings.TrimRight(body, "\n")
    if strings.TrimSpace(body) == "" {
        return "", fmt.Errorf("text template body is required")
    }
    if _, err := template.New("export_text").Funcs(exportTemplateFuncs()).Parse(body); err != nil {
        return "", fmt.Errorf("invalid text template: %w", err)
    }
    return body, nil
}
```

**Attack path:** Users can create custom export templates with format "text" and an arbitrary Go `text/template` body. `text/template` (not `html/template`) is used. While the template FuncMap is restricted to three safe functions (`duration`, `signedDuration`, `hoursPerDay`), Go's `text/template` allows access to **any exported method on the data struct** via `{{.MethodName}}` or `{{call .Field}}`. The data struct is `textSummaryData` (embedding `PeriodExportModel`), which contains only plain value types (strings, ints, slices of structs). There are no methods with side effects on these types.

**However:** If a future refactor adds a method to `PeriodExportModel` or `textSummaryData` that has side effects (e.g., network calls, file I/O), the template engine could invoke it. Additionally, `text/template` does not HTML-escape output, but since this renders to CSV/text files (not HTML), XSS is not a concern here.

**Existing mitigations:**
- The template FuncMap contains only formatting helpers.
- The data struct has no exported methods with side effects.
- The output is saved to a file (not rendered in a browser).
- Template bodies are validated at save time (`normalizeTextBody`).

**Severity: Low**  
No current exploitation path. The risk is future-facing: adding methods to the template data types could inadvertently expose them to template callers. Consider using a more restricted template engine or explicitly documenting that `textSummaryData` must remain side-effect-free.

### OAuth pages: **No issues found**

- `internal/oauthpages/render.go` uses `html/template`, which auto-escapes `{{.ProviderName}}`, `{{.Message}}`, and `{{.HandoffURL}}` in HTML context.
- In the success page, `{{.HandoffURL}}` appears in `href` and `meta http-equiv="refresh"` attributes. `html/template` escapes these appropriately for attribute context.
- `{{.Styles}}` is typed as `template.CSS`, which is the correct safe type for inline CSS.
- Fallback pages (`fallbackSuccessPage`, `fallbackErrorPage`) use `html.EscapeString()` explicitly.

### Broker handoff redirect: **Properly validated**

- `validateDesktopHandoffRedirect` restricts to `http` scheme, `127.0.0.1` hostname only, requires a path, and rejects query/fragment/userinfo. This prevents open redirect attacks.

---

## Summary Table

| # | Category | Finding | Severity | Exploitable? |
|---|----------|---------|----------|--------------|
| 1 | SQL Injection | No issues | — | — |
| 2 | Command Injection | `openFolder` passes config path to OS command | Low | No (config-derived, no shell) |
| 3 | Path Traversal | `SaveExportFile` writes to dialog-selected path | Low | No (native OS dialog) |
| 4 | SSRF | AI client fetches user-configured URLs without network restrictions | **Medium** | Theoretical (desktop app, single user) |
| 4b | SSRF | Bitbucket pagination follows server-supplied `Next` URLs | Low | No (TLS-protected upstream) |
| 5 | API Security | No issues | — | — |
| 6 | Template Injection | `text/template` executes user-defined export bodies | Low | No (restricted data, no side-effect methods) |
| 6b | XSS | OAuth pages properly escaped | — | — |

---

## Recommendations

1. **AI endpoint SSRF hardening** (Medium priority): Add a transport-level check to the AI `http.Client` that rejects requests to link-local (`169.254.x.x`), cloud metadata endpoints, and non-HTTP(S) schemes. This provides defense-in-depth against webview compromise.

2. **Bitbucket pagination origin check** (Low priority): Validate that `page.Next` URLs share the same scheme+host as the initial `apiBaseURL` before following them.

3. **Export template sandboxing** (Low priority): Document that `textSummaryData` and `PeriodExportModel` must not gain exported methods with side effects. Alternatively, switch to a restricted template engine or use an explicit field whitelist.

4. **`SetSetting` key validation** (Informational): While not exploitable (single-user desktop app), consider restricting `SetSetting` to known key prefixes to prevent the frontend from writing arbitrary settings.

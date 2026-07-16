# Security Audit Report

**Repository:** `dylanbr0wn/shiet`  
**Audit Date:** 2026-07-16  
**Scope:** Race conditions, business logic, infrastructure, data exposure, OAuth flows

---

## Finding 1 — Server-Side Template Injection (SSTI) via `text/template` in Custom Export Templates

| Field | Value |
|---|---|
| **File** | `internal/service/export_template_crud.go` (lines 264-270), `internal/service/export.go` (lines 297-316) |
| **Severity** | **High** |
| **Category** | Business Logic / Code Execution |

### Description

User-supplied export template bodies for the `text` format are parsed and executed with Go's `text/template` package, not `html/template`. `text/template` provides **unrestricted access to public methods on any value passed into the template context**.

The template data type is `textSummaryData`, which embeds `PeriodExportModel`. While the immediate struct fields are plain data, Go templates can chain method calls on any accessible field. More critically, `text/template` provides no output escaping, and the template itself is user-controlled. An attacker who crafts a malicious template body could:

1. Call methods on exposed data types that have side effects.
2. Access any exported method reachable through the data graph.
3. At minimum, exfiltrate arbitrary field data that might not normally be exposed through the API (e.g., internal IDs, descriptions from other entries that share the period).

### Attack Path

1. User creates or updates a custom export template with `format: "text"`.
2. User provides a crafted `body` containing `text/template` directives.
3. The `normalizeTextBody` function (line 264) validates the template parses but does not restrict what actions/functions/methods are callable.
4. When `PreviewExport` or `RenderPeriodExport` runs, the template executes against live period data.

### Existing Mitigations

- The `FuncMap` is restricted to three formatting helpers (`duration`, `signedDuration`, `hoursPerDay`).
- This is a single-user desktop app — the user attacking themselves limits the blast radius.

### Assessment

In the current single-user desktop context this is **medium** risk — the user is both attacker and victim. If multi-tenancy were ever added, or if templates are shared/imported, this would escalate to **critical** (arbitrary template execution against another user's data). The recommended fix is to sandbox the template or switch to a restricted expression language.

---

## Finding 2 — Metrics Endpoint Exposed Without Authentication

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

## Finding 3 — Broker HTTP Server Listens Without TLS (Plain HTTP)

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

## Finding 4 — `LooksLikeSecret` Heuristic Misses Non-Google Tokens

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

Defense-in-depth gap. The key-based redaction likely catches all current logging paths, but the value heuristic creates a false sense of completeness. Recommend adding prefix patterns for all supported providers.

---

## Finding 5 — No CORS Policy on Broker API

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

## Finding 6 — In-Memory Rate Limiter Does Not Survive Restarts

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

## Finding 7 — `DisallowUnknownFields` on JSON Decoding May Cause Subtle Denial

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

## Summary

| # | Finding | Severity | Exploitable Today? |
|---|---------|----------|-------------------|
| 1 | SSTI via `text/template` in custom exports | High | Low (single-user desktop app) |
| 2 | Unauthenticated `/metrics` endpoint | Medium | Yes (info disclosure) |
| 3 | No TLS on broker process | Medium | Depends on infra |
| 4 | Log redaction misses non-Google token patterns | Medium | Only if future code logs tokens under non-standard keys |
| 5 | No CORS on broker API | Low | No (secret-binding mitigates) |
| 6 | In-memory rate limiter resets on restart | Low | Requires process restart |
| 7 | `DisallowUnknownFields` forward-compatibility | Low | Not a security issue |

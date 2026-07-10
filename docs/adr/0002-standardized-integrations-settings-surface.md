# ADR-0002: Standardized Integrations Settings Surface

- Status: Accepted
- Date: 2026-07-10
- Linear: DYL-110

## Context

shiet's backend integration platform is already generalized:

- **DYL-43** — connection registry, OS keychain token store, authenticated HTTP
  client, desktop OAuth (PKCE + loopback), and broker handoff protocol.
- **DYL-95 / DYL-97** — static `oauth.Provider` descriptors and a provider-neutral
  broker route loop so adding OAuth providers is adapter-sized work.

The frontend and Wails surface are still per-provider:

- Settings tabs in `frontend/src/components/settings/SettingsDialog.tsx` expose
  separate top-level sections for `calendars`, `github`, and `slack`.
- One-off panels (`CalendarSettings.tsx`, `GitHubSettings.tsx`, `SlackSettings.tsx`)
  duplicate connection status badges, connect/disconnect flows, and resource list
  layout.
- Wails methods in `app.go` are per-provider (`ConnectGoogle`, `ConnectGitHub`,
  `ConnectSlack`, separate auth-status introspection, separate resource APIs).

Each new integration currently requires a new settings tab, settings component,
React Query hooks, and Wails methods. That does not scale as shiet adds calendar
sources and activity evidence providers (Google Calendar, GitHub, Slack, Bitbucket,
and future providers).

Long-term, the product may support many integrations and eventually plugin-style
extensions. A plugin runtime is out of scope here, but it motivates a provider-neutral
settings shell with a narrow extension boundary.

## Decision

shiet will expose **one Integrations settings area** driven by a provider-neutral
contract. Adding a provider adds a catalog entry and a kind-specific config adapter —
**not** a new top-level settings tab.

### Information architecture

Replace the per-provider top-level tabs (`calendars`, `github`, `slack`) with a
single **Integrations** section inside Settings:

1. **Catalog** — lists available providers grouped by integration kind. Each row
   shows display name, kind label, and aggregate connection status (connected /
   needs re-auth / not connected).
2. **Detail** — opens when the user selects a provider. Shows a shared connect /
   manage shell plus a kind-specific configuration slot below.

Navigation flow:

```text
Settings → Integrations (catalog)
              ↓ select provider
         Integrations → {Provider} (detail)
              ├── shared: auth-mode copy, connect/disconnect, connection cards
              └── kind slot: calendars | repos/channels | future resources
```

**Hard rule:** adding Bitbucket (or any next activity evidence provider) adds a
catalog entry and an `activity_evidence` config adapter. It does **not** add a new
top-level settings tab.

### Integration kinds

Providers belong to one of two kinds. Both share the connect / status shell; only
the configuration slot differs.

| Kind | Role | Config slot | Examples |
|------|------|-------------|----------|
| `calendar_source` | Schedule import — events become time entries | Select calendars; optional per-calendar default category | Google Calendar (today) |
| `activity_evidence` | Gap-fill context only — AI cites activity; never auto-creates entries | Refresh + multi-select resources (repos, channels, …) | GitHub, Slack; Bitbucket (planned) |

Activity evidence providers follow the product rule in `DESIGN.md`: read-only
context for gap-fill suggestions. Entries only come from calendar events and
user-confirmed gap fills.

### Shared UI primitives

Extract duplicated patterns from the current per-provider panels into shared
components under `frontend/src/components/settings/integrations/`:

| Primitive | Responsibility |
|-----------|----------------|
| `IntegrationCatalog` | Provider list grouped by kind; row status; navigates to detail |
| `IntegrationDetail` | Detail shell: header, back link, auth-mode block, connection list, kind slot |
| `ConnectionCard` | Account label, status badge, connected-at, disconnect / reconnect actions |
| `ConnectionStatusBadge` | `connected` / `needs_reauth` / `disconnected` — single implementation |
| `ConnectActions` | Provider-keyed connect UI (OAuth button, account hint input, PAT `<details>`) |
| `AuthModeDescription` | Broker vs local/BYO status; never shows secrets or token material. Editable mode + BYO credential controls land here per [ADR-0003](0003-in-app-oauth-credential-authority.md) |
| `ResourceMultiSelect` | Scrollable list with per-row selected toggle; optional extra fields via render prop |

Kind-specific slots register against the provider id:

- `CalendarSourceConfig` — calendar import toggles + default category select (today's
  `CalendarSettings` resource section).
- `EvidenceResourceConfig` — generic evidence picker shell; provider supplies list
  data, refresh mutation, and row rendering (repo full name, channel name, etc.).

### Provider extension boundary

Two descriptor layers stay separate:

1. **`oauth.Provider`** (existing, `internal/integration/oauth`) — OAuth protocol
   metadata: endpoints, scopes, auth URL validation, refresh/revoke capabilities.
   No secrets. Shared by desktop and broker.
2. **`IntegrationDescriptor`** (new, desktop product metadata) — what Settings needs
   to render the catalog and connect UI without knowing provider internals.

Proposed `IntegrationDescriptor` shape:

```go
type IntegrationKind string

const (
    IntegrationKindCalendarSource   IntegrationKind = "calendar_source"
    IntegrationKindActivityEvidence IntegrationKind = "activity_evidence"
)

type IntegrationDescriptor struct {
    ID          string          `json:"id"`          // "google", "github", "slack"
    DisplayName string          `json:"displayName"` // "Google Calendar"
    Kind        IntegrationKind `json:"kind"`
    Connect     ConnectCapabilities `json:"connect"`
}

type ConnectCapabilities struct {
    NeedsAccountHint bool `json:"needsAccountHint"` // Google: email before OAuth
    SupportsPAT      bool `json:"supportsPAT"`      // GitHub only today
    OAuthAvailable   bool `json:"oauthAvailable"`   // runtime: broker or BYO configured
}
```

Desktop registers descriptors in Go (compiled catalog, not user plugins). Frontend
maps `descriptor.id` → kind config component via a small registry:

```typescript
const integrationConfigSlots: Record<string, ComponentType<IntegrationConfigProps>> = {
  google: CalendarSourceConfig,
  github: GitHubEvidenceConfig,
  slack: SlackEvidenceConfig,
  // bitbucket: BitbucketEvidenceConfig — no new Settings tab
};
```

**Adding a provider checklist (settings surface only):**

1. Register `IntegrationDescriptor` in the desktop catalog.
2. Implement or extend the provider adapter (`internal/integration/{provider}`).
3. Register a kind config slot component on the frontend.
4. Add provider-specific resource Wails methods if the kind slot needs them.

Do **not** add a `SettingsDialog` top-level tab.

### End-to-end API contract

Generalize connect / disconnect / auth introspection toward provider-keyed Wails
methods. Keep provider-specific resource types behind kind config slots initially.
Use expand–contract so 0.1.0 integration shipping is not blocked.

#### Shared operations (new)

```go
// ListIntegrationProviders returns the compiled product catalog.
func (a *App) ListIntegrationProviders() []IntegrationDescriptor

// GetIntegrationAuthStatus returns read-only auth mode for Settings.
// Never includes client secrets or token material.
func (a *App) GetIntegrationAuthStatus(provider string) (IntegrationAuthStatus, error)

type IntegrationAuthStatus struct {
    Mode          string `json:"mode"`          // "broker" | "local"
    BrokerBaseURL string `json:"brokerBaseUrl"` // set in broker mode
    OAuthAvailable bool  `json:"oauthAvailable"`
}

type ConnectIntegrationOptions struct {
    AccountID    string `json:"accountId,omitempty"`    // Google
    AccountLabel string `json:"accountLabel,omitempty"` // Google
    PAT          string `json:"pat,omitempty"`          // GitHub; empty → OAuth when available
}

// ConnectIntegration connects an account for the given provider.
func (a *App) ConnectIntegration(provider string, opts ConnectIntegrationOptions) (connection.Connection, error)

// DisconnectIntegration removes connection, tokens, and synced resources for an account.
func (a *App) DisconnectIntegration(provider string, accountID string) error
```

`ListIntegrationConnections()` already exists and remains the shared connection list.

#### Resource operations (stay provider-specific initially)

Kind config slots call existing typed APIs. Generic resource RPC is deferred until
a fourth evidence provider forces it.

| Provider | List | Toggle selected | Refresh | Extra |
|----------|------|-----------------|---------|-------|
| Google | `ListCalendars` | `SetCalendarSelected` | implicit in `SyncPeriod` | `SetCalendarDefaultCategory` |
| GitHub | `ListGitHubRepos` | `SetGitHubRepoSelected` | `RefreshGitHubRepos` | — |
| Slack | `ListSlackChannels` | `SetSlackChannelSelected` | `RefreshSlackChannels` | — |

#### Frontend hooks (mirror split)

Shared:

- `useIntegrationProviders`
- `useIntegrationConnections` (existing)
- `useIntegrationAuthStatus(provider)`
- `useConnectIntegration` / `useDisconnectIntegration`

Per-kind / per-provider (unchanged until a later contract ticket):

- `useCalendars`, `useSetCalendarSelected`, `useSetCalendarDefaultCategory`
- `useGitHubRepos`, `useSetGitHubRepoSelected`, `useRefreshGitHubRepos`
- `useSlackChannels`, `useSetSlackChannelSelected`, `useRefreshSlackChannels`

#### Expand–contract for existing Wails methods

Phase 1: add provider-keyed methods; implement as thin delegates to existing
`ConnectGoogle` / `ConnectGitHub` / `ConnectSlack` logic.

Phase 2: switch Integrations UI and hooks to provider-keyed methods.

Phase 3: remove per-provider connect/disconnect/auth aliases once unused.

Per-provider resource methods remain through the migration; removal is a separate
decision once a generic resource contract exists.

### Migration plan

Migration is incremental. Each phase ships without regressing connect/disconnect,
resource selection, or auth-mode behavior (broker vs BYO/PAT).

| Phase | Work | Behavior change |
|-------|------|-----------------|
| 1 | Add Integrations tab + catalog + detail shell; host existing panels as thin wrappers or via registry | None — old tabs remain |
| 2 | Extract shared `ConnectionCard`, `ConnectionStatusBadge`, `AuthModeDescription` | None |
| 3 | Add Wails provider-keyed connect/disconnect/auth (expand) | None — old methods kept |
| 4 | Migrate Google onto shared shell + `calendar_source` slot | Parity: broker/BYO copy, email connect, calendar toggles, default category |
| 5 | Migrate GitHub onto shared shell + `activity_evidence` slot | Parity: OAuth + PAT escape hatch, repo track toggles, refresh |
| 6 | Migrate Slack onto shared shell + `activity_evidence` slot | Parity: OAuth-only, channel track toggles, refresh |
| 7 | Remove `calendars` / `github` / `slack` top-level tabs; delete duplicate helpers | Navigation only |
| 8 | Switch frontend to provider-keyed Wails methods; remove aliases (contract) | API surface only |

**Bitbucket (DYL-36):** lands after the shell exists. Adds `IntegrationDescriptor`
+ `BitbucketEvidenceConfig` + provider-specific resource APIs. No new top-level tab.

**0.1.0 shipping:** this redesign does not block current integration work. Child
implementation issues can land after 0.1.0; phases 1–3 can start in parallel with
integration feature tickets.

## Options Considered

### Keep per-provider top-level tabs

- Pros: no migration; each panel is self-contained today.
- Cons: every new provider copies status/connect/list patterns; Settings nav grows
  without bound; contradicts backend generalization.
- Outcome: reject.

### Fully generic resource RPC now

- Pros: one `ListIntegrationResources(provider)` / `SetResourceSelected` pair for
  all providers.
- Cons: calendar resources carry `defaultCategoryId`; evidence resources are
  account-scoped with different refresh semantics; premature abstraction before
  Bitbucket validates the pattern.
- Outcome: defer. Kind slots call typed APIs until a fourth evidence provider
  forces generalization.

### Plugin runtime / marketplace

- Pros: third-party integrations without shipping a new binary.
- Cons: trust boundary, signing, sandboxing, and distribution are large product
  decisions; out of scope for DYL-110 and 1.0.
- Outcome: reject for now. Descriptor + slot registry is compile-time only.

## Does Not

- Build a plugin runtime or third-party plugin marketplace.
- Change OAuth scopes, token storage, or broker trust boundaries (see ADR-0001).
- Rewrite every provider settings UI in DYL-110 — implementation follows as child work.
- Block 0.1.0 integration shipping on this redesign.

## Consequences

- Settings gains a scalable Integrations area: one tab, catalog + detail, two kinds.
- Adding Bitbucket requires a catalog entry and evidence config adapter — not a new
  top-level settings tab.
- Shared UI primitives reduce duplication across Calendar, GitHub, and Slack panels.
- Wails surface moves toward provider-keyed connect/disconnect/auth while resource
  APIs stay typed until a later contract ticket.
- Child implementation issues (shell, Wails generalization, per-provider migration)
  can proceed independently after this design lands.

## References

- [ADR-0001: Secret-Only OAuth Broker](0001-secret-only-google-oauth-broker.md) —
  broker boundary, provider extension checklist (DYL-97).
- [DESIGN.md](../../DESIGN.md) — product loop, calendar scope, activity evidence rule.
- [CONTEXT.md](../../CONTEXT.md) — glossary terms for integrations settings.
- Linear: DYL-110 (this design), DYL-32, DYL-35, DYL-36, DYL-37, DYL-43, DYL-95, DYL-97.

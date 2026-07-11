# ADR-0004: Standardized Integrations Settings Surface

- Status: Accepted (amended 2026-07-10)
- Date: 2026-07-10
- Linear: DYL-110

## Context

shiet's backend integration platform is already generalized:

- **DYL-43** — connection registry, OS keychain token store, authenticated HTTP
  client, desktop OAuth (PKCE + loopback), and broker handoff protocol.
- **DYL-95 / DYL-97** — static `oauth.Provider` descriptors and a provider-neutral
  broker route loop so adding OAuth providers is adapter-sized work.

The frontend settings surface is still per-provider:

- Settings tabs in `frontend/src/components/settings/SettingsDialog.tsx` expose
  separate top-level sections for `calendars`, `github`, and `slack`.
- One-off panels (`CalendarSettings.tsx`, `GitHubSettings.tsx`, `SlackSettings.tsx`)
  duplicate connection status badges, connect/disconnect flows, and resource list
  layout.
- Connect/disconnect/auth introspection still exist as legacy Wails methods on
  `App` while resource operations already use Connect `IntegrationService`.

Each new integration currently requires a new settings tab, settings component,
React Query hooks, and duplicate transport paths. That does not scale as shiet
adds calendar sources and activity evidence providers (Google Calendar, GitHub,
Slack, Bitbucket, and future providers).

Long-term, the product may support many integrations and eventually plugin-style
extensions. A plugin runtime is out of scope here, but it motivates a provider-neutral
settings shell with a narrow extension boundary.

Application transport is defined in
[ADR-0002](0002-connect-protobuf-api-boundary.md): Connect is the sole frontend
API. Platform-specific OAuth and token storage use adapters in
[ADR-0003](0003-platform-adapters.md).

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
| `AuthModeDescription` | Broker vs local/BYO copy; never shows secrets or token material |
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
2. **`IntegrationDescriptor`** (product metadata) — what Settings needs to render
   the catalog and connect UI without knowing provider internals.

Descriptor shape (Protobuf / Go / TypeScript generated from `application.proto`):

```protobuf
message IntegrationDescriptor {
  string id = 1;
  string display_name = 2;
  IntegrationKind kind = 3;
  ConnectCapabilities connect = 4;
}

message ConnectCapabilities {
  bool needs_account_hint = 1;
  bool supports_pat = 2;
  bool oauth_available = 3;
}
```

The compiled catalog lives in Go (`internal/integration/catalog` or equivalent).
Frontend maps `descriptor.id` → kind config component via a small registry:

```typescript
const integrationConfigSlots: Record<string, ComponentType<IntegrationConfigProps>> = {
  google: CalendarSourceConfig,
  github: GitHubEvidenceConfig,
  slack: SlackEvidenceConfig,
  // bitbucket: BitbucketEvidenceConfig — no new Settings tab
};
```

**Adding a provider checklist (settings surface only):**

1. Register `IntegrationDescriptor` in the compiled catalog.
2. Implement or extend the provider adapter (`internal/integration/{provider}`).
3. Register a kind config slot component on the frontend.
4. Add provider-specific resource Connect RPCs on `IntegrationService` if the kind
   slot needs them.

Do **not** add a `SettingsDialog` top-level tab.

### End-to-end API contract

All integration settings operations use **Connect `IntegrationService`** per
[ADR-0002](0002-connect-protobuf-api-boundary.md). Handlers delegate to
`internal/integration/*` providers via platform adapters
([ADR-0003](0003-platform-adapters.md)).

#### Shared operations (`IntegrationService`)

```protobuf
rpc ListIntegrationProviders(ListIntegrationProvidersRequest)
    returns (ListIntegrationProvidersResponse);

rpc GetIntegrationAuthStatus(GetIntegrationAuthStatusRequest)
    returns (GetIntegrationAuthStatusResponse);

rpc ConnectIntegration(ConnectIntegrationRequest)
    returns (ConnectIntegrationResponse);

rpc DisconnectIntegration(DisconnectIntegrationRequest)
    returns (DisconnectIntegrationResponse);

rpc ListIntegrationConnections(ListIntegrationConnectionsRequest)
    returns (ListIntegrationConnectionsResponse);
```

`GetIntegrationAuthStatus` never includes client secrets or token material.

`ConnectIntegrationRequest` carries provider-specific optional fields:
`account_id`, `account_label` (Google), `pat` (GitHub; empty → OAuth when
available).

Handlers on desktop invoke the same provider `Connect` / `Disconnect` logic used
today; OAuth browser open and keychain writes happen inside the handler, not via
Wails exports.

#### Resource operations (stay provider-specific initially)

Kind config slots call existing typed Connect RPCs. Generic resource RPC is
deferred until a fourth evidence provider forces it.

| Provider | List | Toggle selected | Refresh | Extra |
|----------|------|-----------------|---------|-------|
| Google | `ListCalendars` | `SetCalendarSelected` | implicit in `SyncPeriod` | `SetCalendarDefaultCategory` |
| GitHub | `ListGitHubRepos` | `SetGitHubRepoSelected` | `RefreshGitHubRepos` | — |
| Slack | `ListSlackChannels` | `SetSlackChannelSelected` | `RefreshSlackChannels` | — |

#### Frontend hooks (Connect only)

Shared:

- `useIntegrationProviders`
- `useIntegrationConnections`
- `useIntegrationAuthStatus(provider)`
- `useConnectIntegration` / `useDisconnectIntegration`

Per-kind / per-provider (unchanged until a later contract ticket):

- `useCalendars`, `useSetCalendarSelected`, `useSetCalendarDefaultCategory`
- `useGitHubRepos`, `useSetGitHubRepoSelected`, `useRefreshGitHubRepos`
- `useSlackChannels`, `useSetSlackChannelSelected`, `useRefreshSlackChannels`

All hooks call `shietService` wrappers that use the Connect client only.

#### Legacy Wails methods (deprecate)

Per-provider Wails connect/disconnect/auth methods on `App` are **deprecated**.
Do not extend them. Remove once Connect handlers and frontend hooks ship
(supersedes DYL-113 Wails expand work).

### Migration plan

Migration is incremental. Each phase ships without regressing connect/disconnect,
resource selection, or auth-mode behavior (broker vs BYO/PAT).

| Phase | Work | Behavior change |
|-------|------|-----------------|
| 1 | Add Integrations tab + catalog + detail shell; host existing panels as thin wrappers or via registry | None — old tabs remain |
| 2 | Extract shared `ConnectionCard`, `ConnectionStatusBadge`, `AuthModeDescription` | None |
| 3 | Add Connect RPCs for provider-keyed connect/disconnect/auth/catalog | None — legacy Wails kept until frontend switches |
| 4 | Migrate Google onto shared shell + `calendar_source` slot | Parity: broker/BYO copy, email connect, calendar toggles, default category |
| 5 | Migrate GitHub onto shared shell + `activity_evidence` slot | Parity: OAuth + PAT escape hatch, repo track toggles, refresh |
| 6 | Migrate Slack onto shared shell + `activity_evidence` slot | Parity: OAuth-only, channel track toggles, refresh |
| 7 | Remove `calendars` / `github` / `slack` top-level tabs; delete duplicate helpers | Navigation only |
| 8 | Remove legacy Wails integration methods and `readFromBackend` paths | API surface only |

**Bitbucket (DYL-36):** lands after the shell exists. Adds `IntegrationDescriptor`
+ `BitbucketEvidenceConfig` + provider-specific resource Connect RPCs. No new
top-level tab.

**0.1.0 shipping:** this redesign does not block current integration work. Child
implementation issues can land after 0.1.0.

## Options Considered

### Keep per-provider top-level tabs

- Pros: no migration; each panel is self-contained today.
- Cons: every new provider copies status/connect/list patterns; Settings nav grows
  without bound; contradicts backend generalization.
- Outcome: reject.

### Wails methods for connect/disconnect/auth (DYL-113 original)

- Pros: fastest path using existing `App` bindings.
- Cons: permanent dual transport; contradicts Connect-first direction for desktop
  + hosted; DYL-113 Wails work becomes throwaway.
- Outcome: **superseded** by Connect `IntegrationService` RPCs.

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
- Add new Wails-bound integration APIs.

## Consequences

- Settings gains a scalable Integrations area: one tab, catalog + detail, two kinds.
- Adding Bitbucket requires a catalog entry and evidence config adapter — not a new
  top-level settings tab.
- Shared UI primitives reduce duplication across Calendar, GitHub, and Slack panels.
- Integration connect/disconnect/auth/catalog use the same Connect transport as
  resource lists and the rest of the application API.
- Child implementation issues (shell, Connect generalization, per-provider migration)
  can proceed independently after this design lands.

## References

- [ADR-0001: Secret-Only OAuth Broker](0001-secret-only-google-oauth-broker.md) —
  broker boundary, provider extension checklist (DYL-97).
- [ADR-0002: Connect and Protobuf API Boundary](0002-connect-protobuf-api-boundary.md) —
  sole application transport.
- [ADR-0003: Platform Adapters](0003-platform-adapters.md) — keychain, browser, export.
- [DESIGN.md](../../DESIGN.md) — product loop, calendar scope, activity evidence rule.
- [CONTEXT.md](../../CONTEXT.md) — glossary terms for integrations settings.
- Linear: DYL-110 (this design), DYL-32, DYL-35, DYL-36, DYL-37, DYL-43, DYL-95, DYL-97.

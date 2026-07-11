# shiet — Design

shiet summarizes time spent per pay period: import calendar → categorize events →
fill gaps → export a timesheet report.

## Shape

- **Local desktop app**, distributed as a download. No user accounts, no
  multi-tenant hosting, no org/admin concept. Each install is one independent
  user ("any company" = anyone can download and run it).
- **Privacy-first**: calendar events, activity, and reports stay on the user's
  machine. The only hosted piece is the **secret-only Google OAuth broker**
  ([ADR-0001](docs/adr/0001-secret-only-google-oauth-broker.md)): it holds the
  shared Google `client_secret`, never durable Google tokens or Calendar data.
- **Bring-your-own-model (BYOM)**: user supplies a cloud key or points at a local
  model (OpenAI-compatible HTTP).

## Stack

| Layer | Choice |
|---|---|
| Shell | **Wails v2** (Go core + web frontend) |
| Frontend | **React** + TypeScript (Wails bindings) |
| Storage | **SQLite** (local file, pure-Go `modernc.org/sqlite`) |
| Calendar | **Google Calendar** via official Go client + `x/oauth2` |
| Google auth | **Broker** (public default) or **BYO/local** loopback+PKCE — see ADR-0001 |
| Token storage | **OS keychain** (`go-keyring`) |
| AI | **Go backend → HTTP**: OpenAI-compatible base-URL + key (OpenAI, proxies, Ollama/LM Studio) |

Secrets and AI calls live in the Go layer, never the webview.

## Core loop

### 1. Import (Google Calendar)
- Sign in with Google (broker handoff for public builds, or BYO/local OAuth);
  one consent grants calendar read (offline/refresh). Tokens land in the OS
  keychain only.
- Pull events for the current pay period.
- **Event rules** (defaults, editable in settings):
  - Declined → excluded.
  - Accepted → included.
  - Tentative / not-responded, all-day, and **overlapping** events → **flagged for
    explicit user resolution** (no silent double-counting; overlap = "pick which
    category owns this interval").

### 2. Categorize (layered)
Categories are **user-defined, free-form** (every company's buckets differ).
Assignment runs in layers, fewest clicks over time:
1. **Memory** — event seen before → auto-apply its prior category.
   - Match key: Google `recurringEventId` for series; normalized title (+ organizer)
     for one-offs.
2. **AI** — novel events get a suggested category from title/attendees/description.
3. **Override** — user confirms/corrects; corrections train the memory.

### 3. Gap-fill (interval timeline)
- Pay period config: **cadence** (weekly / bi-weekly / semi-monthly / monthly) +
  **anchor date** + **target hours/day**.
- A day is a **timeline** within a default working window (derived from target).
  Events occupy intervals; **uncovered intervals are gaps** (multiple per day possible).
- **One-click assign**: each gap → assign remaining hours to a category, with a
  remembered default gap-category. Splittable.
- **Extend / manual add**: window is a default, not a cap — user can extend hours
  (overtime) or add a manual block anywhere.
- **Activity integrations (later)**: GitHub, Slack, etc. are **evidence-only** —
  read-only context the AI cites to suggest a category/description per gap interval.
  They never auto-create entries. Entries only ever come from calendar events +
  user-confirmed gap fills.

## Integrations settings

All third-party connections are managed in one **Integrations** area inside
Settings — not as separate top-level tabs per provider.

- **Catalog + detail** — the user picks a provider from a list grouped by kind,
  then opens a detail view to connect accounts and configure resources.
- **Calendar sources** (`calendar_source`) — connect an account, choose which
  calendars to import, optionally map each calendar to a default category. Today:
  Google Calendar.
- **Activity evidence providers** (`activity_evidence`) — connect an account,
  refresh and select resources (repos, channels, …) the AI may cite during
  gap-fill. Evidence never auto-creates time entries.
- **Adding providers** — a new provider (e.g. Bitbucket) adds a catalog entry
  and a kind-specific config panel. It does **not** add a new top-level settings
  tab.

See [ADR-0004](docs/adr/0004-standardized-integrations-settings-surface.md) for
the settings IA, shared UI primitives, Connect API contract, and migration plan.

### 4. Export
- **On-screen summary**: period totals by category + per-day breakdown, copyable.
- **CSV**: category × hours, per day and per period.
- Later: PDF, per-company templates.

## Calendars (multi-source)

- **Scope** — selectable set. List all calendars the account can read; user toggles which
  count. **Defaults to primary only**; add team/project calendars as needed. Keeps
  holidays/birthdays/coworkers out unless opted in.
- **Per-calendar default category** — optional mapping (calendar → category). Mapped-cal
  events pre-fill that category. Layer precedence:
  `manual override > memory > calendar default > AI suggestion`.
  Strong starting signal, still overridable; a past correction (memory) wins over the map.
- **Deselect mid-period** — **soft-hide + retain**: events excluded from totals but data +
  overlays kept; re-selecting restores. Honors "never lose user work", survives accidental
  toggles. (`event` gets a hidden/active state.)
- **Cross-calendar dedup** — same meeting on two selected calendars shares an `iCalUID` but
  has different per-calendar event ids. Dedup is **layered**:
  - Exact `iCalUID` match → confident merge (keep attendee/organizer copy).
  - Else **heuristic** (normalized title + identical start/end + attendee overlap) →
    probable dupe; confident → merge, ambiguous → review queue.
  - Overlay attaches to per-calendar event id; cross-calendar identity = `iCalUID`.

## Timezones

TZ only matters for **placement** (which day an event falls in, where it sits in the
working window) — durations are TZ-invariant.

- **Canonical bucketing TZ** — a configured work TZ, defaulting to the device TZ at period
  creation. Stored as **IANA name** (`America/Toronto`), never a fixed offset, so DST is
  handled (a period crossing DST has a correct 23h/25h day via TZ-aware math, e.g. Go
  `time.LoadLocation`).
- **TZ schedule** — a period holds an ordered list of date-anchored segments
  (`effective-from date → IANA TZ`) covering it. Default = one segment (device TZ). Each
  day buckets by its active segment → handles split-location periods (half here, half
  there). The travel day uses whichever segment its date falls in; manual block editor
  fixes odd travel days.
- **Event storage** — UTC instant + original IANA TZ from Google. Placement into day +
  window always uses the **active segment TZ** (where the user was), not the event's own TZ.
- **All-day events** — date-only / floating: bucketed to that literal date, no conversion.
- Later nicety: auto-suggest segment boundaries from detected device-TZ changes.

Schema delta: `period` drops a single TZ field in favor of
`tz_segment(id, period_id, effective_from_date, iana_tz)`.

## Persistence & period model

A period is a **live editable working record**; finalizing freezes an **immutable
submission**. Two concepts, separated: mutable `period` + immutable `submission`.

- **Period identity** — boundaries are deterministic from cadence + anchor. A `period`
  row is created lazily on first open of a date range, looked up by range after.
- **Layered overlay** — imported calendar events are stored as synced *facts*; user
  *decisions* (category override, resolved overlap, manual blocks, gap fills) are stored
  separately and re-attached by **Google event id** (+ instance id for recurring
  occurrences, which share a series id but differ by `originalStartTime`).
  Invariant: **re-sync never destroys a user decision.**
- **Re-sync** — manual **Sync** button + "last synced N ago" hint. A 3-way merge
  (base = last import, theirs = new pull, mine = overlays):
  - New event → added + auto-categorized (memory/AI).
  - Unchanged event → kept; overlay preserved.
  - **Time-only** change → category kept silently (category describes *what*, not *when*).
  - **Material title** change → re-flagged for review (basis changed).
  - Deleted/declined-but-categorized → **queued for review** (suggest drop, never silent).
  - New event inside an already-filled gap → **queued** (double-count conflict).
  - Safe changes auto-apply; only genuine conflicts hit the **review queue**.
- **Finalize** — writes an immutable `submission` snapshot (events + overlays frozen).
  Re-opening is free; each finalize writes a **new version** (v1, v2…), prior versions
  retained → corrections allowed, full audit trail of what was attested and when.

Schema sketch (see `internal/db/migrations/` for the live schema):
```
period(id, start_date, end_date, cadence, anchor_date, target_hours_per_day, last_synced_at)
tz_segment(id, period_id, effective_from_date, iana_tz)
calendar(id, provider, external_id, name, is_primary, selected, default_category_id)
event(id, period_id, calendar_id, provider, external_id, instance_id,
      recurring_event_id, ical_uid, title, …, active, source_hash)  -- synced facts
overlay(id, period_id, provider, external_id, instance_id,
        category_id, resolved_overlap, note, kind)                   -- user decisions
gap_fill(id, period_id, day, start_utc, end_utc, category_id, note, source)
category(id, name, key, description, color, is_default_gap)
memory(match_key, category_id, hits)
review_item(id, period_id, kind, event_id, payload, status, conflict_key, …)
submission(id, period_id, version, finalized_at, frozen_blob)        -- immutable
app_setting(key, value)                                              -- non-secret config
```

## Model configuration (BYOM)

- **Discovery** — auto-probe known local ports on setup (Ollama `11434`, LM Studio `1234`,
  via `/v1/models`); offer detected endpoints + their model list. A manual **URL + key**
  field is always available for cloud / custom / remote endpoints.
- **Model list** — when an endpoint is reachable, fetch `/v1/models` → dropdown, with
  free-text fallback for names not listed.
- **Validate on save** — tiny test call; surface reachable/unreachable + the privacy
  verdict (local vs cloud, per the classification rules) inline at config time.

## Privacy (BYOM data handling)

Provider-aware: a **local** model gets full event context with no warning (data never
leaves the machine); a **cloud** model gets a minimized payload + disclosure.

- **Classification** — hybrid. Heuristic decides (loopback / LAN `192.168.*`,`10.*` /
  `*.local` / known ports like Ollama `11434`, LM Studio `1234` = local; else cloud).
  App shows the verdict before any call; user can override. **Fails safe**: anything
  ambiguous is treated as **cloud (leaking)**.
- **Privacy floor** — any cloud model necessarily receives the user's **category list**
  (the AI picks from it). If category names are client names, that already leaves the
  machine. Unavoidable when a cloud model does the picking.
- **Cloud payload default** — `title + attendee domains (not addresses) + duration`.
  Description **excluded by default** (higher leak, low marginal signal).
- **Controls** — full per-field toggles (title / attendees / description / location)
  live in **Settings**, defaulting to the above.
- **First-run** — a one-time privacy confirmation surfaces the data-sharing settings the
  first time a cloud model is configured; tucked into Settings thereafter.
- **Ongoing disclosure** — a **persistent ambient badge** during categorize/gap-fill:
  `Cloud · sharing title+domains` or `Private · on-device`, click → Settings. No
  per-run nag. Escalate to an explicit confirm only when enabling a higher-leak field
  (e.g. turning description on).
- **Activity integrations carry the same (higher-leak) policy** — commit messages,
  channel/thread text are sensitive; their cloud evidence payload must be minimized
  under the same rules. (Detail TBD when integrations are built.)

## Roadmap (post-MVP)
- Microsoft 365 / Outlook calendar (Graph) — the other half of "any company".
- `.ics` upload fallback; CalDAV.
- Activity integrations (GitHub, Slack, …) feeding AI gap-fill.
- PDF export, configurable report templates.
- Org/B2B layer (shared categories, admin) if ever needed.

## Open questions (not yet decided)
_None outstanding from the initial design pass._

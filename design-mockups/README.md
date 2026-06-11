# Clockr UI Redesign Mockups

shadcn/ui-styled (zinc, light mode) redesign mockups for Clockr, built per DESIGN.md goals:
import calendar → categorize → fill gaps → export.

| File | Screen |
|---|---|
| `main.html` | Period Schedule — top bar (period nav, privacy badge, sync, 1d/7d/14d toggle), scheduler grid (working-hours band, overlap layout, gap + AI-suggestion blocks, now line), sidebar (period progress, category totals, review queue, export/finalize) |
| `review.html` | Review Queue modal — overlap resolution, deleted-but-categorized event, new-event-inside-filled-gap conflict |
| `settings.html` | Settings · AI Model — BYOM endpoint discovery (Ollama/LM Studio/custom), connection fields, local/cloud privacy verdict callout, cloud data-sharing toggles |

## Preview

```bash
python3 -m http.server 4173 --directory design-mockups
# open http://localhost:4173/main.html
```

## Push into Figma

Target file: https://www.figma.com/design/HOLmlqTZ0Sw3eOXnr8qIYn (Clockr UI Redesign — already
contains shadcn variable collection + finished top bar built via use_figma).

Figma MCP on the Starter plan allows only **6 tool calls/month** — exhausted June 2026.
Once limit resets (or after plan upgrade), capture each page with the `generate_figma_design`
MCP tool against fileKey `HOLmlqTZ0Sw3eOXnr8qIYn` while the server above is running.

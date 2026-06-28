---
name: gtqa UX timing clarity
overview: Make gtqa's Fleet QA UI answer "when did it start, when will it end, how long has it run?" on Batches and Batch detail views, plus a focused pass on labels, sidebar, and progress affordances. Backend adds duration/ETA to the batch list API; frontend surfaces timing with local time + relative hints.
todos:
  - id: api-batch-timing
    content: Extend batchListItem with duration, profile, estimated_end_at; add ETA helper + tests; fix running stub to load summary from disk
    status: pending
  - id: js-time-utils
    content: Add formatDateTime, formatRelative, parseDuration, estimateBatchEnd, shortBatchId, formatBatchTiming in app.js
    status: pending
  - id: batches-table-columns
    content: "Batches table reordered: Status, Remaining, Start, End, Duration, batch_id, pass/fail, Open"
    status: pending
  - id: batch-timeline-strip
    content: Batch detail timeline strip + dual progress row + edge states (finishing, overdue, cancelled)
    status: pending
  - id: sidebar-labels
    content: Improve recent-batches sidebar with relative times; rename Active batch → Batch detail
    status: pending
  - id: ux-polish
    content: Username in jobs table, bus_drops tooltip, header running indicator, tabular-nums, a11y, minimal CSS
    status: pending
isProject: false
---

# gtqa UX: timing clarity and friendlier ops UI

## Current state (audit)

**Product:** Internal ops dashboard ([`go-trader-qa/web/`](go-trader-qa/web/)) for fleet soak QA — select subaccounts, start multi-server batches, watch jobs, read reports.

**Classifier:** APP UI (ops dashboard). Dense, scannable, utility copy.

**Aesthetic:** Industrial/utilitarian — DM Sans, Atlassian-blue sidebar, dense tables. Appropriate for ops; the gap is **information design**, not a full visual rebrand.

**What works:** 3-tab nav (Fleet · Batches · Batch detail), sortable tables, inline reports, polling every 10s, 16px body text, 44px touch targets, focus-visible rings.

**Core pain:**

| View | Today | Problem |
|------|-------|---------|
| Batches | One `started` column | No end time, no duration, no sense of "still running vs done" |
| Batch detail | `Jobs 2/3`, PASS/FAIL tags, job-count progress bar | No started/ended/ETA; progress bar is **jobs finished**, not **time remaining** |

**Data already exists** — UI just does not show it:

- Batch detail API (`GET /api/batches/{id}`) returns full [`SoakBatch`](go-trader-qa/internal/batch/types.go): `started_at`, `completed_at`, `duration`, `interval`, `profile`, `concurrency`.
- Batch list API returns `started_at` and `completed_at` in [`batchListItem`](go-trader-qa/internal/api/handlers.go) but **not** `duration`. Running in-memory stubs may lack times until summary is read from disk.

**ETA math (matches runner):** Jobs start with **30s stagger** between starts ([`defaultStagger`](go-trader-qa/internal/batch/runner.go)); each job soaks for `duration`.

```text
estimated_end = started_at + (job_count - 1) * 30s + duration
```

---

## Design direction

**Memorable thing:** *"I always know when my soak finishes and whether it's healthy."*

**Time format (confirmed):** Local time + relative hints (`14:05 · 8m left`, `Ended 12:32 · 2h ago`). Full ISO on `title` hover.

**SAFE:** Keep dense tables, status tags, DM Sans, existing CSS variables in [`app.css`](go-trader-qa/web/static/app.css).

**RISKS:**
1. Timeline strip with labeled fields (not a sentence blob)
2. Dual progress (jobs + time remaining)
3. Rename nav `Active batch` → `Batch detail`
4. Reorder Batches columns: **status and time first** (user confirmed)

---

## Information hierarchy

### Batches table — scan order (left to right)

Ops users scan for **what's running and when it ends**. Column order:

```text
status | remaining | start | end | duration | batch_id | pass | fail | actions
```

- **remaining** (sort key `estimated_end_at` or `completed_at`): running → `8m left`; complete → `2h ago`; cancelled → `cancelled`
- **end**: running → `~14:35` with `title="Estimated; includes 30s stagger between servers"`; complete → actual end time
- **batch_id**: truncated, monospace, full id on hover

### Batch detail — first viewport priority

```text
1. Status badge + friendly title (Jun 28, 12:00 soak)
2. Timeline strip (Started · Duration · Ends ~ / Ended · Profile)
3. Dual progress (jobs bar + time label)
4. PASS/FAIL/SKIPPED counts
5. Per-server jobs table
```

### ASCII wireframe — Batches

```text
┌─ Soak runs ────────────────────────────────────────────────┐
│ status   remaining  start        end       dur  batch_id │
│ RUNNING  8m left    Jun 28 14:05 ~14:35    30m  batch-…  │
│ complete 2h ago     Jun 28 10:00 12:32     30m  batch-…  │
└──────────────────────────────────────────────────────────┘
```

### ASCII wireframe — Batch detail timeline

```text
┌─ Jun 28, 12:00 soak ──────────────── running [Cancel] ──┐
│ Started      Duration    Ends ~         Profile          │
│ Jun 28 14:05  30m         ~14:35 (8m)    wss-only · c=2  │
│ Jobs 2/3  ████████░░░░░░░░░░░░░░░░░░░░░░░░  8m left      │
│ PASS 1  FAIL 0  SKIPPED 0                                │
│ [jobs table]                                              │
└───────────────────────────────────────────────────────────┘
```

---

## Interaction state table

| Feature | Loading | Empty | Error | Success | Partial |
|---------|---------|-------|-------|---------|---------|
| Batches list | "Loading runs…" | "No soak runs yet" + Fleet CTA | "Could not load soak runs" | Table with timing columns | Some rows missing `started_at` → show `—`, load from summary on next poll |
| Batches row (running) | — | — | — | `8m left`, `~end` time | ETA passed → `finishing…` (not negative countdown) |
| Batches row (complete) | — | — | — | End time + `Nm ago` | `completed_at` missing → show status only |
| Batches row (cancelled) | — | — | — | `cancelled` in remaining col | Partial jobs done |
| Batch timeline | Spinner in timeline area on first load | "No batch selected" empty state | "Batch not found" toast | Full timeline + dual progress | Running but no duration in summary → hide ETA, show "duration unknown" |
| Time remaining label | — | — | — | Updates every 10s on poll | Past ETA: `finishing…` + tooltip "Last server may still be soaking" |

---

## User journey (timing focus)

| Step | User does | User feels | Plan specifies |
|------|-----------|------------|----------------|
| 1 | Opens Batches after starting 30m soak | "When will this end?" | Remaining column shows `29m left` immediately |
| 2 | Glances sidebar while on Fleet | "Is it still running?" | Sidebar shows `running · 29m left` |
| 3 | Opens Batch detail | "What's the full picture?" | Timeline strip + dual progress, not just job count |
| 4 | Soak completes | "Done — how long did it take?" | End time + `just ended` / elapsed since end |
| 5 | Views old batch next day | "When was that?" | `Ended Jun 28, 12:32 · yesterday` |

---

## Implementation

### 1. Backend — expose timing on list endpoint

**File:** [`internal/api/handlers.go`](go-trader-qa/internal/api/handlers.go)

Extend `batchListItem`:

```go
Duration       string     `json:"duration"`
Profile        string     `json:"profile,omitempty"`
EstimatedEndAt *time.Time `json:"estimated_end_at,omitempty"` // running only
```

**Bug fix:** In `listBatches`, in-memory running stubs currently set only `ID`, `Status`, `Running`. Always merge from `loadBatchSummary(id)` when disk summary exists so `started_at`, `duration`, and counts populate immediately after batch start.

```go
func estimateBatchEnd(started time.Time, jobCount int, duration time.Duration) time.Time {
    stagger := 30 * time.Second
    if jobCount < 1 { jobCount = 1 }
    return started.Add(time.Duration(jobCount-1)*stagger + duration)
}
```

Export helper for tests. Add unit tests in [`handlers_test.go`](go-trader-qa/internal/api/handlers_test.go).

### 2. Frontend — shared time utilities

**File:** [`web/static/app.js`](go-trader-qa/web/static/app.js)

```js
// Core helpers
parseDuration(s)           // "30m" → ms; mirror Go ParseDuration subset
formatDateTime(iso)        // "Jun 28, 14:05" local
formatRelativePast(iso)    // "2h ago" / "just now"
formatRemaining(endIso)    // "8m left" | "finishing…" if past
shortBatchId(id)           // batch-20260628T120000Z → "Jun 28, 12:00"
estimateBatchEnd(...)      // client fallback matching backend

// Composite — single source for Batches row + Batch detail + sidebar
formatBatchTiming(batch, { running, jobCount }) → {
  start, end, endIsEstimate, duration, remaining, remainingPast
}
```

Use `estimated_end_at` from API when present; else client `estimateBatchEnd`.

**Edge cases:**
- Missing `started_at` → all time fields `—`
- Missing `duration` → hide ETA, show "duration unknown" in timeline
- `remaining` past zero → `finishing…`, never negative numbers
- Cancelled → remaining = `cancelled`, end = `completed_at` or `—`

### 3. Batches table

**Files:** [`web/index.html`](go-trader-qa/web/index.html), [`app.js`](go-trader-qa/web/static/app.js)

Column order (confirmed):

| Column | Header | Running | Complete |
|--------|--------|---------|----------|
| status | status | running tag | complete/cancelled tag |
| remaining | remaining | `8m left` | `2h ago` |
| start | start | `Jun 28, 14:05` | same |
| end | end | `~14:35` | actual end |
| duration | duration | `30m` | `30m` |
| id | batch_id | truncated + title | same |
| pass/fail | pass / fail | counts | counts |
| actions | — | Open | Open |

Sort keys: `status`, `estimated_end_at`, `started_at`, `completed_at`, `duration`, `id`, `pass_count`, `fail_count`.

Running rows: `.row-running` left border `3px solid var(--accent)` (no pulse animation — respects `prefers-reduced-motion`).

### 4. Batch detail — timeline strip + dual progress

**Files:** [`index.html`](go-trader-qa/web/index.html), [`app.js`](go-trader-qa/web/static/app.js), [`app.css`](go-trader-qa/web/static/app.css)

```html
<dl class="batch-timeline" id="batch-timeline">
  <!-- dt/dd pairs: Started, Duration, Ends ~ or Ended, Profile -->
</dl>
<div class="batch-progress-row" role="group" aria-label="Batch progress">
  <span class="progress-label">Jobs <strong id="batch-jobs-label">2/3</strong></span>
  <div class="progress-bar" aria-hidden="true"><div id="batch-progress"></div></div>
  <span class="progress-label" id="batch-time-label" aria-live="polite">8m left</span>
</div>
<div class="batch-outcomes" id="batch-outcomes"><!-- PASS/FAIL/SKIPPED tags --></div>
```

- h2: `shortBatchId(id)` + status badge; full id in `title`
- `aria-live="polite"` on time label only (not whole page)
- Tooltip on estimated end: "Estimated end includes 30s stagger between server soaks"

Keep PASS/FAIL/SKIPPED separate from timeline (outcomes vs schedule).

### 5. Sidebar recent batches

```text
Jun 28 12:00 · running · 8m left
Jun 27 18:00 · complete · 2 pass · 1 fail
```

Max 2 lines per item; truncate batch id if needed.

### 6. Other UX fixes

| Issue | Fix |
|-------|-----|
| Tab "Active batch" | → **Batch detail** |
| `bus_drops` | **Bus drops** + `title` tooltip |
| Jobs table | Add **username** column (fleet lookup); keep server_id |
| Header | Append `· soak running · 8m left` when batch running |
| Jobs queued/running | Show status tag; no fake time per job (out of scope) |

### 7. CSS

**File:** [`app.css`](go-trader-qa/web/static/app.css)

```css
.batch-timeline { display: grid; grid-template-columns: auto 1fr; gap: 4px 16px; padding: 12px 20px; }
.batch-timeline dt { color: var(--muted); font-size: 13px; }
.batch-timeline dd { font-weight: 600; font-variant-numeric: tabular-nums; }
.col-time { white-space: nowrap; font-variant-numeric: tabular-nums; }
.batch-progress-row { display: flex; align-items: center; gap: 12px; padding: 0 20px 12px; }
.row-running { box-shadow: inset 3px 0 0 var(--accent); }
.end-estimate::before { content: "~"; opacity: 0.7; }
```

No new fonts. No decorative cards.

### 8. Docs (optional)

One paragraph in [`docs/phase-2.md`](go-trader-qa/docs/phase-2.md) § Batch dashboard: timeline strip + Batches timing columns.

---

## What already exists

- CSS variables and DM Sans in [`app.css`](go-trader-qa/web/static/app.css)
- `formatStartedAt()` in [`app.js`](go-trader-qa/web/static/app.js) — extend, don't duplicate
- `table-wrap` horizontal scroll for wide tables
- Interaction state patterns in [`docs/phase-2.md`](go-trader-qa/docs/phase-2.md) § Web UI
- Phase 2 spec: progress bar = jobs not time (plan keeps both)

## NOT in scope

- Full `DESIGN.md` / AI mockups (designer binary unavailable; HTML wireframes in plan suffice)
- Per-job start/end timestamps (runner/schema change)
- Time-only progress bar replacing job bar
- Dark mode
- Running-batch filter chip ("Show running only")
- Fleet tab timing changes

---

## Test plan

1. `go test ./internal/api/...` — ETA helper + list JSON includes duration, estimated_end_at
2. Start 15s soak on 2 servers → Batches: `remaining` counts down, `end` shows `~`
3. After complete → `remaining` = `Nm ago`, `end` = actual time
4. Batch detail timeline matches Batches row
5. ETA past due → `finishing…` not `-3m left`
6. Cancelled batch → `remaining` = `cancelled`
7. `prefers-reduced-motion` — no animation added
8. Legacy hash routes unchanged

---

## Implementation Tasks

Synthesized from design review. P1 blocks ship.

- [ ] **T1 (P1, human: ~1h / CC: ~15min)** — API — Extend batch list timing fields + fix running stub merge
  - Surfaced by: Pass 2 (States) — running rows missing started_at from in-memory stub
  - Files: `internal/api/handlers.go`, `internal/api/handlers_test.go`
  - Verify: `go test ./internal/api/...`

- [ ] **T2 (P1, human: ~1.5h / CC: ~20min)** — Frontend — `formatBatchTiming` composite + time utils
  - Surfaced by: Pass 7 — single source of truth for Batches/detail/sidebar
  - Files: `web/static/app.js`
  - Verify: manual 15s soak countdown

- [ ] **T3 (P1, human: ~1h / CC: ~15min)** — Batches table — reordered columns + edge states
  - Surfaced by: Pass 1 — user chose status/time-first scan order
  - Files: `web/index.html`, `web/static/app.js`, `web/static/app.css`
  - Verify: scan test — running batch visible without horizontal scroll on 1280px

- [ ] **T4 (P1, human: ~1h / CC: ~15min)** — Batch detail — timeline dl + dual progress + aria-live
  - Surfaced by: Pass 1/3 — primary hierarchy for "when will it end"
  - Files: `web/index.html`, `web/static/app.js`, `web/static/app.css`
  - Verify: timeline matches Batches row; screen reader announces time updates

- [ ] **T5 (P2, human: ~30min / CC: ~10min)** — Sidebar + nav rename + jobs username column
  - Surfaced by: Pass 3 — glanceable running state from Fleet tab
  - Files: `web/index.html`, `web/static/app.js`
  - Verify: sidebar shows relative time; nav says Batch detail

- [ ] **T6 (P3, human: ~15min / CC: ~5min)** — Docs — phase-2.md timing columns paragraph
  - Surfaced by: Pass 5 — no DESIGN.md; document in phase-2 instead
  - Files: `docs/phase-2.md`
  - Verify: read-through

---

## GSTACK REVIEW REPORT

**Skill:** plan-design-review  
**Plan:** gtqa UX timing clarity  
**Classifier:** APP UI  
**Initial design score:** 6/10 (strong problem diagnosis + ETA math; weak on scan hierarchy, edge states, a11y, column order)  
**Final design score:** 9/10 (remaining gap: no DESIGN.md; acceptable for scoped UX pass)

### Pass scores

| Pass | Before | After | Notes |
|------|--------|-------|-------|
| 1 Information Architecture | 5/10 | 9/10 | Column order fixed (status/time first); ASCII wireframes added |
| 2 Interaction States | 4/10 | 9/10 | Full state table; finishing/overdue/cancelled/missing data |
| 3 User Journey | 6/10 | 9/10 | 5-step emotional arc for timing anxiety |
| 4 AI Slop Risk | 9/10 | 9/10 | No slop risk; utilitarian ops UI |
| 5 Design System | 5/10 | 8/10 | No DESIGN.md; reuses existing tokens; tabular-nums specified |
| 6 Responsive/a11y | 5/10 | 8/10 | aria-live on countdown; no pulse; table-wrap; dt/dd semantics |
| 7 Unresolved Decisions | — | 1 resolved | Batches column order → time-first (user confirmed) |

### Decisions made

1. **Batches column order:** status · remaining · start · end · duration · batch_id · pass · fail · actions
2. **Remaining column name:** `remaining` (not ambiguous `time`)
3. **Past ETA display:** `finishing…` not negative countdown
4. **Running row accent:** inset left border, no pulse (motion/a11y)
5. **Timeline markup:** `<dl>` dt/dd pairs, not freeform spans
6. **Estimated end prefix:** CSS `~` via `.end-estimate`, tooltip explains stagger

### Decisions deferred

- Running-only filter on Batches tab (P3 follow-up)
- Per-job ETA columns (needs runner changes)

### Litmus scorecard (APP UI)

| Check | Result |
|-------|--------|
| Product unmistakable in first screen? | YES — gtqa Fleet QA header |
| One strong visual anchor? | YES — status + remaining column for running soaks |
| Scannable by headlines only? | YES after column reorder |
| Each section one job? | YES — timeline=schedule, outcomes=results, table=per-server |
| Cards necessary? | YES — existing card pattern retained |
| Motion improves hierarchy? | N/A — static; no decorative motion |
| Premium without shadows? | YES — flat ops UI |

### Hard rejections triggered

None.

### Approved mockups

None (DESIGN_NOT_AVAILABLE). ASCII wireframes in plan serve as visual reference.

### Review readiness

Design plan review: **clean** (9/10, 0 unresolved). Run `/design-review` after implementation for visual QA.

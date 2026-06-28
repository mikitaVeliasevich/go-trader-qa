# Design Audit — gtqa Fleet QA Web UI

**URL:** http://127.0.0.1:8080  
**Date:** 2026-06-28  
**Classifier:** APP UI (ops dashboard)  
**Variant:** C Light Split (DM Sans, blue sidebar, card layout)

## Headline Scores

| Metric | Baseline | After fixes |
|--------|----------|-------------|
| **Design Score** | B | B+ |
| **AI Slop Score** | A- | A- |

**Verdict:** Solid internal ops UI. Intentional Atlassian-adjacent palette, not generic purple SaaS slop. Main gaps were keyboard/a11y and a few touch-target sizes — fixed in this pass.

## First Impression

The site communicates **competent internal tooling for fleet soak QA**.

I notice the blue app bar and left nav immediately — this reads as a focused admin console, not a marketing page. The subaccounts table dominates the fleet view, which matches the primary task.

The first 3 things my eye goes to are: **gtqa header**, **Sync fleet button**, **Subaccounts table**.

If I had to describe this in one word: **utilitarian**.

### Page Area Test (Fleet view)

| Area | Purpose | Clear? |
|------|---------|--------|
| App bar | Product ID + sync status + sync action | Yes |
| Sidebar Views | Navigate fleet / batch / report | Yes |
| Sidebar Recent | Jump to past batches | Yes |
| Main card | Select subs, configure soak, start | Yes |

**Trunk test:** PASS (5/6 — no global search, acceptable for internal tool)

## Inferred Design System

- **Fonts:** DM Sans (primary), system fallback
- **Colors:** `#0747a6` sidebar, `#0052cc` accent, `#172b4d` text, semantic pass/fail greens/reds
- **Spacing:** 4/8px rhythm, 40px table rows, 44px touch target token defined
- **Components:** Cards, tags, chips, data tables — cards earn their role (table container)

## Category Grades (baseline)

| Category | Grade | Notes |
|----------|-------|-------|
| Visual Hierarchy | A- | Clear scan path; batch empty state was confusing |
| Typography | B+ | DM Sans, 16px body, utility headings |
| Spacing & Layout | B+ | Consistent card/toolbar rhythm |
| Color & Contrast | A- | Semantic tags, good contrast |
| Interaction States | C+ | Missing focus-visible; hover only |
| Responsive | B | Not tested on mobile this pass; layout is sidebar+main |
| Content Quality | B+ | Utility copy, no happy talk |
| AI Slop | A- | Not template SaaS; intentional ops aesthetic |
| Motion | B | Progress bar transition only |
| Performance Feel | A | Instant local load |

## Findings

### FINDING-001 — No focus-visible styles (HIGH, a11y)

**Category:** Interaction States  
**Fix:** Add `:focus-visible` rings on buttons, links, inputs, chips.  
**Status:** Fixed — `web/static/app.css`

### FINDING-002 — Checkbox names expose as "on" (HIGH, a11y)

**Category:** Interaction States  
**Fix:** `aria-label` per fleet row checkbox with username + server_id.  
**Status:** Fixed — `web/static/app.js` `renderFleet()`

### FINDING-003 — Hidden views leak into a11y tree (MEDIUM, a11y)

**Category:** Interaction States  
**Fix:** `aria-hidden` + `inert` on inactive `.view` panels in `setView()`.  
**Status:** Fixed — `app.js`, `index.html`

### FINDING-004 — Active batch nav shows empty when batch is running (MEDIUM, UX)

**Category:** Visual Hierarchy / Journey  
**Fix:** Auto-select running (or most recent) batch when opening Active batch.  
**Status:** Fixed — `pickDefaultBatch()` in `app.js`

### FINDING-005 — Cancel batch button below 44px (MEDIUM, touch)

**Category:** Responsive / Touch  
**Fix:** `min-height: var(--touch-min)` on `.btn-danger`.  
**Status:** Fixed — `app.css`

### FINDING-006 — Summary link touch target too small (MEDIUM, touch)

**Category:** Responsive / Touch  
**Fix:** Inline-flex + min-height on `.card-footer a` and `.btn-link`.  
**Status:** Fixed — `app.css`

### FINDING-007 — Cryptic recent batch labels "P0 F0" (POLISH, content)

**Category:** Content Quality  
**Fix:** Show `N pass · M fail` instead of `PN FN`.  
**Status:** Fixed — `loadRecentBatches()`

### FINDING-008 — Ineligible reason only in tooltip (POLISH, content)

**Category:** Content Quality  
**Fix:** Truncated `.reason-hint` under eligible=no tag.  
**Status:** Fixed — `renderFleet()` + CSS

## Quick Wins (applied)

1. Focus-visible rings — keyboard users can see where they are
2. Checkbox aria-labels — screen readers name each row
3. Auto-select batch on Active batch nav — removes dead-end empty state
4. Touch target bumps on Cancel / links

## Deferred

| Item | Rationale |
|------|-----------|
| Mobile sidebar collapse | Internal desktop-first tool; no mobile spec in phase-2 |
| `prefers-reduced-motion` | No motion beyond progress bar; low risk |
| Visited link distinction | Few in-app links; external artifact links only |
| DESIGN.md file | Offer to generate from inferred system on request |

## Goodwill Reservoir (fleet → batch → report flow)

```
Goodwill: 70 ████████████████████░░░░░░░░░░
  Step 1: Fleet sync/select     70 → 80  (+10 obvious primary task)
  Step 2: Active batch (before)  80 → 65  (-15 empty state despite running batch in sidebar)
  Step 2: Active batch (after)   65 → 75  (+10 auto-select fix)
  Step 3: Report view            75 → 80  (+5 PASS banner, clear markdown)
  FINAL: 80/100 — healthy
```

## Litmus Checks (APP UI)

| Check | Result |
|-------|--------|
| Brand unmistakable in first screen? | YES — gtqa + Fleet QA |
| One strong visual anchor? | YES — blue app bar |
| Scannable by headlines only? | YES |
| Each section has one job? | YES |
| Cards necessary? | YES — table container |
| Motion improves hierarchy? | N/A — minimal motion |
| Premium without decorative shadows? | YES |

## PR Summary

> Design review found 8 issues, fixed 8. Design score B → B+. AI slop score A- (unchanged). Main wins: keyboard focus rings, screen reader labels, auto-select active batch.

## Files Changed (fix pass)

- `web/static/app.css`
- `web/static/app.js`
- `web/index.html`

# Phase 1: Manager Client + Fleet Table + Single Remote Soak

**Status:** APPROVED (CLI-first, 2026-06-26)  
**Repo:** go-trader-qa  
**Reference:** `../go-trader/.gstack/qa-reports/manager-fleet-api-spec.md`  
**Smoke baseline:** `./scripts/smoke-manager.sh 11` (all endpoints HTTP 200, 2026-06-26)

---

## Goal

Prove the **Manager-only** path end-to-end on one host (`server_id=11`, subaccount 7):

1. Sync fleet inventory from `GET /exchange/accounts/{id}/subs`
2. Print eligibility + pre-flight status
3. Run a **30-minute** remote soak that writes `metrics.tsv` + `soak.log` under `reports/`

No Web UI, no Telegram, no multi-select batches, no G1–G7 analyzer yet.

---

## Non-goals (defer to Phase 2+)

| Item | Phase |
|------|-------|
| Web UI fleet table | 2 |
| Multi-server parallel soaks | 2 |
| `soak-analyze.sh` port (G1–G7) | 2 |
| Telegram notifications | 3 |
| Bot start/stop via Manager | 2+ (observe-only soak in P1) |
| Long-lived service token | 2 |
| `/subs` pagination | 2 (when account > ~100 subs) |

---

## Premises (locked)

1. **Manager-only** — QA never calls bot private IPs; all I/O via Bearer JWT.
2. **Required headers** — staging nginx returns 403 without `Referer: https://staging.yatrade.org/` (verified). Go client must send referer + user-agent on every request.
3. **All Manager proxy routes live** — `/status`, `/config`, `/logs`, `/debug/vars` verified on server 11.
4. **Observe-only soak in P1** — do not start/stop bots; pre-flight warns if `running: false`.
5. **Artifact layout matches local soak** — same `metrics.tsv` columns and `soak.log` name so Phase 2 analyzer is a straight port.

---

## Deliverables

| # | Deliverable | Acceptance |
|---|-------------|------------|
| D1 | `go.mod` + module `github.com/dlisovsky/go-trader-qa` | `go build ./...` passes |
| D2 | `internal/config` — load `.env` + env vars | Missing token → clear error |
| D3 | `internal/manager.Client` — Bearer HTTP with referer | httptest covers headers |
| D4 | `GET /subs` → `[]FleetRow` with eligibility | CLI prints table; sub 7 eligible |
| D5 | Provision methods: Status, Config, Logs, DebugVars | `gtqa smoke 11` replaces bash script |
| D6 | `gtqa soak run --server-id 11 --duration 30m` | `reports/{timestamp}-{server_id}/metrics.tsv` ≥ 6 rows |
| D7 | Pre-flight checks before soak | Fails fast if debug/vars not 200 |

---

## Package layout

```
go-trader-qa/
├── cmd/gtqa/
│   └── main.go              # cobra root
├── internal/
│   ├── config/
│   │   └── config.go        # MANAGER_* env
│   ├── manager/
│   │   ├── client.go        # HTTP transport, auth headers
│   │   ├── subs.go          # GET /exchange/accounts/{id}/subs
│   │   ├── provision.go     # /provision/servers/{id}/*
│   │   └── types.go         # JSON structs
│   ├── fleet/
│   │   └── row.go           # join db_subs + sync_data, pair_id
│   ├── metrics/
│   │   └── row.go           # TSV header, RowFromVars, log-grep helpers
│   └── sampler/
│       └── remote.go        # 30m loop: debug/vars + logs tail
├── reports/                 # gitignored run artifacts
└── docs/phase-1.md          # this file
```

**CLI commands (Phase 1 only):**

```
gtqa fleet sync              # fetch /subs, print table to stdout
gtqa fleet sync --json       # machine-readable rows
gtqa smoke <server_id>       # status, config, logs, debug/vars
gtqa soak run --server-id N --duration 30m [--interval 5m]
```

---

## Data model

### `FleetRow` (normalized)

```go
type FleetRow struct {
    PairID            string   // "{uid}@{server_id}"
    DBSubaccountID    int
    UID               int64
    Username          string
    ServerID          int
    DeployedImageHash string
    QAEligible        bool
    IneligibleReason  string   // "", "no_server", "app_inactive", "app_not_deployed"
    Categories        []string // names only for P1 display
}
```

### Eligibility (from API spec)

```
qa_eligible = has_server && server_id != 0 && app_is_active == true
```

### Pre-flight (before soak, stricter)

| Check | Source | Pass |
|-------|--------|------|
| Fleet eligible | `/subs` | `qa_eligible` |
| Bot reachable | `/status` | HTTP 200 |
| Metrics reachable | `/debug/vars` | HTTP 200, JSON has `bus_drops` key |
| API keys | `/status` | `hasApiKeys == true` (warn if false) |
| Running | `/status` | warn if `running == false`; do not block P1 observe soak |

---

## Manager client

### Config env vars

```bash
MANAGER_API_BASE_URL=https://staging.yatrade.org/api
MANAGER_ACCOUNT_ID=1
MANAGER_BEARER_TOKEN=<jwt>
QA_ARTIFACTS_DIR=./reports   # optional, default ./reports
```

### Request headers (every call)

```http
Authorization: Bearer <token>
Accept: */*
Content-Type: application/json
Referer: https://staging.yatrade.org/
User-Agent: go-trader-qa/1.0
```

### Endpoints

| Method | Path | Response notes |
|--------|------|----------------|
| GET | `/exchange/accounts/{accountID}/subs` | Join `db_subs` + `sync_data` |
| GET | `/provision/servers/{id}/status` | Bot status JSON |
| GET | `/provision/servers/{id}/config` | Bot config JSON |
| GET | `/provision/servers/{id}/logs?tail=500` | `{logs: "...", success: true}` — extract `logs` string |
| GET | `/provision/servers/{id}/debug/vars` | Raw expvar JSON map |

### Error handling

| HTTP | Meaning | Action |
|------|---------|--------|
| 403 | nginx/WAF | Check referer header |
| 404 + `Bot returned error` | Bot route missing | Fail pre-flight, suggest image redeploy |
| 502/504 | Bot unreachable | Retry once, then fail |

Timeout: **10s** per request (match Manager proxy spec).

---

## Remote sampler (port of soak-monitor.sh)

### Run directory

```
reports/{timestamp}-{server_id}/
├── run.env          # metadata (server_id, pair_id, duration, started)
├── metrics.tsv      # same header as local soak
├── soak.log         # accumulated log text
└── issues.log       # sampler warnings
```

### Loop (every `interval`, default 5m)

1. `GET .../debug/vars` → append TSV row (same 26 columns as `soak-monitor.sh` lines 111–176)
2. `GET .../logs?tail=500` → append new lines to `soak.log` (dedupe: drop lines already present at end of buffer; if entire tail block repeats, skip append)
3. Log grep counts from accumulated `soak.log` (same regexes as local monitor lines 165–169):
   - `connection_lost`, `reconnected`, `reconnect_failed`, `errors_10003`, `order_failures`
4. If `bus_drops > 0` on sample, append snippet to `issues.log` (mirror lines 178–184)

### Context / cancellation

`gtqa soak run` must honor `SIGINT`/`SIGTERM`: write `monitor_shutdown` to `issues.log`, flush `metrics.tsv`, exit 0 if loop completed ≥1 sample else 1.

### Duration

Default **30m** for P1 smoke soak (6 samples @ 5m). Configurable via `--duration 30m`.

### Shutdown

Phase 1: **no SIGINT to bot** (`--no-shutdown` equivalent). Observe-only.

---

## Implementation milestones

### M0 — Scaffold (½ day)

- [ ] `go mod init github.com/dlisovsky/go-trader-qa`, cobra CLI skeleton
- [ ] `internal/config` from env + optional `.env` via `github.com/joho/godotenv` (same as go-trader)
- [ ] `go build -o bin/gtqa ./cmd/gtqa`

### M1 — Manager client + fleet sync (1 day)

- [ ] `manager.Client` with referer headers
- [ ] Parse `/subs` JSON into `[]FleetRow`
- [ ] `gtqa fleet sync` — table output (eligible highlighted)
- [ ] Unit tests with `httptest` for subs parsing + eligibility

### M2 — Provision client + smoke (½ day)

- [ ] Status, Config, Logs, DebugVars methods
- [ ] `gtqa smoke <id>` — parity with `scripts/smoke-manager.sh`
- [ ] Deprecate bash script in README (keep as fallback)

### M3 — Remote sampler (1 day)

- [ ] `internal/metrics` — TSV header + `RowFromVars()` + log-grep column helpers (single source; mirror `soak-monitor.sh`)
- [ ] `sampler.RemoteRun` — full 26-column parity, 30m loop on one `server_id`
- [ ] `gtqa soak run --server-id 11 --duration 30m` with context cancel on signal
- [ ] Pre-flight gate before loop starts
- [ ] Manual test: `diff` header against local soak `metrics.tsv`

### M4 — Polish (½ day)

- [ ] README update with `gtqa` commands
- [ ] `CLAUDE.md` point at this doc for implementation
- [ ] Fix stale todos in go-trader fleet plan (`debug/vars` → done)

---

## Test plan

| Test | Type |
|------|------|
| Subs JSON → FleetRow join | unit (`testdata/subs_staging.json`) |
| Eligibility matrix | table-driven unit |
| Client sends Referer header | httptest |
| Logs response unwrap | unit |
| `metrics.RowFromVars` + TSV row format | unit (golden file) |
| Log grep helpers (10003, reconnect) | table-driven unit on fixture `soak.log` |
| Duration parse `30m`, `4h` | unit |
| Log tail dedupe | unit |
| `gtqa fleet sync` against staging | manual (requires `.env`) |
| `gtqa soak run` 30m on server 11 | manual integration |

**Fixture:** capture redacted `/subs` response once, commit as `internal/manager/testdata/subs_staging.json`.

---

## Risks

| Risk | Mitigation |
|------|------------|
| JWT expires mid-soak | Document refresh; P2 service token |
| Log tail overlap duplicates | Dedupe consecutive identical tail blocks |
| Bot not running → flat metrics | Pre-flight warn; document in report |
| Fleet plan in go-trader still says `:6060` | Update sibling doc when P1 ships |

---

## Success criteria (Phase 1 done)

- [ ] `gtqa fleet sync` lists all subs; sub 7 shows eligible with `server_id=11`
- [ ] `gtqa smoke 11` exits 0 with all four endpoints 200
- [ ] `gtqa soak run --server-id 11 --duration 30m` produces valid `metrics.tsv` (≥6 rows, correct header)
- [ ] No code in go-trader repo changes required for P1
- [ ] Bash smoke script optional / documented as legacy

---

## After Phase 1

Phase 2 adds `internal/analyze` (port `soak-analyze.sh`), multi-server runs, and a minimal web UI or TUI for fleet selection.

---

## GSTACK REVIEW REPORT

**Reviewer:** /plan-eng-review  
**Date:** 2026-06-26  
**Verdict:** APPROVED WITH CHANGES (folded into plan above)

### Step 0 — Scope challenge

| Check | Result |
|-------|--------|
| Existing code to reuse | `scripts/smoke-manager.sh`, `go-trader` `soak-monitor.sh`, `internal/alerts/client.go` |
| Minimum viable scope | M0–M2 ship fleet sync + smoke; M3 required for Phase 1 done |
| File count | 8 files + `internal/metrics` — at threshold, acceptable |
| Innovation tokens | cobra + godotenv — boring defaults **[Layer 1]** |

**Scope accepted as-is** after fixes. User confirmed **full soak-monitor TSV parity** in M3.

### What already exists

| Asset | Reuse |
|-------|-------|
| `go-trader/.gstack/qa-reports/soak-monitor.sh` | TSV header, expvar fields, log grep regexes |
| `go-trader/internal/alerts/client.go` | Bearer + HTTP client pattern |
| `go-trader-qa/scripts/smoke-manager.sh` | Required Referer header |
| `manager-fleet-api-spec.md` | Eligibility + endpoints |

### NOT in scope

- Web UI, Telegram, multi-server concurrency, analyzer, bot start/stop
- `/subs` pagination, long-lived service token
- `soak-finish.sh` / `qa-report.md` (Phase 2)
- CI publish for `bin/gtqa` (local build only in P1)

### Findings folded into plan

| # | Sev | Issue | Resolution |
|---|-----|-------|------------|
| 1 | P1 | D6 path `{slug}` vs `{timestamp}-{server_id}` | Fixed in deliverables |
| 2 | P1 | Sampler scope ambiguous | Full 26-column parity; add `internal/metrics` |
| 3 | P2 | Pre-flight vs API spec (`running` required) | P1 observe-only overrides; warn in `issues.log` |
| 4 | P2 | No SIGINT handling for 30m loop | Added context cancel section |
| 5 | P2 | Module path TBD | Locked `github.com/dlisovsky/go-trader-qa` |
| 6 | P2 | `.env` loader unspecified | `godotenv` per go-trader |
| 7 | P3 | `internal/fleet` thin package | Keep for now |

### Test gaps to close in implementation

- Eligibility matrix, Referer httptest, logs JSON unwrap
- `metrics.RowFromVars` golden row, log grep helpers, tail dedupe
- Duration parse `30m` / `4h`

### Failure modes (critical)

| Path | Risk | Mitigation |
|------|------|------------|
| Missing Referer | nginx 403 | httptest on headers |
| Log tail dupes | wrong grep counts | dedupe unit test |
| JWT mid-soak | sample failures | document refresh; exit 1 |

### Parallelization

Sequential — shared `manager.Client` dependency. No worktree split for P1.

**Unresolved:** none blocking M0.

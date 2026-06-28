# go-trader-qa

Fleet soak QA orchestrator for [go-trader](../go-trader). Manager-only access (Bearer JWT); no direct bot IPs.

## Setup

```bash
cp .env.example .env   # MANAGER_BEARER_TOKEN, MANAGER_ACCOUNT_ID, MANAGER_API_BASE_URL
go build -o bin/gtqa ./cmd/gtqa
go build -o bin/gtqa-server ./cmd/gtqa-server
```

## Quick start (Web UI)

```bash
# Production-style (explicit binary)
go build -o bin/gtqa-server ./cmd/gtqa-server
./bin/gtqa-server                    # http://127.0.0.1:8080

# Dev (no manual build — same idea as go-trader's `go run local/main.go`)
go run ./cmd/gtqa-server

# Dev with auto-restart on Go changes (install air once)
go install github.com/air-verse/air@latest
make dev                             # or: air
```

**When do you need to restart?**

| Change | Rebuild binary? | Restart server? | Browser |
|--------|-----------------|-----------------|---------|
| `web/static/*`, `web/index.html` | No | No | Hard refresh |
| `internal/api/*`, other `.go` | Yes (or use `make dev`) | Yes | — |

Static UI is served from the `web/` folder on disk, not baked into the binary — so CSS/JS/HTML edits show up after refresh only. API route/handler changes need a recompiled server; `go run` or `make dev` avoids typing `go build` yourself.

```bash
open http://127.0.0.1:8080
```

1. **Sync fleet** — loads subaccounts from Manager `/subs`
2. **Eligible only** + **Select all eligible**
3. **Start soak** — default profile `wss-only`, duration picker
4. **Active batch** — poll progress, cancel, view per-job reports
5. **Recent batches** — reopen completed runs from the sidebar

## CLI — fleet & smoke

```bash
./bin/gtqa fleet sync
./bin/gtqa fleet sync --json
./bin/gtqa smoke 11                  # provision proxy: status, config, logs, debug/vars
```

## CLI — single soak (Phase 1)

```bash
./bin/gtqa soak run --server-id 11 --duration 30m --interval 5m
```

Artifacts: `reports/{timestamp}-{server_id}/` (`metrics.tsv`, `soak.log`, `issues.log`, `run.env`).

## CLI — analyze (Phase 2)

```bash
./bin/gtqa analyze reports/2026-06-28T10-51-41Z-11
./bin/gtqa analyze reports/2026-06-28T10-51-41Z-11 --profile wss-only
./bin/gtqa analyze reports/2026-06-28T10-51-41Z-11 --profile lifecycle
```

Writes `qa-report.md` with G1–G7 gate results. Default profile: **wss-only** (observe-only subs). **lifecycle** expects active trading during the soak window.

## CLI — batch soak (Phase 2)

```bash
./bin/gtqa soak batch --server-ids 11,12 --duration 30m --interval 5m --concurrency 2
```

Batch dir: `reports/batch-{timestamp}/` with per-job run dirs, auto-analyze, `batch-summary.md` + `batch-summary.json`.

## HTTP API (`gtqa-server`)

| Endpoint | Purpose |
|----------|---------|
| `GET /api/health` | Liveness |
| `GET /api/fleet` | Cached fleet rows (503 until sync) |
| `POST /api/fleet/sync` | Refresh from Manager |
| `POST /api/batches` | Start batch (`server_ids`, `duration`, `profile`, …) |
| `GET /api/batches` | Recent batches (default limit 20) |
| `GET /api/batches/{id}` | Batch status + jobs |
| `POST /api/batches/{id}/cancel` | Cancel running batch |
| `GET /api/batches/{id}/jobs/{server_id}/report` | `qa-report.md` |
| `GET /api/batches/{id}/jobs/{server_id}/artifacts/{name}` | Whitelisted artifacts |

Static Web UI: `GET /` serves `web/`.

**Env (optional):**

| Variable | Default |
|----------|---------|
| `GTQA_LISTEN_ADDR` | `127.0.0.1:8080` |
| `GTQA_MAX_CONCURRENCY` | `2` |
| `GTQA_MAX_CONCURRENCY_HARD` | `3` |
| `QA_ARTIFACTS_DIR` | `./reports` |

Bind stays localhost by default — no auth on the control plane; do not expose publicly.

## Reference docs (sibling repo)

| Topic | Path in `go-trader` |
|-------|---------------------|
| Manager API spec | `.gstack/qa-reports/manager-fleet-api-spec.md` |
| Fleet QA design | `.gstack/qa-reports/fleet_qa_automation_design_0691c1b0.md` |
| Metrics catalog | `.gstack/qa-reports/metrics-catalog.json` |
| Gates G1–G7 | `.gstack/qa-reports/README.md` |

## Plans

- [docs/phase-1.md](docs/phase-1.md) — fleet sync, smoke, single remote soak (DONE)
- [docs/phase-2.md](docs/phase-2.md) — analyzer, batch, API, Web UI (DONE; run `/design-review` on live UI)

## Package layout

```
cmd/gtqa/              # CLI: fleet, smoke, soak, analyze
cmd/gtqa-server/       # HTTP API + static web/
internal/analyze/      # G1–G7 gates, qa-report.md
internal/api/          # REST handlers, fleet cache, batch registry
internal/batch/        # parallel soak runner
internal/catalog/      # metrics-catalog.json loader
internal/config/       # MANAGER_* + GTQA_* env
internal/fleet/        # eligibility join
internal/manager/      # /subs, /provision/servers/{id}/*
internal/metrics/      # TSV contract, ReadTSV, deltas
internal/sampler/      # remote observe loop
web/                   # index.html + static/ (no build step)
reports/               # gitignored run artifacts
```

**Legacy:** `./scripts/smoke-manager.sh 11` — same checks as `gtqa smoke`.

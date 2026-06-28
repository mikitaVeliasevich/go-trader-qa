# go-trader-qa

Fleet soak orchestrator. **go-trader** sibling (`../go-trader`) has bot code, soak scripts, and design docs.

## Read before coding

1. `../go-trader/.gstack/qa-reports/manager-fleet-api-spec.md` — Manager endpoints
2. `../go-trader/.gstack/qa-reports/fleet_qa_automation_design_0691c1b0.md` — architecture
3. `docs/phase-2.md` — current scope, API contract, Web UI spec
4. Port logic from `../go-trader/.gstack/qa-reports/soak-monitor.sh` and `soak-analyze.sh` (now `internal/sampler` + `internal/analyze`)

## Rules

- **Manager-only** — `MANAGER_BEARER_TOKEN` on server; QA never calls bot private IPs
- **Observe-only** — do not start/stop bots from QA; lifecycle profile may FAIL on stopped bots
- Default analyzer profile: **wss-only**; lifecycle is explicit opt-in with UI confirm
- Local single-machine soak stays in go-trader (`soak-run.sh`)
- Env: `.env.example`; never commit `.env`
- `gtqa-server` binds `127.0.0.1` by default — no auth layer

## Binaries

```bash
go build -o bin/gtqa ./cmd/gtqa
go build -o bin/gtqa-server ./cmd/gtqa-server
```

## Common workflows

```bash
# Staging smoke
./bin/gtqa fleet sync && ./bin/gtqa smoke 11

# Analyze fixture (golden: wss-only + lifecycle-strict PASS on 10-51-41Z-11)
./bin/gtqa analyze --run-dir reports/2026-06-28T10-51-41Z-11 --profile wss-only
./bin/gtqa analyze --run-dir reports/2026-06-28T10-51-41Z-11 --profile lifecycle-strict

# Short batch
./bin/gtqa soak batch --server-ids 11 --duration 15s --interval 5s --concurrency 1

# Control plane + Web UI
go run ./cmd/gtqa-server             # dev: compile + run (no bin/gtqa-server step)
make dev                             # dev: auto-restart on .go changes (air)
./bin/gtqa-server                    # after go build, or CI/deploy
```

## Package map

| Package | Role |
|---------|------|
| `internal/manager` | HTTP client for `/subs` and provision proxy |
| `internal/fleet` | Join db_subs + sync_data → `FleetRow`, eligibility |
| `internal/sampler` | Remote observe loop → `metrics.tsv`, `soak.log` |
| `internal/metrics` | TSV I/O, deltas, log helpers |
| `internal/analyze` | G1–G16 gates, `qa-report.md`, profiles |
| `internal/catalog` | Embedded `metrics-catalog.json` (gate/profile schema) |
| `internal/batch` | Multi-server soak, semaphore, batch-summary |
| `internal/api` | `gtqa-server` routes, artifact whitelist |
| `web/` | Static UI (no React, no build step) |

## Tests

```bash
go test ./...
```

Golden fixture: `reports/2026-06-28T10-51-41Z-11/` (wss-only, lifecycle, lifecycle-strict PASS when no position activity).

Profiles: `wss-only` (G1,G2,G11), `lifecycle` (G1–G7), `lifecycle-strict` (G1–G16), `tpsl-health` (G10,G12–G14).

## Phase status

- Phase 1: DONE — [docs/phase-1.md](docs/phase-1.md)
- Phase 2: DONE (code) — [docs/phase-2.md](docs/phase-2.md); visual polish via `/design-review` on live UI

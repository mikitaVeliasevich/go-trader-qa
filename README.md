# go-trader-qa

Fleet soak QA orchestrator for [go-trader](../go-trader). Manager-only access (Bearer JWT); no direct bot IPs.

## Setup

```bash
cp .env.example .env   # set MANAGER_BEARER_TOKEN, MANAGER_ACCOUNT_ID, MANAGER_API_BASE_URL
go build -o bin/gtqa ./cmd/gtqa
```

## Commands (Phase 1)

```bash
# Fleet inventory from Manager /subs
./bin/gtqa fleet sync
./bin/gtqa fleet sync --json

# Smoke-test provision proxy (status, config, logs, debug/vars)
./bin/gtqa smoke 11

# Observe-only remote soak (default 30m, 5m interval)
./bin/gtqa soak run --server-id 11 --duration 30m
./bin/gtqa soak run --server-id 11 --duration 30m --interval 5m
```

Artifacts land in `reports/{timestamp}-{server_id}/` (`metrics.tsv`, `soak.log`, `issues.log`, `run.env`).

**Legacy:** `./scripts/smoke-manager.sh 11` — same checks as `gtqa smoke`; kept as fallback.

## Reference docs (sibling repo)

| Topic | Path in `go-trader` |
|-------|---------------------|
| Manager API spec | `.gstack/qa-reports/manager-fleet-api-spec.md` |
| Fleet QA design | `.gstack/qa-reports/fleet_qa_automation_design_0691c1b0.md` |
| Local soak scripts (ported from) | `.gstack/qa-reports/soak-monitor.sh`, `soak-analyze.sh` |
| Gates G1–G7 | `.gstack/qa-reports/README.md` |

## Phase 1 plan

[docs/phase-1.md](docs/phase-1.md) — fleet sync, smoke, 30m remote soak on one `server_id`.

## Package layout

```
cmd/gtqa/           # CLI (fleet, smoke, soak)
internal/config/    # MANAGER_* env
internal/manager/   # /subs, /provision/servers/{id}/*
internal/fleet/     # eligibility join
internal/metrics/   # TSV contract (soak-monitor parity)
internal/sampler/   # remote observe loop
reports/            # gitignored run artifacts
```

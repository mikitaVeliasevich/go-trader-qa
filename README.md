# go-trader-qa

Fleet soak QA orchestrator for [go-trader](../go-trader). Manager-only access (Bearer JWT); no direct bot IPs.

## Setup

```bash
cp .env.example .env   # set MANAGER_BEARER_TOKEN
./scripts/smoke-manager.sh 11
```

## Reference docs (sibling repo)

| Topic | Path in `go-trader` |
|-------|---------------------|
| Manager API spec | `.gstack/qa-reports/manager-fleet-api-spec.md` |
| Fleet QA design | `.gstack/qa-reports/fleet_qa_automation_design_0691c1b0.plan.md` |
| Local soak scripts (port from) | `.gstack/qa-reports/soak-*.sh` |
| Gates G1–G7 | `.gstack/qa-reports/soak-analyze.sh`, `README.md` |

## Target layout

```
cmd/qa-orchestrator/
internal/manager/   # /subs, /provision/servers/{id}/*
internal/sampler/   # port soak-monitor.sh
internal/analyze/   # port soak-analyze.sh
internal/report/
```

## Phase 1

Manager client + `/subs` fleet table + remote sampling on `server_id=11`.

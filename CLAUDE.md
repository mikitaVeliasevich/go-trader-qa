# go-trader-qa

Fleet soak orchestrator. **go-trader** sibling (`../go-trader`) has bot code, soak scripts, and design docs.

## Read before coding

1. `../go-trader/.gstack/qa-reports/manager-fleet-api-spec.md` — Manager endpoints
2. `../go-trader/.gstack/qa-reports/fleet_qa_automation_design_0691c1b0.plan.md` — architecture
3. Port logic from `../go-trader/.gstack/qa-reports/soak-monitor.sh` and `soak-analyze.sh`

## Rules

- Manager-only HTTP (`MANAGER_BEARER_TOKEN`); never call bot private IPs
- Local single-machine soak stays in go-trader (`soak-run.sh`)
- Env: `.env.example`; never commit `.env`

## Smoke test

`./scripts/smoke-manager.sh [server_id]`

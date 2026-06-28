# Session context (handoff from go-trader fleet QA design)

**Created:** 2026-06-26  
**Source:** Cursor design session on `go-trader` (`feature/wss-reliability` area)  
**This repo:** `go-trader-qa` — fleet soak orchestrator (separate from the trading bot)

## What we are building

Scale soak QA from **local-only** scripts in [go-trader `.gstack/qa-reports/`](../../go-trader/.gstack/qa-reports/) to **fleet-wide automation** for up to ~500 Bybit subaccounts (each: AWS server + Docker bot), via yatrade **Manager** APIs.

| Layer | Repo / system | Role |
|-------|----------------|------|
| Inventory | Manager `GET /exchange/accounts/{id}/subs` | Fleet list, eligibility |
| Orchestration | **go-trader-qa** (this repo) | Batches, sampling, gates, reports, UI, Telegram |
| Bot metrics | [go-trader](https://github.com/dlisovsky/go-trader) | Trading bot + `GET :3228/debug/vars` |
| Proxy | Manager | Bearer JWT → bot `:3228` (status, config, logs, debug/vars) |

## Decisions already made

1. **Separate app** — orchestrator is not `cmd/qa-orchestrator` inside go-trader.
2. **Manager-only access** — QA never calls bot private IPs; all bot I/O via Manager proxy + JWT.
3. **Telegram** — batch summaries / FAIL alerts only; **Web UI** for fleet control at scale.
4. **Metrics on :3228** — `GET /debug/vars` on main bot router; **`:6060` / `PPROF_ENABLED` removed** from go-trader (done).
5. **Logs proxy** — `GET .../logs?tail=N` for log-grep gates (G2, reconnect); incremental `tail=500` during soak.
6. **Log-grep only is not enough** — G1, G3–G7 need expvar via `GET .../debug/vars` (Manager proxy **live** on staging; bot image must include route).

## Manager API status

| Endpoint | Status |
|----------|--------|
| `GET /api/exchange/accounts/{id}/subs` | Done |
| `GET /api/provision/servers/{id}/status` | Done |
| `GET /api/provision/servers/{id}/config` | Done |
| `GET /api/provision/servers/{id}/logs?tail=N` | Done |
| `GET /api/provision/servers/{id}/debug/vars` | **Done** (proxy live 2026-06-26); bot must run go-trader with `GET /debug/vars` on `:3228` |

Staging base: `https://staging.yatrade.org/api/`  
Auth v1: `MANAGER_BEARER_TOKEN` + `MANAGER_ACCOUNT_ID=1` (env, never commit).

## go-trader work already done

- [`internal/router/router.go`](../../go-trader/internal/router/router.go) — `GET /debug/vars` (`expvar.Handler`)
- Removed `PPROF_ENABLED` / `:6060` from `cmd/bot/main.go` and `local/main.go`
- Soak scripts use `http://127.0.0.1:3228/debug/vars`

## Reference implementation (port from go-trader)

Local soak logic to port into Go:

| Script | Port to |
|--------|---------|
| [soak-monitor.sh](../../go-trader/.gstack/qa-reports/soak-monitor.sh) | `internal/sampler/` — metrics.tsv + log grep |
| [soak-analyze.sh](../../go-trader/.gstack/qa-reports/soak-analyze.sh) | `internal/analyze/` — gates G1–G7 |
| [soak-report.sh](../../go-trader/.gstack/qa-reports/soak-report.sh) | report generator |

Gate definitions: [local-soak-reference.md](./local-soak-reference.md)

## Phased rollout (this repo)

| Phase | Scope |
|-------|--------|
| **0** | Docs + env (this file) |
| **1** | Manager client, `/subs` fleet table, smoke on `server_id=11` (`scripts/smoke-manager.sh`) |
| **2** | Sampler + analyzer + `metrics-catalog.json`; first remote 30m soak |
| **3** | Web UI, multi-select batches, Telegram |
| **4** | Canary presets, scheduling |

## Blockers

- None for Phase 1 smoke on server 11 (verified 2026-06-26 after bot redeploy).
- Bot is **initialized** but **not running** (`running: false`) — start trading before lifecycle soak gates G3–G7 can fire.

## Plans in this repo

- [fleet-qa-automation-design.md](./plans/fleet-qa-automation-design.md) — full architecture (update paths for this repo)
- [manager-debug-vars-proxy.md](./plans/manager-debug-vars-proxy.md) — Manager team handoff
- [manager-fleet-api-spec.md](./manager-fleet-api-spec.md) — API field mapping

## Open product decisions

- **Destructive vs observe-only soak** on live subaccounts — prefer observe-only on production pairs; full SIGINT lifecycle only on dedicated QA subs.
- **Orchestrator hosting** — any host with HTTPS to `staging.yatrade.org`.

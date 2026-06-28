# Local soak reference (go-trader)

Fleet QA ports behavior from go-trader local soak scripts. **Do not duplicate** those scripts here long-term — reimplement in Go, use scripts as spec.

**Source repo:** `../go-trader/.gstack/qa-reports/`

## Profiles

| Profile | Gates | Intent |
|---------|-------|--------|
| `wss-only` | G1, G2 | WSS reliability |
| `lifecycle` (default) | G1–G7 | price_move trading lifecycle |

## Gates

### wss-only

- **G1:** `bus_drops` delta == 0 (from `/debug/vars`)
- **G2:** no `retCode=10003` in accumulated logs

### lifecycle

- **G1:** `bus_drops` delta == 0
- **G2:** no `retCode=10003` in logs
- **G3:** `order_filter_cancel` delta > 0 OR `order_create_ok` bounded (~≤30 resting proxy)
- **G4:** `position_opened` delta ≥ 1 AND `position_reset` delta ≥ 1
- **G5:** `order_create_ok` delta ≥ 85% of `position_reset` delta
- **G6:** `position_reset` delta ≤ `position_opened` delta + 1
- **G7:** pause/resume pairing (`algo_paused` vs `algo_resumed_ok`, with in-flight allowance)

## metrics.tsv columns

**From expvar** (`GET :3228/debug/vars`):

| Column | expvar key |
|--------|------------|
| `ws_messages` | `ws_messages_received` |
| `bus_drops` | `bus_drops` |
| `bus_publishes` | `bus_publishes` |
| `ticker_parsed` | `ticker_messages_parsed` |
| `private_parsed` | `private_messages_parsed` |
| `kline_parsed` | `kline_messages_parsed` |
| `order_create_ok` | `order_create_ok` |
| `order_amend_ok` | `order_amend_ok` |
| `order_cancel_ok` | `order_cancel_ok` |
| `order_filter_cancel` | `order_filter_cancel` |
| `order_create_blocked_position` | `order_create_blocked_position` |
| `position_opened` | `position_opened` |
| `position_reset` | `position_reset` |
| `algo_paused` | `algo_paused` |
| `algo_resumed_ok` | `algo_resumed_ok` |

**From log grep** (fleet: Manager `/logs`):

| Column | Pattern |
|--------|---------|
| `errors_10003` | `retCode=10003` |
| `connection_lost` | connection loss lines |
| `reconnected` | reconnect success |
| `reconnect_failed` | reconnect failure |

## Sampling

- Interval: **5 minutes**
- Fleet logs: `GET .../logs?tail=500`, append new lines to per-job `soak.log`
- Fleet metrics: `GET .../debug/vars` each sample

## Local dev (still in go-trader)

```bash
cd ../go-trader
.gstack/qa-reports/soak-run.sh price-move-lifecycle-8h 4h
```

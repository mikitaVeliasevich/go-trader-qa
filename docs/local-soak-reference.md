# Local soak reference (go-trader)

Fleet QA ports behavior from go-trader local soak scripts. **Do not duplicate** those scripts here long-term — reimplement in Go, use scripts as spec.

**Source repo:** `../go-trader/.gstack/qa-reports/`

## Profiles

| Profile | Gates | Intent |
|---------|-------|--------|
| `wss-only` | G1, G2, G11 | WSS reliability + reconnect health |
| `lifecycle` (default) | G1–G7 | price_move trading lifecycle |
| `lifecycle-strict` | G1–G16 | Full elite QA including failures, TP/SL, wake path |
| `tpsl-health` | G10, G12–G14 | TP/SL and position-reset breakdown |

### lifecycle-strict (G8–G16)

- **G8:** `algo_stopped_permanent` delta == 0
- **G9:** `order_fail_ret_10003` delta == 0
- **G10:** when `position_reset` > 0, SL+TP+cancel+liquidation+manual+other == reset
- **G11:** `reconnect_failed` last == 0 AND `connection_lost` last ≤ `reconnected` last + 1
- **G12:** `tp_missing_create_fail` == 0 AND `tp_missing_close_position` == 0
- **G13:** `sl_setup_fail` == 0 AND `sl_price_past_immediate_close` == 0
- **G14:** when partial TP or lifecycle active: `partial_tp_needs_cancel` ≤ `order_cancel_ok` delta
- **G15:** `order_fail_total` (create+amend+cancel) delta == 0
- **G16:** wake path: `price_exec_runs` > 0 when `price_wake_signals` > 0; `private_order_events_drained` > 0 when `position_opened` > 0

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

**Elite optional columns** (after `algo_resumed_ok`, before log columns; default 0 if absent):

Order failures: `order_fail_create`, `order_fail_amend`, `order_fail_cancel`, `order_fail_ret_*` (10003, 10006, 10001, 10016, 110001, 110007, 110013, 110021, 110090, 110094, 110059, 110123, 110126), `algo_stopped_permanent`, `risk_limit_margin_reduced`.

TP/SL: `tp_missing_create_ok`, `tp_missing_create_fail`, `tp_missing_close_position`, `sl_setup_started`, `sl_setup_ok`, `sl_setup_fail`, `sl_setup_cancelled`, `sl_price_past_immediate_close`, `partial_tp_needs_cancel`, `position_reset_liquidation`, `position_reset_manual`.

Wake path: `price_wake_signals`, `price_exec_runs`, `private_order_wake_signals`, `private_order_events_signaled`, `private_order_drain_batches`, `private_order_events_drained`.

Expvar key equals TSV column name for all elite columns.

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

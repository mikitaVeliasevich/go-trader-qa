package catalog

// GuideMetric describes one metrics.tsv column for the UI reference.
type GuideMetric struct {
	Column      string `json:"column"`
	Expvar      string `json:"expvar,omitempty"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// GuideMetricGroup groups related columns.
type GuideMetricGroup struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Metrics []GuideMetric `json:"metrics"`
}

// GuideGate describes one QA gate (G1–G16).
type GuideGate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Rule        string   `json:"rule"`
	Why         string   `json:"why"`
	Profiles    []string `json:"profiles"`
	ReportHint  string   `json:"report_hint"`
}

// GuideProfile maps a profile name to its gates and intent.
type GuideProfile struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Gates       []string `json:"gates"`
	Description string   `json:"description"`
}

// MetricsGuide is the reference payload for the Metrics guide UI tab.
type MetricsGuide struct {
	Intro struct {
		Title       string `json:"title"`
		GateNaming  string `json:"gate_naming"`
		HowSampling string `json:"how_sampling"`
		HowReports  string `json:"how_reports"`
	} `json:"intro"`
	Profiles []GuideProfile   `json:"profiles"`
	Gates    []GuideGate      `json:"gates"`
	Groups   []GuideMetricGroup `json:"groups"`
}

// BuildGuide returns the static metrics and gates reference for gtqa UI.
func BuildGuide() MetricsGuide {
	c, _ := Load("")
	gateRules := make(map[string]string, len(c.Gates))
	for _, g := range c.Gates {
		gateRules[g.ID] = g.Rule
	}

	guide := MetricsGuide{}
	guide.Intro.Title = "Metrics & gates reference"
	guide.Intro.GateNaming = "G stands for Gate: a pass/fail check in qa-report.md. Each gate compares metric deltas or log patterns over your soak window. G1 is the oldest check; G8–G16 were added for elite QA (order failures, TP/SL recovery, wake path)."
	guide.Intro.HowSampling = "Every 5 minutes gtqa samples GET /debug/vars from each bot (via Manager) and appends a row to metrics.tsv. Deltas are last row minus first row. Log columns come from grep on soak.log (connection events, retCode=10003, order failure lines)."
	guide.Intro.HowReports = "After a soak, the analyzer runs the profile you chose (wss-only, lifecycle, lifecycle-strict, or tpsl-health). Each gate prints PASS or FAIL with a detail line. OVERALL is PASS only when every gate in that profile passes."

	guide.Profiles = []GuideProfile{
		{ID: "wss-only", Label: "wss-only", Gates: c.Profiles["wss-only"], Description: "WebSocket reliability only. Safe default when bots are idle or you only care about bus drops and reconnect health."},
		{ID: "lifecycle", Label: "lifecycle", Gates: c.Profiles["lifecycle"], Description: "Classic price_move lifecycle (G1–G7). Expects some trading activity during the window."},
		{ID: "lifecycle-strict", Label: "lifecycle-strict", Gates: c.Profiles["lifecycle-strict"], Description: "Full elite QA (G1–G16). Requires bots on the latest go-trader build with all expvar counters."},
		{ID: "tpsl-health", Label: "tpsl-health", Gates: c.Profiles["tpsl-health"], Description: "Focused TP/SL and position-reset breakdown checks without full lifecycle trading gates."},
	}

	guide.Gates = []GuideGate{
		{ID: "G1", Title: "No bus drops", Rule: gateRules["G1"], Why: "Messages dropped on the internal pub/sub bus mean price or order events never reached the algo.", Profiles: profilesForGate(c, "G1"), ReportHint: "FAIL when bus_drops delta > 0"},
		{ID: "G2", Title: "No 10003 in logs", Rule: gateRules["G2"], Why: "Bybit retCode 10003 (invalid API key) is a hard auth failure; must never appear during a soak.", Profiles: profilesForGate(c, "G2"), ReportHint: "FAIL when soak.log contains retCode=10003"},
		{ID: "G3", Title: "Filter or bounded resting", Rule: gateRules["G3"], Why: "Confirms the bot is actively managing resting orders (filter cancels) or has a small bounded resting book.", Profiles: profilesForGate(c, "G3"), ReportHint: "PASS if order_filter_cancel increased OR order_create_ok stayed ≤ ~30"},
		{ID: "G4", Title: "Position lifecycle seen", Rule: gateRules["G4"], Why: "Lifecycle profile expects at least one open and one reset during the window to validate end-to-end trading.", Profiles: profilesForGate(c, "G4"), ReportHint: "FAIL on idle bots with no position_opened/position_reset activity"},
		{ID: "G5", Title: "Creates track resets", Rule: gateRules["G5"], Why: "Most position resets should follow order creates; a large gap suggests silent failures.", Profiles: profilesForGate(c, "G5"), ReportHint: "order_create_ok delta should be ≥ 85% of position_reset delta"},
		{ID: "G6", Title: "Resets bounded by opens", Rule: gateRules["G6"], Why: "You cannot reset more positions than you opened (plus one tolerance for edge timing).", Profiles: profilesForGate(c, "G6"), ReportHint: "position_reset delta ≤ position_opened delta + 1"},
		{ID: "G7", Title: "Pause/resume pairing", Rule: gateRules["G7"], Why: "When the algo pauses (e.g. margin/risk), it must resume cleanly; orphaned pauses mean stuck state.", Profiles: profilesForGate(c, "G7"), ReportHint: "Compare algo_paused vs algo_resumed_ok deltas; report shows in_flight allowance"},
		{ID: "G8", Title: "No permanent algo stop", Rule: gateRules["G8"], Why: "retCode 110123 stops the algo permanently with no cooldown resume.", Profiles: profilesForGate(c, "G8"), ReportHint: "algo_stopped_permanent delta must be 0"},
		{ID: "G9", Title: "No order-fail 10003 counter", Rule: gateRules["G9"], Why: "Separate expvar counter for 10003 on order REST failures (complements G2 log grep).", Profiles: profilesForGate(c, "G9"), ReportHint: "order_fail_ret_10003 delta must be 0"},
		{ID: "G10", Title: "Reset reason breakdown", Rule: gateRules["G10"], Why: "When positions reset, SL+TP+cancel+liquidation+manual+other must sum to position_reset.", Profiles: profilesForGate(c, "G10"), ReportHint: "Skipped when position_reset delta is 0"},
		{ID: "G11", Title: "Reconnect health", Rule: gateRules["G11"], Why: "WebSocket disconnects should recover; reconnect_failed must stay zero and lost/reconnected counts stay paired.", Profiles: profilesForGate(c, "G11"), ReportHint: "Uses last-row log counters connection_lost, reconnected, reconnect_failed"},
		{ID: "G12", Title: "TP recovery clean", Rule: gateRules["G12"], Why: "Missing take-profit recovery must not fail or force emergency closes.", Profiles: profilesForGate(c, "G12"), ReportHint: "tp_missing_create_fail and tp_missing_close_position deltas must be 0"},
		{ID: "G13", Title: "SL setup clean", Rule: gateRules["G13"], Why: "Stop-loss placement after entry must succeed; price-past-SL immediate closes indicate bad timing.", Profiles: profilesForGate(c, "G13"), ReportHint: "sl_setup_fail and sl_price_past_immediate_close deltas must be 0"},
		{ID: "G14", Title: "Partial TP cancels", Rule: gateRules["G14"], Why: "When partial TP needs a cancel, the bot must actually cancel orders.", Profiles: profilesForGate(c, "G14"), ReportHint: "partial_tp_needs_cancel ≤ order_cancel_ok when lifecycle active"},
		{ID: "G15", Title: "No order REST failures", Rule: gateRules["G15"], Why: "Any failed create/amend/cancel during the window is a regression signal.", Profiles: profilesForGate(c, "G15"), ReportHint: "order_fail_total = create + amend + cancel failure deltas"},
		{ID: "G16", Title: "Wake path health", Rule: gateRules["G16"], Why: "Price wakes must trigger executeOrder runs; private order events must drain when positions open.", Profiles: profilesForGate(c, "G16"), ReportHint: "Requires new bot counters; fails if fleet not upgraded"},
	}

	guide.Groups = []GuideMetricGroup{
		{
			Title:   "WSS & event bus",
			Summary: "Inbound WebSocket volume and internal pub/sub health.",
			Metrics: []GuideMetric{
				{Column: "ws_messages", Expvar: "ws_messages_received", Source: "expvar", Description: "Total private + public WS frames received."},
				{Column: "ticker_parsed", Expvar: "ticker_messages_parsed", Source: "expvar", Description: "Public ticker messages parsed (price feed)."},
				{Column: "private_parsed", Expvar: "private_messages_parsed", Source: "expvar", Description: "Private WS messages parsed (orders, positions)."},
				{Column: "kline_parsed", Expvar: "kline_messages_parsed", Source: "expvar", Description: "Kline/candle messages parsed."},
				{Column: "bus_publishes", Expvar: "bus_publishes", Source: "expvar", Description: "Events published on the internal bus."},
				{Column: "bus_drops", Expvar: "bus_drops", Source: "expvar", Description: "Events dropped because subscribers were slow (G1)."},
			},
		},
		{
			Title:   "Order lifecycle",
			Summary: "Successful order operations and filter activity.",
			Metrics: []GuideMetric{
				{Column: "order_create_ok", Expvar: "order_create_ok", Source: "expvar", Description: "REST order creates that succeeded."},
				{Column: "order_amend_ok", Expvar: "order_amend_ok", Source: "expvar", Description: "REST amends that succeeded."},
				{Column: "order_cancel_ok", Expvar: "order_cancel_ok", Source: "expvar", Description: "REST cancels that succeeded."},
				{Column: "order_filter_cancel", Expvar: "order_filter_cancel", Source: "expvar", Description: "Resting orders cancelled by the filter (G3 signal)."},
				{Column: "order_create_blocked_position", Expvar: "order_create_blocked_position", Source: "expvar", Description: "Create skipped because a position already exists."},
			},
		},
		{
			Title:   "Position lifecycle",
			Summary: "Opens, resets, and reason splits.",
			Metrics: []GuideMetric{
				{Column: "position_opened", Expvar: "position_opened", Source: "expvar", Description: "New positions opened by price_move."},
				{Column: "position_reset", Expvar: "position_reset", Source: "expvar", Description: "Positions fully closed/reset."},
				{Column: "position_reset_sl", Expvar: "position_reset_sl", Source: "expvar", Description: "Reset triggered by stop-loss hit."},
				{Column: "position_reset_tp", Expvar: "position_reset_tp", Source: "expvar", Description: "Reset triggered by take-profit hit."},
				{Column: "position_reset_cancel", Expvar: "position_reset_cancel", Source: "expvar", Description: "Reset after resting-order cancel path (not SL/TP)."},
				{Column: "position_reset_liquidation", Expvar: "position_reset_liquidation", Source: "expvar", Description: "Reset due to liquidation."},
				{Column: "position_reset_manual", Expvar: "position_reset_manual", Source: "expvar", Description: "Reset due to manual close."},
				{Column: "position_reset_other", Expvar: "position_reset_other", Source: "expvar", Description: "Reset with unknown/other reason (G10)."},
			},
		},
		{
			Title:   "Algo pause/resume",
			Summary: "Temporary halts vs successful resumes.",
			Metrics: []GuideMetric{
				{Column: "algo_paused", Expvar: "algo_paused", Source: "expvar", Description: "Algo entered a paused state (risk/margin/cooldown)."},
				{Column: "algo_resumed_ok", Expvar: "algo_resumed_ok", Source: "expvar", Description: "Algo resumed after pause (G7 pairing)."},
				{Column: "algo_stopped_permanent", Expvar: "algo_stopped_permanent", Source: "expvar", Description: "Permanent stop on retCode 110123 (G8)."},
			},
		},
		{
			Title:   "Order failures (Tier 1)",
			Summary: "REST failures by action and Bybit retCode.",
			Metrics: []GuideMetric{
				{Column: "order_fail_create", Expvar: "order_fail_create", Source: "expvar", Description: "Failed order creates."},
				{Column: "order_fail_amend", Expvar: "order_fail_amend", Source: "expvar", Description: "Failed order amends."},
				{Column: "order_fail_cancel", Expvar: "order_fail_cancel", Source: "expvar", Description: "Failed order cancels."},
				{Column: "order_fail_ret_10003", Expvar: "order_fail_ret_10003", Source: "expvar", Description: "Failures with invalid API key (G9)."},
				{Column: "order_fail_ret_10006", Expvar: "order_fail_ret_10006", Source: "expvar", Description: "Rate limit / too many visits."},
				{Column: "order_fail_ret_10001", Expvar: "order_fail_ret_10001", Source: "expvar", Description: "Parameter error."},
				{Column: "order_fail_ret_10016", Expvar: "order_fail_ret_10016", Source: "expvar", Description: "Server error / maintenance."},
				{Column: "order_fail_ret_110001", Expvar: "order_fail_ret_110001", Source: "expvar", Description: "Order does not exist."},
				{Column: "order_fail_ret_110007", Expvar: "order_fail_ret_110007", Source: "expvar", Description: "Insufficient available balance."},
				{Column: "order_fail_ret_110013", Expvar: "order_fail_ret_110013", Source: "expvar", Description: "Cannot set leverage."},
				{Column: "order_fail_ret_110021", Expvar: "order_fail_ret_110021", Source: "expvar", Description: "Order quantity invalid."},
				{Column: "order_fail_ret_110090", Expvar: "order_fail_ret_110090", Source: "expvar", Description: "Risk limit / margin reduced path."},
				{Column: "order_fail_ret_110094", Expvar: "order_fail_ret_110094", Source: "expvar", Description: "Order would trigger liquidation."},
				{Column: "order_fail_ret_110059", Expvar: "order_fail_ret_110059", Source: "expvar", Description: "No new positions allowed."},
				{Column: "order_fail_ret_110123", Expvar: "order_fail_ret_110123", Source: "expvar", Description: "Permanent algo stop trigger."},
				{Column: "order_fail_ret_110126", Expvar: "order_fail_ret_110126", Source: "expvar", Description: "Cross/isolated margin mode conflict."},
				{Column: "risk_limit_margin_reduced", Expvar: "risk_limit_margin_reduced", Source: "expvar", Description: "110090 handler reduced margin/risk limit."},
			},
		},
		{
			Title:   "TP/SL recovery (Tier 2)",
			Summary: "Take-profit and stop-loss setup after entry.",
			Metrics: []GuideMetric{
				{Column: "tp_missing_create_ok", Expvar: "tp_missing_create_ok", Source: "expvar", Description: "Successfully recreated missing TP order."},
				{Column: "tp_missing_create_fail", Expvar: "tp_missing_create_fail", Source: "expvar", Description: "Failed to recreate missing TP (G12)."},
				{Column: "tp_missing_close_position", Expvar: "tp_missing_close_position", Source: "expvar", Description: "Emergency close when TP recovery failed (G12)."},
				{Column: "sl_setup_started", Expvar: "sl_setup_started", Source: "expvar", Description: "Stop-loss setup attempt started."},
				{Column: "sl_setup_ok", Expvar: "sl_setup_ok", Source: "expvar", Description: "Stop-loss placed successfully."},
				{Column: "sl_setup_fail", Expvar: "sl_setup_fail", Source: "expvar", Description: "Stop-loss placement failed (G13)."},
				{Column: "sl_setup_cancelled", Expvar: "sl_setup_cancelled", Source: "expvar", Description: "SL setup cancelled during reset."},
				{Column: "sl_price_past_immediate_close", Expvar: "sl_price_past_immediate_close", Source: "expvar", Description: "Price already past SL; immediate close (G13)."},
				{Column: "partial_tp_needs_cancel", Expvar: "partial_tp_needs_cancel", Source: "expvar", Description: "Partial TP path needed a resting cancel (G14)."},
			},
		},
		{
			Title:   "Wake path (Tier 3)",
			Summary: "Coalesced price wakes and private order drain.",
			Metrics: []GuideMetric{
				{Column: "price_wake_signals", Expvar: "price_wake_signals", Source: "expvar", Description: "Ticker price changes that signaled a wake."},
				{Column: "price_exec_runs", Expvar: "price_exec_runs", Source: "expvar", Description: "executeOrder runs after coalesced wakes (G16)."},
				{Column: "private_order_wake_signals", Expvar: "private_order_wake_signals", Source: "expvar", Description: "Private order stream wake signals."},
				{Column: "private_order_events_signaled", Expvar: "private_order_events_signaled", Source: "expvar", Description: "Private order events queued for drain."},
				{Column: "private_order_drain_batches", Expvar: "private_order_drain_batches", Source: "expvar", Description: "Times private orders were drained."},
				{Column: "private_order_events_drained", Expvar: "private_order_events_drained", Source: "expvar", Description: "Private order events processed (G16)."},
			},
		},
		{
			Title:   "Log-derived columns",
			Summary: "Counted from soak.log grep, not expvar.",
			Metrics: []GuideMetric{
				{Column: "connection_lost", Source: "log", Description: "WS disconnect lines (last row used in G11)."},
				{Column: "reconnected", Source: "log", Description: "Successful reconnect/resubscribe lines."},
				{Column: "reconnect_failed", Source: "log", Description: "Failed reconnect attempts (G11)."},
				{Column: "errors_10003", Source: "log", Description: "Log lines with retCode=10003 (related to G2)."},
				{Column: "order_failures", Source: "log", Description: "Lines matching Order create/amend/cancel failed."},
			},
		},
	}

	return guide
}

func profilesForGate(c Catalog, gateID string) []string {
	var out []string
	for name, gates := range c.Profiles {
		for _, id := range gates {
			if id == gateID {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

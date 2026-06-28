package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

// ReportMeta holds run metadata for qa-report.md header.
type ReportMeta struct {
	Title            string
	ServerID         string
	Started          string
	Duration         string
	Interval         string
	Mode             string
	Window           string
	MetricsEstimated bool
	Profile          Profile
	RunDir           string
	SoakLogPath      string
}

// WriteReport writes qa-report.md for a completed soak run.
func WriteReport(path string, meta ReportMeta, d Deltas, gates []GateResult, pass bool) error {
	var b strings.Builder

	title := meta.Title
	if title == "" {
		title = filepath.Base(meta.RunDir)
	}

	fmt.Fprintf(&b, "# QA Report: %s\n\n", title)
	if meta.ServerID != "" {
		fmt.Fprintf(&b, "**Server ID:** %s  \n", meta.ServerID)
	}
	if meta.Started != "" {
		fmt.Fprintf(&b, "**Started:** %s  \n", meta.Started)
	}
	if meta.Duration != "" {
		fmt.Fprintf(&b, "**Duration:** %s  \n", meta.Duration)
	}
	if meta.Interval != "" {
		fmt.Fprintf(&b, "**Interval:** %s  \n", meta.Interval)
	}
	if meta.Mode != "" {
		fmt.Fprintf(&b, "**Mode:** `%s`  \n", meta.Mode)
	}
	if meta.Window != "" {
		fmt.Fprintf(&b, "**Window:** `%s`  \n", meta.Window)
	}
	fmt.Fprintf(&b, "**Profile:** `%s`  \n", meta.Profile)
	if meta.RunDir != "" {
		fmt.Fprintf(&b, "**Run dir:** `%s`  \n", meta.RunDir)
	}

	elapsedSpan := d.LastElapsed - d.FirstElapsed
	fmt.Fprintf(&b, "\n## Summary\n\n")
	fmt.Fprintf(&b, "Samples: **%d** | Elapsed: **%d→%d min** (%d min span)\n\n", d.SampleCount, d.FirstElapsed, d.LastElapsed, elapsedSpan)
	fmt.Fprintf(&b, "**WS messages (overall):** %s", formatInt64(d.WSMessagesOverall))
	if d.WSMessagesDelta > 0 {
		fmt.Fprintf(&b, " (+%s during window)", formatInt64(d.WSMessagesDelta))
	}
	b.WriteString("\n\n")

	if meta.MetricsEstimated {
		fmt.Fprintf(&b, "> **Note:** Start-of-window metrics for some counters were estimated from current expvar (no log proxy). Treat small deltas on `bus_drops`, `order_filter_cancel`, and position counters with caution.\n\n")
	}

	fmt.Fprintf(&b, "### Key deltas (first→last row)\n\n")
	fmt.Fprintf(&b, "```\n")
	fmt.Fprintf(&b, "ws_messages_delta=%s  ws_messages_overall=%s\n", formatInt64(d.WSMessagesDelta), formatInt64(d.WSMessagesOverall))
	fmt.Fprintf(&b, "bus_drops=%s  order_create_ok=%s  order_filter_cancel=%s\n",
		formatInt64(d.BusDrops), formatInt64(d.OrderCreateOK), formatInt64(d.OrderFilterCancel))
	if meta.Profile == ProfileLifecycle || meta.Profile == ProfileLifecycleStrict {
		fmt.Fprintf(&b, "position_opened=%s  position_reset=%s  resting_proxy=%s\n",
			formatInt64(d.PositionOpened), formatInt64(d.PositionReset), formatInt64(d.RestingProxy))
		fmt.Fprintf(&b, "algo_paused=%s  algo_resumed_ok=%s\n",
			formatInt64(d.AlgoPaused), formatInt64(d.AlgoResumedOK))
	}
	fmt.Fprintf(&b, "```\n\n")

	writeLifecycleBreakdown(&b, d, meta.Profile)
	writeFailureTable(&b, d, meta.Profile)
	writeActivityRates(&b, d, elapsedSpan)
	writeWakeHealth(&b, d, meta.Profile)
	writeG7Detail(&b, d, meta.Profile)
	writeLogAppendix(&b, meta.SoakLogPath)

	fmt.Fprintf(&b, "## Gate Results (`gtqa analyze --profile %s`)\n\n", meta.Profile)
	fmt.Fprintf(&b, "| Gate | Result | Detail |\n")
	fmt.Fprintf(&b, "|------|--------|--------|\n")
	for _, g := range gates {
		result := "FAIL"
		if g.Pass {
			result = "PASS"
		}
		fmt.Fprintf(&b, "| %s | **%s** | %s |\n", g.ID, result, g.Detail)
	}

	fmt.Fprintf(&b, "\n## Status\n\n")
	if pass {
		fmt.Fprintf(&b, "**OVERALL: PASS**\n")
	} else {
		fmt.Fprintf(&b, "**OVERALL: FAIL**\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeLifecycleBreakdown(b *strings.Builder, d Deltas, profile Profile) {
	if profile != ProfileLifecycle && profile != ProfileLifecycleStrict && profile != ProfileTPSLHealth {
		return
	}
	if d.PositionReset == 0 && d.PositionOpened == 0 {
		return
	}

	fmt.Fprintf(b, "## Lifecycle breakdown\n\n")
	fmt.Fprintf(b, "| Metric | Delta |\n")
	fmt.Fprintf(b, "|--------|-------|\n")
	fmt.Fprintf(b, "| position_opened | %s |\n", formatInt64(d.PositionOpened))
	fmt.Fprintf(b, "| position_reset | %s |\n", formatInt64(d.PositionReset))
	fmt.Fprintf(b, "| position_reset_sl | %s |\n", formatInt64(d.PositionResetSL))
	fmt.Fprintf(b, "| position_reset_tp | %s |\n", formatInt64(d.PositionResetTP))
	fmt.Fprintf(b, "| position_reset_cancel | %s |\n", formatInt64(d.PositionResetCancel))
	fmt.Fprintf(b, "| position_reset_liquidation | %s |\n", formatInt64(d.PositionResetLiquidation))
	fmt.Fprintf(b, "| position_reset_manual | %s |\n", formatInt64(d.PositionResetManual))
	fmt.Fprintf(b, "| position_reset_other | %s |\n\n", formatInt64(d.PositionResetOther))
}

func writeFailureTable(b *strings.Builder, d Deltas, profile Profile) {
	if profile != ProfileLifecycleStrict && profile != ProfileTPSLHealth {
		return
	}

	type row struct {
		name string
		val  int64
	}
	rows := []row{
		{"order_fail_create", d.OrderFailCreate},
		{"order_fail_amend", d.OrderFailAmend},
		{"order_fail_cancel", d.OrderFailCancel},
		{"order_fail_ret_10003", d.OrderFailRet10003},
		{"order_fail_ret_10006", d.OrderFailRet10006},
		{"order_fail_ret_10001", d.OrderFailRet10001},
		{"order_fail_ret_10016", d.OrderFailRet10016},
		{"order_fail_ret_110001", d.OrderFailRet110001},
		{"order_fail_ret_110007", d.OrderFailRet110007},
		{"order_fail_ret_110013", d.OrderFailRet110013},
		{"order_fail_ret_110021", d.OrderFailRet110021},
		{"order_fail_ret_110090", d.OrderFailRet110090},
		{"order_fail_ret_110094", d.OrderFailRet110094},
		{"order_fail_ret_110059", d.OrderFailRet110059},
		{"order_fail_ret_110123", d.OrderFailRet110123},
		{"order_fail_ret_110126", d.OrderFailRet110126},
		{"algo_stopped_permanent", d.AlgoStoppedPermanent},
	}

	hasFailure := false
	for _, r := range rows {
		if r.val > 0 {
			hasFailure = true
			break
		}
	}
	if !hasFailure && d.OrderFailTotal == 0 {
		return
	}

	fmt.Fprintf(b, "## Failure table (retCode deltas)\n\n")
	fmt.Fprintf(b, "| Counter | Delta |\n")
	fmt.Fprintf(b, "|---------|-------|\n")
	for _, r := range rows {
		if r.val > 0 {
			fmt.Fprintf(b, "| %s | %s |\n", r.name, formatInt64(r.val))
		}
	}
	fmt.Fprintf(b, "| **order_fail_total** | **%s** |\n\n", formatInt64(d.OrderFailTotal))
}

func writeActivityRates(b *strings.Builder, d Deltas, elapsedMin int) {
	if elapsedMin <= 0 {
		return
	}
	hours := float64(elapsedMin) / 60.0

	fmt.Fprintf(b, "## Activity rates (per hour)\n\n")
	fmt.Fprintf(b, "| Metric | Delta | Per hour |\n")
	fmt.Fprintf(b, "|--------|-------|----------|\n")
	rateRows := []struct {
		name string
		val  int64
	}{
		{"order_create_ok", d.OrderCreateOK},
		{"order_amend_ok", d.OrderAmendOK},
		{"order_cancel_ok", d.OrderCancelOK},
		{"position_opened", d.PositionOpened},
		{"position_reset", d.PositionReset},
		{"price_wake_signals", d.PriceWakeSignals},
		{"price_exec_runs", d.PriceExecRuns},
	}
	for _, r := range rateRows {
		if r.val == 0 {
			continue
		}
		perHour := float64(r.val) / hours
		fmt.Fprintf(b, "| %s | %s | %.1f |\n", r.name, formatInt64(r.val), perHour)
	}
	b.WriteString("\n")
}

func writeWakeHealth(b *strings.Builder, d Deltas, profile Profile) {
	if profile != ProfileLifecycleStrict && profile != ProfileTPSLHealth && profile != ProfileWSSOnly {
		return
	}
	if d.PriceWakeSignals == 0 && d.PrivateOrderWakeSignals == 0 && d.PrivateOrderEventsSignaled == 0 {
		return
	}

	fmt.Fprintf(b, "## Wake health\n\n")
	fmt.Fprintf(b, "| Metric | Delta |\n")
	fmt.Fprintf(b, "|--------|-------|\n")
	fmt.Fprintf(b, "| price_wake_signals | %s |\n", formatInt64(d.PriceWakeSignals))
	fmt.Fprintf(b, "| price_exec_runs | %s |\n", formatInt64(d.PriceExecRuns))
	fmt.Fprintf(b, "| private_order_wake_signals | %s |\n", formatInt64(d.PrivateOrderWakeSignals))
	fmt.Fprintf(b, "| private_order_events_signaled | %s |\n", formatInt64(d.PrivateOrderEventsSignaled))
	fmt.Fprintf(b, "| private_order_drain_batches | %s |\n", formatInt64(d.PrivateOrderDrainBatches))
	fmt.Fprintf(b, "| private_order_events_drained | %s |\n\n", formatInt64(d.PrivateOrderEventsDrained))
}

func writeG7Detail(b *strings.Builder, d Deltas, profile Profile) {
	if profile != ProfileLifecycle && profile != ProfileLifecycleStrict {
		return
	}
	fmt.Fprintf(b, "## G7 pause/resume detail\n\n")
	fmt.Fprintf(b, "in_flight=%s allow_missing=%s first_in_flight=%s\n\n",
		formatInt64(d.InFlight), formatInt64(d.AllowMissing), formatInt64(d.FirstInFlight))
}

func writeLogAppendix(b *strings.Builder, soakLogPath string) {
	if soakLogPath == "" {
		return
	}
	data, err := os.ReadFile(soakLogPath)
	if err != nil {
		return
	}
	counts := metrics.CountLogPatterns(string(data))
	if counts.ConnectionLost == 0 && counts.Reconnected == 0 && counts.ReconnectFailed == 0 &&
		counts.Errors10003 == 0 && counts.OrderFailures == 0 {
		return
	}

	fmt.Fprintf(b, "## Log appendix (grep counts)\n\n")
	fmt.Fprintf(b, "| Pattern | Count |\n")
	fmt.Fprintf(b, "|---------|-------|\n")
	fmt.Fprintf(b, "| connection_lost | %d |\n", counts.ConnectionLost)
	fmt.Fprintf(b, "| reconnected | %d |\n", counts.Reconnected)
	fmt.Fprintf(b, "| reconnect_failed | %d |\n", counts.ReconnectFailed)
	fmt.Fprintf(b, "| errors_10003 | %d |\n", counts.Errors10003)
	fmt.Fprintf(b, "| order_failures | %d |\n\n", counts.OrderFailures)
}

func formatInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

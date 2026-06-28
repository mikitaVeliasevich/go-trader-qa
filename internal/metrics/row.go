package metrics

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const TSVHeader = "timestamp_utc\telapsed_min\tws_messages\tbus_drops\tbus_publishes\tticker_parsed\tprivate_parsed\tkline_parsed\torder_create_ok\torder_amend_ok\torder_cancel_ok\torder_filter_cancel\torder_create_blocked_position\tposition_opened\tposition_reset\tposition_reset_sl\tposition_reset_tp\tposition_reset_cancel\tposition_reset_other\talgo_paused\talgo_resumed_ok\torder_fail_create\torder_fail_amend\torder_fail_cancel\torder_fail_ret_10003\torder_fail_ret_10006\torder_fail_ret_10001\torder_fail_ret_10016\torder_fail_ret_110001\torder_fail_ret_110007\torder_fail_ret_110013\torder_fail_ret_110021\torder_fail_ret_110090\torder_fail_ret_110094\torder_fail_ret_110059\torder_fail_ret_110123\torder_fail_ret_110126\talgo_stopped_permanent\trisk_limit_margin_reduced\ttp_missing_create_ok\ttp_missing_create_fail\ttp_missing_close_position\tsl_setup_started\tsl_setup_ok\tsl_setup_fail\tsl_setup_cancelled\tsl_price_past_immediate_close\tpartial_tp_needs_cancel\tposition_reset_liquidation\tposition_reset_manual\tprice_wake_signals\tprice_exec_runs\tprivate_order_wake_signals\tprivate_order_events_signaled\tprivate_order_drain_batches\tprivate_order_events_drained\tconnection_lost\treconnected\treconnect_failed\terrors_10003\torder_failures"

var logPatterns = struct {
	connectionLost  *regexp.Regexp
	reconnected     *regexp.Regexp
	reconnectFailed *regexp.Regexp
	errors10003     *regexp.Regexp
	orderFailures   *regexp.Regexp
}{
	connectionLost:  regexp.MustCompile(`connection lost|Connection lost|public.*disconnect`),
	reconnected:     regexp.MustCompile(`reconnected|Reconnected|resubscribe`),
	reconnectFailed: regexp.MustCompile(`reconnect failed|Reconnect failed`),
	errors10003:     regexp.MustCompile(`retCode=10003|retCode":10003`),
	orderFailures:   regexp.MustCompile(`Order (create|amend|cancel) failed`),
}

// LogCounts holds grep-derived counters from accumulated soak.log text.
type LogCounts struct {
	ConnectionLost  int
	Reconnected     int
	ReconnectFailed int
	Errors10003     int
	OrderFailures   int
}

// CountLogPatterns counts monitor grep patterns in logText (line counts, like grep -cE).
func CountLogPatterns(logText string) LogCounts {
	lines := strings.Split(logText, "\n")
	return LogCounts{
		ConnectionLost:  countMatchingLines(lines, logPatterns.connectionLost),
		Reconnected:     countMatchingLines(lines, logPatterns.reconnected),
		ReconnectFailed: countMatchingLines(lines, logPatterns.reconnectFailed),
		Errors10003:     countMatchingLines(lines, logPatterns.errors10003),
		OrderFailures:   countMatchingLines(lines, logPatterns.orderFailures),
	}
}

func countMatchingLines(lines []string, re *regexp.Regexp) int {
	n := 0
	for _, line := range lines {
		if line != "" && re.MatchString(line) {
			n++
		}
	}
	return n
}

// IntFromVars reads an integer expvar value (JSON number or quoted string).
func IntFromVars(vars map[string]json.RawMessage, key string) int64 {
	raw, ok := vars[key]
	if !ok || len(raw) == 0 {
		return 0
	}

	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			return v
		}
	}

	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int64(f)
	}

	return 0
}

// Row is one metrics.tsv sample (61 columns with elite optional metrics).
type Row struct {
	TimestampUTC               string
	ElapsedMin                 int
	WSMessages                 int64
	BusDrops                   int64
	BusPublishes               int64
	TickerParsed               int64
	PrivateParsed              int64
	KlineParsed                int64
	OrderCreateOK              int64
	OrderAmendOK               int64
	OrderCancelOK              int64
	OrderFilterCancel          int64
	OrderCreateBlockedPosition int64
	PositionOpened             int64
	PositionReset              int64
	PositionResetSL            int64
	PositionResetTP            int64
	PositionResetCancel        int64
	PositionResetOther         int64
	AlgoPaused                 int64
	AlgoResumedOK              int64
	OrderFailCreate            int64
	OrderFailAmend             int64
	OrderFailCancel            int64
	OrderFailRet10003          int64
	OrderFailRet10006          int64
	OrderFailRet10001          int64
	OrderFailRet10016          int64
	OrderFailRet110001         int64
	OrderFailRet110007         int64
	OrderFailRet110013         int64
	OrderFailRet110021         int64
	OrderFailRet110090         int64
	OrderFailRet110094         int64
	OrderFailRet110059         int64
	OrderFailRet110123         int64
	OrderFailRet110126         int64
	AlgoStoppedPermanent       int64
	RiskLimitMarginReduced     int64
	TPMissingCreateOK          int64
	TPMissingCreateFail        int64
	TPMissingClosePosition     int64
	SLSetupStarted             int64
	SLSetupOK                  int64
	SLSetupFail                int64
	SLSetupCancelled           int64
	SLPricePastImmediateClose  int64
	PartialTPNeedsCancel       int64
	PositionResetLiquidation   int64
	PositionResetManual        int64
	PriceWakeSignals           int64
	PriceExecRuns              int64
	PrivateOrderWakeSignals    int64
	PrivateOrderEventsSignaled int64
	PrivateOrderDrainBatches   int64
	PrivateOrderEventsDrained  int64
	ConnectionLost             int
	Reconnected                int
	ReconnectFailed            int
	Errors10003                int
	OrderFailures              int
}

// RowFromVars builds a Row from expvar JSON and log grep counts.
func RowFromVars(timestamp string, elapsedMin int, vars map[string]json.RawMessage, counts LogCounts) Row {
	row := Row{
		TimestampUTC:               timestamp,
		ElapsedMin:                 elapsedMin,
		WSMessages:                 IntFromVars(vars, "ws_messages_received"),
		BusDrops:                   IntFromVars(vars, "bus_drops"),
		BusPublishes:               IntFromVars(vars, "bus_publishes"),
		TickerParsed:               IntFromVars(vars, "ticker_messages_parsed"),
		PrivateParsed:              IntFromVars(vars, "private_messages_parsed"),
		KlineParsed:                IntFromVars(vars, "kline_messages_parsed"),
		OrderCreateOK:              IntFromVars(vars, "order_create_ok"),
		OrderAmendOK:               IntFromVars(vars, "order_amend_ok"),
		OrderCancelOK:              IntFromVars(vars, "order_cancel_ok"),
		OrderFilterCancel:          IntFromVars(vars, "order_filter_cancel"),
		OrderCreateBlockedPosition: IntFromVars(vars, "order_create_blocked_position"),
		PositionOpened:             IntFromVars(vars, "position_opened"),
		PositionReset:              IntFromVars(vars, "position_reset"),
		PositionResetSL:            IntFromVars(vars, "position_reset_sl"),
		PositionResetTP:            IntFromVars(vars, "position_reset_tp"),
		PositionResetCancel:        IntFromVars(vars, "position_reset_cancel"),
		PositionResetOther:         IntFromVars(vars, "position_reset_other"),
		AlgoPaused:                 IntFromVars(vars, "algo_paused"),
		AlgoResumedOK:              IntFromVars(vars, "algo_resumed_ok"),
		ConnectionLost:             counts.ConnectionLost,
		Reconnected:                counts.Reconnected,
		ReconnectFailed:            counts.ReconnectFailed,
		Errors10003:                counts.Errors10003,
		OrderFailures:              counts.OrderFailures,
	}
	for _, col := range OptionalTSVColumns {
		setOptionalFromVars(&row, col, IntFromVars(vars, col))
	}
	return row
}

func setOptionalFromVars(row *Row, col string, v int64) {
	switch col {
	case "order_fail_create":
		row.OrderFailCreate = v
	case "order_fail_amend":
		row.OrderFailAmend = v
	case "order_fail_cancel":
		row.OrderFailCancel = v
	case "order_fail_ret_10003":
		row.OrderFailRet10003 = v
	case "order_fail_ret_10006":
		row.OrderFailRet10006 = v
	case "order_fail_ret_10001":
		row.OrderFailRet10001 = v
	case "order_fail_ret_10016":
		row.OrderFailRet10016 = v
	case "order_fail_ret_110001":
		row.OrderFailRet110001 = v
	case "order_fail_ret_110007":
		row.OrderFailRet110007 = v
	case "order_fail_ret_110013":
		row.OrderFailRet110013 = v
	case "order_fail_ret_110021":
		row.OrderFailRet110021 = v
	case "order_fail_ret_110090":
		row.OrderFailRet110090 = v
	case "order_fail_ret_110094":
		row.OrderFailRet110094 = v
	case "order_fail_ret_110059":
		row.OrderFailRet110059 = v
	case "order_fail_ret_110123":
		row.OrderFailRet110123 = v
	case "order_fail_ret_110126":
		row.OrderFailRet110126 = v
	case "algo_stopped_permanent":
		row.AlgoStoppedPermanent = v
	case "risk_limit_margin_reduced":
		row.RiskLimitMarginReduced = v
	case "tp_missing_create_ok":
		row.TPMissingCreateOK = v
	case "tp_missing_create_fail":
		row.TPMissingCreateFail = v
	case "tp_missing_close_position":
		row.TPMissingClosePosition = v
	case "sl_setup_started":
		row.SLSetupStarted = v
	case "sl_setup_ok":
		row.SLSetupOK = v
	case "sl_setup_fail":
		row.SLSetupFail = v
	case "sl_setup_cancelled":
		row.SLSetupCancelled = v
	case "sl_price_past_immediate_close":
		row.SLPricePastImmediateClose = v
	case "partial_tp_needs_cancel":
		row.PartialTPNeedsCancel = v
	case "position_reset_liquidation":
		row.PositionResetLiquidation = v
	case "position_reset_manual":
		row.PositionResetManual = v
	case "price_wake_signals":
		row.PriceWakeSignals = v
	case "price_exec_runs":
		row.PriceExecRuns = v
	case "private_order_wake_signals":
		row.PrivateOrderWakeSignals = v
	case "private_order_events_signaled":
		row.PrivateOrderEventsSignaled = v
	case "private_order_drain_batches":
		row.PrivateOrderDrainBatches = v
	case "private_order_events_drained":
		row.PrivateOrderEventsDrained = v
	}
}

func formatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func formatInt(v int) string {
	return strconv.Itoa(v)
}

// TSVLine returns a tab-separated metrics row without trailing newline.
func (r Row) TSVLine() string {
	fields := []string{
		r.TimestampUTC,
		formatInt(r.ElapsedMin),
		formatInt64(r.WSMessages),
		formatInt64(r.BusDrops),
		formatInt64(r.BusPublishes),
		formatInt64(r.TickerParsed),
		formatInt64(r.PrivateParsed),
		formatInt64(r.KlineParsed),
		formatInt64(r.OrderCreateOK),
		formatInt64(r.OrderAmendOK),
		formatInt64(r.OrderCancelOK),
		formatInt64(r.OrderFilterCancel),
		formatInt64(r.OrderCreateBlockedPosition),
		formatInt64(r.PositionOpened),
		formatInt64(r.PositionReset),
		formatInt64(r.PositionResetSL),
		formatInt64(r.PositionResetTP),
		formatInt64(r.PositionResetCancel),
		formatInt64(r.PositionResetOther),
		formatInt64(r.AlgoPaused),
		formatInt64(r.AlgoResumedOK),
	}
	for _, col := range OptionalTSVColumns {
		fields = append(fields, formatInt64(rowInt64(r, col)))
	}
	fields = append(fields,
		formatInt(r.ConnectionLost),
		formatInt(r.Reconnected),
		formatInt(r.ReconnectFailed),
		formatInt(r.Errors10003),
		formatInt(r.OrderFailures),
	)
	return strings.Join(fields, "\t")
}

// FormatIssueSnippet mirrors soak-monitor.sh issues.log bus_drops block.
func FormatIssueSnippet(timestamp string, elapsedMin int, busDrops int64, counts LogCounts, soakLog string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== ISSUE @ %s elapsed=%dmin drops=%d reconnect_failed=%d 10003=%d ===\n",
		timestamp, elapsedMin, busDrops, counts.ReconnectFailed, counts.Errors10003)
	b.WriteString(lastLines(soakLog, 20))
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func lastLines(text string, n int) string {
	if text == "" || n <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

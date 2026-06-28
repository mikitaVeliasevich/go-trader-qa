package metrics

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const TSVHeader = "timestamp_utc\telapsed_min\tws_messages\tbus_drops\tbus_publishes\tticker_parsed\tprivate_parsed\tkline_parsed\torder_create_ok\torder_amend_ok\torder_cancel_ok\torder_filter_cancel\torder_create_blocked_position\tposition_opened\tposition_reset\tposition_reset_sl\tposition_reset_tp\tposition_reset_cancel\tposition_reset_other\talgo_paused\talgo_resumed_ok\tconnection_lost\treconnected\treconnect_failed\terrors_10003\torder_failures"

var logPatterns = struct {
	connectionLost    *regexp.Regexp
	reconnected       *regexp.Regexp
	reconnectFailed   *regexp.Regexp
	errors10003       *regexp.Regexp
	orderFailures     *regexp.Regexp
}{
	connectionLost:  regexp.MustCompile(`connection lost|Connection lost|public.*disconnect`),
	reconnected:     regexp.MustCompile(`reconnected|Reconnected|resubscribe`),
	reconnectFailed: regexp.MustCompile(`reconnect failed|Reconnect failed`),
	errors10003:     regexp.MustCompile(`retCode=10003|retCode":10003`),
	orderFailures:   regexp.MustCompile(`Order failed|order failed`),
}

// LogCounts holds grep-derived counters from accumulated soak.log text.
type LogCounts struct {
	ConnectionLost    int
	Reconnected       int
	ReconnectFailed   int
	Errors10003       int
	OrderFailures     int
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

// Row is one metrics.tsv sample (26 columns).
type Row struct {
	TimestampUTC              string
	ElapsedMin                int
	WSMessages                int64
	BusDrops                  int64
	BusPublishes              int64
	TickerParsed              int64
	PrivateParsed             int64
	KlineParsed               int64
	OrderCreateOK             int64
	OrderAmendOK              int64
	OrderCancelOK             int64
	OrderFilterCancel         int64
	OrderCreateBlockedPosition int64
	PositionOpened            int64
	PositionReset             int64
	PositionResetSL           int64
	PositionResetTP           int64
	PositionResetCancel       int64
	PositionResetOther        int64
	AlgoPaused                int64
	AlgoResumedOK             int64
	ConnectionLost            int
	Reconnected               int
	ReconnectFailed           int
	Errors10003               int
	OrderFailures             int
}

// RowFromVars builds a Row from expvar JSON and log grep counts.
func RowFromVars(timestamp string, elapsedMin int, vars map[string]json.RawMessage, counts LogCounts) Row {
	return Row{
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
		formatInt(r.ConnectionLost),
		formatInt(r.Reconnected),
		formatInt(r.ReconnectFailed),
		formatInt(r.Errors10003),
		formatInt(r.OrderFailures),
	}
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

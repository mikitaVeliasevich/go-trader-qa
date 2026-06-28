package metrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var requiredTSVColumns = []string{
	"timestamp_utc",
	"elapsed_min",
	"ws_messages",
	"bus_drops",
	"bus_publishes",
	"ticker_parsed",
	"private_parsed",
	"kline_parsed",
	"order_create_ok",
	"order_amend_ok",
	"order_cancel_ok",
	"order_filter_cancel",
	"order_create_blocked_position",
	"position_opened",
	"position_reset",
	"position_reset_sl",
	"position_reset_tp",
	"position_reset_cancel",
	"position_reset_other",
	"algo_paused",
	"algo_resumed_ok",
	"connection_lost",
	"reconnected",
	"reconnect_failed",
	"errors_10003",
	"order_failures",
}

// ReadTSV parses metrics.tsv into data rows (header skipped).
func ReadTSV(path string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("empty metrics file: %s", path)
	}

	header := strings.Split(scanner.Text(), "\t")
	colIndex := make(map[string]int, len(header))
	for i, name := range header {
		colIndex[name] = i
	}

	var rows []Row
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		row, err := parseTSVRow(fields, colIndex)
		if err != nil {
			return nil, fmt.Errorf("parse row %q: %w", line, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data rows in %s", path)
	}
	return rows, nil
}

func parseTSVRow(fields []string, colIndex map[string]int) (Row, error) {
	getRequired := func(col string) (string, error) {
		idx, ok := colIndex[col]
		if !ok {
			return "", fmt.Errorf("missing column %q", col)
		}
		if idx >= len(fields) {
			return "0", nil
		}
		return fields[idx], nil
	}

	getOptional := func(col string) string {
		idx, ok := colIndex[col]
		if !ok || idx >= len(fields) {
			return "0"
		}
		return fields[idx]
	}

	parseInt := func(s string) (int, error) {
		if s == "" {
			return 0, nil
		}
		return strconv.Atoi(s)
	}

	parseInt64 := func(s string) (int64, error) {
		if s == "" {
			return 0, nil
		}
		return strconv.ParseInt(s, 10, 64)
	}

	for _, col := range requiredTSVColumns {
		if _, ok := colIndex[col]; !ok {
			return Row{}, fmt.Errorf("missing column %q", col)
		}
	}

	ts, err := getRequired("timestamp_utc")
	if err != nil {
		return Row{}, err
	}
	elapsedStr, err := getRequired("elapsed_min")
	if err != nil {
		return Row{}, err
	}
	elapsed, err := parseInt(elapsedStr)
	if err != nil {
		return Row{}, err
	}

	row := Row{TimestampUTC: ts, ElapsedMin: elapsed}

	int64Cols := map[string]*int64{
		"ws_messages":                   &row.WSMessages,
		"bus_drops":                     &row.BusDrops,
		"bus_publishes":                 &row.BusPublishes,
		"ticker_parsed":                 &row.TickerParsed,
		"private_parsed":                &row.PrivateParsed,
		"kline_parsed":                  &row.KlineParsed,
		"order_create_ok":               &row.OrderCreateOK,
		"order_amend_ok":                &row.OrderAmendOK,
		"order_cancel_ok":               &row.OrderCancelOK,
		"order_filter_cancel":           &row.OrderFilterCancel,
		"order_create_blocked_position": &row.OrderCreateBlockedPosition,
		"position_opened":               &row.PositionOpened,
		"position_reset":                &row.PositionReset,
		"position_reset_sl":             &row.PositionResetSL,
		"position_reset_tp":             &row.PositionResetTP,
		"position_reset_cancel":         &row.PositionResetCancel,
		"position_reset_other":          &row.PositionResetOther,
		"algo_paused":                   &row.AlgoPaused,
		"algo_resumed_ok":               &row.AlgoResumedOK,
	}
	for col, dst := range int64Cols {
		s, err := getRequired(col)
		if err != nil {
			return Row{}, err
		}
		v, err := parseInt64(s)
		if err != nil {
			return Row{}, err
		}
		*dst = v
	}

	for _, col := range OptionalTSVColumns {
		s := getOptional(col)
		v, err := parseInt64(s)
		if err != nil {
			return Row{}, fmt.Errorf("column %q: %w", col, err)
		}
		setOptionalFromVars(&row, col, v)
	}

	intCols := map[string]*int{
		"connection_lost":  &row.ConnectionLost,
		"reconnected":      &row.Reconnected,
		"reconnect_failed": &row.ReconnectFailed,
		"errors_10003":     &row.Errors10003,
		"order_failures":   &row.OrderFailures,
	}
	for col, dst := range intCols {
		s, err := getRequired(col)
		if err != nil {
			return Row{}, err
		}
		v, err := parseInt(s)
		if err != nil {
			return Row{}, err
		}
		*dst = v
	}

	return row, nil
}

// MetricVal returns a column value for data row index (0 = first data row).
func MetricVal(rows []Row, column string, rowIndex int) int64 {
	if rowIndex < 0 || rowIndex >= len(rows) {
		return 0
	}
	return rowInt64(rows[rowIndex], column)
}

// MetricDelta returns last-minus-first for a named TSV column.
func MetricDelta(rows []Row, column string) int64 {
	if len(rows) < 1 {
		return 0
	}
	return rowInt64(rows[len(rows)-1], column) - rowInt64(rows[0], column)
}

func rowInt64(r Row, column string) int64 {
	switch column {
	case "ws_messages":
		return r.WSMessages
	case "bus_drops":
		return r.BusDrops
	case "bus_publishes":
		return r.BusPublishes
	case "ticker_parsed":
		return r.TickerParsed
	case "private_parsed":
		return r.PrivateParsed
	case "kline_parsed":
		return r.KlineParsed
	case "order_create_ok":
		return r.OrderCreateOK
	case "order_amend_ok":
		return r.OrderAmendOK
	case "order_cancel_ok":
		return r.OrderCancelOK
	case "order_filter_cancel":
		return r.OrderFilterCancel
	case "order_create_blocked_position":
		return r.OrderCreateBlockedPosition
	case "position_opened":
		return r.PositionOpened
	case "position_reset":
		return r.PositionReset
	case "position_reset_sl":
		return r.PositionResetSL
	case "position_reset_tp":
		return r.PositionResetTP
	case "position_reset_cancel":
		return r.PositionResetCancel
	case "position_reset_other":
		return r.PositionResetOther
	case "algo_paused":
		return r.AlgoPaused
	case "algo_resumed_ok":
		return r.AlgoResumedOK
	case "order_fail_create":
		return r.OrderFailCreate
	case "order_fail_amend":
		return r.OrderFailAmend
	case "order_fail_cancel":
		return r.OrderFailCancel
	case "order_fail_ret_10003":
		return r.OrderFailRet10003
	case "order_fail_ret_10006":
		return r.OrderFailRet10006
	case "order_fail_ret_10001":
		return r.OrderFailRet10001
	case "order_fail_ret_10016":
		return r.OrderFailRet10016
	case "order_fail_ret_110001":
		return r.OrderFailRet110001
	case "order_fail_ret_110007":
		return r.OrderFailRet110007
	case "order_fail_ret_110013":
		return r.OrderFailRet110013
	case "order_fail_ret_110021":
		return r.OrderFailRet110021
	case "order_fail_ret_110090":
		return r.OrderFailRet110090
	case "order_fail_ret_110094":
		return r.OrderFailRet110094
	case "order_fail_ret_110059":
		return r.OrderFailRet110059
	case "order_fail_ret_110123":
		return r.OrderFailRet110123
	case "order_fail_ret_110126":
		return r.OrderFailRet110126
	case "algo_stopped_permanent":
		return r.AlgoStoppedPermanent
	case "risk_limit_margin_reduced":
		return r.RiskLimitMarginReduced
	case "tp_missing_create_ok":
		return r.TPMissingCreateOK
	case "tp_missing_create_fail":
		return r.TPMissingCreateFail
	case "tp_missing_close_position":
		return r.TPMissingClosePosition
	case "sl_setup_started":
		return r.SLSetupStarted
	case "sl_setup_ok":
		return r.SLSetupOK
	case "sl_setup_fail":
		return r.SLSetupFail
	case "sl_setup_cancelled":
		return r.SLSetupCancelled
	case "sl_price_past_immediate_close":
		return r.SLPricePastImmediateClose
	case "partial_tp_needs_cancel":
		return r.PartialTPNeedsCancel
	case "position_reset_liquidation":
		return r.PositionResetLiquidation
	case "position_reset_manual":
		return r.PositionResetManual
	case "price_wake_signals":
		return r.PriceWakeSignals
	case "price_exec_runs":
		return r.PriceExecRuns
	case "private_order_wake_signals":
		return r.PrivateOrderWakeSignals
	case "private_order_events_signaled":
		return r.PrivateOrderEventsSignaled
	case "private_order_drain_batches":
		return r.PrivateOrderDrainBatches
	case "private_order_events_drained":
		return r.PrivateOrderEventsDrained
	case "connection_lost":
		return int64(r.ConnectionLost)
	case "reconnected":
		return int64(r.Reconnected)
	case "reconnect_failed":
		return int64(r.ReconnectFailed)
	case "errors_10003":
		return int64(r.Errors10003)
	case "order_failures":
		return int64(r.OrderFailures)
	case "elapsed_min":
		return int64(r.ElapsedMin)
	default:
		return 0
	}
}

package metrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

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
	get := func(col string) (string, error) {
		idx, ok := colIndex[col]
		if !ok {
			return "", fmt.Errorf("missing column %q", col)
		}
		if idx >= len(fields) {
			return "0", nil
		}
		return fields[idx], nil
	}

	parseInt := func(col string) (int, error) {
		s, err := get(col)
		if err != nil {
			return 0, err
		}
		if s == "" {
			return 0, nil
		}
		return strconv.Atoi(s)
	}

	parseInt64 := func(col string) (int64, error) {
		s, err := get(col)
		if err != nil {
			return 0, err
		}
		if s == "" {
			return 0, nil
		}
		return strconv.ParseInt(s, 10, 64)
	}

	ts, err := get("timestamp_utc")
	if err != nil {
		return Row{}, err
	}
	elapsed, err := parseInt("elapsed_min")
	if err != nil {
		return Row{}, err
	}

	row := Row{TimestampUTC: ts, ElapsedMin: elapsed}

	int64Cols := map[string]*int64{
		"ws_messages":                   &row.WSMessages,
		"bus_drops":                       &row.BusDrops,
		"bus_publishes":                   &row.BusPublishes,
		"ticker_parsed":                   &row.TickerParsed,
		"private_parsed":                  &row.PrivateParsed,
		"kline_parsed":                    &row.KlineParsed,
		"order_create_ok":                 &row.OrderCreateOK,
		"order_amend_ok":                  &row.OrderAmendOK,
		"order_cancel_ok":                 &row.OrderCancelOK,
		"order_filter_cancel":             &row.OrderFilterCancel,
		"order_create_blocked_position":   &row.OrderCreateBlockedPosition,
		"position_opened":                 &row.PositionOpened,
		"position_reset":                  &row.PositionReset,
		"position_reset_sl":               &row.PositionResetSL,
		"position_reset_tp":               &row.PositionResetTP,
		"position_reset_cancel":           &row.PositionResetCancel,
		"position_reset_other":            &row.PositionResetOther,
		"algo_paused":                     &row.AlgoPaused,
		"algo_resumed_ok":                 &row.AlgoResumedOK,
	}
	for col, dst := range int64Cols {
		v, err := parseInt64(col)
		if err != nil {
			return Row{}, err
		}
		*dst = v
	}

	intCols := map[string]*int{
		"connection_lost":   &row.ConnectionLost,
		"reconnected":       &row.Reconnected,
		"reconnect_failed":  &row.ReconnectFailed,
		"errors_10003":      &row.Errors10003,
		"order_failures":    &row.OrderFailures,
	}
	for col, dst := range intCols {
		v, err := parseInt(col)
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

package metrics

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTSVHeaderGolden(t *testing.T) {
	want := "timestamp_utc\telapsed_min\tws_messages\tbus_drops\tbus_publishes\tticker_parsed\tprivate_parsed\tkline_parsed\torder_create_ok\torder_amend_ok\torder_cancel_ok\torder_filter_cancel\torder_create_blocked_position\tposition_opened\tposition_reset\tposition_reset_sl\tposition_reset_tp\tposition_reset_cancel\tposition_reset_other\talgo_paused\talgo_resumed_ok\tconnection_lost\treconnected\treconnect_failed\terrors_10003\torder_failures"
	if TSVHeader != want {
		t.Errorf("TSVHeader mismatch\ngot:  %q\nwant: %q", TSVHeader, want)
	}
	fields := strings.Split(TSVHeader, "\t")
	if len(fields) != 26 {
		t.Fatalf("TSVHeader has %d columns, want 26", len(fields))
	}
}

func TestRowFromVarsFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/vars_sample.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var vars map[string]json.RawMessage
	if err := json.Unmarshal(data, &vars); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	counts := LogCounts{
		ConnectionLost:  2,
		Reconnected:     1,
		ReconnectFailed: 0,
		Errors10003:     3,
		OrderFailures:   1,
	}

	row := RowFromVars("2026-06-28T12:00:00Z", 5, vars, counts)

	want := "2026-06-28T12:00:00Z\t5\t100\t0\t50\t10\t20\t30\t1\t2\t3\t4\t5\t6\t7\t8\t9\t10\t11\t12\t13\t2\t1\t0\t3\t1"
	if got := row.TSVLine(); got != want {
		t.Errorf("TSVLine = %q, want %q", got, want)
	}
}

func TestCountLogPatternsFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/soak_sample.log")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got := CountLogPatterns(string(data))
	want := LogCounts{
		ConnectionLost:  2,
		Reconnected:     2,
		ReconnectFailed: 1,
		Errors10003:     2,
		OrderFailures:   2,
	}
	if got != want {
		t.Errorf("CountLogPatterns = %+v, want %+v", got, want)
	}
}

func TestIntFromVars(t *testing.T) {
	vars := map[string]json.RawMessage{
		"num":   json.RawMessage(`42`),
		"str":   json.RawMessage(`"99"`),
		"float": json.RawMessage(`12.0`),
		"missing": nil,
	}

	if got := IntFromVars(vars, "num"); got != 42 {
		t.Errorf("num = %d, want 42", got)
	}
	if got := IntFromVars(vars, "str"); got != 99 {
		t.Errorf("str = %d, want 99", got)
	}
	if got := IntFromVars(vars, "float"); got != 12 {
		t.Errorf("float = %d, want 12", got)
	}
	if got := IntFromVars(vars, "missing"); got != 0 {
		t.Errorf("missing = %d, want 0", got)
	}
}

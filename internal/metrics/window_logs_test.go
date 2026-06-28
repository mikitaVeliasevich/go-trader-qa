package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestParseWindow(t *testing.T) {
	w, err := ParseWindow("30m")
	if err != nil {
		t.Fatal(err)
	}
	if w.Lifecycle || w.Duration != 30*time.Minute {
		t.Fatalf("got %+v", w)
	}

	w, err = ParseWindow("lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	if !w.Lifecycle {
		t.Fatalf("got %+v", w)
	}
}

func TestParseLogTimestamp(t *testing.T) {
	ts, ok := ParseLogTimestamp("2026/06/28 10:19:47 📢[INFO] hello")
	if !ok {
		t.Fatal("expected parse ok")
	}
	if ts.Year() != 2026 || ts.Month() != 6 || ts.Day() != 28 {
		t.Fatalf("unexpected ts: %v", ts)
	}

	_, ok = ParseLogTimestamp("no timestamp here")
	if ok {
		t.Fatal("expected parse fail")
	}
}

func TestFilterLogsByWindow(t *testing.T) {
	logs := strings.Join([]string{
		"2026/06/28 10:00:00 early",
		"2026/06/28 10:30:00 in",
		"2026/06/28 11:00:00 late",
	}, "\n") + "\n"

	start, _ := time.ParseInLocation(logTimestampLayout, "2026/06/28 10:29:00", time.Local)
	end, _ := time.ParseInLocation(logTimestampLayout, "2026/06/28 10:31:00", time.Local)

	got := FilterLogsByWindow(logs, start, end)
	if strings.Contains(got, "early") || strings.Contains(got, "late") {
		t.Fatalf("filter leaked lines: %q", got)
	}
	if !strings.Contains(got, "in") {
		t.Fatalf("missing in-window line: %q", got)
	}
}

func TestFetchLogsForWindowExpandsTail(t *testing.T) {
	calls := 0
	start, _ := time.ParseInLocation(logTimestampLayout, "2026/06/28 10:00:00", time.Local)

	result, err := FetchLogsForWindow(func(tail int) (string, error) {
		calls++
		if tail < 5000 {
			return "2026/06/28 10:30:00 recent only\n", nil
		}
		return "2026/06/28 09:50:00 old\n2026/06/28 10:30:00 recent\n", nil
	}, start)
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("expected multiple tail calls, got %d", calls)
	}
	if !strings.Contains(result.Logs, "old") {
		t.Fatalf("logs = %q", result.Logs)
	}
}

func TestReconstructStartRowSubtractsWindowEvents(t *testing.T) {
	end := Row{
		TimestampUTC:  "2026-06-28T11:00:00Z",
		ElapsedMin:    30,
		OrderCreateOK: 10,
		BusDrops:      2,
	}
	events := WindowEventCounts{OrderCreateOK: 3}
	got := ReconstructStartRow(end, events, false)
	if got.Start.OrderCreateOK != 7 {
		t.Fatalf("order_create_ok start = %d, want 7", got.Start.OrderCreateOK)
	}
	if !got.Estimated {
		t.Fatal("expected estimated=true when bus_drops present")
	}
}

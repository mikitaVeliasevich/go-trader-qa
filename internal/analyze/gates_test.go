package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

func TestGateG4NoLifecycle(t *testing.T) {
	d := Deltas{PositionOpened: 0, PositionReset: 0}
	gates, pass := EvaluateGates(d, "", ProfileLifecycle)
	if !pass {
		t.Fatalf("expected pass for 0/0 lifecycle, got FAIL")
	}
	if gates[3].ID != "G4" || !gates[3].Pass {
		t.Fatalf("G4: %+v", gates[3])
	}
}

func TestGateG4BothActive(t *testing.T) {
	d := Deltas{PositionOpened: 10, PositionReset: 8}
	gates, _ := EvaluateGates(d, "", ProfileLifecycle)
	if gates[3].ID != "G4" || !gates[3].Pass {
		t.Fatalf("G4: %+v", gates[3])
	}
}

func TestGateG4Inconsistent(t *testing.T) {
	d := Deltas{PositionOpened: 5, PositionReset: 0}
	gates, pass := EvaluateGates(d, "", ProfileLifecycle)
	if pass {
		t.Fatal("expected FAIL for inconsistent lifecycle")
	}
	if gates[3].ID != "G4" || gates[3].Pass {
		t.Fatalf("G4: %+v", gates[3])
	}
}

func TestComputeDeltasWSMessages(t *testing.T) {
	rows := []metrics.Row{
		{ElapsedMin: 0, WSMessages: 11172799},
		{ElapsedMin: 5, WSMessages: 11451533},
	}
	d := ComputeDeltas(rows)
	if d.WSMessagesOverall != 11451533 {
		t.Fatalf("overall=%d", d.WSMessagesOverall)
	}
	if d.WSMessagesDelta != 278734 {
		t.Fatalf("delta=%d", d.WSMessagesDelta)
	}
}

func TestRunWritesWSMessagesToReport(t *testing.T) {
	dir := t.TempDir()
	tsv := strings.Join([]string{
		metrics.TSVHeader,
		"2026-06-28T13:44:01Z\t0\t11172799\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0",
		"2026-06-28T13:48:53Z\t5\t11451533\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "metrics.tsv"), []byte(tsv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "soak.log"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(dir, ProfileLifecycle)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "**WS messages (overall):** 11451533") {
		t.Fatalf("missing ws overall in report:\n%s", text)
	}
	if !strings.Contains(text, "ws_messages_overall=11451533") {
		t.Fatalf("missing ws in key deltas:\n%s", text)
	}
}

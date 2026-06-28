package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReportMetricsEstimatedFootnote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qa-report.md")

	meta := ReportMeta{
		Title:            "test-run",
		Profile:          ProfileWSSOnly,
		Mode:             "analyze",
		Window:           "30m",
		MetricsEstimated: true,
	}
	d := Deltas{SampleCount: 2, FirstElapsed: 0, LastElapsed: 30}
	gates := []GateResult{{ID: "G1", Pass: true, Detail: "bus_drops delta=0"}}

	if err := WriteReport(path, meta, d, gates, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "estimated from current expvar") {
		t.Fatalf("missing footnote: %s", body)
	}
	if !strings.Contains(body, "**Mode:** `analyze`") {
		t.Fatalf("missing mode header: %s", body)
	}
}

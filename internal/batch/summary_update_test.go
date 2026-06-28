package batch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateJobOverallFromReanalyze(t *testing.T) {
	root := t.TempDir()
	batchDir := filepath.Join(root, "batch-test")
	jobsDir := filepath.Join(batchDir, "jobs")
	runDir := filepath.Join(jobsDir, "2026-06-28T13-43-53Z-11")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}

	summary := BatchSummary{
		Batch: SoakBatch{ID: "batch-test", Status: "complete"},
		Jobs: []SoakJob{
			{ServerID: 11, RunDir: runDir, Status: jobComplete, Overall: overallFail},
		},
		Fail: 1,
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(batchDir, "batch-summary.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateJobOverallFromReanalyze(runDir, true); err != nil {
		t.Fatalf("UpdateJobOverallFromReanalyze: %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(batchDir, "batch-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out BatchSummary
	if err := json.Unmarshal(updated, &out); err != nil {
		t.Fatal(err)
	}
	if out.Jobs[0].Overall != overallPass {
		t.Fatalf("overall = %q, want PASS", out.Jobs[0].Overall)
	}
	if out.Pass != 1 || out.Fail != 0 {
		t.Fatalf("counts pass=%d fail=%d, want 1/0", out.Pass, out.Fail)
	}
}

func TestUpdateJobOverallFromReanalyze_nonBatchRunDir(t *testing.T) {
	t.Parallel()
	if err := UpdateJobOverallFromReanalyze("/tmp/standalone-run", true); err != nil {
		t.Fatalf("expected nil for non-batch path, got %v", err)
	}
}

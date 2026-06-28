package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateJobOverallFromReanalyze updates batch-summary.json (and .md) after a manual
// re-analyze when runDir lives under a batch jobs/ directory.
func UpdateJobOverallFromReanalyze(runDir string, pass bool) error {
	batchDir, ok := batchDirForRunDir(runDir)
	if !ok {
		return nil
	}

	summaryPath := filepath.Join(batchDir, "batch-summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("read batch-summary.json: %w", err)
	}

	var summary BatchSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("decode batch-summary.json: %w", err)
	}

	target := absPath(runDir)
	updated := false
	overall := overallFail
	if pass {
		overall = overallPass
	}

	for i := range summary.Jobs {
		if runDirsMatch(summary.Jobs[i].RunDir, target) {
			summary.Jobs[i].Overall = overall
			updated = true
			break
		}
	}
	if !updated {
		return nil
	}

	recomputeCounts(&summary)
	if err := writeSummaryJSON(batchDir, summary); err != nil {
		return err
	}
	if err := writeSummaryMarkdown(batchDir, summary); err != nil {
		return err
	}
	return nil
}

func batchDirForRunDir(runDir string) (string, bool) {
	clean := absPath(runDir)
	jobsDir := filepath.Dir(clean)
	if filepath.Base(jobsDir) != "jobs" {
		return "", false
	}
	batchDir := filepath.Dir(jobsDir)
	if _, err := os.Stat(filepath.Join(batchDir, "batch-summary.json")); err != nil {
		return "", false
	}
	return batchDir, true
}

func absPath(p string) string {
	p = filepath.Clean(p)
	if filepath.IsAbs(p) {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	return filepath.Clean(filepath.Join(wd, p))
}

func runDirsMatch(stored, target string) bool {
	a := absPath(stored)
	b := absPath(target)
	if a == b {
		return true
	}
	return strings.HasSuffix(a, string(filepath.Separator)+filepath.Base(b)) &&
		filepath.Base(a) == filepath.Base(b)
}

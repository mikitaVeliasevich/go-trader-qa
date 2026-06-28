package analyze

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

// Result is the outcome of analyzing one soak run directory.
type Result struct {
	RunDir    string
	Profile   Profile
	Pass      bool
	Gates     []GateResult
	Deltas    Deltas
	ReportPath string
}

// Run reads metrics.tsv + soak.log, evaluates gates, writes qa-report.md.
func Run(runDir string, profile Profile) (Result, error) {
	runDir = strings.TrimSpace(runDir)
	if runDir == "" {
		return Result{}, fmt.Errorf("run dir is required")
	}

	switch profile {
	case ProfileLifecycle, ProfileWSSOnly:
	default:
		return Result{}, fmt.Errorf("profile must be lifecycle or wss-only")
	}

	metricsPath := filepath.Join(runDir, "metrics.tsv")
	rows, err := metrics.ReadTSV(metricsPath)
	if err != nil {
		return Result{}, err
	}

	d := ComputeDeltas(rows)
	soakLog := filepath.Join(runDir, "soak.log")
	gates, pass := EvaluateGates(d, soakLog, profile)

	meta := ReportMeta{
		Title:    filepath.Base(runDir),
		Profile:  profile,
		RunDir:   runDir,
	}
	loadRunEnv(filepath.Join(runDir, "run.env"), &meta)

	reportPath := filepath.Join(runDir, "qa-report.md")
	if err := WriteReport(reportPath, meta, d, gates, pass); err != nil {
		return Result{}, err
	}

	return Result{
		RunDir:     runDir,
		Profile:    profile,
		Pass:       pass,
		Gates:      gates,
		Deltas:     d,
		ReportPath: reportPath,
	}, nil
}

func loadRunEnv(path string, meta *ReportMeta) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "server_id":
			meta.ServerID = val
		case "started":
			meta.Started = val
		case "duration":
			meta.Duration = val
		case "interval":
			meta.Interval = val
		case "mode":
			meta.Mode = val
		case "window":
			meta.Window = val
		case "metrics_estimated":
			meta.MetricsEstimated = val == "true"
		}
	}
	if err := scanner.Err(); err != nil {
		return
	}
}

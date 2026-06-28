package sampler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

const retrospectiveJobTimeout = 5 * time.Minute

// RetrospectiveOptions configures a backward analyze run.
type RetrospectiveOptions struct {
	ServerID     int
	Window       metrics.WindowSpec
	ArtifactsDir string
	AccountID    int
}

// RetrospectiveRun fetches logs + current expvar, builds synthetic 2-row metrics.tsv and filtered soak.log.
func RetrospectiveRun(ctx context.Context, client *manager.Client, opts RetrospectiveOptions) (RunResult, error) {
	if opts.ServerID <= 0 {
		return RunResult{}, fmt.Errorf("server_id must be positive")
	}
	if strings.TrimSpace(opts.ArtifactsDir) == "" {
		return RunResult{}, fmt.Errorf("artifacts dir is required")
	}

	ctx, cancel := context.WithTimeout(ctx, retrospectiveJobTimeout)
	defer cancel()

	started := time.Now()
	runDirName := fmt.Sprintf("%s-%d", started.UTC().Format("2006-01-02T15-04-05Z"), opts.ServerID)
	runDir := filepath.Join(opts.ArtifactsDir, runDirName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return RunResult{}, fmt.Errorf("create run dir: %w", err)
	}

	windowEnd := started
	var (
		rawLogs      string
		windowStart  time.Time
		fetchSteps   []int
		finalTail    int
		issuesPath   = filepath.Join(runDir, "issues.log")
	)

	fetch := func(tail int) (string, error) {
		return client.ServerLogs(ctx, opts.ServerID, tail)
	}

	if opts.Window.Lifecycle {
		logs, found, err := metrics.FetchLogsFromBotStart(fetch)
		if err != nil {
			return RunResult{RunDir: runDir}, fmt.Errorf("bootstrap soak.log: %w", err)
		}
		if !found {
			fmt.Fprintf(os.Stderr, "warning: bot start banner %q not found for retrospective lifecycle\n", metrics.BotStartMarker)
		}
		rawLogs = metrics.TrimToLastBotStart(logs)
		oldest, _, ok := logTimeBoundsLocal(rawLogs)
		if !ok {
			return RunResult{RunDir: runDir}, fmt.Errorf("no parseable timestamps after bot boot banner")
		}
		windowStart = oldest
	} else {
		windowStart = windowEnd.Add(-opts.Window.Duration)
		result, err := metrics.FetchLogsForWindow(fetch, windowStart)
		if err != nil {
			return RunResult{RunDir: runDir}, err
		}
		rawLogs = result.Logs
		fetchSteps = result.Steps
		finalTail = result.FinalTail
		_ = appendFile(issuesPath, fmt.Sprintf("log_fetch_steps=%v final_tail=%d oldest=%s\n",
			result.Steps, result.FinalTail, result.Oldest.Format(time.RFC3339)))
	}

	filtered := metrics.FilterLogsByWindow(rawLogs, windowStart, windowEnd)
	if strings.TrimSpace(filtered) == "" {
		return RunResult{RunDir: runDir}, fmt.Errorf("no log lines in window [%s, %s]",
			windowStart.Format(time.RFC3339), windowEnd.Format(time.RFC3339))
	}

	soakLogPath := filepath.Join(runDir, "soak.log")
	if err := os.WriteFile(soakLogPath, []byte(filtered), 0o644); err != nil {
		return RunResult{RunDir: runDir}, fmt.Errorf("write soak.log: %w", err)
	}

	vars, err := client.ServerDebugVars(ctx, opts.ServerID)
	if err != nil {
		return RunResult{RunDir: runDir}, fmt.Errorf("debug/vars: %w", err)
	}

	elapsedMin := int(windowEnd.Sub(windowStart).Minutes())
	if elapsedMin < 1 && windowEnd.After(windowStart) {
		elapsedMin = 1
	}

	logCounts := metrics.CountLogPatterns(filtered)
	windowEvents := metrics.CountWindowEvents(filtered)

	endRow := metrics.RowFromVars(
		windowEnd.UTC().Format("2006-01-02T15:04:05Z"),
		elapsedMin,
		vars,
		logCounts,
	)

	recon := metrics.ReconstructStartRow(endRow, windowEvents, opts.Window.Lifecycle)
	startRow := recon.Start
	startRow.TimestampUTC = windowStart.UTC().Format("2006-01-02T15:04:05Z")
	startRow.ElapsedMin = 0
	if opts.Window.Lifecycle {
		startRow = zeroStartRow(startRow.TimestampUTC)
	}

	metricsPath := filepath.Join(runDir, "metrics.tsv")
	tsv := metrics.TSVHeader + "\n" + startRow.TSVLine() + "\n" + endRow.TSVLine() + "\n"
	if err := os.WriteFile(metricsPath, []byte(tsv), 0o644); err != nil {
		return RunResult{RunDir: runDir}, fmt.Errorf("write metrics.tsv: %w", err)
	}

	runEnv := fmt.Sprintf(
		"server_id=%d\nmode=analyze\nwindow=%s\nretrospective=true\nstarted=%s\nwindow_start=%s\nwindow_end=%s\nlog_tail_final=%d\nmetrics_estimated=%t\n",
		opts.ServerID,
		opts.Window.String(),
		started.UTC().Format(time.RFC3339),
		windowStart.Format(time.RFC3339),
		windowEnd.Format(time.RFC3339),
		finalTail,
		recon.Estimated,
	)
	if len(fetchSteps) > 0 {
		runEnv += fmt.Sprintf("log_fetch_steps=%v\n", fetchSteps)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.env"), []byte(runEnv), 0o644); err != nil {
		return RunResult{RunDir: runDir}, fmt.Errorf("write run.env: %w", err)
	}

	if err := preflight(ctx, client, samplerOptions(opts)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: retrospective preflight: %v\n", err)
	}

	return RunResult{RunDir: runDir, Samples: 2}, nil
}

func samplerOptions(opts RetrospectiveOptions) Options {
	return Options{
		ServerID:  opts.ServerID,
		AccountID: opts.AccountID,
	}
}

func zeroStartRow(timestamp string) metrics.Row {
	return metrics.Row{
		TimestampUTC: timestamp,
		ElapsedMin:   0,
	}
}

func logTimeBoundsLocal(logs string) (oldest, newest time.Time, ok bool) {
	for _, line := range strings.Split(logs, "\n") {
		ts, parsed := metrics.ParseLogTimestamp(line)
		if !parsed {
			continue
		}
		if !ok {
			oldest, newest, ok = ts, ts, true
			continue
		}
		if ts.Before(oldest) {
			oldest = ts
		}
		if ts.After(newest) {
			newest = ts
		}
	}
	return oldest, newest, ok
}

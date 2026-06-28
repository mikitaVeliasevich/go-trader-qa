package sampler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/fleet"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

const (
	defaultInterval = 5 * time.Minute
	logTailLines    = 500
)

// Options configures a remote soak sample run.
type Options struct {
	ServerID     int
	Duration     time.Duration
	Interval     time.Duration
	ArtifactsDir string
	AccountID    int
}

// RunResult holds artifact paths and sample count.
type RunResult struct {
	RunDir  string
	Samples int
}

// RemoteRun samples debug/vars and logs via Manager until duration elapses or ctx is cancelled.
func RemoteRun(ctx context.Context, client *manager.Client, opts Options) (RunResult, error) {
	if opts.ServerID <= 0 {
		return RunResult{}, fmt.Errorf("server_id must be positive")
	}
	if opts.Duration <= 0 {
		return RunResult{}, fmt.Errorf("duration must be positive")
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultInterval
	}
	if strings.TrimSpace(opts.ArtifactsDir) == "" {
		return RunResult{}, fmt.Errorf("artifacts dir is required")
	}

	started := time.Now().UTC()
	runDirName := fmt.Sprintf("%s-%d", started.Format("2006-01-02T15-04-05Z"), opts.ServerID)
	runDir := filepath.Join(opts.ArtifactsDir, runDirName)

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return RunResult{}, fmt.Errorf("create run dir: %w", err)
	}

	runEnvPath := filepath.Join(runDir, "run.env")
	metricsPath := filepath.Join(runDir, "metrics.tsv")
	soakLogPath := filepath.Join(runDir, "soak.log")
	issuesPath := filepath.Join(runDir, "issues.log")

	durationMin := int(opts.Duration.Minutes())
	plannedSamples := int(opts.Duration / opts.Interval)
	if plannedSamples < 1 {
		plannedSamples = 1
	}

	runEnv := fmt.Sprintf("server_id=%d\nduration=%s\ninterval=%s\nstarted=%s\n",
		opts.ServerID,
		opts.Duration.String(),
		opts.Interval.String(),
		started.Format("2006-01-02T15:04:05Z"),
	)
	if err := os.WriteFile(runEnvPath, []byte(runEnv), 0o644); err != nil {
		return RunResult{}, fmt.Errorf("write run.env: %w", err)
	}

	monitorStart := fmt.Sprintf("monitor_start=%s duration_min=%d samples=%d\n",
		started.Format("2006-01-02T15:04:05Z"), durationMin, plannedSamples)
	if err := appendFile(issuesPath, monitorStart); err != nil {
		return RunResult{}, err
	}

	if err := os.WriteFile(metricsPath, []byte(metrics.TSVHeader+"\n"), 0o644); err != nil {
		return RunResult{}, fmt.Errorf("write metrics.tsv header: %w", err)
	}

	if err := preflight(ctx, client, opts); err != nil {
		return RunResult{RunDir: runDir}, err
	}

	soakLog, foundStart, err := metrics.FetchLogsFromBotStart(func(tail int) (string, error) {
		return client.ServerLogs(ctx, opts.ServerID, tail)
	})
	if err != nil {
		return RunResult{RunDir: runDir}, fmt.Errorf("bootstrap soak.log: %w", err)
	}
	if !foundStart {
		fmt.Fprintf(os.Stderr, "warning: bot start banner %q not found (tried tail up to %d)\n",
			metrics.BotStartMarker, metrics.BootstrapTailSteps[len(metrics.BootstrapTailSteps)-1])
	}
	if err := os.WriteFile(soakLogPath, []byte(soakLog), 0o644); err != nil {
		return RunResult{RunDir: runDir}, fmt.Errorf("write soak.log: %w", err)
	}

	deadline := started.Add(opts.Duration)
	samples := 0
	cancelled := false

	for {
		if ctx.Err() != nil {
			cancelled = true
			break
		}

		now := time.Now().UTC()
		elapsedMin := int(now.Sub(started).Minutes())

		vars, err := client.ServerDebugVars(ctx, opts.ServerID)
		if err != nil {
			return RunResult{RunDir: runDir, Samples: samples}, fmt.Errorf("debug/vars sample: %w", err)
		}

		logTail, err := client.ServerLogs(ctx, opts.ServerID, logTailLines)
		if err != nil {
			return RunResult{RunDir: runDir, Samples: samples}, fmt.Errorf("logs sample: %w", err)
		}
		soakLog = metrics.AppendNewLogLines(soakLog, logTail)
		if err := os.WriteFile(soakLogPath, []byte(soakLog), 0o644); err != nil {
			return RunResult{RunDir: runDir, Samples: samples}, fmt.Errorf("write soak.log: %w", err)
		}

		counts := metrics.CountLogPatterns(soakLog)
		row := metrics.RowFromVars(now.Format("2006-01-02T15:04:05Z"), elapsedMin, vars, counts)
		if err := appendFile(metricsPath, row.TSVLine()+"\n"); err != nil {
			return RunResult{RunDir: runDir, Samples: samples}, err
		}
		samples++

		if row.BusDrops > 0 {
			snippet := metrics.FormatIssueSnippet(row.TimestampUTC, row.ElapsedMin, row.BusDrops, counts, soakLog)
			if err := appendFile(issuesPath, snippet); err != nil {
				return RunResult{RunDir: runDir, Samples: samples}, err
			}
		}

		if !time.Now().Before(deadline) {
			break
		}

		sleep := opts.Interval
		remaining := time.Until(deadline)
		if sleep > remaining {
			sleep = remaining
		}
		if sleep <= 0 {
			break
		}

		select {
		case <-ctx.Done():
			cancelled = true
			goto done
		case <-time.After(sleep):
		}
	}

done:
	if cancelled {
		shutdownLine := fmt.Sprintf("monitor_shutdown=%s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
		if err := appendFile(issuesPath, shutdownLine); err != nil {
			return RunResult{RunDir: runDir, Samples: samples}, err
		}
	}

	return RunResult{RunDir: runDir, Samples: samples}, nil
}

func preflight(ctx context.Context, client *manager.Client, opts Options) error {
	status, err := client.ServerStatus(ctx, opts.ServerID)
	if err != nil {
		return fmt.Errorf("pre-flight status: %w", err)
	}

	vars, err := client.ServerDebugVars(ctx, opts.ServerID)
	if err != nil {
		return fmt.Errorf("pre-flight debug/vars: %w", err)
	}
	if _, ok := vars["bus_drops"]; !ok {
		return fmt.Errorf("pre-flight debug/vars: missing bus_drops key")
	}

	if !status.HasAPIKeys {
		fmt.Fprintf(os.Stderr, "warning: server %d hasApiKeys=false\n", opts.ServerID)
	}
	if !status.Running {
		fmt.Fprintf(os.Stderr, "warning: server %d running=false\n", opts.ServerID)
	}

	if opts.AccountID > 0 {
		if err := warnFleetEligibility(ctx, client, opts); err != nil {
			fmt.Fprintf(os.Stderr, "warning: fleet pre-flight: %v\n", err)
		}
	}

	return nil
}

func warnFleetEligibility(ctx context.Context, client *manager.Client, opts Options) error {
	resp, err := client.FleetSubs(ctx, opts.AccountID)
	if err != nil {
		return err
	}

	rows := fleet.BuildRows(resp, resp.Categories)
	for _, row := range rows {
		if row.ServerID != opts.ServerID {
			continue
		}
		if !row.QAEligible {
			fmt.Fprintf(os.Stderr, "warning: sub %d on server %d not QA eligible: %s\n",
				row.DBSubaccountID, opts.ServerID, row.IneligibleReason)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "warning: no sub found for server_id=%d in account %d\n", opts.ServerID, opts.AccountID)
	return nil
}

func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

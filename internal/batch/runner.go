package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/analyze"
	"github.com/dlisovsky/go-trader-qa/internal/fleet"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/dlisovsky/go-trader-qa/internal/metrics"
	"github.com/dlisovsky/go-trader-qa/internal/sampler"
)

// RunOptions configures batch execution.
type RunOptions struct {
	Spec     BatchSpec
	OnUpdate func(Progress)
}

// Run executes a multi-server soak batch.
func Run(ctx context.Context, client *manager.Client, opts RunOptions) (BatchResult, error) {
	spec := opts.Spec
	if len(spec.ServerIDs) == 0 {
		return BatchResult{}, fmt.Errorf("server_ids is required")
	}
	if err := NormalizeBatchSpec(&spec); err != nil {
		return BatchResult{}, err
	}
	if spec.Interval <= 0 {
		spec.Interval = 5 * time.Minute
	}
	if spec.Concurrency <= 0 {
		spec.Concurrency = 2
	}
	if strings.TrimSpace(spec.Profile) == "" {
		spec.Profile = string(analyze.ProfileWSSOnly)
	}
	if strings.TrimSpace(spec.ArtifactsDir) == "" {
		return BatchResult{}, fmt.Errorf("artifacts dir is required")
	}

	batchID := strings.TrimSpace(spec.BatchID)
	if batchID == "" {
		batchID = "batch-" + time.Now().UTC().Format("20060102T150405Z")
	}
	batchDir := filepath.Join(spec.ArtifactsDir, batchID)
	jobsDir := filepath.Join(batchDir, "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		return BatchResult{}, fmt.Errorf("create batch dir: %w", err)
	}

	started := time.Now().UTC()
	windowLabel := ""
	if spec.Mode == ModeAnalyze {
		windowLabel = spec.Window.String()
	}
	batch := SoakBatch{
		ID:             batchID,
		Mode:           spec.Mode,
		Window:         windowLabel,
		ServerIDs:      append([]int(nil), spec.ServerIDs...),
		Duration:       spec.Duration.String(),
		Interval:       spec.Interval.String(),
		Concurrency:    spec.Concurrency,
		Profile:        spec.Profile,
		SkipIneligible: spec.SkipIneligible,
		Status:         BatchRunning,
		Dir:            batchDir,
		StartedAt:      started,
	}
	if spec.Mode == ModeAnalyze {
		batch.Duration = ""
	}

	if err := writeBatchEnv(batchDir, batch); err != nil {
		return BatchResult{}, err
	}

	eligible := map[int]fleet.FleetRow{}
	if spec.SkipIneligible && spec.AccountID > 0 {
		resp, err := client.FleetSubs(ctx, spec.AccountID)
		if err != nil {
			return BatchResult{}, fmt.Errorf("fleet subs: %w", err)
		}
		for _, row := range fleet.BuildRows(resp, resp.Categories) {
			if row.ServerID > 0 {
				eligible[row.ServerID] = row
			}
		}
	}

	jobs := make([]SoakJob, 0, len(spec.ServerIDs))
	for _, sid := range spec.ServerIDs {
		job := SoakJob{
			BatchID:  batchID,
			ServerID: sid,
			Status:   JobQueued,
		}
		if spec.SkipIneligible {
			row, ok := eligible[sid]
			if !ok {
				job.Status = JobSkipped
				job.SkipReason = "no_server"
				job.Overall = overallUnknown
			} else {
				job.PairID = row.PairID
				if !row.QAEligible {
					job.Status = JobSkipped
					job.SkipReason = row.IneligibleReason
					job.Overall = overallUnknown
				}
			}
		}
		jobs = append(jobs, job)
	}

	summary := BatchSummary{Batch: batch, Jobs: jobs}
	recomputeCounts(&summary)
	if err := writeSummaryJSON(batchDir, summary); err != nil {
		return BatchResult{}, err
	}
	emitProgress(opts.OnUpdate, batch, jobs, true)

	sem := make(chan struct{}, spec.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	staggerIdx := 0

	for i := range jobs {
		if jobs[i].Status == JobSkipped {
			continue
		}
		delay := time.Duration(0)
		if spec.Mode == ModeSoak {
			delay = time.Duration(staggerIdx) * jobStagger
			staggerIdx++
		}
		idx := i

		wg.Add(1)
		go func(jobIdx int, startDelay time.Duration) {
			defer wg.Done()

			select {
			case <-time.After(startDelay):
			case <-ctx.Done():
				mu.Lock()
				if jobs[jobIdx].Status == JobQueued {
					jobs[jobIdx].Status = JobFailed
					jobs[jobIdx].Error = ctx.Err().Error()
					jobs[jobIdx].Overall = overallUnknown
					batch.Status = BatchCancelled
					persistSummary(&mu, batchDir, &batch, jobs, opts.OnUpdate)
				}
				mu.Unlock()
				return
			}

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			runOneJob(ctx, client, spec, batchDir, &batch, jobs, jobIdx, &mu, opts)
		}(idx, delay)
	}

	wg.Wait()

	mu.Lock()
	if batch.Status == BatchRunning {
		batch.Status = BatchComplete
		now := time.Now().UTC()
		batch.CompletedAt = &now
	}
	summary = BatchSummary{Batch: batch, Jobs: jobs}
	recomputeCounts(&summary)
	_ = writeSummaryJSON(batchDir, summary)
	_ = writeSummaryMarkdown(batchDir, summary)
	emitProgress(opts.OnUpdate, batch, jobs, false)
	mu.Unlock()

	return BatchResult{Batch: batch, Jobs: jobs, Summary: summary}, nil
}

func runOneJob(
	ctx context.Context,
	client *manager.Client,
	spec BatchSpec,
	batchDir string,
	batch *SoakBatch,
	jobs []SoakJob,
	idx int,
	mu *sync.Mutex,
	opts RunOptions,
) {
	mu.Lock()
	jobs[idx].Status = JobRunning
	persistSummary(mu, batchDir, batch, jobs, opts.OnUpdate)
	mu.Unlock()

	jobsDir := filepath.Join(batchDir, "jobs")
	var result sampler.RunResult
	var err error

	if spec.Mode == ModeAnalyze {
		result, err = sampler.RetrospectiveRun(ctx, client, sampler.RetrospectiveOptions{
			ServerID:     jobs[idx].ServerID,
			Window:       spec.Window,
			ArtifactsDir: jobsDir,
			AccountID:    spec.AccountID,
		})
	} else {
		result, err = sampler.RemoteRun(ctx, client, sampler.Options{
			ServerID:     jobs[idx].ServerID,
			Duration:     spec.Duration,
			Interval:     spec.Interval,
			ArtifactsDir: jobsDir,
			AccountID:    spec.AccountID,
		})
	}

	mu.Lock()
	defer mu.Unlock()

	jobs[idx].RunDir = result.RunDir
	jobs[idx].Samples = result.Samples

	if result.RunDir != "" {
		_ = writeConfigSnapshot(ctx, client, jobs[idx].ServerID, result.RunDir)
		_ = appendRunEnvBatchFields(result.RunDir, jobs[idx].PairID, spec.Profile, batch.ID)
		jobs[idx].LastBusDrops = lastBusDrops(result.RunDir)
	}

	if err != nil {
		jobs[idx].Status = JobFailed
		jobs[idx].Error = err.Error()
		jobs[idx].Overall = overallUnknown
		if ctx.Err() != nil {
			batch.Status = BatchCancelled
		}
		persistSummary(mu, batchDir, batch, jobs, opts.OnUpdate)
		return
	}

	runAnalyze := spec.Mode == ModeAnalyze || spec.Analyze
	if runAnalyze && result.RunDir != "" {
		analyzeResult, aerr := analyze.Run(result.RunDir, analyze.Profile(spec.Profile))
		if aerr != nil {
			jobs[idx].Status = JobFailed
			jobs[idx].Error = aerr.Error()
			jobs[idx].Overall = overallUnknown
		} else {
			jobs[idx].Status = jobComplete
			if analyzeResult.Pass {
				jobs[idx].Overall = overallPass
			} else {
				jobs[idx].Overall = overallFail
			}
		}
	} else {
		jobs[idx].Status = jobComplete
		jobs[idx].Overall = overallUnknown
	}

	persistSummary(mu, batchDir, batch, jobs, opts.OnUpdate)
}

func persistSummary(mu *sync.Mutex, batchDir string, batch *SoakBatch, jobs []SoakJob, onUpdate func(Progress)) {
	summary := BatchSummary{Batch: *batch, Jobs: jobs}
	recomputeCounts(&summary)
	_ = writeSummaryJSON(batchDir, summary)
	emitProgress(onUpdate, *batch, jobs, batch.Status == BatchRunning)
}

func emitProgress(onUpdate func(Progress), batch SoakBatch, jobs []SoakJob, running bool) {
	if onUpdate == nil {
		return
	}
	copied := append([]SoakJob(nil), jobs...)
	onUpdate(Progress{Batch: batch, Jobs: copied, Running: running})
}

func writeBatchEnv(batchDir string, batch SoakBatch) error {
	content := fmt.Sprintf(
		"batch_id=%s\nmode=%s\nstatus=%s\nduration=%s\nwindow=%s\ninterval=%s\nconcurrency=%d\nprofile=%s\nskip_ineligible=%t\nstarted=%s\n",
		batch.ID,
		batch.Mode,
		batch.Status,
		batch.Duration,
		batch.Window,
		batch.Interval,
		batch.Concurrency,
		batch.Profile,
		batch.SkipIneligible,
		batch.StartedAt.Format(time.RFC3339),
	)
	return os.WriteFile(filepath.Join(batchDir, "batch.env"), []byte(content), 0o644)
}

func writeConfigSnapshot(ctx context.Context, client *manager.Client, serverID int, runDir string) error {
	cfg, err := client.ServerConfig(ctx, serverID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, "config_snapshot.json"), data, 0o644)
}

func appendRunEnvBatchFields(runDir, pairID, profile, batchID string) error {
	path := filepath.Join(runDir, "run.env")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	var b strings.Builder
	if pairID != "" {
		fmt.Fprintf(&b, "pair_id=%s\n", pairID)
	}
	fmt.Fprintf(&b, "profile=%s\nbatch_id=%s\n", profile, batchID)
	_, err = f.WriteString(b.String())
	return err
}

func lastBusDrops(runDir string) int64 {
	rows, err := metrics.ReadTSV(filepath.Join(runDir, "metrics.tsv"))
	if err != nil || len(rows) == 0 {
		return 0
	}
	return rows[len(rows)-1].BusDrops
}

func recomputeCounts(s *BatchSummary) {
	pass, fail, skipped, unknown := 0, 0, 0, 0
	for _, j := range s.Jobs {
		if j.Status == JobSkipped {
			skipped++
			continue
		}
		switch j.Overall {
		case overallPass:
			pass++
		case overallFail:
			fail++
		default:
			if j.Status == JobFailed {
				fail++
			} else {
				unknown++
			}
		}
	}
	s.Pass = pass
	s.Fail = fail
	s.Skipped = skipped
	s.Unknown = unknown
}

func writeSummaryJSON(batchDir string, summary BatchSummary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := filepath.Join(batchDir, "batch-summary.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(batchDir, "batch-summary.json"))
}

func writeSummaryMarkdown(batchDir string, summary BatchSummary) error {
	var b strings.Builder
	batch := summary.Batch
	fmt.Fprintf(&b, "# Batch summary: %s\n\n", batch.ID)
	fmt.Fprintf(&b, "**Status:** %s  \n", batch.Status)
	if batch.Mode != "" {
		fmt.Fprintf(&b, "**Mode:** %s  \n", batch.Mode)
	}
	if batch.Window != "" {
		fmt.Fprintf(&b, "**Window:** %s  \n", batch.Window)
	}
	fmt.Fprintf(&b, "**Profile:** %s  \n", batch.Profile)
	if batch.Duration != "" {
		fmt.Fprintf(&b, "**Duration:** %s  \n", batch.Duration)
	}
	fmt.Fprintf(&b, "**PASS:** %d  **FAIL:** %d  **SKIPPED:** %d\n\n", summary.Pass, summary.Fail, summary.Skipped)

	b.WriteString("| Server | Pair | Status | Overall | Samples | Bus drops | Notes |\n")
	b.WriteString("|--------|------|--------|---------|---------|-----------|-------|\n")
	for _, j := range summary.Jobs {
		notes := j.SkipReason
		if notes == "" {
			notes = j.Error
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %d | %s |\n",
			j.ServerID, j.PairID, j.Status, j.Overall, j.Samples, j.LastBusDrops, notes)
	}

	return os.WriteFile(filepath.Join(batchDir, "batch-summary.md"), []byte(b.String()), 0o644)
}

// LoadSummary reads batch-summary.json from a batch directory.
func LoadSummary(batchDir string) (BatchSummary, error) {
	data, err := os.ReadFile(filepath.Join(batchDir, "batch-summary.json"))
	if err != nil {
		return BatchSummary{}, err
	}
	var summary BatchSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return BatchSummary{}, err
	}
	return summary, nil
}

// ListRecentBatches scans artifactsDir for batch-* directories.
func ListRecentBatches(artifactsDir string, limit int) ([]BatchSummary, error) {
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var summaries []BatchSummary
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "batch-") {
			continue
		}
		summary, err := LoadSummary(filepath.Join(artifactsDir, e.Name()))
		if err != nil {
			continue
		}
		summaries = append(summaries, summary)
	}

	sortSummariesByStarted(summaries)
	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

func sortSummariesByStarted(summaries []BatchSummary) {
	for i := 0; i < len(summaries); i++ {
		for j := i + 1; j < len(summaries); j++ {
			if summaries[j].Batch.StartedAt.After(summaries[i].Batch.StartedAt) {
				summaries[i], summaries[j] = summaries[j], summaries[i]
			}
		}
	}
}

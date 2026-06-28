package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/analyze"
	"github.com/dlisovsky/go-trader-qa/internal/batch"
	"github.com/dlisovsky/go-trader-qa/internal/fleet"
	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

var allowedArtifacts = map[string]bool{
	"metrics.tsv":          true,
	"soak.log":             true,
	"issues.log":           true,
	"run.env":              true,
	"qa-report.md":         true,
	"config_snapshot.json": true,
}

type fleetCache struct {
	mu       sync.RWMutex
	syncedAt time.Time
	rows     []fleet.FleetRow
}

type activeBatch struct {
	progress batch.Progress
	cancel   context.CancelFunc
	done     chan struct{}
}

type batchRegistry struct {
	mu      sync.RWMutex
	active  map[string]*activeBatch
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": version,
	})
}

func (s *Server) handleFleetGet(w http.ResponseWriter, _ *http.Request) {
	s.fleet.mu.RLock()
	defer s.fleet.mu.RUnlock()
	if s.fleet.syncedAt.IsZero() {
		writeError(w, http.StatusServiceUnavailable, "fleet not synced")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"synced_at": s.fleet.syncedAt.UTC().Format(time.RFC3339),
		"rows":      s.fleet.rows,
	})
}

func (s *Server) handleFleetSync(w http.ResponseWriter, r *http.Request) {
	resp, err := s.client.FleetSubs(r.Context(), s.cfg.ManagerAccountID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	rows := fleet.BuildRows(resp, resp.Categories)
	now := time.Now().UTC()

	s.fleet.mu.Lock()
	s.fleet.rows = rows
	s.fleet.syncedAt = now
	s.fleet.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"synced_at": now.Format(time.RFC3339),
		"rows":      rows,
	})
}

type createBatchRequest struct {
	Mode           string `json:"mode"`
	Window         string `json:"window"`
	ServerIDs      []int  `json:"server_ids"`
	Duration       string `json:"duration"`
	Interval       string `json:"interval"`
	Profile        string `json:"profile"`
	Concurrency    int    `json:"concurrency"`
	SkipIneligible *bool  `json:"skip_ineligible"`
	Analyze        *bool  `json:"analyze"`
}

func (s *Server) handleBatchCreate(w http.ResponseWriter, r *http.Request) {
	var req createBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.ServerIDs) == 0 {
		writeError(w, http.StatusBadRequest, "server_ids is required")
		return
	}

	mode := strings.TrimSpace(strings.ToLower(req.Mode))
	if mode == "" {
		mode = batch.ModeSoak
	}

	var (
		dur    time.Duration
		window metrics.WindowSpec
		err    error
	)

	switch mode {
	case batch.ModeAnalyze:
		window, err = batch.ParseBatchWindow(req.Window)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case batch.ModeSoak:
		dur, err = metrics.ParseDuration(strings.TrimSpace(req.Duration))
		if err != nil || dur <= 0 {
			writeError(w, http.StatusBadRequest, "duration is required for soak mode (e.g. 30m)")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "mode must be soak or analyze")
		return
	}

	intervalStr := strings.TrimSpace(req.Interval)
	if intervalStr == "" {
		intervalStr = "5m"
	}
	iv, err := metrics.ParseDuration(intervalStr)
	if err != nil || iv <= 0 {
		writeError(w, http.StatusBadRequest, "invalid interval")
		return
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = s.cfg.MaxConcurrency
	}
	if concurrency > s.cfg.MaxConcurrencyHard {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("concurrency exceeds hard cap (%d)", s.cfg.MaxConcurrencyHard))
		return
	}

	profile := strings.TrimSpace(req.Profile)
	if profile == "" {
		profile = string(analyze.ProfileWSSOnly)
	}

	skipIneligible := true
	if req.SkipIneligible != nil {
		skipIneligible = *req.SkipIneligible
	}

	doAnalyze := true
	if req.Analyze != nil {
		doAnalyze = *req.Analyze
	}

	batchID := "batch-" + time.Now().UTC().Format("20060102T150405Z")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	initialBatch := batch.SoakBatch{
		ID:             batchID,
		Mode:           mode,
		ServerIDs:      req.ServerIDs,
		Duration:       dur.String(),
		Interval:       iv.String(),
		Concurrency:    concurrency,
		Profile:        profile,
		SkipIneligible: skipIneligible,
		Status:         batch.BatchRunning,
		StartedAt:      time.Now().UTC(),
	}
	if mode == batch.ModeAnalyze {
		initialBatch.Duration = ""
		initialBatch.Window = window.String()
	}

	s.batch.mu.Lock()
	if s.batch.active == nil {
		s.batch.active = make(map[string]*activeBatch)
	}
	s.batch.active[batchID] = &activeBatch{
		progress: batch.Progress{
			Batch:   initialBatch,
			Running: true,
		},
		cancel: cancel,
		done:   done,
	}
	s.batch.mu.Unlock()

	go func() {
		defer close(done)
		result, runErr := batch.Run(ctx, s.client, batch.RunOptions{
			Spec: batch.BatchSpec{
				BatchID:        batchID,
				Mode:           mode,
				Window:         window,
				ServerIDs:      req.ServerIDs,
				Duration:       dur,
				Interval:       iv,
				Concurrency:    concurrency,
				Profile:        profile,
				SkipIneligible: skipIneligible,
				Analyze:        doAnalyze,
				ArtifactsDir:   s.cfg.QAArtifactsDir,
				AccountID:      s.cfg.ManagerAccountID,
			},
			OnUpdate: func(p batch.Progress) {
				s.batch.mu.Lock()
				if ab, ok := s.batch.active[batchID]; ok {
					ab.progress = p
				}
				s.batch.mu.Unlock()
			},
		})
		s.batch.mu.Lock()
		if ab, ok := s.batch.active[batchID]; ok {
			if runErr != nil {
				ab.progress.Batch.Status = batch.BatchFailed
				ab.progress.Running = false
			} else {
				ab.progress = batch.Progress{
					Batch:   result.Batch,
					Jobs:    result.Jobs,
					Running: false,
				}
			}
		}
		s.batch.mu.Unlock()
	}()

	go func() {
		<-done
		s.batch.mu.Lock()
		delete(s.batch.active, batchID)
		s.batch.mu.Unlock()
	}()

	writeJSON(w, http.StatusCreated, map[string]any{
		"batch_id": batchID,
		"status":   batch.BatchRunning,
	})
}

func (s *Server) handleBatchList(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}

	summaries, err := batch.ListRecentBatches(s.cfg.QAArtifactsDir, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, batchListItem(summary, s.isBatchRunning(summary.Batch.ID)))
	}

	writeJSON(w, http.StatusOK, map[string]any{"batches": items})
}

func batchListItem(summary batch.BatchSummary, running bool) map[string]any {
	b := summary.Batch
	item := map[string]any{
		"id":             b.ID,
		"mode":           b.Mode,
		"window":         b.Window,
		"status":         b.Status,
		"running":        running,
		"started_at":     b.StartedAt.UTC().Format(time.RFC3339),
		"duration":       b.Duration,
		"profile":        b.Profile,
		"concurrency":    b.Concurrency,
		"job_count":      len(summary.Jobs),
		"pass_count":     summary.Pass,
		"fail_count":     summary.Fail,
		"skipped_count":  summary.Skipped,
		"server_ids":     b.ServerIDs,
	}
	if b.CompletedAt != nil {
		item["completed_at"] = b.CompletedAt.UTC().Format(time.RFC3339)
	}
	return item
}

func (s *Server) handleBatchGet(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("batch_id")
	progress, ok := s.loadBatchProgress(batchID)
	if !ok {
		writeError(w, http.StatusNotFound, "batch not found")
		return
	}

	summary := batch.BatchSummary{Batch: progress.Batch, Jobs: progress.Jobs}
	recomputeFromProgress(&summary)

	writeJSON(w, http.StatusOK, map[string]any{
		"batch":          progress.Batch,
		"jobs":           progress.Jobs,
		"running":        progress.Running,
		"pass_count":     summary.Pass,
		"fail_count":     summary.Fail,
		"skipped_count":  summary.Skipped,
	})
}

const batchDeleteWaitTimeout = 2 * time.Minute

var (
	errBatchRunningNoWait = errors.New("batch is running; delete individually or cancel first")
	errBatchDeleteTimeout = errors.New("batch did not stop in time")
)

type bulkDeleteRequest struct {
	BatchIDs []string `json:"batch_ids"`
}

type bulkDeleteFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func (s *Server) waitBatchDone(batchID string, timeout time.Duration) error {
	s.batch.mu.RLock()
	ab, ok := s.batch.active[batchID]
	s.batch.mu.RUnlock()
	if !ok {
		return nil
	}
	select {
	case <-ab.done:
		return nil
	case <-time.After(timeout):
		return errBatchDeleteTimeout
	}
}

func (s *Server) deleteBatch(batchID string, waitForCancel bool) error {
	if err := batch.ValidateBatchID(batchID); err != nil {
		return err
	}

	s.batch.mu.RLock()
	ab, active := s.batch.active[batchID]
	s.batch.mu.RUnlock()

	if active {
		if !waitForCancel {
			return errBatchRunningNoWait
		}
		ab.cancel()
		if err := s.waitBatchDone(batchID, batchDeleteWaitTimeout); err != nil {
			return err
		}
	}

	return batch.DeleteBatch(s.cfg.QAArtifactsDir, batchID)
}

func (s *Server) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("batch_id")
	err := s.deleteBatch(batchID, true)
	if err != nil {
		switch {
		case errors.Is(err, batch.ErrInvalidBatchID):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, batch.ErrBatchNotFound):
			writeError(w, http.StatusNotFound, "batch not found")
		case errors.Is(err, errBatchDeleteTimeout):
			writeError(w, http.StatusRequestTimeout, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": batchID})
}

func (s *Server) handleBatchBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req bulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.BatchIDs) == 0 {
		writeError(w, http.StatusBadRequest, "batch_ids is required")
		return
	}
	if len(req.BatchIDs) > 50 {
		writeError(w, http.StatusBadRequest, "batch_ids exceeds limit of 50")
		return
	}

	deleted := make([]string, 0, len(req.BatchIDs))
	failed := make([]bulkDeleteFailure, 0)

	for _, batchID := range req.BatchIDs {
		batchID = strings.TrimSpace(batchID)
		if batchID == "" {
			continue
		}
		if err := s.deleteBatch(batchID, false); err != nil {
			msg := err.Error()
			if errors.Is(err, errBatchRunningNoWait) {
				msg = errBatchRunningNoWait.Error()
			}
			failed = append(failed, bulkDeleteFailure{ID: batchID, Error: msg})
			continue
		}
		deleted = append(deleted, batchID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func (s *Server) handleBatchCancel(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("batch_id")

	s.batch.mu.Lock()
	ab, ok := s.batch.active[batchID]
	if ok {
		ab.cancel()
	}
	s.batch.mu.Unlock()

	if !ok {
		writeError(w, http.StatusNotFound, "batch not found or not running")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelling"})
}

func (s *Server) handleBatchSummary(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("batch_id")
	path := filepath.Join(s.cfg.QAArtifactsDir, batchID, "batch-summary.json")
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "batch summary not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleJobGet(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("batch_id")
	serverID, err := strconv.Atoi(r.PathValue("server_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}

	progress, ok := s.loadBatchProgress(batchID)
	if !ok {
		writeError(w, http.StatusNotFound, "batch not found")
		return
	}

	for _, job := range progress.Jobs {
		if job.ServerID == serverID {
			writeJSON(w, http.StatusOK, job)
			return
		}
	}
	writeError(w, http.StatusNotFound, "job not found")
}

func (s *Server) handleJobReport(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("batch_id")
	serverID, err := strconv.Atoi(r.PathValue("server_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}

	progress, ok := s.loadBatchProgress(batchID)
	if !ok {
		writeError(w, http.StatusNotFound, "batch not found")
		return
	}

	var job *batch.SoakJob
	for i := range progress.Jobs {
		if progress.Jobs[i].ServerID == serverID {
			job = &progress.Jobs[i]
			break
		}
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status == batch.JobRunning || job.Status == batch.JobQueued {
		writeError(w, http.StatusConflict, "job still running")
		return
	}
	if job.RunDir == "" {
		writeError(w, http.StatusNotFound, "report not available")
		return
	}

	reportPath := filepath.Join(job.RunDir, "qa-report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "qa-report.md not found")
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleJobArtifact(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || name == ".." {
		writeError(w, http.StatusBadRequest, "invalid artifact name")
		return
	}
	if !allowedArtifacts[name] {
		writeError(w, http.StatusBadRequest, "artifact not allowed")
		return
	}

	batchID := r.PathValue("batch_id")
	serverID, err := strconv.Atoi(r.PathValue("server_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}

	progress, ok := s.loadBatchProgress(batchID)
	if !ok {
		writeError(w, http.StatusNotFound, "batch not found")
		return
	}

	var runDir string
	for _, job := range progress.Jobs {
		if job.ServerID == serverID {
			runDir = job.RunDir
			break
		}
	}
	if runDir == "" {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	artifactPath := filepath.Join(runDir, name)
	cleanRun := filepath.Clean(runDir)
	cleanArtifact := filepath.Clean(artifactPath)
	if !strings.HasPrefix(cleanArtifact, cleanRun+string(filepath.Separator)) && cleanArtifact != cleanRun {
		writeError(w, http.StatusBadRequest, "invalid artifact path")
		return
	}

	f, err := os.Open(cleanArtifact)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

type analyzeRequest struct {
	RunDir  string `json:"run_dir"`
	Profile string `json:"profile"`
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	runDir := strings.TrimSpace(req.RunDir)
	if runDir == "" {
		writeError(w, http.StatusBadRequest, "run_dir is required")
		return
	}

	profile := analyze.Profile(strings.TrimSpace(req.Profile))
	if profile == "" {
		profile = analyze.ProfileWSSOnly
	}

	result, err := analyze.Run(runDir, profile)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := batch.UpdateJobOverallFromReanalyze(runDir, result.Pass); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	overall := "FAIL"
	if result.Pass {
		overall = "PASS"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"overall":     overall,
		"report_path": result.ReportPath,
	})
}

func (s *Server) loadBatchProgress(batchID string) (batch.Progress, bool) {
	s.batch.mu.RLock()
	ab, live := s.batch.active[batchID]
	s.batch.mu.RUnlock()
	if live {
		return ab.progress, true
	}

	summary, err := batch.LoadSummary(filepath.Join(s.cfg.QAArtifactsDir, batchID))
	if err != nil {
		return batch.Progress{}, false
	}
	return batch.Progress{
		Batch:   summary.Batch,
		Jobs:    summary.Jobs,
		Running: summary.Batch.Status == batch.BatchRunning,
	}, true
}

func (s *Server) isBatchRunning(batchID string) bool {
	s.batch.mu.RLock()
	_, ok := s.batch.active[batchID]
	s.batch.mu.RUnlock()
	return ok
}

func recomputeFromProgress(summary *batch.BatchSummary) {
	pass, fail, skipped := 0, 0, 0
	for _, j := range summary.Jobs {
		if j.Status == batch.JobSkipped {
			skipped++
			continue
		}
		switch j.Overall {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		}
	}
	summary.Pass = pass
	summary.Fail = fail
	summary.Skipped = skipped
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

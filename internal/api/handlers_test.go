package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	artifacts := t.TempDir()
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		ManagerAPIBaseURL:  "https://example.com/api",
		ManagerAccountID:     1,
		ManagerBearerToken:   "test-token",
		QAArtifactsDir:       artifacts,
		ListenAddr:           "127.0.0.1:0",
		MaxConcurrency:       2,
		MaxConcurrencyHard:   3,
	}

	client, err := manager.NewClient(cfg.ManagerAPIBaseURL, cfg.ManagerBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(cfg, client, webDir), artifacts
}

func TestHandleHealth(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v", body["ok"])
	}
	if body["version"] != version {
		t.Fatalf("version = %v, want %s", body["version"], version)
	}
}

func TestHandleFleetNotSynced(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHandleMetricsGuide(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics-guide", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	gates, ok := out["gates"].([]any)
	if !ok || len(gates) != 16 {
		t.Fatalf("gates = %v", out["gates"])
	}
}

func TestHandleBatchCreateAnalyzeLifecycle(t *testing.T) {
	s, _ := testServer(t)
	payload := map[string]any{
		"server_ids": []int{11},
		"mode":       "analyze",
		"window":     "lifecycle",
		"profile":    "lifecycle",
		"interval":   "5m",
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["batch_id"] == "" {
		t.Fatalf("missing batch_id: %v", out)
	}
}

func TestHandleBatchCreateConcurrencyHardCap(t *testing.T) {
	s, _ := testServer(t)
	payload := map[string]any{
		"server_ids":  []int{11},
		"duration":    "15s",
		"concurrency": 99,
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func writeTestBatchDir(t *testing.T, artifacts, batchID string) {
	t.Helper()
	batchDir := filepath.Join(artifacts, batchID)
	if err := os.MkdirAll(batchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	summary := map[string]any{
		"batch": map[string]any{
			"id":     batchID,
			"status": "complete",
		},
		"jobs": []any{},
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filepath.Join(batchDir, "batch-summary.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandleBatchDeleteCompleted(t *testing.T) {
	s, artifacts := testServer(t)
	batchID := "batch-delete-complete"
	writeTestBatchDir(t, artifacts, batchID)

	req := httptest.NewRequest(http.MethodDelete, "/api/batches/"+batchID, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(artifacts, batchID)); !os.IsNotExist(err) {
		t.Fatal("batch dir should be removed")
	}
}

func TestHandleBatchDeleteNotFound(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/batches/batch-missing", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleBatchDeleteActiveCancelWait(t *testing.T) {
	s, artifacts := testServer(t)
	batchID := "batch-delete-active"
	writeTestBatchDir(t, artifacts, batchID)

	done := make(chan struct{})
	_, cancel := context.WithCancel(context.Background())

	s.batch.mu.Lock()
	if s.batch.active == nil {
		s.batch.active = make(map[string]*activeBatch)
	}
	s.batch.active[batchID] = &activeBatch{
		cancel: cancel,
		done:   done,
	}
	s.batch.mu.Unlock()

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(done)
		s.batch.mu.Lock()
		delete(s.batch.active, batchID)
		s.batch.mu.Unlock()
	}()

	req := httptest.NewRequest(http.MethodDelete, "/api/batches/"+batchID, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(artifacts, batchID)); !os.IsNotExist(err) {
		t.Fatal("batch dir should be removed after cancel wait")
	}
}

func TestHandleBatchBulkDeletePartial(t *testing.T) {
	s, artifacts := testServer(t)
	okID := "batch-bulk-ok"
	writeTestBatchDir(t, artifacts, okID)

	runID := "batch-bulk-running"
	writeTestBatchDir(t, artifacts, runID)
	done := make(chan struct{})
	s.batch.mu.Lock()
	if s.batch.active == nil {
		s.batch.active = make(map[string]*activeBatch)
	}
	s.batch.active[runID] = &activeBatch{
		cancel: func() {},
		done:   done,
	}
	s.batch.mu.Unlock()

	payload := map[string]any{"batch_ids": []string{okID, runID, "batch-bulk-missing"}}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/batches/bulk-delete", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	deleted := out["deleted"].([]any)
	if len(deleted) != 1 || deleted[0] != okID {
		t.Fatalf("deleted = %v, want [%s]", deleted, okID)
	}
	failed := out["failed"].([]any)
	if len(failed) != 2 {
		t.Fatalf("failed count = %d, want 2", len(failed))
	}
	if _, err := os.Stat(filepath.Join(artifacts, okID)); !os.IsNotExist(err) {
		t.Fatal("ok batch should be deleted")
	}
	if _, err := os.Stat(filepath.Join(artifacts, runID)); err != nil {
		t.Fatal("running batch dir should remain")
	}
}

func TestHandleJobArtifactRejectsTraversal(t *testing.T) {
	s, artifacts := testServer(t)

	batchID := "batch-test"
	batchDir := filepath.Join(artifacts, batchID)
	jobsDir := filepath.Join(batchDir, "jobs")
	runDir := filepath.Join(jobsDir, "2026-06-28T13-43-53Z-11")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "metrics.tsv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	summary := map[string]any{
		"batch": map[string]any{
			"id":     batchID,
			"status": "complete",
		},
		"jobs": []map[string]any{
			{
				"batch_id":  batchID,
				"server_id": 11,
				"run_dir":   runDir,
				"status":    "complete",
			},
		},
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filepath.Join(batchDir, "batch-summary.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/batches/"+batchID+"/jobs/11/artifacts/..%2F..%2F..%2Fetc%2Fpasswd", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

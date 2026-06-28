package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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

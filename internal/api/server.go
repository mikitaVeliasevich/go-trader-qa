package api

import (
	"net/http"

	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
)

const version = "0.2.0"

// Server is the gtqa-server HTTP control plane.
type Server struct {
	cfg    config.Config
	client *manager.Client
	webDir string
	mux    *http.ServeMux

	fleet fleetCache
	batch batchRegistry
}

// NewServer builds an HTTP server with API routes and static web assets.
func NewServer(cfg config.Config, client *manager.Client, webDir string) *Server {
	s := &Server{
		cfg:    cfg,
		client: client,
		webDir: webDir,
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/fleet", s.handleFleetGet)
	s.mux.HandleFunc("POST /api/fleet/sync", s.handleFleetSync)

	s.mux.HandleFunc("POST /api/batches", s.handleBatchCreate)
	s.mux.HandleFunc("GET /api/batches", s.handleBatchList)
	s.mux.HandleFunc("GET /api/batches/{batch_id}", s.handleBatchGet)
	s.mux.HandleFunc("DELETE /api/batches/{batch_id}", s.handleBatchDelete)
	s.mux.HandleFunc("POST /api/batches/bulk-delete", s.handleBatchBulkDelete)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/cancel", s.handleBatchCancel)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/summary", s.handleBatchSummary)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/jobs/{server_id}", s.handleJobGet)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/jobs/{server_id}/report", s.handleJobReport)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/jobs/{server_id}/artifacts/{name}", s.handleJobArtifact)

	s.mux.HandleFunc("POST /api/analyze", s.handleAnalyze)

	s.registerStatic()
}

func (s *Server) registerStatic() {
	fs := http.FileServer(http.Dir(s.webDir))
	s.mux.Handle("GET /static/{path...}", fs)
	s.mux.Handle("GET /{$}", fs)
}

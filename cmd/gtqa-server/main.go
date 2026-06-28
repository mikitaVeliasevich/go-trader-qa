package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dlisovsky/go-trader-qa/internal/api"
	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client, err := manager.NewClient(cfg.ManagerAPIBaseURL, cfg.ManagerBearerToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	webDir := findWebDir()
	srv := api.NewServer(cfg, client, webDir)

	log.Printf("gtqa-server listening on %s (web=%s)", cfg.ListenAddr, webDir)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findWebDir() string {
	candidates := []string{"web", filepath.Join("..", "web")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web"))
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
			return dir
		}
	}
	return "web"
}

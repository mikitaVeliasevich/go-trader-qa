package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultArtifactsDir        = "./reports"
	defaultListenAddr          = "127.0.0.1:8080"
	defaultMaxConcurrency      = 2
	defaultMaxConcurrencyHard  = 3
)

// Config holds Manager API credentials and QA paths.
type Config struct {
	ManagerAPIBaseURL  string
	ManagerAccountID   int
	ManagerBearerToken string
	QAArtifactsDir     string
	ListenAddr         string
	MaxConcurrency     int
	MaxConcurrencyHard int
}

// Load reads optional .env then required MANAGER_* environment variables.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		ManagerAPIBaseURL:  strings.TrimSpace(os.Getenv("MANAGER_API_BASE_URL")),
		ManagerBearerToken: strings.TrimSpace(os.Getenv("MANAGER_BEARER_TOKEN")),
		QAArtifactsDir:     strings.TrimSpace(os.Getenv("QA_ARTIFACTS_DIR")),
		ListenAddr:         strings.TrimSpace(os.Getenv("GTQA_LISTEN_ADDR")),
	}

	if cfg.QAArtifactsDir == "" {
		cfg.QAArtifactsDir = defaultArtifactsDir
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}

	var err error
	cfg.MaxConcurrency, err = envInt("GTQA_MAX_CONCURRENCY", defaultMaxConcurrency)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxConcurrencyHard, err = envInt("GTQA_MAX_CONCURRENCY_HARD", defaultMaxConcurrencyHard)
	if err != nil {
		return Config{}, err
	}

	accountIDStr := strings.TrimSpace(os.Getenv("MANAGER_ACCOUNT_ID"))
	if accountIDStr == "" {
		return Config{}, fmt.Errorf("MANAGER_ACCOUNT_ID is required")
	}
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		return Config{}, fmt.Errorf("MANAGER_ACCOUNT_ID must be an integer: %w", err)
	}
	cfg.ManagerAccountID = accountID

	if cfg.ManagerAPIBaseURL == "" {
		return Config{}, fmt.Errorf("MANAGER_API_BASE_URL is required")
	}
	if cfg.ManagerBearerToken == "" {
		return Config{}, fmt.Errorf("MANAGER_BEARER_TOKEN is required")
	}

	return cfg, nil
}

func envInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return v, nil
}

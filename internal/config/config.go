package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const defaultArtifactsDir = "./reports"

// Config holds Manager API credentials and QA paths.
type Config struct {
	ManagerAPIBaseURL string
	ManagerAccountID  int
	ManagerBearerToken string
	QAArtifactsDir    string
}

// Load reads optional .env then required MANAGER_* environment variables.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		ManagerAPIBaseURL:  strings.TrimSpace(os.Getenv("MANAGER_API_BASE_URL")),
		ManagerBearerToken: strings.TrimSpace(os.Getenv("MANAGER_BEARER_TOKEN")),
		QAArtifactsDir:     strings.TrimSpace(os.Getenv("QA_ARTIFACTS_DIR")),
	}

	if cfg.QAArtifactsDir == "" {
		cfg.QAArtifactsDir = defaultArtifactsDir
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

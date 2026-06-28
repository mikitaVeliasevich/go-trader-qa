package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/spf13/cobra"
)

func newSmokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "smoke [server_id]",
		Short: "Smoke-test Manager provision endpoints",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("server_id must be an integer: %w", err)
			}
			return runSmoke(cmd.Context(), serverID)
		},
	}
}

type smokeCheck struct {
	displayPath string
	apiPath     string
}

func runSmoke(ctx context.Context, serverID int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	client, err := manager.NewClient(cfg.ManagerAPIBaseURL, cfg.ManagerBearerToken)
	if err != nil {
		return err
	}

	checks := []smokeCheck{
		{displayPath: "status", apiPath: fmt.Sprintf("/provision/servers/%d/status", serverID)},
		{displayPath: "config", apiPath: fmt.Sprintf("/provision/servers/%d/config", serverID)},
		{displayPath: "logs?tail=3", apiPath: fmt.Sprintf("/provision/servers/%d/logs?tail=3", serverID)},
		{displayPath: "debug/vars", apiPath: fmt.Sprintf("/provision/servers/%d/debug/vars", serverID)},
	}

	fmt.Printf("Manager smoke: server_id=%d\n", serverID)

	var failed bool
	for _, check := range checks {
		status, body, err := client.Get(ctx, check.apiPath)
		fmt.Printf("=== GET /%s HTTP %d ===\n", check.displayPath, status)
		if err != nil {
			printSmokeBody(body)
			fmt.Println()
			fmt.Fprintf(os.Stderr, "GET /%s failed: %v\n", check.displayPath, err)
			failed = true
			continue
		}
		printSmokeBody(body)
		fmt.Println()
	}

	fmt.Println("Expected: status/config/logs=200; debug/vars=200 with bus_drops, ws_messages_received, ...")
	fmt.Println("If debug/vars returns 404 from bot, redeploy go-trader image with GET /debug/vars on :3228.")

	if failed {
		return fmt.Errorf("one or more smoke checks failed")
	}
	return nil
}

func printSmokeBody(body []byte) {
	var v any
	if err := json.Unmarshal(body, &v); err == nil {
		out, err := json.MarshalIndent(v, "  ", "  ")
		if err == nil {
			fmt.Print("  ")
			fmt.Println(string(out))
			return
		}
	}

	const max = 400
	s := string(body)
	if len(s) > max {
		s = s[:max]
	}
	fmt.Print("  ")
	fmt.Println(s)
}

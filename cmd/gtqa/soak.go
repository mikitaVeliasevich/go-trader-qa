package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/dlisovsky/go-trader-qa/internal/metrics"
	"github.com/dlisovsky/go-trader-qa/internal/sampler"
	"github.com/spf13/cobra"
)

func newSoakCmd() *cobra.Command {
	var (
		serverID int
		duration string
		interval string
	)

	cmd := &cobra.Command{
		Use:   "soak",
		Short: "Remote observe-only soak via Manager",
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Sample debug/vars and logs for a duration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoak(serverID, duration, interval)
		},
	}
	runCmd.Flags().IntVar(&serverID, "server-id", 0, "provision server id (required)")
	runCmd.Flags().StringVar(&duration, "duration", "30m", "soak duration (e.g. 30m, 4h)")
	runCmd.Flags().StringVar(&interval, "interval", "5m", "sample interval (e.g. 5m)")
	_ = runCmd.MarkFlagRequired("server-id")

	cmd.AddCommand(runCmd)
	cmd.AddCommand(newSoakBatchCmd())
	return cmd
}

func runSoak(serverID int, durationStr, intervalStr string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dur, err := metrics.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("parse --duration: %w", err)
	}
	iv, err := metrics.ParseDuration(intervalStr)
	if err != nil {
		return fmt.Errorf("parse --interval: %w", err)
	}

	client, err := manager.NewClient(cfg.ManagerAPIBaseURL, cfg.ManagerBearerToken)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := sampler.RemoteRun(ctx, client, sampler.Options{
		ServerID:     serverID,
		Duration:     dur,
		Interval:     iv,
		ArtifactsDir: cfg.QAArtifactsDir,
		AccountID:    cfg.ManagerAccountID,
	})
	if err != nil {
		if result.RunDir != "" {
			fmt.Fprintf(os.Stderr, "run dir (partial): %s\n", result.RunDir)
		}
		return err
	}

	if result.Samples < 1 {
		return fmt.Errorf("no samples collected in %s", result.RunDir)
	}

	fmt.Println(result.RunDir)
	return nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/analyze"
	"github.com/dlisovsky/go-trader-qa/internal/batch"
	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/dlisovsky/go-trader-qa/internal/metrics"
	"github.com/spf13/cobra"
)

func newSoakBatchCmd() *cobra.Command {
	var (
		serverIDs      string
		mode           string
		window         string
		duration       string
		interval       string
		concurrency    int
		profile        string
		skipIneligible bool
		doAnalyze      bool
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run parallel observe-only soaks or retrospective analyze across server IDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoakBatch(serverIDs, mode, window, duration, interval, concurrency, profile, skipIneligible, doAnalyze)
		},
	}

	cmd.Flags().StringVar(&serverIDs, "server-ids", "", "comma-separated server IDs (required)")
	cmd.Flags().StringVar(&mode, "mode", batch.ModeSoak, "batch mode: soak or analyze")
	cmd.Flags().StringVar(&window, "window", "", "analyze window: 5m, 30m, 1h, 4h, 8h, lifecycle (required for analyze mode)")
	cmd.Flags().StringVar(&duration, "duration", "30m", "soak duration per job (e.g. 30m, 4h)")
	cmd.Flags().StringVar(&interval, "interval", "5m", "sample interval (e.g. 5m)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "max parallel jobs")
	cmd.Flags().StringVar(&profile, "profile", string(analyze.ProfileWSSOnly), "analyzer profile: wss-only, lifecycle, lifecycle-strict, or tpsl-health")
	cmd.Flags().BoolVar(&skipIneligible, "skip-ineligible", true, "skip servers not QA eligible")
	cmd.Flags().BoolVar(&doAnalyze, "analyze", true, "run analyzer after each soak job")
	_ = cmd.MarkFlagRequired("server-ids")

	return cmd
}

func runSoakBatch(serverIDsStr, mode, windowStr, durationStr, intervalStr string, concurrency int, profile string, skipIneligible, doAnalyze bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ids, err := parseServerIDs(serverIDsStr)
	if err != nil {
		return err
	}

	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = batch.ModeSoak
	}

	var (
		dur          time.Duration
		window       metrics.WindowSpec
	)
	switch mode {
	case batch.ModeAnalyze:
		window, err = batch.ParseBatchWindow(windowStr)
		if err != nil {
			return err
		}
	case batch.ModeSoak:
		dur, err = metrics.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("parse --duration: %w", err)
		}
	default:
		return fmt.Errorf("mode must be soak or analyze")
	}
	iv, err := metrics.ParseDuration(intervalStr)
	if err != nil {
		return fmt.Errorf("parse --interval: %w", err)
	}

	if concurrency <= 0 {
		concurrency = cfg.MaxConcurrency
	}
	if concurrency > cfg.MaxConcurrencyHard {
		return fmt.Errorf("concurrency %d exceeds hard cap %d", concurrency, cfg.MaxConcurrencyHard)
	}

	client, err := manager.NewClient(cfg.ManagerAPIBaseURL, cfg.ManagerBearerToken)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := batch.Run(ctx, client, batch.RunOptions{
		Spec: batch.BatchSpec{
			Mode:           mode,
			Window:         window,
			ServerIDs:      ids,
			Duration:       dur,
			Interval:       iv,
			Concurrency:    concurrency,
			Profile:        profile,
			SkipIneligible: skipIneligible,
			Analyze:        doAnalyze,
			ArtifactsDir:   cfg.QAArtifactsDir,
			AccountID:      cfg.ManagerAccountID,
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("Batch %s complete: PASS=%d FAIL=%d SKIPPED=%d\n",
		result.Batch.ID, result.Summary.Pass, result.Summary.Fail, result.Summary.Skipped)
	fmt.Println(result.Batch.Dir)
	return nil
}

func parseServerIDs(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	var ids []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid server id %q", p)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("server_ids is required")
	}
	return ids, nil
}

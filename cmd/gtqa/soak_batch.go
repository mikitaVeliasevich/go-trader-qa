package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

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
		duration       string
		interval       string
		concurrency    int
		profile        string
		skipIneligible bool
		doAnalyze      bool
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run parallel observe-only soaks across server IDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoakBatch(serverIDs, duration, interval, concurrency, profile, skipIneligible, doAnalyze)
		},
	}

	cmd.Flags().StringVar(&serverIDs, "server-ids", "", "comma-separated server IDs (required)")
	cmd.Flags().StringVar(&duration, "duration", "30m", "soak duration per job (e.g. 30m, 4h)")
	cmd.Flags().StringVar(&interval, "interval", "5m", "sample interval (e.g. 5m)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "max parallel jobs")
	cmd.Flags().StringVar(&profile, "profile", string(analyze.ProfileWSSOnly), "analyzer profile: wss-only or lifecycle")
	cmd.Flags().BoolVar(&skipIneligible, "skip-ineligible", true, "skip servers not QA eligible")
	cmd.Flags().BoolVar(&doAnalyze, "analyze", true, "run analyzer after each job")
	_ = cmd.MarkFlagRequired("server-ids")

	return cmd
}

func runSoakBatch(serverIDsStr, durationStr, intervalStr string, concurrency int, profile string, skipIneligible, doAnalyze bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ids, err := parseServerIDs(serverIDsStr)
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

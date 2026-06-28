package main

import (
	"fmt"
	"os"

	"github.com/dlisovsky/go-trader-qa/internal/analyze"
	"github.com/spf13/cobra"
)

func newAnalyzeCmd() *cobra.Command {
	var (
		runDir  string
		profile string
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Evaluate soak gates and write qa-report.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := analyze.Run(runDir, analyze.Profile(profile))
			if err != nil {
				return err
			}

			fmt.Printf("Wrote %s\n", result.ReportPath)
			for _, g := range result.Gates {
				status := "FAIL"
				if g.Pass {
					status = "PASS"
				}
				fmt.Printf("  %s: %s — %s\n", g.ID, status, g.Detail)
			}
			fmt.Printf("WS messages overall: %d (delta +%d)\n", result.Deltas.WSMessagesOverall, result.Deltas.WSMessagesDelta)
			if result.Pass {
				fmt.Println("OVERALL: PASS")
				return nil
			}
			fmt.Println("OVERALL: FAIL")
			os.Exit(1)
			return nil
		},
	}

	cmd.Flags().StringVar(&runDir, "run-dir", "", "soak run directory (required)")
	cmd.Flags().StringVar(&profile, "profile", "lifecycle", "gate profile: lifecycle or wss-only")
	_ = cmd.MarkFlagRequired("run-dir")

	return cmd
}

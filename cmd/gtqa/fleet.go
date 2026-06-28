package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/dlisovsky/go-trader-qa/internal/config"
	"github.com/dlisovsky/go-trader-qa/internal/fleet"
	"github.com/dlisovsky/go-trader-qa/internal/manager"
	"github.com/spf13/cobra"
)

func newFleetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Fleet inventory from Manager /subs",
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch subaccounts and print eligibility table",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFleetSync(cmd.Context(), jsonOut)
		},
	}
	syncCmd.Flags().BoolVar(&jsonOut, "json", false, "output rows as JSON")
	cmd.AddCommand(syncCmd)

	return cmd
}

func runFleetSync(ctx context.Context, jsonOut bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	client, err := manager.NewClient(cfg.ManagerAPIBaseURL, cfg.ManagerBearerToken)
	if err != nil {
		return err
	}

	resp, err := client.FleetSubs(ctx, cfg.ManagerAccountID)
	if err != nil {
		return err
	}

	rows := fleet.BuildRows(resp, resp.Categories)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	printFleetTable(rows)
	return nil
}

func printFleetTable(rows []fleet.FleetRow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MARK\tPAIR_ID\tUSERNAME\tSERVER_ID\tELIGIBLE\tREASON\tIMAGE_HASH")

	for _, row := range rows {
		mark := " "
		eligible := "NO"
		if row.QAEligible {
			mark = "*"
			eligible = "YES"
		}

		serverID := ""
		if row.ServerID > 0 {
			serverID = fmt.Sprintf("%d", row.ServerID)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			mark,
			row.PairID,
			row.Username,
			serverID,
			eligible,
			row.IneligibleReason,
			row.DeployedImageHash,
		)
	}

	_ = w.Flush()
	fmt.Println()
	fmt.Println(strings.TrimSpace("Eligible rows marked with * (YES). Ineligible rows show reason."))
}

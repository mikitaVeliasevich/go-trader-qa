package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "gtqa",
		Short: "go-trader fleet QA orchestrator",
	}

	root.AddCommand(newFleetCmd())
	root.AddCommand(newSmokeCmd())
	root.AddCommand(newSoakCmd())
	root.AddCommand(newAnalyzeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

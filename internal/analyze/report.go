package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReportMeta holds run metadata for qa-report.md header.
type ReportMeta struct {
	Title     string
	ServerID  string
	Started   string
	Duration  string
	Interval  string
	Profile   Profile
	RunDir    string
}

// WriteReport writes qa-report.md for a completed soak run.
func WriteReport(path string, meta ReportMeta, d Deltas, gates []GateResult, pass bool) error {
	var b strings.Builder

	title := meta.Title
	if title == "" {
		title = filepath.Base(meta.RunDir)
	}

	fmt.Fprintf(&b, "# QA Report: %s\n\n", title)
	if meta.ServerID != "" {
		fmt.Fprintf(&b, "**Server ID:** %s  \n", meta.ServerID)
	}
	if meta.Started != "" {
		fmt.Fprintf(&b, "**Started:** %s  \n", meta.Started)
	}
	if meta.Duration != "" {
		fmt.Fprintf(&b, "**Duration:** %s  \n", meta.Duration)
	}
	if meta.Interval != "" {
		fmt.Fprintf(&b, "**Interval:** %s  \n", meta.Interval)
	}
	fmt.Fprintf(&b, "**Profile:** `%s`  \n", meta.Profile)
	if meta.RunDir != "" {
		fmt.Fprintf(&b, "**Run dir:** `%s`  \n", meta.RunDir)
	}

	elapsedSpan := d.LastElapsed - d.FirstElapsed
	fmt.Fprintf(&b, "\n## Summary\n\n")
	fmt.Fprintf(&b, "Samples: **%d** | Elapsed: **%d→%d min** (%d min span)\n\n", d.SampleCount, d.FirstElapsed, d.LastElapsed, elapsedSpan)
	fmt.Fprintf(&b, "**WS messages (overall):** %s", formatInt64(d.WSMessagesOverall))
	if d.WSMessagesDelta > 0 {
		fmt.Fprintf(&b, " (+%s during window)", formatInt64(d.WSMessagesDelta))
	}
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "### Key deltas (first→last row)\n\n")
	fmt.Fprintf(&b, "```\n")
	fmt.Fprintf(&b, "ws_messages_delta=%s  ws_messages_overall=%s\n", formatInt64(d.WSMessagesDelta), formatInt64(d.WSMessagesOverall))
	fmt.Fprintf(&b, "bus_drops=%s  order_create_ok=%s  order_filter_cancel=%s\n",
		formatInt64(d.BusDrops), formatInt64(d.OrderCreateOK), formatInt64(d.OrderFilterCancel))
	if meta.Profile == ProfileLifecycle {
		fmt.Fprintf(&b, "position_opened=%s  position_reset=%s  resting_proxy=%s\n",
			formatInt64(d.PositionOpened), formatInt64(d.PositionReset), formatInt64(d.RestingProxy))
		fmt.Fprintf(&b, "algo_paused=%s  algo_resumed_ok=%s\n",
			formatInt64(d.AlgoPaused), formatInt64(d.AlgoResumedOK))
	}
	fmt.Fprintf(&b, "```\n\n")

	fmt.Fprintf(&b, "## Gate Results (`gtqa analyze --profile %s`)\n\n", meta.Profile)
	fmt.Fprintf(&b, "| Gate | Result | Detail |\n")
	fmt.Fprintf(&b, "|------|--------|--------|\n")
	for _, g := range gates {
		result := "FAIL"
		if g.Pass {
			result = "PASS"
		}
		fmt.Fprintf(&b, "| %s | **%s** | %s |\n", g.ID, result, g.Detail)
	}

	fmt.Fprintf(&b, "\n## Status\n\n")
	if pass {
		fmt.Fprintf(&b, "**OVERALL: PASS**\n")
	} else {
		fmt.Fprintf(&b, "**OVERALL: FAIL**\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func formatInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

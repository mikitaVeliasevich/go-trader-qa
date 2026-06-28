package metrics

import "strings"

// BotStartMarker appears in go-trader cmd/bot/main.go printBanner() on process start.
const BotStartMarker = "Trading Bot"

// botBannerTop is the first line of printBanner() in cmd/bot/main.go.
const botBannerTop = "╔══════════════════════════════════════════════════════════════╗"

// BootstrapTailSteps grows log tail until BotStartMarker is found or the list ends.
var BootstrapTailSteps = []int{500, 2000, 5000, 10000, 25000, 50000}

// FetchLogsFromBotStart requests increasingly large log tails until the bot startup
// banner is present. When found, returns logs trimmed to the last boot (Docker may
// retain output from prior container runs before the banner).
func FetchLogsFromBotStart(fetch func(tail int) (string, error)) (string, bool, error) {
	var last string
	for _, tail := range BootstrapTailSteps {
		logs, err := fetch(tail)
		if err != nil {
			return "", false, err
		}
		last = logs
		if strings.Contains(logs, BotStartMarker) {
			return TrimToLastBotStart(logs), true, nil
		}
	}
	return last, false, nil
}

// TrimToLastBotStart drops Docker log lines before the most recent bot boot banner.
func TrimToLastBotStart(logs string) string {
	start := lastIndexLineStart(logs, botBannerTop)
	if start < 0 {
		start = lastIndexLineStart(logs, BotStartMarker)
	}
	if start < 0 {
		return logs
	}
	return logs[start:]
}

func lastIndexLineStart(logs, marker string) int {
	idx := strings.LastIndex(logs, marker)
	if idx < 0 {
		return -1
	}
	lineStart := strings.LastIndex(logs[:idx], "\n")
	if lineStart < 0 {
		return 0
	}
	return lineStart + 1
}

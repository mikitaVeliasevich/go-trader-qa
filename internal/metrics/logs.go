package metrics

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

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

var logTimestampRE = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)

const logTimestampLayout = "2006/01/02 15:04:05"

// ParseLogTimestamp parses the leading timestamp on a go-trader log line (server local time).
func ParseLogTimestamp(line string) (time.Time, bool) {
	m := logTimestampRE.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) < 2 {
		return time.Time{}, false
	}
	ts, err := time.ParseInLocation(logTimestampLayout, m[1], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

// LogFetchResult is the outcome of adaptive tail expansion for a retrospective window.
type LogFetchResult struct {
	Logs      string
	Oldest    time.Time
	Newest    time.Time
	FinalTail int
	Steps     []int
}

// FetchLogsForWindow grows logs?tail=N until oldest parsed timestamp is <= windowStart.
// No hard tail cap — expansion continues until covered or fetch returns an error.
func FetchLogsForWindow(fetch func(tail int) (string, error), windowStart time.Time) (LogFetchResult, error) {
	var (
		steps   []int
		oldest  time.Time
		newest  time.Time
		hasTime bool
	)

	for _, tail := range expandingTailSteps() {
		logs, err := fetch(tail)
		if err != nil {
			return LogFetchResult{}, err
		}
		steps = append(steps, tail)
		last := logs

		oldest, newest, hasTime = logTimeBounds(last)
		if !hasTime {
			continue
		}
		if !oldest.After(windowStart) {
			return LogFetchResult{
				Logs:      last,
				Oldest:    oldest,
				Newest:    newest,
				FinalTail: tail,
				Steps:     steps,
			}, nil
		}
	}

	if !hasTime {
		return LogFetchResult{}, fmt.Errorf("no parseable log timestamps in fetched logs")
	}
	return LogFetchResult{}, fmt.Errorf(
		"window longer than log history: oldest=%s window_start=%s (tried tails %v)",
		oldest.Format(time.RFC3339), windowStart.Format(time.RFC3339), steps,
	)
}

func expandingTailSteps() []int {
	steps := append([]int{}, BootstrapTailSteps...)
	for tail := 100000; tail <= 2000000; tail += 50000 {
		steps = append(steps, tail)
	}
	return steps
}

func logTimeBounds(logs string) (oldest, newest time.Time, ok bool) {
	var found bool
	for _, line := range strings.Split(logs, "\n") {
		ts, parsed := ParseLogTimestamp(line)
		if !parsed {
			continue
		}
		if !found {
			oldest, newest, found = ts, ts, true
			continue
		}
		if ts.Before(oldest) {
			oldest = ts
		}
		if ts.After(newest) {
			newest = ts
		}
	}
	return oldest, newest, found
}

// FilterLogsByWindow keeps lines with timestamps in [start, end] inclusive.
func FilterLogsByWindow(logs string, start, end time.Time) string {
	var b strings.Builder
	for _, line := range strings.Split(logs, "\n") {
		ts, ok := ParseLogTimestamp(line)
		if !ok {
			continue
		}
		if ts.Before(start) || ts.After(end) {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

package metrics

import (
	"regexp"
	"strings"
)

// WindowEventCounts holds grep-derived event counts inside a filtered soak.log window.
type WindowEventCounts struct {
	OrderCreateOK  int
	OrderAmendOK   int
	OrderCancelOK  int
	Errors10003    int
	AlgoResumedOK  int
}

var windowEventPatterns = struct {
	orderCreate  *regexp.Regexp
	orderAmend   *regexp.Regexp
	orderCancel  *regexp.Regexp
	errors10003  *regexp.Regexp
	algoResumed  *regexp.Regexp
}{
	orderCreate: regexp.MustCompile(`Order successful: action=create`),
	orderAmend:  regexp.MustCompile(`Order successful: action=amend`),
	orderCancel: regexp.MustCompile(`Order successful: action=cancel`),
	errors10003: regexp.MustCompile(`retCode=10003|retCode":10003`),
	algoResumed: regexp.MustCompile(`Resumed after .+ cooldown`),
}

// CountWindowEvents counts reconstructable log events in window-filtered soak.log text.
func CountWindowEvents(logText string) WindowEventCounts {
	lines := strings.Split(logText, "\n")
	return WindowEventCounts{
		OrderCreateOK: countMatchingLines(lines, windowEventPatterns.orderCreate),
		OrderAmendOK:  countMatchingLines(lines, windowEventPatterns.orderAmend),
		OrderCancelOK: countMatchingLines(lines, windowEventPatterns.orderCancel),
		Errors10003:   countMatchingLines(lines, windowEventPatterns.errors10003),
		AlgoResumedOK: countMatchingLines(lines, windowEventPatterns.algoResumed),
	}
}

// ReconstructResult is the synthetic start metrics row plus estimation flags.
type ReconstructResult struct {
	Start     Row
	Estimated bool
}

// ReconstructStartRow derives start-of-window counters from end expvar row and in-window log events.
func ReconstructStartRow(end Row, events WindowEventCounts, lifecycle bool) ReconstructResult {
	if lifecycle {
		return ReconstructResult{
			Start: Row{
				TimestampUTC: end.TimestampUTC,
				ElapsedMin:   0,
			},
			Estimated: false,
		}
	}

	start := end
	start.ElapsedMin = 0
	start.OrderCreateOK = subNonNeg(end.OrderCreateOK, int64(events.OrderCreateOK))
	start.OrderAmendOK = subNonNeg(end.OrderAmendOK, int64(events.OrderAmendOK))
	start.OrderCancelOK = subNonNeg(end.OrderCancelOK, int64(events.OrderCancelOK))
	start.AlgoResumedOK = subNonNeg(end.AlgoResumedOK, int64(events.AlgoResumedOK))

	estimated := false
	// Counters without reliable log proxies keep end values (delta 0) — footnote in qa-report.
	if end.BusDrops > 0 || end.OrderFilterCancel > 0 || end.PositionOpened > 0 ||
		end.PositionReset > 0 || end.AlgoPaused > 0 || end.WSMessages > 0 {
		start.BusDrops = end.BusDrops
		start.OrderFilterCancel = end.OrderFilterCancel
		start.PositionOpened = end.PositionOpened
		start.PositionReset = end.PositionReset
		start.PositionResetSL = end.PositionResetSL
		start.PositionResetTP = end.PositionResetTP
		start.PositionResetCancel = end.PositionResetCancel
		start.PositionResetOther = end.PositionResetOther
		start.AlgoPaused = end.AlgoPaused
		start.WSMessages = end.WSMessages
		start.BusPublishes = end.BusPublishes
		start.TickerParsed = end.TickerParsed
		start.PrivateParsed = end.PrivateParsed
		start.KlineParsed = end.KlineParsed
		start.OrderCreateBlockedPosition = end.OrderCreateBlockedPosition
		estimated = true
	}

	start.ConnectionLost = 0
	start.Reconnected = 0
	start.ReconnectFailed = 0
	start.Errors10003 = 0
	start.OrderFailures = 0

	return ReconstructResult{Start: start, Estimated: estimated}
}

func subNonNeg(a int64, b int64) int64 {
	if a > b {
		return a - b
	}
	return 0
}

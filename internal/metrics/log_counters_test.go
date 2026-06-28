package metrics

import (
	"strings"
	"testing"
)

func TestOrderFailRegexMatchesCreateFailed(t *testing.T) {
	log := strings.Join([]string{
		"2026-06-28T12:00:00Z Order create failed: retCode=110001",
		"2026-06-28T12:00:01Z Order amend failed: retCode=10006",
		"2026-06-28T12:00:02Z Order cancel failed: retCode=10001",
		"2026-06-28T12:00:03Z unrelated line",
	}, "\n")

	got := CountLogPatterns(log)
	if got.OrderFailures != 3 {
		t.Fatalf("OrderFailures=%d want 3", got.OrderFailures)
	}
}

func TestOrderFailRegexIgnoresOldPattern(t *testing.T) {
	log := "2026-06-28T12:00:00Z Order failed: something\n"
	got := CountLogPatterns(log)
	if got.OrderFailures != 0 {
		t.Fatalf("OrderFailures=%d want 0 for legacy pattern", got.OrderFailures)
	}
}

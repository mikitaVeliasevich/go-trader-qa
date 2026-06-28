package metrics

import (
	"fmt"
	"strings"
	"testing"
)

func TestFetchLogsFromBotStartFoundOnFirstTry(t *testing.T) {
	logs, found, err := FetchLogsFromBotStart(func(tail int) (string, error) {
		return "╔══\n║  Trading Bot\n╚══\nstarted\n", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected marker found")
	}
	if !strings.Contains(logs, BotStartMarker) {
		t.Fatalf("logs missing marker: %q", logs)
	}
}

func TestFetchLogsFromBotStartGrowsTailUntilFound(t *testing.T) {
	calls := 0
	logs, found, err := FetchLogsFromBotStart(func(tail int) (string, error) {
		calls++
		if tail < 5000 {
			return fmt.Sprintf("recent line %d\n", tail), nil
		}
		return "╔══\n║  Trading Bot\n╚══\n", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected marker found")
	}
	if calls < 3 {
		t.Fatalf("expected multiple tail attempts, got %d", calls)
	}
	if !strings.Contains(logs, BotStartMarker) {
		t.Fatalf("logs missing marker: %q", logs)
	}
}

func TestFetchLogsFromBotStartNotFoundReturnsLast(t *testing.T) {
	lastTail := 0
	logs, found, err := FetchLogsFromBotStart(func(tail int) (string, error) {
		lastTail = tail
		return fmt.Sprintf("only recent %d\n", tail), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected marker not found")
	}
	if lastTail != BootstrapTailSteps[len(BootstrapTailSteps)-1] {
		t.Fatalf("last tail = %d, want %d", lastTail, BootstrapTailSteps[len(BootstrapTailSteps)-1])
	}
	if !strings.Contains(logs, "only recent") {
		t.Fatalf("unexpected logs: %q", logs)
	}
}

func TestTrimToLastBotStart(t *testing.T) {
	old := "old session\nnoise\n"
	banner := "╔══════════════════════════════════════════════════════════════╗\n║                     Trading Bot                              ║\n╚══════════════════════════════════════════════════════════════╝\n"
	current := "after boot\n"
	logs := old + banner + current

	trimmed := TrimToLastBotStart(logs)
	if strings.Contains(trimmed, "old session") {
		t.Fatalf("expected old session trimmed, got %q", trimmed)
	}
	if !strings.HasPrefix(trimmed, "╔") {
		t.Fatalf("expected trim at banner, got %q", trimmed)
	}
	if !strings.Contains(trimmed, "after boot") {
		t.Fatalf("expected post-banner logs kept: %q", trimmed)
	}
}

package metrics

import (
	"fmt"
	"strings"
	"time"
)

const WindowLifecycle = "lifecycle"

// WindowSpec is a retrospective analyze time window.
type WindowSpec struct {
	Lifecycle bool
	Duration  time.Duration
}

// ParseWindow parses analyze window strings: 5m, 30m, 1h, 4h, 8h, lifecycle.
func ParseWindow(s string) (WindowSpec, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return WindowSpec{}, fmt.Errorf("window is required")
	}
	if s == WindowLifecycle {
		return WindowSpec{Lifecycle: true}, nil
	}
	d, err := ParseDuration(s)
	if err != nil {
		return WindowSpec{}, fmt.Errorf("invalid window %q: %w", s, err)
	}
	if d <= 0 {
		return WindowSpec{}, fmt.Errorf("window must be positive")
	}
	return WindowSpec{Duration: d}, nil
}

// String returns the canonical window label for run.env / batch summary.
func (w WindowSpec) String() string {
	if w.Lifecycle {
		return WindowLifecycle
	}
	return w.Duration.String()
}

package batch

import (
	"fmt"
	"strings"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

// NormalizeBatchSpec fills defaults and validates mode-specific fields.
func NormalizeBatchSpec(spec *BatchSpec) error {
	if strings.TrimSpace(spec.Mode) == "" {
		spec.Mode = ModeSoak
	}
	spec.Mode = strings.ToLower(strings.TrimSpace(spec.Mode))

	switch spec.Mode {
	case ModeSoak:
		if spec.Duration <= 0 {
			return fmt.Errorf("duration must be positive for soak mode")
		}
	case ModeAnalyze:
		if !spec.Window.Lifecycle && spec.Window.Duration <= 0 {
			return fmt.Errorf("window is required for analyze mode")
		}
	default:
		return fmt.Errorf("mode must be soak or analyze")
	}
	return nil
}

// ParseBatchWindow parses window string for analyze mode batches.
func ParseBatchWindow(raw string) (metrics.WindowSpec, error) {
	return metrics.ParseWindow(raw)
}

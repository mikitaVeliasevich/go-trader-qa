package batch

import (
	"testing"
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

func TestNormalizeBatchSpecSoakRequiresDuration(t *testing.T) {
	spec := BatchSpec{Mode: ModeSoak}
	if err := NormalizeBatchSpec(&spec); err == nil {
		t.Fatal("expected error for missing duration")
	}
}

func TestNormalizeBatchSpecAnalyzeRequiresWindow(t *testing.T) {
	spec := BatchSpec{Mode: ModeAnalyze}
	if err := NormalizeBatchSpec(&spec); err == nil {
		t.Fatal("expected error for missing window")
	}

	spec = BatchSpec{
		Mode:   ModeAnalyze,
		Window: metrics.WindowSpec{Duration: 30 * time.Minute},
	}
	if err := NormalizeBatchSpec(&spec); err != nil {
		t.Fatal(err)
	}
}

func TestParseBatchWindowLifecycle(t *testing.T) {
	w, err := ParseBatchWindow("lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	if !w.Lifecycle {
		t.Fatalf("got %+v", w)
	}
}

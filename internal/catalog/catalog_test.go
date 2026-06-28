package catalog

import (
	"testing"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

func TestEmbeddedCatalog(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 2 {
		t.Fatalf("version=%d", c.Version)
	}
	if len(c.Metrics) < 50 {
		t.Fatalf("metrics=%d", len(c.Metrics))
	}
	gates, ok := c.GatesForProfile("lifecycle-strict")
	if !ok || len(gates) != 16 {
		t.Fatalf("lifecycle-strict gates=%v", gates)
	}
	gates, ok = c.GatesForProfile("tpsl-health")
	if !ok || len(gates) != 4 {
		t.Fatalf("tpsl-health gates=%v", gates)
	}
}

func TestCatalogMatchesOptionalTSVColumns(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	optional := make(map[string]bool, len(metrics.OptionalTSVColumns))
	for _, col := range metrics.OptionalTSVColumns {
		optional[col] = true
	}
	for _, m := range c.Metrics {
		if optional[m.TSV] {
			delete(optional, m.TSV)
		}
	}
	if len(optional) > 0 {
		t.Fatalf("catalog missing optional TSV columns: %v", optional)
	}
}

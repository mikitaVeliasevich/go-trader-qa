package catalog

import "testing"

func TestBuildGuide(t *testing.T) {
	g := BuildGuide()
	if len(g.Gates) != 16 {
		t.Fatalf("gates=%d want 16", len(g.Gates))
	}
	if len(g.Profiles) != 4 {
		t.Fatalf("profiles=%d want 4", len(g.Profiles))
	}
	if g.Intro.GateNaming == "" {
		t.Fatal("missing gate naming intro")
	}
	totalMetrics := 0
	for _, grp := range g.Groups {
		totalMetrics += len(grp.Metrics)
	}
	if totalMetrics < 50 {
		t.Fatalf("metrics=%d", totalMetrics)
	}
}

package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

func TestGateG4NoLifecycle(t *testing.T) {
	d := Deltas{PositionOpened: 0, PositionReset: 0}
	gates, pass := EvaluateGates(d, nil, "", ProfileLifecycle)
	if !pass {
		t.Fatalf("expected pass for 0/0 lifecycle, got FAIL")
	}
	if gates[3].ID != "G4" || !gates[3].Pass {
		t.Fatalf("G4: %+v", gates[3])
	}
}

func TestGateG4BothActive(t *testing.T) {
	d := Deltas{PositionOpened: 10, PositionReset: 8}
	gates, _ := EvaluateGates(d, nil, "", ProfileLifecycle)
	if gates[3].ID != "G4" || !gates[3].Pass {
		t.Fatalf("G4: %+v", gates[3])
	}
}

func TestGateG4Inconsistent(t *testing.T) {
	d := Deltas{PositionOpened: 5, PositionReset: 0}
	gates, pass := EvaluateGates(d, nil, "", ProfileLifecycle)
	if pass {
		t.Fatal("expected FAIL for inconsistent lifecycle")
	}
	if gates[3].ID != "G4" || gates[3].Pass {
		t.Fatalf("G4: %+v", gates[3])
	}
}

func TestComputeDeltasWSMessages(t *testing.T) {
	rows := []metrics.Row{
		{ElapsedMin: 0, WSMessages: 11172799},
		{ElapsedMin: 5, WSMessages: 11451533},
	}
	d := ComputeDeltas(rows)
	if d.WSMessagesOverall != 11451533 {
		t.Fatalf("overall=%d", d.WSMessagesOverall)
	}
	if d.WSMessagesDelta != 278734 {
		t.Fatalf("delta=%d", d.WSMessagesDelta)
	}
}

func TestRunWritesWSMessagesToReport(t *testing.T) {
	dir := t.TempDir()
	optionalZeros := strings.Repeat("\t0", len(metrics.OptionalTSVColumns))
	tsv := strings.Join([]string{
		metrics.TSVHeader,
		"2026-06-28T13:44:01Z\t0\t11172799\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0" + optionalZeros + "\t0\t0\t0\t0\t0",
		"2026-06-28T13:48:53Z\t5\t11451533\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0" + optionalZeros + "\t0\t0\t0\t0\t0",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "metrics.tsv"), []byte(tsv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "soak.log"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(dir, ProfileLifecycle)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "**WS messages (overall):** 11451533") {
		t.Fatalf("missing ws overall in report:\n%s", text)
	}
	if !strings.Contains(text, "ws_messages_overall=11451533") {
		t.Fatalf("missing ws in key deltas:\n%s", text)
	}
}

func TestGatesG8G16(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		d       Deltas
		wantID  string
		wantPass bool
	}{
		{
			name: "G8 pass", profile: ProfileLifecycleStrict,
			d: Deltas{AlgoStoppedPermanent: 0},
			wantID: "G8", wantPass: true,
		},
		{
			name: "G8 fail", profile: ProfileLifecycleStrict,
			d: Deltas{AlgoStoppedPermanent: 2},
			wantID: "G8", wantPass: false,
		},
		{
			name: "G9 pass", profile: ProfileLifecycleStrict,
			d: Deltas{OrderFailRet10003: 0},
			wantID: "G9", wantPass: true,
		},
		{
			name: "G9 fail", profile: ProfileLifecycleStrict,
			d: Deltas{OrderFailRet10003: 1},
			wantID: "G9", wantPass: false,
		},
		{
			name: "G10 pass breakdown", profile: ProfileTPSLHealth,
			d: Deltas{
				PositionReset: 10, PositionResetSL: 3, PositionResetTP: 2,
				PositionResetCancel: 1, PositionResetLiquidation: 2,
				PositionResetManual: 1, PositionResetOther: 1,
			},
			wantID: "G10", wantPass: true,
		},
		{
			name: "G10 fail breakdown", profile: ProfileTPSLHealth,
			d: Deltas{PositionReset: 5, PositionResetSL: 2},
			wantID: "G10", wantPass: false,
		},
		{
			name: "G11 pass", profile: ProfileWSSOnly,
			d: Deltas{ConnectionLostLast: 2, ReconnectedLast: 2, ReconnectFailedLast: 0},
			wantID: "G11", wantPass: true,
		},
		{
			name: "G11 fail reconnect", profile: ProfileWSSOnly,
			d: Deltas{ConnectionLostLast: 0, ReconnectedLast: 0, ReconnectFailedLast: 1},
			wantID: "G11", wantPass: false,
		},
		{
			name: "G12 pass", profile: ProfileTPSLHealth,
			d: Deltas{TPMissingCreateFail: 0, TPMissingClosePosition: 0},
			wantID: "G12", wantPass: true,
		},
		{
			name: "G12 fail", profile: ProfileTPSLHealth,
			d: Deltas{TPMissingCreateFail: 1},
			wantID: "G12", wantPass: false,
		},
		{
			name: "G13 pass", profile: ProfileTPSLHealth,
			d: Deltas{SLSetupFail: 0, SLPricePastImmediateClose: 0},
			wantID: "G13", wantPass: true,
		},
		{
			name: "G13 fail", profile: ProfileTPSLHealth,
			d: Deltas{SLSetupFail: 1},
			wantID: "G13", wantPass: false,
		},
		{
			name: "G14 pass", profile: ProfileTPSLHealth,
			d: Deltas{PartialTPNeedsCancel: 2, OrderCancelOK: 3, PositionOpened: 1},
			wantID: "G14", wantPass: true,
		},
		{
			name: "G14 fail", profile: ProfileTPSLHealth,
			d: Deltas{PartialTPNeedsCancel: 5, OrderCancelOK: 2, PositionOpened: 1},
			wantID: "G14", wantPass: false,
		},
		{
			name: "G15 pass", profile: ProfileLifecycleStrict,
			d: Deltas{OrderFailCreate: 0, OrderFailAmend: 0, OrderFailCancel: 0, OrderFailTotal: 0},
			wantID: "G15", wantPass: true,
		},
		{
			name: "G15 fail", profile: ProfileLifecycleStrict,
			d: Deltas{OrderFailCreate: 1, OrderFailTotal: 1},
			wantID: "G15", wantPass: false,
		},
		{
			name: "G16 pass", profile: ProfileLifecycleStrict,
			d: Deltas{PriceWakeSignals: 10, PriceExecRuns: 5, PositionOpened: 2, PrivateOrderEventsDrained: 3},
			wantID: "G16", wantPass: true,
		},
		{
			name: "G16 fail wake", profile: ProfileLifecycleStrict,
			d: Deltas{PriceWakeSignals: 5, PriceExecRuns: 0},
			wantID: "G16", wantPass: false,
		},
		{
			name: "G16 fail private drain", profile: ProfileLifecycleStrict,
			d: Deltas{PositionOpened: 2, PrivateOrderEventsDrained: 0},
			wantID: "G16", wantPass: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gates, _ := EvaluateGates(tc.d, nil, "", tc.profile)
			var got *GateResult
			for i := range gates {
				if gates[i].ID == tc.wantID {
					got = &gates[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("gate %s not found in %+v", tc.wantID, gates)
			}
			if got.Pass != tc.wantPass {
				t.Fatalf("%s pass=%v detail=%q", tc.wantID, got.Pass, got.Detail)
			}
		})
	}
}

func TestValidProfile(t *testing.T) {
	for _, p := range []Profile{ProfileWSSOnly, ProfileLifecycle, ProfileLifecycleStrict, ProfileTPSLHealth} {
		if !ValidProfile(p) {
			t.Fatalf("expected valid profile %q", p)
		}
	}
	if ValidProfile(Profile("unknown")) {
		t.Fatal("expected invalid profile")
	}
}

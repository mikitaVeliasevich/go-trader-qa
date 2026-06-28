package analyze

import (
	"fmt"
	"os"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

const activeMarketsMax = 30

// Profile selects which gates run.
type Profile string

const (
	ProfileLifecycle Profile = "lifecycle"
	ProfileWSSOnly   Profile = "wss-only"
)

// GateResult is one gate evaluation.
type GateResult struct {
	ID     string
	Pass   bool
	Detail string
}

// EvaluateGates runs G1–G7 (lifecycle) or G1–G2 (wss-only).
func EvaluateGates(d Deltas, soakLogPath string, profile Profile) ([]GateResult, bool) {
	var results []GateResult
	pass := true

	add := func(id string, ok bool, detail string) {
		results = append(results, GateResult{ID: id, Pass: ok, Detail: detail})
		if !ok {
			pass = false
		}
	}

	if d.BusDrops == 0 {
		add("G1", true, "bus_drops delta=0 (first→last)")
	} else {
		add("G1", false, fmt.Sprintf("bus_drops delta=%d (expected 0)", d.BusDrops))
	}

	e10003 := countRetCode10003(soakLogPath)
	if e10003 == 0 {
		add("G2", true, "no retCode=10003 in soak.log")
	} else {
		add("G2", false, fmt.Sprintf("found %d retCode=10003 in soak.log", e10003))
	}

	if profile == ProfileLifecycle {
		evalLifecycleGates(d, add)
	}

	return results, pass
}

func evalLifecycleGates(d Deltas, add func(string, bool, string)) {
	if d.OrderFilterCancel > 0 {
		add("G3", true, fmt.Sprintf("order_filter_cancel delta=%d", d.OrderFilterCancel))
	} else if d.RestingProxy <= activeMarketsMax {
		add("G3", true, fmt.Sprintf("order_create_ok bounded: resting_proxy=%d <= %d", d.RestingProxy, activeMarketsMax))
	} else {
		add("G3", false, fmt.Sprintf("order_filter_cancel delta=0 and resting_proxy=%d > %d", d.RestingProxy, activeMarketsMax))
	}

	switch {
	case d.PositionOpened == 0 && d.PositionReset == 0:
		add("G4", true, "no position lifecycle during window (position_opened delta=0, position_reset delta=0)")
	case d.PositionOpened >= 1 && d.PositionReset >= 1:
		add("G4", true, fmt.Sprintf("position_opened delta=%d, position_reset delta=%d", d.PositionOpened, d.PositionReset))
	default:
		add("G4", false, fmt.Sprintf("position_opened delta=%d, position_reset delta=%d (inconsistent lifecycle)", d.PositionOpened, d.PositionReset))
	}

	minCreates := (d.PositionReset*85 + 99) / 100
	if d.OrderCreateOK >= minCreates {
		add("G5", true, fmt.Sprintf("order_create_ok delta=%d >= %d (85%% of position_reset delta=%d)", d.OrderCreateOK, minCreates, d.PositionReset))
	} else {
		add("G5", false, fmt.Sprintf("order_create_ok delta=%d < %d (85%% of position_reset delta=%d)", d.OrderCreateOK, minCreates, d.PositionReset))
	}

	maxReset := d.PositionOpened + 1
	if d.PositionReset <= maxReset {
		add("G6", true, fmt.Sprintf("position_reset delta=%d <= position_opened delta=%d + 1", d.PositionReset, d.PositionOpened))
	} else {
		add("G6", false, fmt.Sprintf("position_reset delta=%d > position_opened delta=%d + 1", d.PositionReset, d.PositionOpened))
	}

	firstInFlight := d.FirstAlgoPaused - d.FirstAlgoResumed
	inFlight := d.LastAlgoPaused - d.LastAlgoResumed
	allowMissing := inFlight - firstInFlight
	maxResumed := d.AlgoPaused + d.FirstAlgoPaused
	minResumed := d.AlgoPaused - allowMissing - 1
	if minResumed < 0 {
		minResumed = 0
	}

	switch {
	case d.AlgoResumedOK > maxResumed:
		add("G7", false, fmt.Sprintf("algo_resumed_ok delta=%d > algo_paused delta=%d + first_sample_paused=%d", d.AlgoResumedOK, d.AlgoPaused, d.FirstAlgoPaused))
	case d.AlgoPaused > 0 && d.AlgoResumedOK < minResumed:
		add("G7", false, fmt.Sprintf("algo_paused delta=%d but algo_resumed_ok delta=%d (expected >= %d, in_flight=%d first_in_flight=%d)", d.AlgoPaused, d.AlgoResumedOK, minResumed, inFlight, firstInFlight))
	default:
		add("G7", true, fmt.Sprintf("algo_paused delta=%d, algo_resumed_ok delta=%d (in_flight=%d, allow_missing=%d)", d.AlgoPaused, d.AlgoResumedOK, inFlight, allowMissing))
	}
}

func countRetCode10003(soakLogPath string) int {
	if soakLogPath == "" {
		return 0
	}
	data, err := os.ReadFile(soakLogPath)
	if err != nil {
		return 0
	}
	return metrics.CountLogPatterns(string(data)).Errors10003
}

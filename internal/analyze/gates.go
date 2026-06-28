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
	ProfileLifecycle       Profile = "lifecycle"
	ProfileWSSOnly         Profile = "wss-only"
	ProfileLifecycleStrict Profile = "lifecycle-strict"
	ProfileTPSLHealth      Profile = "tpsl-health"
)

// ValidProfile reports whether p is a supported analyzer profile.
func ValidProfile(p Profile) bool {
	switch p {
	case ProfileLifecycle, ProfileWSSOnly, ProfileLifecycleStrict, ProfileTPSLHealth:
		return true
	default:
		return false
	}
}

// GateResult is one gate evaluation.
type GateResult struct {
	ID     string
	Pass   bool
	Detail string
}

func profileGateIDs(profile Profile) map[string]bool {
	switch profile {
	case ProfileWSSOnly:
		return map[string]bool{"G1": true, "G2": true, "G11": true}
	case ProfileLifecycle:
		return map[string]bool{
			"G1": true, "G2": true, "G3": true, "G4": true,
			"G5": true, "G6": true, "G7": true,
		}
	case ProfileLifecycleStrict:
		ids := make(map[string]bool, 16)
		for i := 1; i <= 16; i++ {
			ids[fmt.Sprintf("G%d", i)] = true
		}
		return ids
	case ProfileTPSLHealth:
		return map[string]bool{"G10": true, "G12": true, "G13": true, "G14": true}
	default:
		return nil
	}
}

// EvaluateGates runs profile-selected gates against deltas and optional metrics rows.
func EvaluateGates(d Deltas, rows []metrics.Row, soakLogPath string, profile Profile) ([]GateResult, bool) {
	active := profileGateIDs(profile)
	var results []GateResult
	pass := true

	add := func(id string, ok bool, detail string) {
		if !active[id] {
			return
		}
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

	evalLifecycleGates(d, add)
	evalStrictGates(d, add)
	evalReconnectGate(d, add)
	evalTPSLGates(d, add)
	evalWakeGates(d, add)

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

	maxResumed := d.AlgoPaused + d.FirstAlgoPaused
	minResumed := d.AlgoPaused - d.AllowMissing - 1
	if minResumed < 0 {
		minResumed = 0
	}

	switch {
	case d.AlgoResumedOK > maxResumed:
		add("G7", false, fmt.Sprintf("algo_resumed_ok delta=%d > algo_paused delta=%d + first_sample_paused=%d", d.AlgoResumedOK, d.AlgoPaused, d.FirstAlgoPaused))
	case d.AlgoPaused > 0 && d.AlgoResumedOK < minResumed:
		add("G7", false, fmt.Sprintf("algo_paused delta=%d but algo_resumed_ok delta=%d (expected >= %d, in_flight=%d first_in_flight=%d)", d.AlgoPaused, d.AlgoResumedOK, minResumed, d.InFlight, d.FirstInFlight))
	default:
		add("G7", true, fmt.Sprintf("algo_paused delta=%d, algo_resumed_ok delta=%d (in_flight=%d, allow_missing=%d)", d.AlgoPaused, d.AlgoResumedOK, d.InFlight, d.AllowMissing))
	}
}

func evalStrictGates(d Deltas, add func(string, bool, string)) {
	if d.AlgoStoppedPermanent == 0 {
		add("G8", true, "algo_stopped_permanent delta=0")
	} else {
		add("G8", false, fmt.Sprintf("algo_stopped_permanent delta=%d (expected 0)", d.AlgoStoppedPermanent))
	}

	if d.OrderFailRet10003 == 0 {
		add("G9", true, "order_fail_ret_10003 delta=0")
	} else {
		add("G9", false, fmt.Sprintf("order_fail_ret_10003 delta=%d (expected 0)", d.OrderFailRet10003))
	}

	if d.OrderFailTotal == 0 {
		add("G15", true, "order_fail_total delta=0 (create+amend+cancel)")
	} else {
		add("G15", false, fmt.Sprintf("order_fail_total delta=%d (create=%d amend=%d cancel=%d)", d.OrderFailTotal, d.OrderFailCreate, d.OrderFailAmend, d.OrderFailCancel))
	}
}

func evalReconnectGate(d Deltas, add func(string, bool, string)) {
	if d.ReconnectFailedLast != 0 {
		add("G11", false, fmt.Sprintf("reconnect_failed last=%d (expected 0)", d.ReconnectFailedLast))
		return
	}
	maxLost := d.ReconnectedLast + 1
	if d.ConnectionLostLast <= maxLost {
		add("G11", true, fmt.Sprintf("connection_lost last=%d <= reconnected last=%d + 1", d.ConnectionLostLast, d.ReconnectedLast))
	} else {
		add("G11", false, fmt.Sprintf("connection_lost last=%d > reconnected last=%d + 1", d.ConnectionLostLast, d.ReconnectedLast))
	}
}

func evalTPSLGates(d Deltas, add func(string, bool, string)) {
	if d.PositionReset > 0 {
		breakdown := d.PositionResetSL + d.PositionResetTP + d.PositionResetCancel +
			d.PositionResetLiquidation + d.PositionResetManual + d.PositionResetOther
		if breakdown == d.PositionReset {
			add("G10", true, fmt.Sprintf("position_reset breakdown=%d (sl=%d tp=%d cancel=%d liq=%d manual=%d other=%d)",
				breakdown, d.PositionResetSL, d.PositionResetTP, d.PositionResetCancel, d.PositionResetLiquidation, d.PositionResetManual, d.PositionResetOther))
		} else {
			add("G10", false, fmt.Sprintf("position_reset delta=%d but breakdown=%d (sl=%d tp=%d cancel=%d liq=%d manual=%d other=%d)",
				d.PositionReset, breakdown, d.PositionResetSL, d.PositionResetTP, d.PositionResetCancel, d.PositionResetLiquidation, d.PositionResetManual, d.PositionResetOther))
		}
	} else {
		add("G10", true, "position_reset delta=0 (breakdown check skipped)")
	}

	if d.TPMissingCreateFail == 0 && d.TPMissingClosePosition == 0 {
		add("G12", true, "tp_missing_create_fail delta=0 and tp_missing_close_position delta=0")
	} else {
		add("G12", false, fmt.Sprintf("tp_missing_create_fail delta=%d, tp_missing_close_position delta=%d", d.TPMissingCreateFail, d.TPMissingClosePosition))
	}

	if d.SLSetupFail == 0 && d.SLPricePastImmediateClose == 0 {
		add("G13", true, "sl_setup_fail delta=0 and sl_price_past_immediate_close delta=0")
	} else {
		add("G13", false, fmt.Sprintf("sl_setup_fail delta=%d, sl_price_past_immediate_close delta=%d", d.SLSetupFail, d.SLPricePastImmediateClose))
	}

	lifecycleActive := d.PositionOpened > 0 || d.PositionReset > 0
	if d.PartialTPNeedsCancel > 0 || lifecycleActive {
		if d.PartialTPNeedsCancel <= d.OrderCancelOK {
			add("G14", true, fmt.Sprintf("partial_tp_needs_cancel delta=%d <= order_cancel_ok delta=%d", d.PartialTPNeedsCancel, d.OrderCancelOK))
		} else {
			add("G14", false, fmt.Sprintf("partial_tp_needs_cancel delta=%d > order_cancel_ok delta=%d", d.PartialTPNeedsCancel, d.OrderCancelOK))
		}
	} else {
		add("G14", true, "partial_tp_needs_cancel delta=0 and no position lifecycle (check skipped)")
	}
}

func evalWakeGates(d Deltas, add func(string, bool, string)) {
	var failures []string

	if d.PriceWakeSignals > 0 && d.PriceExecRuns <= 0 {
		failures = append(failures, fmt.Sprintf("price_wake_signals delta=%d but price_exec_runs delta=%d", d.PriceWakeSignals, d.PriceExecRuns))
	}
	if d.PositionOpened > 0 && d.PrivateOrderEventsDrained <= 0 {
		failures = append(failures, fmt.Sprintf("position_opened delta=%d but private_order_events_drained delta=%d", d.PositionOpened, d.PrivateOrderEventsDrained))
	}

	if len(failures) == 0 {
		add("G16", true, fmt.Sprintf("wake health ok (price_wake=%d price_exec=%d position_opened=%d private_drained=%d)",
			d.PriceWakeSignals, d.PriceExecRuns, d.PositionOpened, d.PrivateOrderEventsDrained))
	} else {
		add("G16", false, failures[0])
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

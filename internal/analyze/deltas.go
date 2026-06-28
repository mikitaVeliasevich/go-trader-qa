package analyze

import "github.com/dlisovsky/go-trader-qa/internal/metrics"

// Deltas holds first→last metric window values from metrics.tsv.
type Deltas struct {
	SampleCount   int
	FirstElapsed  int
	LastElapsed   int
	LastTimestamp string

	BusDrops          int64
	OrderCreateOK     int64
	OrderAmendOK      int64
	OrderCancelOK     int64
	OrderFilterCancel int64
	PositionOpened    int64
	PositionReset     int64
	PositionResetSL   int64
	PositionResetTP   int64
	PositionResetCancel int64
	PositionResetOther int64
	PositionResetLiquidation int64
	PositionResetManual int64
	AlgoPaused        int64
	AlgoResumedOK     int64
	RestingProxy      int64

	OrderFailCreate int64
	OrderFailAmend  int64
	OrderFailCancel int64
	OrderFailTotal  int64

	OrderFailRet10003  int64
	OrderFailRet10006  int64
	OrderFailRet10001  int64
	OrderFailRet10016  int64
	OrderFailRet110001 int64
	OrderFailRet110007 int64
	OrderFailRet110013 int64
	OrderFailRet110021 int64
	OrderFailRet110090 int64
	OrderFailRet110094 int64
	OrderFailRet110059 int64
	OrderFailRet110123 int64
	OrderFailRet110126 int64

	AlgoStoppedPermanent      int64
	RiskLimitMarginReduced    int64
	TPMissingCreateOK         int64
	TPMissingCreateFail       int64
	TPMissingClosePosition    int64
	SLSetupStarted            int64
	SLSetupOK                 int64
	SLSetupFail               int64
	SLSetupCancelled          int64
	SLPricePastImmediateClose int64
	PartialTPNeedsCancel      int64

	PriceWakeSignals           int64
	PriceExecRuns              int64
	PrivateOrderWakeSignals    int64
	PrivateOrderEventsSignaled int64
	PrivateOrderDrainBatches   int64
	PrivateOrderEventsDrained  int64

	WSMessagesDelta   int64
	WSMessagesOverall int64

	FirstAlgoPaused  int64
	FirstAlgoResumed int64
	LastAlgoPaused   int64
	LastAlgoResumed  int64
	FirstInFlight    int64
	InFlight         int64
	AllowMissing     int64

	ConnectionLostLast   int
	ReconnectedLast    int
	ReconnectFailedLast int
}

// ComputeDeltas derives gate inputs from parsed TSV rows.
func ComputeDeltas(rows []metrics.Row) Deltas {
	first := rows[0]
	last := rows[len(rows)-1]

	d := Deltas{
		SampleCount:       len(rows),
		FirstElapsed:      first.ElapsedMin,
		LastElapsed:       last.ElapsedMin,
		LastTimestamp:     last.TimestampUTC,
		BusDrops:          metrics.MetricDelta(rows, "bus_drops"),
		OrderCreateOK:     metrics.MetricDelta(rows, "order_create_ok"),
		OrderAmendOK:      metrics.MetricDelta(rows, "order_amend_ok"),
		OrderCancelOK:     metrics.MetricDelta(rows, "order_cancel_ok"),
		OrderFilterCancel: metrics.MetricDelta(rows, "order_filter_cancel"),
		PositionOpened:    metrics.MetricDelta(rows, "position_opened"),
		PositionReset:     metrics.MetricDelta(rows, "position_reset"),
		PositionResetSL:   metrics.MetricDelta(rows, "position_reset_sl"),
		PositionResetTP:   metrics.MetricDelta(rows, "position_reset_tp"),
		PositionResetCancel: metrics.MetricDelta(rows, "position_reset_cancel"),
		PositionResetOther: metrics.MetricDelta(rows, "position_reset_other"),
		PositionResetLiquidation: metrics.MetricDelta(rows, "position_reset_liquidation"),
		PositionResetManual: metrics.MetricDelta(rows, "position_reset_manual"),
		AlgoPaused:        metrics.MetricDelta(rows, "algo_paused"),
		AlgoResumedOK:     metrics.MetricDelta(rows, "algo_resumed_ok"),
		OrderFailCreate:   metrics.MetricDelta(rows, "order_fail_create"),
		OrderFailAmend:    metrics.MetricDelta(rows, "order_fail_amend"),
		OrderFailCancel:   metrics.MetricDelta(rows, "order_fail_cancel"),
		OrderFailRet10003:  metrics.MetricDelta(rows, "order_fail_ret_10003"),
		OrderFailRet10006:  metrics.MetricDelta(rows, "order_fail_ret_10006"),
		OrderFailRet10001:  metrics.MetricDelta(rows, "order_fail_ret_10001"),
		OrderFailRet10016:  metrics.MetricDelta(rows, "order_fail_ret_10016"),
		OrderFailRet110001: metrics.MetricDelta(rows, "order_fail_ret_110001"),
		OrderFailRet110007: metrics.MetricDelta(rows, "order_fail_ret_110007"),
		OrderFailRet110013: metrics.MetricDelta(rows, "order_fail_ret_110013"),
		OrderFailRet110021: metrics.MetricDelta(rows, "order_fail_ret_110021"),
		OrderFailRet110090: metrics.MetricDelta(rows, "order_fail_ret_110090"),
		OrderFailRet110094: metrics.MetricDelta(rows, "order_fail_ret_110094"),
		OrderFailRet110059: metrics.MetricDelta(rows, "order_fail_ret_110059"),
		OrderFailRet110123: metrics.MetricDelta(rows, "order_fail_ret_110123"),
		OrderFailRet110126: metrics.MetricDelta(rows, "order_fail_ret_110126"),
		AlgoStoppedPermanent:      metrics.MetricDelta(rows, "algo_stopped_permanent"),
		RiskLimitMarginReduced:    metrics.MetricDelta(rows, "risk_limit_margin_reduced"),
		TPMissingCreateOK:         metrics.MetricDelta(rows, "tp_missing_create_ok"),
		TPMissingCreateFail:       metrics.MetricDelta(rows, "tp_missing_create_fail"),
		TPMissingClosePosition:    metrics.MetricDelta(rows, "tp_missing_close_position"),
		SLSetupStarted:            metrics.MetricDelta(rows, "sl_setup_started"),
		SLSetupOK:                 metrics.MetricDelta(rows, "sl_setup_ok"),
		SLSetupFail:               metrics.MetricDelta(rows, "sl_setup_fail"),
		SLSetupCancelled:          metrics.MetricDelta(rows, "sl_setup_cancelled"),
		SLPricePastImmediateClose: metrics.MetricDelta(rows, "sl_price_past_immediate_close"),
		PartialTPNeedsCancel:      metrics.MetricDelta(rows, "partial_tp_needs_cancel"),
		PriceWakeSignals:           metrics.MetricDelta(rows, "price_wake_signals"),
		PriceExecRuns:              metrics.MetricDelta(rows, "price_exec_runs"),
		PrivateOrderWakeSignals:    metrics.MetricDelta(rows, "private_order_wake_signals"),
		PrivateOrderEventsSignaled: metrics.MetricDelta(rows, "private_order_events_signaled"),
		PrivateOrderDrainBatches:   metrics.MetricDelta(rows, "private_order_drain_batches"),
		PrivateOrderEventsDrained:  metrics.MetricDelta(rows, "private_order_events_drained"),
		WSMessagesDelta:   metrics.MetricDelta(rows, "ws_messages"),
		WSMessagesOverall: last.WSMessages,
		FirstAlgoPaused:   first.AlgoPaused,
		FirstAlgoResumed:  first.AlgoResumedOK,
		LastAlgoPaused:    last.AlgoPaused,
		LastAlgoResumed:   last.AlgoResumedOK,
		ConnectionLostLast:    last.ConnectionLost,
		ReconnectFailedLast:   last.ReconnectFailed,
		ReconnectedLast:       last.Reconnected,
	}
	d.OrderFailTotal = d.OrderFailCreate + d.OrderFailAmend + d.OrderFailCancel
	d.FirstInFlight = d.FirstAlgoPaused - d.FirstAlgoResumed
	d.InFlight = d.LastAlgoPaused - d.LastAlgoResumed
	d.AllowMissing = d.InFlight - d.FirstInFlight
	d.RestingProxy = d.OrderCreateOK - d.PositionReset
	return d
}

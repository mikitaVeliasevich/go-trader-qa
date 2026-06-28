package analyze

import "github.com/dlisovsky/go-trader-qa/internal/metrics"

// Deltas holds first→last metric window values from metrics.tsv.
type Deltas struct {
	SampleCount int
	FirstElapsed int
	LastElapsed  int
	LastTimestamp string

	BusDrops           int64
	OrderCreateOK      int64
	OrderFilterCancel  int64
	PositionOpened     int64
	PositionReset      int64
	AlgoPaused         int64
	AlgoResumedOK      int64
	RestingProxy       int64

	WSMessagesDelta   int64
	WSMessagesOverall int64

	FirstAlgoPaused  int64
	FirstAlgoResumed int64
	LastAlgoPaused   int64
	LastAlgoResumed  int64
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
		OrderFilterCancel: metrics.MetricDelta(rows, "order_filter_cancel"),
		PositionOpened:    metrics.MetricDelta(rows, "position_opened"),
		PositionReset:     metrics.MetricDelta(rows, "position_reset"),
		AlgoPaused:        metrics.MetricDelta(rows, "algo_paused"),
		AlgoResumedOK:     metrics.MetricDelta(rows, "algo_resumed_ok"),
		WSMessagesDelta:   metrics.MetricDelta(rows, "ws_messages"),
		WSMessagesOverall: last.WSMessages,
		FirstAlgoPaused:   first.AlgoPaused,
		FirstAlgoResumed:  first.AlgoResumedOK,
		LastAlgoPaused:    last.AlgoPaused,
		LastAlgoResumed:   last.AlgoResumedOK,
	}
	d.RestingProxy = d.OrderCreateOK - d.PositionReset
	return d
}

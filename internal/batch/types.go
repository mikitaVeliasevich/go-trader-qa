package batch

import (
	"time"

	"github.com/dlisovsky/go-trader-qa/internal/metrics"
)

const (
	BatchPending   = "pending"
	BatchRunning   = "running"
	BatchComplete  = "complete"
	BatchFailed    = "failed"
	BatchCancelled = "cancelled"

	JobQueued  = "queued"
	JobRunning = "running"
	jobComplete = "complete"
	JobFailed  = "failed"
	JobSkipped = "skipped"

	overallPass    = "PASS"
	overallFail    = "FAIL"
	overallUnknown = "UNKNOWN"
)

const jobStagger = 30 * time.Second

const (
	ModeSoak    = "soak"
	ModeAnalyze = "analyze"
)

// SoakBatch describes one multi-server soak run.
type SoakBatch struct {
	ID             string     `json:"id"`
	Mode           string     `json:"mode,omitempty"`
	Window         string     `json:"window,omitempty"`
	ServerIDs      []int      `json:"server_ids"`
	Duration       string     `json:"duration"`
	Interval       string     `json:"interval"`
	Concurrency    int        `json:"concurrency"`
	Profile        string     `json:"profile"`
	SkipIneligible bool       `json:"skip_ineligible"`
	Status         string     `json:"status"`
	Dir            string     `json:"dir"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// SoakJob is one server job within a batch.
type SoakJob struct {
	BatchID      string `json:"batch_id"`
	ServerID     int    `json:"server_id"`
	PairID       string `json:"pair_id,omitempty"`
	RunDir       string `json:"run_dir,omitempty"`
	Status       string `json:"status"`
	SkipReason   string `json:"skip_reason,omitempty"`
	Overall      string `json:"overall,omitempty"`
	Samples      int    `json:"samples,omitempty"`
	LastBusDrops int64  `json:"last_bus_drops,omitempty"`
	Error        string `json:"error,omitempty"`
}

// BatchSpec configures a batch run.
type BatchSpec struct {
	BatchID        string
	Mode           string
	Window         metrics.WindowSpec
	ServerIDs      []int
	Duration       time.Duration
	Interval       time.Duration
	Concurrency    int
	Profile        string
	SkipIneligible bool
	Analyze        bool
	ArtifactsDir   string
	AccountID      int
}

// BatchResult is the final outcome of Run.
type BatchResult struct {
	Batch   SoakBatch
	Jobs    []SoakJob
	Summary BatchSummary
}

// BatchSummary is persisted as batch-summary.json.
type BatchSummary struct {
	Batch   SoakBatch `json:"batch"`
	Jobs    []SoakJob `json:"jobs"`
	Pass    int       `json:"pass_count"`
	Fail    int       `json:"fail_count"`
	Skipped int       `json:"skipped_count"`
	Unknown int       `json:"unknown_count,omitempty"`
}

// Progress is a live snapshot during Run.
type Progress struct {
	Batch   SoakBatch
	Jobs    []SoakJob
	Running bool
}

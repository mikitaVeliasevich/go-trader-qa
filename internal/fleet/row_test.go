package fleet

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dlisovsky/go-trader-qa/internal/manager"
)

func TestBuildRowsFromFixture(t *testing.T) {
	data, err := os.ReadFile("../manager/testdata/subs_staging.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var resp manager.SubsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	rows := BuildRows(&resp, resp.Categories)
	byID := map[int]FleetRow{}
	for _, row := range rows {
		byID[row.DBSubaccountID] = row
	}

	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}

	sub1 := byID[1]
	if sub1.QAEligible || sub1.IneligibleReason != "no_server" {
		t.Errorf("sub 1: eligible=%v reason=%q, want ineligible no_server", sub1.QAEligible, sub1.IneligibleReason)
	}
	if sub1.PairID != "" {
		t.Errorf("sub 1 pair_id = %q, want empty", sub1.PairID)
	}

	sub2 := byID[2]
	if sub2.QAEligible || sub2.IneligibleReason != "app_inactive" {
		t.Errorf("sub 2: eligible=%v reason=%q, want ineligible app_inactive", sub2.QAEligible, sub2.IneligibleReason)
	}
	if sub2.PairID != "200002@23" {
		t.Errorf("sub 2 pair_id = %q, want 200002@23", sub2.PairID)
	}

	sub7 := byID[7]
	if !sub7.QAEligible || sub7.IneligibleReason != "" {
		t.Errorf("sub 7: eligible=%v reason=%q, want eligible with empty reason", sub7.QAEligible, sub7.IneligibleReason)
	}
	if sub7.PairID != "514690847@11" {
		t.Errorf("sub 7 pair_id = %q, want 514690847@11", sub7.PairID)
	}
	if sub7.DeployedImageHash != "8659e4e089e7" {
		t.Errorf("sub 7 image_hash = %q", sub7.DeployedImageHash)
	}
	if len(sub7.Categories) != 1 || sub7.Categories[0] != "dev subs" {
		t.Errorf("sub 7 categories = %v, want [dev subs]", sub7.Categories)
	}
}

func TestEligibilityMatrix(t *testing.T) {
	tests := []struct {
		name     string
		sync     manager.SyncDataItem
		eligible bool
		reason   string
	}{
		{
			name:     "no_server missing server",
			sync:     manager.SyncDataItem{HasServer: false, ServerID: nil, AppIsActive: nil},
			eligible: false,
			reason:   "no_server",
		},
		{
			name:     "no_server zero server_id",
			sync:     manager.SyncDataItem{HasServer: true, ServerID: intPtr(0), AppIsActive: boolPtr(true)},
			eligible: false,
			reason:   "no_server",
		},
		{
			name:     "app_not_deployed",
			sync:     manager.SyncDataItem{HasServer: true, ServerID: intPtr(11), AppIsActive: nil},
			eligible: false,
			reason:   "app_not_deployed",
		},
		{
			name:     "app_inactive",
			sync:     manager.SyncDataItem{HasServer: true, ServerID: intPtr(23), AppIsActive: boolPtr(false)},
			eligible: false,
			reason:   "app_inactive",
		},
		{
			name:     "eligible",
			sync:     manager.SyncDataItem{HasServer: true, ServerID: intPtr(11), AppIsActive: boolPtr(true)},
			eligible: true,
			reason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEligible, gotReason := eligibility(tt.sync)
			if gotEligible != tt.eligible || gotReason != tt.reason {
				t.Errorf("eligibility() = (%v, %q), want (%v, %q)", gotEligible, gotReason, tt.eligible, tt.reason)
			}
		})
	}
}

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }

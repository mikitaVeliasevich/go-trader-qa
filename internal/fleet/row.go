package fleet

import (
	"fmt"
	"strconv"

	"github.com/dlisovsky/go-trader-qa/internal/manager"
)

// FleetRow is a normalized subaccount row for fleet display and soak selection.
type FleetRow struct {
	PairID            string   `json:"pair_id"`
	DBSubaccountID    int      `json:"db_subaccount_id"`
	UID               int64    `json:"uid"`
	Username          string   `json:"username"`
	ServerID          int      `json:"server_id"`
	DeployedImageHash string   `json:"deployed_image_hash"`
	QAEligible        bool     `json:"qa_eligible"`
	IneligibleReason  string   `json:"ineligible_reason"`
	Categories        []string `json:"categories"`
}

// BuildRows joins db_subs with sync_data and applies eligibility rules.
func BuildRows(resp *manager.SubsResponse, categories map[string][]manager.Category) []FleetRow {
	if resp == nil {
		return nil
	}
	if categories == nil {
		categories = resp.Categories
	}

	rows := make([]FleetRow, 0, len(resp.DBSubs))
	for _, sub := range resp.DBSubs {
		syncKey := strconv.Itoa(sub.ID)
		sync, hasSync := resp.SyncData[syncKey]

		row := FleetRow{
			DBSubaccountID: sub.ID,
			UID:            sub.UID,
			Username:       sub.Username,
		}

		if cats, ok := categories[syncKey]; ok {
			for _, c := range cats {
				row.Categories = append(row.Categories, c.Name)
			}
		}

		if hasSync {
			row.DeployedImageHash = sync.DeployedImageHash
			if sync.ServerID != nil {
				row.ServerID = *sync.ServerID
			}
			row.QAEligible, row.IneligibleReason = eligibility(sync)
			if row.ServerID > 0 {
				row.PairID = fmt.Sprintf("%d@%d", sub.UID, row.ServerID)
			}
		} else {
			row.IneligibleReason = "no_server"
		}

		rows = append(rows, row)
	}
	return rows
}

func eligibility(sync manager.SyncDataItem) (eligible bool, reason string) {
	serverID := 0
	if sync.ServerID != nil {
		serverID = *sync.ServerID
	}

	if !sync.HasServer || serverID == 0 {
		return false, "no_server"
	}

	if sync.AppIsActive == nil {
		return false, "app_not_deployed"
	}
	if !*sync.AppIsActive {
		return false, "app_inactive"
	}

	return true, ""
}

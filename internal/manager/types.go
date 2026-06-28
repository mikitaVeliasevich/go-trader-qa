package manager

import "encoding/json"

// SubsResponse is the JSON body from GET /exchange/accounts/{id}/subs.
type SubsResponse struct {
	OK       bool                    `json:"ok"`
	DBSubs   []DBSub                 `json:"db_subs"`
	SyncData map[string]SyncDataItem `json:"sync_data"`
	Categories map[string][]Category `json:"categories"`
}

// DBSub is a subaccount record from the exchange account.
type DBSub struct {
	ID         int    `json:"id"`
	UID        int64  `json:"uid"`
	ExternalID string `json:"external_id"`
	Username   string `json:"username"`
	Exchange   string `json:"exchange"`
}

// SyncDataItem holds provision state for a subaccount keyed by DB id.
type SyncDataItem struct {
	SubaccountID      int    `json:"subaccount_id"`
	HasServer         bool   `json:"has_server"`
	ServerID          *int   `json:"server_id"`
	AppIsActive       *bool  `json:"app_is_active"`
	DeployedImageHash string `json:"deployed_image_hash"`
}

// Category is a tag assigned to a subaccount.
type Category struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// BotStatus is the JSON body from GET /provision/servers/{id}/status.
type BotStatus struct {
	Status       string          `json:"status"`
	Running      bool            `json:"running"`
	Initializing bool            `json:"initializing"`
	Initialized  bool            `json:"initialized"`
	HasAPIKeys   bool            `json:"hasApiKeys"`
	Algorithms   json.RawMessage `json:"algorithms"`
	LoggerLevel  string          `json:"loggerLevel"`
}

// BotConfig is the JSON body from GET /provision/servers/{id}/config.
type BotConfig struct {
	Algorithms  json.RawMessage `json:"algorithms"`
	LoggerLevel string          `json:"logger_level"`
}

// LogsResponse is the JSON body from GET /provision/servers/{id}/logs.
type LogsResponse struct {
	ServerID int    `json:"server_id"`
	Logs     string `json:"logs"`
	Success  bool   `json:"success"`
}

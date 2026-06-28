package manager

import (
	"context"
	"encoding/json"
	"fmt"
)

func provisionPath(serverID int, suffix string) string {
	return fmt.Sprintf("/provision/servers/%d/%s", serverID, suffix)
}

// ServerStatus fetches bot status via GET /provision/servers/{id}/status.
func (c *Client) ServerStatus(ctx context.Context, serverID int) (*BotStatus, error) {
	path := provisionPath(serverID, "status")
	var st BotStatus
	if err := c.getJSON(ctx, path, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// ServerConfig fetches bot config via GET /provision/servers/{id}/config.
func (c *Client) ServerConfig(ctx context.Context, serverID int) (*BotConfig, error) {
	path := provisionPath(serverID, "config")
	var cfg BotConfig
	if err := c.getJSON(ctx, path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ServerLogs fetches bot logs via GET /provision/servers/{id}/logs?tail=N.
func (c *Client) ServerLogs(ctx context.Context, serverID int, tail int) (string, error) {
	path := fmt.Sprintf("/provision/servers/%d/logs?tail=%d", serverID, tail)
	var resp LogsResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return "", err
	}
	return resp.Logs, nil
}

// ServerDebugVars fetches expvar metrics via GET /provision/servers/{id}/debug/vars.
func (c *Client) ServerDebugVars(ctx context.Context, serverID int) (map[string]json.RawMessage, error) {
	path := provisionPath(serverID, "debug/vars")
	var vars map[string]json.RawMessage
	if err := c.getJSON(ctx, path, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerLogsUnwrapsLogsField(t *testing.T) {
	const wantLogs = "line one\nline two\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provision/servers/11/logs" {
			t.Errorf("path = %q, want /provision/servers/11/logs", r.URL.Path)
		}
		if got := r.URL.Query().Get("tail"); got != "5" {
			t.Errorf("tail = %q, want 5", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LogsResponse{
			ServerID: 11,
			Logs:     wantLogs,
			Success:  true,
		})
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, "token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	logs, err := client.ServerLogs(context.Background(), 11, 5)
	if err != nil {
		t.Fatalf("ServerLogs: %v", err)
	}
	if logs != wantLogs {
		t.Errorf("logs = %q, want %q", logs, wantLogs)
	}
}

func TestProvisionMethodsHitCorrectPaths(t *testing.T) {
	const serverID = 42
	wantPaths := []string{
		"/provision/servers/42/status",
		"/provision/servers/42/config",
		"/provision/servers/42/logs",
		"/provision/servers/42/debug/vars",
	}
	gotPaths := make([]string, 0, len(wantPaths))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/provision/servers/42/status":
			_, _ = w.Write([]byte(`{"status":"running","running":true,"initializing":false,"initialized":true,"hasApiKeys":true,"algorithms":[],"loggerLevel":"info"}`))
		case "/provision/servers/42/config":
			_, _ = w.Write([]byte(`{"algorithms":[],"logger_level":"info"}`))
		case "/provision/servers/42/logs":
			if got := r.URL.Query().Get("tail"); got != "10" {
				t.Errorf("logs tail = %q, want 10", got)
			}
			_, _ = w.Write([]byte(`{"server_id":42,"logs":"ok","success":true}`))
		case "/provision/servers/42/debug/vars":
			_, _ = w.Write([]byte(`{"bus_drops":"0","ws_messages_received":"1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, "token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()

	if _, err := client.ServerStatus(ctx, serverID); err != nil {
		t.Fatalf("ServerStatus: %v", err)
	}
	if _, err := client.ServerConfig(ctx, serverID); err != nil {
		t.Fatalf("ServerConfig: %v", err)
	}
	if _, err := client.ServerLogs(ctx, serverID, 10); err != nil {
		t.Fatalf("ServerLogs: %v", err)
	}
	if _, err := client.ServerDebugVars(ctx, serverID); err != nil {
		t.Fatalf("ServerDebugVars: %v", err)
	}

	if len(gotPaths) != len(wantPaths) {
		t.Fatalf("got %d requests, want %d: %v", len(gotPaths), len(wantPaths), gotPaths)
	}
	for i, want := range wantPaths {
		if gotPaths[i] != want {
			t.Errorf("request[%d] path = %q, want %q", i, gotPaths[i], want)
		}
	}
}

package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRequestHeaders(t *testing.T) {
	const token = "test-jwt-token"

	var gotAuth, gotReferer, gotUserAgent, gotAccept, gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("Referer")
		gotUserAgent = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"db_subs":[],"sync_data":{}}`))
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL+"/api", token)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.FleetSubs(context.Background(), 1); err != nil {
		t.Fatalf("FleetSubs: %v", err)
	}

	if gotAuth != "Bearer "+token {
		t.Errorf("Authorization = %q, want Bearer %q", gotAuth, token)
	}
	if gotReferer != srv.URL+"/" {
		t.Errorf("Referer = %q, want %q", gotReferer, srv.URL+"/")
	}
	if gotUserAgent != userAgent {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, userAgent)
	}
	if gotAccept != "*/*" {
		t.Errorf("Accept = %q, want */*", gotAccept)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

func TestClientRetriesOn502(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"db_subs":[],"sync_data":{}}`))
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, "token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.FleetSubs(context.Background(), 1); err != nil {
		t.Fatalf("FleetSubs after retry: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	userAgent      = "go-trader-qa/1.0"
)

// Client performs authenticated HTTP calls to the Manager API.
type Client struct {
	baseURL    string
	token      string
	referer    string
	httpClient *http.Client
}

// NewClient builds a Manager client with required auth headers.
func NewClient(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("bearer token is required")
	}

	referer, err := refererFromBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		referer: referer,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func refererFromBaseURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse MANAGER_API_BASE_URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("MANAGER_API_BASE_URL must include scheme and host")
	}
	return u.Scheme + "://" + u.Host + "/", nil
}

func (c *Client) do(ctx context.Context, method, path string) ([]byte, int, error) {
	fullURL := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
		if err != nil {
			return nil, 0, err
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}

		if (resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout) && attempt == 0 {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return body, resp.StatusCode, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, path, truncateBody(body))
		}

		return body, resp.StatusCode, nil
	}

	return nil, 0, lastErr
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", c.referer)
	req.Header.Set("User-Agent", userAgent)
}

// Get performs an authenticated GET and returns the HTTP status and response body.
func (c *Client) Get(ctx context.Context, path string) (int, []byte, error) {
	body, code, err := c.do(ctx, http.MethodGet, path)
	return code, body, err
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	body, _, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func truncateBody(body []byte) string {
	const max = 200
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

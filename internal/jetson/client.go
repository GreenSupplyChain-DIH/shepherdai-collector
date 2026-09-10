package jetson

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
)

type Client struct {
	baseURL    string
	endpoint   string
	apiKeyName string
	apiKey     string
	httpClient *http.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		baseURL:    cfg.JetsonBaseURL,
		endpoint:   cfg.JetsonEndpoint,
		apiKeyName: cfg.JetsonAPIKeyName,
		apiKey:     cfg.JetsonAPIKey,
		httpClient: &http.Client{Timeout: cfg.RequestTimeout},
	}
}

func (c *Client) Fetch(ctx context.Context) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+c.endpoint, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set(c.apiKeyName, c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("jetson returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err == nil {
		return list, nil
	}

	var wrapped struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Data != nil {
		return wrapped.Data, nil
	}

	return nil, fmt.Errorf("unsupported Jetson payload shape at %s", time.Now().Format(time.RFC3339))
}

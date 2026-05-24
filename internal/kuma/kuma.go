package kuma

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	pushURL    string
	httpClient *http.Client
}

func NewClient(pushURL string, timeout time.Duration) *Client {
	return &Client{
		pushURL:    pushURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Push(ctx context.Context, status string, message string, ping time.Duration) error {
	endpoint, err := url.Parse(c.pushURL)
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("status", status)
	query.Set("msg", message)
	if ping > 0 {
		query.Set("ping", fmt.Sprintf("%d", ping.Milliseconds()))
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("kuma returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

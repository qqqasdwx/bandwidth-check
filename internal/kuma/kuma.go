package kuma

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	pushURL    string
	httpClient *http.Client
}

type PushResult struct {
	StatusCode int
	Body       string
}

func NewClient(pushURL string, timeout time.Duration) *Client {
	return &Client{
		pushURL:    pushURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Push(ctx context.Context, status string, message string, ping time.Duration) (PushResult, error) {
	endpoint, err := url.Parse(c.pushURL)
	if err != nil {
		return PushResult{}, err
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
		return PushResult{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PushResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	result := PushResult{
		StatusCode: resp.StatusCode,
		Body:       sanitizeKumaBody(string(body)),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("Kuma 返回 HTTP %d: %s", resp.StatusCode, result.Body)
	}
	return result, nil
}

func sanitizeKumaBody(body string) string {
	text := strings.TrimSpace(body)
	if text == "" {
		return ""
	}
	const marker = "/push/"
	start := strings.Index(text, marker)
	if start < 0 {
		return text
	}
	tokenStart := start + len(marker)
	tokenEnd := tokenStart
	for tokenEnd < len(text) {
		switch text[tokenEnd] {
		case '?', '#', '/', ' ', '\t', '\r', '\n':
			if tokenEnd == tokenStart {
				return text
			}
			return text[:tokenStart] + "***" + text[tokenEnd:]
		}
		tokenEnd++
	}
	if tokenEnd == tokenStart {
		return text
	}
	return text[:tokenStart] + "***" + text[tokenEnd:]
}

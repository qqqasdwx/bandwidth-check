package zte

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL      *url.URL
	username     string
	password     string
	sessionToken string
	loggedIn     bool
	httpClient   *http.Client
}

func NewClient(routerURL, username, password string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(routerURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("router URL must include scheme and host")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:    parsed,
		username:   strings.TrimSpace(username),
		password:   password,
		httpClient: &http.Client{Jar: jar, Timeout: timeout},
	}, nil
}

func (c *Client) WANPortStatus(ctx context.Context, alias string) (PortStatus, error) {
	if !c.loggedIn {
		if err := c.Login(ctx); err != nil {
			return PortStatus{}, err
		}
	}

	body, err := c.get(ctx, "vueData", "vue_internet_ethport_data", nil)
	if err != nil {
		return PortStatus{}, err
	}
	if isSessionTimeout(body) {
		c.loggedIn = false
		if err := c.Login(ctx); err != nil {
			return PortStatus{}, err
		}
		body, err = c.get(ctx, "vueData", "vue_internet_ethport_data", nil)
		if err != nil {
			return PortStatus{}, err
		}
	}
	if isSessionTimeout(body) {
		return PortStatus{}, fmt.Errorf("router session timed out")
	}

	ports, err := ParseEthernetPorts(body)
	if err != nil {
		return PortStatus{}, err
	}
	return FindWANPort(ports, alias)
}

func (c *Client) Login(ctx context.Context) error {
	username := c.username
	if username == "" {
		initial, err := c.getRaw(ctx, "hiddenScene", "initial_info_json", nil, false)
		if err != nil {
			return fmt.Errorf("get initial info: %w", err)
		}
		username = parseInitialUsername(initial)
		if username == "" {
			username = "admin"
		}
	}

	tokenBody, err := c.getRaw(ctx, "loginsceneData", "login_token_json", nil, false)
	if err != nil {
		return fmt.Errorf("get login token: %w", err)
	}
	var token loginTokenResponse
	if err := json.Unmarshal(tokenBody, &token); err != nil {
		return fmt.Errorf("parse login token: %w", err)
	}
	if token.LoginToken == "" || token.SessionToken == "" {
		return fmt.Errorf("router returned an incomplete login token")
	}

	c.sessionToken = token.SessionToken
	passwordHash := sha256.Sum256([]byte(c.password + token.LoginToken))
	form := url.Values{
		"Username":       {username},
		"Password":       {hex.EncodeToString(passwordHash[:])},
		"action":         {"login"},
		"Frm_Logintoken": {""},
		"captchaCode":    {""},
		"_sessionTOKEN":  {c.sessionToken},
	}

	loginBody, err := c.post(ctx, "loginData", "login_entry", form)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	var login loginResponse
	if err := json.Unmarshal(loginBody, &login); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}
	if login.SessionToken != "" {
		c.sessionToken = login.SessionToken
	}
	if !login.LoginNeedRefresh {
		message := firstNonEmpty(login.LoginErrMsg, login.PromptMsg, "login failed")
		return fmt.Errorf("router login failed: %s", message)
	}

	c.username = username
	c.loggedIn = true
	return nil
}

func (c *Client) get(ctx context.Context, dataType, tag string, extra url.Values) ([]byte, error) {
	return c.getRaw(ctx, dataType, tag, extra, true)
}

func (c *Client) getRaw(ctx context.Context, dataType, tag string, extra url.Values, cacheBust bool) ([]byte, error) {
	query := orderedRouterQuery(dataType, tag, extra, cacheBust)
	return c.do(ctx, http.MethodGet, query, nil)
}

func (c *Client) post(ctx context.Context, dataType, tag string, form url.Values) ([]byte, error) {
	query := orderedRouterQuery(dataType, tag, nil, false)
	return c.do(ctx, http.MethodPost, query, form)
}

func (c *Client) do(ctx context.Context, method string, query string, form url.Values) ([]byte, error) {
	endpoint := *c.baseURL
	endpoint.Path = "/"
	endpoint.RawQuery = query

	var body io.Reader
	if form != nil {
		body = bytes.NewBufferString(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if token := resp.Header.Get("X_XSRF_TOKEN"); token != "" {
		c.sessionToken = token
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("router returned HTTP %d", resp.StatusCode)
	}
	return respBody, nil
}

func parseInitialUsername(body []byte) string {
	var initial initialInfoResponse
	if err := json.Unmarshal(body, &initial); err != nil {
		return ""
	}
	return strings.TrimSpace(initial.Data.User)
}

func isSessionTimeout(body []byte) bool {
	text := string(body)
	return strings.Contains(text, "<IF_ERRORSTR>SessionTimeout</IF_ERRORSTR>") ||
		strings.Contains(text, "SessionTimeout")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func orderedRouterQuery(dataType, tag string, extra url.Values, cacheBust bool) string {
	parts := []string{
		"_type=" + url.QueryEscape(dataType),
		"_tag=" + url.QueryEscape(tag),
	}
	for key, vals := range extra {
		for _, val := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(val))
		}
	}
	if cacheBust {
		parts = append(parts, "_="+fmt.Sprintf("%d", time.Now().UnixMilli()))
	}
	return strings.Join(parts, "&")
}

type initialInfoResponse struct {
	Data struct {
		User string `json:"user"`
	} `json:"data"`
}

type loginTokenResponse struct {
	SessionToken string `json:"_sessionToken"`
	LoginToken   string `json:"logintoken"`
}

type loginResponse struct {
	LoginNeedRefresh bool   `json:"login_need_refresh"`
	LoginErrMsg      string `json:"loginErrMsg"`
	PromptMsg        string `json:"promptMsg"`
	SessionToken     string `json:"sess_token"`
}

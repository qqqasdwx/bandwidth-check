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

type WANPortResult struct {
	Port                PortStatus
	AvailablePorts      []PortStatus
	PortMatchMethod     string
	SessionReused       bool
	LoginAttempted      bool
	InitialLogin        bool
	RetriedAfterTimeout bool
	ResponseBytes       int
	Stage               string
	Events              []string
}

func NewClient(routerURL, username, password string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(routerURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("路由器地址必须包含协议和主机")
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

func (c *Client) WANPortStatus(ctx context.Context, alias string) (WANPortResult, error) {
	result := WANPortResult{
		SessionReused: c.loggedIn,
		Stage:         "检查会话",
	}
	result.addEvent("进入检查，已有会话=%t", c.loggedIn)
	if !c.loggedIn {
		result.Stage = "登录路由器"
		result.LoginAttempted = true
		result.InitialLogin = true
		result.addEvent("没有可复用会话，开始登录")
		if err := c.Login(ctx); err != nil {
			result.addEvent("登录失败: %v", err)
			return result, fmt.Errorf("登录路由器失败: %w", err)
		}
		result.addEvent("登录成功")
	} else {
		result.addEvent("复用已有会话，跳过登录")
	}

	result.Stage = "读取网口数据"
	result.addEvent("请求路由器网口数据")
	body, err := c.get(ctx, "vueData", "vue_internet_ethport_data", nil)
	if err != nil {
		result.addEvent("读取网口数据失败: %v", err)
		return result, fmt.Errorf("读取网口数据失败: %w", err)
	}
	result.ResponseBytes = len(body)
	result.addEvent("收到网口数据，响应大小=%d字节", len(body))
	if isSessionTimeout(body) {
		result.Stage = "会话过期后重新登录"
		c.loggedIn = false
		result.RetriedAfterTimeout = true
		result.LoginAttempted = true
		result.InitialLogin = false
		result.addEvent("路由器返回 SessionTimeout，清除会话并重新登录")
		if err := c.Login(ctx); err != nil {
			result.addEvent("重新登录失败: %v", err)
			return result, fmt.Errorf("会话过期后重新登录失败: %w", err)
		}
		result.addEvent("重新登录成功")
		result.Stage = "重新读取网口数据"
		body, err = c.get(ctx, "vueData", "vue_internet_ethport_data", nil)
		if err != nil {
			result.addEvent("重新读取网口数据失败: %v", err)
			return result, fmt.Errorf("重新读取网口数据失败: %w", err)
		}
		result.ResponseBytes = len(body)
		result.addEvent("重新收到网口数据，响应大小=%d字节", len(body))
	}
	if isSessionTimeout(body) {
		result.Stage = "会话仍然超时"
		result.addEvent("重新登录后仍然返回 SessionTimeout")
		return result, fmt.Errorf("路由器会话超时")
	}

	result.Stage = "解析网口数据"
	ports, err := ParseEthernetPorts(body)
	if err != nil {
		result.addEvent("解析网口数据失败: %v", err)
		return result, fmt.Errorf("解析网口数据失败: %w", err)
	}
	result.AvailablePorts = ports
	result.addEvent("解析到 %d 个网口", len(ports))
	result.Stage = "选择 WAN 网口"
	match, err := FindWANPortMatch(ports, alias)
	if err != nil {
		result.addEvent("选择目标网口失败: %v", err)
		return result, fmt.Errorf("选择目标网口失败: %w", err)
	}
	result.Port = match.Port
	result.PortMatchMethod = match.Method
	result.Stage = "完成"
	result.addEvent("命中目标网口: 匹配方式=%s, %s", match.Method, match.Port.Summary())
	return result, nil
}

func (r *WANPortResult) addEvent(format string, args ...any) {
	r.Events = append(r.Events, fmt.Sprintf(format, args...))
}

func (c *Client) Login(ctx context.Context) error {
	username := c.username
	if username == "" {
		initial, err := c.getRaw(ctx, "hiddenScene", "initial_info_json", nil, false)
		if err != nil {
			return fmt.Errorf("获取初始用户信息失败: %w", err)
		}
		username = parseInitialUsername(initial)
		if username == "" {
			username = "admin"
		}
	}

	tokenBody, err := c.getRaw(ctx, "loginsceneData", "login_token_json", nil, false)
	if err != nil {
		return fmt.Errorf("获取登录 token 失败: %w", err)
	}
	var token loginTokenResponse
	if err := json.Unmarshal(tokenBody, &token); err != nil {
		return fmt.Errorf("解析登录 token 失败: %w", err)
	}
	if token.LoginToken == "" || token.SessionToken == "" {
		return fmt.Errorf("路由器返回的登录 token 不完整")
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
		return fmt.Errorf("提交登录请求失败: %w", err)
	}
	var login loginResponse
	if err := json.Unmarshal(loginBody, &login); err != nil {
		return fmt.Errorf("解析登录响应失败: %w", err)
	}
	if login.SessionToken != "" {
		c.sessionToken = login.SessionToken
	}
	if !login.LoginNeedRefresh {
		message := firstNonEmpty(login.LoginErrMsg, login.PromptMsg, "登录失败")
		return fmt.Errorf("路由器登录失败: %s", message)
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
		return nil, fmt.Errorf("%s ?%s 请求失败: %w", method, query, err)
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
		return nil, fmt.Errorf("%s ?%s 返回 HTTP %d，响应片段=%q", method, query, resp.StatusCode, responseSnippet(respBody))
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

func responseSnippet(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 256 {
		return text[:256]
	}
	return text
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

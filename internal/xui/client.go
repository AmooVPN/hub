package xui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type XUIClient struct {
	PanelID    int64
	BaseURL    string
	Username   string
	Password   string
	APIToken   string
	UserAgent  string
	HTTPClient *http.Client
	compat     XUICompatibility
	loggedIn   bool
}

func NewClient(panelID int64, baseURL, username, password, apiToken string) *XUIClient {
	jar, _ := cookiejar.New(nil)
	return &XUIClient{
		PanelID:   panelID,
		BaseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Username:  username,
		Password:  password,
		APIToken:  apiToken,
		UserAgent: "hub/1.0",
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
			Jar:     jar,
		},
		compat: XUICompatibility{
			SupportsClientEnableDisable: true,
			SupportsTrafficReset:        true,
			SupportsOnlineUsers:         true,
			SupportsSubscriptionAPI:     true,
		},
	}
}

func (c *XUIClient) SetUserAgent(userAgent string) { c.UserAgent = userAgent }

func (c *XUIClient) Login(ctx context.Context) error {
	if strings.TrimSpace(c.APIToken) != "" {
		c.loggedIn = true
		return nil
	}
	form := url.Values{}
	form.Set("username", c.Username)
	form.Set("password", c.Password)
	resp, err := c.do(ctx, http.MethodPost, "/login", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &XUIError{PanelID: c.PanelID, Operation: "login", StatusCode: resp.StatusCode, Message: ErrXUIUnauthorized.Error(), Cause: ErrXUIUnauthorized}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &XUIError{PanelID: c.PanelID, Operation: "login", StatusCode: resp.StatusCode, Message: ErrXUIBadResponse.Error(), Cause: ErrXUIBadResponse}
	}
	c.loggedIn = true
	return nil
}

func (c *XUIClient) ListInbounds(ctx context.Context) ([]XUIInbound, error) {
	var payload struct {
		Data []XUIInbound `json:"data"`
	}
	if err := c.getJSON(ctx, "/panel/api/inbounds/list", &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (c *XUIClient) AddClient(ctx context.Context, inboundID int, req XUIClientCreateRequest) (*XUIClientResult, error) {
	var result XUIClientResult
	path := fmt.Sprintf("/panel/api/inbounds/%d/client/add", inboundID)
	if err := c.postJSON(ctx, path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *XUIClient) UpdateClient(ctx context.Context, inboundID int, clientID string, req XUIClientUpdateRequest) error {
	path := fmt.Sprintf("/panel/api/inbounds/%d/client/%s", inboundID, clientID)
	return c.putJSON(ctx, path, req, nil)
}

func (c *XUIClient) DeleteClient(ctx context.Context, inboundID int, clientID string) error {
	path := fmt.Sprintf("/panel/api/inbounds/%d/client/%s", inboundID, clientID)
	return c.deleteJSON(ctx, path, nil)
}

func (c *XUIClient) ResetClientTraffic(ctx context.Context, inboundID int, clientID string) error {
	path := fmt.Sprintf("/panel/api/inbounds/%d/client/%s/traffic/reset", inboundID, clientID)
	return c.postJSON(ctx, path, nil, nil)
}

func (c *XUIClient) GetClientTraffic(ctx context.Context, clientID string) (*XUITraffic, error) {
	var traffic XUITraffic
	if err := c.getJSON(ctx, "/panel/api/client/traffic/"+clientID, &traffic); err != nil {
		return nil, err
	}
	return &traffic, nil
}

func (c *XUIClient) Compatibility() XUICompatibility { return c.compat }

func (c *XUIClient) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if token := strings.TrimSpace(c.APIToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.HTTPClient.Do(req)
}

func (c *XUIClient) getJSON(ctx context.Context, path string, out any) error {
	return c.requestJSON(ctx, http.MethodGet, path, nil, out)
}

func (c *XUIClient) postJSON(ctx context.Context, path string, in any, out any) error {
	return c.requestJSON(ctx, http.MethodPost, path, in, out)
}

func (c *XUIClient) putJSON(ctx context.Context, path string, in any, out any) error {
	return c.requestJSON(ctx, http.MethodPut, path, in, out)
}

func (c *XUIClient) deleteJSON(ctx context.Context, path string, out any) error {
	return c.requestJSON(ctx, http.MethodDelete, path, nil, out)
}

func (c *XUIClient) requestJSON(ctx context.Context, method, path string, in any, out any) error {
	var payload []byte
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return &XUIError{PanelID: c.PanelID, Operation: method + " " + path, Message: ErrXUIBadResponse.Error(), Cause: err}
		}
		payload = buf
	}
	doRequest := func() (*http.Response, error) {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		return c.do(ctx, method, path, body, "application/json")
	}
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := doRequest()
		if err != nil {
			return normalizeHTTPError(c.PanelID, method+" "+path, err)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			if attempt == 0 && c.loggedIn {
				_ = resp.Body.Close()
				if err := c.Login(ctx); err == nil {
					continue
				}
			}
			defer resp.Body.Close()
			return normalizeStatusError(c.PanelID, method+" "+path, resp.StatusCode)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return normalizeStatusError(c.PanelID, method+" "+path, resp.StatusCode)
		}
		if out == nil {
			return nil
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return ErrXUIBadResponse
}

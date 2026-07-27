package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	defaultContextPath   = "/nacos"
	maxResponseBytes     = 4 << 20
	adminClientUserAgent = "AegisCore-Nacos-Config-Seed"
)

type seedOptions struct {
	Addr      string
	Namespace string
	Group     string
	ConfigDir string
	DataIDs   []string
	Timeout   time.Duration
	Username  string
	Password  string
}

func (o seedOptions) Validate() error {
	if strings.TrimSpace(o.Addr) == "" {
		return fmt.Errorf("addr is required")
	}
	if strings.TrimSpace(o.Namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(o.Group) == "" {
		return fmt.Errorf("group is required")
	}
	if strings.TrimSpace(o.ConfigDir) == "" {
		return fmt.Errorf("config-dir is required")
	}
	if len(o.DataIDs) == 0 {
		return fmt.Errorf("at least one data-id is required")
	}
	if (strings.TrimSpace(o.Username) != "") != (strings.TrimSpace(o.Password) != "") {
		return fmt.Errorf("username and password must be set together")
	}
	return nil
}

type adminClient struct {
	server *url.URL
	client *http.Client
	token  string
}

type apiResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *T     `json:"data"`
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
}

func newAdminClient(options seedOptions) (*adminClient, error) {
	server, err := parseServerURL(options.Addr)
	if err != nil {
		return nil, err
	}
	client := &adminClient{
		server: server,
		client: &http.Client{Timeout: options.Timeout},
	}
	return client, nil
}

func (c *adminClient) Seed(ctx context.Context, options seedOptions, documents map[string][]byte) error {
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	if options.Username != "" {
		token, err := c.login(ctx, options.Username, options.Password)
		if err != nil {
			return fmt.Errorf("login to Nacos: %w", err)
		}
		c.token = token
	}
	exists, err := c.namespaceExists(ctx, options.Namespace)
	if err != nil {
		return fmt.Errorf("check namespace %s: %w", options.Namespace, err)
	}
	if !exists {
		if err := c.createNamespace(ctx, options.Namespace); err != nil {
			return fmt.Errorf("create namespace %s: %w", options.Namespace, err)
		}
	}
	for _, dataID := range options.DataIDs {
		if err := c.publishConfig(ctx, options.Namespace, options.Group, dataID, documents[dataID]); err != nil {
			return fmt.Errorf("publish config %s/%s/%s: %w", options.Namespace, options.Group, dataID, err)
		}
	}
	return nil
}

func (c *adminClient) login(ctx context.Context, username, password string) (string, error) {
	form := url.Values{"username": {username}, "password": {password}}
	var response loginResponse
	if err := c.doForm(ctx, http.MethodPost, "/v3/auth/user/login", form, &response, false); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", fmt.Errorf("login response does not contain access token")
	}
	return response.AccessToken, nil
}

func (c *adminClient) namespaceExists(ctx context.Context, namespace string) (bool, error) {
	endpoint := c.endpoint("/v3/admin/core/namespace/check")
	query := endpoint.Query()
	query.Set("namespaceId", namespace)
	endpoint.RawQuery = query.Encode()
	var response apiResponse[int]
	if err := c.do(ctx, http.MethodGet, endpoint, "", &response, true); err != nil {
		return false, err
	}
	if response.Code != 0 || response.Data == nil {
		return false, apiError(response.Code, response.Message)
	}
	return *response.Data != 0, nil
}

func (c *adminClient) createNamespace(ctx context.Context, namespace string) error {
	form := url.Values{
		"namespaceId":   {namespace},
		"namespaceName": {namespace},
		"namespaceDesc": {"AegisCore local compose"},
	}
	var response apiResponse[bool]
	if err := c.doForm(ctx, http.MethodPost, "/v3/admin/core/namespace", form, &response, true); err != nil {
		return err
	}
	if response.Code != 0 || response.Data == nil || !*response.Data {
		return apiError(response.Code, response.Message)
	}
	return nil
}

func (c *adminClient) publishConfig(ctx context.Context, namespace, group, dataID string, content []byte) error {
	form := url.Values{
		"namespaceId": {namespace},
		"groupName":   {group},
		"dataId":      {dataID},
		"content":     {string(content)},
		"type":        {"yaml"},
	}
	var response apiResponse[bool]
	if err := c.doForm(ctx, http.MethodPost, "/v3/admin/cs/config", form, &response, true); err != nil {
		return err
	}
	if response.Code != 0 || response.Data == nil || !*response.Data {
		return apiError(response.Code, response.Message)
	}
	return nil
}

func (c *adminClient) doForm(ctx context.Context, method, apiPath string, form url.Values, target any, authenticated bool) error {
	return c.do(ctx, method, c.endpoint(apiPath), form.Encode(), target, authenticated)
}

func (c *adminClient) do(ctx context.Context, method string, endpoint *url.URL, body string, target any, authenticated bool) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", adminClientUserAgent)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if authenticated && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("accessToken", c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(payload) > maxResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}
	return nil
}

func (c *adminClient) endpoint(apiPath string) *url.URL {
	endpoint := *c.server
	endpoint.Path = path.Join(c.server.Path, apiPath)
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return &endpoint
}

func parseServerURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, ",") {
		return nil, fmt.Errorf("seed tool requires exactly one Nacos address")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse Nacos address: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("parse Nacos address: scheme must be http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("parse Nacos address: credentials, query and fragment are not allowed")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("parse Nacos address: host is required")
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, fmt.Errorf("parse Nacos address: port is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = defaultContextPath
	}
	parsed.RawPath = ""
	return parsed, nil
}

func apiError(code int, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "unspecified error"
	}
	if len(message) > 512 {
		message = message[:512] + "..."
	}
	return fmt.Errorf("api code %d: %s", code, message)
}

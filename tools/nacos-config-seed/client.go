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

// Validate 校验 seed 所需字段，并要求认证用户名与密码成对提供。
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

// newAdminClient 创建连接配置中心的管理客户端，负责校验服务地址是否合法。
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

// Seed 在同一个总超时内完成可选登录、namespace 确保和配置顺序发布。
// 任一步失败都会停止后续操作，但已经创建的 namespace 或已发布的配置不会回滚。
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

// login 使用管理员账号密码向配置中心申请登录令牌；登录成功后才能进行后续的配置推送。
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

// namespaceExists 检查配置中心中是否已经创建了指定的环境分区（命名空间），避免重复创建报错。
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

// createNamespace 在配置中心中创建新的环境分区（命名空间），用于隔离不同环境的配置文件。
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

// publishConfig 将本地的配置文件内容推送到配置中心中保存并发布。
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

// doForm 将表单形式的参数打包并向配置中心发起 HTTP 请求。
func (c *adminClient) doForm(ctx context.Context, method, apiPath string, form url.Values, target any, authenticated bool) error {
	return c.do(ctx, method, c.endpoint(apiPath), form.Encode(), target, authenticated)
}

// do 发起网络请求并处理配置中心的返回结果；遇到请求超时或服务端返回失败时报错。
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
	// 多读一个字节即可判定越界，同时限制异常服务端响应占用的内存。
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

// endpoint 根据相对路径拼接配置中心的完整访问网址。
func (c *adminClient) endpoint(apiPath string) *url.URL {
	// 每次请求复制基准 URL，避免修改共享 client 状态并使并发构造 endpoint 保持安全。
	endpoint := *c.server
	endpoint.Path = path.Join(c.server.Path, apiPath)
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return &endpoint
}

// parseServerURL 解析配置中心的服务地址（如 http://127.0.0.1:8848），确保地址格式合法且不包含多台混写。
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

// apiError 将配置中心返回的错误状态码与提示信息整理成统一的错误对象，且不打印敏感账号信息。
func apiError(code int, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "unspecified error"
	}
	// 服务端消息不受本工具控制，限制长度可避免诊断输出被异常响应无限放大。
	if len(message) > 512 {
		message = message[:512] + "..."
	}
	return fmt.Errorf("api code %d: %s", code, message)
}

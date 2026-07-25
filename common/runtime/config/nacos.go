package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultNacosContextPath = "/nacos"
	nacosConfigAPIPath      = "/v3/client/cs/config"
	nacosLoginAPIPath       = "/v3/auth/user/login"
	nacosClientUserAgent    = "AegisCore-Config-Client"
	maxNacosResponseBytes   = 4 << 20
)

type nacosDocumentLoader interface {
	LoadConfigDocument(ctx context.Context, env NacosEnv, dataID string) ([]byte, error)
}

type nacosV3Loader struct {
	servers  []*url.URL
	client   *http.Client
	username string
	password string

	authMu      sync.Mutex
	accessToken string
}

type nacosAPIResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *T     `json:"data"`
}

type nacosConfigData struct {
	Content    string `json:"content"`
	Success    bool   `json:"success"`
	ResultCode int    `json:"resultCode"`
	ErrorCode  int    `json:"errorCode"`
	Message    string `json:"message"`
}

type nacosLoginResponse struct {
	AccessToken string `json:"accessToken"`
}

// LoadNacosMergedSettings 读取环境变量、拉取 Nacos 文档并返回合并配置。
func LoadNacosMergedSettings(ctx context.Context) (map[string]any, SourceMetadata, error) {
	env, err := LoadNacosEnv()
	if err != nil {
		return nil, SourceMetadata{}, err
	}
	docs, err := loadNacosDocuments(ctx, env, nil)
	if err != nil {
		return nil, SourceMetadata{}, err
	}
	settings, err := DeepMergeYAML(docs)
	if err != nil {
		return nil, SourceMetadata{}, err
	}
	digest, err := DigestSettings(settings)
	if err != nil {
		return nil, SourceMetadata{}, err
	}
	return settings, sourceMetadata(env, digest), nil
}

func loadNacosDocuments(ctx context.Context, env NacosEnv, loader nacosDocumentLoader) ([]ConfigDocument, error) {
	if loader == nil {
		var err error
		loader, err = newNacosV3Loader(env, nil)
		if err != nil {
			return nil, err
		}
	}
	docs := make([]ConfigDocument, 0, len(env.DataIDs))
	for _, dataID := range env.DataIDs {
		content, err := loader.LoadConfigDocument(ctx, env, dataID)
		if err != nil {
			return nil, fmt.Errorf("load nacos config %s/%s/%s: %w", env.Namespace, env.Group, dataID, err)
		}
		if len(bytes.TrimSpace(content)) == 0 {
			return nil, fmt.Errorf("load nacos config %s/%s/%s: document is empty or not found", env.Namespace, env.Group, dataID)
		}
		docs = append(docs, ConfigDocument{DataID: dataID, Content: content})
	}
	return docs, nil
}

func newNacosV3Loader(env NacosEnv, client *http.Client) (*nacosV3Loader, error) {
	servers, err := nacosServerURLs(env.Addr)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: env.Timeout}
	}
	return &nacosV3Loader{
		servers: servers, client: client, username: env.Username, password: env.Password,
	}, nil
}

func (l *nacosV3Loader) LoadConfigDocument(ctx context.Context, env NacosEnv, dataID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, env.Timeout)
	defer cancel()

	var attempts []error
	for index, server := range l.servers {
		if err := ctx.Err(); err != nil {
			attempts = append(attempts, err)
			break
		}
		attemptTimeout := nacosAttemptTimeout(ctx, len(l.servers)-index)
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, attemptTimeout)
		content, err := l.loadFromServer(attemptCtx, server, env, dataID)
		cancelAttempt()
		if err == nil {
			return content, nil
		}
		attempts = append(attempts, fmt.Errorf("%s: %w", serverOrigin(server), err))
	}
	return nil, fmt.Errorf("all nacos servers failed: %w", errors.Join(attempts...))
}

// nacosAttemptTimeout 将剩余总预算平均分配给尚未尝试的服务器，避免单个故障节点耗尽全部预算。
func nacosAttemptTimeout(ctx context.Context, serversRemaining int) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok || serversRemaining <= 1 {
		return time.Until(deadline)
	}
	remaining := time.Until(deadline)
	budget := remaining / time.Duration(serversRemaining)
	if budget <= 0 {
		return remaining
	}
	return budget
}

func (l *nacosV3Loader) loadFromServer(ctx context.Context, server *url.URL, env NacosEnv, dataID string) ([]byte, error) {
	token, err := l.token(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	endpoint := nacosEndpoint(server, nacosConfigAPIPath)
	query := endpoint.Query()
	query.Set("namespaceId", env.Namespace)
	query.Set("groupName", env.Group)
	query.Set("dataId", dataID)
	endpoint.RawQuery = query.Encode()

	req, err := l.newRequest(ctx, http.MethodGet, endpoint, nil, token)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	var envelope nacosAPIResponse[nacosConfigData]
	if err := l.doJSON(req, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 || envelope.Data == nil {
		return nil, fmt.Errorf("api code %d: %s", envelope.Code, safeNacosMessage(envelope.Message))
	}
	if !envelope.Data.Success || envelope.Data.ResultCode != http.StatusOK {
		return nil, fmt.Errorf(
			"config result failed: result_code=%d error_code=%d message=%s",
			envelope.Data.ResultCode,
			envelope.Data.ErrorCode,
			safeNacosMessage(envelope.Data.Message),
		)
	}
	return []byte(envelope.Data.Content), nil
}

func (l *nacosV3Loader) token(ctx context.Context, server *url.URL) (string, error) {
	if l.username == "" {
		return "", nil
	}

	l.authMu.Lock()
	defer l.authMu.Unlock()
	if l.accessToken != "" {
		return l.accessToken, nil
	}

	form := url.Values{}
	form.Set("username", l.username)
	form.Set("password", l.password)
	req, err := l.newRequest(
		ctx,
		http.MethodPost,
		nacosEndpoint(server, nacosLoginAPIPath),
		strings.NewReader(form.Encode()),
		"",
	)
	if err != nil {
		return "", fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response nacosLoginResponse
	if err := l.doJSON(req, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", fmt.Errorf("login response does not contain access token")
	}
	l.accessToken = response.AccessToken
	return l.accessToken, nil
}

func (l *nacosV3Loader) newRequest(
	ctx context.Context,
	method string,
	endpoint *url.URL,
	body io.Reader,
	token string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", nacosClientUserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func (l *nacosV3Loader) doJSON(req *http.Request, target any) error {
	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxNacosResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(payload) > maxNacosResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxNacosResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}
	return nil
}

func nacosServerURLs(addr string) ([]*url.URL, error) {
	parts := strings.Split(addr, ",")
	servers := make([]*url.URL, 0, len(parts))
	for _, part := range parts {
		server, err := nacosServerURL(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func nacosServerURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("nacos server address is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse nacos server address %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("parse nacos server address %q: scheme must be http or https", raw)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("parse nacos server address %q: credentials, query and fragment are not allowed", raw)
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("parse nacos server address %q: host is required", raw)
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, fmt.Errorf("parse nacos server address %q: port is required", raw)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = defaultNacosContextPath
	}
	parsed.RawPath = ""
	return parsed, nil
}

func nacosEndpoint(server *url.URL, apiPath string) *url.URL {
	endpoint := *server
	endpoint.Path = path.Join(server.Path, apiPath)
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return &endpoint
}

func serverOrigin(server *url.URL) string {
	return server.Scheme + "://" + server.Host + server.Path
}

func safeNacosMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "unspecified error"
	}
	const maxMessageBytes = 512
	if len(message) > maxMessageBytes {
		return message[:maxMessageBytes] + "..."
	}
	return message
}

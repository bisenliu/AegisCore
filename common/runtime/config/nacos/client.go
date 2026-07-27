package nacos

import (
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
	defaultContextPath = "/nacos"
	configAPIPath      = "/v3/client/cs/config"
	loginAPIPath       = "/v3/auth/user/login"
	clientUserAgent    = "AegisCore-Config-Client"
	maxResponseBytes   = 4 << 20
)

type v3Loader struct {
	servers  []*url.URL
	client   *http.Client
	username string
	password string

	authMu      sync.Mutex
	accessToken string
}

type apiResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *T     `json:"data"`
}

type configData struct {
	Content    string `json:"content"`
	Success    bool   `json:"success"`
	ResultCode int    `json:"resultCode"`
	ErrorCode  int    `json:"errorCode"`
	Message    string `json:"message"`
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
}

func newV3Loader(env Env, client *http.Client) (*v3Loader, error) {
	servers, err := serverURLs(env.Addr)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: env.Timeout}
	}
	return &v3Loader{
		servers: servers, client: client, username: env.Username, password: env.Password,
	}, nil
}

func (l *v3Loader) LoadConfigDocument(ctx context.Context, env Env, dataID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, env.Timeout)
	defer cancel()

	var attempts []error
	for index, server := range l.servers {
		if err := ctx.Err(); err != nil {
			attempts = append(attempts, err)
			break
		}
		attemptTimeout := attemptTimeout(ctx, len(l.servers)-index)
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

// attemptTimeout 将剩余总预算平均分配给尚未尝试的服务器，避免单个故障节点耗尽全部预算。
func attemptTimeout(ctx context.Context, serversRemaining int) time.Duration {
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

func (l *v3Loader) loadFromServer(ctx context.Context, server *url.URL, env Env, dataID string) ([]byte, error) {
	token, err := l.token(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	endpoint := endpoint(server, configAPIPath)
	query := endpoint.Query()
	query.Set("namespaceId", env.Namespace)
	query.Set("groupName", env.Group)
	query.Set("dataId", dataID)
	endpoint.RawQuery = query.Encode()

	req, err := l.newRequest(ctx, http.MethodGet, endpoint, nil, token)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	var envelope apiResponse[configData]
	if err := l.doJSON(req, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 || envelope.Data == nil {
		return nil, fmt.Errorf("api code %d: %s", envelope.Code, safeMessage(envelope.Message))
	}
	if !envelope.Data.Success || envelope.Data.ResultCode != http.StatusOK {
		return nil, fmt.Errorf(
			"config result failed: result_code=%d error_code=%d message=%s",
			envelope.Data.ResultCode,
			envelope.Data.ErrorCode,
			safeMessage(envelope.Data.Message),
		)
	}
	return []byte(envelope.Data.Content), nil
}

func (l *v3Loader) token(ctx context.Context, server *url.URL) (string, error) {
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
		endpoint(server, loginAPIPath),
		strings.NewReader(form.Encode()),
		"",
	)
	if err != nil {
		return "", fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response loginResponse
	if err := l.doJSON(req, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", fmt.Errorf("login response does not contain access token")
	}
	l.accessToken = response.AccessToken
	return l.accessToken, nil
}

func (l *v3Loader) newRequest(
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
	req.Header.Set("User-Agent", clientUserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func (l *v3Loader) doJSON(req *http.Request, target any) error {
	resp, err := l.client.Do(req)
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

func serverURLs(addr string) ([]*url.URL, error) {
	parts := strings.Split(addr, ",")
	servers := make([]*url.URL, 0, len(parts))
	for _, part := range parts {
		server, err := serverURL(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func serverURL(raw string) (*url.URL, error) {
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
		parsed.Path = defaultContextPath
	}
	parsed.RawPath = ""
	return parsed, nil
}

func endpoint(server *url.URL, apiPath string) *url.URL {
	result := *server
	result.Path = path.Join(server.Path, apiPath)
	result.RawPath = ""
	result.RawQuery = ""
	result.Fragment = ""
	return &result
}

func serverOrigin(server *url.URL) string {
	return server.Scheme + "://" + server.Host + server.Path
}

func safeMessage(message string) string {
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

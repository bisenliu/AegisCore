package nacos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
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

// newV3Loader 根据环境中的地址列表创建 Nacos v3 文档加载器。
// 调用方未传入 HTTP client 时使用 env.Timeout 作为 client 级兜底超时。
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

// loadFromServer 从单个 Nacos server 加载一个 dataId 的配置内容。
// 多 server failover、总超时切分和错误聚合由 LoadConfigDocument 负责。
func (l *v3Loader) loadFromServer(ctx context.Context, server *url.URL, env Env, dataID string) ([]byte, error) {
	token, err := l.token(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	// query 参数保持 Nacos API 原始命名，避免在 common 中引入服务私有配置语义。
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

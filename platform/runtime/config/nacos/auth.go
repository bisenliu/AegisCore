package nacos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// token 返回访问 Nacos v3 API 所需的 bearer token。
// 未配置用户名时表示匿名访问；配置用户名时只登录一次并复用同一个 loader 内缓存的 token。
func (l *v3Loader) token(ctx context.Context, server *url.URL) (string, error) {
	if l.username == "" {
		return "", nil
	}

	// 登录接口可能被多个 dataId 加载路径并发触发，用锁保证只执行一次远端登录。
	l.authMu.Lock()
	defer l.authMu.Unlock()
	if l.accessToken != "" {
		return l.accessToken, nil
	}

	// Nacos v3 登录接口使用 form 编码，密码只作为请求体发送，不进入错误文本或日志。
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

package config

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadNacosEnvDefaultsDataIDs(t *testing.T) {
	env, err := loadNacosEnv(mapLookup(map[string]string{
		EnvService:        " user-service ",
		EnvNacosAddr:      "nacos:8848",
		EnvNacosNamespace: "local",
		EnvNacosGroup:     "AEGISCORE",
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"base.yaml", "resources.yaml", "user-service.yaml"}, env.DataIDs)
	require.Equal(t, defaultNacosTimeout, env.Timeout)
}

func TestLoadNacosEnvRejectsMissingRequired(t *testing.T) {
	_, err := loadNacosEnv(mapLookup(map[string]string{
		EnvService:        "user-service",
		EnvNacosNamespace: "local",
		EnvNacosGroup:     "AEGISCORE",
	}))
	require.ErrorContains(t, err, "read config env: AEGISCORE_NACOS_ADDR is required")
}

func TestLoadNacosEnvParsesExplicitDataIDs(t *testing.T) {
	env, err := loadNacosEnv(mapLookup(map[string]string{
		EnvService:        "user-service",
		EnvNacosAddr:      "nacos:8848",
		EnvNacosNamespace: "local",
		EnvNacosGroup:     "AEGISCORE",
		EnvNacosDataIDs:   "base.yaml, resources.yaml , override.yaml",
		EnvNacosTimeout:   "2s",
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"base.yaml", "resources.yaml", "override.yaml"}, env.DataIDs)
	require.Equal(t, 2*time.Second, env.Timeout)
}

func TestLoadNacosEnvRejectsPartialCredentials(t *testing.T) {
	_, err := loadNacosEnv(mapLookup(map[string]string{
		EnvService:        "user-service",
		EnvNacosAddr:      "nacos:8848",
		EnvNacosNamespace: "local",
		EnvNacosGroup:     "AEGISCORE",
		EnvNacosUsername:  "nacos",
	}))
	require.ErrorContains(t, err, "AEGISCORE_NACOS_USERNAME and AEGISCORE_NACOS_PASSWORD must be set together")
}

func TestLoadNacosEnvPreservesPasswordWhitespace(t *testing.T) {
	env, err := loadNacosEnv(mapLookup(map[string]string{
		EnvService:        "user-service",
		EnvNacosAddr:      "nacos:8848",
		EnvNacosNamespace: "local",
		EnvNacosGroup:     "AEGISCORE",
		EnvNacosUsername:  " nacos ",
		EnvNacosPassword:  " secret-password ",
	}))
	require.NoError(t, err)
	require.Equal(t, "nacos", env.Username)
	require.Equal(t, " secret-password ", env.Password)
}

func TestDeepMergeYAML(t *testing.T) {
	settings, err := DeepMergeYAML([]ConfigDocument{
		{DataID: "base.yaml", Content: []byte("log:\n  level: info\n  format: json\nitems:\n  - a\nclear_me:\n  nested: true\n")},
		{DataID: "user-service.yaml", Content: []byte("log:\n  level: debug\nitems:\n  - b\nclear_me: null\n")},
	})
	require.NoError(t, err)
	require.Equal(t, "debug", settings["log"].(map[string]any)["level"])
	require.Equal(t, "json", settings["log"].(map[string]any)["format"])
	require.Equal(t, []any{"b"}, settings["items"])
	require.Contains(t, settings, "clear_me")
	require.Nil(t, settings["clear_me"])
}

func TestDecodeStrictRejectsUnknownKey(t *testing.T) {
	_, err := DecodeStrict[Config](map[string]any{
		"runtime": map[string]any{"gin": map[string]any{"bad_mode": "debug"}},
	}, Config.Validate)
	require.ErrorContains(t, err, "unknown configuration keys: runtime.gin.bad_mode")
}

func TestRedactSettingsAndDigest(t *testing.T) {
	settings := map[string]any{
		"auth": map[string]any{"jwt": map[string]any{"secret": "secret-value"}},
		"resources": map[string]any{
			"redis":    map[string]any{"cache_redis": map[string]any{"password": "redis-secret"}},
			"postgres": map[string]any{"primary_db": map[string]any{"password": "pg-secret"}},
		},
	}
	redacted := RedactSettings(settings, nil)
	require.Equal(t, "***", redacted["auth"].(map[string]any)["jwt"].(map[string]any)["secret"])
	require.Equal(t, "secret-value", settings["auth"].(map[string]any)["jwt"].(map[string]any)["secret"])
	first, err := DigestSettings(settings)
	require.NoError(t, err)
	second, err := DigestSettings(map[string]any{
		"resources": settings["resources"],
		"auth":      settings["auth"],
	})
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestLoadNacosDocumentsWrapsDataID(t *testing.T) {
	boom := errors.New("timeout after 5s")
	_, err := loadNacosDocuments(context.Background(), NacosEnv{
		Namespace: "local", Group: "AEGISCORE", DataIDs: []string{"base.yaml"},
	}, fakeNacosLoader{err: boom})
	require.ErrorContains(t, err, "load nacos config local/AEGISCORE/base.yaml")
	require.ErrorIs(t, err, boom)
}

func TestLoadNacosDocumentsRejectsEmptyConfig(t *testing.T) {
	_, err := loadNacosDocuments(context.Background(), NacosEnv{
		Namespace: "local", Group: "AEGISCORE", DataIDs: []string{"base.yaml"},
	}, fakeNacosLoader{content: []byte(" \n\t")})
	require.ErrorContains(t, err, "load nacos config local/AEGISCORE/base.yaml: document is empty or not found")
}

func TestNacosV3LoaderLoadsConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/nacos/v3/client/cs/config", r.URL.Path)
		require.Equal(t, "local", r.URL.Query().Get("namespaceId"))
		require.Equal(t, "AEGISCORE", r.URL.Query().Get("groupName"))
		require.Equal(t, "base.yaml", r.URL.Query().Get("dataId"))
		require.Equal(t, nacosClientUserAgent, r.Header.Get("User-Agent"))
		writeNacosConfigResponse(t, w, "app:\n  environment: local\n")
	}))
	defer server.Close()

	env := testNacosEnv(server.URL)
	loader, err := newNacosV3Loader(env, server.Client())
	require.NoError(t, err)
	content, err := loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "app:\n  environment: local\n", string(content))
}

func TestNacosV3LoaderLogsInOnceAndReusesBearerToken(t *testing.T) {
	loginCalls := 0
	configCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v3/auth/user/login":
			loginCalls++
			require.NoError(t, r.ParseForm())
			require.Equal(t, "nacos", r.Form.Get("username"))
			require.Equal(t, " secret-password ", r.Form.Get("password"))
			_, err := w.Write([]byte(`{"accessToken":"token-value","tokenTtl":18000,"username":"nacos"}`))
			require.NoError(t, err)
		case "/nacos/v3/client/cs/config":
			configCalls++
			require.Equal(t, "Bearer token-value", r.Header.Get("Authorization"))
			writeNacosConfigResponse(t, w, "value: true\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := testNacosEnv(server.URL)
	env.Username = "nacos"
	env.Password = " secret-password "
	env.DataIDs = []string{"base.yaml", "user-service.yaml"}
	loader, err := newNacosV3Loader(env, server.Client())
	require.NoError(t, err)
	_, err = loadNacosDocuments(context.Background(), env, loader)
	require.NoError(t, err)
	require.Equal(t, 1, loginCalls)
	require.Equal(t, 2, configCalls)
}

func TestNacosV3LoaderFailsOverInDeclaredOrder(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	firstURL := first.URL
	first.Close()
	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondCalls++
		writeNacosConfigResponse(t, w, "value: fallback\n")
	}))
	defer second.Close()

	env := testNacosEnv(firstURL + "," + second.URL)
	loader, err := newNacosV3Loader(env, second.Client())
	require.NoError(t, err)
	content, err := loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "value: fallback\n", string(content))
	require.Equal(t, 1, secondCalls)
}

func TestNacosV3LoaderFailsOverAfterFirstServerAttemptTimeout(t *testing.T) {
	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		firstCalls++
		<-r.Context().Done()
	}))
	defer first.Close()

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondCalls++
		writeNacosConfigResponse(t, w, "value: fallback-after-timeout\n")
	}))
	defer second.Close()

	env := testNacosEnv(first.URL + "," + second.URL)
	env.Timeout = 400 * time.Millisecond
	loader, err := newNacosV3Loader(env, second.Client())
	require.NoError(t, err)
	content, err := loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "value: fallback-after-timeout\n", string(content))
	require.Equal(t, 1, firstCalls)
	require.Equal(t, 1, secondCalls)
}

func TestNacosV3LoaderPreservesCustomContextPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/custom/v3/client/cs/config", r.URL.Path)
		writeNacosConfigResponse(t, w, "value: custom\n")
	}))
	defer server.Close()

	env := testNacosEnv(server.URL + "/custom")
	loader, err := newNacosV3Loader(env, server.Client())
	require.NoError(t, err)
	_, err = loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
}

func TestNacosV3LoaderRejectsFailedEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"code":20004,"message":"resource not found","data":null}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	env := testNacosEnv(server.URL)
	loader, err := newNacosV3Loader(env, server.Client())
	require.NoError(t, err)
	_, err = loader.LoadConfigDocument(context.Background(), env, "missing.yaml")
	require.ErrorContains(t, err, "api code 20004: resource not found")
}

func TestNacosServerURLsParseV3Endpoints(t *testing.T) {
	servers, err := nacosServerURLs("nacos-a:8848,https://nacos-b:9443/custom")
	require.NoError(t, err)
	require.Len(t, servers, 2)
	require.Equal(t, "http://nacos-a:8848/nacos", servers[0].String())
	require.Equal(t, "https://nacos-b:9443/custom", servers[1].String())
}

func TestNacosServerURLRejectsMissingPort(t *testing.T) {
	_, err := nacosServerURL("nacos.local")
	require.ErrorContains(t, err, "port is required")
}

func testNacosEnv(addr string) NacosEnv {
	return NacosEnv{
		Service: "user-service", Addr: addr, Namespace: "local", Group: "AEGISCORE",
		DataIDs: []string{"base.yaml"}, Timeout: 2 * time.Second,
	}
}

func writeNacosConfigResponse(t *testing.T, w http.ResponseWriter, content string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintf(
		w,
		`{"code":0,"message":"success","data":{"resultCode":200,"errorCode":0,"content":%q,"success":true}}`,
		content,
	)
	require.NoError(t, err)
}

type fakeNacosLoader struct {
	content []byte
	err     error
}

func (f fakeNacosLoader) LoadConfigDocument(context.Context, NacosEnv, string) ([]byte, error) {
	return f.content, f.err
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

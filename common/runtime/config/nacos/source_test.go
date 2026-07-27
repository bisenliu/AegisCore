package nacos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	commonconfig "github.com/aegiscore/common/runtime/config"
)

func TestSourceLoadsAndPipelineMergesDocumentsWithMetadata(t *testing.T) {
	documents := map[string]string{
		"base.yaml":         "log:\n  level: info\n  format: json\nitems:\n  - a\n",
		"user-service.yaml": "log:\n  level: debug\nitems:\n  - b\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeConfigResponse(t, w, documents[r.URL.Query().Get("dataId")])
	}))
	defer server.Close()

	env := testEnv(server.URL)
	env.DataIDs = []string{"base.yaml", "user-service.yaml"}
	loader, err := newV3Loader(env, server.Client())
	require.NoError(t, err)
	source := newSource(env, loader)
	settings, metadata, err := commonconfig.LoadSource(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, "debug", settings["log"].(map[string]any)["level"])
	require.Equal(t, "json", settings["log"].(map[string]any)["format"])
	require.Equal(t, []any{"b"}, settings["items"])
	require.Equal(t, "nacos", metadata.Provider)
	require.Equal(t, env.Service, metadata.Service)
	require.Equal(t, env.Namespace, metadata.Namespace)
	require.Equal(t, env.Group, metadata.Group)
	require.Equal(t, env.DataIDs, metadata.DataIDs)

	digest, err := commonconfig.DigestSettings(settings)
	require.NoError(t, err)
	require.Equal(t, digest, metadata.Digest)
}

func TestSourceWrapsDataID(t *testing.T) {
	boom := errors.New("timeout after 5s")
	source := newSource(Env{
		Namespace: "local", Group: "AEGISCORE", DataIDs: []string{"base.yaml"},
	}, fakeLoader{err: boom})
	_, _, err := source.LoadDocuments(context.Background())
	require.ErrorContains(t, err, "load nacos config local/AEGISCORE/base.yaml")
	require.ErrorIs(t, err, boom)
}

func TestSourceRejectsEmptyConfig(t *testing.T) {
	source := newSource(Env{
		Namespace: "local", Group: "AEGISCORE", DataIDs: []string{"base.yaml"},
	}, fakeLoader{content: []byte(" \n\t")})
	_, _, err := source.LoadDocuments(context.Background())
	require.ErrorContains(t, err, "load nacos config local/AEGISCORE/base.yaml: document is empty or not found")
}

func TestV3LoaderLoadsConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/nacos/v3/client/cs/config", r.URL.Path)
		require.Equal(t, "local", r.URL.Query().Get("namespaceId"))
		require.Equal(t, "AEGISCORE", r.URL.Query().Get("groupName"))
		require.Equal(t, "base.yaml", r.URL.Query().Get("dataId"))
		require.Equal(t, clientUserAgent, r.Header.Get("User-Agent"))
		writeConfigResponse(t, w, "app:\n  environment: local\n")
	}))
	defer server.Close()

	env := testEnv(server.URL)
	loader, err := newV3Loader(env, server.Client())
	require.NoError(t, err)
	content, err := loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "app:\n  environment: local\n", string(content))
}

func TestV3LoaderLogsInOnceAndReusesBearerToken(t *testing.T) {
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
			writeConfigResponse(t, w, "value: true\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := testEnv(server.URL)
	env.Username = "nacos"
	env.Password = " secret-password "
	env.DataIDs = []string{"base.yaml", "user-service.yaml"}
	loader, err := newV3Loader(env, server.Client())
	require.NoError(t, err)
	source := newSource(env, loader)
	_, _, err = source.LoadDocuments(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, loginCalls)
	require.Equal(t, 2, configCalls)
}

func TestV3LoaderFailsOverInDeclaredOrder(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	firstURL := first.URL
	first.Close()
	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondCalls++
		writeConfigResponse(t, w, "value: fallback\n")
	}))
	defer second.Close()

	env := testEnv(firstURL + "," + second.URL)
	loader, err := newV3Loader(env, second.Client())
	require.NoError(t, err)
	content, err := loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "value: fallback\n", string(content))
	require.Equal(t, 1, secondCalls)
}

func TestV3LoaderFailsOverAfterFirstServerAttemptTimeout(t *testing.T) {
	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		firstCalls++
		<-r.Context().Done()
	}))
	defer first.Close()

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondCalls++
		writeConfigResponse(t, w, "value: fallback-after-timeout\n")
	}))
	defer second.Close()

	env := testEnv(first.URL + "," + second.URL)
	env.Timeout = 400 * time.Millisecond
	loader, err := newV3Loader(env, second.Client())
	require.NoError(t, err)
	content, err := loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "value: fallback-after-timeout\n", string(content))
	require.Equal(t, 1, firstCalls)
	require.Equal(t, 1, secondCalls)
}

func TestV3LoaderPreservesCustomContextPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/custom/v3/client/cs/config", r.URL.Path)
		writeConfigResponse(t, w, "value: custom\n")
	}))
	defer server.Close()

	env := testEnv(server.URL + "/custom")
	loader, err := newV3Loader(env, server.Client())
	require.NoError(t, err)
	_, err = loader.LoadConfigDocument(context.Background(), env, "base.yaml")
	require.NoError(t, err)
}

func TestV3LoaderRejectsFailedEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"code":20004,"message":"resource not found","data":null}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	env := testEnv(server.URL)
	loader, err := newV3Loader(env, server.Client())
	require.NoError(t, err)
	_, err = loader.LoadConfigDocument(context.Background(), env, "missing.yaml")
	require.ErrorContains(t, err, "api code 20004: resource not found")
}

func TestServerURLsParseV3Endpoints(t *testing.T) {
	servers, err := serverURLs("nacos-a:8848,https://nacos-b:9443/custom")
	require.NoError(t, err)
	require.Len(t, servers, 2)
	require.Equal(t, "http://nacos-a:8848/nacos", servers[0].String())
	require.Equal(t, "https://nacos-b:9443/custom", servers[1].String())
}

func TestServerURLRejectsMissingPort(t *testing.T) {
	_, err := serverURL("nacos.local")
	require.ErrorContains(t, err, "port is required")
}

func testEnv(addr string) Env {
	return Env{
		Service: "user-service", Addr: addr, Namespace: "local", Group: "AEGISCORE",
		DataIDs: []string{"base.yaml"}, Timeout: 2 * time.Second,
	}
}

func writeConfigResponse(t *testing.T, w http.ResponseWriter, content string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintf(
		w,
		`{"code":0,"message":"success","data":{"resultCode":200,"errorCode":0,"content":%q,"success":true}}`,
		content,
	)
	require.NoError(t, err)
}

type fakeLoader struct {
	content []byte
	err     error
}

func (f fakeLoader) LoadConfigDocument(context.Context, Env, string) ([]byte, error) {
	return f.content, f.err
}

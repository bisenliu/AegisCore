package nacos_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/config/nacos"
)

// ExampleNewSource 展示 Nacos source 如何接入 common 配置加载管线，
// 包括按 dataId 读取文档、递归合并后保留 source metadata。
func ExampleNewSource() {
	documents := map[string]string{
		"base.yaml":         "service:\n  name: user-service\nlog:\n  level: info\nfeature:\n  enabled: false\n",
		"user-service.yaml": "log:\n  level: debug\nfeature:\n  enabled: true\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/client/cs/config" {
			http.NotFound(w, r)
			return
		}
		dataID := r.URL.Query().Get("dataId")
		content, ok := documents[dataID]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(
			w,
			`{"code":0,"message":"success","data":{"resultCode":200,"errorCode":0,"content":%q,"success":true}}`,
			content,
		)
	}))
	defer server.Close()

	source, err := nacos.NewSource(nacos.Env{
		Service:   "user-service",
		Addr:      server.URL,
		Namespace: "local",
		Group:     "AEGISCORE",
		DataIDs:   []string{"base.yaml", "user-service.yaml"},
		Timeout:   2 * time.Second,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	settings, metadata, err := commonconfig.LoadSource(context.Background(), source)
	if err != nil {
		fmt.Println(err)
		return
	}

	service := settings["service"].(map[string]any)
	log := settings["log"].(map[string]any)
	feature := settings["feature"].(map[string]any)
	fmt.Println(service["name"])
	fmt.Println(log["level"])
	fmt.Println(feature["enabled"])
	fmt.Printf("%s %s %s %s %t\n", metadata.Provider, metadata.Namespace, metadata.Group, metadata.DataIDsCSV(), metadata.Digest != "")

	// Output:
	// user-service
	// debug
	// true
	// nacos local AEGISCORE base.yaml,user-service.yaml true
}

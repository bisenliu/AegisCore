package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunSeedsNamespaceAndDocuments(t *testing.T) {
	dir := writeConfigDocuments(t)
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/nacos/v3/admin/core/namespace/check":
			if got := r.URL.Query().Get("namespaceId"); got != "loca" {
				t.Fatalf("namespaceId = %q", got)
			}
			writeResponse(t, w, `{"code":0,"message":"success","data":0}`)
		case "/nacos/v3/admin/core/namespace":
			requireFormValue(t, r, "namespaceId", "loca")
			writeResponse(t, w, `{"code":0,"message":"success","data":true}`)
		case "/nacos/v3/admin/cs/config":
			requireFormValue(t, r, "groupName", "AEGISCORE")
			if r.Form.Get("content") == "" {
				t.Fatal("config content is empty")
			}
			writeResponse(t, w, `{"code":0,"message":"success","data":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"-addr", server.URL,
		"-namespace", "loca",
		"-group", "AEGISCORE",
		"-config-dir", dir,
		"-data-ids", "base.yaml,resources.yaml,user-service.yaml",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); got != "seeded Nacos namespace=loca group=AEGISCORE data_ids=base.yaml,resources.yaml,user-service.yaml\n" {
		t.Fatalf("stdout = %q", got)
	}
	if len(calls) != 5 {
		t.Fatalf("calls = %v", calls)
	}
}

func TestAdminClientLogsInAndCarriesToken(t *testing.T) {
	dir := writeConfigDocuments(t)
	t.Setenv("AEGISCORE_NACOS_USERNAME", " nacos ")
	t.Setenv("AEGISCORE_NACOS_PASSWORD", " secret-password ")
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nacos/v3/auth/user/login" {
			loginCalls++
			requireFormValue(t, r, "username", "nacos")
			requireFormValue(t, r, "password", " secret-password ")
			writeResponse(t, w, `{"accessToken":"token-value"}`)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-value" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.URL.Path == "/nacos/v3/admin/core/namespace/check" {
			writeResponse(t, w, `{"code":0,"message":"success","data":1}`)
			return
		}
		writeResponse(t, w, `{"code":0,"message":"success","data":true}`)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"-addr", server.URL,
		"-config-dir", dir,
		"-data-ids", "base.yaml,resources.yaml,user-service.yaml",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if loginCalls != 1 {
		t.Fatalf("loginCalls = %d", loginCalls)
	}
}

func TestRunRejectsFailedEnvelope(t *testing.T) {
	dir := writeConfigDocuments(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, `{"code":403,"message":"access denied","data":null}`)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"-addr", server.URL,
		"-config-dir", dir,
		"-data-ids", "base.yaml,resources.yaml,user-service.yaml",
	}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("code = %d", code)
	}
	if got := stderr.String(); !bytes.Contains([]byte(got), []byte("api code 403: access denied")) {
		t.Fatalf("stderr = %q", got)
	}
}

func TestRunPreloadsAllDocumentsBeforeNetwork(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(*testing.T, string)
		errorText string
	}{
		{
			name: "missing last document",
			prepare: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, "user-service.yaml")); err != nil {
					t.Fatal(err)
				}
			},
			errorText: "read config document user-service.yaml",
		},
		{
			name: "empty middle document",
			prepare: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte("  \n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errorText: "read config document resources.yaml: content is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeConfigDocuments(t)
			tt.prepare(t, dir)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				writeResponse(t, w, `{"code":0,"message":"success","data":1}`)
			}))
			defer server.Close()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run(context.Background(), []string{
				"-addr", server.URL,
				"-config-dir", dir,
				"-data-ids", "base.yaml,resources.yaml,user-service.yaml",
			}, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("code = %d", code)
			}
			if got := stderr.String(); !strings.Contains(got, tt.errorText) {
				t.Fatalf("stderr = %q, want substring %q", got, tt.errorText)
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("network requests = %d, want 0", got)
			}
		})
	}
}

func writeConfigDocuments(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, dataID := range []string{"base.yaml", "resources.yaml", "user-service.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, dataID), []byte("value: "+dataID+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func requireFormValue(t *testing.T, r *http.Request, name, want string) {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if got := r.Form.Get(name); got != want {
		t.Fatalf("%s = %q, want %q; form=%s", name, got, want, url.Values(r.Form).Encode())
	}
}

func writeResponse(t *testing.T, w http.ResponseWriter, payload string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprint(w, payload); err != nil {
		t.Fatal(err)
	}
}

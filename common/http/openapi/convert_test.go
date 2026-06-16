package openapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestConvertSwagger2JSONAppliesOptions(t *testing.T) {
	document, err := ConvertSwagger2JSON(context.Background(), swaggerFixture(), ConvertOptions{
		OpenAPIVersion: "3.0.3",
		Servers:        []Server{{URL: "/service"}},
		PathServers:    map[string][]Server{"/healthz": {{URL: "/"}}},
		SecuritySchemes: map[string]SecurityScheme{
			"TokenAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "输入访问令牌。",
			},
		},
	})
	if err != nil {
		t.Fatalf("ConvertSwagger2JSON() error = %v", err)
	}

	var doc openapi3.T
	if err := json.Unmarshal(document.JSON, &doc); err != nil {
		t.Fatalf("unmarshal OpenAPI JSON: %v", err)
	}

	if document.OpenAPI != "3.0.3" || doc.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version = %q/%q, want 3.0.3", document.OpenAPI, doc.OpenAPI)
	}
	if document.PathCount != 2 {
		t.Fatalf("PathCount = %d, want 2", document.PathCount)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "/service" {
		t.Fatalf("global servers = %#v, want /service", doc.Servers)
	}

	healthPath := doc.Paths.Find("/healthz")
	if healthPath == nil {
		t.Fatal("missing /healthz path")
	}
	if len(healthPath.Servers) != 1 || healthPath.Servers[0].URL != "/" {
		t.Fatalf("/healthz servers = %#v, want /", healthPath.Servers)
	}

	scheme := doc.Components.SecuritySchemes["TokenAuth"]
	if scheme == nil || scheme.Value == nil {
		t.Fatal("missing TokenAuth security scheme")
	}
	if scheme.Value.Type != "http" || scheme.Value.Scheme != "bearer" || scheme.Value.BearerFormat != "JWT" {
		t.Fatalf("TokenAuth = %#v, want JWT bearer scheme", scheme.Value)
	}
	if scheme.Value.Description != "输入访问令牌。" {
		t.Fatalf("TokenAuth description = %q", scheme.Value.Description)
	}
	if len(document.YAML) == 0 {
		t.Fatal("YAML output is empty")
	}
}

func TestConvertSwagger2JSONWithoutOptionsDoesNotInjectServiceValues(t *testing.T) {
	document, err := ConvertSwagger2JSON(context.Background(), swaggerFixture(), ConvertOptions{})
	if err != nil {
		t.Fatalf("ConvertSwagger2JSON() error = %v", err)
	}

	var doc openapi3.T
	if err := json.Unmarshal(document.JSON, &doc); err != nil {
		t.Fatalf("unmarshal OpenAPI JSON: %v", err)
	}

	if doc.OpenAPI != DefaultOpenAPIVersion {
		t.Fatalf("OpenAPI version = %q, want %q", doc.OpenAPI, DefaultOpenAPIVersion)
	}
	if len(doc.Servers) != 0 {
		t.Fatalf("servers = %#v, want none", doc.Servers)
	}
	if strings.Contains(string(document.JSON), "/service") {
		t.Fatal("JSON output contains option-specific /service")
	}
	if strings.Contains(string(document.JSON), "TokenAuth") {
		t.Fatal("JSON output contains option-specific TokenAuth")
	}

	healthPath := doc.Paths.Find("/healthz")
	if healthPath == nil {
		t.Fatal("missing /healthz path")
	}
	if len(healthPath.Servers) != 0 {
		t.Fatalf("/healthz servers = %#v, want none", healthPath.Servers)
	}
}

func TestConvertSwagger2JSONRejectsInvalidInput(t *testing.T) {
	_, err := ConvertSwagger2JSON(context.Background(), []byte("{"), ConvertOptions{})
	if err == nil {
		t.Fatal("ConvertSwagger2JSON() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode Swagger input") {
		t.Fatalf("error = %q, want decode Swagger input", err)
	}
}

func swaggerFixture() []byte {
	return []byte(`{
  "swagger": "2.0",
  "info": {
    "title": "Test API",
    "version": "1.0.0"
  },
  "paths": {
    "/users": {
      "get": {
        "responses": {
          "200": {
            "description": "ok"
          }
        }
      }
    },
    "/healthz": {
      "get": {
        "responses": {
          "200": {
            "description": "ok"
          }
        }
      }
    }
  }
}`)
}

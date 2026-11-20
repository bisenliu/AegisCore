package openapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	var doc openapi3.T
	require.NoError(t, json.Unmarshal(document.JSON, &doc))

	require.Equal(t, "3.0.3", document.OpenAPI)
	require.Equal(t, "3.0.3", doc.OpenAPI)
	require.Equal(t, 2, document.PathCount)
	require.Len(t, doc.Servers, 1)
	require.Equal(t, "/service", doc.Servers[0].URL)

	healthPath := doc.Paths.Find("/healthz")
	require.NotNil(t, healthPath)
	require.Len(t, healthPath.Servers, 1)
	require.Equal(t, "/", healthPath.Servers[0].URL)

	scheme := doc.Components.SecuritySchemes["TokenAuth"]
	require.NotNil(t, scheme)
	require.NotNil(t, scheme.Value)
	require.Equal(t, "http", scheme.Value.Type)
	require.Equal(t, "bearer", scheme.Value.Scheme)
	require.Equal(t, "JWT", scheme.Value.BearerFormat)
	require.Equal(t, "输入访问令牌。", scheme.Value.Description)
	require.NotEmpty(t, document.YAML)
}

func TestConvertSwagger2JSONWithoutOptionsDoesNotInjectServiceValues(t *testing.T) {
	document, err := ConvertSwagger2JSON(context.Background(), swaggerFixture(), ConvertOptions{})
	require.NoError(t, err)

	var doc openapi3.T
	require.NoError(t, json.Unmarshal(document.JSON, &doc))

	require.Equal(t, DefaultOpenAPIVersion, doc.OpenAPI)
	require.Empty(t, doc.Servers)
	require.NotContains(t, string(document.JSON), "/service")
	require.NotContains(t, string(document.JSON), "TokenAuth")

	healthPath := doc.Paths.Find("/healthz")
	require.NotNil(t, healthPath)
	require.Empty(t, healthPath.Servers)
}

func TestConvertSwagger2JSONRejectsInvalidInput(t *testing.T) {
	_, err := ConvertSwagger2JSON(context.Background(), []byte("{"), ConvertOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode Swagger input")
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

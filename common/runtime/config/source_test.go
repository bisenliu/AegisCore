package config

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadSourceMergesDocumentsAndCalculatesRawDigest(t *testing.T) {
	metadata := SourceMetadata{
		Provider: "memory",
		Service:  "test-service",
		DataIDs:  []string{"base.yaml", "override.yaml"},
	}
	source := fakeDocumentSource{
		docs: []ConfigDocument{
			{DataID: "base.yaml", Content: []byte("log:\n  level: info\n  format: json\nitems:\n  - a\nclear_me:\n  nested: true\n")},
			{DataID: "override.yaml", Content: []byte("log:\n  level: debug\nitems:\n  - b\nclear_me: null\n")},
		},
		metadata: metadata,
	}

	settings, loadedMetadata, err := LoadSource(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, "debug", settings["log"].(map[string]any)["level"])
	require.Equal(t, "json", settings["log"].(map[string]any)["format"])
	require.Equal(t, []any{"b"}, settings["items"])
	require.Contains(t, settings, "clear_me")
	require.Nil(t, settings["clear_me"])

	digest, err := DigestSettings(settings)
	require.NoError(t, err)
	require.Equal(t, digest, loadedMetadata.Digest)
	require.Equal(t, "memory", loadedMetadata.Provider)
	require.Equal(t, "test-service", loadedMetadata.Service)
	require.Equal(t, "base.yaml,override.yaml", loadedMetadata.DataIDsCSV())

	metadata.DataIDs[0] = "mutated.yaml"
	require.Equal(t, "base.yaml", loadedMetadata.DataIDs[0])
}

func TestLoadSourceWrapsSourceError(t *testing.T) {
	boom := errors.New("source unavailable")
	_, _, err := LoadSource(context.Background(), fakeDocumentSource{err: boom})
	require.ErrorContains(t, err, "load config source")
	require.ErrorIs(t, err, boom)
}

func TestLoadSourceRequiresDocumentSource(t *testing.T) {
	_, _, err := LoadSource(context.Background(), nil)
	require.EqualError(t, err, "load config source: document source is required")
}

func TestRedactSettingsAndDigestRemainStable(t *testing.T) {
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

type fakeDocumentSource struct {
	docs     []ConfigDocument
	metadata SourceMetadata
	err      error
}

func (s fakeDocumentSource) LoadDocuments(context.Context) ([]ConfigDocument, SourceMetadata, error) {
	return s.docs, s.metadata, s.err
}

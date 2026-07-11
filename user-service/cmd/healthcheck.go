package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHealthcheckURL     = "http://127.0.0.1:8080/readyz"
	defaultHealthcheckTimeout = 2 * time.Second
	maxHealthcheckBodyBytes   = 1 << 20
)

type healthcheckOptions struct {
	url     string
	timeout time.Duration
}

type healthcheckResponse struct {
	Status string `json:"status"`
}

func runHealthcheck(ctx context.Context, opts healthcheckOptions) error {
	if strings.TrimSpace(opts.url) == "" {
		return fmt.Errorf("healthcheck url is required")
	}
	target, err := url.Parse(opts.url)
	if err != nil {
		return fmt.Errorf("parse healthcheck url: %w", err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("healthcheck url scheme must be http or https")
	}
	if target.Host == "" {
		return fmt.Errorf("healthcheck url host is required")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("healthcheck timeout must be positive")
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}
	client := http.Client{Timeout: opts.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("healthcheck returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHealthcheckBodyBytes))
	if err != nil {
		return fmt.Errorf("read healthcheck response: %w", err)
	}
	var payload healthcheckResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode healthcheck response: %w", err)
	}
	if payload.Status != "ok" {
		if payload.Status == "" {
			return fmt.Errorf("healthcheck response missing status")
		}
		return fmt.Errorf("healthcheck status is %q", payload.Status)
	}
	return nil
}

package judgement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.typesafe.ai/v1"

// Config is copied by NewClient. Timeout covers all attempts and retry delays.
// MaxRetries counts attempts after the initial request; zero disables retries.
type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	MaxRetries int
}

// Client is safe for concurrent use. Configuration is immutable after creation.
type Client struct {
	key, base string
	http      *http.Client
	timeout   time.Duration
	retries   int
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("TYPESAFE_API_KEY is required")
	}
	if strings.ContainsAny(cfg.APIKey, "\r\n") {
		return nil, errors.New("API key contains invalid characters")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("base URL must be an HTTP(S) URL without credentials, query, or fragment")
	}
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return nil, errors.New("unencrypted HTTP is only allowed for loopback development servers")
	}
	if cfg.MaxRetries < 0 || cfg.MaxRetries > 10 {
		return nil, errors.New("max retries must be between 0 and 10")
	}
	if cfg.Timeout < 0 {
		return nil, errors.New("timeout must be positive")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	hc := http.Client{}
	if cfg.HTTPClient != nil {
		hc = *cfg.HTTPClient
	}
	// Redirects must never forward the bearer token to a different endpoint.
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{key: cfg.APIKey, base: strings.TrimRight(cfg.BaseURL, "/"), http: &hc, timeout: cfg.Timeout, retries: cfg.MaxRetries}, nil
}

// APIError is a sanitized remote failure. Bodies are intentionally not included:
// providers and proxies may echo credentials or private request data.
type APIError struct {
	StatusCode int    `json:"status"`
	RequestID  string `json:"request_id,omitempty"`
	Retryable  bool   `json:"retryable"`
}

func (e *APIError) Error() string { return fmt.Sprintf("TypeSafe API returned HTTP %d", e.StatusCode) }

type TransportError struct{ cause error }

func (e *TransportError) Error() string { return "TypeSafe request failed during transport" }
func (e *TransportError) Unwrap() error { return e.cause }

func (c *Client) Evaluate(ctx context.Context, r Request) (*Response, error) {
	raw, err := c.EvaluateRaw(ctx, r)
	if err != nil {
		return nil, err
	}
	var result Response
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, errors.New("API returned an invalid evaluation response")
	}
	return &result, nil
}

// EvaluateRaw preserves all response fields, including future API additions.
func (c *Client) EvaluateRaw(ctx context.Context, r Request) (json.RawMessage, error) {
	if r.Model == "" {
		r.Model = DefaultModel
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, "/systemone", b)
}
func (c *Client) ListModels(ctx context.Context) (*ModelsResponse, error) {
	raw, err := c.ListModelsRaw(ctx)
	if err != nil {
		return nil, err
	}
	var result ModelsResponse
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, errors.New("API returned an invalid model list")
	}
	return &result, nil
}
func (c *Client) ListModelsRaw(ctx context.Context) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/models", nil)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "judgement-go/0.1")
		resp, err := c.http.Do(req)
		// Ambiguous network failures are not retried: a paid evaluation may have run.
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, &TransportError{cause: err}
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024+1))
		resp.Body.Close()
		if readErr != nil {
			return nil, &TransportError{cause: readErr}
		}
		if len(data) > 32*1024*1024 {
			return nil, errors.New("API response exceeded 32 MiB")
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			trimmed := bytes.TrimSpace(data)
			if !json.Valid(data) || len(trimmed) == 0 || trimmed[0] != '{' {
				return nil, errors.New("API returned an invalid JSON object")
			}
			return json.RawMessage(data), nil
		}
		retryable := resp.StatusCode == 408 || resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 || resp.StatusCode == 529
		failure := &APIError{StatusCode: resp.StatusCode, RequestID: strings.ReplaceAll(resp.Header.Get("x-request-id"), c.key, "[REDACTED]"), Retryable: retryable}
		if !retryable || attempt >= c.retries {
			return nil, failure
		}
		delay := retryDelay(resp.Header, attempt)
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
			return nil, failure
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func retryDelay(h http.Header, attempt int) time.Duration {
	if v, err := strconv.ParseFloat(h.Get("retry-after-ms"), 64); err == nil && v >= 0 && v <= 86400000 {
		return time.Duration(v * float64(time.Millisecond))
	}
	if v, err := strconv.ParseFloat(h.Get("Retry-After"), 64); err == nil && v >= 0 && v <= 86400 {
		return time.Duration(v * float64(time.Second))
	}
	if t, err := http.ParseTime(h.Get("Retry-After")); err == nil && time.Until(t) > 0 {
		return time.Until(t)
	}
	d := 500 * time.Millisecond * time.Duration(1<<min(attempt, 5))
	return time.Duration(float64(d) * (0.75 + rand.Float64()*0.25))
}

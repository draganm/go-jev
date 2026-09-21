package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Environment variables read by [NewClient].
const (
	EnvAPIKey       = "TYPESAFE_API_KEY"
	EnvBaseURL      = "TYPESAFE_BASE_URL"
	EnvDefaultModel = "TYPESAFE_DEFAULT_MODEL"
)

// Defaults used by [NewClient].
const (
	DefaultBaseURL = "https://api.typesafe.ai"
	DefaultModel   = "jev-latest"
	DefaultTimeout = 10 * time.Second
)

// Model aliases.
const (
	ModelLatest  = "jev-latest"
	ModelPreview = "jev-preview"
)

const requestIDHeader = "X-Typesafe-Request-Id"

// ErrNoAPIKey is returned by [NewClient] when no API key is configured.
var ErrNoAPIKey = errors.New("typesafe: no API key (set " + EnvAPIKey + " or use WithAPIKey)")

// Client talks to the TypeSafe API. It is safe for concurrent use.
type Client struct {
	apiKey       string
	baseURL      string
	defaultModel string
	timeout      time.Duration
	userAgent    string
	httpClient   *http.Client
	retry        RetryPolicy
}

// Option configures a [Client].
type Option func(*Client)

// WithAPIKey sets the API key, overriding TYPESAFE_API_KEY.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithBaseURL sets the API base URL, overriding TYPESAFE_BASE_URL.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithDefaultModel sets the model used when [Request.Model] is empty,
// overriding TYPESAFE_DEFAULT_MODEL.
func WithDefaultModel(m string) Option { return func(c *Client) { c.defaultModel = m } }

// WithHTTPClient sets the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.httpClient = hc } }

// WithTimeout sets the timeout for each HTTP attempt. Zero disables it.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithRetryPolicy sets the retry policy. Use [NoRetries] to disable retries.
func WithRetryPolicy(p RetryPolicy) Option { return func(c *Client) { c.retry = p } }

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// NewClient creates a client. The API key, base URL and default model are
// read from the environment unless set with options.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		apiKey:       os.Getenv(EnvAPIKey),
		baseURL:      orDefault(os.Getenv(EnvBaseURL), DefaultBaseURL),
		defaultModel: orDefault(os.Getenv(EnvDefaultModel), DefaultModel),
		timeout:      DefaultTimeout,
		userAgent:    "go-jev",
		httpClient:   http.DefaultClient,
		retry:        DefaultRetryPolicy(),
	}
	for _, o := range opts {
		o(c)
	}
	if c.apiKey == "" {
		return nil, ErrNoAPIKey
	}
	if _, err := url.Parse(c.baseURL); err != nil {
		return nil, fmt.Errorf("typesafe: invalid base URL: %w", err)
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	return c, nil
}

// Ask evaluates the request's state against its questions
// (POST /v1/systemone).
func (c *Client) Ask(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("typesafe: nil request")
	}
	if len(req.Questions) == 0 {
		return nil, errors.New("typesafe: at least one question is required")
	}
	for id, q := range req.Questions {
		if q == nil {
			return nil, fmt.Errorf("typesafe: question %q is nil", id)
		}
		if err := q.validate(); err != nil {
			return nil, fmt.Errorf("typesafe: question %q: %w", id, err)
		}
	}
	payload := *req
	if payload.Model == "" {
		payload.Model = c.defaultModel
	}
	body, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("typesafe: encoding request: %w", err)
	}

	var res Response
	h, err := c.do(ctx, http.MethodPost, "/v1/systemone", body, &res)
	if err != nil {
		return nil, err
	}
	res.RequestID = h.Get(requestIDHeader)
	for id, q := range req.Questions {
		a, ok := res.Answers[id]
		if !ok {
			return nil, fmt.Errorf("typesafe: response has no answer for question %q", id)
		}
		if a.Type != q.QuestionType() {
			return nil, fmt.Errorf("typesafe: answer for question %q has type %q, want %q", id, a.Type, q.QuestionType())
		}
	}
	return &res, nil
}

// ListModels returns the models and aliases the account can use
// (GET /v1/models).
func (c *Client) ListModels(ctx context.Context) ([]ModelCard, error) {
	var res struct {
		Models []ModelCard `json:"models"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/v1/models", nil, &res); err != nil {
		return nil, err
	}
	return res.Models, nil
}

// do performs the request with retries and decodes a successful JSON
// response into out.
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) (http.Header, error) {
	u := c.baseURL + path
	for attempt := 0; ; attempt++ {
		h, retryAfter, err := c.attempt(ctx, method, u, body, out)
		if err == nil {
			return h, nil
		}
		if ctx.Err() != nil {
			return nil, err
		}
		if attempt >= c.retry.MaxRetries || !c.retry.shouldRetry(err) {
			return nil, err
		}
		t := time.NewTimer(c.retry.delay(attempt, retryAfter))
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, err
		case <-t.C:
		}
	}
}

// attempt performs a single HTTP exchange. On an API error it also returns the
// server-requested retry delay, or -1 if none was given.
func (c *Client) attempt(ctx context.Context, method, u string, body []byte, out any) (http.Header, time.Duration, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, -1, fmt.Errorf("typesafe: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, -1, &ConnectionError{Method: method, URL: u, Err: err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, &ConnectionError{Method: method, URL: u, Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, retryAfter(resp.Header), &APIError{
			StatusCode: resp.StatusCode,
			Method:     method,
			URL:        u,
			Body:       data,
			Header:     resp.Header,
			RequestID:  resp.Header.Get(requestIDHeader),
		}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, -1, fmt.Errorf("typesafe: decoding %s %s response: %w", method, u, err)
	}
	return resp.Header, -1, nil
}

// ConnectionError is a request that failed without an HTTP response,
// including timeouts.
type ConnectionError struct {
	Method string
	URL    string
	Err    error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("typesafe: %s %s: %v", e.Method, e.URL, e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

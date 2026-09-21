package jev

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	fast := DefaultRetryPolicy()
	fast.InitialBackoff = time.Millisecond
	fast.MaxBackoff = 2 * time.Millisecond
	c, err := NewClient(append([]Option{WithAPIKey("test-key"), WithBaseURL(srv.URL), WithRetryPolicy(fast)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestQuestionMarshal(t *testing.T) {
	cases := []struct {
		name string
		q    Question
		want string
	}{
		{
			"noul",
			Noul{Instructions: "Urgent?"},
			`{"type":"noul","instructions":"Urgent?"}`,
		},
		{
			"noul with criteria",
			Noul{Instructions: "Urgent?", Criteria: &NoulCriteria{True: "yes", False: "no"}},
			`{"type":"noul","instructions":"Urgent?","criteria":{"true":"yes","false":"no"}}`,
		},
		{
			"choice keeps option order",
			Choice{Instructions: "Team?", Options: []ChoiceOption{
				{Name: "technical", Description: "Bugs"},
				{Name: "billing"},
				{Name: "sales", Description: map[string]any{"covers": []string{"pricing"}}},
			}},
			`{"type":"choice","instructions":"Team?","criteria":{"technical":"Bugs","billing":null,"sales":{"covers":["pricing"]}}}`,
		},
		{
			"score",
			Score{Instructions: map[string]any{"question": "How angry?"}, Levels: Levels("Calm", "Angry")},
			`{"type":"score","instructions":{"question":"How angry?"},"criteria":["Calm","Angry"]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestQuestionValidation(t *testing.T) {
	many := make([]string, MaxChoiceOptions+1)
	for i := range many {
		many[i] = strings.Repeat("x", i+1)
	}
	cases := map[string]Question{
		"noul without instructions":    Noul{},
		"choice without options":       Choice{Instructions: "?"},
		"choice with too many":         Choice{Instructions: "?", Options: ChoiceOptions(many...)},
		"choice with duplicate":        Choice{Instructions: "?", Options: ChoiceOptions("a", "a")},
		"choice with empty name":       Choice{Instructions: "?", Options: ChoiceOptions("")},
		"score with one level":         Score{Instructions: "?", Levels: Levels("a")},
		"score with too many levels":   Score{Instructions: "?", Levels: make([]any, MaxScoreLevels+1)},
		"score with empty instruction": Score{Instructions: "", Levels: Levels("a", "b")},
	}
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	for name, q := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := c.Ask(t.Context(), &Request{State: "s", Questions: map[string]Question{"q": q}})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if _, err := c.Ask(t.Context(), &Request{State: "s"}); err == nil {
		t.Fatal("expected error for no questions")
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("invalid requests reached the server %d times", n)
	}
}

func TestAsk(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var body struct {
			State     any                        `json:"state"`
			Model     string                     `json:"model"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != DefaultModel {
			t.Errorf("model = %q, want default %q", body.Model, DefaultModel)
		}
		if body.State != "Help! My payouts have been failing for 3 days." {
			t.Errorf("state = %v", body.State)
		}
		if len(body.Questions) != 3 {
			t.Errorf("got %d questions", len(body.Questions))
		}
		w.Header().Set("X-Typesafe-Request-Id", "req-123")
		io.WriteString(w, `{
			"model": "jev-1.13.0",
			"answers": {
				"is_urgent": {"type": "noul", "noul": 0.95},
				"department": {"type": "choice", "choice": "billing",
					"probabilities": {"billing": 0.88, "technical": 0.12, "sales": 0.0}, "confidence": 0.81},
				"frustration": {"type": "score", "score": 1.05,
					"legend": {"0": "Calm", "1": "Frustrated", "2": "Very angry"},
					"probabilities": {"0": 0.0, "1": 0.95, "2": 0.05}, "confidence": 0.92}
			},
			"usage": {"input_tokens": 318, "output_tokens": 34}
		}`)
	})

	res, err := c.Ask(t.Context(), &Request{
		State: "Help! My payouts have been failing for 3 days.",
		Questions: map[string]Question{
			"is_urgent": Noul{Instructions: "Does this convey urgency?"},
			"department": Choice{Instructions: "Which team should handle this?", Options: []ChoiceOption{
				{Name: "billing", Description: "Payments, invoicing, refunds"},
				{Name: "technical", Description: "Bugs, outages, integrations"},
				{Name: "sales", Description: "Pricing, upgrades, new accounts"},
			}},
			"frustration": Score{Instructions: "How frustrated is the customer?", Levels: Levels("Calm", "Frustrated", "Very angry")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "jev-1.13.0" || res.RequestID != "req-123" {
		t.Errorf("model %q, request id %q", res.Model, res.RequestID)
	}
	if res.Usage != (Usage{InputTokens: 318, OutputTokens: 34}) {
		t.Errorf("usage = %+v", res.Usage)
	}
	if v, err := res.Noul("is_urgent"); err != nil || v != 0.95 {
		t.Errorf("noul = %v, %v", v, err)
	}
	if a, err := res.Choice("department"); err != nil || a.Choice != "billing" || a.Confidence != 0.81 || a.Probabilities["technical"] != 0.12 {
		t.Errorf("choice = %+v, %v", a, err)
	}
	if a, err := res.Score("frustration"); err != nil || a.Score != 1.05 || a.Legend["2"] != "Very angry" {
		t.Errorf("score = %+v, %v", a, err)
	}
	if _, err := res.Score("is_urgent"); err == nil {
		t.Error("expected type mismatch error")
	}
	if _, err := res.Noul("missing"); err == nil {
		t.Error("expected missing answer error")
	}
}

func TestAskExplicitModel(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Model string }
		json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "jev-1.13.0" {
			t.Errorf("model = %q", body.Model)
		}
		io.WriteString(w, `{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},"usage":{}}`)
	}, WithDefaultModel("ignored"))
	_, err := c.Ask(t.Context(), &Request{Model: "jev-1.13.0", State: "s", Questions: map[string]Question{"q": Noul{Instructions: "?"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAskValidatesResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"model":"m","answers":{"q":{"type":"choice","choice":"a"}},"usage":{}}`)
	})
	q := map[string]Question{"q": Noul{Instructions: "?"}}
	if _, err := c.Ask(t.Context(), &Request{State: "s", Questions: q}); err == nil {
		t.Fatal("expected error for mismatched answer type")
	}
	q = map[string]Question{"q": Choice{Instructions: "?", Options: ChoiceOptions("a")}, "other": Noul{Instructions: "?"}}
	if _, err := c.Ask(t.Context(), &Request{State: "s", Questions: q}); err == nil {
		t.Fatal("expected error for missing answer")
	}
}

func TestListModels(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"models":[{"name":"jev-latest","description":"Stable","release_date":"2026-05-01"}]}`)
	})
	models, err := c.ListModels(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0] != (ModelCard{Name: "jev-latest", Description: "Stable", ReleaseDate: "2026-05-01"}) {
		t.Fatalf("models = %+v", models)
	}
}

func TestAPIErrors(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{400, ErrBadRequest},
		{401, ErrAuthentication},
		{403, ErrPermissionDenied},
		{404, ErrNotFound},
		{422, ErrUnprocessable},
		{429, ErrRateLimited},
		{529, ErrOverloaded},
		{503, ErrServer},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Typesafe-Request-Id", "req-err")
				w.WriteHeader(tc.status)
				io.WriteString(w, `{"detail":"nope"}`)
			}, WithRetryPolicy(NoRetries))
			_, err := c.ListModels(t.Context())
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			apiErr, ok := errors.AsType[*APIError](err)
			if !ok {
				t.Fatalf("not an *APIError: %T", err)
			}
			if apiErr.StatusCode != tc.status || apiErr.RequestID != "req-err" || string(apiErr.Body) != `{"detail":"nope"}` {
				t.Fatalf("apiErr = %+v", apiErr)
			}
		})
	}
}

func TestRetries(t *testing.T) {
	t.Run("retries overloaded then succeeds", func(t *testing.T) {
		var calls atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if len(b) == 0 {
				t.Error("empty body on retry")
			}
			if calls.Add(1) < 3 {
				w.WriteHeader(StatusOverloaded)
				return
			}
			io.WriteString(w, `{"model":"m","answers":{"q":{"type":"noul","noul":1}},"usage":{}}`)
		})
		_, err := c.Ask(t.Context(), &Request{State: "s", Questions: map[string]Question{"q": Noul{Instructions: "?"}}})
		if err != nil {
			t.Fatal(err)
		}
		if n := calls.Load(); n != 3 {
			t.Fatalf("calls = %d, want 3", n)
		}
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		var calls atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
		})
		if _, err := c.ListModels(t.Context()); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
		if n := calls.Load(); n != 3 {
			t.Fatalf("calls = %d, want 3", n)
		}
	})

	t.Run("does not retry client errors", func(t *testing.T) {
		var calls atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusUnprocessableEntity)
		})
		c.ListModels(t.Context())
		if n := calls.Load(); n != 1 {
			t.Fatalf("calls = %d, want 1", n)
		}
	})

	t.Run("retries timeouts", func(t *testing.T) {
		var calls atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				<-r.Context().Done()
				return
			}
			io.WriteString(w, `{"models":[]}`)
		}, WithTimeout(50*time.Millisecond))
		if _, err := c.ListModels(t.Context()); err != nil {
			t.Fatal(err)
		}
		if n := calls.Load(); n != 2 {
			t.Fatalf("calls = %d, want 2", n)
		}
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			cancel()
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		start := time.Now()
		if _, err := c.ListModels(ctx); err == nil {
			t.Fatal("expected error")
		}
		if time.Since(start) > time.Second {
			t.Fatal("retried after cancellation")
		}
	})
}

func TestRetryDelay(t *testing.T) {
	p := DefaultRetryPolicy()
	p.Jitter = 0
	if d := p.delay(0, -1); d != 500*time.Millisecond {
		t.Errorf("attempt 0 = %v", d)
	}
	if d := p.delay(1, -1); d != time.Second {
		t.Errorf("attempt 1 = %v", d)
	}
	if d := p.delay(10, -1); d != 5*time.Second {
		t.Errorf("attempt 10 = %v, want capped", d)
	}
	if d := p.delay(0, 2*time.Second); d != 2*time.Second {
		t.Errorf("server delay = %v", d)
	}
	if d := p.delay(0, 2*time.Minute); d != 500*time.Millisecond {
		t.Errorf("oversized server delay = %v, want backoff", d)
	}

	h := http.Header{}
	if d := retryAfter(h); d != -1 {
		t.Errorf("no header = %v", d)
	}
	h.Set("Retry-After", "3")
	if d := retryAfter(h); d != 3*time.Second {
		t.Errorf("Retry-After = %v", d)
	}
	h.Set("Retry-After-Ms", "250")
	if d := retryAfter(h); d != 250*time.Millisecond {
		t.Errorf("retry-after-ms = %v", d)
	}
}

func TestNewClientEnv(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	if _, err := NewClient(); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("err = %v", err)
	}
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvBaseURL, "https://example.test/")
	t.Setenv(EnvDefaultModel, "jev-preview")
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.apiKey != "env-key" || c.baseURL != "https://example.test" || c.defaultModel != "jev-preview" {
		t.Fatalf("client = %+v", c)
	}
}

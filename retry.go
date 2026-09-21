package jev

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy controls how failed requests are retried. Delays grow
// exponentially from InitialBackoff, capped at MaxBackoff.
type RetryPolicy struct {
	// MaxRetries is the number of retries after the initial attempt. Zero
	// disables retries.
	MaxRetries int
	// InitialBackoff is the first backoff delay; it doubles on each retry.
	InitialBackoff time.Duration
	// MaxBackoff caps the backoff delay.
	MaxBackoff time.Duration
	// Jitter is the fraction of each backoff delay randomly subtracted, 0 to 1.
	Jitter float64
	// RetryStatus reports whether an HTTP status should be retried. If nil,
	// 408, 429 and 5xx (including 529 Overloaded) are retried.
	RetryStatus func(status int) bool
	// RetryConnectionErrors retries failures without an HTTP response,
	// including timeouts.
	RetryConnectionErrors bool
	// RespectRetryAfter honors the Retry-After and retry-after-ms headers,
	// up to MaxRetryAfter; longer server delays fall back to backoff.
	RespectRetryAfter bool
	MaxRetryAfter     time.Duration
}

// DefaultRetryPolicy returns the policy used by [NewClient], matching the
// official SDKs.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:            2,
		InitialBackoff:        500 * time.Millisecond,
		MaxBackoff:            5 * time.Second,
		Jitter:                0.25,
		RetryConnectionErrors: true,
		RespectRetryAfter:     true,
		MaxRetryAfter:         time.Minute,
	}
}

// NoRetries is a policy that never retries.
var NoRetries = RetryPolicy{}

func defaultRetryStatus(s int) bool {
	return s == http.StatusRequestTimeout || s == http.StatusTooManyRequests || s >= 500
}

func (p RetryPolicy) shouldRetry(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if p.RetryStatus != nil {
			return p.RetryStatus(apiErr.StatusCode)
		}
		return defaultRetryStatus(apiErr.StatusCode)
	}
	if _, ok := errors.AsType[*ConnectionError](err); ok {
		return p.RetryConnectionErrors
	}
	return false
}

// delay returns how long to wait before retry number attempt+1. serverDelay
// is the server-requested delay, or negative if none was given.
func (p RetryPolicy) delay(attempt int, serverDelay time.Duration) time.Duration {
	if p.RespectRetryAfter && serverDelay >= 0 && serverDelay <= p.MaxRetryAfter {
		return serverDelay
	}
	d := p.InitialBackoff << attempt
	if d > p.MaxBackoff || d <= 0 {
		d = p.MaxBackoff
	}
	if p.Jitter > 0 {
		d -= time.Duration(rand.Float64() * p.Jitter * float64(d))
	}
	return d
}

// retryAfter parses retry-after-ms and Retry-After, returning -1 if neither
// is present or valid.
func retryAfter(h http.Header) time.Duration {
	if v := h.Get("Retry-After-Ms"); v != "" {
		if ms, err := strconv.ParseFloat(v, 64); err == nil && ms >= 0 {
			return time.Duration(ms * float64(time.Millisecond))
		}
	}
	v := h.Get("Retry-After")
	if v == "" {
		return -1
	}
	if s, err := strconv.ParseFloat(v, 64); err == nil && s >= 0 {
		return time.Duration(s * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return -1
}

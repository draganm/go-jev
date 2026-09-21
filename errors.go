package jev

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors matched by [APIError] via errors.Is.
var (
	ErrBadRequest       = errors.New("bad request")
	ErrAuthentication   = errors.New("authentication failed")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotFound         = errors.New("not found")
	ErrUnprocessable    = errors.New("unprocessable entity")
	ErrRateLimited      = errors.New("rate limited")
	ErrOverloaded       = errors.New("overloaded")
	ErrServer           = errors.New("server error")
)

// StatusOverloaded is the non-standard status TypeSafe returns when it is
// temporarily overloaded.
const StatusOverloaded = 529

// APIError is an unsuccessful HTTP response from the API.
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Method and URL identify the request, without query or credentials.
	Method string
	URL    string
	// Body is the raw response body, usually JSON describing the problem.
	Body []byte
	// Header holds the response headers.
	Header http.Header
	// RequestID is the x-typesafe-request-id response header, if present.
	RequestID string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("typesafe: %s %s: %d %s", e.Method, e.URL, e.StatusCode, http.StatusText(e.StatusCode))
	if e.StatusCode == StatusOverloaded {
		msg = fmt.Sprintf("typesafe: %s %s: %d Overloaded", e.Method, e.URL, e.StatusCode)
	}
	if len(e.Body) > 0 {
		body := e.Body
		if len(body) > 512 {
			body = body[:512]
		}
		msg += ": " + string(body)
	}
	if e.RequestID != "" {
		msg += " (request id " + e.RequestID + ")"
	}
	return msg
}

// Unwrap returns the sentinel error matching the status code, if any.
func (e *APIError) Unwrap() error {
	switch s := e.StatusCode; {
	case s == http.StatusBadRequest:
		return ErrBadRequest
	case s == http.StatusUnauthorized:
		return ErrAuthentication
	case s == http.StatusForbidden:
		return ErrPermissionDenied
	case s == http.StatusNotFound:
		return ErrNotFound
	case s == http.StatusUnprocessableEntity:
		return ErrUnprocessable
	case s == http.StatusTooManyRequests:
		return ErrRateLimited
	case s == StatusOverloaded:
		return ErrOverloaded
	case s >= 500:
		return ErrServer
	}
	return nil
}

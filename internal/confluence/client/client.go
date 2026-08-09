package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
)

// Client performs bounded, GET-only Confluence REST API requests.
type Client struct {
	baseURL          *url.URL
	httpClient       *http.Client
	maxResponseBytes int64
}

// HTTPError is a sanitized Confluence response failure.
type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
	Detail     any
}

// Error returns the stable code and sanitized message.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New builds a Confluence REST client rooted at /rest/api under base.
func New(base string, hc *http.Client, maxResponseBytes int64) *Client {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		u = &url.URL{}
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = 10 << 20
	}
	return &Client{baseURL: u, httpClient: hc, maxResponseBytes: maxResponseBytes}
}

// GetJSON issues one authenticated GET request and decodes a JSON response into out.
func (c *Client) GetJSON(ctx context.Context, cred auth.Credential, apiPath string, query map[string][]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urlFor(apiPath, query), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(cred.Username(), cred.Password())
	req.Header.Set("Accept", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return &HTTPError{Code: "UPSTREAM_UNREACHABLE", Message: "Confluence request failed"}
	}
	defer res.Body.Close()
	limited := io.LimitReader(res.Body, c.maxResponseBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return &HTTPError{StatusCode: res.StatusCode, Code: "UPSTREAM_SERVER_ERROR", Message: "Confluence response could not be read"}
	}
	if int64(len(b)) > c.maxResponseBytes {
		return &HTTPError{StatusCode: res.StatusCode, Code: "RESPONSE_TOO_LARGE", Message: "Confluence response exceeded ATLASSIAN_MAX_RESPONSE_BYTES"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return mapHTTPError(res.StatusCode, b)
	}
	if out == nil || len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &HTTPError{StatusCode: res.StatusCode, Code: "UPSTREAM_SERVER_ERROR", Message: "Confluence response was not valid JSON"}
	}
	return nil
}

// urlFor preserves a configured context path while adding Confluence's /rest/api prefix.
func (c *Client) urlFor(apiPath string, query map[string][]string) string {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + "/rest/api/" + strings.TrimLeft(apiPath, "/")
	q := url.Values{}
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func mapHTTPError(status int, body []byte) *HTTPError {
	code := mapStatusCode(status)
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) == nil {
		return &HTTPError{StatusCode: status, Code: code, Message: "Confluence returned an error", Detail: observability.Redact(parsed)}
	}
	return &HTTPError{StatusCode: status, Code: code, Message: "Confluence returned a non-JSON error"}
}

func mapStatusCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "UPSTREAM_AUTHENTICATION_FAILED"
	case http.StatusForbidden:
		return "UPSTREAM_PERMISSION_DENIED"
	case http.StatusNotFound:
		return "NOT_FOUND_OR_NOT_VISIBLE"
	case http.StatusConflict:
		return "UPSTREAM_CONFLICT"
	case http.StatusTooManyRequests:
		return "UPSTREAM_RATE_LIMITED"
	default:
		if status >= 500 {
			return "UPSTREAM_SERVER_ERROR"
		}
		return "VALIDATION_ERROR"
	}
}

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/chiendao1808/atlassian-mcp/internal/observability"
)

var errInvalidPath = errors.New("Bitbucket path segment is invalid")

type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
	Detail     any
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type upstreamErrors struct {
	Errors []map[string]any `json:"errors"`
}

func (c *Client) mapHTTPError(status int, body []byte) *HTTPError {
	code := mapStatusCode(status)
	var parsed upstreamErrors
	if json.Unmarshal(body, &parsed) == nil && len(parsed.Errors) > 0 {
		detail := c.sanitize(parsed.Errors)
		message := "Bitbucket returned an error"
		if first, ok := detail.([]map[string]any); ok {
			if raw, ok := first[0]["message"].(string); ok && raw != "" {
				message = raw
			}
		}
		return &HTTPError{StatusCode: status, Code: code, Message: message, Detail: detail}
	}
	return &HTTPError{StatusCode: status, Code: code, Message: "Bitbucket returned a non-JSON error"}
}

func mapStatusCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "UPSTREAM_AUTHENTICATION_FAILED"
	case http.StatusForbidden:
		return "UPSTREAM_PERMISSION_DENIED"
	case http.StatusNotFound:
		return "UPSTREAM_NOT_FOUND"
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

func (c *Client) sanitize(v any) any {
	return replaceToken(observability.Redact(v), c.token)
}

func replaceToken(v any, token string) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = replaceToken(v, token)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(x))
		for i := range x {
			out[i] = replaceToken(map[string]any(x[i]), token).(map[string]any)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = replaceToken(x[i], token)
		}
		return out
	case string:
		return sanitizeToken(x, token)
	default:
		return v
	}
}

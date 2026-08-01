package observability

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const redacted = "[REDACTED]"

// Redact recursively removes sensitive fields and auth-bearing string values from JSON-like data.
func Redact(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			if sensitiveKey(k) {
				out[k] = redacted
			} else {
				out[k] = Redact(v)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = Redact(x[i])
		}
		return out
	case string:
		return redactString(x)
	default:
		return v
	}
}

// FormatSanitized formats arbitrary log/tool data after first normalizing typed structs to JSON-like data.
func FormatSanitized(v any) string {
	normalized, ok := normalizeJSON(v)
	if !ok {
		return fmt.Sprint(Redact(fmt.Sprint(v)))
	}
	b, err := json.Marshal(Redact(normalized))
	if err == nil {
		return string(b)
	}
	return fmt.Sprint(Redact(fmt.Sprint(v)))
}

// normalizeJSON gives redaction one representation for typed tool inputs, result envelopes, and log payloads.
func normalizeJSON(v any) (any, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false
	}
	return out, true
}

// sensitiveKey identifies field names that should never expose their value in logs or MCP results.
func sensitiveKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "password") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "authorization") ||
		strings.Contains(k, "secret")
}

// redactString removes common auth headers and credentials embedded in URLs.
func redactString(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "basic ") || strings.HasPrefix(strings.ToLower(s), "bearer ") {
		return redacted
	}
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		if u.User != nil {
			u.User = url.User(redacted)
		}
		q := u.Query()
		for key := range q {
			if sensitiveKey(key) {
				q.Set(key, redacted)
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	}
	return s
}

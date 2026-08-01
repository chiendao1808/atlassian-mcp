package observability

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const redacted = "[REDACTED]"

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

func FormatSanitized(v any) string {
	b, err := json.Marshal(Redact(v))
	if err == nil {
		return string(b)
	}
	return fmt.Sprint(Redact(fmt.Sprint(v)))
}

func sensitiveKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "password") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "authorization") ||
		strings.Contains(k, "secret")
}

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

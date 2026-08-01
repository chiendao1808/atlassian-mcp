package observability

import (
	"strings"
	"testing"
)

func TestRedactSecretsRecursively(t *testing.T) {
	input := map[string]any{
		"Authorization": "Basic abc",
		"password":      "sentinel-password",
		"url":           "https://user:secret@example.com/path?token=abc",
		"nested": map[string]any{
			"BITBUCKET_BEARER_TOKEN": "sentinel-token",
		},
	}
	got := Redact(input)
	text := strings.ToLower(FormatSanitized(got))
	for _, secret := range []string{"sentinel-password", "sentinel-token", "basic abc", "secret", "token=abc"} {
		if strings.Contains(text, secret) {
			t.Fatalf("sanitized output still contains %q: %s", secret, text)
		}
	}
}

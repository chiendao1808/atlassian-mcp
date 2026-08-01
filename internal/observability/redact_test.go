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

func TestFormatSanitizedRedactsTypedToolInput(t *testing.T) {
	input := struct {
		Tool  string `json:"tool"`
		Input struct {
			Username      string `json:"username"`
			Password      string `json:"password"`
			Authorization string `json:"authorization"`
		} `json:"input"`
	}{
		Tool: "jira_authenticate",
	}
	input.Input.Username = "alice"
	input.Input.Password = "sentinel-password"
	input.Input.Authorization = "Basic sentinel-auth"

	text := strings.ToLower(FormatSanitized(input))
	for _, secret := range []string{"sentinel-password", "sentinel-auth", "basic "} {
		if strings.Contains(text, secret) {
			t.Fatalf("sanitized typed input still contains %q: %s", secret, text)
		}
	}
}

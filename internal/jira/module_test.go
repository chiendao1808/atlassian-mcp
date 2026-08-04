package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

func TestModuleUsesSharedTLSClientConfig(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not pem"), 0600); err != nil {
		t.Fatal(err)
	}
	module := NewModule(func(key string) string {
		switch key {
		case "JIRA_BASE_URL":
			return "https://jira.internal.example.com/jira"
		case "JIRA_CA_FILE":
			return ca
		default:
			return ""
		}
	})
	err := module.ValidateStaticConfig(config.Shared{TLSVerify: true})
	if err == nil || !strings.Contains(err.Error(), "PEM") {
		t.Fatalf("expected shared transport CA validation error, got %v", err)
	}
}

func TestAutoAuthenticateUsesEnvironmentCredentialsWhenPresent(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "6.4.14"})
		case "/rest/api/2/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "alice"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"JIRA_BASE_URL": server.URL,
		"JIRA_USERNAME": "alice",
		"JIRA_PASSWORD": "secret",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	module.AutoAuthenticate(context.Background(), &stderr)

	if got := paths; len(got) != 2 || got[0] != "/rest/api/2/serverInfo" || got[1] != "/rest/api/2/myself" {
		t.Fatalf("paths=%v", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestAutoAuthenticateSkipsNetworkWhenCredentialsAbsent(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	env := map[string]string{"JIRA_BASE_URL": server.URL}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	module.AutoAuthenticate(context.Background(), &stderr)

	if calls != 0 {
		t.Fatalf("auto-authenticate without credentials sent %d requests", calls)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr when credentials are absent: %s", stderr.String())
	}
}

func TestAutoAuthenticateLogsWarningOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"JIRA_BASE_URL": server.URL,
		"JIRA_USERNAME": "alice",
		"JIRA_PASSWORD": "wrong",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	module.AutoAuthenticate(context.Background(), &stderr)

	if !strings.Contains(stderr.String(), "automatic authentication") {
		t.Fatalf("expected a logged warning, got: %q", stderr.String())
	}
}

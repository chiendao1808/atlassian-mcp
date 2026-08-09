package confluence

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
	ctools "github.com/chiendao1808/atlassian-mcp/internal/confluence/tools"
)

func TestModuleUsesSharedTLSClientConfig(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not pem"), 0600); err != nil {
		t.Fatal(err)
	}
	module := NewModule(func(key string) string {
		switch key {
		case "CONFLUENCE_BASE_URL":
			return "https://wiki.internal.example.com/confluence"
		case "CONFLUENCE_CA_FILE":
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
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": "alice"})
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"CONFLUENCE_BASE_URL": server.URL,
		"CONFLUENCE_USERNAME": "alice",
		"CONFLUENCE_PASSWORD": "secret",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	module.AutoAuthenticate(context.Background(), &stderr)

	if got := paths; len(got) != 1 || got[0] != "/rest/api/user/current" {
		t.Fatalf("paths=%v", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestAutoAuthenticateSkipsNetworkWhenCredentialsAbsent(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "neither credential", env: nil},
		{name: "username only", env: map[string]string{"CONFLUENCE_USERNAME": "alice"}},
		{name: "password only", env: map[string]string{"CONFLUENCE_PASSWORD": "secret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls++
			}))
			t.Cleanup(server.Close)

			env := map[string]string{"CONFLUENCE_BASE_URL": server.URL}
			for key, value := range tt.env {
				env[key] = value
			}
			module := NewModule(func(key string) string { return env[key] })
			if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
				t.Fatalf("ValidateStaticConfig: %v", err)
			}

			var stderr bytes.Buffer
			module.AutoAuthenticate(context.Background(), &stderr)

			if calls != 0 {
				t.Fatalf("auto-authenticate without complete credentials sent %d requests", calls)
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr when credentials are incomplete: %s", stderr.String())
			}
		})
	}
}

func TestAutoAuthenticateLogsWarningOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"CONFLUENCE_BASE_URL": server.URL,
		"CONFLUENCE_USERNAME": "alice",
		"CONFLUENCE_PASSWORD": "wrong",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	module.AutoAuthenticate(context.Background(), &stderr)

	if !strings.Contains(stderr.String(), "automatic authentication") || strings.Contains(stderr.String(), "wrong") {
		t.Fatalf("expected a sanitized logged warning, got: %q", stderr.String())
	}
}

func TestAutoAuthenticateSkipsWhenSessionAlreadyExists(t *testing.T) {
	envRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, _, _ := r.BasicAuth()
		if username == "env-alice" {
			envRequests++
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": username})
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"CONFLUENCE_BASE_URL": server.URL,
		"CONFLUENCE_USERNAME": "env-alice",
		"CONFLUENCE_PASSWORD": "env-secret",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}
	explicit := module.svc.Authenticate(context.Background(), ctools.AuthenticateInput{Username: "explicit-bob", Password: "explicit-secret"})
	if !explicit.Success {
		t.Fatalf("explicit=%+v", explicit)
	}

	var stderr bytes.Buffer
	module.AutoAuthenticate(context.Background(), &stderr)

	if envRequests != 0 {
		t.Fatalf("auto-auth sent %d env credential requests despite existing session", envRequests)
	}
	snap, err := module.svc.Store().Snapshot()
	if err != nil || snap.Username() != "explicit-bob" {
		t.Fatalf("session changed: snap=%+v err=%v", snap, err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestAutoAuthenticateRaceLeavesImmediateReadUnauthenticated(t *testing.T) {
	authStarted := make(chan struct{})
	releaseAuth := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(authStarted)
		<-releaseAuth
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": "alice"})
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"CONFLUENCE_BASE_URL": server.URL,
		"CONFLUENCE_USERNAME": "alice",
		"CONFLUENCE_PASSWORD": "secret",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		module.AutoAuthenticate(context.Background(), &stderr)
	}()
	<-authStarted

	out := module.svc.SearchContent(context.Background(), ctools.SearchContentInput{CQL: "type = page"})
	if out.Success || out.Error == nil || out.Error.Code != "CONFLUENCE_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}

	close(releaseAuth)
	<-done
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestAutoAuthenticateDoesNotOverwriteNewerExplicitSession(t *testing.T) {
	authStarted := make(chan struct{})
	releaseAuto := make(chan struct{})
	autoDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, _, _ := r.BasicAuth()
		if username == "env-alice" {
			close(authStarted)
			<-releaseAuto
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": "env-alice"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": username})
	}))
	t.Cleanup(server.Close)

	env := map[string]string{
		"CONFLUENCE_BASE_URL": server.URL,
		"CONFLUENCE_USERNAME": "env-alice",
		"CONFLUENCE_PASSWORD": "env-secret",
	}
	module := NewModule(func(key string) string { return env[key] })
	if err := module.ValidateStaticConfig(config.Shared{}); err != nil {
		t.Fatalf("ValidateStaticConfig: %v", err)
	}

	var stderr bytes.Buffer
	go func() {
		defer close(autoDone)
		module.AutoAuthenticate(context.Background(), &stderr)
	}()
	<-authStarted

	explicit := module.svc.Authenticate(context.Background(), ctools.AuthenticateInput{Username: "explicit-bob", Password: "explicit-secret"})
	if !explicit.Success {
		t.Fatalf("explicit=%+v", explicit)
	}

	close(releaseAuto)
	<-autoDone
	snap, err := module.svc.Store().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Username() != "explicit-bob" {
		t.Fatalf("auto-auth overwrote newer explicit session: %s", snap.Username())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

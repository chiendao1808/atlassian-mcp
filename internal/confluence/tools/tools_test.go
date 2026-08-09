package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence/client"
)

func basicAuthValue(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func newTestService(baseURL string, hc *http.Client) *Service {
	return newTestServiceWithEnv(baseURL, hc, nil)
}

func newTestServiceWithEnv(baseURL string, hc *http.Client, env map[string]string) *Service {
	store := auth.NewSessionStore()
	getenv := func(key string) string { return env[key] }
	return NewService(client.New(baseURL, hc, 1<<20), store, getenv)
}

func intPtr(v int) *int { return &v }

func TestAuthenticateActivatesKnownCurrentUser(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": "alice"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.Authenticate(context.Background(), AuthenticateInput{Username: "alice", Password: "secret"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if got := paths; len(got) != 1 || got[0] != "/rest/api/user/current" {
		t.Fatalf("paths=%v", got)
	}
}

func TestAuthenticateRejectsAnonymousUserAndPreservesOldSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "anonymous"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "old-secret"))
	out := svc.Authenticate(context.Background(), AuthenticateInput{Username: "anon", Password: "secret"})
	if out.Success || out.Error == nil || out.Error.Code != "CONFLUENCE_AUTHENTICATION_FAILED" {
		t.Fatalf("out=%+v", out)
	}
	snap, err := svc.Store().Snapshot()
	if err != nil || snap.Password() != "old-secret" {
		t.Fatalf("old session was not preserved: snap=%+v err=%v", snap, err)
	}
}

func TestAuthenticateFallsBackToEnvironmentWhenInputOmitted(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": "alice"})
	}))
	t.Cleanup(server.Close)

	svc := newTestServiceWithEnv(server.URL, server.Client(), map[string]string{
		"CONFLUENCE_USERNAME": "alice",
		"CONFLUENCE_PASSWORD": "secret",
	})
	out := svc.Authenticate(context.Background(), AuthenticateInput{})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if want := "Basic " + basicAuthValue("alice", "secret"); gotAuth != want {
		t.Fatalf("authorization=%q, want %q", gotAuth, want)
	}
}

func TestSearchContentPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).SearchContent(context.Background(), SearchContentInput{CQL: "type = page"})
	if out.Success || out.Error == nil || out.Error.Code != "CONFLUENCE_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestSearchContentSendsRawCQLAndDefaultLimit(t *testing.T) {
	var seenPath, seenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "start": 0, "limit": 25, "_links": map[string]any{"self": "x"}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.SearchContent(context.Background(), SearchContentInput{CQL: "space = ENG AND type = page", Expand: "space,version"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if seenPath != "/rest/api/content/search" || !strings.Contains(seenQuery, "limit=25") || strings.Contains(seenQuery, "start=") || !strings.Contains(seenQuery, "cql=space+%3D+ENG+AND+type+%3D+page") || !strings.Contains(seenQuery, "expand=space%2Cversion") {
		t.Fatalf("path=%q query=%q", seenPath, seenQuery)
	}
	data := out.Data.(map[string]any)
	if data["results"] == nil || data["_links"] == nil {
		t.Fatalf("upstream shape not preserved: %+v", data)
	}
}

func TestGetContentValidatesIDAndSendsOptionalQuery(t *testing.T) {
	calls := 0
	var seenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		seenQuery = r.URL.RawQuery
		if r.URL.Path != "/rest/api/content/12345" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "12345", "_expandable": map[string]any{"body": ""}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	bad := svc.GetContent(context.Background(), GetContentInput{ContentID: "123/45"})
	if bad.Success || bad.Error == nil || bad.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("bad=%+v", bad)
	}
	zero := svc.GetContent(context.Background(), GetContentInput{ContentID: "12345", Version: intPtr(0)})
	if zero.Success || zero.Error == nil || zero.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("zero=%+v", zero)
	}
	good := svc.GetContent(context.Background(), GetContentInput{ContentID: "12345", Status: "current", Version: intPtr(3), Expand: "body.storage,body.view"})
	if !good.Success || calls != 1 {
		t.Fatalf("good=%+v calls=%d", good, calls)
	}
	if !strings.Contains(seenQuery, "status=current") || !strings.Contains(seenQuery, "version=3") || !strings.Contains(seenQuery, "expand=body.storage%2Cbody.view") {
		t.Fatalf("query=%q", seenQuery)
	}
}

func TestConfluenceToolDefinitionsAreExactlyV1ReadTools(t *testing.T) {
	defs := Definitions()
	seen := map[string]bool{}
	for _, def := range defs {
		seen[def.Name] = true
		if def.Annotations == nil || def.Annotations.OpenWorldHint == nil || !*def.Annotations.OpenWorldHint {
			t.Fatalf("%s missing open-world annotation", def.Name)
		}
		description := strings.ToLower(def.Description)
		if def.Name == "confluence_authenticate" && !strings.Contains(description, "explicit setup/recovery") {
			t.Fatalf("authenticate description must mention setup/recovery: %q", def.Description)
		}
		if def.Name != "confluence_authenticate" && !def.Annotations.ReadOnlyHint {
			t.Fatalf("%s must be read-only", def.Name)
		}
		if def.Name != "confluence_authenticate" && !strings.Contains(description, "authenticated confluence session") {
			t.Fatalf("%s description must mention authenticated Confluence session: %q", def.Name, def.Description)
		}
	}
	for _, name := range []string{"confluence_authenticate", "confluence_search_content", "confluence_get_content"} {
		if !seen[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
	if len(defs) != 3 {
		t.Fatalf("expected exactly 3 tool definitions, got %d", len(defs))
	}
}

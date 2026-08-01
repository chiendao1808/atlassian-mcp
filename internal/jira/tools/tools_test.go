package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/jira/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/jira/client"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
)

func newTestService(baseURL string, hc *http.Client) *Service {
	store := auth.NewSessionStore()
	return NewService(client.New(baseURL, hc, 1<<20), store)
}

func TestGetIssueRejectsPathSeparatorWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.GetIssue(context.Background(), GetIssueInput{IssueIDOrKey: "PROJ/1"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid issue key sent %d requests", calls)
	}
}

func TestToolErrorPreservesSanitizedJiraDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["bad"],"errors":{"password":"sentinel-secret"}}`))
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.GetIssue(context.Background(), GetIssueInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Detail == nil {
		t.Fatalf("out=%+v", out)
	}
	text := strings.ToLower(observability.FormatSanitized(out.Error.Detail))
	if strings.Contains(text, "sentinel-secret") || !strings.Contains(text, "errormessages") {
		t.Fatalf("detail not preserved and sanitized: %s", text)
	}
}
func TestGetIssuePreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).GetIssue(context.Background(), GetIssueInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestAuthenticateActivatesOnlyAfterServerInfoAndMyself(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "6.4.14"})
		case "/rest/api/2/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "alice", "displayName": "Alice"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.Authenticate(context.Background(), AuthenticateInput{Username: "alice", Password: "secret"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if got := paths; len(got) != 2 || got[0] != "/rest/api/2/serverInfo" || got[1] != "/rest/api/2/myself" {
		t.Fatalf("paths=%v", got)
	}
}

func TestUpdateIssueReturnsPartialSuccessWhenRefreshFails(t *testing.T) {
	var puts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "PUT /rest/api/2/issue/PROJ-1":
			puts++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			http.Error(w, "proxy unavailable", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateIssueFields(context.Background(), UpdateIssueInput{
		IssueIDOrKey: "PROJ-1",
		Fields:       map[string]any{"summary": "new"},
		ReturnIssue:  true,
	})
	if !out.Success || puts != 1 {
		t.Fatalf("out=%+v puts=%d", out, puts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["refreshError"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestTransitionByNameRejectsAmbiguousMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{
				{"id": "1", "name": "Done"},
				{"id": "2", "name": "Done"},
			}})
			return
		}
		t.Fatal("ambiguous name should not post transition")
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.TransitionIssue(context.Background(), TransitionIssueInput{IssueIDOrKey: "PROJ-1", TransitionName: "Done"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_TRANSITION_AMBIGUOUS" {
		t.Fatalf("out=%+v", out)
	}
}

func TestJiraToolDefinitionsHaveSecurityAnnotations(t *testing.T) {
	defs := Definitions()
	seen := map[string]bool{}
	for _, def := range defs {
		seen[def.Name] = true
		if def.Annotations == nil || def.Annotations.OpenWorldHint == nil || !*def.Annotations.OpenWorldHint {
			t.Fatalf("%s missing open-world annotation", def.Name)
		}
		if def.Name == "jira_get_issue" && !def.Annotations.ReadOnlyHint {
			t.Fatal("jira_get_issue must be read-only")
		}
		if def.Name == "jira_add_issue_comment" && (def.Annotations.ReadOnlyHint || def.Annotations.DestructiveHint == nil || *def.Annotations.DestructiveHint) {
			t.Fatal("comment tool should be additive write")
		}
	}
	for _, name := range []string{"jira_authenticate", "jira_get_issue", "jira_add_issue_comment", "jira_update_issue_fields", "jira_transition_issue"} {
		if !seen[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
}

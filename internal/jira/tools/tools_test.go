package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/jira/client"
	"github.com/chiendao1808/atlassian-mcp/internal/observability"
	"github.com/chiendao1808/atlassian-mcp/internal/result"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func strPtr(v string) *string { return &v }

type requestRoute struct {
	method string
	path   string
}

// newMCPJiraTestClient exposes Jira tools through the real SDK registration path for schema tests.
func newMCPJiraTestClient(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-jira-server", Version: "v0"}, nil)
	svc.Register(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-jira-client", Version: "v0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		if err := serverSession.Wait(); err != nil {
			t.Fatalf("server session wait: %v", err)
		}
	})
	return clientSession
}

// requireSchemaMap normalizes SDK schema values to maps for focused registration assertions.
func requireSchemaMap(t *testing.T, schema any) map[string]any {
	t.Helper()
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal schema %s: %v", string(data), err)
	}
	return out
}

func requireSchemaProperties(t *testing.T, schema map[string]any, names ...string) {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties=%#v", schema["properties"])
	}
	for _, name := range names {
		if _, ok := props[name]; !ok {
			t.Fatalf("schema missing property %q in %#v", name, props)
		}
	}
}

func forbidSchemaProperties(t *testing.T, schema map[string]any, names ...string) {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties=%#v", schema["properties"])
	}
	for _, name := range names {
		if _, ok := props[name]; ok {
			t.Fatalf("schema should not expose property %q in %#v", name, props)
		}
	}
}

func requireSchemaRequired(t *testing.T, schema map[string]any, want []string) {
	t.Helper()
	var got []string
	if raw, ok := schema["required"].([]any); ok {
		for _, item := range raw {
			got = append(got, item.(string))
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("required=%v, want %v", got, want)
	}
}

func callJiraMCPEnvelope(t *testing.T, client *mcp.ClientSession, name string, args map[string]any) result.Envelope {
	t.Helper()
	res, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s tool error: %#v", name, res.Content)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s marshal structured content: %v", name, err)
	}
	var out result.Envelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("%s unmarshal envelope %s: %v", name, string(data), err)
	}
	return out
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

func TestAuthenticateFallsBackToEnvironmentWhenInputOmitted(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
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

	svc := newTestServiceWithEnv(server.URL, server.Client(), map[string]string{
		"JIRA_USERNAME": "alice",
		"JIRA_PASSWORD": "secret",
	})
	out := svc.Authenticate(context.Background(), AuthenticateInput{})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if want := "Basic " + basicAuthValue("alice", "secret"); gotAuth != want {
		t.Fatalf("authorization=%q, want %q", gotAuth, want)
	}
}

func TestAuthenticateExplicitInputOverridesEnvironment(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "6.4.14"})
		case "/rest/api/2/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "bob"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestServiceWithEnv(server.URL, server.Client(), map[string]string{
		"JIRA_USERNAME": "alice",
		"JIRA_PASSWORD": "env-secret",
	})
	out := svc.Authenticate(context.Background(), AuthenticateInput{Username: "bob", Password: "input-secret"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if want := "Basic " + basicAuthValue("bob", "input-secret"); gotAuth != want {
		t.Fatalf("authorization=%q, want %q", gotAuth, want)
	}
}

func TestAuthenticateWithoutInputOrEnvironmentFailsValidation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	out := svc.Authenticate(context.Background(), AuthenticateInput{})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("missing credentials sent %d requests", calls)
	}
}

func TestAuthenticateResultOmitsSensitiveUpstreamData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version":       "6.4.14",
				"password":      "server-secret",
				"authorization": "Basic server-secret",
			})
		case "/rest/api/2/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":       "alice",
				"authHeader": "Basic user-secret",
				"password":   "user-secret",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).Authenticate(context.Background(), AuthenticateInput{Username: "alice", Password: "client-secret"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(raw))
	for _, secret := range []string{"client-secret", "server-secret", "user-secret", "basic "} {
		if strings.Contains(text, secret) {
			t.Fatalf("auth result leaked %q: %s", secret, text)
		}
	}
}

func TestGetIssueReturnsIssueUnderDataIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/issue/PROJ-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.GetIssue(context.Background(), GetIssueInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["issue"] == nil {
		t.Fatalf("data should contain issue: %#v", out.Data)
	}
}

func TestAddIssueCommentRejectsBlankVisibilityValueWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueComment(context.Background(), AddCommentInput{
		IssueIDOrKey: "PROJ-1",
		Body:         "looks good",
		Visibility:   &Visibility{Type: "role", Value: " "},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid visibility sent %d requests", calls)
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

func TestTransitionRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.TransitionIssue(context.Background(), TransitionIssueInput{
		IssueIDOrKey: "PROJ-1",
		TransitionID: "31",
		ReturnFields: []string{"summary"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
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

func TestTransitionByNamePreservesWhitespaceForExactMatch(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{
				{"id": "31", "name": "Done"},
			}})
		case http.MethodPost:
			posts++
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.TransitionIssue(context.Background(), TransitionIssueInput{IssueIDOrKey: "PROJ-1", TransitionName: " Done "})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_TRANSITION_NOT_FOUND" {
		t.Fatalf("out=%+v", out)
	}
	if posts != 0 {
		t.Fatalf("whitespace-padded transition name posted %d transition requests", posts)
	}
}

func TestQueryAddOmitsBlankValueAndAccumulatesChainedCalls(t *testing.T) {
	var q query
	q = q.add("jql", "project = X").add("orderBy", "  ").add("expand", "changelog")
	if len(q) != 2 {
		t.Fatalf("query=%+v", q)
	}
	if got := q["jql"]; len(got) != 1 || got[0] != "project = X" {
		t.Fatalf("jql=%+v", got)
	}
	if got := q["expand"]; len(got) != 1 || got[0] != "changelog" {
		t.Fatalf("expand=%+v", got)
	}
	if _, ok := q["orderBy"]; ok {
		t.Fatalf("blank value should omit the key: %+v", q)
	}
}

func TestQueryBoolOmitsNilPointerAndAddsCorrectStringForm(t *testing.T) {
	var q query
	q = q.bool("validateQuery", nil)
	if len(q) != 0 {
		t.Fatalf("nil bool pointer should omit the key: %+v", q)
	}
	yes := true
	q = q.bool("validateQuery", &yes)
	if got := q["validateQuery"]; len(got) != 1 || got[0] != "true" {
		t.Fatalf("validateQuery=%+v", got)
	}
	no := false
	q = q.bool("deleteSubtasks", &no)
	if got := q["deleteSubtasks"]; len(got) != 1 || got[0] != "false" {
		t.Fatalf("deleteSubtasks=%+v", got)
	}
}

func TestQueryIntOmitsNilPointerAndChainsAccumulate(t *testing.T) {
	var q query
	q = q.int("startAt", nil)
	if len(q) != 0 {
		t.Fatalf("nil int pointer should omit the key: %+v", q)
	}
	startAt, maxResults := 10, 50
	q = q.int("startAt", &startAt).int("maxResults", &maxResults)
	if got := q["startAt"]; len(got) != 1 || got[0] != "10" {
		t.Fatalf("startAt=%+v", got)
	}
	if got := q["maxResults"]; len(got) != 1 || got[0] != "50" {
		t.Fatalf("maxResults=%+v", got)
	}
}

func TestCreateIssuePreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).CreateIssue(context.Background(), CreateIssueInput{
		Fields: map[string]any{"summary": "new issue"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestCreateIssueRejectsEmptyFieldsWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateIssue(context.Background(), CreateIssueInput{})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("empty fields sent %d requests", calls)
	}
}

func TestCreateIssuePostsFieldsAndUpdateAndReturnsCreatedObject(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-2", "self": "http://jira/issue/10001"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateIssue(context.Background(), CreateIssueInput{
		Fields: map[string]any{"summary": "new issue", "project": map[string]any{"key": "PROJ"}},
		Update: map[string]any{"labels": []map[string]any{{"add": "triage"}}},
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody["fields"] == nil || gotBody["update"] == nil {
		t.Fatalf("request body missing fields/update: %+v", gotBody)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["key"] != "PROJ-2" {
		t.Fatalf("data should carry the created issue: %#v", out.Data)
	}
}

func TestBulkCreateIssuesPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).BulkCreateIssues(context.Background(), BulkCreateIssuesInput{
		IssueUpdates: []map[string]any{{"fields": map[string]any{"summary": "a"}}},
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestBulkCreateIssuesRejectsEmptyListWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.BulkCreateIssues(context.Background(), BulkCreateIssuesInput{})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("empty list sent %d requests", calls)
	}
}

// TestBulkCreateIssuesReportsSuccessWithPartialFailureErrors directly tests decision D-I: a
// non-empty upstream "errors" array in a 2xx bulk-create response must NOT flip the tool to
// Success:false. Callers are expected to inspect data.errors themselves.
func TestBulkCreateIssuesReportsSuccessWithPartialFailureErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/bulk" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issues": []map[string]any{{"id": "1", "key": "PROJ-1"}},
			"errors": []map[string]any{{"failedElementNumber": 1, "elementErrors": map[string]any{"errorMessages": []string{"bad summary"}}}},
		})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.BulkCreateIssues(context.Background(), BulkCreateIssuesInput{
		IssueUpdates: []map[string]any{
			{"fields": map[string]any{"summary": "ok"}},
			{"fields": map[string]any{"summary": ""}},
		},
	})
	if !out.Success {
		t.Fatalf("D-I: partial failure must still report success, out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", out.Data)
	}
	errs, ok := data["errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("D-I: non-empty upstream errors must be surfaced in data.errors: %#v", data)
	}
}

func TestDeleteIssuePreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).DeleteIssue(context.Background(), DeleteIssueInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestDeleteIssueRejectsPathSeparatorWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssue(context.Background(), DeleteIssueInput{IssueIDOrKey: "PROJ/1"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid issue key sent %d requests", calls)
	}
}

func TestDeleteIssueOmitsQueryParamWhenSubtasksNotRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/issue/PROJ-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Has("deleteSubtasks") {
			t.Fatalf("deleteSubtasks should be omitted when not requested: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssue(context.Background(), DeleteIssueInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true {
		t.Fatalf("data=%+v", data)
	}
}

func TestDeleteIssueSendsQueryParamWhenSubtasksRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("deleteSubtasks") != "true" {
			t.Fatalf("expected deleteSubtasks=true, got %s", r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssue(context.Background(), DeleteIssueInput{IssueIDOrKey: "PROJ-1", DeleteSubtasks: true})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
}

func TestAssignIssuePreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).AssignIssue(context.Background(), AssignIssueInput{IssueIDOrKey: "PROJ-1", Name: "alice"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestAssignIssueRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AssignIssue(context.Background(), AssignIssueInput{
		IssueIDOrKey: "PROJ-1",
		ReturnFields: []string{"summary"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestAssignIssueSendsEmptyStringBodyWhenNameOmitted directly tests decisions D-A/D-H: the
// unassign body must be the literal {"name":""}, never a null body.
func TestAssignIssueSendsEmptyStringBodyWhenNameOmitted(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/issue/PROJ-1/assignee" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = strings.TrimSpace(string(b))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AssignIssue(context.Background(), AssignIssueInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody != `{"name":""}` {
		t.Fatalf("D-H: unassign body must be the literal empty string, got %q", gotBody)
	}
}

func TestAssignIssueSendsNameBodyWhenNameSet(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = strings.TrimSpace(string(b))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AssignIssue(context.Background(), AssignIssueInput{IssueIDOrKey: "PROJ-1", Name: "alice"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody != `{"name":"alice"}` {
		t.Fatalf("gotBody=%q", gotBody)
	}
}

func TestAssignIssueReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var puts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "PUT /rest/api/2/issue/PROJ-1/assignee":
			puts++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AssignIssue(context.Background(), AssignIssueInput{IssueIDOrKey: "PROJ-1", Name: "alice", ReturnIssue: true})
	if !out.Success || puts != 1 {
		t.Fatalf("out=%+v puts=%d", out, puts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestSearchIssuesPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).SearchIssues(context.Background(), SearchIssuesInput{JQL: "project = PROJ"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestSearchIssuesRejectsBlankJQLWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.SearchIssues(context.Background(), SearchIssuesInput{JQL: "   "})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank jql sent %d requests", calls)
	}
}

func TestSearchIssuesBodyContainsJQLAndOmitsUnsetOptionalFields(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/search" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"startAt": 0, "maxResults": 50, "total": 1, "issues": []map[string]any{{"id": "1", "key": "PROJ-1"}}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.SearchIssues(context.Background(), SearchIssuesInput{JQL: "project = PROJ"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody["jql"] != "project = PROJ" {
		t.Fatalf("gotBody=%+v", gotBody)
	}
	for _, unset := range []string{"startAt", "maxResults", "fields", "expand", "validateQuery"} {
		if _, present := gotBody[unset]; present {
			t.Fatalf("unset optional field %q should be omitted from the request body: %+v", unset, gotBody)
		}
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["total"] != float64(1) {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestSearchIssuesBodyIncludesSetOptionalFields(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"startAt": 10, "maxResults": 5, "total": 0, "issues": []map[string]any{}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	startAt, maxResults := 10, 5
	validateQuery := true
	out := svc.SearchIssues(context.Background(), SearchIssuesInput{
		JQL:           "project = PROJ",
		StartAt:       &startAt,
		MaxResults:    &maxResults,
		Fields:        []string{"summary", "status"},
		Expand:        []string{"changelog"},
		ValidateQuery: &validateQuery,
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody["startAt"] != float64(10) || gotBody["maxResults"] != float64(5) || gotBody["validateQuery"] != true {
		t.Fatalf("gotBody=%+v", gotBody)
	}
	fields, ok := gotBody["fields"].([]any)
	if !ok || len(fields) != 2 {
		t.Fatalf("fields=%+v", gotBody["fields"])
	}
}

func TestListIssueCommentsPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).ListIssueComments(context.Background(), ListIssueCommentsInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestListIssueCommentsRejectsPathSeparatorWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueComments(context.Background(), ListIssueCommentsInput{IssueIDOrKey: "PROJ/1"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid issue key sent %d requests", calls)
	}
}

// TestListIssueCommentsOmitsOrderByWhenUnset directly guards the plan's risk note: orderBy support on
// Jira 6.4.14 is patch-dependent, so it must never be defaulted or guessed -- only sent when the
// caller actually set it. This test also asserts startAt/maxResults/expand are sent when set.
func TestListIssueCommentsOmitsOrderByWhenUnset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/issue/PROJ-1/comment" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Has("orderBy") {
			t.Fatalf("orderBy should be omitted when not requested: %s", r.URL.String())
		}
		if got := r.URL.Query().Get("startAt"); got != "5" {
			t.Fatalf("startAt=%q", got)
		}
		if got := r.URL.Query().Get("maxResults"); got != "10" {
			t.Fatalf("maxResults=%q", got)
		}
		if got := r.URL.Query().Get("expand"); got != "renderedBody" {
			t.Fatalf("expand=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"startAt": 5, "maxResults": 10, "total": 0, "comments": []map[string]any{}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	startAt, maxResults := 5, 10
	out := svc.ListIssueComments(context.Background(), ListIssueCommentsInput{
		IssueIDOrKey: "PROJ-1",
		StartAt:      &startAt,
		MaxResults:   &maxResults,
		Expand:       []string{"renderedBody"},
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["total"] != float64(0) {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestListIssueCommentsSendsOrderByWhenSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("orderBy"); got != "-created" {
			t.Fatalf("orderBy=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"startAt": 0, "maxResults": 50, "total": 0, "comments": []map[string]any{}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueComments(context.Background(), ListIssueCommentsInput{IssueIDOrKey: "PROJ-1", OrderBy: "-created"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
}

func TestUpdateIssueCommentPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).UpdateIssueComment(context.Background(), UpdateIssueCommentInput{
		IssueIDOrKey: "PROJ-1", CommentID: "10010", Body: "updated",
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestUpdateIssueCommentRejectsBlankCommentIDWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateIssueComment(context.Background(), UpdateIssueCommentInput{IssueIDOrKey: "PROJ-1", CommentID: "  ", Body: "updated"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank commentId sent %d requests", calls)
	}
}

func TestUpdateIssueCommentRejectsBlankBodyWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateIssueComment(context.Background(), UpdateIssueCommentInput{IssueIDOrKey: "PROJ-1", CommentID: "10010", Body: "   "})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank body sent %d requests", calls)
	}
}

// TestUpdateIssueCommentRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork mirrors
// TestAssignIssueRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork exactly: the same guard is
// required on every † tool per the plan.
func TestUpdateIssueCommentRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateIssueComment(context.Background(), UpdateIssueCommentInput{
		IssueIDOrKey: "PROJ-1",
		CommentID:    "10010",
		Body:         "updated",
		ReturnFields: []string{"summary"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

func TestUpdateIssueCommentSendsBodyAndVisibilityToExactPath(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/issue/PROJ-1/comment/10010" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10010", "body": "updated"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateIssueComment(context.Background(), UpdateIssueCommentInput{
		IssueIDOrKey: "PROJ-1",
		CommentID:    "10010",
		Body:         "updated",
		Visibility:   &Visibility{Type: "role", Value: "Administrators"},
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody["body"] != "updated" {
		t.Fatalf("gotBody=%+v", gotBody)
	}
	visibility, ok := gotBody["visibility"].(map[string]any)
	if !ok || visibility["type"] != "role" || visibility["value"] != "Administrators" {
		t.Fatalf("visibility=%+v", gotBody["visibility"])
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["comment"] == nil {
		t.Fatalf("data should carry the updated comment: %#v", out.Data)
	}
}

// TestUpdateIssueCommentReadsBackIssueWhenReturnIssueTrue mirrors
// TestAssignIssueReadsBackIssueWhenReturnIssueTrue exactly, plus asserts the updated comment is still
// present in data alongside the refreshed issue.
func TestUpdateIssueCommentReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var puts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "PUT /rest/api/2/issue/PROJ-1/comment/10010":
			puts++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10010", "body": "updated"})
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateIssueComment(context.Background(), UpdateIssueCommentInput{
		IssueIDOrKey: "PROJ-1",
		CommentID:    "10010",
		Body:         "updated",
		ReturnIssue:  true,
	})
	if !out.Success || puts != 1 {
		t.Fatalf("out=%+v puts=%d", out, puts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil || data["comment"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestDeleteIssueCommentPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).DeleteIssueComment(context.Background(), DeleteIssueCommentInput{
		IssueIDOrKey: "PROJ-1", CommentID: "10010",
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestDeleteIssueCommentRejectsBlankCommentIDWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssueComment(context.Background(), DeleteIssueCommentInput{IssueIDOrKey: "PROJ-1", CommentID: " "})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank commentId sent %d requests", calls)
	}
}

func TestDeleteIssueCommentRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssueComment(context.Background(), DeleteIssueCommentInput{
		IssueIDOrKey: "PROJ-1",
		CommentID:    "10010",
		ReturnExpand: []string{"renderedFields"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

func TestDeleteIssueCommentSendsDeleteToExactPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/issue/PROJ-1/comment/10010" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssueComment(context.Background(), DeleteIssueCommentInput{IssueIDOrKey: "PROJ-1", CommentID: "10010"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true {
		t.Fatalf("data=%+v", data)
	}
}

// TestDeleteIssueCommentReadsBackIssueWhenReturnIssueTrue mirrors
// TestAssignIssueReadsBackIssueWhenReturnIssueTrue exactly.
func TestDeleteIssueCommentReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "DELETE /rest/api/2/issue/PROJ-1/comment/10010":
			deletes++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssueComment(context.Background(), DeleteIssueCommentInput{
		IssueIDOrKey: "PROJ-1",
		CommentID:    "10010",
		ReturnIssue:  true,
	})
	if !out.Success || deletes != 1 {
		t.Fatalf("out=%+v deletes=%d", out, deletes)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestListIssueTransitionsPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).ListIssueTransitions(context.Background(), ListIssueTransitionsInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestListIssueTransitionsRejectsPathSeparatorWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueTransitions(context.Background(), ListIssueTransitionsInput{IssueIDOrKey: "PROJ/1"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid issue key sent %d requests", calls)
	}
}

// TestListIssueTransitionsDefaultsExpandWhenOmitted asserts the spec-recommended default
// expand=transitions.fields is sent when the caller does not supply Expand.
func TestListIssueTransitionsDefaultsExpandWhenOmitted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/issue/PROJ-1/transitions" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("expand"); got != "transitions.fields" {
			t.Fatalf("expand=%q, want default transitions.fields", got)
		}
		if r.URL.Query().Has("transitionId") {
			t.Fatalf("transitionId should be omitted when not requested: %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{{"id": "31", "name": "Done"}}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueTransitions(context.Background(), ListIssueTransitionsInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["transitions"] == nil {
		t.Fatalf("data=%#v", out.Data)
	}
}

// TestListIssueTransitionsUsesCallerExpandInsteadOfDefault asserts a caller-supplied Expand replaces
// the transitions.fields default outright rather than being merged/appended alongside it.
func TestListIssueTransitionsUsesCallerExpandInsteadOfDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("expand"); got != "changelog" {
			t.Fatalf("expand=%q, want caller-supplied changelog (not merged with the default)", got)
		}
		if got := r.URL.Query().Get("transitionId"); got != "31" {
			t.Fatalf("transitionId=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{{"id": "31", "name": "Done"}}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueTransitions(context.Background(), ListIssueTransitionsInput{
		IssueIDOrKey: "PROJ-1",
		TransitionID: "31",
		Expand:       []string{"changelog"},
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
}

func TestAddIssueAttachmentPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).AddIssueAttachment(context.Background(), AddIssueAttachmentInput{
		IssueIDOrKey:  "PROJ-1",
		Filename:      "note.txt",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestAddIssueAttachmentRejectsBlankFilenameWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueAttachment(context.Background(), AddIssueAttachmentInput{
		IssueIDOrKey:  "PROJ-1",
		Filename:      "   ",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank filename sent %d requests", calls)
	}
}

func TestAddIssueAttachmentRejectsInvalidBase64WithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueAttachment(context.Background(), AddIssueAttachmentInput{
		IssueIDOrKey:  "PROJ-1",
		Filename:      "note.txt",
		ContentBase64: "not-valid-base64!!!",
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid base64 sent %d requests", calls)
	}
}

func TestAddIssueAttachmentRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueAttachment(context.Background(), AddIssueAttachmentInput{
		IssueIDOrKey:  "PROJ-1",
		Filename:      "note.txt",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello")),
		ReturnFields:  []string{"summary"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestAddIssueAttachmentUploadsMultipartWithMandatoryHeaderAndUnwrapsArrayResponse asserts every
// upload-specific regression risk called out in the plan: the request is multipart/form-data, the
// file field name is "file", the X-Atlassian-Token: nocheck XSRF-bypass header is present (Jira
// 6.4.x rejects uploads without it), and Jira's 200 JSON ARRAY response body unwraps correctly under
// data.attachments.
func TestAddIssueAttachmentUploadsMultipartWithMandatoryHeaderAndUnwrapsArrayResponse(t *testing.T) {
	var gotContentType, gotToken, gotFilename, gotContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/PROJ-1/attachments" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		gotContentType = r.Header.Get("Content-Type")
		gotToken = r.Header.Get("X-Atlassian-Token")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		gotFilename = header.Filename
		content, _ := io.ReadAll(file)
		gotContent = string(content)
		// Jira returns HTTP 200 (not 201) with a JSON ARRAY body -- not an object.
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "10100", "filename": "note.txt"}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueAttachment(context.Background(), AddIssueAttachmentInput{
		IssueIDOrKey:  "PROJ-1",
		Filename:      "note.txt",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello world")),
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Fatalf("Content-Type=%q, want multipart/form-data prefix", gotContentType)
	}
	if gotToken != "nocheck" {
		t.Fatalf("X-Atlassian-Token=%q, want nocheck", gotToken)
	}
	if gotFilename != "note.txt" {
		t.Fatalf("uploaded filename=%q", gotFilename)
	}
	if gotContent != "hello world" {
		t.Fatalf("uploaded content=%q", gotContent)
	}
	data, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", out.Data)
	}
	attachments, ok := data["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("data.attachments should unwrap the JSON array response: %#v", data)
	}
	first, ok := attachments[0].(map[string]any)
	if !ok || first["id"] != "10100" {
		t.Fatalf("attachments[0]=%#v", attachments[0])
	}
}

// TestAddIssueAttachmentReadsBackIssueWhenReturnIssueTrue mirrors
// TestAssignIssueReadsBackIssueWhenReturnIssueTrue: the attachments array must still be present
// alongside the refreshed issue.
func TestAddIssueAttachmentReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /rest/api/2/issue/PROJ-1/attachments":
			posts++
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "10100", "filename": "note.txt"}})
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueAttachment(context.Background(), AddIssueAttachmentInput{
		IssueIDOrKey:  "PROJ-1",
		Filename:      "note.txt",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello")),
		ReturnIssue:   true,
	})
	if !out.Success || posts != 1 {
		t.Fatalf("out=%+v posts=%d", out, posts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil || data["attachments"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestDeleteIssueAttachmentPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).DeleteIssueAttachment(context.Background(), DeleteIssueAttachmentInput{AttachmentID: "10100"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestDeleteIssueAttachmentRejectsBlankAttachmentIDWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssueAttachment(context.Background(), DeleteIssueAttachmentInput{AttachmentID: "  "})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank attachmentId sent %d requests", calls)
	}
}

// TestDeleteIssueAttachmentUsesAttachmentScopedPathWithoutIssueSegment is the specific regression
// guard this endpoint is prone to: the path root is /attachment/{id}, NOT /issue/.../attachment/{id}.
// This assertion would fail if someone accidentally routed the call through an issue-scoped path.
func TestDeleteIssueAttachmentUsesAttachmentScopedPathWithoutIssueSegment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/attachment/10100" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if strings.Contains(r.URL.Path, "issue/") {
			t.Fatalf("attachment delete path must not contain an issue/ segment: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.DeleteIssueAttachment(context.Background(), DeleteIssueAttachmentInput{AttachmentID: "10100"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true {
		t.Fatalf("data=%+v", data)
	}
}

func TestListIssueWorklogsPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).ListIssueWorklogs(context.Background(), ListIssueWorklogsInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestListIssueWorklogsRejectsPathSeparatorWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueWorklogs(context.Background(), ListIssueWorklogsInput{IssueIDOrKey: "PROJ/1"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid issue key sent %d requests", calls)
	}
}

func TestListIssueWorklogsReturnsWorklogsFromExactPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/issue/PROJ-1/worklog" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"startAt": 0, "maxResults": 20, "total": 1, "worklogs": []map[string]any{{"id": "1", "timeSpentSeconds": 3600}}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.ListIssueWorklogs(context.Background(), ListIssueWorklogsInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["worklogs"] == nil {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestAddIssueWorklogPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).AddIssueWorklog(context.Background(), AddIssueWorklogInput{
		IssueIDOrKey: "PROJ-1", TimeSpentSeconds: 3600,
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestAddIssueWorklogRejectsNonPositiveTimeSpentWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWorklog(context.Background(), AddIssueWorklogInput{IssueIDOrKey: "PROJ-1", TimeSpentSeconds: 0})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("non-positive timeSpentSeconds sent %d requests", calls)
	}
}

func TestAddIssueWorklogRejectsInvalidAdjustEstimateWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWorklog(context.Background(), AddIssueWorklogInput{
		IssueIDOrKey:     "PROJ-1",
		TimeSpentSeconds: 3600,
		AdjustEstimate:   "bogus",
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid adjustEstimate sent %d requests", calls)
	}
}

func TestAddIssueWorklogRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWorklog(context.Background(), AddIssueWorklogInput{
		IssueIDOrKey:     "PROJ-1",
		TimeSpentSeconds: 3600,
		ReturnExpand:     []string{"renderedFields"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestAddIssueWorklogSplitsQueryAndBodyFields is the specific regression guard called out in the
// dispatch: adjustEstimate/newEstimate/reduceBy must land on the query string, while
// comment/started/timeSpentSeconds must land in the JSON body -- never the reverse.
func TestAddIssueWorklogSplitsQueryAndBodyFields(t *testing.T) {
	var gotBody map[string]any
	var gotQuery map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/PROJ-1/worklog" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		gotQuery = map[string][]string(r.URL.Query())
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "20001", "timeSpentSeconds": 3600})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWorklog(context.Background(), AddIssueWorklogInput{
		IssueIDOrKey:     "PROJ-1",
		TimeSpentSeconds: 3600,
		Comment:          "worked on it",
		Started:          "2026-08-04T10:00:00.000+0000",
		AdjustEstimate:   "new",
		NewEstimate:      "2h",
		ReduceBy:         "1h",
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	// Query-side assertions: adjustEstimate/newEstimate/reduceBy must be on the query string.
	if got := gotQuery["adjustEstimate"]; len(got) != 1 || got[0] != "new" {
		t.Fatalf("query adjustEstimate=%v", gotQuery["adjustEstimate"])
	}
	if got := gotQuery["newEstimate"]; len(got) != 1 || got[0] != "2h" {
		t.Fatalf("query newEstimate=%v", gotQuery["newEstimate"])
	}
	if got := gotQuery["reduceBy"]; len(got) != 1 || got[0] != "1h" {
		t.Fatalf("query reduceBy=%v", gotQuery["reduceBy"])
	}
	// Body-side assertions: comment/started/timeSpentSeconds must be in the JSON body, and the query
	// params above must NOT also appear there.
	if gotBody["comment"] != "worked on it" {
		t.Fatalf("body comment=%v", gotBody["comment"])
	}
	if gotBody["started"] != "2026-08-04T10:00:00.000+0000" {
		t.Fatalf("body started=%v", gotBody["started"])
	}
	if gotBody["timeSpentSeconds"] != float64(3600) {
		t.Fatalf("body timeSpentSeconds=%v", gotBody["timeSpentSeconds"])
	}
	for _, key := range []string{"adjustEstimate", "newEstimate", "reduceBy"} {
		if _, present := gotBody[key]; present {
			t.Fatalf("body must not carry query-only field %q: %+v", key, gotBody)
		}
	}
	data, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", out.Data)
	}
	worklog, ok := data["worklog"].(map[string]any)
	if !ok || worklog["id"] != "20001" {
		t.Fatalf("data.worklog=%#v", data["worklog"])
	}
}

// TestAddIssueWorklogOmitsAdjustEstimateQueryWhenUnset asserts newEstimate/reduceBy are never sent
// when adjustEstimate itself is unset, matching the "sent only when adjustEstimate is set" rule.
func TestAddIssueWorklogOmitsAdjustEstimateQueryWhenUnset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("adjustEstimate") || r.URL.Query().Has("newEstimate") || r.URL.Query().Has("reduceBy") {
			t.Fatalf("estimate query params should be omitted when adjustEstimate is unset: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "20002", "timeSpentSeconds": 1800})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWorklog(context.Background(), AddIssueWorklogInput{IssueIDOrKey: "PROJ-1", TimeSpentSeconds: 1800})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
}

// TestAddIssueWorklogReadsBackIssueWhenReturnIssueTrue mirrors
// TestAssignIssueReadsBackIssueWhenReturnIssueTrue: the created worklog must still be present
// alongside the refreshed issue.
func TestAddIssueWorklogReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /rest/api/2/issue/PROJ-1/worklog":
			posts++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "20001", "timeSpentSeconds": 3600})
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWorklog(context.Background(), AddIssueWorklogInput{
		IssueIDOrKey:     "PROJ-1",
		TimeSpentSeconds: 3600,
		ReturnIssue:      true,
	})
	if !out.Success || posts != 1 {
		t.Fatalf("out=%+v posts=%d", out, posts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil || data["worklog"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestGetIssueWatchersPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).GetIssueWatchers(context.Background(), GetIssueWatchersInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestGetIssueWatchersRejectsPathSeparatorWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.GetIssueWatchers(context.Background(), GetIssueWatchersInput{IssueIDOrKey: "PROJ/1"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid issue key sent %d requests", calls)
	}
}

func TestGetIssueWatchersReturnsWatchersFromExactPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/issue/PROJ-1/watchers" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"isWatching": true, "watchCount": 1, "watchers": []map[string]any{{"name": "alice"}}})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.GetIssueWatchers(context.Background(), GetIssueWatchersInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["watchers"] == nil || data["watchCount"] != float64(1) {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestAddIssueWatcherPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).AddIssueWatcher(context.Background(), AddIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "bob"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestAddIssueWatcherRejectsBlankUsernameWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWatcher(context.Background(), AddIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "  "})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank username sent %d requests", calls)
	}
}

func TestAddIssueWatcherRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWatcher(context.Background(), AddIssueWatcherInput{
		IssueIDOrKey: "PROJ-1",
		Username:     "bob",
		ReturnFields: []string{"summary"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestAddIssueWatcherSendsBareJSONStringBody directly tests decision D-W: the wire body must be the
// literal JSON string "bob", never an object like {"username":"bob"}.
func TestAddIssueWatcherSendsBareJSONStringBody(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/PROJ-1/watchers" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = strings.TrimSpace(string(b))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWatcher(context.Background(), AddIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "bob"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody != `"bob"` {
		t.Fatalf("D-W: watcher-add body must be the bare JSON string, got %q", gotBody)
	}
}

// TestAddIssueWatcherReadsBackIssueWhenReturnIssueTrue mirrors
// TestAssignIssueReadsBackIssueWhenReturnIssueTrue exactly.
func TestAddIssueWatcherReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /rest/api/2/issue/PROJ-1/watchers":
			posts++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.AddIssueWatcher(context.Background(), AddIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "bob", ReturnIssue: true})
	if !out.Success || posts != 1 {
		t.Fatalf("out=%+v posts=%d", out, posts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestRemoveIssueWatcherPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).RemoveIssueWatcher(context.Background(), RemoveIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "bob"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestRemoveIssueWatcherRejectsBlankUsernameWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.RemoveIssueWatcher(context.Background(), RemoveIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "  "})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("blank username sent %d requests", calls)
	}
}

func TestRemoveIssueWatcherRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.RemoveIssueWatcher(context.Background(), RemoveIssueWatcherInput{
		IssueIDOrKey: "PROJ-1",
		Username:     "bob",
		ReturnExpand: []string{"renderedFields"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestRemoveIssueWatcherSendsUsernameOnQueryStringWithoutBody directly tests decision D-W: username
// travels on the query string for this endpoint, and there must be no request body at all.
func TestRemoveIssueWatcherSendsUsernameOnQueryStringWithoutBody(t *testing.T) {
	var gotQuery, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/issue/PROJ-1/watchers" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		gotQuery = r.URL.Query().Get("username")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.RemoveIssueWatcher(context.Background(), RemoveIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "bob"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotQuery != "bob" {
		t.Fatalf("D-W: username must be on the query string, got query=%q", gotQuery)
	}
	if gotBody != "" {
		t.Fatalf("D-W: watcher-remove must send no request body, got %q", gotBody)
	}
}

// TestRemoveIssueWatcherReadsBackIssueWhenReturnIssueTrue mirrors
// TestAssignIssueReadsBackIssueWhenReturnIssueTrue exactly.
func TestRemoveIssueWatcherReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "DELETE /rest/api/2/issue/PROJ-1/watchers":
			deletes++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.RemoveIssueWatcher(context.Background(), RemoveIssueWatcherInput{IssueIDOrKey: "PROJ-1", Username: "bob", ReturnIssue: true})
	if !out.Success || deletes != 1 {
		t.Fatalf("out=%+v deletes=%d", out, deletes)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestVoteIssuePreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).VoteIssue(context.Background(), VoteIssueInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestVoteIssueRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.VoteIssue(context.Background(), VoteIssueInput{IssueIDOrKey: "PROJ-1", ReturnFields: []string{"summary"}})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestVoteIssueSendsNoBodyToVotesPath asserts the POST /votes call carries no request body at all.
func TestVoteIssueSendsNoBodyToVotesPath(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/PROJ-1/votes" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.VoteIssue(context.Background(), VoteIssueInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	if gotBody != "" {
		t.Fatalf("vote must send no request body, got %q", gotBody)
	}
}

// TestVoteIssueReadsBackIssueWhenReturnIssueTrue mirrors TestAssignIssueReadsBackIssueWhenReturnIssueTrue exactly.
func TestVoteIssueReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /rest/api/2/issue/PROJ-1/votes":
			posts++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.VoteIssue(context.Background(), VoteIssueInput{IssueIDOrKey: "PROJ-1", ReturnIssue: true})
	if !out.Success || posts != 1 {
		t.Fatalf("out=%+v posts=%d", out, posts)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestUnvoteIssuePreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).UnvoteIssue(context.Background(), UnvoteIssueInput{IssueIDOrKey: "PROJ-1"})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestUnvoteIssueRejectsRefreshOptionsWhenReturnIssueFalseWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UnvoteIssue(context.Background(), UnvoteIssueInput{IssueIDOrKey: "PROJ-1", ReturnExpand: []string{"renderedFields"}})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("invalid refresh options sent %d requests", calls)
	}
}

// TestUnvoteIssueSendsDeleteWithNoQueryToVotesPath asserts the DELETE /votes call carries no query
// parameters at all (unlike watcher removal, which requires one).
func TestUnvoteIssueSendsDeleteWithNoQueryToVotesPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/issue/PROJ-1/votes" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if len(r.URL.Query()) != 0 {
			t.Fatalf("unvote should carry no query parameters: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UnvoteIssue(context.Background(), UnvoteIssueInput{IssueIDOrKey: "PROJ-1"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
}

// TestUnvoteIssueReadsBackIssueWhenReturnIssueTrue mirrors TestAssignIssueReadsBackIssueWhenReturnIssueTrue exactly.
func TestUnvoteIssueReadsBackIssueWhenReturnIssueTrue(t *testing.T) {
	var deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "DELETE /rest/api/2/issue/PROJ-1/votes":
			deletes++
			w.WriteHeader(http.StatusNoContent)
		case "GET /rest/api/2/issue/PROJ-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "10001", "key": "PROJ-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UnvoteIssue(context.Background(), UnvoteIssueInput{IssueIDOrKey: "PROJ-1", ReturnIssue: true})
	if !out.Success || deletes != 1 {
		t.Fatalf("out=%+v deletes=%d", out, deletes)
	}
	data := out.Data.(map[string]any)
	if data["mutationApplied"] != true || data["issue"] == nil {
		t.Fatalf("data=%+v", data)
	}
}

func TestCreateIssueLinkPreAuthSendsNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	out := newTestService(server.URL, server.Client()).CreateIssueLink(context.Background(), CreateIssueLinkInput{
		Type:         map[string]any{"name": "Blocks"},
		InwardIssue:  map[string]any{"key": "PROJ-123"},
		OutwardIssue: map[string]any{"key": "PROJ-124"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("pre-auth sent %d requests", calls)
	}
}

func TestCreateIssueLinkRejectsEmptyTypeWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateIssueLink(context.Background(), CreateIssueLinkInput{
		InwardIssue:  map[string]any{"key": "PROJ-123"},
		OutwardIssue: map[string]any{"key": "PROJ-124"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("empty type sent %d requests", calls)
	}
}

func TestCreateIssueLinkRejectsEmptyInwardIssueWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateIssueLink(context.Background(), CreateIssueLinkInput{
		Type:         map[string]any{"name": "Blocks"},
		OutwardIssue: map[string]any{"key": "PROJ-124"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("empty inwardIssue sent %d requests", calls)
	}
}

func TestCreateIssueLinkRejectsEmptyOutwardIssueWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateIssueLink(context.Background(), CreateIssueLinkInput{
		Type:        map[string]any{"name": "Blocks"},
		InwardIssue: map[string]any{"key": "PROJ-123"},
	})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("empty outwardIssue sent %d requests", calls)
	}
}

// TestCreateIssueLinkPostsToIssueLinkPathOutsideIssueTree is the specific regression guard this
// endpoint is prone to: the path is /issueLink at the API root, NOT nested under /issue/...
func TestCreateIssueLinkPostsToIssueLinkPathOutsideIssueTree(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issueLink" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if strings.Contains(r.URL.Path, "issue/") {
			t.Fatalf("issueLink path must not contain an issue/ segment: %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateIssueLink(context.Background(), CreateIssueLinkInput{
		Type:         map[string]any{"name": "Blocks"},
		InwardIssue:  map[string]any{"key": "PROJ-123"},
		OutwardIssue: map[string]any{"key": "PROJ-124"},
		Comment:      map[string]any{"body": "linking these"},
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	linkType, ok := gotBody["type"].(map[string]any)
	if !ok || linkType["name"] != "Blocks" {
		t.Fatalf("gotBody.type=%+v", gotBody["type"])
	}
	inward, ok := gotBody["inwardIssue"].(map[string]any)
	if !ok || inward["key"] != "PROJ-123" {
		t.Fatalf("gotBody.inwardIssue=%+v", gotBody["inwardIssue"])
	}
	outward, ok := gotBody["outwardIssue"].(map[string]any)
	if !ok || outward["key"] != "PROJ-124" {
		t.Fatalf("gotBody.outwardIssue=%+v", gotBody["outwardIssue"])
	}
	comment, ok := gotBody["comment"].(map[string]any)
	if !ok || comment["body"] != "linking these" {
		t.Fatalf("gotBody.comment=%+v", gotBody["comment"])
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["mutationApplied"] != true {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestComponentToolsPreAuthSendNoNetworkRequest(t *testing.T) {
	cases := []struct {
		name string
		call func(*Service) result.Envelope
	}{
		{name: "create", call: func(s *Service) result.Envelope {
			return s.CreateComponent(context.Background(), CreateComponentInput{ProjectKey: "PROJ", Name: "Backend"})
		}},
		{name: "get", call: func(s *Service) result.Envelope {
			return s.GetComponent(context.Background(), GetComponentInput{ComponentID: "10010"})
		}},
		{name: "update", call: func(s *Service) result.Envelope {
			return s.UpdateComponent(context.Background(), UpdateComponentInput{ComponentID: "10010", Name: strPtr("API")})
		}},
		{name: "delete", call: func(s *Service) result.Envelope {
			return s.DeleteComponent(context.Background(), DeleteComponentInput{ComponentID: "10010"})
		}},
		{name: "issue count", call: func(s *Service) result.Envelope {
			return s.GetComponentIssueCount(context.Background(), GetComponentIssueCountInput{ComponentID: "10010"})
		}},
		{name: "project list", call: func(s *Service) result.Envelope {
			return s.ListProjectComponents(context.Background(), ListProjectComponentsInput{ProjectIDOrKey: "PROJ"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls++
			}))
			t.Cleanup(server.Close)

			out := tc.call(newTestService(server.URL, server.Client()))
			if out.Success || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
				t.Fatalf("out=%+v", out)
			}
			if calls != 0 {
				t.Fatalf("pre-auth sent %d requests", calls)
			}
		})
	}
}

func TestCreateComponentPostsProjectKeyAsProjectAndReturnsRedactedComponent(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/component" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if len(r.URL.Query()) != 0 {
			t.Fatalf("create component should not send query parameters: %s", r.URL.String())
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       "10010",
			"name":     "Backend",
			"project":  "PROJ",
			"password": "component-secret",
		})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.CreateComponent(context.Background(), CreateComponentInput{
		ProjectKey:   " PROJ ",
		Name:         " Backend ",
		Description:  "Backend services",
		LeadUserName: "alice",
		AssigneeType: "UNASSIGNED",
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	wantBody := map[string]any{
		"project":      " PROJ ",
		"name":         " Backend ",
		"description":  "Backend services",
		"leadUserName": "alice",
		"assigneeType": "UNASSIGNED",
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Fatalf("body=%+v, want %+v", gotBody, wantBody)
	}
	if _, ok := gotBody["projectKey"]; ok {
		t.Fatalf("body must not send projectKey: %+v", gotBody)
	}
	if _, ok := gotBody["projectId"]; ok {
		t.Fatalf("body must not send projectId: %+v", gotBody)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["id"] != "10010" || data["name"] != "Backend" {
		t.Fatalf("data=%#v", out.Data)
	}
	raw, _ := json.Marshal(out)
	if strings.Contains(strings.ToLower(string(raw)), "component-secret") {
		t.Fatalf("component result leaked sentinel secret: %s", string(raw))
	}
}

func TestCreateComponentRejectsBlankRequiredFieldsWithoutNetwork(t *testing.T) {
	cases := []CreateComponentInput{
		{ProjectKey: " ", Name: "Backend"},
		{ProjectKey: "PROJ", Name: " "},
	}
	for _, input := range cases {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			calls++
		}))
		t.Cleanup(server.Close)

		svc := newTestService(server.URL, server.Client())
		svc.Store().Replace(auth.NewCredential("alice", "secret"))
		out := svc.CreateComponent(context.Background(), input)
		if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
			t.Fatalf("input=%+v out=%+v", input, out)
		}
		if calls != 0 {
			t.Fatalf("invalid create input sent %d requests", calls)
		}
	}
}

func TestGetComponentGetsExactPathAndReturnsComponent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/component/10010" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10010", "name": "Backend"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.GetComponent(context.Background(), GetComponentInput{ComponentID: "10010"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["id"] != "10010" || data["name"] != "Backend" {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestComponentPathValidationRejectsUnsafeSegmentsWithoutNetwork(t *testing.T) {
	cases := []struct {
		name string
		call func(*Service) result.Envelope
	}{
		{name: "get component", call: func(s *Service) result.Envelope {
			return s.GetComponent(context.Background(), GetComponentInput{ComponentID: "10/010"})
		}},
		{name: "update component", call: func(s *Service) result.Envelope {
			return s.UpdateComponent(context.Background(), UpdateComponentInput{ComponentID: "10?010", Name: strPtr("Backend")})
		}},
		{name: "delete component", call: func(s *Service) result.Envelope {
			return s.DeleteComponent(context.Background(), DeleteComponentInput{ComponentID: "10#010"})
		}},
		{name: "delete replacement", call: func(s *Service) result.Envelope {
			return s.DeleteComponent(context.Background(), DeleteComponentInput{ComponentID: "10010", MoveIssuesTo: strPtr("10/011")})
		}},
		{name: "issue count", call: func(s *Service) result.Envelope {
			return s.GetComponentIssueCount(context.Background(), GetComponentIssueCountInput{ComponentID: "10\\010"})
		}},
		{name: "project list", call: func(s *Service) result.Envelope {
			return s.ListProjectComponents(context.Background(), ListProjectComponentsInput{ProjectIDOrKey: "PROJ/DEV"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls++
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			out := tc.call(svc)
			if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
				t.Fatalf("out=%+v", out)
			}
			if calls != 0 {
				t.Fatalf("invalid path segment sent %d requests", calls)
			}
		})
	}
}

func TestUpdateComponentPreservesOmittedAndExplicitEmptyFields(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/component/10010" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10010", "description": "", "leadUserName": ""})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateComponent(context.Background(), UpdateComponentInput{
		ComponentID:  "10010",
		Description:  strPtr(""),
		LeadUserName: strPtr(""),
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	wantBody := map[string]any{"description": "", "leadUserName": ""}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Fatalf("body=%+v, want %+v", gotBody, wantBody)
	}
	if _, ok := gotBody["name"]; ok {
		t.Fatalf("omitted name should not be sent: %+v", gotBody)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["id"] != "10010" {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestUpdateComponentAcceptsEmptyAndJSONSuccessBodies(t *testing.T) {
	cases := []struct {
		name     string
		response func(http.ResponseWriter)
		wantData func(t *testing.T, data any)
	}{
		{
			name: "empty 200",
			response: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusOK)
			},
			wantData: func(t *testing.T, data any) {
				got, ok := data.(map[string]any)
				if !ok || got["mutationApplied"] != true {
					t.Fatalf("data=%#v", data)
				}
			},
		},
		{
			name: "json 200",
			response: func(w http.ResponseWriter) {
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "10010", "name": "API"})
			},
			wantData: func(t *testing.T, data any) {
				got, ok := data.(map[string]any)
				if !ok || got["id"] != "10010" || got["name"] != "API" {
					t.Fatalf("data=%#v", data)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			puts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/component/10010" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				}
				puts++
				tc.response(w)
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			out := svc.UpdateComponent(context.Background(), UpdateComponentInput{ComponentID: "10010", Name: strPtr("API")})
			if !out.Success || puts != 1 {
				t.Fatalf("out=%+v puts=%d", out, puts)
			}
			tc.wantData(t, out.Data)
		})
	}
}

func TestUpdateComponentRequiresAtLeastOneFieldWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateComponent(context.Background(), UpdateComponentInput{ComponentID: "10010"})
	if out.Success || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("out=%+v", out)
	}
	if calls != 0 {
		t.Fatalf("empty update sent %d requests", calls)
	}
}

func TestUpdateComponentRejectsMalformedJSONSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/component/10010" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{`))
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	out := svc.UpdateComponent(context.Background(), UpdateComponentInput{ComponentID: "10010", Name: strPtr("API")})
	if out.Success || out.Error == nil || out.Error.Code != "UPSTREAM_SERVER_ERROR" {
		t.Fatalf("out=%+v", out)
	}
}

func TestDeleteComponentSendsOptionalMoveIssuesToOnce(t *testing.T) {
	cases := []struct {
		name      string
		input     DeleteComponentInput
		wantQuery string
	}{
		{name: "no replacement", input: DeleteComponentInput{ComponentID: "10010"}},
		{name: "replacement", input: DeleteComponentInput{ComponentID: "10010", MoveIssuesTo: strPtr("10011")}, wantQuery: "10011"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deletes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/component/10010" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				}
				deletes++
				if got := r.URL.Query().Get("moveIssuesTo"); got != tc.wantQuery {
					t.Fatalf("moveIssuesTo=%q, want %q in %s", got, tc.wantQuery, r.URL.String())
				}
				if tc.wantQuery == "" && len(r.URL.Query()) != 0 {
					t.Fatalf("delete without replacement should not send query: %s", r.URL.String())
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			out := svc.DeleteComponent(context.Background(), tc.input)
			if !out.Success || deletes != 1 {
				t.Fatalf("out=%+v deletes=%d", out, deletes)
			}
			data, ok := out.Data.(map[string]any)
			if !ok || data["mutationApplied"] != true {
				t.Fatalf("data=%#v", out.Data)
			}
		})
	}
}

func TestComponentToolsPassThroughJiraPolicySuccessesAndErrors(t *testing.T) {
	tests := []struct {
		name      string
		call      func(*Service) result.Envelope
		wantRoute string
		status    int
		body      string
		wantCode  string
	}{
		{
			name: "duplicate create error",
			call: func(s *Service) result.Envelope {
				return s.CreateComponent(context.Background(), CreateComponentInput{ProjectKey: "PROJ", Name: "Duplicate"})
			},
			wantRoute: "POST /rest/api/2/component",
			status:    http.StatusBadRequest,
			body:      `{"errorMessages":["duplicate component name"],"errors":{"name":"already exists","password":"policy-secret"}}`,
			wantCode:  "VALIDATION_ERROR",
		},
		{
			name: "unassigned create success",
			call: func(s *Service) result.Envelope {
				return s.CreateComponent(context.Background(), CreateComponentInput{ProjectKey: "PROJ", Name: "Ops", AssigneeType: "UNASSIGNED"})
			},
			wantRoute: "POST /rest/api/2/component",
			status:    http.StatusCreated,
			body:      `{"id":"10012","name":"Ops","assigneeType":"UNASSIGNED"}`,
		},
		{
			name: "same replacement delete reaches Jira",
			call: func(s *Service) result.Envelope {
				return s.DeleteComponent(context.Background(), DeleteComponentInput{ComponentID: "10010", MoveIssuesTo: strPtr("10010")})
			},
			wantRoute: "DELETE /rest/api/2/component/10010",
			status:    http.StatusBadRequest,
			body:      `{"errorMessages":["cannot replace with same component"]}`,
			wantCode:  "VALIDATION_ERROR",
		},
		{
			name: "missing replacement delete reaches Jira",
			call: func(s *Service) result.Envelope {
				return s.DeleteComponent(context.Background(), DeleteComponentInput{ComponentID: "10010", MoveIssuesTo: strPtr("404")})
			},
			wantRoute: "DELETE /rest/api/2/component/10010",
			status:    http.StatusNotFound,
			body:      `{"errorMessages":["replacement component not found"]}`,
			wantCode:  "UPSTREAM_NOT_FOUND",
		},
		{
			name: "cross project replacement delete reaches Jira",
			call: func(s *Service) result.Envelope {
				return s.DeleteComponent(context.Background(), DeleteComponentInput{ComponentID: "10010", MoveIssuesTo: strPtr("20000")})
			},
			wantRoute: "DELETE /rest/api/2/component/10010",
			status:    http.StatusBadRequest,
			body:      `{"errorMessages":["replacement belongs to another project"]}`,
			wantCode:  "VALIDATION_ERROR",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if gotRoute := r.Method + " " + r.URL.Path; gotRoute != tt.wantRoute {
					t.Fatalf("route=%s, want %s", gotRoute, tt.wantRoute)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			out := tt.call(svc)
			if calls != 1 {
				t.Fatalf("policy case sent %d requests, want exactly one", calls)
			}
			if tt.wantCode == "" {
				if !out.Success {
					t.Fatalf("out=%+v", out)
				}
				return
			}
			if out.Success || out.Error == nil || out.Error.Code != tt.wantCode {
				t.Fatalf("out=%+v, want code %s", out, tt.wantCode)
			}
			raw, _ := json.Marshal(out)
			if strings.Contains(strings.ToLower(string(raw)), "policy-secret") {
				t.Fatalf("error envelope leaked sentinel secret: %s", string(raw))
			}
		})
	}
}

func TestGetComponentIssueCountAndListProjectComponentsReturnNativeJSON(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Service) result.Envelope
		want   string
		body   any
		assert func(t *testing.T, data any)
	}{
		{
			name: "zero issue count",
			call: func(s *Service) result.Envelope {
				return s.GetComponentIssueCount(context.Background(), GetComponentIssueCountInput{ComponentID: "10010"})
			},
			want: "GET /rest/api/2/component/10010/relatedIssueCounts",
			body: map[string]any{"issueCount": 0},
			assert: func(t *testing.T, data any) {
				got := data.(map[string]any)
				if got["issueCount"] != float64(0) {
					t.Fatalf("data=%#v", data)
				}
			},
		},
		{
			name: "nonzero issue count",
			call: func(s *Service) result.Envelope {
				return s.GetComponentIssueCount(context.Background(), GetComponentIssueCountInput{ComponentID: "10010"})
			},
			want: "GET /rest/api/2/component/10010/relatedIssueCounts",
			body: map[string]any{"issueCount": 3},
			assert: func(t *testing.T, data any) {
				got := data.(map[string]any)
				if got["issueCount"] != float64(3) {
					t.Fatalf("data=%#v", data)
				}
			},
		},
		{
			name: "empty project component list",
			call: func(s *Service) result.Envelope {
				return s.ListProjectComponents(context.Background(), ListProjectComponentsInput{ProjectIDOrKey: "PROJ"})
			},
			want: "GET /rest/api/2/project/PROJ/components",
			body: []any{},
			assert: func(t *testing.T, data any) {
				got := data.([]any)
				if len(got) != 0 {
					t.Fatalf("data=%#v", data)
				}
			},
		},
		{
			name: "multi project component list",
			call: func(s *Service) result.Envelope {
				return s.ListProjectComponents(context.Background(), ListProjectComponentsInput{ProjectIDOrKey: "PROJ"})
			},
			want: "GET /rest/api/2/project/PROJ/components",
			body: []map[string]any{{"id": "10010", "name": "Backend"}, {"id": "10011", "name": "Frontend"}},
			assert: func(t *testing.T, data any) {
				got := data.([]any)
				if len(got) != 2 || got[0].(map[string]any)["name"] != "Backend" || got[1].(map[string]any)["name"] != "Frontend" {
					t.Fatalf("data=%#v", data)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if gotRoute := r.Method + " " + r.URL.Path; gotRoute != tc.want {
					t.Fatalf("route=%s, want %s", gotRoute, tc.want)
				}
				_ = json.NewEncoder(w).Encode(tc.body)
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			out := tc.call(svc)
			if !out.Success {
				t.Fatalf("out=%+v", out)
			}
			tc.assert(t, out.Data)
		})
	}
}

func TestJiraComponentRegistrationSchemasAnnotationsAndBindings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/component/10010" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "10010", "name": "Backend"})
	}))
	t.Cleanup(server.Close)

	svc := newTestService(server.URL, server.Client())
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	client := newMCPJiraTestClient(t, svc)

	list, err := client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	registered := map[string]*mcp.Tool{}
	for _, tool := range list.Tools {
		registered[tool.Name] = tool
		if tool.OutputSchema == nil {
			t.Fatalf("%s missing output schema", tool.Name)
		}
	}
	schemas := map[string]struct {
		required []string
		props    []string
		forbid   []string
	}{
		"jira_create_component":          {required: []string{"projectKey", "name"}, props: []string{"projectKey", "name", "description", "leadUserName", "assigneeType"}, forbid: []string{"projectId", "projectIdOrKey"}},
		"jira_get_component":             {required: []string{"componentId"}, props: []string{"componentId"}},
		"jira_update_component":          {required: []string{"componentId"}, props: []string{"componentId", "name", "description", "leadUserName", "assigneeType"}},
		"jira_delete_component":          {required: []string{"componentId"}, props: []string{"componentId", "moveIssuesTo"}},
		"jira_get_component_issue_count": {required: []string{"componentId"}, props: []string{"componentId"}},
		"jira_list_project_components":   {required: []string{"projectIdOrKey"}, props: []string{"projectIdOrKey"}},
	}
	for name, want := range schemas {
		tool := registered[name]
		if tool == nil {
			t.Fatalf("missing registered tool %s", name)
		}
		schema := requireSchemaMap(t, tool.InputSchema)
		requireSchemaRequired(t, schema, want.required)
		requireSchemaProperties(t, schema, want.props...)
		forbidSchemaProperties(t, schema, want.forbid...)
	}
	annotations := map[string]struct {
		readOnly    bool
		destructive bool
		idempotent  bool
	}{
		"jira_create_component":          {readOnly: false, destructive: false, idempotent: false},
		"jira_get_component":             {readOnly: true, destructive: false, idempotent: true},
		"jira_update_component":          {readOnly: false, destructive: true, idempotent: true},
		"jira_delete_component":          {readOnly: false, destructive: true, idempotent: true},
		"jira_get_component_issue_count": {readOnly: true, destructive: false, idempotent: true},
		"jira_list_project_components":   {readOnly: true, destructive: false, idempotent: true},
	}
	for name, want := range annotations {
		got := registered[name].Annotations
		if got == nil || got.OpenWorldHint == nil || !*got.OpenWorldHint {
			t.Fatalf("%s missing open-world annotation", name)
		}
		destructive := true
		if got.DestructiveHint != nil {
			destructive = *got.DestructiveHint
		}
		if got.ReadOnlyHint != want.readOnly || destructive != want.destructive || got.IdempotentHint != want.idempotent {
			t.Fatalf("%s annotations=%+v", name, got)
		}
	}

	out := callJiraMCPEnvelope(t, client, "jira_get_component", map[string]any{"componentId": "10010"})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	data, ok := out.Data.(map[string]any)
	if !ok || data["id"] != "10010" {
		t.Fatalf("data=%#v", out.Data)
	}
}

func TestJiraComponentMCPDispatchCoversEveryRegistration(t *testing.T) {
	type requestRecord struct {
		method string
		path   string
		query  string
		body   string
	}
	cases := []struct {
		name      string
		args      map[string]any
		want      requestRecord
		status    int
		body      string
		assert    func(t *testing.T, out result.Envelope)
		wantCalls int
	}{
		{
			name:   "jira_create_component",
			args:   map[string]any{"projectKey": "PROJ", "name": "Backend", "description": "Services"},
			want:   requestRecord{method: http.MethodPost, path: "/rest/api/2/component", body: `{"description":"Services","name":"Backend","project":"PROJ"}`},
			status: http.StatusCreated,
			body:   `{"id":"10010","name":"Backend"}`,
			assert: func(t *testing.T, out result.Envelope) {
				data := out.Data.(map[string]any)
				if data["id"] != "10010" || data["name"] != "Backend" {
					t.Fatalf("data=%#v", out.Data)
				}
			},
			wantCalls: 1,
		},
		{
			name:   "jira_get_component",
			args:   map[string]any{"componentId": "10010"},
			want:   requestRecord{method: http.MethodGet, path: "/rest/api/2/component/10010"},
			status: http.StatusOK,
			body:   `{"id":"10010","name":"Backend"}`,
			assert: func(t *testing.T, out result.Envelope) {
				if out.Data.(map[string]any)["id"] != "10010" {
					t.Fatalf("data=%#v", out.Data)
				}
			},
			wantCalls: 1,
		},
		{
			name:   "jira_update_component",
			args:   map[string]any{"componentId": "10010", "description": ""},
			want:   requestRecord{method: http.MethodPut, path: "/rest/api/2/component/10010", body: `{"description":""}`},
			status: http.StatusOK,
			body:   `{"id":"10010","description":""}`,
			assert: func(t *testing.T, out result.Envelope) {
				if out.Data.(map[string]any)["id"] != "10010" {
					t.Fatalf("data=%#v", out.Data)
				}
			},
			wantCalls: 1,
		},
		{
			name:   "jira_delete_component",
			args:   map[string]any{"componentId": "10010", "moveIssuesTo": "10011"},
			want:   requestRecord{method: http.MethodDelete, path: "/rest/api/2/component/10010", query: "moveIssuesTo=10011"},
			status: http.StatusNoContent,
			assert: func(t *testing.T, out result.Envelope) {
				if out.Data.(map[string]any)["mutationApplied"] != true {
					t.Fatalf("data=%#v", out.Data)
				}
			},
			wantCalls: 1,
		},
		{
			name:   "jira_get_component_issue_count",
			args:   map[string]any{"componentId": "10010"},
			want:   requestRecord{method: http.MethodGet, path: "/rest/api/2/component/10010/relatedIssueCounts"},
			status: http.StatusOK,
			body:   `{"issueCount":7}`,
			assert: func(t *testing.T, out result.Envelope) {
				if out.Data.(map[string]any)["issueCount"] != float64(7) {
					t.Fatalf("data=%#v", out.Data)
				}
			},
			wantCalls: 1,
		},
		{
			name:   "jira_list_project_components",
			args:   map[string]any{"projectIdOrKey": "PROJ"},
			want:   requestRecord{method: http.MethodGet, path: "/rest/api/2/project/PROJ/components"},
			status: http.StatusOK,
			body:   `[{"id":"10010","name":"Backend"}]`,
			assert: func(t *testing.T, out result.Envelope) {
				if len(out.Data.([]any)) != 1 {
					t.Fatalf("data=%#v", out.Data)
				}
			},
			wantCalls: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got requestRecord
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				got = requestRecord{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(body)}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			client := newMCPJiraTestClient(t, svc)
			out := callJiraMCPEnvelope(t, client, tc.name, tc.args)
			if !out.Success || out.Tool != tc.name {
				t.Fatalf("out=%+v", out)
			}
			if got != tc.want {
				t.Fatalf("request=%+v, want %+v", got, tc.want)
			}
			if calls != tc.wantCalls {
				t.Fatalf("calls=%d, want %d", calls, tc.wantCalls)
			}
			tc.assert(t, out)
		})
	}
}

func TestJiraComponentMCPDispatchPreAuthAndRedactedErrors(t *testing.T) {
	preAuthCases := []struct {
		name string
		args map[string]any
	}{
		{name: "jira_create_component", args: map[string]any{"projectKey": "PROJ", "name": "Backend"}},
		{name: "jira_get_component", args: map[string]any{"componentId": "10010"}},
		{name: "jira_update_component", args: map[string]any{"componentId": "10010", "name": "Backend"}},
		{name: "jira_delete_component", args: map[string]any{"componentId": "10010"}},
		{name: "jira_get_component_issue_count", args: map[string]any{"componentId": "10010"}},
		{name: "jira_list_project_components", args: map[string]any{"projectIdOrKey": "PROJ"}},
	}
	for _, tc := range preAuthCases {
		t.Run("preauth "+tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
			t.Cleanup(server.Close)

			client := newMCPJiraTestClient(t, newTestService(server.URL, server.Client()))
			out := callJiraMCPEnvelope(t, client, tc.name, tc.args)
			if out.Success || out.Tool != tc.name || out.Error == nil || out.Error.Code != "JIRA_NOT_AUTHENTICATED" {
				t.Fatalf("out=%+v", out)
			}
			if calls != 0 {
				t.Fatalf("pre-auth %s sent %d requests", tc.name, calls)
			}
		})
	}

	errorCases := []struct {
		name string
		args map[string]any
		want requestRoute
	}{
		{name: "jira_create_component", args: map[string]any{"projectKey": "PROJ", "name": "Backend"}, want: requestRoute{method: http.MethodPost, path: "/rest/api/2/component"}},
		{name: "jira_get_component", args: map[string]any{"componentId": "10010"}, want: requestRoute{method: http.MethodGet, path: "/rest/api/2/component/10010"}},
		{name: "jira_update_component", args: map[string]any{"componentId": "10010", "name": "Backend"}, want: requestRoute{method: http.MethodPut, path: "/rest/api/2/component/10010"}},
		{name: "jira_get_component_issue_count", args: map[string]any{"componentId": "10010"}, want: requestRoute{method: http.MethodGet, path: "/rest/api/2/component/10010/relatedIssueCounts"}},
		{name: "jira_list_project_components", args: map[string]any{"projectIdOrKey": "PROJ"}, want: requestRoute{method: http.MethodGet, path: "/rest/api/2/project/PROJ/components"}},
	}
	for _, tc := range errorCases {
		t.Run("redacted error "+tc.name, func(t *testing.T) {
			var got requestRoute
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = requestRoute{method: r.Method, path: r.URL.Path}
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errorMessages":["bad component"],"errors":{"password":"mcp-secret"}}`))
			}))
			t.Cleanup(server.Close)

			svc := newTestService(server.URL, server.Client())
			svc.Store().Replace(auth.NewCredential("alice", "secret"))
			client := newMCPJiraTestClient(t, svc)
			out := callJiraMCPEnvelope(t, client, tc.name, tc.args)
			if out.Success || out.Tool != tc.name || out.Error == nil || out.Error.Code != "VALIDATION_ERROR" {
				t.Fatalf("out=%+v", out)
			}
			if got != tc.want {
				t.Fatalf("route=%+v, want %+v", got, tc.want)
			}
			raw, _ := json.Marshal(out)
			if strings.Contains(strings.ToLower(string(raw)), "mcp-secret") {
				t.Fatalf("error envelope leaked sentinel secret: %s", string(raw))
			}
		})
	}
}

func TestJiraToolDefinitionsHaveSecurityAnnotations(t *testing.T) {
	defs := Definitions()
	seen := map[string]bool{}
	for _, def := range defs {
		if seen[def.Name] {
			t.Fatalf("duplicate tool name %s", def.Name)
		}
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
		switch def.Name {
		case "jira_search_issues":
			if !def.Annotations.ReadOnlyHint {
				t.Fatal("jira_search_issues must be read-only")
			}
		case "jira_create_issue", "jira_bulk_create_issues":
			if def.Annotations.ReadOnlyHint || def.Annotations.DestructiveHint == nil || *def.Annotations.DestructiveHint {
				t.Fatalf("%s should be additive write", def.Name)
			}
		case "jira_delete_issue", "jira_assign_issue", "jira_update_issue_comment", "jira_delete_issue_comment", "jira_delete_issue_attachment",
			"jira_remove_issue_watcher", "jira_unvote_issue", "jira_update_component", "jira_delete_component":
			if def.Annotations.ReadOnlyHint || def.Annotations.DestructiveHint == nil || !*def.Annotations.DestructiveHint {
				t.Fatalf("%s should be destructive", def.Name)
			}
		case "jira_list_issue_comments", "jira_list_issue_transitions", "jira_list_issue_worklogs", "jira_get_issue_watchers",
			"jira_get_component", "jira_get_component_issue_count", "jira_list_project_components":
			if !def.Annotations.ReadOnlyHint {
				t.Fatalf("%s must be read-only", def.Name)
			}
		case "jira_add_issue_attachment", "jira_add_issue_worklog", "jira_add_issue_watcher", "jira_vote_issue", "jira_create_issue_link",
			"jira_create_component":
			if def.Annotations.ReadOnlyHint || def.Annotations.DestructiveHint == nil || *def.Annotations.DestructiveHint {
				t.Fatalf("%s should be additive write", def.Name)
			}
		}
	}
	// The full 30-tool roster: the existing 24 Jira tools plus exactly six Component tools.
	for _, name := range []string{
		"jira_authenticate", "jira_get_issue", "jira_add_issue_comment", "jira_update_issue_fields", "jira_transition_issue",
		"jira_create_issue", "jira_bulk_create_issues", "jira_delete_issue", "jira_assign_issue", "jira_search_issues",
		"jira_list_issue_comments", "jira_update_issue_comment", "jira_delete_issue_comment", "jira_list_issue_transitions",
		"jira_add_issue_attachment", "jira_delete_issue_attachment", "jira_list_issue_worklogs", "jira_add_issue_worklog",
		"jira_get_issue_watchers", "jira_add_issue_watcher", "jira_remove_issue_watcher", "jira_vote_issue", "jira_unvote_issue",
		"jira_create_issue_link", "jira_create_component", "jira_get_component", "jira_update_component", "jira_delete_component",
		"jira_get_component_issue_count", "jira_list_project_components",
	} {
		if !seen[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
	if len(defs) != 30 {
		t.Fatalf("expected exactly 30 tool definitions, got %d", len(defs))
	}
}

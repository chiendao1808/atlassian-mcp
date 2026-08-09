package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/auth"
	"github.com/chiendao1808/atlassian-mcp/internal/confluence/client"
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

func intPtr(v int) *int { return &v }

func authenticatedTestService(baseURL string, hc *http.Client) *Service {
	svc := newTestService(baseURL, hc)
	svc.Store().Replace(auth.NewCredential("alice", "secret"))
	return svc
}

// newMCPTestClient exposes a Service through the real SDK server/client path.
func newMCPTestClient(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-confluence-server", Version: "v0"}, nil)
	svc.Register(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-confluence-client", Version: "v0"}, nil)
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

func requireQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	query, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("query %q did not parse: %v", raw, err)
	}
	return query
}

// requireSchemaMap normalizes SDK schema values to maps for focused assertions.
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

// requireSchemaProperties checks only the boundary fields this package owns.
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

// requireSchemaRequired proves positional bindings expose the expected input type.
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

// callMCPEnvelope invokes a public MCP tool and decodes the shared result envelope.
func callMCPEnvelope(t *testing.T, client *mcp.ClientSession, name string, args map[string]any) result.Envelope {
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

func requireErrorCode(t *testing.T, out result.Envelope, code string) {
	t.Helper()
	if out.Success || out.Error == nil || out.Error.Code != code {
		t.Fatalf("out=%+v, want error code %s", out, code)
	}
}

// TestConfluenceRegisterExposesNineBoundMCPTools locks the MCP registration boundary against definition/handler swaps.
func TestConfluenceRegisterExposesNineBoundMCPTools(t *testing.T) {
	type requestRecord struct {
		method string
		path   string
		query  url.Values
	}
	var seen []requestRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, requestRecord{method: r.Method, path: r.URL.Path, query: r.URL.Query()})
		if r.URL.Path == "/rest/api/user/current" {
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "known", "username": "alice"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"sentinel": r.URL.Path})
	}))
	t.Cleanup(server.Close)

	client := newMCPTestClient(t, newTestService(server.URL, server.Client()))

	tools, err := client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	wantNames := []string{
		"confluence_authenticate",
		"confluence_search_content",
		"confluence_get_content",
		"confluence_list_content",
		"confluence_list_content_properties",
		"confluence_get_content_property",
		"confluence_list_spaces",
		"confluence_get_space",
		"confluence_list_space_content",
	}
	if len(tools.Tools) != len(wantNames) {
		t.Fatalf("registered tools=%d, want %d", len(tools.Tools), len(wantNames))
	}
	registered := map[string]*mcp.Tool{}
	for _, tool := range tools.Tools {
		registered[tool.Name] = tool
		if tool.OutputSchema == nil {
			t.Fatalf("%s missing output schema", tool.Name)
		}
		requireSchemaProperties(t, requireSchemaMap(t, tool.OutputSchema), "success", "service", "tool", "data", "error")
	}
	for _, name := range wantNames {
		if registered[name] == nil {
			t.Fatalf("registered tools missing %s", name)
		}
	}
	schemas := map[string]struct {
		required []string
		props    []string
	}{
		"confluence_authenticate":            {props: []string{"username", "password"}},
		"confluence_search_content":          {required: []string{"cql"}, props: []string{"cql", "cqlcontext", "expand", "start", "limit"}},
		"confluence_get_content":             {required: []string{"contentId"}, props: []string{"contentId", "status", "version", "expand"}},
		"confluence_list_content":            {props: []string{"type", "spaceKey", "title", "status", "postingDay", "expand", "start", "limit"}},
		"confluence_list_content_properties": {required: []string{"contentId"}, props: []string{"contentId", "expand", "start", "limit"}},
		"confluence_get_content_property":    {required: []string{"contentId", "key"}, props: []string{"contentId", "key", "expand"}},
		"confluence_list_spaces":             {props: []string{"spaceKey", "type", "status", "label", "expand", "start", "limit"}},
		"confluence_get_space":               {required: []string{"spaceKey"}, props: []string{"spaceKey", "expand"}},
		"confluence_list_space_content":      {required: []string{"spaceKey"}, props: []string{"spaceKey", "depth", "expand", "start", "limit"}},
	}
	for name, want := range schemas {
		schema := requireSchemaMap(t, registered[name].InputSchema)
		requireSchemaRequired(t, schema, want.required)
		requireSchemaProperties(t, schema, want.props...)
	}

	calls := []struct {
		name      string
		args      map[string]any
		wantPath  string
		wantQuery map[string]string
	}{
		{
			name:     "confluence_authenticate",
			args:     map[string]any{"username": "alice", "password": "secret"},
			wantPath: "/rest/api/user/current",
		},
		{
			name:     "confluence_search_content",
			args:     map[string]any{"cql": "space = ENG", "cqlcontext": "ctx", "expand": "space", "start": 1, "limit": 2},
			wantPath: "/rest/api/content/search",
			wantQuery: map[string]string{
				"cql": "space = ENG", "cqlcontext": "ctx", "expand": "space", "start": "1", "limit": "2",
			},
		},
		{
			name:     "confluence_get_content",
			args:     map[string]any{"contentId": "12345", "status": "current", "version": 3, "expand": "body.storage"},
			wantPath: "/rest/api/content/12345",
			wantQuery: map[string]string{
				"status": "current", "version": "3", "expand": "body.storage",
			},
		},
		{
			name:     "confluence_list_content",
			args:     map[string]any{"type": "blogpost", "spaceKey": "ENG", "title": "API Guide", "status": "current", "postingDay": "2026-08-09", "expand": "version", "start": 4, "limit": 5},
			wantPath: "/rest/api/content",
			wantQuery: map[string]string{
				"type": "blogpost", "spaceKey": "ENG", "title": "API Guide", "status": "current", "postingDay": "2026-08-09", "expand": "version", "start": "4", "limit": "5",
			},
		},
		{
			name:     "confluence_list_content_properties",
			args:     map[string]any{"contentId": "12345", "expand": "version", "start": 6, "limit": 7},
			wantPath: "/rest/api/content/12345/property",
			wantQuery: map[string]string{
				"expand": "version", "start": "6", "limit": "7",
			},
		},
		{
			name:     "confluence_get_content_property",
			args:     map[string]any{"contentId": "12345", "key": "build", "expand": "version"},
			wantPath: "/rest/api/content/12345/property/build",
			wantQuery: map[string]string{
				"expand": "version",
			},
		},
		{
			name:     "confluence_list_spaces",
			args:     map[string]any{"spaceKey": "ENG", "type": "global", "status": "current", "label": "team", "expand": "homepage", "start": 8, "limit": 9},
			wantPath: "/rest/api/space",
			wantQuery: map[string]string{
				"spaceKey": "ENG", "type": "global", "status": "current", "label": "team", "expand": "homepage", "start": "8", "limit": "9",
			},
		},
		{
			name:     "confluence_get_space",
			args:     map[string]any{"spaceKey": "ENG", "expand": "homepage"},
			wantPath: "/rest/api/space/ENG",
			wantQuery: map[string]string{
				"expand": "homepage",
			},
		},
		{
			name:     "confluence_list_space_content",
			args:     map[string]any{"spaceKey": "ENG", "depth": "root", "expand": "version", "start": 10, "limit": 11},
			wantPath: "/rest/api/space/ENG/content",
			wantQuery: map[string]string{
				"depth": "root", "expand": "version", "start": "10", "limit": "11",
			},
		},
	}
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			before := len(seen)
			out := callMCPEnvelope(t, client, call.name, call.args)
			if !out.Success || out.Tool != call.name {
				t.Fatalf("envelope=%+v, want success for %s", out, call.name)
			}
			if len(seen) != before+1 {
				t.Fatalf("%s sent %d requests, want 1", call.name, len(seen)-before)
			}
			got := seen[len(seen)-1]
			if got.method != http.MethodGet || got.path != call.wantPath {
				t.Fatalf("%s request=%s %s, want GET %s", call.name, got.method, got.path, call.wantPath)
			}
			for key, want := range call.wantQuery {
				if got.query.Get(key) != want {
					t.Fatalf("%s query[%s]=%q, want %q in %v", call.name, key, got.query.Get(key), want, got.query)
				}
			}
		})
	}
}

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
	want := []string{
		"confluence_authenticate",
		"confluence_search_content",
		"confluence_get_content",
		"confluence_list_content",
		"confluence_list_content_properties",
		"confluence_get_content_property",
		"confluence_list_spaces",
		"confluence_get_space",
		"confluence_list_space_content",
	}
	if len(defs) != len(want) {
		t.Fatalf("expected exactly %d tool definitions, got %d", len(want), len(defs))
	}
	for _, def := range defs {
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
	for i, name := range want {
		if defs[i].Name != name {
			t.Fatalf("definition %d=%q, want %q", i, defs[i].Name, name)
		}
	}
	seen := map[string]bool{}
	for _, def := range defs {
		seen[def.Name] = true
	}
	for _, name := range []string{
		"confluence_list_space_content_by_type",
		"confluence_create_content_property",
		"confluence_update_content_property",
		"confluence_delete_content_property",
		"confluence_list_space_properties",
		"confluence_request",
	} {
		if seen[name] {
			t.Fatalf("unexpected excluded tool %s", name)
		}
	}
}

func TestNewReadToolsPreAuthSendNoNetworkRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	tests := []struct {
		name string
		call func(*Service) result.Envelope
	}{
		{"list content", func(s *Service) result.Envelope { return s.ListContent(context.Background(), ListContentInput{}) }},
		{"list content properties", func(s *Service) result.Envelope {
			return s.ListContentProperties(context.Background(), ListContentPropertiesInput{ContentID: "12345"})
		}},
		{"get content property", func(s *Service) result.Envelope {
			return s.GetContentProperty(context.Background(), GetContentPropertyInput{ContentID: "12345", Key: "build"})
		}},
		{"list spaces", func(s *Service) result.Envelope { return s.ListSpaces(context.Background(), ListSpacesInput{}) }},
		{"get space", func(s *Service) result.Envelope {
			return s.GetSpace(context.Background(), GetSpaceInput{SpaceKey: "ENG"})
		}},
		{"list space content", func(s *Service) result.Envelope {
			return s.ListSpaceContent(context.Background(), ListSpaceContentInput{SpaceKey: "ENG"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorCode(t, tt.call(newTestService(server.URL, server.Client())), "CONFLUENCE_NOT_AUTHENTICATED")
			if calls != 0 {
				t.Fatalf("pre-auth sent %d requests", calls)
			}
		})
	}
}

func TestListContentSendsDocumentedQueryDefaultsAndPreservesOutput(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results":     []any{map[string]any{"id": "12345", "type": "page"}},
			"start":       0,
			"limit":       25,
			"size":        1,
			"_links":      map[string]any{"self": "x"},
			"_expandable": map[string]any{"space": ""},
			"sentinel":    "kept",
		})
	}))
	t.Cleanup(server.Close)

	svc := authenticatedTestService(server.URL, server.Client())
	out := svc.ListContent(context.Background(), ListContentInput{
		SpaceKey:   "ENG",
		Title:      "API Guide",
		Status:     "current",
		PostingDay: "2026-08-09",
		Expand:     "space,version",
	})
	if !out.Success {
		t.Fatalf("out=%+v", out)
	}
	second := svc.ListContent(context.Background(), ListContentInput{
		Type:  "blogpost",
		Start: intPtr(2),
		Limit: intPtr(7),
	})
	if !second.Success {
		t.Fatalf("second=%+v", second)
	}
	if len(seen) != 2 {
		t.Fatalf("seen=%v", seen)
	}
	firstQuery := strings.TrimPrefix(seen[0], "GET /rest/api/content?")
	query := requireQuery(t, firstQuery)
	for key, want := range map[string]string{
		"spaceKey":   "ENG",
		"title":      "API Guide",
		"status":     "current",
		"postingDay": "2026-08-09",
		"expand":     "space,version",
		"limit":      "25",
	} {
		if query.Get(key) != want {
			t.Fatalf("query[%s]=%q, want %q in %q", key, query.Get(key), want, seen[0])
		}
	}
	if query.Has("start") || query.Has("type") {
		t.Fatalf("query should omit start and type: %q", seen[0])
	}
	secondQuery := requireQuery(t, strings.TrimPrefix(seen[1], "GET /rest/api/content?"))
	for key, want := range map[string]string{"type": "blogpost", "start": "2", "limit": "7"} {
		if secondQuery.Get(key) != want {
			t.Fatalf("second query[%s]=%q, want %q in %q", key, secondQuery.Get(key), want, seen[1])
		}
	}
	data := out.Data.(map[string]any)
	if data["sentinel"] != "kept" || data["_links"] == nil || data["_expandable"] == nil || data["results"] == nil {
		t.Fatalf("upstream shape not preserved: %+v", data)
	}
}

func TestListContentValidatesInputsWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	t.Cleanup(server.Close)

	tests := []struct {
		name  string
		input ListContentInput
	}{
		{"negative start", ListContentInput{Start: intPtr(-1)}},
		{"zero limit", ListContentInput{Limit: intPtr(0)}},
		{"unsupported type", ListContentInput{Type: "comment"}},
		{"bad posting day", ListContentInput{PostingDay: "08/09/2026"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorCode(t, authenticatedTestService(server.URL, server.Client()).ListContent(context.Background(), tt.input), "VALIDATION_ERROR")
			if calls != 0 {
				t.Fatalf("validation sent %d requests", calls)
			}
		})
	}
}

func TestContentPropertyReadsSendDocumentedRequestsAndPreserveOutput(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/rest/api/content/12345/property":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results":  []any{map[string]any{"key": "build", "value": map[string]any{"number": 42}, "version": map[string]any{"number": 2}}},
				"start":    0,
				"limit":    10,
				"size":     1,
				"_links":   map[string]any{"self": "x"},
				"sentinel": "kept",
			})
		case "/rest/api/content/12345/property/build":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"key":      "build",
				"value":    map[string]any{"status": "passed"},
				"version":  map[string]any{"number": 1},
				"_links":   map[string]any{"self": "x"},
				"sentinel": "kept",
			})
		default:
			t.Fatalf("path=%q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	svc := authenticatedTestService(server.URL, server.Client())
	list := svc.ListContentProperties(context.Background(), ListContentPropertiesInput{ContentID: " 12345 "})
	if !list.Success {
		t.Fatalf("list=%+v", list)
	}
	paged := svc.ListContentProperties(context.Background(), ListContentPropertiesInput{
		ContentID: "12345",
		Expand:    "version",
		Start:     intPtr(2),
		Limit:     intPtr(5),
	})
	if !paged.Success {
		t.Fatalf("paged=%+v", paged)
	}
	get := svc.GetContentProperty(context.Background(), GetContentPropertyInput{ContentID: "12345", Key: "build", Expand: "version"})
	if !get.Success {
		t.Fatalf("get=%+v", get)
	}
	if len(seen) != 3 ||
		seen[0] != "GET /rest/api/content/12345/property?limit=10" ||
		seen[1] != "GET /rest/api/content/12345/property?expand=version&limit=5&start=2" ||
		seen[2] != "GET /rest/api/content/12345/property/build?expand=version" {
		t.Fatalf("seen=%v", seen)
	}
	if list.Data.(map[string]any)["sentinel"] != "kept" || get.Data.(map[string]any)["sentinel"] != "kept" {
		t.Fatalf("upstream data not preserved: list=%+v get=%+v", list.Data, get.Data)
	}
}

func TestContentPropertyReadsValidateInputsWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	t.Cleanup(server.Close)
	svc := authenticatedTestService(server.URL, server.Client())

	tests := []struct {
		name string
		call func() result.Envelope
	}{
		{"blank content id", func() result.Envelope {
			return svc.ListContentProperties(context.Background(), ListContentPropertiesInput{ContentID: " "})
		}},
		{"unsafe content id", func() result.Envelope {
			return svc.ListContentProperties(context.Background(), ListContentPropertiesInput{ContentID: "12/345"})
		}},
		{"negative start", func() result.Envelope {
			return svc.ListContentProperties(context.Background(), ListContentPropertiesInput{ContentID: "12345", Start: intPtr(-1)})
		}},
		{"zero limit", func() result.Envelope {
			return svc.ListContentProperties(context.Background(), ListContentPropertiesInput{ContentID: "12345", Limit: intPtr(0)})
		}},
		{"blank key", func() result.Envelope {
			return svc.GetContentProperty(context.Background(), GetContentPropertyInput{ContentID: "12345", Key: " "})
		}},
		{"unsafe key", func() result.Envelope {
			return svc.GetContentProperty(context.Background(), GetContentPropertyInput{ContentID: "12345", Key: "build/latest"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorCode(t, tt.call(), "VALIDATION_ERROR")
			if calls != 0 {
				t.Fatalf("validation sent %d requests", calls)
			}
		})
	}
}

func TestGetContentPropertyPreservesNeutralNotFoundMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "missing"})
	}))
	t.Cleanup(server.Close)

	out := authenticatedTestService(server.URL, server.Client()).GetContentProperty(context.Background(), GetContentPropertyInput{ContentID: "12345", Key: "build"})
	requireErrorCode(t, out, "NOT_FOUND_OR_NOT_VISIBLE")
}

func TestSpaceReadsSendDocumentedRequestsDefaultsAndPreserveOutput(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/rest/api/space":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"key": "ENG"}}, "limit": 25, "sentinel": "spaces"})
		case "/rest/api/space/ENG":
			_ = json.NewEncoder(w).Encode(map[string]any{"key": "ENG", "homepage": map[string]any{"id": "12345"}, "sentinel": "space"})
		case "/rest/api/space/ENG/content":
			_ = json.NewEncoder(w).Encode(map[string]any{"page": map[string]any{"results": []any{}}, "blogpost": map[string]any{"results": []any{}}, "sentinel": "content"})
		default:
			t.Fatalf("path=%q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	svc := authenticatedTestService(server.URL, server.Client())
	list := svc.ListSpaces(context.Background(), ListSpacesInput{
		SpaceKey: "ENG",
		Type:     "global",
		Status:   "current",
		Label:    "team",
		Expand:   "description.plain,homepage",
		Start:    intPtr(2),
	})
	get := svc.GetSpace(context.Background(), GetSpaceInput{SpaceKey: " ENG ", Expand: "homepage"})
	content := svc.ListSpaceContent(context.Background(), ListSpaceContentInput{SpaceKey: "ENG", Depth: "root", Expand: "version"})
	defaultContent := svc.ListSpaceContent(context.Background(), ListSpaceContentInput{SpaceKey: "ENG"})
	if !list.Success || !get.Success || !content.Success || !defaultContent.Success {
		t.Fatalf("list=%+v get=%+v content=%+v default=%+v", list, get, content, defaultContent)
	}
	if len(seen) != 4 {
		t.Fatalf("seen=%v", seen)
	}
	listQuery := requireQuery(t, strings.TrimPrefix(seen[0], "GET /rest/api/space?"))
	for key, want := range map[string]string{
		"spaceKey": "ENG",
		"type":     "global",
		"status":   "current",
		"label":    "team",
		"expand":   "description.plain,homepage",
		"start":    "2",
		"limit":    "25",
	} {
		if listQuery.Get(key) != want {
			t.Fatalf("list query[%s]=%q, want %q in %q", key, listQuery.Get(key), want, seen[0])
		}
	}
	if seen[1] != "GET /rest/api/space/ENG?expand=homepage" ||
		seen[2] != "GET /rest/api/space/ENG/content?depth=root&expand=version&limit=25" ||
		seen[3] != "GET /rest/api/space/ENG/content?limit=25" {
		t.Fatalf("seen=%v", seen)
	}
	if list.Data.(map[string]any)["sentinel"] != "spaces" || get.Data.(map[string]any)["sentinel"] != "space" || content.Data.(map[string]any)["sentinel"] != "content" {
		t.Fatalf("upstream data not preserved: list=%+v get=%+v content=%+v", list.Data, get.Data, content.Data)
	}
}

func TestSpaceReadsValidateInputsWithoutNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	t.Cleanup(server.Close)
	svc := authenticatedTestService(server.URL, server.Client())

	tests := []struct {
		name string
		call func() result.Envelope
	}{
		{"unsupported space type", func() result.Envelope {
			return svc.ListSpaces(context.Background(), ListSpacesInput{Type: "team"})
		}},
		{"unsupported space status", func() result.Envelope {
			return svc.ListSpaces(context.Background(), ListSpacesInput{Status: "trashed"})
		}},
		{"negative list spaces start", func() result.Envelope {
			return svc.ListSpaces(context.Background(), ListSpacesInput{Start: intPtr(-1)})
		}},
		{"unsafe get space key", func() result.Envelope {
			return svc.GetSpace(context.Background(), GetSpaceInput{SpaceKey: "ENG/DEV"})
		}},
		{"unsupported depth", func() result.Envelope {
			return svc.ListSpaceContent(context.Background(), ListSpaceContentInput{SpaceKey: "ENG", Depth: "children"})
		}},
		{"unsafe content space key", func() result.Envelope {
			return svc.ListSpaceContent(context.Background(), ListSpaceContentInput{SpaceKey: "ENG?"})
		}},
		{"zero space content limit", func() result.Envelope {
			return svc.ListSpaceContent(context.Background(), ListSpaceContentInput{SpaceKey: "ENG", Limit: intPtr(0)})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorCode(t, tt.call(), "VALIDATION_ERROR")
			if calls != 0 {
				t.Fatalf("validation sent %d requests", calls)
			}
		})
	}
}

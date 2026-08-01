package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chiendao1808/atlassian-mcp/internal/jira/auth"
)

func TestClientBuildsContextPathURLAndAddsBasicAuth(t *testing.T) {
	var seenPath, seenQuery, seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.RawQuery
		seenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
	}))
	t.Cleanup(server.Close)

	c := New(server.URL+"/jira", server.Client(), 1<<20)
	var out map[string]any
	err := c.GetJSON(context.Background(), auth.NewCredential("alice", "secret"), "/issue/PROJ-1", map[string][]string{"fields": {"summary,status"}}, &out)
	if err != nil {
		t.Fatalf("GetJSON error = %v", err)
	}
	if seenPath != "/jira/rest/api/2/issue/PROJ-1" || seenQuery != "fields=summary%2Cstatus" {
		t.Fatalf("path=%q query=%q", seenPath, seenQuery)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q", seenAuth)
	}
}

func TestClientMapsJiraAndProxyErrorsWithoutLeakingAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["bad password"],"errors":{"password":"sentinel-secret"}}`))
	}))
	t.Cleanup(server.Close)

	c := New(server.URL, server.Client(), 1<<20)
	var out map[string]any
	err := c.GetJSON(context.Background(), auth.NewCredential("alice", "sentinel-secret"), "/myself", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "sentinel-secret") || strings.Contains(strings.ToLower(err.Error()), "authorization") {
		t.Fatalf("error leaked secret: %v", err)
	}
}
